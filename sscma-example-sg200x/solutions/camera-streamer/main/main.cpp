#include <chrono>
#include <iostream>
#include <set>
#include <queue>
#include <mutex>
#include <atomic>
#include <string.h>
#include <signal.h>
#include <unistd.h>
#include <stdint.h>
#include <string>
#include <cinttypes>
#include <stdio.h>
#include <stdlib.h>
#include <cstring> // Needed for memset
#include <vector>
#include <type_traits>
#include <sys/timex.h>  // For adjtimex() NTP sync check
#include <openssl/sha.h>  // For SHA256 frame hashing (evidence integrity)

extern "C" {
#include "video.h"
#include "video_shm.h"
#include "video_raw_shm.h"
#include "mongoose.h"
#include <cvi_sys.h>
}

#define TAG "camera-streamer"
#define WS_PORT "8765"
#define MAX_QUEUE_SIZE 30  // Drop frames if queue gets too large
#define NUM_CHANNELS 3     // CH0, CH1, CH2 (H.264 encoded)
#define RAW_CHANNEL_ID 2   // Use CH2 VPSS for raw RGB frames (shared with low-res H.264)
#define POLL_TIMEOUT_MS 10 // Mongoose poll timeout
#define MAX_FRAMES_PER_BATCH 3 // Process up to 3 frames per channel per iteration

// NTP Synchronization state
// Timestamps are only valid when NTP is synchronized. We check this before
// calibrating PTS and before writing frames to shared memory.
static std::atomic<bool> g_ntp_synced{false};
static std::atomic<uint64_t> g_last_ntp_check{0};
static constexpr uint64_t NTP_CHECK_INTERVAL_MS = 5000;  // Check NTP sync every 5 seconds
static constexpr int64_t MIN_VALID_EPOCH_MS = 1577836800000LL;  // Jan 1, 2020 00:00:00 UTC

// PTS Calibration for timestamp synchronization (declared early for use in update_ntp_status)
static std::atomic<bool> g_pts_calibrated{false};

// Check if NTP is synchronized using adjtimex()
// Returns true if system clock is synchronized to NTP
static bool check_ntp_sync() {
    struct timex tx;
    memset(&tx, 0, sizeof(tx));

    int status = adjtimex(&tx);
    if (status < 0) {
        // adjtimex failed, fall back to checking if time is reasonable
        auto now = std::chrono::system_clock::now();
        int64_t epoch_ms = std::chrono::duration_cast<std::chrono::milliseconds>(
            now.time_since_epoch()).count();
        return epoch_ms > MIN_VALID_EPOCH_MS;
    }

    // Check if clock is synchronized (STA_UNSYNC flag is NOT set)
    // status == TIME_OK means clock is synchronized
    // STA_UNSYNC in tx.status means clock is not synchronized
    bool synced = (status == TIME_OK) || !(tx.status & STA_UNSYNC);

    // Also verify time is reasonable (not 1970)
    if (synced) {
        auto now = std::chrono::system_clock::now();
        int64_t epoch_ms = std::chrono::duration_cast<std::chrono::milliseconds>(
            now.time_since_epoch()).count();
        if (epoch_ms < MIN_VALID_EPOCH_MS) {
            synced = false;
        }
    }

    return synced;
}

// Update NTP sync status (called periodically)
static void update_ntp_status() {
    auto now = std::chrono::system_clock::now();
    uint64_t now_ms = std::chrono::duration_cast<std::chrono::milliseconds>(
        now.time_since_epoch()).count();

    uint64_t last_check = g_last_ntp_check.load(std::memory_order_acquire);
    if (now_ms - last_check < NTP_CHECK_INTERVAL_MS) {
        return;  // Not time to check yet
    }

    bool was_synced = g_ntp_synced.load(std::memory_order_acquire);
    bool is_synced = check_ntp_sync();

    if (is_synced != was_synced) {
        g_ntp_synced.store(is_synced, std::memory_order_release);
        if (is_synced) {
            printf("%s: NTP synchronized - timestamps are now valid\n", TAG);
            // Force recalibration on next frame
            g_pts_calibrated.store(false, std::memory_order_release);
        } else {
            printf("%s: WARNING: NTP sync lost - timestamps may be inaccurate\n", TAG);
        }
    }

    g_last_ntp_check.store(now_ms, std::memory_order_release);
}

// Check if timestamps are valid (NTP synced)
static bool is_timestamp_valid() {
    return g_ntp_synced.load(std::memory_order_acquire);
}

// PTS Calibration for timestamp synchronization
// The hardware PTS provides a stable monotonic timestamp from the encoder,
// which we calibrate to epoch time at first frame. This provides better
// timing accuracy than using system clock at callback time.
// NOTE: Only calibrates when NTP is synchronized!
// g_pts_calibrated is declared earlier (needed by update_ntp_status)
static std::atomic<int64_t> g_pts_offset{0};        // Offset to convert PTS → epoch ms
static std::atomic<uint64_t> g_pts_timebase{1000};  // PTS units per millisecond (default: microseconds)
static std::atomic<uint64_t> g_last_recalibration_check{0};  // Last time we checked for drift
static constexpr int64_t PTS_DRIFT_THRESHOLD_MS = 1000;      // Recalibrate if drift > 1 second
static constexpr uint64_t RECALIBRATION_CHECK_INTERVAL_MS = 60000;  // Check every 60 seconds

// Calibrate PTS to epoch time on first H.264 frame
// Hardware PTS is typically in microseconds since video start, so we establish
// a mapping to epoch time here.
// Returns false if NTP is not synced (timestamps would be invalid)
static bool calibrate_pts(uint64_t hardware_pts) {
    // Check NTP sync status periodically
    update_ntp_status();

    // Don't calibrate if NTP is not synced - timestamps would be meaningless
    if (!g_ntp_synced.load(std::memory_order_acquire)) {
        return false;
    }

    auto now = std::chrono::system_clock::now();
    int64_t epoch_ms = std::chrono::duration_cast<std::chrono::milliseconds>(
        now.time_since_epoch()).count();
    int64_t pts_ms = static_cast<int64_t>(hardware_pts / g_pts_timebase.load());

    if (!g_pts_calibrated.load(std::memory_order_acquire)) {
        // Initial calibration
        g_pts_offset.store(epoch_ms - pts_ms, std::memory_order_release);
        g_pts_calibrated.store(true, std::memory_order_release);
        g_last_recalibration_check.store(static_cast<uint64_t>(epoch_ms), std::memory_order_release);

        printf("%s: PTS calibrated (NTP synced): hardware_pts=%lu, epoch_ms=%ld, offset=%ld\n",
               TAG, hardware_pts, epoch_ms, g_pts_offset.load());
        return true;
    }

    // Periodic drift check (handles NTP adjustments)
    uint64_t last_check = g_last_recalibration_check.load(std::memory_order_acquire);
    if (static_cast<uint64_t>(epoch_ms) - last_check < RECALIBRATION_CHECK_INTERVAL_MS) {
        return true;  // Already calibrated, not time to check drift yet
    }

    // Calculate expected timestamp vs actual system time
    int64_t expected_ms = g_pts_offset.load(std::memory_order_acquire) + pts_ms;
    int64_t drift = epoch_ms - expected_ms;

    if (drift > PTS_DRIFT_THRESHOLD_MS || drift < -PTS_DRIFT_THRESHOLD_MS) {
        // Significant drift detected (likely NTP adjustment), recalibrate
        int64_t new_offset = epoch_ms - pts_ms;
        g_pts_offset.store(new_offset, std::memory_order_release);
        printf("%s: PTS recalibrated due to %ldms drift (NTP adjustment?), new offset=%ld\n",
               TAG, drift, new_offset);
    }

    g_last_recalibration_check.store(static_cast<uint64_t>(epoch_ms), std::memory_order_release);
    return true;
}

// Convert hardware PTS to epoch milliseconds
// Returns 0 if NTP is not synced or PTS not calibrated (invalid timestamp)
static uint64_t pts_to_epoch_ms(uint64_t hardware_pts) {
    // If NTP is not synced, return 0 to indicate invalid timestamp
    if (!g_ntp_synced.load(std::memory_order_acquire)) {
        return 0;
    }

    if (!g_pts_calibrated.load(std::memory_order_acquire)) {
        // NTP is synced but PTS not yet calibrated - use system clock
        auto now = std::chrono::system_clock::now();
        return std::chrono::duration_cast<std::chrono::milliseconds>(
            now.time_since_epoch()).count();
    }

    int64_t pts_ms = static_cast<int64_t>(hardware_pts / g_pts_timebase.load());
    return static_cast<uint64_t>(g_pts_offset.load(std::memory_order_acquire) + pts_ms);
}

// Get current epoch timestamp (for raw frames)
// Returns 0 if NTP is not synced (invalid timestamp)
static uint64_t get_current_timestamp_ms() {
    // If NTP is not synced, return 0 to indicate invalid timestamp
    if (!g_ntp_synced.load(std::memory_order_acquire)) {
        return 0;
    }

    auto now = std::chrono::system_clock::now();
    return std::chrono::duration_cast<std::chrono::milliseconds>(
        now.time_since_epoch()).count();
}

// Evidence chain configuration
// Frame hashing is disabled by default for performance. Enable via environment variable.
static std::atomic<bool> g_evidence_chain_enabled{false};
static std::atomic<bool> g_evidence_config_loaded{false};

// Check if evidence chain is enabled (cached after first check)
static bool is_evidence_chain_enabled() {
    if (!g_evidence_config_loaded.load(std::memory_order_acquire)) {
        // Check environment variable
        const char* env = getenv("RECAMERA_EVIDENCE_CHAIN");
        bool enabled = (env != nullptr && (strcmp(env, "1") == 0 || strcmp(env, "true") == 0));
        g_evidence_chain_enabled.store(enabled, std::memory_order_release);
        g_evidence_config_loaded.store(true, std::memory_order_release);
        if (enabled) {
            printf("%s: Evidence chain enabled - computing frame hashes\n", TAG);
        }
    }
    return g_evidence_chain_enabled.load(std::memory_order_acquire);
}

// Compute SHA256 hash of frame data for evidence integrity
// This binds the frame content to its metadata, preventing tampering
static void compute_frame_hash(const uint8_t* data, size_t len, uint8_t* hash_out) {
    SHA256(data, len, hash_out);
}

// Per-channel state structure
typedef struct {
    video_ch_index_t channel_id;
    video_ch_param_t params;
    std::queue<std::pair<uint8_t*, size_t>> frame_queue;
    std::mutex queue_mutex;
    std::set<struct mg_connection*> ws_clients;
    std::mutex clients_mutex;
    video_shm_producer_t shm_producer;
    bool shm_enabled;
    std::vector<uint8_t> sps_cache;  // Cache SPS for keyframes
    std::vector<uint8_t> pps_cache;  // Cache PPS for keyframes
    std::mutex header_mutex;
} channel_state_t;

static volatile bool g_running = true;
static struct mg_mgr g_mgr;
static channel_state_t g_channels[NUM_CHANNELS];

// Raw frame state for ML inference
static video_raw_producer_t g_raw_producer;
static bool g_raw_shm_enabled = false;
static uint32_t g_raw_frame_sequence = 0;

// Signal handler for graceful shutdown
static void signal_handler(int signo) {
    if (signo == SIGINT || signo == SIGTERM) {
        printf("%s: Received signal %d, shutting down...\n", TAG, signo);
        g_running = false;
    }
}

// Helper function to parse WebSocket channel parameter
static int parse_channel_param(struct mg_http_message *hm) {
    char channel_str[8] = {0};
    int channel = -1;
    
    // Extract channel parameter from query string
    int len = mg_http_get_var(&hm->query, "channel", channel_str, sizeof(channel_str));
    if (len > 0) {
        channel = atoi(channel_str);
        if (channel >= 0 && channel < NUM_CHANNELS) {
            return channel;
        }
    }
    return -1;  // Invalid or missing channel parameter
}

// Helper to set channel ID in connection data
static void set_connection_channel(struct mg_connection *c, int channel) {
    // Store channel in first byte of connection data
    c->data[0] = (char)channel;
}

// Helper to get channel ID from connection data
static int get_connection_channel(struct mg_connection *c) {
    return (int)(unsigned char)c->data[0];
}

// WebSocket event handler
static void ws_handler(struct mg_connection *c, int ev, void *ev_data) {
    if (ev == MG_EV_HTTP_MSG) {
        struct mg_http_message *hm = (struct mg_http_message *) ev_data;
        if (mg_match(hm->uri, mg_str("/"), NULL)) {
            // Parse required channel parameter
            int channel = parse_channel_param(hm);
            if (channel < 0) {
                // Reject connection - channel parameter required
                mg_http_reply(c, 400, "Content-Type: text/plain\r\n",
                            "Error: channel parameter required (0-2)\n"
                            "Example: ws://device-ip:8765/?channel=1\n");
                return;
            }
            
            // Store channel ID in connection data
            set_connection_channel(c, channel);
            mg_ws_upgrade(c, hm, NULL);
        } else {
            mg_http_reply(c, 404, "", "Not Found\n");
        }
    } else if (ev == MG_EV_WS_OPEN) {
        int channel = get_connection_channel(c);
        std::lock_guard<std::mutex> lock(g_channels[channel].clients_mutex);
        g_channels[channel].ws_clients.insert(c);
        printf("%s: WebSocket client connected to CH%d (%zu total)\n", 
               TAG, channel, g_channels[channel].ws_clients.size());
    } else if (ev == MG_EV_CLOSE || ev == MG_EV_ERROR) {
        // Remove from the channel this connection was subscribed to
        int channel = get_connection_channel(c);
        if (channel >= 0 && channel < NUM_CHANNELS) {
            std::lock_guard<std::mutex> lock(g_channels[channel].clients_mutex);
            auto it = g_channels[channel].ws_clients.find(c);
            if (it != g_channels[channel].ws_clients.end()) {
                g_channels[channel].ws_clients.erase(it);
                printf("%s: WebSocket client disconnected from CH%d (%zu remaining)\n", 
                       TAG, channel, g_channels[channel].ws_clients.size());
            }
        }
    }
}

// Video frame callback - queues frames for main thread to send
static int video_frame_callback(void* pData, void* pArgs, void* pUserData) {
    VENC_STREAM_S* pstStream = (VENC_STREAM_S*)pData;
    channel_state_t* channel = (channel_state_t*)pUserData;

    if (!g_running || pstStream->u32PackCount == 0 || !channel) {
        return CVI_SUCCESS;
    }

    // Prepare frame data
    for (CVI_U32 i = 0; i < pstStream->u32PackCount; i++) {
        VENC_PACK_S* ppack = &pstStream->pstPack[i];

        // Use hardware PTS for accurate timestamp synchronization
        // Calibrate on first frame to establish PTS-to-epoch mapping
        // calibrate_pts returns false if NTP is not synced
        calibrate_pts(ppack->u64PTS);
        uint64_t timestamp = pts_to_epoch_ms(ppack->u64PTS);
        // Note: timestamp will be 0 if NTP is not synced - consumers should handle this
        uint8_t* frame_data = ppack->pu8Addr + ppack->u32Offset;
        uint32_t frame_len = ppack->u32Len - ppack->u32Offset;

        // Detect SPS/PPS and cache them
        bool is_sps = (ppack->DataType.enH264EType == H264E_NALU_SPS);
        bool is_pps = (ppack->DataType.enH264EType == H264E_NALU_PPS);
        bool is_keyframe = (ppack->DataType.enH264EType == H264E_NALU_IDRSLICE ||
                           ppack->DataType.enH264EType == H264E_NALU_ISLICE);

        if (is_sps) {
            std::lock_guard<std::mutex> lock(channel->header_mutex);
            channel->sps_cache.assign(frame_data, frame_data + frame_len);
            // Don't send SPS separately, we'll prepend to keyframes
            continue;
        }
        
        if (is_pps) {
            std::lock_guard<std::mutex> lock(channel->header_mutex);
            channel->pps_cache.assign(frame_data, frame_data + frame_len);
            // Don't send PPS separately, we'll prepend to keyframes
            continue;
        }

        // For keyframes, prepend SPS+PPS - allocate persistent buffer
        uint8_t* final_frame_data = frame_data;
        uint32_t final_frame_len = frame_len;
        uint8_t* combined_buffer = nullptr;
        
        if (is_keyframe) {
            std::lock_guard<std::mutex> lock(channel->header_mutex);
            if (!channel->sps_cache.empty() && !channel->pps_cache.empty()) {
                // Allocate persistent buffer for combined frame: SPS + PPS + Keyframe
                final_frame_len = channel->sps_cache.size() + channel->pps_cache.size() + frame_len;
                combined_buffer = new uint8_t[final_frame_len];
                
                // Copy SPS + PPS + Keyframe into persistent buffer
                size_t offset = 0;
                memcpy(combined_buffer + offset, channel->sps_cache.data(), channel->sps_cache.size());
                offset += channel->sps_cache.size();
                memcpy(combined_buffer + offset, channel->pps_cache.data(), channel->pps_cache.size());
                offset += channel->pps_cache.size();
                memcpy(combined_buffer + offset, frame_data, frame_len);
                
                final_frame_data = combined_buffer;
            }
        }

        // Write to shared memory (zero-copy for local apps)
        // Note: timestamp_ms will be 0 if NTP is not synced - consumers should handle this
        if (channel->shm_enabled) {
            video_frame_meta_t meta = {0};
            meta.timestamp_ms = timestamp;
            meta.size = final_frame_len;
            meta.is_keyframe = is_keyframe ? 1 : 0;
            meta.codec = 0;  // H.264
            meta.width = channel->params.width;
            meta.height = channel->params.height;
            meta.fps = channel->params.fps;

            // Compute SHA256 hash only if evidence chain is enabled (avoid overhead otherwise)
            if (is_evidence_chain_enabled()) {
                compute_frame_hash(final_frame_data, final_frame_len, meta.frame_hash);
            }

            if (video_shm_producer_write(&channel->shm_producer, final_frame_data, final_frame_len, &meta) < 0) {
                printf("%s: WARNING: Failed to write frame to shared memory CH%d\n",
                       TAG, channel->channel_id);
            }
        }

        // Allocate buffer for frame: [channel_id(1)] + [frame_data(N)] + [timestamp(8)]
        size_t total_len = 1 + final_frame_len + 8;
        uint8_t* buffer = new uint8_t[total_len];
        
        // Pack: channel ID + frame data + timestamp
        buffer[0] = (uint8_t)channel->channel_id;
        memcpy(buffer + 1, final_frame_data, final_frame_len);
        memcpy(buffer + 1 + final_frame_len, &timestamp, 8);
        
        // Free combined buffer if it was allocated
        if (combined_buffer) {
            delete[] combined_buffer;
        }

        // Queue frame for main thread to send
        {
            std::lock_guard<std::mutex> lock(channel->queue_mutex);
            if (channel->frame_queue.size() < MAX_QUEUE_SIZE) {
                channel->frame_queue.push(std::make_pair(buffer, total_len));
            } else {
                // Queue full, drop frame and free buffer
                delete[] buffer;
            }
        }
    }

    return CVI_SUCCESS;
}

// Raw frame callback for ML inference - receives VIDEO_FRAME_INFO_S from VPSS
// Static buffer for interleaved RGB output
static uint8_t g_interleaved_buffer[VIDEO_RAW_FRAME_SIZE];

static int raw_frame_callback(void* pData, void* pArgs, void* pUserData) {
    if (!g_running || !g_raw_shm_enabled) {
        return CVI_SUCCESS;
    }

    // Note: timestamp_ms will be 0 if NTP is not synced - consumers should handle this

    VIDEO_FRAME_INFO_S* pFrame = (VIDEO_FRAME_INFO_S*)pData;
    VIDEO_FRAME_S* f = &pFrame->stVFrame;

    // Only process RGB888 frames
    if (f->enPixelFormat != PIXEL_FORMAT_RGB_888) {
        return CVI_SUCCESS;
    }

    // Raw frames from VPSS don't have hardware PTS like VENC frames.
    // Use system clock but leverage the PTS calibration from H.264 channel
    // to ensure timestamps are in the same epoch timebase.
    // Note: There may be a small offset between raw and H.264 frames due to
    // pipeline latency, but they'll be in the same time domain.
    uint64_t timestamp_ms = get_current_timestamp_ms();

    uint32_t width = f->u32Width;
    uint32_t height = f->u32Height;
    uint32_t stride = f->u32Stride[0];
    uint32_t total_size = width * height * 3;

    // Detect format: interleaved, planar (separate buffers), or planar-packed (single buffer)
    // - Planar (separate): PhyAddr[1] and PhyAddr[2] non-zero with separate lengths
    // - Planar-packed: All 3 planes packed in single buffer, length[0] = width * height * 3,
    //                  PhyAddr[1/2] = 0. Stride may be width OR width*3 (SDK inconsistency)
    // - True interleaved: stride[0] = width * 3, data is RGB RGB RGB...
    bool is_planar_separate = (f->u64PhyAddr[1] != 0) && (f->u64PhyAddr[2] != 0) &&
                              (f->u32Length[1] != 0) && (f->u32Length[2] != 0);
    // Planar-packed: single buffer with all 3 planes, length = w*h*3, no secondary addresses
    bool is_planar_packed = !is_planar_separate &&
                            (f->u32Length[0] == width * height * 3) &&
                            (f->u64PhyAddr[1] == 0) && (f->u64PhyAddr[2] == 0);
    bool is_interleaved = !is_planar_packed && !is_planar_separate;

    // Debug: log format detection to help diagnose issues
    static bool format_logged = false;
    if (!format_logged) {
        const char* format_name = is_planar_packed ? "planar-packed" :
                                  is_planar_separate ? "planar-separate" : "interleaved";
        printf("%s: Frame %ux%u strides=[%u,%u,%u] lengths=[%u,%u,%u] phyaddr=[0x%lx,0x%lx,0x%lx] format=%s\n",
               TAG, width, height,
               f->u32Stride[0], f->u32Stride[1], f->u32Stride[2],
               f->u32Length[0], f->u32Length[1], f->u32Length[2],
               (unsigned long)f->u64PhyAddr[0], (unsigned long)f->u64PhyAddr[1], (unsigned long)f->u64PhyAddr[2],
               format_name);
        format_logged = true;
    }

    if (width == 0 || height == 0 || total_size > VIDEO_RAW_FRAME_SIZE) {
        printf("%s: Raw frame size invalid: %ux%u (max %u bytes)\n", TAG, width, height, VIDEO_RAW_FRAME_SIZE);
        return CVI_SUCCESS;
    }

    if (is_planar_packed) {
        // Row-planar format: each row contains [R0..Rw][G0..Gw][B0..Bw]
        // Stride = width * 3 = 1920, each color channel is 640 bytes per row
        uint8_t* src = (uint8_t*)CVI_SYS_Mmap(f->u64PhyAddr[0], f->u32Length[0]);
        if (!src) {
            printf("%s: Failed to mmap planar-packed frame (phy=0x%lx, len=%u)\n",
                   TAG, (unsigned long)f->u64PhyAddr[0], f->u32Length[0]);
            return CVI_SUCCESS;
        }

        // Direct memcpy - fastest possible
        memcpy(g_interleaved_buffer, src, width * height * 3);

        CVI_SYS_Munmap(src, f->u32Length[0]);
    } else if (is_interleaved) {
        // True interleaved RGB - fast copy with stride handling
        uint8_t* src = (uint8_t*)CVI_SYS_Mmap(f->u64PhyAddr[0], f->u32Length[0]);
        if (!src) {
            printf("%s: Failed to mmap interleaved frame (phy=0x%lx, len=%u)\n",
                   TAG, (unsigned long)f->u64PhyAddr[0], f->u32Length[0]);
            return CVI_SUCCESS;
        }

        // Fast copy with stride handling
        uint32_t row_bytes = width * 3;
        for (uint32_t y = 0; y < height; y++) {
            memcpy(&g_interleaved_buffer[y * row_bytes], &src[y * stride], row_bytes);
        }

        CVI_SYS_Munmap(src, f->u32Length[0]);
    } else {
        // Planar BGR format - convert to interleaved RGB
        // CVI uses BGR plane order despite RGB_888 name
        uint32_t stride_b = f->u32Stride[0];  // Blue plane stride
        uint32_t stride_g = f->u32Stride[1];  // Green plane stride
        uint32_t stride_r = f->u32Stride[2];  // Red plane stride

        uint8_t* plane_b = (uint8_t*)CVI_SYS_Mmap(f->u64PhyAddr[0], f->u32Length[0]);
        uint8_t* plane_g = (uint8_t*)CVI_SYS_Mmap(f->u64PhyAddr[1], f->u32Length[1]);
        uint8_t* plane_r = (uint8_t*)CVI_SYS_Mmap(f->u64PhyAddr[2], f->u32Length[2]);

        if (!plane_r || !plane_g || !plane_b) {
            printf("%s: Failed to mmap planar frame planes\n", TAG);
            if (plane_b) CVI_SYS_Munmap(plane_b, f->u32Length[0]);
            if (plane_g) CVI_SYS_Munmap(plane_g, f->u32Length[1]);
            if (plane_r) CVI_SYS_Munmap(plane_r, f->u32Length[2]);
            return CVI_SUCCESS;
        }

        // Convert planar BGR to interleaved RGB - optimized with loop unrolling
        for (uint32_t y = 0; y < height; y++) {
            uint8_t* r = &plane_r[y * stride_r];
            uint8_t* g = &plane_g[y * stride_g];
            uint8_t* b = &plane_b[y * stride_b];
            uint8_t* d = &g_interleaved_buffer[y * width * 3];
            uint32_t x = width;

            // Process 4 pixels at a time - output RGB order
            while (x >= 4) {
                d[0] = r[0]; d[1] = g[0]; d[2] = b[0];
                d[3] = r[1]; d[4] = g[1]; d[5] = b[1];
                d[6] = r[2]; d[7] = g[2]; d[8] = b[2];
                d[9] = r[3]; d[10] = g[3]; d[11] = b[3];
                r += 4; g += 4; b += 4; d += 12; x -= 4;
            }
            while (x > 0) {
                d[0] = *r++; d[1] = *g++; d[2] = *b++;
                d += 3; x--;
            }
        }

        CVI_SYS_Munmap(plane_b, f->u32Length[0]);
        CVI_SYS_Munmap(plane_g, f->u32Length[1]);
        CVI_SYS_Munmap(plane_r, f->u32Length[2]);
    }

    // Prepare metadata
    video_raw_meta_t meta = {0};
    meta.timestamp_ms = timestamp_ms;
    meta.size = total_size;
    meta.sequence = g_raw_frame_sequence++;
    meta.width = width;
    meta.height = height;
    meta.format = VIDEO_RAW_FORMAT_RGB888;

    // Compute SHA256 hash only if evidence chain is enabled (avoid overhead otherwise)
    if (is_evidence_chain_enabled()) {
        compute_frame_hash(g_interleaved_buffer, meta.size, meta.frame_hash);
    }

    // Write to raw shared memory
    if (video_raw_producer_write(&g_raw_producer, g_interleaved_buffer, meta.size, &meta) < 0) {
        printf("%s: Failed to write raw frame to shared memory\n", TAG);
    }

    return CVI_SUCCESS;
}

// Initialize a single channel
static int init_channel(channel_state_t* channel, video_ch_index_t ch_id,
                        const video_ch_param_t* params) {
    channel->channel_id = ch_id;
    channel->params = *params;
    channel->shm_enabled = false;
    
    // Initialize shared memory IPC with channel-specific name
    printf("%s: Initializing shared memory for CH%d at /video_stream_ch%d\n", TAG, ch_id, ch_id);
    
    if (video_shm_producer_init_channel(&channel->shm_producer, ch_id) != 0) {
        fprintf(stderr, "%s: WARNING: Failed to initialize shared memory for CH%d\n", TAG, ch_id);
    } else {
        channel->shm_enabled = true;
        printf("%s: Shared memory IPC enabled for CH%d\n", TAG, ch_id);
    }
    
    // Configure video channel
    printf("%s: Configuring CH%d: %dx%d @ %dfps H.264\n", 
           TAG, ch_id, params->width, params->height, params->fps);
    
    if (setupVideo(ch_id, params) != 0) {
        fprintf(stderr, "%s: Failed to setup CH%d\n", TAG, ch_id);
        if (channel->shm_enabled) {
            video_shm_producer_destroy(&channel->shm_producer);
        }
        return -1;
    }
    
    // Register frame callback with channel context
    registerVideoFrameHandler(ch_id, 0, video_frame_callback, channel);
    
    return 0;
}

// Cleanup a single channel
static void cleanup_channel(channel_state_t* channel) {
    printf("%s: Cleaning up CH%d...\n", TAG, channel->channel_id);
    
    // Clear frame queue
    {
        std::lock_guard<std::mutex> lock(channel->queue_mutex);
        while (!channel->frame_queue.empty()) {
            delete[] channel->frame_queue.front().first;
            channel->frame_queue.pop();
        }
    }
    
    // Cleanup shared memory
    if (channel->shm_enabled) {
        video_shm_producer_destroy(&channel->shm_producer);
    }
}

// Process queued frames and send to clients (called from main thread)
static void process_frame_queues() {
    for (int ch = 0; ch < NUM_CHANNELS; ch++) {
        channel_state_t* channel = &g_channels[ch];
        
        // Process multiple frames per iteration for better throughput
        int frames_processed = 0;
        while (frames_processed < MAX_FRAMES_PER_BATCH) {
            std::pair<uint8_t*, size_t> frame;
            bool has_frame = false;
            
            // Get frame from queue
            {
                std::lock_guard<std::mutex> lock(channel->queue_mutex);
                if (!channel->frame_queue.empty()) {
                    frame = channel->frame_queue.front();
                    channel->frame_queue.pop();
                    has_frame = true;
                }
            }
            
            if (!has_frame) {
                break; // No more frames for this channel
            }
            
            // Make a copy of client connections to avoid holding mutex during I/O
            std::vector<struct mg_connection*> clients_copy;
            {
                std::lock_guard<std::mutex> lock(channel->clients_mutex);
                clients_copy.reserve(channel->ws_clients.size());
                for (auto conn : channel->ws_clients) {
                    clients_copy.push_back(conn);
                }
            }
            
            // Send to all clients without holding the mutex
            for (auto conn : clients_copy) {
                if (conn && conn->is_websocket) {
                    mg_ws_send(conn, frame.first, frame.second, WEBSOCKET_OP_BINARY);
                }
            }
            
            // Free buffer
            delete[] frame.first;
            frames_processed++;
        }
    }
}

int main(int argc, char* argv[]) {
    printf("%s: Starting multi-channel camera streamer on port %s\n", TAG, WS_PORT);

    // Setup signal handlers
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);

    // Check initial NTP sync status
    // Timestamps are only valid when NTP is synchronized
    bool initial_ntp_sync = check_ntp_sync();
    g_ntp_synced.store(initial_ntp_sync, std::memory_order_release);
    if (initial_ntp_sync) {
        printf("%s: NTP synchronized - timestamps will be valid\n", TAG);
    } else {
        printf("%s: WARNING: NTP not synchronized - timestamps will be 0 until sync\n", TAG);
    }

    // Initialize video subsystem
    printf("%s: Initializing video subsystem...\n", TAG);
    if (initVideo() != 0) {
        fprintf(stderr, "%s: Failed to initialize video\n", TAG);
        return -1;
    }

    // Configure H.264 encoded channels (CH0, CH1)
    // Note: CH2 is reserved for raw RGB frames for ML inference
    video_ch_param_t params[NUM_CHANNELS] = {
        // CH0: High resolution - 1920x1080 @ 30fps
        { .format = VIDEO_FORMAT_H264, .width = 1920, .height = 1080, .fps = 30 },
        // CH1: Medium resolution - 1280x720 @ 30fps
        { .format = VIDEO_FORMAT_H264, .width = 1280, .height = 720, .fps = 30 },
        // CH2: Raw RGB frames - 640x640 @ 10fps for ML inference
        { .format = VIDEO_FORMAT_RGB888, .width = VIDEO_RAW_WIDTH, .height = VIDEO_RAW_HEIGHT, .fps = 10 }
    };

    // Initialize raw frame shared memory producer
    printf("%s: Initializing raw frame shared memory at /video_raw_ch0\n", TAG);
    if (video_raw_producer_init(&g_raw_producer) != 0) {
        fprintf(stderr, "%s: WARNING: Failed to initialize raw frame shared memory\n", TAG);
        g_raw_shm_enabled = false;
    } else {
        g_raw_shm_enabled = true;
        printf("%s: Raw frame shared memory enabled (%dx%d RGB888)\n", TAG, VIDEO_RAW_WIDTH, VIDEO_RAW_HEIGHT);
    }

    // Initialize all channels
    bool all_channels_ok = true;
    for (int ch = 0; ch < NUM_CHANNELS; ch++) {
        if (init_channel(&g_channels[ch], (video_ch_index_t)ch, &params[ch]) != 0) {
            fprintf(stderr, "%s: Failed to initialize CH%d, continuing with other channels\n", TAG, ch);
            all_channels_ok = false;
        }
    }

    // Register raw frame callback on CH2 (separate from H.264 callback)
    if (g_raw_shm_enabled) {
        // CH2 is configured for RGB888, so register raw frame callback
        registerVideoFrameHandler(VIDEO_CH2, 1, raw_frame_callback, nullptr);
        printf("%s: Raw frame callback registered on CH2\n", TAG);
    }

    if (!all_channels_ok) {
        fprintf(stderr, "%s: WARNING: Not all channels initialized successfully\n", TAG);
    }

    // Initialize Mongoose WebSocket server
    mg_mgr_init(&g_mgr);
    char url[64];
    snprintf(url, sizeof(url), "http://0.0.0.0:%s", WS_PORT);
    
    printf("%s: Starting WebSocket server on %s\n", TAG, url);
    struct mg_connection *listen_conn = mg_http_listen(&g_mgr, url, ws_handler, NULL);
    
    if (listen_conn == NULL) {
        fprintf(stderr, "%s: Failed to start WebSocket server\n", TAG);
        // Cleanup channels before deinit
        for (int ch = 0; ch < NUM_CHANNELS; ch++) {
            cleanup_channel(&g_channels[ch]);
        }
        deinitVideo();
        return -1;
    }

    // Start video streaming
    printf("%s: Starting video streams...\n", TAG);
    if (startVideo() != 0) {
        fprintf(stderr, "%s: Failed to start video streams\n", TAG);
        mg_mgr_free(&g_mgr);
        // Cleanup channels before deinit
        for (int ch = 0; ch < NUM_CHANNELS; ch++) {
            cleanup_channel(&g_channels[ch]);
        }
        deinitVideo();
        return -1;
    }

    printf("%s: Multi-channel camera streamer is running\n", TAG);
    printf("%s: CH0: 1920x1080@30fps (High) - ws://<device-ip>:%s/?channel=0\n", TAG, WS_PORT);
    printf("%s: CH1: 1280x720@30fps (Medium) - ws://<device-ip>:%s/?channel=1\n", TAG, WS_PORT);
    printf("%s: CH2: %dx%d@10fps (Raw RGB for ML) - /video_raw_ch0\n", TAG, VIDEO_RAW_WIDTH, VIDEO_RAW_HEIGHT);
    printf("%s: Press Ctrl+C to stop\n", TAG);

    // Main event loop - process both mongoose events AND frame queues
    while (g_running) {
        mg_mgr_poll(&g_mgr, POLL_TIMEOUT_MS);
        process_frame_queues();   // Send queued frames
    }

    // Cleanup
    printf("%s: Cleaning up...\n", TAG);

    // Cleanup raw frame producer
    if (g_raw_shm_enabled) {
        video_raw_producer_destroy(&g_raw_producer);
    }

    // Cleanup all channels
    for (int ch = 0; ch < NUM_CHANNELS; ch++) {
        cleanup_channel(&g_channels[ch]);
    }

    deinitVideo();
    mg_mgr_free(&g_mgr);

    printf("%s: Shutdown complete\n", TAG);
    return 0;
}
