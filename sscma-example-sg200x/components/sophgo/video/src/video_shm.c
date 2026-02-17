/**
 * @file video_shm.c
 * @brief Shared memory IPC implementation for video streaming
 */

#include "video_shm.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <errno.h>
#include <time.h>

#define LOG_TAG "video_shm"
#define LOG_INFO(fmt, ...) fprintf(stderr, "[%s] INFO: " fmt "\n", LOG_TAG, ##__VA_ARGS__)
#define LOG_ERROR(fmt, ...) fprintf(stderr, "[%s] ERROR: " fmt "\n", LOG_TAG, ##__VA_ARGS__)
#define LOG_DEBUG(fmt, ...) fprintf(stderr, "[%s] DEBUG: " fmt "\n", LOG_TAG, ##__VA_ARGS__)

/* Helper: Get current timestamp in milliseconds */
static uint64_t get_timestamp_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}

/* Producer Implementation */

int video_shm_producer_init_channel(video_shm_producer_t* producer, int channel_id) {
    if (!producer) {
        LOG_ERROR("NULL producer handle");
        return -1;
    }

    memset(producer, 0, sizeof(video_shm_producer_t));
    producer->channel_id = channel_id;

    /* Construct channel-specific names */
    snprintf(producer->shm_name, sizeof(producer->shm_name), 
             "%s_ch%d", VIDEO_SHM_BASE_NAME, channel_id);
    snprintf(producer->sem_write_name, sizeof(producer->sem_write_name),
             "%s_ch%d", VIDEO_SHM_SEM_WRITE_BASE, channel_id);
    snprintf(producer->sem_read_name, sizeof(producer->sem_read_name),
             "%s_ch%d", VIDEO_SHM_SEM_READ_BASE, channel_id);

    /* Create/open shared memory */
    shm_unlink(producer->shm_name);  /* Clean up any stale instance */
    
    producer->shm_fd = shm_open(producer->shm_name, O_CREAT | O_RDWR, 0666);
    if (producer->shm_fd < 0) {
        LOG_ERROR("shm_open(%s) failed: %s", producer->shm_name, strerror(errno));
        return -1;
    }

    /* Set size */
    if (ftruncate(producer->shm_fd, sizeof(video_shm_t)) < 0) {
        LOG_ERROR("ftruncate failed: %s", strerror(errno));
        close(producer->shm_fd);
        shm_unlink(producer->shm_name);
        return -1;
    }

    /* Map memory */
    producer->shm = mmap(NULL, sizeof(video_shm_t), 
                         PROT_READ | PROT_WRITE, MAP_SHARED, 
                         producer->shm_fd, 0);
    if (producer->shm == MAP_FAILED) {
        LOG_ERROR("mmap failed: %s", strerror(errno));
        close(producer->shm_fd);
        shm_unlink(producer->shm_name);
        return -1;
    }

    /* Initialize header */
    memset(producer->shm, 0, sizeof(video_shm_t));
    producer->shm->magic = VIDEO_SHM_MAGIC;
    producer->shm->version = VIDEO_SHM_VERSION;

    /* Create semaphores */
    sem_unlink(producer->sem_write_name);
    sem_unlink(producer->sem_read_name);

    producer->sem_write = sem_open(producer->sem_write_name, O_CREAT, 0666, 1);
    if (producer->sem_write == SEM_FAILED) {
        LOG_ERROR("sem_open(%s) failed: %s", producer->sem_write_name, strerror(errno));
        munmap(producer->shm, sizeof(video_shm_t));
        close(producer->shm_fd);
        shm_unlink(producer->shm_name);
        return -1;
    }

    producer->sem_read = sem_open(producer->sem_read_name, O_CREAT, 0666, 0);
    if (producer->sem_read == SEM_FAILED) {
        LOG_ERROR("sem_open(%s) failed: %s", producer->sem_read_name, strerror(errno));
        sem_close(producer->sem_write);
        sem_unlink(producer->sem_write_name);
        munmap(producer->shm, sizeof(video_shm_t));
        close(producer->shm_fd);
        shm_unlink(producer->shm_name);
        return -1;
    }

    LOG_INFO("Producer initialized (CH%d): shm=%s, shm_size=%zu bytes, ring_size=%d frames",
             channel_id, producer->shm_name, sizeof(video_shm_t), VIDEO_SHM_RING_SIZE);

    return 0;
}

int video_shm_producer_init(video_shm_producer_t* producer) {
    /* Legacy function - defaults to channel 0 */
    return video_shm_producer_init_channel(producer, 0);
}

int video_shm_producer_write(video_shm_producer_t* producer,
                              const uint8_t* data,
                              uint32_t size,
                              const video_frame_meta_t* meta) {
    if (!producer || !producer->shm || !data || !meta) {
        LOG_ERROR("Invalid parameters");
        return -1;
    }

    if (size > VIDEO_SHM_MAX_FRAME_SIZE) {
        LOG_ERROR("Frame too large: %u > %u", size, VIDEO_SHM_MAX_FRAME_SIZE);
        return -1;
    }

    /* Acquire write lock (non-blocking to avoid stalling video pipeline) */
    if (sem_trywait(producer->sem_write) != 0) {
        /* Lock busy - drop frame to maintain real-time performance */
        producer->shm->dropped_frames++;
        LOG_DEBUG("Frame dropped (write lock busy), total=%u", 
                  producer->shm->dropped_frames);
        return 0;  /* Not an error - just dropped */
    }

    /* Get write slot */
    uint32_t idx = producer->shm->write_idx % VIDEO_SHM_RING_SIZE;
    video_frame_slot_t* slot = &producer->shm->frames[idx];

    /* Copy metadata and data */
    memcpy(&slot->meta, meta, sizeof(video_frame_meta_t));
    slot->meta.sequence = producer->sequence++;
    slot->meta.size = size;
    
    if (slot->meta.timestamp_ms == 0) {
        slot->meta.timestamp_ms = get_timestamp_ms();
    }

    memcpy(slot->data, data, size);

    /* Update ring buffer state */
    producer->shm->write_idx++;
    producer->shm->frame_count++;

    /* Release write lock and signal readers */
    sem_post(producer->sem_write);
    sem_post(producer->sem_read);

    // LOG_DEBUG("Frame written: seq=%u, size=%u, idx=%u, keyframe=%d",
    //           slot->meta.sequence, size, idx, slot->meta.is_keyframe);

    return 0;
}

void video_shm_producer_destroy(video_shm_producer_t* producer) {
    if (!producer) return;

    LOG_INFO("Destroying producer (CH%d): total_frames=%u, dropped=%u",
             producer->channel_id,
             producer->shm ? producer->shm->frame_count : 0,
             producer->shm ? producer->shm->dropped_frames : 0);

    if (producer->sem_read != SEM_FAILED) {
        sem_close(producer->sem_read);
        sem_unlink(producer->sem_read_name);
    }

    if (producer->sem_write != SEM_FAILED) {
        sem_close(producer->sem_write);
        sem_unlink(producer->sem_write_name);
    }

    if (producer->shm != MAP_FAILED) {
        munmap(producer->shm, sizeof(video_shm_t));
    }

    if (producer->shm_fd >= 0) {
        close(producer->shm_fd);
        shm_unlink(producer->shm_name);
    }

    memset(producer, 0, sizeof(video_shm_producer_t));
}

/* Consumer Implementation */

int video_shm_consumer_init_channel(video_shm_consumer_t* consumer, int channel_id) {
    if (!consumer) {
        LOG_ERROR("NULL consumer handle");
        return -1;
    }

    memset(consumer, 0, sizeof(video_shm_consumer_t));
    consumer->channel_id = channel_id;

    /* Construct channel-specific names */
    snprintf(consumer->shm_name, sizeof(consumer->shm_name), 
             "%s_ch%d", VIDEO_SHM_BASE_NAME, channel_id);
    snprintf(consumer->sem_write_name, sizeof(consumer->sem_write_name),
             "%s_ch%d", VIDEO_SHM_SEM_WRITE_BASE, channel_id);
    snprintf(consumer->sem_read_name, sizeof(consumer->sem_read_name),
             "%s_ch%d", VIDEO_SHM_SEM_READ_BASE, channel_id);

    /* Open existing shared memory */
    consumer->shm_fd = shm_open(consumer->shm_name, O_RDWR, 0666);
    if (consumer->shm_fd < 0) {
        LOG_ERROR("shm_open(%s) failed: %s (is producer running?)", 
                  consumer->shm_name, strerror(errno));
        return -1;
    }

    /* Map memory (read-write for atomic operations) */
    consumer->shm = mmap(NULL, sizeof(video_shm_t), 
                         PROT_READ | PROT_WRITE, MAP_SHARED, 
                         consumer->shm_fd, 0);
    if (consumer->shm == MAP_FAILED) {
        LOG_ERROR("mmap failed: %s", strerror(errno));
        close(consumer->shm_fd);
        return -1;
    }

    /* Validate header */
    if (consumer->shm->magic != VIDEO_SHM_MAGIC) {
        LOG_ERROR("Invalid magic: 0x%08X (expected 0x%08X)", 
                  consumer->shm->magic, VIDEO_SHM_MAGIC);
        munmap(consumer->shm, sizeof(video_shm_t));
        close(consumer->shm_fd);
        return -1;
    }

    if (consumer->shm->version != VIDEO_SHM_VERSION) {
        LOG_ERROR("Version mismatch: %u (expected %u)", 
                  consumer->shm->version, VIDEO_SHM_VERSION);
        munmap(consumer->shm, sizeof(video_shm_t));
        close(consumer->shm_fd);
        return -1;
    }

    /* Open semaphores */
    consumer->sem_write = sem_open(consumer->sem_write_name, 0);
    if (consumer->sem_write == SEM_FAILED) {
        LOG_ERROR("sem_open(%s) failed: %s", consumer->sem_write_name, strerror(errno));
        munmap(consumer->shm, sizeof(video_shm_t));
        close(consumer->shm_fd);
        return -1;
    }

    consumer->sem_read = sem_open(consumer->sem_read_name, 0);
    if (consumer->sem_read == SEM_FAILED) {
        LOG_ERROR("sem_open(%s) failed: %s", consumer->sem_read_name, strerror(errno));
        sem_close(consumer->sem_write);
        munmap(consumer->shm, sizeof(video_shm_t));
        close(consumer->shm_fd);
        return -1;
    }

    /* Start reading from current position */
    consumer->last_sequence = consumer->shm->frame_count;
    consumer->reader_id = getpid();

    __sync_fetch_and_add(&consumer->shm->active_readers, 1);

    LOG_INFO("Consumer initialized (CH%d): shm=%s, reader_id=%u, starting_seq=%u",
             channel_id, consumer->shm_name, consumer->reader_id, consumer->last_sequence);

    return 0;
}

int video_shm_consumer_init(video_shm_consumer_t* consumer) {
    /* Legacy function - defaults to channel 0 */
    return video_shm_consumer_init_channel(consumer, 0);
}

int video_shm_consumer_read(video_shm_consumer_t* consumer,
                             uint8_t* data,
                             video_frame_meta_t* meta) {
    if (!consumer || !consumer->shm || !data || !meta) {
        LOG_ERROR("Invalid parameters");
        return -1;
    }

    /* Check if new frame available */
    uint32_t current_count = consumer->shm->frame_count;
    if (current_count == consumer->last_sequence) {
        return 0;  /* No new frame */
    }

    /* Calculate read position */
    uint32_t idx = (consumer->shm->write_idx - 1) % VIDEO_SHM_RING_SIZE;
    const video_frame_slot_t* slot = &consumer->shm->frames[idx];

    /* Copy metadata and data */
    memcpy(meta, &slot->meta, sizeof(video_frame_meta_t));
    memcpy(data, slot->data, meta->size);

    /* Update tracking */
    uint32_t missed = current_count - consumer->last_sequence - 1;
    if (missed > 0) {
        LOG_DEBUG("Consumer %u missed %u frames", consumer->reader_id, missed);
    }
    consumer->last_sequence = current_count;

    // LOG_DEBUG("Frame read: seq=%u, size=%u, idx=%u", 
    //           meta->sequence, meta->size, idx);

    return meta->size;
}

int video_shm_consumer_wait(video_shm_consumer_t* consumer,
                             uint8_t* data,
                             video_frame_meta_t* meta,
                             uint32_t timeout_ms) {
    if (!consumer || !consumer->shm || !data || !meta) {
        LOG_ERROR("Invalid parameters");
        return -1;
    }

    /* Wait for new frame signal */
    if (timeout_ms == 0) {
        /* Infinite wait */
        if (sem_wait(consumer->sem_read) != 0) {
            LOG_ERROR("sem_wait failed: %s", strerror(errno));
            return -1;
        }
    } else {
        /* Timed wait */
        struct timespec ts;
        clock_gettime(CLOCK_REALTIME, &ts);
        ts.tv_sec += timeout_ms / 1000;
        ts.tv_nsec += (timeout_ms % 1000) * 1000000;
        if (ts.tv_nsec >= 1000000000) {
            ts.tv_sec++;
            ts.tv_nsec -= 1000000000;
        }

        if (sem_timedwait(consumer->sem_read, &ts) != 0) {
            if (errno == ETIMEDOUT) {
                return 0;  /* Timeout */
            }
            LOG_ERROR("sem_timedwait failed: %s", strerror(errno));
            return -1;
        }
    }

    /* Read the frame */
    return video_shm_consumer_read(consumer, data, meta);
}

int video_shm_consumer_stats(video_shm_consumer_t* consumer,
                              uint32_t* total_frames,
                              uint32_t* dropped_frames,
                              uint32_t* missed_frames) {
    if (!consumer || !consumer->shm) {
        LOG_ERROR("Invalid consumer handle");
        return -1;
    }

    if (total_frames) {
        *total_frames = consumer->shm->frame_count;
    }

    if (dropped_frames) {
        *dropped_frames = consumer->shm->dropped_frames;
    }

    if (missed_frames) {
        uint32_t current = consumer->shm->frame_count;
        *missed_frames = (current > consumer->last_sequence) ? 
                         (current - consumer->last_sequence) : 0;
    }

    return 0;
}

void video_shm_consumer_destroy(video_shm_consumer_t* consumer) {
    if (!consumer) return;

    if (consumer->shm) {
        __sync_fetch_and_sub(&consumer->shm->active_readers, 1);
        LOG_INFO("Consumer destroyed (CH%d): reader_id=%u, last_seq=%u",
                 consumer->channel_id, consumer->reader_id, consumer->last_sequence);
    }

    if (consumer->sem_read != SEM_FAILED) {
        sem_close(consumer->sem_read);
    }

    if (consumer->sem_write != SEM_FAILED) {
        sem_close(consumer->sem_write);
    }

    if (consumer->shm != MAP_FAILED) {
        munmap(consumer->shm, sizeof(video_shm_t));
    }

    if (consumer->shm_fd >= 0) {
        close(consumer->shm_fd);
    }

    memset(consumer, 0, sizeof(video_shm_consumer_t));
}
