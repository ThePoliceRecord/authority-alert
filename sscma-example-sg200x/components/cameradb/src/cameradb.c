/**
 * @file cameradb.c
 * @brief SQLite database implementation for camera events
 */

#include "cameradb.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sqlite3.h>
#include <sys/stat.h>
#include <time.h>
#include <unistd.h>
#include <dirent.h>

#define LOG_TAG "cameradb"
#define LOG_INFO(fmt, ...) fprintf(stderr, "[%s] INFO: " fmt "\n", LOG_TAG, ##__VA_ARGS__)
#define LOG_ERROR(fmt, ...) fprintf(stderr, "[%s] ERROR: " fmt "\n", LOG_TAG, ##__VA_ARGS__)
#define LOG_DEBUG(fmt, ...) fprintf(stderr, "[%s] DEBUG: " fmt "\n", LOG_TAG, ##__VA_ARGS__)

/* Maximum retries for SQLITE_BUSY */
#define MAX_BUSY_RETRIES 10
#define BUSY_RETRY_DELAY_MS 50

/* Internal database structure */
struct cameradb {
    sqlite3* handle;
    sqlite3_stmt* stmt_recording_start;
    sqlite3_stmt* stmt_recording_end;
    sqlite3_stmt* stmt_recording_end_hash;
    sqlite3_stmt* stmt_recording_find_active;
    sqlite3_stmt* stmt_recording_set_uploaded;
    sqlite3_stmt* stmt_recording_set_missing;
    sqlite3_stmt* stmt_detection_insert;
    sqlite3_stmt* stmt_detection_set_uploaded;
    sqlite3_stmt* stmt_detection_set_orphaned;
    sqlite3_stmt* stmt_get_pending_detections;
    sqlite3_stmt* stmt_get_pending_recordings;
};

/* Migration definitions */
typedef struct {
    int version;
    const char* description;
    const char* sql;
} migration_t;

static const migration_t migrations[] = {
    {
        1,
        "Initial schema",
        "CREATE TABLE IF NOT EXISTS schema_version ("
        "    version INTEGER PRIMARY KEY,"
        "    applied_at INTEGER NOT NULL,"
        "    description TEXT"
        ");"
        "CREATE TABLE IF NOT EXISTS recordings ("
        "    id INTEGER PRIMARY KEY AUTOINCREMENT,"
        "    path TEXT NOT NULL,"
        "    start_ts_ms INTEGER NOT NULL,"
        "    end_ts_ms INTEGER,"
        "    duration_ms INTEGER,"
        "    file_size INTEGER,"
        "    uploaded INTEGER DEFAULT 0,"
        "    created_at INTEGER NOT NULL"
        ");"
        "CREATE INDEX IF NOT EXISTS idx_recordings_start ON recordings(start_ts_ms);"
        "CREATE INDEX IF NOT EXISTS idx_recordings_uploaded ON recordings(uploaded);"
        "CREATE TABLE IF NOT EXISTS detections ("
        "    id INTEGER PRIMARY KEY AUTOINCREMENT,"
        "    frame_ts_ms INTEGER NOT NULL,"
        "    class_id INTEGER NOT NULL,"
        "    class_label TEXT,"
        "    confidence REAL NOT NULL,"
        "    bbox_x REAL NOT NULL,"
        "    bbox_y REAL NOT NULL,"
        "    bbox_w REAL NOT NULL,"
        "    bbox_h REAL NOT NULL,"
        "    image_path TEXT,"
        "    recording_id INTEGER,"
        "    uploaded INTEGER DEFAULT 0,"
        "    created_at INTEGER NOT NULL,"
        "    FOREIGN KEY (recording_id) REFERENCES recordings(id)"
        ");"
        "CREATE INDEX IF NOT EXISTS idx_detections_frame_ts ON detections(frame_ts_ms);"
        "CREATE INDEX IF NOT EXISTS idx_detections_recording ON detections(recording_id);"
        "CREATE INDEX IF NOT EXISTS idx_detections_uploaded ON detections(uploaded);"
    },
    {
        2,
        "Add file hash for integrity",
        "ALTER TABLE recordings ADD COLUMN file_hash TEXT;"
    },
    {
        3,
        "Add cleanup status columns",
        "ALTER TABLE recordings ADD COLUMN file_missing INTEGER DEFAULT 0;"
        "ALTER TABLE detections ADD COLUMN orphaned INTEGER DEFAULT 0;"
    }
};

static const int num_migrations = sizeof(migrations) / sizeof(migrations[0]);

/* Helper: Get current timestamp in milliseconds */
static int64_t get_current_time_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_REALTIME, &ts);
    return (int64_t)ts.tv_sec * 1000 + ts.tv_nsec / 1000000;
}

/* Helper: Sleep for milliseconds */
static void sleep_ms(int ms) {
    struct timespec ts;
    ts.tv_sec = ms / 1000;
    ts.tv_nsec = (ms % 1000) * 1000000;
    nanosleep(&ts, NULL);
}

/* Helper: Execute SQL with retry on SQLITE_BUSY */
static int exec_with_retry(sqlite3* db, const char* sql) {
    for (int i = 0; i < MAX_BUSY_RETRIES; i++) {
        int rc = sqlite3_exec(db, sql, NULL, NULL, NULL);
        if (rc == SQLITE_OK) {
            return 0;
        }
        if (rc != SQLITE_BUSY && rc != SQLITE_LOCKED) {
            LOG_ERROR("SQL exec failed: %s (rc=%d)", sqlite3_errmsg(db), rc);
            return -1;
        }
        sleep_ms(BUSY_RETRY_DELAY_MS);
    }
    LOG_ERROR("SQL exec failed after retries: database is locked");
    return -1;
}

/* Helper: Step statement with retry on SQLITE_BUSY */
static int step_with_retry(sqlite3_stmt* stmt) {
    for (int i = 0; i < MAX_BUSY_RETRIES; i++) {
        int rc = sqlite3_step(stmt);
        if (rc == SQLITE_DONE || rc == SQLITE_ROW) {
            return rc;
        }
        if (rc != SQLITE_BUSY && rc != SQLITE_LOCKED) {
            return rc;
        }
        sleep_ms(BUSY_RETRY_DELAY_MS);
    }
    return SQLITE_BUSY;
}

/* Get current schema version */
static int get_schema_version(sqlite3* db) {
    sqlite3_stmt* stmt;
    int version = 0;

    /* Check if schema_version table exists */
    const char* sql = "SELECT MAX(version) FROM schema_version";
    if (sqlite3_prepare_v2(db, sql, -1, &stmt, NULL) != SQLITE_OK) {
        /* Table doesn't exist, version is 0 */
        return 0;
    }

    if (sqlite3_step(stmt) == SQLITE_ROW) {
        version = sqlite3_column_int(stmt, 0);
    }

    sqlite3_finalize(stmt);
    return version;
}

/* Run database migrations */
static int run_migrations(sqlite3* db) {
    int current_version = get_schema_version(db);
    LOG_INFO("Current schema version: %d", current_version);

    for (int i = 0; i < num_migrations; i++) {
        if (migrations[i].version <= current_version) {
            continue;
        }

        LOG_INFO("Applying migration %d: %s", migrations[i].version, migrations[i].description);

        /* Begin transaction */
        if (exec_with_retry(db, "BEGIN TRANSACTION") != 0) {
            return -1;
        }

        /* Apply migration */
        char* errmsg = NULL;
        int rc = sqlite3_exec(db, migrations[i].sql, NULL, NULL, &errmsg);
        if (rc != SQLITE_OK) {
            LOG_ERROR("Migration %d failed: %s", migrations[i].version, errmsg ? errmsg : "unknown");
            sqlite3_free(errmsg);
            sqlite3_exec(db, "ROLLBACK", NULL, NULL, NULL);
            return -1;
        }

        /* Record migration */
        char version_sql[256];
        snprintf(version_sql, sizeof(version_sql),
                 "INSERT INTO schema_version (version, applied_at, description) VALUES (%d, %lld, '%s')",
                 migrations[i].version, (long long)get_current_time_ms(), migrations[i].description);
        if (exec_with_retry(db, version_sql) != 0) {
            sqlite3_exec(db, "ROLLBACK", NULL, NULL, NULL);
            return -1;
        }

        /* Commit transaction */
        if (exec_with_retry(db, "COMMIT") != 0) {
            sqlite3_exec(db, "ROLLBACK", NULL, NULL, NULL);
            return -1;
        }

        LOG_INFO("Migration %d applied successfully", migrations[i].version);
    }

    return 0;
}

/* Prepare all statements */
static int prepare_statements(cameradb_t* db) {
    int rc;

    /* Recording statements */
    rc = sqlite3_prepare_v2(db->handle,
        "INSERT INTO recordings (path, start_ts_ms, created_at) VALUES (?, ?, ?)",
        -1, &db->stmt_recording_start, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "UPDATE recordings SET end_ts_ms = ?, duration_ms = ? - start_ts_ms, file_size = ? WHERE id = ?",
        -1, &db->stmt_recording_end, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "UPDATE recordings SET end_ts_ms = ?, duration_ms = ? - start_ts_ms, file_size = ?, file_hash = ? WHERE id = ?",
        -1, &db->stmt_recording_end_hash, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "SELECT id FROM recordings WHERE start_ts_ms <= ? AND (end_ts_ms IS NULL OR end_ts_ms >= ?) ORDER BY start_ts_ms DESC LIMIT 1",
        -1, &db->stmt_recording_find_active, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "UPDATE recordings SET uploaded = 1 WHERE id = ?",
        -1, &db->stmt_recording_set_uploaded, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "UPDATE recordings SET file_missing = 1 WHERE id = ?",
        -1, &db->stmt_recording_set_missing, NULL);
    if (rc != SQLITE_OK) goto error;

    /* Detection statements */
    rc = sqlite3_prepare_v2(db->handle,
        "INSERT INTO detections (frame_ts_ms, class_id, class_label, confidence, "
        "bbox_x, bbox_y, bbox_w, bbox_h, image_path, recording_id, created_at) "
        "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
        -1, &db->stmt_detection_insert, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "UPDATE detections SET uploaded = 1 WHERE id = ?",
        -1, &db->stmt_detection_set_uploaded, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "UPDATE detections SET orphaned = 1 WHERE id = ?",
        -1, &db->stmt_detection_set_orphaned, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "SELECT id, frame_ts_ms, class_id, class_label, confidence, "
        "bbox_x, bbox_y, bbox_w, bbox_h, image_path, recording_id, created_at "
        "FROM detections WHERE uploaded = 0 AND orphaned = 0 ORDER BY frame_ts_ms ASC LIMIT ?",
        -1, &db->stmt_get_pending_detections, NULL);
    if (rc != SQLITE_OK) goto error;

    rc = sqlite3_prepare_v2(db->handle,
        "SELECT id, path, start_ts_ms, end_ts_ms, duration_ms, file_size, file_hash, file_missing, created_at "
        "FROM recordings WHERE uploaded = 0 AND file_missing = 0 AND end_ts_ms IS NOT NULL ORDER BY start_ts_ms ASC LIMIT ?",
        -1, &db->stmt_get_pending_recordings, NULL);
    if (rc != SQLITE_OK) goto error;

    return 0;

error:
    LOG_ERROR("Failed to prepare statement: %s", sqlite3_errmsg(db->handle));
    return -1;
}

/* Finalize all statements */
static void finalize_statements(cameradb_t* db) {
    if (db->stmt_recording_start) sqlite3_finalize(db->stmt_recording_start);
    if (db->stmt_recording_end) sqlite3_finalize(db->stmt_recording_end);
    if (db->stmt_recording_end_hash) sqlite3_finalize(db->stmt_recording_end_hash);
    if (db->stmt_recording_find_active) sqlite3_finalize(db->stmt_recording_find_active);
    if (db->stmt_recording_set_uploaded) sqlite3_finalize(db->stmt_recording_set_uploaded);
    if (db->stmt_recording_set_missing) sqlite3_finalize(db->stmt_recording_set_missing);
    if (db->stmt_detection_insert) sqlite3_finalize(db->stmt_detection_insert);
    if (db->stmt_detection_set_uploaded) sqlite3_finalize(db->stmt_detection_set_uploaded);
    if (db->stmt_detection_set_orphaned) sqlite3_finalize(db->stmt_detection_set_orphaned);
    if (db->stmt_get_pending_detections) sqlite3_finalize(db->stmt_get_pending_detections);
    if (db->stmt_get_pending_recordings) sqlite3_finalize(db->stmt_get_pending_recordings);
}

/*
 * Public API implementation
 */

int cameradb_open(cameradb_t** out) {
    if (!out) {
        return -1;
    }

    cameradb_t* db = calloc(1, sizeof(cameradb_t));
    if (!db) {
        LOG_ERROR("Failed to allocate database handle");
        return -1;
    }

    /* Ensure directory exists */
    const char* dir = "/userdata";
    struct stat st;
    if (stat(dir, &st) != 0) {
        if (mkdir(dir, 0755) != 0) {
            LOG_ERROR("Failed to create directory: %s", dir);
            free(db);
            return -1;
        }
    }

    /* Open database with thread-safe mode */
    int flags = SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE | SQLITE_OPEN_FULLMUTEX;
    int rc = sqlite3_open_v2(CAMERADB_PATH, &db->handle, flags, NULL);
    if (rc != SQLITE_OK) {
        LOG_ERROR("Failed to open database: %s", sqlite3_errmsg(db->handle));
        sqlite3_close(db->handle);
        free(db);
        return -1;
    }

    /* Enable WAL mode for concurrent access */
    if (exec_with_retry(db->handle, "PRAGMA journal_mode = WAL") != 0) {
        LOG_ERROR("Failed to enable WAL mode");
        sqlite3_close(db->handle);
        free(db);
        return -1;
    }

    /* Set synchronous mode for balance of safety/performance */
    if (exec_with_retry(db->handle, "PRAGMA synchronous = NORMAL") != 0) {
        LOG_ERROR("Failed to set synchronous mode");
        sqlite3_close(db->handle);
        free(db);
        return -1;
    }

    /* Set cache size (2MB) */
    if (exec_with_retry(db->handle, "PRAGMA cache_size = -2000") != 0) {
        LOG_ERROR("Failed to set cache size");
        sqlite3_close(db->handle);
        free(db);
        return -1;
    }

    /* Enable foreign keys */
    if (exec_with_retry(db->handle, "PRAGMA foreign_keys = ON") != 0) {
        LOG_ERROR("Failed to enable foreign keys");
        sqlite3_close(db->handle);
        free(db);
        return -1;
    }

    /* Run migrations */
    if (run_migrations(db->handle) != 0) {
        LOG_ERROR("Failed to run migrations");
        sqlite3_close(db->handle);
        free(db);
        return -1;
    }

    /* Prepare statements */
    if (prepare_statements(db) != 0) {
        sqlite3_close(db->handle);
        free(db);
        return -1;
    }

    LOG_INFO("Database opened: %s (schema version %d)", CAMERADB_PATH, CAMERADB_SCHEMA_VERSION);
    *out = db;
    return 0;
}

void cameradb_close(cameradb_t* db) {
    if (!db) return;

    finalize_statements(db);

    if (db->handle) {
        sqlite3_close(db->handle);
    }

    free(db);
    LOG_INFO("Database closed");
}

/*
 * Recording operations
 */

int64_t cameradb_recording_start(cameradb_t* db, const char* path, int64_t start_ts_ms) {
    if (!db || !path) return -1;

    sqlite3_stmt* stmt = db->stmt_recording_start;
    sqlite3_reset(stmt);
    sqlite3_bind_text(stmt, 1, path, -1, SQLITE_TRANSIENT);
    sqlite3_bind_int64(stmt, 2, start_ts_ms);
    sqlite3_bind_int64(stmt, 3, get_current_time_ms());

    int rc = step_with_retry(stmt);
    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to insert recording: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    int64_t id = sqlite3_last_insert_rowid(db->handle);
    LOG_DEBUG("Recording started: id=%lld, path=%s", (long long)id, path);
    return id;
}

int cameradb_recording_end(cameradb_t* db, int64_t id, int64_t end_ts_ms, int64_t file_size) {
    if (!db) return -1;

    sqlite3_stmt* stmt = db->stmt_recording_end;
    sqlite3_reset(stmt);
    sqlite3_bind_int64(stmt, 1, end_ts_ms);
    sqlite3_bind_int64(stmt, 2, end_ts_ms);
    sqlite3_bind_int64(stmt, 3, file_size);
    sqlite3_bind_int64(stmt, 4, id);

    int rc = step_with_retry(stmt);
    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to update recording: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    LOG_DEBUG("Recording ended: id=%lld, size=%lld", (long long)id, (long long)file_size);
    return 0;
}

int cameradb_recording_end_with_hash(cameradb_t* db, int64_t id, int64_t end_ts_ms,
                                      int64_t file_size, const char* file_hash) {
    if (!db) return -1;

    sqlite3_stmt* stmt = db->stmt_recording_end_hash;
    sqlite3_reset(stmt);
    sqlite3_bind_int64(stmt, 1, end_ts_ms);
    sqlite3_bind_int64(stmt, 2, end_ts_ms);
    sqlite3_bind_int64(stmt, 3, file_size);
    sqlite3_bind_text(stmt, 4, file_hash, -1, SQLITE_TRANSIENT);
    sqlite3_bind_int64(stmt, 5, id);

    int rc = step_with_retry(stmt);
    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to update recording with hash: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    LOG_DEBUG("Recording ended with hash: id=%lld, hash=%s", (long long)id, file_hash);
    return 0;
}

int64_t cameradb_recording_find_active(cameradb_t* db, int64_t timestamp_ms) {
    if (!db) return -1;

    sqlite3_stmt* stmt = db->stmt_recording_find_active;
    sqlite3_reset(stmt);
    sqlite3_bind_int64(stmt, 1, timestamp_ms);
    sqlite3_bind_int64(stmt, 2, timestamp_ms);

    int rc = step_with_retry(stmt);
    if (rc == SQLITE_ROW) {
        return sqlite3_column_int64(stmt, 0);
    }
    if (rc == SQLITE_DONE) {
        return 0;  /* No active recording */
    }

    LOG_ERROR("Failed to find active recording: %s", sqlite3_errmsg(db->handle));
    return -1;
}

int cameradb_recording_set_uploaded(cameradb_t* db, int64_t id) {
    if (!db) return -1;

    sqlite3_stmt* stmt = db->stmt_recording_set_uploaded;
    sqlite3_reset(stmt);
    sqlite3_bind_int64(stmt, 1, id);

    int rc = step_with_retry(stmt);
    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to mark recording uploaded: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    return 0;
}

int cameradb_recording_set_missing(cameradb_t* db, int64_t id) {
    if (!db) return -1;

    sqlite3_stmt* stmt = db->stmt_recording_set_missing;
    sqlite3_reset(stmt);
    sqlite3_bind_int64(stmt, 1, id);

    int rc = step_with_retry(stmt);
    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to mark recording missing: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    return 0;
}

/*
 * Detection operations
 */

int64_t cameradb_detection_insert(cameradb_t* db, const cameradb_detection_t* det) {
    if (!db || !det) return -1;

    sqlite3_stmt* stmt = db->stmt_detection_insert;
    sqlite3_reset(stmt);
    sqlite3_bind_int64(stmt, 1, det->frame_ts_ms);
    sqlite3_bind_int(stmt, 2, det->class_id);
    if (det->class_label) {
        sqlite3_bind_text(stmt, 3, det->class_label, -1, SQLITE_TRANSIENT);
    } else {
        sqlite3_bind_null(stmt, 3);
    }
    sqlite3_bind_double(stmt, 4, det->confidence);
    sqlite3_bind_double(stmt, 5, det->bbox_x);
    sqlite3_bind_double(stmt, 6, det->bbox_y);
    sqlite3_bind_double(stmt, 7, det->bbox_w);
    sqlite3_bind_double(stmt, 8, det->bbox_h);
    if (det->image_path) {
        sqlite3_bind_text(stmt, 9, det->image_path, -1, SQLITE_TRANSIENT);
    } else {
        sqlite3_bind_null(stmt, 9);
    }
    if (det->recording_id > 0) {
        sqlite3_bind_int64(stmt, 10, det->recording_id);
    } else {
        sqlite3_bind_null(stmt, 10);
    }
    sqlite3_bind_int64(stmt, 11, get_current_time_ms());

    int rc = step_with_retry(stmt);
    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to insert detection: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    return sqlite3_last_insert_rowid(db->handle);
}

int cameradb_detection_set_uploaded(cameradb_t* db, int64_t id) {
    if (!db) return -1;

    sqlite3_stmt* stmt = db->stmt_detection_set_uploaded;
    sqlite3_reset(stmt);
    sqlite3_bind_int64(stmt, 1, id);

    int rc = step_with_retry(stmt);
    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to mark detection uploaded: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    return 0;
}

int cameradb_detection_set_orphaned(cameradb_t* db, int64_t id) {
    if (!db) return -1;

    sqlite3_stmt* stmt = db->stmt_detection_set_orphaned;
    sqlite3_reset(stmt);
    sqlite3_bind_int64(stmt, 1, id);

    int rc = step_with_retry(stmt);
    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to mark detection orphaned: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    return 0;
}

/*
 * Query operations
 */

int cameradb_get_pending_detections(cameradb_t* db, cameradb_detection_t** out,
                                     int* count, int limit) {
    if (!db || !out || !count) return -1;

    if (limit <= 0) limit = 100;

    sqlite3_stmt* stmt = db->stmt_get_pending_detections;
    sqlite3_reset(stmt);
    sqlite3_bind_int(stmt, 1, limit);

    /* Count results first */
    int capacity = 16;
    int n = 0;
    cameradb_detection_t* dets = calloc(capacity, sizeof(cameradb_detection_t));
    if (!dets) {
        LOG_ERROR("Failed to allocate detection array");
        return -1;
    }

    int rc;
    while ((rc = step_with_retry(stmt)) == SQLITE_ROW) {
        if (n >= capacity) {
            capacity *= 2;
            cameradb_detection_t* new_dets = realloc(dets, capacity * sizeof(cameradb_detection_t));
            if (!new_dets) {
                LOG_ERROR("Failed to expand detection array");
                cameradb_free_detections(dets, n);
                return -1;
            }
            dets = new_dets;
        }

        cameradb_detection_t* d = &dets[n];
        memset(d, 0, sizeof(*d));

        d->id = sqlite3_column_int64(stmt, 0);
        d->frame_ts_ms = sqlite3_column_int64(stmt, 1);
        d->class_id = sqlite3_column_int(stmt, 2);
        const char* label = (const char*)sqlite3_column_text(stmt, 3);
        d->class_label = label ? strdup(label) : NULL;
        d->confidence = (float)sqlite3_column_double(stmt, 4);
        d->bbox_x = (float)sqlite3_column_double(stmt, 5);
        d->bbox_y = (float)sqlite3_column_double(stmt, 6);
        d->bbox_w = (float)sqlite3_column_double(stmt, 7);
        d->bbox_h = (float)sqlite3_column_double(stmt, 8);
        const char* path = (const char*)sqlite3_column_text(stmt, 9);
        d->image_path = path ? strdup(path) : NULL;
        d->recording_id = sqlite3_column_int64(stmt, 10);
        d->created_at = sqlite3_column_int64(stmt, 11);

        n++;
    }

    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to query detections: %s", sqlite3_errmsg(db->handle));
        cameradb_free_detections(dets, n);
        return -1;
    }

    *out = dets;
    *count = n;
    return 0;
}

int cameradb_get_pending_recordings(cameradb_t* db, cameradb_recording_t** out,
                                     int* count, int limit) {
    if (!db || !out || !count) return -1;

    if (limit <= 0) limit = 100;

    sqlite3_stmt* stmt = db->stmt_get_pending_recordings;
    sqlite3_reset(stmt);
    sqlite3_bind_int(stmt, 1, limit);

    int capacity = 16;
    int n = 0;
    cameradb_recording_t* recs = calloc(capacity, sizeof(cameradb_recording_t));
    if (!recs) {
        LOG_ERROR("Failed to allocate recording array");
        return -1;
    }

    int rc;
    while ((rc = step_with_retry(stmt)) == SQLITE_ROW) {
        if (n >= capacity) {
            capacity *= 2;
            cameradb_recording_t* new_recs = realloc(recs, capacity * sizeof(cameradb_recording_t));
            if (!new_recs) {
                LOG_ERROR("Failed to expand recording array");
                cameradb_free_recordings(recs, n);
                return -1;
            }
            recs = new_recs;
        }

        cameradb_recording_t* r = &recs[n];
        memset(r, 0, sizeof(*r));

        r->id = sqlite3_column_int64(stmt, 0);
        const char* path = (const char*)sqlite3_column_text(stmt, 1);
        r->path = path ? strdup(path) : NULL;
        r->start_ts_ms = sqlite3_column_int64(stmt, 2);
        r->end_ts_ms = sqlite3_column_int64(stmt, 3);
        r->duration_ms = sqlite3_column_int64(stmt, 4);
        r->file_size = sqlite3_column_int64(stmt, 5);
        const char* hash = (const char*)sqlite3_column_text(stmt, 6);
        r->file_hash = hash ? strdup(hash) : NULL;
        r->file_missing = sqlite3_column_int(stmt, 7) != 0;
        r->created_at = sqlite3_column_int64(stmt, 8);

        n++;
    }

    if (rc != SQLITE_DONE) {
        LOG_ERROR("Failed to query recordings: %s", sqlite3_errmsg(db->handle));
        cameradb_free_recordings(recs, n);
        return -1;
    }

    *out = recs;
    *count = n;
    return 0;
}

int cameradb_get_recording(cameradb_t* db, int64_t id, cameradb_recording_t* out) {
    if (!db || !out) return -1;

    sqlite3_stmt* stmt;
    int rc = sqlite3_prepare_v2(db->handle,
        "SELECT id, path, start_ts_ms, end_ts_ms, duration_ms, file_size, file_hash, uploaded, file_missing, created_at "
        "FROM recordings WHERE id = ?",
        -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        LOG_ERROR("Failed to prepare query: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    sqlite3_bind_int64(stmt, 1, id);

    rc = step_with_retry(stmt);
    if (rc != SQLITE_ROW) {
        sqlite3_finalize(stmt);
        return -1;
    }

    memset(out, 0, sizeof(*out));
    out->id = sqlite3_column_int64(stmt, 0);
    const char* path = (const char*)sqlite3_column_text(stmt, 1);
    out->path = path ? strdup(path) : NULL;
    out->start_ts_ms = sqlite3_column_int64(stmt, 2);
    out->end_ts_ms = sqlite3_column_int64(stmt, 3);
    out->duration_ms = sqlite3_column_int64(stmt, 4);
    out->file_size = sqlite3_column_int64(stmt, 5);
    const char* hash = (const char*)sqlite3_column_text(stmt, 6);
    out->file_hash = hash ? strdup(hash) : NULL;
    out->uploaded = sqlite3_column_int(stmt, 7) != 0;
    out->file_missing = sqlite3_column_int(stmt, 8) != 0;
    out->created_at = sqlite3_column_int64(stmt, 9);

    sqlite3_finalize(stmt);
    return 0;
}

int cameradb_get_detection(cameradb_t* db, int64_t id, cameradb_detection_t* out) {
    if (!db || !out) return -1;

    sqlite3_stmt* stmt;
    int rc = sqlite3_prepare_v2(db->handle,
        "SELECT id, frame_ts_ms, class_id, class_label, confidence, "
        "bbox_x, bbox_y, bbox_w, bbox_h, image_path, recording_id, uploaded, orphaned, created_at "
        "FROM detections WHERE id = ?",
        -1, &stmt, NULL);
    if (rc != SQLITE_OK) {
        LOG_ERROR("Failed to prepare query: %s", sqlite3_errmsg(db->handle));
        return -1;
    }

    sqlite3_bind_int64(stmt, 1, id);

    rc = step_with_retry(stmt);
    if (rc != SQLITE_ROW) {
        sqlite3_finalize(stmt);
        return -1;
    }

    memset(out, 0, sizeof(*out));
    out->id = sqlite3_column_int64(stmt, 0);
    out->frame_ts_ms = sqlite3_column_int64(stmt, 1);
    out->class_id = sqlite3_column_int(stmt, 2);
    const char* label = (const char*)sqlite3_column_text(stmt, 3);
    out->class_label = label ? strdup(label) : NULL;
    out->confidence = (float)sqlite3_column_double(stmt, 4);
    out->bbox_x = (float)sqlite3_column_double(stmt, 5);
    out->bbox_y = (float)sqlite3_column_double(stmt, 6);
    out->bbox_w = (float)sqlite3_column_double(stmt, 7);
    out->bbox_h = (float)sqlite3_column_double(stmt, 8);
    const char* path = (const char*)sqlite3_column_text(stmt, 9);
    out->image_path = path ? strdup(path) : NULL;
    out->recording_id = sqlite3_column_int64(stmt, 10);
    out->uploaded = sqlite3_column_int(stmt, 11) != 0;
    out->orphaned = sqlite3_column_int(stmt, 12) != 0;
    out->created_at = sqlite3_column_int64(stmt, 13);

    sqlite3_finalize(stmt);
    return 0;
}

/*
 * Cleanup operations
 */

int cameradb_get_cleanup_status(cameradb_t* db, cameradb_cleanup_status_t* status) {
    if (!db || !status) return -1;

    memset(status, 0, sizeof(*status));

    sqlite3_stmt* stmt;
    int rc;

    /* Count missing recording files */
    rc = sqlite3_prepare_v2(db->handle,
        "SELECT COUNT(*) FROM recordings WHERE file_missing = 1",
        -1, &stmt, NULL);
    if (rc == SQLITE_OK) {
        if (sqlite3_step(stmt) == SQLITE_ROW) {
            status->missing_recording_files = sqlite3_column_int(stmt, 0);
        }
        sqlite3_finalize(stmt);
    }

    /* Count orphaned detections */
    rc = sqlite3_prepare_v2(db->handle,
        "SELECT COUNT(*) FROM detections WHERE orphaned = 1",
        -1, &stmt, NULL);
    if (rc == SQLITE_OK) {
        if (sqlite3_step(stmt) == SQLITE_ROW) {
            status->orphaned_detections = sqlite3_column_int(stmt, 0);
        }
        sqlite3_finalize(stmt);
    }

    /* Recommend cleanup if significant issues exist */
    status->cleanup_recommended = (status->missing_recording_files > 10 ||
                                    status->orphaned_detections > 50 ||
                                    status->hash_mismatches > 0);

    return 0;
}

int cameradb_run_cleanup(cameradb_t* db, int flags, cameradb_cleanup_result_t* result) {
    if (!db) return -1;

    cameradb_cleanup_result_t local_result = {0};
    int rc;

    if (flags & CLEANUP_REMOVE_MISSING_RECORDINGS) {
        rc = sqlite3_exec(db->handle,
            "DELETE FROM recordings WHERE file_missing = 1",
            NULL, NULL, NULL);
        if (rc == SQLITE_OK) {
            local_result.recordings_removed = sqlite3_changes(db->handle);
        }
    }

    if (flags & CLEANUP_REMOVE_ORPHANED_DETECTIONS) {
        rc = sqlite3_exec(db->handle,
            "DELETE FROM detections WHERE orphaned = 1",
            NULL, NULL, NULL);
        if (rc == SQLITE_OK) {
            local_result.detections_removed = sqlite3_changes(db->handle);
        }
    }

    if (result) {
        *result = local_result;
    }

    LOG_INFO("Cleanup complete: %lld recordings, %lld detections removed",
             (long long)local_result.recordings_removed,
             (long long)local_result.detections_removed);

    return 0;
}

int cameradb_verify_recording(cameradb_t* db, int64_t recording_id) {
    if (!db) return -1;

    cameradb_recording_t rec;
    if (cameradb_get_recording(db, recording_id, &rec) != 0) {
        return -1;  /* Error */
    }

    /* Check if file exists */
    struct stat st;
    if (stat(rec.path, &st) != 0) {
        cameradb_free_recording(&rec);
        return 2;  /* File missing */
    }

    /* If no hash stored, we can't verify */
    if (!rec.file_hash || strlen(rec.file_hash) == 0) {
        cameradb_free_recording(&rec);
        return 0;  /* OK (no hash to verify) */
    }

    /* TODO: Implement hash verification when OpenSSL is available */
    /* For now, return OK if file exists */

    cameradb_free_recording(&rec);
    return 0;
}

/*
 * Memory management
 */

void cameradb_free_detection(cameradb_detection_t* det) {
    if (!det) return;
    free(det->class_label);
    free(det->image_path);
    det->class_label = NULL;
    det->image_path = NULL;
}

void cameradb_free_recording(cameradb_recording_t* rec) {
    if (!rec) return;
    free(rec->path);
    free(rec->file_hash);
    rec->path = NULL;
    rec->file_hash = NULL;
}

void cameradb_free_detections(cameradb_detection_t* dets, int count) {
    if (!dets) return;
    for (int i = 0; i < count; i++) {
        cameradb_free_detection(&dets[i]);
    }
    free(dets);
}

void cameradb_free_recordings(cameradb_recording_t* recs, int count) {
    if (!recs) return;
    for (int i = 0; i < count; i++) {
        cameradb_free_recording(&recs[i]);
    }
    free(recs);
}

/*
 * Statistics
 */

int cameradb_get_stats(cameradb_t* db, int64_t* total_recordings,
                        int64_t* total_detections, int64_t* pending_uploads) {
    if (!db) return -1;

    sqlite3_stmt* stmt;
    int rc;

    if (total_recordings) {
        rc = sqlite3_prepare_v2(db->handle, "SELECT COUNT(*) FROM recordings", -1, &stmt, NULL);
        if (rc == SQLITE_OK) {
            if (sqlite3_step(stmt) == SQLITE_ROW) {
                *total_recordings = sqlite3_column_int64(stmt, 0);
            }
            sqlite3_finalize(stmt);
        }
    }

    if (total_detections) {
        rc = sqlite3_prepare_v2(db->handle, "SELECT COUNT(*) FROM detections", -1, &stmt, NULL);
        if (rc == SQLITE_OK) {
            if (sqlite3_step(stmt) == SQLITE_ROW) {
                *total_detections = sqlite3_column_int64(stmt, 0);
            }
            sqlite3_finalize(stmt);
        }
    }

    if (pending_uploads) {
        rc = sqlite3_prepare_v2(db->handle,
            "SELECT COUNT(*) FROM detections WHERE uploaded = 0 AND orphaned = 0",
            -1, &stmt, NULL);
        if (rc == SQLITE_OK) {
            if (sqlite3_step(stmt) == SQLITE_ROW) {
                *pending_uploads = sqlite3_column_int64(stmt, 0);
            }
            sqlite3_finalize(stmt);
        }
    }

    return 0;
}
