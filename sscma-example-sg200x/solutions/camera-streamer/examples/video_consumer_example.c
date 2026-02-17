/**
 * @file video_consumer_example.c
 * @brief Example consumer application that reads video frames from shared memory
 * 
 * This demonstrates how other applications can access the video stream
 * from camera-streamer using zero-copy shared memory IPC.
 * 
 * Compile:
 *   gcc -o video_consumer video_consumer_example.c video_shm.c -lrt -lpthread
 * 
 * Usage:
 *   ./video_consumer [options]
 *     -s          Print statistics only (no frame data)
 *     -c COUNT    Exit after COUNT frames
 *     -t TIMEOUT  Timeout in milliseconds (0=infinite)
 *     -o FILE     Save frames to file (H.264 format)
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <signal.h>
#include <unistd.h>
#include <getopt.h>
#include "video_shm.h"

static volatile int g_running = 1;
static FILE* g_output_file = NULL;

static void signal_handler(int signo) {
    if (signo == SIGINT || signo == SIGTERM) {
        printf("\nReceived signal %d, shutting down...\n", signo);
        g_running = 0;
    }
}

static void print_frame_info(const video_frame_meta_t* meta, int frame_num) {
    const char* codec_str = (meta->codec == 0) ? "H.264" : 
                            (meta->codec == 1) ? "H.265" : "JPEG";
    const char* type_str = meta->is_keyframe ? "I-frame" : "P-frame";
    
    printf("[Frame %d] seq=%u, size=%u bytes, %s, %s, %ux%u@%dfps, ts=%lu ms\n",
           frame_num,
           meta->sequence,
           meta->size,
           codec_str,
           type_str,
           meta->width,
           meta->height,
           meta->fps,
           meta->timestamp_ms);
}

static void print_statistics(video_shm_consumer_t* consumer) {
    uint32_t total, dropped, missed;
    
    if (video_shm_consumer_stats(consumer, &total, &dropped, &missed) == 0) {
        printf("\n=== Statistics ===\n");
        printf("Total frames:   %u\n", total);
        printf("Dropped frames: %u (%.2f%%)\n", 
               dropped, total > 0 ? (100.0 * dropped / total) : 0.0);
        printf("Missed frames:  %u (%.2f%%)\n", 
               missed, total > 0 ? (100.0 * missed / total) : 0.0);
        printf("==================\n");
    }
}

int main(int argc, char* argv[]) {
    video_shm_consumer_t consumer;
    uint8_t* frame_buffer = NULL;
    video_frame_meta_t meta;
    int stats_only = 0;
    int max_frames = 0;
    int timeout_ms = 0;
    const char* output_file = NULL;
    int opt;
    int frame_count = 0;
    int ret = 0;

    // Parse command line options
    while ((opt = getopt(argc, argv, "sc:t:o:h")) != -1) {
        switch (opt) {
            case 's':
                stats_only = 1;
                break;
            case 'c':
                max_frames = atoi(optarg);
                break;
            case 't':
                timeout_ms = atoi(optarg);
                break;
            case 'o':
                output_file = optarg;
                break;
            case 'h':
            default:
                printf("Usage: %s [options]\n", argv[0]);
                printf("  -s          Print statistics only (no frame data)\n");
                printf("  -c COUNT    Exit after COUNT frames\n");
                printf("  -t TIMEOUT  Timeout in milliseconds (0=infinite)\n");
                printf("  -o FILE     Save frames to file (H.264 format)\n");
                printf("  -h          Show this help\n");
                return (opt == 'h') ? 0 : 1;
        }
    }

    printf("Video Consumer Example\n");
    printf("======================\n");
    printf("Connecting to shared memory: %s\n", VIDEO_SHM_NAME);
    if (max_frames > 0) {
        printf("Will exit after %d frames\n", max_frames);
    }
    if (timeout_ms > 0) {
        printf("Timeout: %d ms\n", timeout_ms);
    }
    printf("\n");

    // Setup signal handlers
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);

    // Initialize consumer
    if (video_shm_consumer_init(&consumer) != 0) {
        fprintf(stderr, "ERROR: Failed to initialize consumer\n");
        fprintf(stderr, "Is camera-streamer running?\n");
        return 1;
    }

    printf("Connected successfully!\n");
    printf("Press Ctrl+C to stop\n\n");

    // Allocate frame buffer
    frame_buffer = malloc(VIDEO_SHM_MAX_FRAME_SIZE);
    if (!frame_buffer) {
        fprintf(stderr, "ERROR: Failed to allocate frame buffer\n");
        video_shm_consumer_destroy(&consumer);
        return 1;
    }

    // Open output file if specified
    if (output_file) {
        g_output_file = fopen(output_file, "wb");
        if (!g_output_file) {
            fprintf(stderr, "ERROR: Failed to open output file: %s\n", output_file);
            free(frame_buffer);
            video_shm_consumer_destroy(&consumer);
            return 1;
        }
        printf("Saving frames to: %s\n\n", output_file);
    }

    // Main loop - read frames
    while (g_running) {
        int frame_size;

        // Wait for next frame
        if (timeout_ms > 0) {
            frame_size = video_shm_consumer_wait(&consumer, frame_buffer, &meta, timeout_ms);
            if (frame_size == 0) {
                printf("Timeout waiting for frame\n");
                continue;
            }
        } else {
            frame_size = video_shm_consumer_wait(&consumer, frame_buffer, &meta, 0);
        }

        if (frame_size < 0) {
            fprintf(stderr, "ERROR: Failed to read frame\n");
            ret = 1;
            break;
        }

        if (frame_size == 0) {
            continue;  /* No frame available */
        }

        frame_count++;

        // Print frame info (unless stats-only mode)
        if (!stats_only) {
            print_frame_info(&meta, frame_count);
        }

        // Save to file if requested
        if (g_output_file) {
            if (fwrite(frame_buffer, 1, frame_size, g_output_file) != (size_t)frame_size) {
                fprintf(stderr, "ERROR: Failed to write frame to file\n");
                ret = 1;
                break;
            }
        }

        // Check if we've reached max frames
        if (max_frames > 0 && frame_count >= max_frames) {
            printf("\nReached maximum frame count (%d)\n", max_frames);
            break;
        }

        // Print periodic statistics in stats-only mode
        if (stats_only && (frame_count % 30 == 0)) {
            print_statistics(&consumer);
        }
    }

    // Final statistics
    printf("\n");
    print_statistics(&consumer);
    printf("Total frames received: %d\n", frame_count);

    // Cleanup
    if (g_output_file) {
        fclose(g_output_file);
        printf("Saved to: %s\n", output_file);
    }

    free(frame_buffer);
    video_shm_consumer_destroy(&consumer);

    printf("Consumer shutdown complete\n");
    return ret;
}
