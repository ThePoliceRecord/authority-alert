/**
 * @file detection_ws_client.cpp
 * @brief WebSocket client for detection queue
 */

#include "detection_ws_client.hpp"

#include <cstdio>
#include <cstdlib>
#include <algorithm>
#include <sstream>
#include <iomanip>

// Include mongoose implementation
#include "mongoose.h"

// Use libjpeg for JPEG encoding (more robust than stb_image_write)
#include <jpeglib.h>
#include <jerror.h>

#define TAG "detection_ws_client"

// Custom libjpeg destination manager for in-memory encoding
struct MemoryDestMgr {
    struct jpeg_destination_mgr pub;
    std::vector<uint8_t>* buffer;
    JOCTET* work_buffer;
    size_t work_buffer_size;
};

static void mem_init_destination(j_compress_ptr cinfo) {
    MemoryDestMgr* dest = (MemoryDestMgr*)cinfo->dest;
    dest->pub.next_output_byte = dest->work_buffer;
    dest->pub.free_in_buffer = dest->work_buffer_size;
}

static boolean mem_empty_output_buffer(j_compress_ptr cinfo) {
    MemoryDestMgr* dest = (MemoryDestMgr*)cinfo->dest;
    // Append work buffer to output
    dest->buffer->insert(dest->buffer->end(),
                         dest->work_buffer,
                         dest->work_buffer + dest->work_buffer_size);
    dest->pub.next_output_byte = dest->work_buffer;
    dest->pub.free_in_buffer = dest->work_buffer_size;
    return TRUE;
}

static void mem_term_destination(j_compress_ptr cinfo) {
    MemoryDestMgr* dest = (MemoryDestMgr*)cinfo->dest;
    size_t bytes_written = dest->work_buffer_size - dest->pub.free_in_buffer;
    if (bytes_written > 0) {
        dest->buffer->insert(dest->buffer->end(),
                             dest->work_buffer,
                             dest->work_buffer + bytes_written);
    }
}

DetectionWsClient::DetectionWsClient(int port)
    : m_mgr(nullptr)
    , m_conn(nullptr)
    , m_port(port)
    , m_connected(false)
    , m_jpeg_quality(85)
    , m_sent_count(0)
    , m_dropped_count(0)
    , m_failed_count(0)
    , m_last_connect_attempt(0)
    , m_reconnect_delay_ms(INITIAL_RECONNECT_DELAY_MS)
{
    // Build WebSocket URL
    char url_buf[64];
    snprintf(url_buf, sizeof(url_buf), "ws://127.0.0.1:%d/ws/detection", port);
    m_url = url_buf;

    // Pre-allocate buffers (typical detection JPEGs are 50-100KB)
    m_jpeg_buffer.reserve(150 * 1024);
    m_base64_buffer.reserve(200 * 1024);  // Base64 is ~33% larger

    // Initialize mongoose event manager
    m_mgr = new struct mg_mgr;
    mg_mgr_init(m_mgr);

    fprintf(stderr, "[%s] ========================================\n", TAG);
    fprintf(stderr, "[%s] WebSocket Detection Client Starting\n", TAG);
    fprintf(stderr, "[%s] URL: %s\n", TAG, m_url.c_str());
    fprintf(stderr, "[%s] JPEG Quality: %d\n", TAG, m_jpeg_quality);
    fprintf(stderr, "[%s] ========================================\n", TAG);

    // Start initial connection attempt
    connect();
}

DetectionWsClient::~DetectionWsClient() {
    if (m_mgr) {
        mg_mgr_free(m_mgr);
        delete m_mgr;
        m_mgr = nullptr;
    }
    fprintf(stderr, "[%s] Client destroyed: sent=%u, dropped=%u, failed=%u\n",
            TAG, m_sent_count, m_dropped_count, m_failed_count);
}

void DetectionWsClient::connect() {
    if (m_conn != nullptr) {
        return;  // Already connected or connecting
    }

    fprintf(stderr, "[%s] Connecting to %s...\n", TAG, m_url.c_str());
    m_conn = mg_ws_connect(m_mgr, m_url.c_str(), eventHandler, this, NULL);

    if (m_conn == nullptr) {
        fprintf(stderr, "[%s] Failed to initiate connection\n", TAG);
    }

    m_last_connect_attempt = mg_millis();
}

void DetectionWsClient::poll(int timeout_ms) {
    if (m_mgr == nullptr) {
        return;
    }

    mg_mgr_poll(m_mgr, timeout_ms);

    // Handle reconnection with exponential backoff
    if (!m_connected.load() && m_conn == nullptr) {
        uint64_t now = mg_millis();
        if (now - m_last_connect_attempt >= m_reconnect_delay_ms) {
            connect();
            // Increase backoff for next attempt
            m_reconnect_delay_ms = std::min(m_reconnect_delay_ms * 2, MAX_RECONNECT_DELAY_MS);
        }
    }
}

void DetectionWsClient::eventHandler(struct mg_connection* c, int ev, void* ev_data) {
    DetectionWsClient* client = static_cast<DetectionWsClient*>(c->fn_data);
    if (client) {
        client->handleEvent(c, ev, ev_data);
    }
}

void DetectionWsClient::handleEvent(struct mg_connection* c, int ev, void* ev_data) {
    switch (ev) {
        case MG_EV_CONNECT: {
            fprintf(stderr, "[%s] TCP connected, initiating WebSocket handshake...\n", TAG);
            break;
        }

        case MG_EV_ERROR: {
            const char* err = static_cast<const char*>(ev_data);
            fprintf(stderr, "[%s] CONNECTION ERROR: %s\n", TAG, err ? err : "unknown");
            fprintf(stderr, "[%s] Will retry in %u ms\n", TAG, m_reconnect_delay_ms);
            m_connected.store(false);
            m_conn = nullptr;
            break;
        }

        case MG_EV_WS_OPEN: {
            fprintf(stderr, "[%s] *** WebSocket CONNECTED ***\n", TAG);
            fprintf(stderr, "[%s] Ready to send detections\n", TAG);
            m_connected.store(true);
            m_reconnect_delay_ms = INITIAL_RECONNECT_DELAY_MS;  // Reset backoff
            break;
        }

        case MG_EV_WS_MSG: {
            struct mg_ws_message* wm = static_cast<struct mg_ws_message*>(ev_data);
            fprintf(stderr, "[%s] Received server response: %.*s\n", TAG,
                    (int)std::min(wm->data.len, (size_t)200), wm->data.buf);
            break;
        }

        case MG_EV_CLOSE: {
            fprintf(stderr, "[%s] Connection CLOSED\n", TAG);
            fprintf(stderr, "[%s] Stats: sent=%u, dropped=%u, failed=%u\n",
                    TAG, m_sent_count, m_dropped_count, m_failed_count);
            m_connected.store(false);
            m_conn = nullptr;
            break;
        }

        case MG_EV_POLL: {
            // Silent - this fires every poll
            break;
        }

        default:
            // Log unexpected events for debugging
            if (ev != MG_EV_READ && ev != MG_EV_WRITE) {
                fprintf(stderr, "[%s] Event: %d\n", TAG, ev);
            }
            break;
    }
}

void DetectionWsClient::setJpegQuality(int quality) {
    m_jpeg_quality = std::max(1, std::min(100, quality));
}

size_t DetectionWsClient::encodeJpeg(const uint8_t* bgr_data, int width, int height) {
    m_jpeg_buffer.clear();

    // Initialize libjpeg compression
    struct jpeg_compress_struct cinfo;
    struct jpeg_error_mgr jerr;

    cinfo.err = jpeg_std_error(&jerr);
    jpeg_create_compress(&cinfo);

    // Set up custom memory destination
    const size_t WORK_BUFFER_SIZE = 64 * 1024;  // 64KB work buffer
    std::vector<JOCTET> work_buffer(WORK_BUFFER_SIZE);

    MemoryDestMgr dest_mgr;
    dest_mgr.pub.init_destination = mem_init_destination;
    dest_mgr.pub.empty_output_buffer = mem_empty_output_buffer;
    dest_mgr.pub.term_destination = mem_term_destination;
    dest_mgr.buffer = &m_jpeg_buffer;
    dest_mgr.work_buffer = work_buffer.data();
    dest_mgr.work_buffer_size = WORK_BUFFER_SIZE;
    cinfo.dest = (struct jpeg_destination_mgr*)&dest_mgr;

    // Set image parameters
    cinfo.image_width = width;
    cinfo.image_height = height;
    cinfo.input_components = 3;
    cinfo.in_color_space = JCS_RGB;

    jpeg_set_defaults(&cinfo);
    jpeg_set_quality(&cinfo, m_jpeg_quality, TRUE);

    // Start compression
    jpeg_start_compress(&cinfo, TRUE);

    // Allocate row buffer for BGR->RGB conversion
    std::vector<uint8_t> row_buffer(width * 3);
    JSAMPROW row_pointer[1];
    row_pointer[0] = row_buffer.data();

    // Write scanlines (converting BGR to RGB on the fly)
    while (cinfo.next_scanline < cinfo.image_height) {
        const uint8_t* src_row = bgr_data + cinfo.next_scanline * width * 3;
        // Convert BGR to RGB
        for (int x = 0; x < width; x++) {
            row_buffer[x * 3 + 0] = src_row[x * 3 + 2];  // R = B
            row_buffer[x * 3 + 1] = src_row[x * 3 + 1];  // G = G
            row_buffer[x * 3 + 2] = src_row[x * 3 + 0];  // B = R
        }
        jpeg_write_scanlines(&cinfo, row_pointer, 1);
    }

    jpeg_finish_compress(&cinfo);
    jpeg_destroy_compress(&cinfo);

    static bool first_log = true;
    if (first_log) {
        fprintf(stderr, "[%s] JPEG encoded (libjpeg): %zu bytes (quality=%d, %dx%d)\n",
                TAG, m_jpeg_buffer.size(), m_jpeg_quality, width, height);
        first_log = false;
    }

    return m_jpeg_buffer.size();
}

std::string DetectionWsClient::hashToHex(const uint8_t* hash, size_t len) {
    std::ostringstream oss;
    oss << std::hex << std::setfill('0');
    for (size_t i = 0; i < len; i++) {
        oss << std::setw(2) << static_cast<int>(hash[i]);
    }
    return oss.str();
}

std::string DetectionWsClient::buildJson(const std::vector<Detection>& detections,
                                          const FrameEvidence& evidence) {
    std::ostringstream json;
    json << std::fixed << std::setprecision(4);

    json << "{";
    json << "\"type\":\"detection\",";
    json << "\"timestamp_ms\":" << evidence.timestamp_ms << ",";

    // Frame hash (hex string)
    if (evidence.valid) {
        json << "\"frame_hash\":\"" << hashToHex(evidence.frame_hash, 32) << "\",";
    }

    // Base64 encoded JPEG image
    // Calculate required buffer size for base64
    size_t base64_len = 4 * ((m_jpeg_buffer.size() + 2) / 3) + 1;
    m_base64_buffer.resize(base64_len);

    size_t encoded_len = mg_base64_encode(
        m_jpeg_buffer.data(),
        m_jpeg_buffer.size(),
        m_base64_buffer.data(),
        m_base64_buffer.size()
    );
    m_base64_buffer[encoded_len] = '\0';

    json << "\"image_base64\":\"" << m_base64_buffer.data() << "\",";

    // Detections array
    json << "\"detections\":[";
    for (size_t i = 0; i < detections.size(); i++) {
        const auto& det = detections[i];
        if (i > 0) json << ",";
        json << "{";
        json << "\"class_id\":" << det.class_id << ",";
        json << "\"class_label\":\"" << det.class_label << "\",";
        json << "\"confidence\":" << det.confidence << ",";
        json << "\"bbox\":[" << det.x << "," << det.y << "," << det.w << "," << det.h << "]";
        json << "}";
    }
    json << "]";

    json << "}";

    return json.str();
}

bool DetectionWsClient::send(const uint8_t* bgr_data, int width, int height,
                              const std::vector<Detection>& detections,
                              const FrameEvidence& evidence) {
    if (detections.empty()) {
        return true;  // Nothing to send
    }

    // Check connection status
    if (!m_connected.load() || m_conn == nullptr) {
        m_dropped_count++;
        if (m_dropped_count == 1 || m_dropped_count % 10 == 0) {
            fprintf(stderr, "[%s] NOT CONNECTED - dropped %u detections (waiting for supervisor)\n",
                    TAG, m_dropped_count);
        }
        return false;
    }

    // Encode frame as JPEG
    size_t jpeg_size = encodeJpeg(bgr_data, width, height);
    if (jpeg_size == 0) {
        fprintf(stderr, "[%s] JPEG encoding failed for %dx%d frame\n", TAG, width, height);
        m_failed_count++;
        return false;
    }

    // Build JSON message
    std::string json = buildJson(detections, evidence);

    // Log what we're sending
    fprintf(stderr, "[%s] Sending detection: %zu detections, jpeg=%zu bytes, json=%zu bytes\n",
            TAG, detections.size(), jpeg_size, json.size());
    for (size_t i = 0; i < detections.size(); i++) {
        const auto& det = detections[i];
        fprintf(stderr, "[%s]   [%zu] class=%s (id=%d) conf=%.2f bbox=[%.3f,%.3f,%.3f,%.3f]\n",
                TAG, i, det.class_label.c_str(), det.class_id, det.confidence,
                det.x, det.y, det.w, det.h);
    }

    // Send over WebSocket
    size_t sent = mg_ws_send(m_conn, json.c_str(), json.size(), WEBSOCKET_OP_TEXT);
    if (sent == 0) {
        fprintf(stderr, "[%s] FAILED to send WebSocket message (sent=0)\n", TAG);
        m_failed_count++;
        return false;
    }

    m_sent_count++;
    fprintf(stderr, "[%s] Detection SENT successfully (seq=%u, bytes=%zu)\n",
            TAG, m_sent_count, sent);

    return true;
}
