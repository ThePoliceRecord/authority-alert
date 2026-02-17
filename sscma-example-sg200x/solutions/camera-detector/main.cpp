/**
 * @file main.cpp
 * @brief ML Object Detection from Raw Frame Shared Memory
 *
 * Reads raw RGB frames from shared memory (produced by camera-streamer)
 * and runs object detection inference using sscma-micro.
 * Detections are sent to the local supervisor via WebSocket.
 */

#include <iostream>
#include <fstream>
#include <sstream>
#include <chrono>
#include <signal.h>
#include <cstdlib>
#include <cstring>
#include <iterator>
#include <vector>
#include <string>
#include <map>

#include <core/ma_core.h>
#include <core/engine/ma_engine_cvi.h>
#include <core/model/ma_model_factory.h>
#include <core/model/ma_model_detector.h>

#include "detection_ws_client.hpp"

extern "C" {
#include "video_raw_shm.h"
}

#define TAG "camera-detector"
#define MODEL_JSON_PATH "/userdata/Models/model.json"

// Dynamic class labels loaded from model.json
static std::vector<std::string> g_class_labels;

// Class labels for common detection models (COCO)
static const std::map<int, std::string> COCO_LABELS = {
    {0, "person"}, {1, "bicycle"}, {2, "car"}, {3, "motorcycle"}, {4, "airplane"},
    {5, "bus"}, {6, "train"}, {7, "truck"}, {8, "boat"}, {9, "traffic light"},
    {10, "fire hydrant"}, {11, "stop sign"}, {12, "parking meter"}, {13, "bench"},
    {14, "bird"}, {15, "cat"}, {16, "dog"}, {17, "horse"}, {18, "sheep"},
    {19, "cow"}, {20, "elephant"}, {21, "bear"}, {22, "zebra"}, {23, "giraffe"},
    {24, "backpack"}, {25, "umbrella"}, {26, "handbag"}, {27, "tie"}, {28, "suitcase"},
    {29, "frisbee"}, {30, "skis"}, {31, "snowboard"}, {32, "sports ball"}, {33, "kite"},
    {34, "baseball bat"}, {35, "baseball glove"}, {36, "skateboard"}, {37, "surfboard"},
    {38, "tennis racket"}, {39, "bottle"}, {40, "wine glass"}, {41, "cup"}, {42, "fork"},
    {43, "knife"}, {44, "spoon"}, {45, "bowl"}, {46, "banana"}, {47, "apple"},
    {48, "sandwich"}, {49, "orange"}, {50, "broccoli"}, {51, "carrot"}, {52, "hot dog"},
    {53, "pizza"}, {54, "donut"}, {55, "cake"}, {56, "chair"}, {57, "couch"},
    {58, "potted plant"}, {59, "bed"}, {60, "dining table"}, {61, "toilet"}, {62, "tv"},
    {63, "laptop"}, {64, "mouse"}, {65, "remote"}, {66, "keyboard"}, {67, "cell phone"},
    {68, "microwave"}, {69, "oven"}, {70, "toaster"}, {71, "sink"}, {72, "refrigerator"},
    {73, "book"}, {74, "clock"}, {75, "vase"}, {76, "scissors"}, {77, "teddy bear"},
    {78, "hair drier"}, {79, "toothbrush"}
};

// Load class labels from model.json if available
static std::vector<std::string> loadClassLabels(const char* json_path) {
    std::vector<std::string> labels;

    std::ifstream file(json_path);
    if (!file.is_open()) {
        return labels;  // Empty = use fallback
    }

    // Read entire file content
    std::string content((std::istreambuf_iterator<char>(file)),
                         std::istreambuf_iterator<char>());
    file.close();

    // Simple JSON parsing for "classes": ["label1", "label2", ...]
    size_t pos = content.find("\"classes\"");
    if (pos == std::string::npos) return labels;

    pos = content.find('[', pos);
    size_t end = content.find(']', pos);
    if (pos == std::string::npos || end == std::string::npos) return labels;

    std::string arr = content.substr(pos + 1, end - pos - 1);

    // Parse quoted strings from array
    size_t start = 0;
    while ((start = arr.find('"', start)) != std::string::npos) {
        size_t close = arr.find('"', start + 1);
        if (close == std::string::npos) break;
        labels.push_back(arr.substr(start + 1, close - start - 1));
        start = close + 1;
    }

    return labels;
}

static std::string getClassLabel(int class_id) {
    // If dynamic labels are loaded, use them exclusively
    if (!g_class_labels.empty()) {
        if (class_id >= 0 && class_id < (int)g_class_labels.size()) {
            return g_class_labels[class_id];
        }
        // Out of sync - return empty to signal skip
        return "";
    }

    // No dynamic labels - fall back to COCO labels
    auto it = COCO_LABELS.find(class_id);
    if (it != COCO_LABELS.end()) {
        return it->second;
    }
    return "class_" + std::to_string(class_id);
}

using namespace ma;

static volatile bool g_running = true;

static void signal_handler(int sig) {
    printf("\n%s: Received signal %d, shutting down...\n", TAG, sig);
    g_running = false;
}

int main(int argc, char** argv) {
    if (argc < 2) {
        printf("Usage: %s <model.cvimodel> [threshold] [--no-ws] [--ws-port PORT]\n", argv[0]);
        printf("Example: %s /userdata/Models/yolo11.cvimodel 0.5\n", argv[0]);
        printf("Options:\n");
        printf("  --no-ws       Disable sending detections via WebSocket\n");
        printf("  --ws-port N   WebSocket port (default: %d)\n", DetectionWsClient::DEFAULT_WS_PORT);
        return 1;
    }

    const char* model_path = argv[1];
    float threshold = (argc >= 3 && argv[2][0] != '-') ? atof(argv[2]) : 0.5f;

    // Check for command line flags
    bool enable_ws = true;
    int ws_port = DetectionWsClient::DEFAULT_WS_PORT;
    for (int i = 2; i < argc; i++) {
        if (strcmp(argv[i], "--no-ws") == 0) {
            enable_ws = false;
        } else if (strcmp(argv[i], "--ws-port") == 0 && i + 1 < argc) {
            ws_port = atoi(argv[++i]);
        }
    }

    // Setup signal handlers
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);

    printf("%s: Starting ML detector\n", TAG);
    printf("%s: Model: %s\n", TAG, model_path);
    printf("%s: Threshold: %.2f\n", TAG, threshold);

    // Initialize ML engine
    printf("%s: Initializing CVI engine...\n", TAG);
    auto* engine = new ma::engine::EngineCVI();
    ma_err_t ret = engine->init();
    if (ret != MA_OK) {
        fprintf(stderr, "%s: Failed to initialize engine (error %d)\n", TAG, ret);
        delete engine;
        return 1;
    }

    // Load model
    printf("%s: Loading model...\n", TAG);
    ret = engine->load(model_path);
    if (ret != MA_OK) {
        fprintf(stderr, "%s: Failed to load model: %s (error %d)\n", TAG, model_path, ret);
        delete engine;
        return 1;
    }

    // Create model instance
    ma::Model* model = ma::ModelFactory::create(engine);
    if (!model) {
        fprintf(stderr, "%s: Failed to create model - unsupported model type\n", TAG);
        delete engine;
        return 1;
    }

    printf("%s: Model type: %d\n", TAG, model->getType());

    // Verify input type
    if (model->getInputType() != MA_INPUT_TYPE_IMAGE) {
        fprintf(stderr, "%s: Model input type not supported (expected image)\n", TAG);
        ma::ModelFactory::remove(model);
        delete engine;
        return 1;
    }

    // Verify output type (must be bbox for detection)
    if (model->getOutputType() != MA_OUTPUT_TYPE_BBOX) {
        fprintf(stderr, "%s: Model output type not supported (expected bbox)\n", TAG);
        ma::ModelFactory::remove(model);
        delete engine;
        return 1;
    }

    // Load class labels from model.json
    g_class_labels = loadClassLabels(MODEL_JSON_PATH);
    if (g_class_labels.empty()) {
        printf("%s: Using default COCO labels\n", TAG);
    } else {
        printf("%s: Loaded %zu custom class labels from model.json\n", TAG, g_class_labels.size());
        for (size_t i = 0; i < g_class_labels.size() && i < 5; i++) {
            printf("%s:   [%zu] %s\n", TAG, i, g_class_labels[i].c_str());
        }
        if (g_class_labels.size() > 5) {
            printf("%s:   ... and %zu more\n", TAG, g_class_labels.size() - 5);
        }
    }

    // Get detector and set threshold
    ma::model::Detector* detector = static_cast<ma::model::Detector*>(model);
    detector->setConfig(MA_MODEL_CFG_OPT_THRESHOLD, threshold);

    // Get model input dimensions
    const ma_img_t* model_input = static_cast<const ma_img_t*>(model->getInput());
    int input_width = model_input->width;
    int input_height = model_input->height;
    printf("%s: Model input size: %dx%d\n", TAG, input_width, input_height);

    // Verify dimensions match expected raw frame size
    if (input_width != VIDEO_RAW_WIDTH || input_height != VIDEO_RAW_HEIGHT) {
        printf("%s: WARNING: Model expects %dx%d but raw frames are %dx%d\n",
               TAG, input_width, input_height, VIDEO_RAW_WIDTH, VIDEO_RAW_HEIGHT);
    }

    // Initialize detection WebSocket client
    DetectionWsClient* detection_client = nullptr;
    if (enable_ws) {
        detection_client = new DetectionWsClient(ws_port);
        printf("%s: Detection WebSocket client enabled (port %d)\n", TAG, ws_port);
    } else {
        printf("%s: Detection WebSocket disabled\n", TAG);
    }

    // Initialize shared memory consumer
    printf("%s: Connecting to raw frame shared memory...\n", TAG);
    video_raw_consumer_t consumer;
    if (video_raw_consumer_init(&consumer) != 0) {
        fprintf(stderr, "%s: Failed to connect to shared memory (is camera-streamer running?)\n", TAG);
        ma::ModelFactory::remove(model);
        delete engine;
        return 1;
    }

    // Allocate frame buffer
    uint8_t* frame_buffer = (uint8_t*)malloc(VIDEO_RAW_FRAME_SIZE);
    if (!frame_buffer) {
        fprintf(stderr, "%s: Failed to allocate frame buffer\n", TAG);
        video_raw_consumer_destroy(&consumer);
        ma::ModelFactory::remove(model);
        delete engine;
        return 1;
    }

    video_raw_meta_t meta;
    int frame_count = 0;
    long long total_inference_time = 0;

    printf("%s: Waiting for frames...\n", TAG);
    printf("%s: Press Ctrl+C to stop\n", TAG);

    while (g_running) {
        // Wait for next frame (100ms timeout)
        int size = video_raw_consumer_wait(&consumer, frame_buffer, &meta, 100);

        // Always poll WebSocket client to handle reconnection and I/O
        if (detection_client) {
            detection_client->poll(10);  // Give mongoose time to process events
        }

        if (size <= 0) {
            // Timeout or error, continue
            continue;
        }

        frame_count++;
        auto start_time = std::chrono::high_resolution_clock::now();

        // Create tensor from frame data
        ma_tensor_t tensor = {
            .size = (uint32_t)size,
            .is_physical = false,  // Virtual address from shared memory
            .is_variable = false,
        };
        tensor.data.data = frame_buffer;

        // Set input
        engine->setInput(0, tensor);

        // Run inference
        detector->run(nullptr);
        auto results = detector->getResults();

        auto end_time = std::chrono::high_resolution_clock::now();
        auto inference_ms = std::chrono::duration_cast<std::chrono::milliseconds>(end_time - start_time).count();
        total_inference_time += inference_ms;

        // Count detections
        size_t num_detections = std::distance(results.begin(), results.end());

        // Print results and send detections
        if (num_detections > 0) {
            printf("Frame %d (seq=%u): %zu detection(s) in %lld ms\n",
                   frame_count, meta.sequence, num_detections, inference_ms);

            // Build detection list
            std::vector<Detection> detections;
            for (const auto& bbox : results) {
                std::string label = getClassLabel(bbox.target);

                // Skip detections with unknown class (out of sync with model labels)
                if (label.empty()) {
                    continue;
                }

                printf("  - Class %d (%s): %.2f%% at [%.0f,%.0f,%.0f,%.0f]\n",
                       bbox.target,
                       label.c_str(),
                       bbox.score * 100.0f,
                       bbox.x * meta.width,
                       bbox.y * meta.height,
                       (bbox.x + bbox.w) * meta.width,
                       (bbox.y + bbox.h) * meta.height);

                Detection det;
                det.class_id = bbox.target;
                det.class_label = label;
                det.confidence = bbox.score;
                det.x = bbox.x;
                det.y = bbox.y;
                det.w = bbox.w;
                det.h = bbox.h;
                detections.push_back(det);
            }

            // Send detections via WebSocket with frame evidence for trusted timestamps
            if (detection_client && !detections.empty()) {
                // Build frame evidence from metadata (for trusted timestamp chain)
                FrameEvidence evidence;
                evidence.timestamp_ms = meta.timestamp_ms;
                std::memcpy(evidence.frame_hash, meta.frame_hash, 32);
                evidence.valid = (meta.timestamp_ms > 0);  // Only valid if NTP was synced at capture

                if (!detection_client->send(frame_buffer, meta.width, meta.height, detections, evidence)) {
                    // Log failure but don't stop processing
                    uint32_t total_issues = detection_client->getFailedCount() + detection_client->getDroppedCount();
                    if (total_issues % 10 == 1) {
                        fprintf(stderr, "%s: Detection send issue (failed: %u, dropped: %u)\n",
                                TAG, detection_client->getFailedCount(), detection_client->getDroppedCount());
                    }
                }
            }
        } else if (frame_count % 30 == 0) {
            // Print periodic status even when no detections
            double avg_time = static_cast<double>(total_inference_time) / frame_count;
            printf("Frame %d: No detections (avg inference: %.1f ms)\n", frame_count, avg_time);
        }
    }

    // Print summary statistics
    if (frame_count > 0) {
        double avg_time = static_cast<double>(total_inference_time) / frame_count;
        printf("\n%s: Processed %d frames, average inference time: %.1f ms\n",
               TAG, frame_count, avg_time);
    }

    // Print detection statistics
    if (detection_client) {
        printf("%s: Detections sent: %u, dropped: %u, failed: %u\n",
               TAG, detection_client->getSentCount(),
               detection_client->getDroppedCount(),
               detection_client->getFailedCount());
    }

    // Cleanup
    printf("%s: Cleaning up...\n", TAG);
    if (detection_client) {
        delete detection_client;
    }
    free(frame_buffer);
    video_raw_consumer_destroy(&consumer);
    ma::ModelFactory::remove(model);
    delete engine;

    printf("%s: Shutdown complete\n", TAG);
    return 0;
}
