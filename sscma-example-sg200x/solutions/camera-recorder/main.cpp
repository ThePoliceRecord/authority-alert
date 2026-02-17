#include "../../components/sophgo/video/include/video_shm.h"
#include <iostream>
#include <fstream>
#include <string>
#include <vector>
#include <cstdlib>
#include <cstring>
#include <csignal>
#include <unistd.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <utime.h>
#include <chrono>
#include <algorithm>
#include <cstdio>
#include <atomic>

extern "C" {
#include "cameradb.h"
}

extern "C" {
#include <libavformat/avformat.h>
#include <libavcodec/avcodec.h>
#include <libavutil/opt.h>
#include <libavutil/imgutils.h>
#include <libavutil/timestamp.h>
}

constexpr int64_t MAX_FILE_SIZE = 4LL * 1024 * 1024 * 1024; // 4 GB
constexpr int64_t MAX_DURATION_MS = 1 * 60 * 60 * 1000; // 1 hour

class Recorder {
private:
    std::string outputDir;
    video_shm_consumer_t consumer;
    uint8_t* buffer;
    std::atomic<bool> running;
    bool consumerInitialized;

    // libav contexts
    AVFormatContext* formatCtx;
    AVStream* videoStream;
    AVCodecContext* codecCtx;
    AVPacket* packet;

    // Timing
    std::chrono::steady_clock::time_point startTime;
    int64_t bytesWritten;
    int64_t frameCount;
    int64_t firstFrameTimestamp;
    int64_t lastDts; // Track last DTS for monotonic timestamps
    uint8_t detectedFps;
    std::string currentFilename;  // Store current recording filename

    // SPS/PPS caching for codec configuration
    std::vector<uint8_t> spsData;
    std::vector<uint8_t> ppsData;
    bool codecConfigured;

    // Configuration
    int videoWidth;
    int videoHeight;
    int videoFramerate;

    // Database integration
    cameradb_t* db;
    int64_t currentRecordingId;
    bool dbEnabled;
    
    // H.264 NAL unit types
    static const uint8_t NAL_TYPE_SPS = 7;
    static const uint8_t NAL_TYPE_PPS = 8;
    static const uint8_t NAL_TYPE_IDR = 5;
    
    std::string generateFilename() {
        auto now = std::chrono::system_clock::now();
        auto time = std::chrono::system_clock::to_time_t(now);
        char buf[64];
        strftime(buf, sizeof(buf), "recording_%Y%m%d_%H%M%S.mp4", localtime(&time));
        return outputDir + "/" + buf;
    }

    // Extract NAL units from frame data (Annex-B format)
    void extractNALUnits(const uint8_t* data, int size) {
        for (int i = 0; i <= size - 4; i++) {
            bool isStartCode = false;
            int startCodeLen = 0;
            
            if (data[i] == 0x00 && data[i+1] == 0x00) {
                if (data[i+2] == 0x01) {
                    isStartCode = true;
                    startCodeLen = 3;
                } else if (i + 3 < size && data[i+2] == 0x00 && data[i+3] == 0x01) {
                    isStartCode = true;
                    startCodeLen = 4;
                }
            }
            
            if (isStartCode && i + startCodeLen < size) {
                uint8_t nalType = data[i + startCodeLen] & 0x1F;
                
                // Find next start code to determine NAL unit size
                int nextStart = size;
                for (int j = i + startCodeLen; j < size - 4; j++) {
                    if ((data[j] == 0x00 && data[j+1] == 0x00 && data[j+2] == 0x01) ||
                        (data[j] == 0x00 && data[j+1] == 0x00 && data[j+2] == 0x00 && data[j+3] == 0x01)) {
                        nextStart = j;
                        break;
                    }
                }
                
                int nalSize = nextStart - i;
                
                if (nalType == NAL_TYPE_SPS && spsData.empty()) {
                    // Store NAL unit without start code
                    spsData.assign(data + i + startCodeLen, data + i + nalSize);
                } else if (nalType == NAL_TYPE_PPS && ppsData.empty()) {
                    // Store NAL unit without start code
                    ppsData.assign(data + i + startCodeLen, data + i + nalSize);
                }
                
                i += nalSize - 1;
            }
        }
    }

    bool configureCodec() {
        if (codecConfigured || spsData.empty() || ppsData.empty()) {
            return codecConfigured;
        }

        // Create extradata with SPS and PPS
        size_t extradataSize = spsData.size() + ppsData.size() + 100; // Extra space for headers
        uint8_t* extradata = (uint8_t*)av_malloc(extradataSize);
        if (!extradata) {
            std::cerr << "Failed to allocate extradata" << std::endl;
            return false;
        }

        // Build avcC format extradata
        uint8_t* p = extradata;
        *p++ = 1; // configurationVersion
        *p++ = spsData[1]; // AVCProfileIndication
        *p++ = spsData[2]; // profile_compatibility
        *p++ = spsData[3]; // AVCLevelIndication
        *p++ = 0xFF; // lengthSizeMinusOne (4 bytes)
        
        // SPS
        *p++ = 0xE1; // numOfSequenceParameterSets (1)
        *p++ = (spsData.size() >> 8) & 0xFF;
        *p++ = spsData.size() & 0xFF;
        memcpy(p, spsData.data(), spsData.size());
        p += spsData.size();
        
        // PPS
        *p++ = 1; // numOfPictureParameterSets
        *p++ = (ppsData.size() >> 8) & 0xFF;
        *p++ = ppsData.size() & 0xFF;
        memcpy(p, ppsData.data(), ppsData.size());
        p += ppsData.size();

        int extradataActualSize = p - extradata;

        // Set extradata for stream codecpar
        videoStream->codecpar->extradata = (uint8_t*)av_malloc(extradataActualSize + AV_INPUT_BUFFER_PADDING_SIZE);
        if (!videoStream->codecpar->extradata) {
            std::cerr << "Failed to allocate stream extradata" << std::endl;
            av_free(extradata);
            return false;
        }
        memcpy(videoStream->codecpar->extradata, extradata, extradataActualSize);
        memset(videoStream->codecpar->extradata + extradataActualSize, 0, AV_INPUT_BUFFER_PADDING_SIZE);
        videoStream->codecpar->extradata_size = extradataActualSize;

        // Also set for codec context (allocate separately)
        codecCtx->extradata = (uint8_t*)av_malloc(extradataActualSize + AV_INPUT_BUFFER_PADDING_SIZE);
        if (!codecCtx->extradata) {
            std::cerr << "Failed to allocate codec extradata" << std::endl;
            av_free(videoStream->codecpar->extradata);
            videoStream->codecpar->extradata = nullptr;
            videoStream->codecpar->extradata_size = 0;
            av_free(extradata);
            return false;
        }
        memcpy(codecCtx->extradata, extradata, extradataActualSize);
        memset(codecCtx->extradata + extradataActualSize, 0, AV_INPUT_BUFFER_PADDING_SIZE);
        codecCtx->extradata_size = extradataActualSize;

        // Free the temporary buffer
        av_free(extradata);

        codecConfigured = true;
        return true;
    }

    bool openOutputFile(const std::string& filename) {
        // Allocate format context
        int ret = avformat_alloc_output_context2(&formatCtx, nullptr, "mp4", filename.c_str());
        if (ret < 0) {
            char errbuf[AV_ERROR_MAX_STRING_SIZE];
            av_strerror(ret, errbuf, sizeof(errbuf));
            std::cerr << "Failed to allocate output context: " << errbuf << std::endl;
            return false;
        }

        // Create video stream
        videoStream = avformat_new_stream(formatCtx, nullptr);
        if (!videoStream) {
            std::cerr << "Failed to create video stream" << std::endl;
            return false;
        }
        videoStream->id = formatCtx->nb_streams - 1;

        // Configure stream codec parameters directly
        videoStream->codecpar->codec_type = AVMEDIA_TYPE_VIDEO;
        videoStream->codecpar->codec_id = AV_CODEC_ID_H264;
        videoStream->codecpar->width = videoWidth;
        videoStream->codecpar->height = videoHeight;
        videoStream->codecpar->format = AV_PIX_FMT_YUV420P;
        
        // Set time base for 30 fps
        videoStream->time_base = (AVRational){1, 90000}; // Use 90kHz timebase for H.264

        // Allocate codec context for later use
        const AVCodec* codec = avcodec_find_decoder(AV_CODEC_ID_H264);
        if (!codec) {
            std::cerr << "H.264 codec not found" << std::endl;
            return false;
        }

        codecCtx = avcodec_alloc_context3(codec);
        if (!codecCtx) {
            std::cerr << "Failed to allocate codec context" << std::endl;
            return false;
        }

        codecCtx->codec_id = AV_CODEC_ID_H264;
        codecCtx->codec_type = AVMEDIA_TYPE_VIDEO;
        codecCtx->width = videoWidth;
        codecCtx->height = videoHeight;
        codecCtx->time_base = (AVRational){1, 90000};
        codecCtx->framerate = (AVRational){videoFramerate, 1};
        codecCtx->pix_fmt = AV_PIX_FMT_YUV420P;

        // Configure codec if we already have SPS/PPS
        if (!spsData.empty() && !ppsData.empty()) {
            if (!configureCodec()) {
                return false;
            }
        }

        // Open output file
        if (!(formatCtx->oformat->flags & AVFMT_NOFILE)) {
            ret = avio_open(&formatCtx->pb, filename.c_str(), AVIO_FLAG_WRITE);
            if (ret < 0) {
                char errbuf[AV_ERROR_MAX_STRING_SIZE];
                av_strerror(ret, errbuf, sizeof(errbuf));
                std::cerr << "Failed to open output file: " << errbuf << std::endl;
                return false;
            }
        }

        // Write header with fragmented MP4 for crash resistance
        AVDictionary* opts = nullptr;
        // Use fragmented MP4 but with proper initialization
        // frag_keyframe: start new fragment at each keyframe
        // empty_moov: minimal moov at start (no duration info)
        // omit_tfhd_offset: simpler fragment format
        av_dict_set(&opts, "movflags", "frag_keyframe+empty_moov+omit_tfhd_offset+default_base_moof", 0);
        ret = avformat_write_header(formatCtx, &opts);
        av_dict_free(&opts);
        
        if (ret < 0) {
            char errbuf[AV_ERROR_MAX_STRING_SIZE];
            av_strerror(ret, errbuf, sizeof(errbuf));
            std::cerr << "Failed to write header: " << errbuf << std::endl;
            return false;
        }

        std::cout << "Started new recording: " << filename << std::endl;
        return true;
    }

    void closeOutputFile() {
        if (formatCtx) {
            if (formatCtx->pb) {
                av_write_trailer(formatCtx);
                avio_closep(&formatCtx->pb);
            }
            avformat_free_context(formatCtx);
            formatCtx = nullptr;
        }
        if (codecCtx) {
            avcodec_free_context(&codecCtx);
            codecCtx = nullptr;
        }
        videoStream = nullptr;
        codecConfigured = false;

        // End the current recording in the database
        if (db && currentRecordingId > 0 && !currentFilename.empty()) {
            // Get file size
            struct stat st;
            int64_t fileSize = 0;
            if (stat(currentFilename.c_str(), &st) == 0) {
                fileSize = st.st_size;
            }

            // Calculate end timestamp from first frame + elapsed time
            int64_t endTsMs = firstFrameTimestamp;
            if (firstFrameTimestamp > 0) {
                auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                    std::chrono::steady_clock::now() - startTime).count();
                endTsMs = firstFrameTimestamp + elapsed;
            }

            cameradb_recording_end(db, currentRecordingId, endTsMs, fileSize);
            currentRecordingId = 0;
        }
    }

    bool rotateFile() {
        // Close previous file (this also ends the recording in the DB)
        closeOutputFile();

        currentFilename = generateFilename();
        if (!openOutputFile(currentFilename)) {
            return false;
        }

        startTime = std::chrono::steady_clock::now();
        bytesWritten = 0;
        frameCount = 0;
        firstFrameTimestamp = 0;
        lastDts = 0;
        currentRecordingId = 0;
        return true;
    }

    // Set file mtime to match first frame timestamp for filesystem-level correlation
    void setFileMtime(int64_t timestampMs) {
        if (currentFilename.empty()) return;

        struct utimbuf times;
        times.actime = timestampMs / 1000;   // Access time (seconds)
        times.modtime = timestampMs / 1000;  // Modification time (seconds)
        utime(currentFilename.c_str(), &times);
    }

    // Convert Annex-B NAL units to AVCC format (length-prefixed)
    int convertAnnexBToAvcc(const uint8_t* annexb, int size, uint8_t* avcc) {
        int outPos = 0;
        int i = 0;

        while (i < size) {
            // Find start code
            int startCodeLen = 0;
            if (i + 2 < size && annexb[i] == 0 && annexb[i+1] == 0 && annexb[i+2] == 1) {
                startCodeLen = 3;
            } else if (i + 3 < size && annexb[i] == 0 && annexb[i+1] == 0 && annexb[i+2] == 0 && annexb[i+3] == 1) {
                startCodeLen = 4;
            }

            if (startCodeLen == 0) {
                i++;
                continue;
            }

            i += startCodeLen;
            int nalStart = i;

            // Find next start code to determine NAL size
            int nalEnd = size;
            for (int j = i; j < size - 2; j++) {
                if ((j + 2 < size && annexb[j] == 0 && annexb[j+1] == 0 && annexb[j+2] == 1) ||
                    (j + 3 < size && annexb[j] == 0 && annexb[j+1] == 0 && annexb[j+2] == 0 && annexb[j+3] == 1)) {
                    nalEnd = j;
                    break;
                }
            }

            int nalSize = nalEnd - nalStart;
            if (nalSize > 0) {
                // Skip SPS/PPS in stream (they're in extradata)
                uint8_t nalType = annexb[nalStart] & 0x1F;
                if (nalType != NAL_TYPE_SPS && nalType != NAL_TYPE_PPS) {
                    // Write length prefix (4 bytes, big-endian)
                    avcc[outPos++] = (nalSize >> 24) & 0xFF;
                    avcc[outPos++] = (nalSize >> 16) & 0xFF;
                    avcc[outPos++] = (nalSize >> 8) & 0xFF;
                    avcc[outPos++] = nalSize & 0xFF;
                    
                    // Copy NAL data
                    memcpy(avcc + outPos, annexb + nalStart, nalSize);
                    outPos += nalSize;
                }
            }

            i = nalEnd;
        }

        return outPos;
    }

public:
    Recorder(const std::string& dir)
        : outputDir(dir), buffer(nullptr), running(false), consumerInitialized(false),
          formatCtx(nullptr), videoStream(nullptr), codecCtx(nullptr), packet(nullptr),
          bytesWritten(0), frameCount(0), firstFrameTimestamp(0), lastDts(0), detectedFps(30), codecConfigured(false),
          videoWidth(1920), videoHeight(1080), videoFramerate(30),
          db(nullptr), currentRecordingId(0), dbEnabled(true) {
        memset(&consumer, 0, sizeof(consumer));
    }

    ~Recorder() {
        close();
    }

    void setDbEnabled(bool enabled) {
        dbEnabled = enabled;
    }

    bool init() {
        // Create output directory
        mkdir(outputDir.c_str(), 0755);

        // Initialize consumer
        if (video_shm_consumer_init(&consumer) != 0) {
            std::cerr << "Failed to initialize consumer" << std::endl;
            return false;
        }
        consumerInitialized = true;

        // Allocate buffer (double size for format conversion)
        buffer = (uint8_t*)malloc(VIDEO_SHM_MAX_FRAME_SIZE * 2);
        if (!buffer) {
            std::cerr << "Failed to allocate buffer" << std::endl;
            if (consumerInitialized) {
                video_shm_consumer_destroy(&consumer);
                consumerInitialized = false;
            }
            return false;
        }

        // Allocate packet
        packet = av_packet_alloc();
        if (!packet) {
            std::cerr << "Failed to allocate packet" << std::endl;
            free(buffer);
            buffer = nullptr;
            if (consumerInitialized) {
                video_shm_consumer_destroy(&consumer);
                consumerInitialized = false;
            }
            return false;
        }

        // Initialize database
        if (dbEnabled) {
            if (cameradb_open(&db) == 0) {
                std::cout << "Database logging enabled" << std::endl;
            } else {
                std::cerr << "WARNING: Failed to open database, logging disabled" << std::endl;
                db = nullptr;
            }
        }

        running = true;
        return true;
    }

    void close() {
        running = false;
        closeOutputFile();

        if (db) {
            cameradb_close(db);
            db = nullptr;
        }

        if (packet) {
            av_packet_free(&packet);
            packet = nullptr;
        }
        if (buffer) {
            free(buffer);
            buffer = nullptr;
        }
        if (consumerInitialized) {
            video_shm_consumer_destroy(&consumer);
            consumerInitialized = false;
        }
    }

    void run() {
        video_frame_meta_t meta;
        std::cout << "Recorder started. Waiting for SPS/PPS and keyframe..." << std::endl;

        bool fileCreated = false;
        uint8_t* avccBuffer = (uint8_t*)malloc(VIDEO_SHM_MAX_FRAME_SIZE * 2);
        if (!avccBuffer) {
            std::cerr << "Failed to allocate AVCC buffer" << std::endl;
            return;
        }
        
        while (running) {
            // Wait for frame (100ms timeout)
            int frameSize = video_shm_consumer_wait(&consumer, buffer, &meta, 100);

            if (frameSize < 0) {
                std::cerr << "Error reading frame" << std::endl;
                continue;
            }

            if (frameSize == 0) {
                continue; // Timeout
            }

            // Extract SPS/PPS if we don't have them yet
            if (spsData.empty() || ppsData.empty()) {
                extractNALUnits(buffer, frameSize);
            }

            // Wait for keyframe and codec configuration before starting file
            if (!fileCreated) {
                if (meta.is_keyframe != 1 || spsData.empty() || ppsData.empty()) {
                    continue;
                }
                
                // Capture FPS from frame metadata and update configuration
                if (meta.fps > 0) {
                    detectedFps = meta.fps;
                    videoFramerate = meta.fps;
                    std::cout << "Detected FPS: " << static_cast<int>(detectedFps) << std::endl;
                }
                
                if (!rotateFile()) {
                    std::cerr << "Error creating initial file" << std::endl;
                    free(avccBuffer);
                    return;
                }
                fileCreated = true;
                std::cout << "Recording started: " << currentFilename << std::endl;
            }

            // Configure codec with SPS/PPS if not already done
            if (!codecConfigured && !spsData.empty() && !ppsData.empty()) {
                if (!configureCodec()) {
                    std::cerr << "Failed to configure codec" << std::endl;
                    free(avccBuffer);
                    return;
                }
            }

            // Check rotation limits
            auto elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - startTime).count();

            if (bytesWritten >= MAX_FILE_SIZE || elapsed >= MAX_DURATION_MS) {
                if (meta.is_keyframe == 1) {
                    std::cout << "Rotating file: bytes=" << bytesWritten 
                              << ", elapsed=" << elapsed << "ms, frames=" << frameCount << std::endl;
                    if (!rotateFile()) {
                        std::cerr << "Error rotating file" << std::endl;
                        free(avccBuffer);
                        return;
                    }
                    std::cout << "Started new file: " << currentFilename << std::endl;
                }
            }

            // Convert Annex-B to AVCC format
            int avccSize = convertAnnexBToAvcc(buffer, frameSize, avccBuffer);
            if (avccSize <= 0) {
                continue; // Skip frames with no valid NAL units
            }

            // Store first frame timestamp for relative timing
            if (firstFrameTimestamp == 0) {
                firstFrameTimestamp = meta.timestamp_ms;

                // Register recording in database with first frame timestamp
                if (db && currentRecordingId == 0) {
                    currentRecordingId = cameradb_recording_start(db, currentFilename.c_str(), firstFrameTimestamp);
                    if (currentRecordingId > 0) {
                        // Set file mtime to match first frame timestamp
                        setFileMtime(firstFrameTimestamp);
                    }
                }
            }

            // Prepare packet with proper buffer allocation
            av_packet_unref(packet);
            if (av_new_packet(packet, avccSize) < 0) {
                std::cerr << "Failed to allocate packet buffer" << std::endl;
                continue;
            }
            
            // Copy AVCC data to packet
            memcpy(packet->data, avccBuffer, avccSize);
            packet->stream_index = videoStream->index;
            
            // Calculate timestamps using actual frame timestamp (convert ms to 90kHz timebase)
            // PTS in timebase units = (timestamp_ms - firstFrameTimestamp) * 90
            int64_t relativeTimeMs = meta.timestamp_ms - firstFrameTimestamp;
            int64_t pts = relativeTimeMs * 90;
            
            // For H.264 streams without B-frames (typical for camera), DTS = PTS
            // Ensure monotonically increasing DTS to avoid decoder errors
            int64_t dts = pts;
            if (dts <= lastDts) {
                // Ensure strictly monotonic increase
                dts = lastDts + 1;
                // Also adjust PTS to maintain PTS >= DTS invariant
                if (pts < dts) {
                    pts = dts;
                }
            }
            
            packet->pts = pts;
            packet->dts = dts;
            lastDts = dts;
            
            if (meta.is_keyframe == 1) {
                packet->flags |= AV_PKT_FLAG_KEY;
            }

            // Write packet
            int ret = av_interleaved_write_frame(formatCtx, packet);
            if (ret < 0) {
                char errbuf[AV_ERROR_MAX_STRING_SIZE];
                av_strerror(ret, errbuf, sizeof(errbuf));
                std::cerr << "Error writing frame: " << errbuf << std::endl;
            }

            bytesWritten += avccSize;
            frameCount++;
            
            // For fragmented MP4, flush every ~2 seconds (60 frames @ 30fps)
            // to update duration metadata for real-time viewing
            if (frameCount % 60 == 0) {
                avio_flush(formatCtx->pb);
            }
        }

        free(avccBuffer);
    }

    void stop() {
        running.store(false);
    }
};

static Recorder* g_recorder = nullptr;

void signalHandler(int sig) {
    if (g_recorder) {
        std::cout << "\nShutting down..." << std::endl;
        g_recorder->stop();
    }
}

int main(int argc, char* argv[]) {
    std::string outputDir;
    
    // Simple argument parsing
    bool userSpecifiedDir = false;
    bool dbEnabled = true;
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-o") == 0 && i + 1 < argc) {
            outputDir = argv[++i];
            userSpecifiedDir = true;
        } else if (strcmp(argv[i], "--no-db") == 0) {
            dbEnabled = false;
        } else if (strcmp(argv[i], "--help") == 0 || strcmp(argv[i], "-h") == 0) {
            std::cout << "Usage: " << argv[0] << " [-o output_dir] [--no-db]" << std::endl;
            std::cout << "Options:" << std::endl;
            std::cout << "  -o dir    Output directory (default: /mnt/sd or /userdata/video)" << std::endl;
            std::cout << "  --no-db   Disable SQLite database logging" << std::endl;
            return 0;
        }
    }

    // Auto-select output directory if not specified
    if (!userSpecifiedDir) {
        struct stat st;
        if (stat("/mnt/sd", &st) == 0 && S_ISDIR(st.st_mode)) {
            outputDir = "/mnt/sd";
            std::cout << "Using SD card: /mnt/sd" << std::endl;
        } else {
            outputDir = "/userdata/video";
            std::cout << "SD card not found, using: /userdata/video" << std::endl;
            
            // Ensure /userdata exists
            mkdir("/userdata", 0755);
            // Create /userdata/video (ignore errors if already exists)
            mkdir("/userdata/video", 0755);
        }
    }

    Recorder recorder(outputDir);
    recorder.setDbEnabled(dbEnabled);
    g_recorder = &recorder;

    // Setup signal handlers
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);

    if (!recorder.init()) {
        std::cerr << "Failed to initialize recorder" << std::endl;
        return 1;
    }

    recorder.run();

    return 0;
}
