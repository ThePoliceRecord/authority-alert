/**
 * @file cameradb.h
 * @brief SQLite database for camera events (recordings and detections)
 *
 * Provides a shared database for correlating recordings and detections
 * using timestamps from camera-streamer. Uses SQLite WAL mode for
 * concurrent access from multiple processes.
 */

#ifndef CAMERADB_H
#define CAMERADB_H

#ifdef __cplusplus
extern "C" {
#endif

#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

#define CAMERADB_PATH "/userdata/cameradb.sqlite"

/* Schema version - increment when schema changes */
#define CAMERADB_SCHEMA_VERSION 3

/* Opaque database handle */
typedef struct cameradb cameradb_t;

/* Recording info */
typedef struct {
    int64_t id;
    char* path;              /* Owned by caller after cameradb_get_* */
    int64_t start_ts_ms;     /* First frame capture timestamp (ms since epoch) */
    int64_t end_ts_ms;       /* Last frame timestamp (0 if in progress) */
    int64_t duration_ms;     /* Computed: end_ts - start_ts */
    int64_t file_size;       /* Bytes */
    char* file_hash;         /* SHA-256 hex string (owned by caller) */
    bool uploaded;           /* 0=pending, 1=uploaded */
    bool file_missing;       /* 1 if file no longer exists */
    int64_t created_at;      /* Row creation timestamp */
} cameradb_recording_t;

/* Detection info */
typedef struct {
    int64_t id;
    int64_t frame_ts_ms;     /* Frame capture timestamp (ms since epoch) */
    int class_id;            /* COCO class ID */
    char* class_label;       /* "person", "car", etc. (owned by caller) */
    float confidence;        /* 0.0-1.0 */
    float bbox_x;            /* Normalized 0-1 */
    float bbox_y;
    float bbox_w;
    float bbox_h;
    char* image_path;        /* JPEG snapshot path (owned by caller, may be NULL) */
    int64_t recording_id;    /* FK to recordings (0 if not linked) */
    bool uploaded;           /* 0=pending, 1=uploaded */
    bool orphaned;           /* 1 if image file is missing */
    int64_t created_at;      /* Row creation timestamp */
} cameradb_detection_t;

/* Cleanup status */
typedef struct {
    int missing_recording_files;    /* Recordings where file doesn't exist */
    int orphaned_detections;        /* Detections with missing image files */
    int hash_mismatches;            /* Files that don't match stored hash */
    int64_t wasted_bytes;           /* Space used by orphaned image files */
    bool cleanup_recommended;       /* True if cleanup should be run */
} cameradb_cleanup_status_t;

/* Cleanup flags */
#define CLEANUP_REMOVE_MISSING_RECORDINGS  0x01  /* Remove DB records for missing files */
#define CLEANUP_REMOVE_ORPHANED_DETECTIONS 0x02  /* Remove detections with no image */
#define CLEANUP_REMOVE_ORPHANED_IMAGES     0x04  /* Delete image files not in DB */
#define CLEANUP_VERIFY_HASHES              0x08  /* Mark hash mismatches */

/* Cleanup result */
typedef struct {
    int64_t recordings_removed;
    int64_t detections_removed;
    int64_t images_deleted;
    int64_t bytes_freed;
} cameradb_cleanup_result_t;

/*
 * Database lifecycle
 */

/**
 * Open the database (creates if needed, runs migrations)
 * @param db Output: database handle
 * @return 0 on success, -1 on error
 */
int cameradb_open(cameradb_t** db);

/**
 * Close the database
 */
void cameradb_close(cameradb_t* db);

/*
 * Recording operations
 */

/**
 * Start a new recording
 * @param db Database handle
 * @param path File path for the recording
 * @param start_ts_ms First frame capture timestamp
 * @return Recording ID (>0) on success, -1 on error
 */
int64_t cameradb_recording_start(cameradb_t* db, const char* path, int64_t start_ts_ms);

/**
 * End a recording
 * @param db Database handle
 * @param id Recording ID
 * @param end_ts_ms Last frame timestamp
 * @param file_size File size in bytes
 * @return 0 on success, -1 on error
 */
int cameradb_recording_end(cameradb_t* db, int64_t id, int64_t end_ts_ms, int64_t file_size);

/**
 * End a recording with file hash for integrity verification
 * @param db Database handle
 * @param id Recording ID
 * @param end_ts_ms Last frame timestamp
 * @param file_size File size in bytes
 * @param file_hash SHA-256 hex string
 * @return 0 on success, -1 on error
 */
int cameradb_recording_end_with_hash(cameradb_t* db, int64_t id, int64_t end_ts_ms,
                                      int64_t file_size, const char* file_hash);

/**
 * Find the active recording for a given timestamp
 * @param db Database handle
 * @param timestamp_ms Frame timestamp
 * @return Recording ID (>0) if found, 0 if no active recording, -1 on error
 */
int64_t cameradb_recording_find_active(cameradb_t* db, int64_t timestamp_ms);

/**
 * Mark a recording as uploaded
 * @param db Database handle
 * @param id Recording ID
 * @return 0 on success, -1 on error
 */
int cameradb_recording_set_uploaded(cameradb_t* db, int64_t id);

/**
 * Mark a recording as missing (file doesn't exist)
 * @param db Database handle
 * @param id Recording ID
 * @return 0 on success, -1 on error
 */
int cameradb_recording_set_missing(cameradb_t* db, int64_t id);

/*
 * Detection operations
 */

/**
 * Insert a detection
 * @param db Database handle
 * @param det Detection info (id field is ignored)
 * @return Detection ID (>0) on success, -1 on error
 */
int64_t cameradb_detection_insert(cameradb_t* db, const cameradb_detection_t* det);

/**
 * Mark a detection as uploaded
 * @param db Database handle
 * @param id Detection ID
 * @return 0 on success, -1 on error
 */
int cameradb_detection_set_uploaded(cameradb_t* db, int64_t id);

/**
 * Mark a detection as orphaned (image file missing)
 * @param db Database handle
 * @param id Detection ID
 * @return 0 on success, -1 on error
 */
int cameradb_detection_set_orphaned(cameradb_t* db, int64_t id);

/*
 * Query operations
 */

/**
 * Get pending detections for upload
 * @param db Database handle
 * @param out Output: array of detections (caller must free with cameradb_free_detections)
 * @param count Output: number of detections
 * @param limit Maximum number to return (0 = no limit)
 * @return 0 on success, -1 on error
 */
int cameradb_get_pending_detections(cameradb_t* db, cameradb_detection_t** out,
                                     int* count, int limit);

/**
 * Get pending recordings for upload
 * @param db Database handle
 * @param out Output: array of recordings (caller must free with cameradb_free_recordings)
 * @param count Output: number of recordings
 * @param limit Maximum number to return (0 = no limit)
 * @return 0 on success, -1 on error
 */
int cameradb_get_pending_recordings(cameradb_t* db, cameradb_recording_t** out,
                                     int* count, int limit);

/**
 * Get a recording by ID
 * @param db Database handle
 * @param id Recording ID
 * @param out Output: recording info (caller must free strings and the struct)
 * @return 0 on success, -1 on error (including not found)
 */
int cameradb_get_recording(cameradb_t* db, int64_t id, cameradb_recording_t* out);

/**
 * Get a detection by ID
 * @param db Database handle
 * @param id Detection ID
 * @param out Output: detection info (caller must free strings and the struct)
 * @return 0 on success, -1 on error (including not found)
 */
int cameradb_get_detection(cameradb_t* db, int64_t id, cameradb_detection_t* out);

/*
 * Cleanup operations
 */

/**
 * Get cleanup status
 * @param db Database handle
 * @param status Output: cleanup status
 * @return 0 on success, -1 on error
 */
int cameradb_get_cleanup_status(cameradb_t* db, cameradb_cleanup_status_t* status);

/**
 * Run cleanup with specified options
 * @param db Database handle
 * @param flags Combination of CLEANUP_* flags
 * @param result Output: cleanup result (may be NULL)
 * @return 0 on success, -1 on error
 */
int cameradb_run_cleanup(cameradb_t* db, int flags, cameradb_cleanup_result_t* result);

/**
 * Verify file integrity (compares stored hash with actual file)
 * @param db Database handle
 * @param recording_id Recording ID to verify
 * @return 0=OK, 1=hash mismatch, 2=file missing, -1=error
 */
int cameradb_verify_recording(cameradb_t* db, int64_t recording_id);

/*
 * Memory management
 */

/**
 * Free an array of detections returned by cameradb_get_pending_detections
 */
void cameradb_free_detections(cameradb_detection_t* dets, int count);

/**
 * Free an array of recordings returned by cameradb_get_pending_recordings
 */
void cameradb_free_recordings(cameradb_recording_t* recs, int count);

/**
 * Free a single detection's allocated strings
 */
void cameradb_free_detection(cameradb_detection_t* det);

/**
 * Free a single recording's allocated strings
 */
void cameradb_free_recording(cameradb_recording_t* rec);

/*
 * Statistics
 */

/**
 * Get database statistics
 * @param db Database handle
 * @param total_recordings Output: total recording count
 * @param total_detections Output: total detection count
 * @param pending_uploads Output: pending detection uploads
 * @return 0 on success, -1 on error
 */
int cameradb_get_stats(cameradb_t* db, int64_t* total_recordings,
                        int64_t* total_detections, int64_t* pending_uploads);

#ifdef __cplusplus
}
#endif

#endif /* CAMERADB_H */
