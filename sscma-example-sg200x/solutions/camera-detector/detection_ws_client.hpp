/**
 * @file detection_ws_client.hpp
 * @brief WebSocket client for detection queue
 *
 * Sends detections over WebSocket to the local supervisor.
 * Replaces shared memory IPC with WebSocket for simpler, more reliable communication.
 */

#ifndef DETECTION_WS_CLIENT_HPP
#define DETECTION_WS_CLIENT_HPP

#include <string>
#include <vector>
#include <cstdint>
#include <cstring>
#include <atomic>

// Forward declarations for mongoose types
struct mg_mgr;
struct mg_connection;

/**
 * Frame evidence metadata for trusted timestamps
 */
struct FrameEvidence {
    uint64_t timestamp_ms;              // Capture timestamp from camera
    uint8_t frame_hash[32];             // SHA256 of raw frame data
    bool valid;                         // True if NTP was synced at capture time

    FrameEvidence() : timestamp_ms(0), valid(false) {
        memset(frame_hash, 0, sizeof(frame_hash));
    }
};

/**
 * Detection bounding box with metadata
 */
struct Detection {
    int class_id;
    std::string class_label;
    float confidence;
    float x, y, w, h;  // Normalized 0-1
};

/**
 * DetectionWsClient sends detections over WebSocket to the supervisor.
 * Uses mongoose for WebSocket client with auto-reconnect.
 */
class DetectionWsClient {
public:
    // Default WebSocket port for detection pipeline
    static const int DEFAULT_WS_PORT = 8089;

    /**
     * Constructor - initializes WebSocket client
     * @param port WebSocket port (default: 8089)
     */
    explicit DetectionWsClient(int port = DEFAULT_WS_PORT);
    ~DetectionWsClient();

    /**
     * Check if client is connected
     */
    bool isConnected() const { return m_connected.load(); }

    /**
     * Poll the event loop - must be called periodically
     * @param timeout_ms Poll timeout in milliseconds
     */
    void poll(int timeout_ms = 0);

    /**
     * Send detection with JPEG-encoded frame over WebSocket
     * @param bgr_data BGR888 frame data (converted to RGB internally)
     * @param width Frame width
     * @param height Frame height
     * @param detections Vector of detections to send
     * @param evidence Frame evidence metadata (timestamp, hash) for trusted timestamps
     * @return true if message queued for sending, false on error
     */
    bool send(const uint8_t* bgr_data, int width, int height,
              const std::vector<Detection>& detections,
              const FrameEvidence& evidence = FrameEvidence());

    /**
     * Set JPEG encoding quality
     * @param quality JPEG quality (1-100, default 85)
     */
    void setJpegQuality(int quality);

    /**
     * Get number of successful sends
     */
    uint32_t getSentCount() const { return m_sent_count; }

    /**
     * Get number of dropped sends (not connected)
     */
    uint32_t getDroppedCount() const { return m_dropped_count; }

    /**
     * Get number of failed sends (errors)
     */
    uint32_t getFailedCount() const { return m_failed_count; }

private:
    struct mg_mgr* m_mgr;
    struct mg_connection* m_conn;
    int m_port;
    std::string m_url;
    std::atomic<bool> m_connected;
    std::vector<uint8_t> m_jpeg_buffer;
    std::vector<char> m_base64_buffer;
    std::string m_json_buffer;
    int m_jpeg_quality;
    uint32_t m_sent_count;
    uint32_t m_dropped_count;
    uint32_t m_failed_count;

    // Reconnection state
    uint64_t m_last_connect_attempt;
    uint32_t m_reconnect_delay_ms;
    static const uint32_t INITIAL_RECONNECT_DELAY_MS = 1000;
    static const uint32_t MAX_RECONNECT_DELAY_MS = 30000;

    /**
     * Attempt to connect to WebSocket server
     */
    void connect();

    /**
     * Encode BGR frame to JPEG in memory
     * @param bgr_data BGR888 frame data
     * @param width Frame width
     * @param height Frame height
     * @return Size of encoded JPEG, 0 on failure
     */
    size_t encodeJpeg(const uint8_t* bgr_data, int width, int height);

    /**
     * Convert frame hash to hex string
     */
    static std::string hashToHex(const uint8_t* hash, size_t len);

    /**
     * Build JSON message for detection
     */
    std::string buildJson(const std::vector<Detection>& detections,
                          const FrameEvidence& evidence);

    /**
     * Mongoose event handler (static callback)
     */
    static void eventHandler(struct mg_connection* c, int ev, void* ev_data);

    /**
     * Instance event handler
     */
    void handleEvent(struct mg_connection* c, int ev, void* ev_data);
};

#endif // DETECTION_WS_CLIENT_HPP
