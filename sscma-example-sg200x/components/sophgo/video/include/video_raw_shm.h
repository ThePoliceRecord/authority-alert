/**
 * @file video_raw_shm.h
 * @brief Shared memory IPC for raw RGB frames (ML inference)
 *
 * Zero-copy raw frame distribution using POSIX shared memory and semaphores.
 * Designed for ML inference pipelines on the SG200x platform.
 */

#ifndef VIDEO_RAW_SHM_H
#define VIDEO_RAW_SHM_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stdbool.h>
#include <semaphore.h>

/* Configuration */
#define VIDEO_RAW_SHM_BASE_NAME     "/video_raw"
#define VIDEO_RAW_SEM_WRITE_BASE    "/video_raw_sem_write"
#define VIDEO_RAW_SEM_READ_BASE     "/video_raw_sem_read"

#define VIDEO_RAW_WIDTH             640
#define VIDEO_RAW_HEIGHT            640
#define VIDEO_RAW_FRAME_SIZE        (VIDEO_RAW_WIDTH * VIDEO_RAW_HEIGHT * 3)  /* RGB888 = 1.2MB */
#define VIDEO_RAW_RING_SIZE         10   /* 10 frames buffer (~12MB total) */
#define VIDEO_RAW_SHM_MAGIC         0x52415746  /* "RAWF" magic number */
#define VIDEO_RAW_SHM_VERSION       1

/* Raw frame format types */
typedef enum {
    VIDEO_RAW_FORMAT_RGB888 = 0,
    VIDEO_RAW_FORMAT_BGR888 = 1,
    VIDEO_RAW_FORMAT_RGB888_PLANAR = 2,
} video_raw_format_t;

/* Frame metadata */
typedef struct {
    uint64_t timestamp_ms;      /* Capture timestamp in milliseconds */
    uint32_t size;              /* Frame data size in bytes */
    uint32_t sequence;          /* Monotonic sequence number */
    uint16_t width;             /* Frame width */
    uint16_t height;            /* Frame height */
    uint8_t  format;            /* video_raw_format_t */
    uint8_t  frame_hash[32];    /* SHA256 hash of frame data for evidence integrity */
    uint8_t  reserved[5];       /* Padding to 56 bytes */
} video_raw_meta_t;

/* Ring buffer slot */
typedef struct {
    video_raw_meta_t meta;
    uint8_t data[VIDEO_RAW_FRAME_SIZE];
} video_raw_slot_t;

/* Shared memory header */
typedef struct {
    uint32_t magic;             /* Magic number for validation */
    uint32_t version;           /* Protocol version */
    uint32_t write_idx;         /* Next write position (producer) */
    uint32_t read_idx;          /* Last read position (consumer hint) */
    uint32_t frame_count;       /* Total frames written (wraps at UINT32_MAX) */
    uint32_t dropped_frames;    /* Frames dropped due to overrun */
    uint32_t active_readers;    /* Number of active consumer processes */
    uint32_t reserved[9];       /* Padding to 64 bytes */
    video_raw_slot_t frames[VIDEO_RAW_RING_SIZE];
} video_raw_shm_t;

/* Producer handle */
typedef struct {
    int shm_fd;
    video_raw_shm_t* shm;
    sem_t* sem_write;
    sem_t* sem_read;
    uint32_t sequence;
    int channel_id;
    char shm_name[64];
    char sem_write_name[64];
    char sem_read_name[64];
} video_raw_producer_t;

/* Consumer handle */
typedef struct {
    int shm_fd;
    video_raw_shm_t* shm;
    sem_t* sem_write;
    sem_t* sem_read;
    uint32_t last_sequence;
    uint32_t reader_id;
    int channel_id;
    char shm_name[64];
    char sem_write_name[64];
    char sem_read_name[64];
} video_raw_consumer_t;

/* Producer API */

/**
 * Initialize raw frame shared memory producer with channel ID
 * @param producer Producer handle
 * @param channel_id Channel identifier (typically 0 for raw frames)
 * @return 0 on success, -1 on error
 */
int video_raw_producer_init_channel(video_raw_producer_t* producer, int channel_id);

/**
 * Initialize raw frame shared memory producer (defaults to channel 0)
 * @param producer Producer handle
 * @return 0 on success, -1 on error
 */
int video_raw_producer_init(video_raw_producer_t* producer);

/**
 * Write a raw frame to shared memory
 * @param producer Producer handle
 * @param data Frame data (RGB888)
 * @param size Frame size in bytes
 * @param meta Frame metadata
 * @return 0 on success, -1 on error
 */
int video_raw_producer_write(video_raw_producer_t* producer,
                              const uint8_t* data,
                              uint32_t size,
                              const video_raw_meta_t* meta);

/**
 * Cleanup producer resources
 */
void video_raw_producer_destroy(video_raw_producer_t* producer);

/* Consumer API */

/**
 * Initialize raw frame shared memory consumer with channel ID
 * @param consumer Consumer handle
 * @param channel_id Channel identifier
 * @return 0 on success, -1 on error
 */
int video_raw_consumer_init_channel(video_raw_consumer_t* consumer, int channel_id);

/**
 * Initialize raw frame shared memory consumer (defaults to channel 0)
 * @param consumer Consumer handle
 * @return 0 on success, -1 on error
 */
int video_raw_consumer_init(video_raw_consumer_t* consumer);

/**
 * Read next available frame (non-blocking)
 * @param consumer Consumer handle
 * @param data Output buffer (must be >= VIDEO_RAW_FRAME_SIZE)
 * @param meta Output metadata
 * @return Frame size on success, 0 if no new frame, -1 on error
 */
int video_raw_consumer_read(video_raw_consumer_t* consumer,
                             uint8_t* data,
                             video_raw_meta_t* meta);

/**
 * Wait for next frame (blocking with timeout)
 * @param consumer Consumer handle
 * @param data Output buffer
 * @param meta Output metadata
 * @param timeout_ms Timeout in milliseconds (0 = infinite)
 * @return Frame size on success, 0 on timeout, -1 on error
 */
int video_raw_consumer_wait(video_raw_consumer_t* consumer,
                             uint8_t* data,
                             video_raw_meta_t* meta,
                             uint32_t timeout_ms);

/**
 * Get statistics
 * @param consumer Consumer handle
 * @param total_frames Output: total frames written
 * @param dropped_frames Output: frames dropped
 * @param missed_frames Output: frames missed by this consumer
 * @return 0 on success, -1 on error
 */
int video_raw_consumer_stats(video_raw_consumer_t* consumer,
                              uint32_t* total_frames,
                              uint32_t* dropped_frames,
                              uint32_t* missed_frames);

/**
 * Cleanup consumer resources
 */
void video_raw_consumer_destroy(video_raw_consumer_t* consumer);

#ifdef __cplusplus
}
#endif

#endif /* VIDEO_RAW_SHM_H */
