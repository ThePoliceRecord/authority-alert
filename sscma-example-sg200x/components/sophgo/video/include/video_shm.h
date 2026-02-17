/**
 * @file video_shm.h
 * @brief Shared memory IPC for video streaming
 * 
 * Zero-copy video frame distribution using POSIX shared memory and semaphores.
 * Designed for embedded SG200x platform with constrained resources.
 */

#ifndef VIDEO_SHM_H
#define VIDEO_SHM_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stdbool.h>
#include <semaphore.h>

/* Configuration */
#define VIDEO_SHM_BASE_NAME     "/video_stream"
#define VIDEO_SHM_SEM_WRITE_BASE "/video_sem_write"
#define VIDEO_SHM_SEM_READ_BASE  "/video_sem_read"

// Legacy name (deprecated, use channel-specific names)
#define VIDEO_SHM_NAME          "/video_stream"
#define VIDEO_SHM_SEM_WRITE     "/video_sem_write"
#define VIDEO_SHM_SEM_READ      "/video_sem_read"

#define VIDEO_SHM_RING_SIZE     30              /* 30 frames @ 30fps = 1 second buffer */
#define VIDEO_SHM_MAX_FRAME_SIZE (512 * 1024)   /* 512KB max per frame (H.264 @ 1080p) */
#define VIDEO_SHM_MAGIC         0x56494445      /* "VIDE" magic number */
#define VIDEO_SHM_VERSION       1

/* Frame metadata */
typedef struct {
    uint64_t timestamp_ms;      /* Capture timestamp in milliseconds */
    uint32_t size;              /* Frame data size in bytes */
    uint32_t sequence;          /* Monotonic sequence number */
    uint8_t  is_keyframe;       /* 1 if I-frame, 0 otherwise */
    uint8_t  codec;             /* 0=H.264, 1=H.265, 2=JPEG */
    uint16_t width;             /* Frame width */
    uint16_t height;            /* Frame height */
    uint8_t  fps;               /* Frames per second */
    uint8_t  frame_hash[32];    /* SHA256 hash of frame data for evidence integrity */
    uint8_t  reserved[5];       /* Padding to 64 bytes */
} video_frame_meta_t;

/* Ring buffer slot */
typedef struct {
    video_frame_meta_t meta;
    uint8_t data[VIDEO_SHM_MAX_FRAME_SIZE];
} video_frame_slot_t;

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
    video_frame_slot_t frames[VIDEO_SHM_RING_SIZE];
} video_shm_t;

/* Producer handle */
typedef struct {
    int shm_fd;
    video_shm_t* shm;
    sem_t* sem_write;
    sem_t* sem_read;
    uint32_t sequence;
    int channel_id;  /* Channel ID for multi-channel support */
    char shm_name[64];   /* Actual shared memory name */
    char sem_write_name[64];  /* Semaphore names */
    char sem_read_name[64];
} video_shm_producer_t;

/* Consumer handle */
typedef struct {
    int shm_fd;
    video_shm_t* shm;
    sem_t* sem_write;
    sem_t* sem_read;
    uint32_t last_sequence;     /* Track missed frames */
    uint32_t reader_id;
    int channel_id;  /* Channel ID for multi-channel support */
    char shm_name[64];
    char sem_write_name[64];
    char sem_read_name[64];
} video_shm_consumer_t;

/* Producer API */

/**
 * Initialize shared memory producer (camera-streamer) with channel ID
 * @param producer Producer handle
 * @param channel_id Channel identifier (0-2 for CH0, CH1, CH2)
 * @return 0 on success, -1 on error
 */
int video_shm_producer_init_channel(video_shm_producer_t* producer, int channel_id);

/**
 * Initialize shared memory producer (legacy, uses channel 0)
 * @return 0 on success, -1 on error
 */
int video_shm_producer_init(video_shm_producer_t* producer);

/**
 * Write a video frame to shared memory
 * @param producer Producer handle
 * @param data Frame data
 * @param size Frame size in bytes
 * @param meta Frame metadata
 * @return 0 on success, -1 on error
 */
int video_shm_producer_write(video_shm_producer_t* producer,
                              const uint8_t* data,
                              uint32_t size,
                              const video_frame_meta_t* meta);

/**
 * Cleanup producer resources
 */
void video_shm_producer_destroy(video_shm_producer_t* producer);

/* Consumer API */

/**
 * Initialize shared memory consumer with channel ID
 * @param consumer Consumer handle
 * @param channel_id Channel identifier (0-2)
 * @return 0 on success, -1 on error
 */
int video_shm_consumer_init_channel(video_shm_consumer_t* consumer, int channel_id);

/**
 * Initialize shared memory consumer (legacy, uses channel 0)
 * @return 0 on success, -1 on error
 */
int video_shm_consumer_init(video_shm_consumer_t* consumer);

/**
 * Read next available frame (non-blocking)
 * @param consumer Consumer handle
 * @param data Output buffer (must be >= VIDEO_SHM_MAX_FRAME_SIZE)
 * @param meta Output metadata
 * @return Frame size on success, 0 if no new frame, -1 on error
 */
int video_shm_consumer_read(video_shm_consumer_t* consumer,
                             uint8_t* data,
                             video_frame_meta_t* meta);

/**
 * Wait for next frame (blocking with timeout)
 * @param consumer Consumer handle
 * @param data Output buffer
 * @param meta Output metadata
 * @param timeout_ms Timeout in milliseconds (0 = infinite)
 * @return Frame size on success, 0 on timeout, -1 on error
 */
int video_shm_consumer_wait(video_shm_consumer_t* consumer,
                             uint8_t* data,
                             video_frame_meta_t* meta,
                             uint32_t timeout_ms);

/**
 * Get statistics
 * @param consumer Consumer handle
 * @param total_frames Output: total frames written
 * @param dropped_frames Output: frames dropped
 * @param missed_frames Output: frames missed by this consumer
 * @return 0 on success, -1 on error
 */
int video_shm_consumer_stats(video_shm_consumer_t* consumer,
                              uint32_t* total_frames,
                              uint32_t* dropped_frames,
                              uint32_t* missed_frames);

/**
 * Cleanup consumer resources
 */
void video_shm_consumer_destroy(video_shm_consumer_t* consumer);

#ifdef __cplusplus
}
#endif

#endif /* VIDEO_SHM_H */
