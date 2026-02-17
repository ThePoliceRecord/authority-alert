// Package cameradb provides access to the shared SQLite database for camera events.
package cameradb

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"supervisor/pkg/logger"

	_ "modernc.org/sqlite"
)

const (
	// DBPath is the path to the camera database
	DBPath = "/userdata/cameradb.sqlite"

	// SchemaVersion is the current schema version
	SchemaVersion = 3
)

var (
	db   *sql.DB
	once sync.Once
	mu   sync.Mutex
)

// Detection represents a detection record in the database
type Detection struct {
	ID          int64
	FrameTsMs   int64
	ClassID     int
	ClassLabel  string
	Confidence  float64
	BboxX       float64
	BboxY       float64
	BboxW       float64
	BboxH       float64
	ImagePath   string
	RecordingID int64
	Uploaded    bool
	Orphaned    bool
	CreatedAt   int64
}

// Recording represents a recording record in the database
type Recording struct {
	ID          int64
	Path        string
	StartTsMs   int64
	EndTsMs     int64
	DurationMs  int64
	FileSize    int64
	FileHash    string
	Uploaded    bool
	FileMissing bool
	CreatedAt   int64
}

// Stats contains database statistics
type Stats struct {
	TotalRecordings int64 `json:"total_recordings"`
	TotalDetections int64 `json:"total_detections"`
	PendingUploads  int64 `json:"pending_uploads"`
}

// Schema migrations - must match C cameradb migrations
var migrations = []struct {
	version     int
	description string
	sql         string
}{
	{
		version:     1,
		description: "Initial schema",
		sql: `
			CREATE TABLE IF NOT EXISTS schema_version (
				version INTEGER PRIMARY KEY,
				applied_at INTEGER NOT NULL,
				description TEXT
			);
			CREATE TABLE IF NOT EXISTS recordings (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				path TEXT NOT NULL,
				start_ts_ms INTEGER NOT NULL,
				end_ts_ms INTEGER,
				duration_ms INTEGER,
				file_size INTEGER,
				uploaded INTEGER DEFAULT 0,
				created_at INTEGER NOT NULL
			);
			CREATE INDEX IF NOT EXISTS idx_recordings_start ON recordings(start_ts_ms);
			CREATE INDEX IF NOT EXISTS idx_recordings_uploaded ON recordings(uploaded);
			CREATE TABLE IF NOT EXISTS detections (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				frame_ts_ms INTEGER NOT NULL,
				class_id INTEGER NOT NULL,
				class_label TEXT,
				confidence REAL NOT NULL,
				bbox_x REAL NOT NULL,
				bbox_y REAL NOT NULL,
				bbox_w REAL NOT NULL,
				bbox_h REAL NOT NULL,
				image_path TEXT,
				recording_id INTEGER,
				uploaded INTEGER DEFAULT 0,
				created_at INTEGER NOT NULL,
				FOREIGN KEY (recording_id) REFERENCES recordings(id)
			);
			CREATE INDEX IF NOT EXISTS idx_detections_frame_ts ON detections(frame_ts_ms);
			CREATE INDEX IF NOT EXISTS idx_detections_recording ON detections(recording_id);
			CREATE INDEX IF NOT EXISTS idx_detections_uploaded ON detections(uploaded);
		`,
	},
	{
		version:     2,
		description: "Add file hash for integrity",
		sql:         "ALTER TABLE recordings ADD COLUMN file_hash TEXT;",
	},
	{
		version:     3,
		description: "Add cleanup status columns",
		sql: `
			ALTER TABLE recordings ADD COLUMN file_missing INTEGER DEFAULT 0;
			ALTER TABLE detections ADD COLUMN orphaned INTEGER DEFAULT 0;
		`,
	},
}

// runMigrations applies any pending database migrations
func runMigrations(database *sql.DB) error {
	// Get current schema version
	var currentVersion int
	err := database.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&currentVersion)
	if err != nil {
		// Table doesn't exist yet, version is 0
		currentVersion = 0
	}

	logger.Info("Database schema version: %d", currentVersion)

	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}

		logger.Info("Applying migration %d: %s", m.version, m.description)

		tx, err := database.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Apply migration
		_, err = tx.Exec(m.sql)
		if err != nil {
			tx.Rollback()
			// Check if it's an "already exists" error for ALTER TABLE
			if m.version > 1 {
				// For ALTER TABLE migrations, column may already exist
				logger.Warning("Migration %d may have been partially applied: %v", m.version, err)
				// Record the version anyway
				_, err = database.Exec("INSERT OR REPLACE INTO schema_version (version, applied_at, description) VALUES (?, ?, ?)",
					m.version, time.Now().UnixMilli(), m.description)
				if err != nil {
					return fmt.Errorf("failed to record migration: %w", err)
				}
				continue
			}
			return fmt.Errorf("migration %d failed: %w", m.version, err)
		}

		// Record migration
		_, err = tx.Exec("INSERT INTO schema_version (version, applied_at, description) VALUES (?, ?, ?)",
			m.version, time.Now().UnixMilli(), m.description)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration: %w", err)
		}

		logger.Info("Migration %d applied successfully", m.version)
	}

	return nil
}

// Open opens the database connection (singleton)
func Open() (*sql.DB, error) {
	var err error
	once.Do(func() {
		dsn := DBPath + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-2000)"
		db, err = sql.Open("sqlite", dsn)
		if err != nil {
			logger.Error("Failed to open database: %v", err)
			return
		}

		// Set connection pool settings
		db.SetMaxOpenConns(1) // SQLite only supports one writer
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0) // Keep connection open

		// Enable foreign keys
		_, err = db.Exec("PRAGMA foreign_keys = ON")
		if err != nil {
			logger.Error("Failed to enable foreign keys: %v", err)
		}

		// Run migrations to ensure schema exists
		if err = runMigrations(db); err != nil {
			logger.Error("Failed to run migrations: %v", err)
			db.Close()
			db = nil
			return
		}

		logger.Info("Database connection opened: %s", DBPath)
	})
	return db, err
}

// Close closes the database connection
func Close() {
	mu.Lock()
	defer mu.Unlock()

	if db != nil {
		db.Close()
		db = nil
		logger.Info("Database connection closed")
	}
}

// GetPendingDetections returns detections that haven't been uploaded yet
func GetPendingDetections(limit int) ([]Detection, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query(`
		SELECT id, frame_ts_ms, class_id, class_label, confidence,
		       bbox_x, bbox_y, bbox_w, bbox_h, image_path,
		       COALESCE(recording_id, 0), uploaded, orphaned, created_at
		FROM detections
		WHERE uploaded = 0 AND orphaned = 0
		ORDER BY frame_ts_ms ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var dets []Detection
	for rows.Next() {
		var d Detection
		var classLabel, imagePath sql.NullString
		err := rows.Scan(
			&d.ID, &d.FrameTsMs, &d.ClassID, &classLabel, &d.Confidence,
			&d.BboxX, &d.BboxY, &d.BboxW, &d.BboxH, &imagePath,
			&d.RecordingID, &d.Uploaded, &d.Orphaned, &d.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		d.ClassLabel = classLabel.String
		d.ImagePath = imagePath.String
		dets = append(dets, d)
	}

	return dets, rows.Err()
}

// GetPendingRecordings returns recordings that haven't been uploaded yet
func GetPendingRecordings(limit int) ([]Recording, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := db.Query(`
		SELECT id, path, start_ts_ms, COALESCE(end_ts_ms, 0),
		       COALESCE(duration_ms, 0), COALESCE(file_size, 0),
		       COALESCE(file_hash, ''), file_missing, created_at
		FROM recordings
		WHERE uploaded = 0 AND file_missing = 0 AND end_ts_ms IS NOT NULL
		ORDER BY start_ts_ms ASC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var recs []Recording
	for rows.Next() {
		var r Recording
		err := rows.Scan(
			&r.ID, &r.Path, &r.StartTsMs, &r.EndTsMs,
			&r.DurationMs, &r.FileSize, &r.FileHash,
			&r.FileMissing, &r.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		recs = append(recs, r)
	}

	return recs, rows.Err()
}

// MarkDetectionUploaded marks a detection as uploaded
func MarkDetectionUploaded(id int64) error {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return fmt.Errorf("database not open")
	}

	_, err := db.Exec("UPDATE detections SET uploaded = 1 WHERE id = ?", id)
	return err
}

// MarkDetectionOrphaned marks a detection as orphaned (image missing)
func MarkDetectionOrphaned(id int64) error {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return fmt.Errorf("database not open")
	}

	_, err := db.Exec("UPDATE detections SET orphaned = 1 WHERE id = ?", id)
	return err
}

// MarkRecordingUploaded marks a recording as uploaded
func MarkRecordingUploaded(id int64) error {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return fmt.Errorf("database not open")
	}

	_, err := db.Exec("UPDATE recordings SET uploaded = 1 WHERE id = ?", id)
	return err
}

// MarkRecordingMissing marks a recording as having a missing file
func MarkRecordingMissing(id int64) error {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return fmt.Errorf("database not open")
	}

	_, err := db.Exec("UPDATE recordings SET file_missing = 1 WHERE id = ?", id)
	return err
}

// FindRecordingForTimestamp finds the recording that contains a given timestamp
func FindRecordingForTimestamp(tsMs int64) (*Recording, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	var r Recording
	err := db.QueryRow(`
		SELECT id, path, start_ts_ms, COALESCE(end_ts_ms, 0),
		       COALESCE(duration_ms, 0), COALESCE(file_size, 0),
		       COALESCE(file_hash, ''), file_missing, created_at
		FROM recordings
		WHERE start_ts_ms <= ? AND (end_ts_ms IS NULL OR end_ts_ms >= ?)
		ORDER BY start_ts_ms DESC
		LIMIT 1`, tsMs, tsMs).Scan(
		&r.ID, &r.Path, &r.StartTsMs, &r.EndTsMs,
		&r.DurationMs, &r.FileSize, &r.FileHash,
		&r.FileMissing, &r.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &r, nil
}

// GetRecording gets a recording by ID
func GetRecording(id int64) (*Recording, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	var r Recording
	err := db.QueryRow(`
		SELECT id, path, start_ts_ms, COALESCE(end_ts_ms, 0),
		       COALESCE(duration_ms, 0), COALESCE(file_size, 0),
		       COALESCE(file_hash, ''), uploaded, file_missing, created_at
		FROM recordings
		WHERE id = ?`, id).Scan(
		&r.ID, &r.Path, &r.StartTsMs, &r.EndTsMs,
		&r.DurationMs, &r.FileSize, &r.FileHash,
		&r.Uploaded, &r.FileMissing, &r.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &r, nil
}

// GetDetection gets a detection by ID
func GetDetection(id int64) (*Detection, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	var d Detection
	var classLabel, imagePath sql.NullString
	err := db.QueryRow(`
		SELECT id, frame_ts_ms, class_id, class_label, confidence,
		       bbox_x, bbox_y, bbox_w, bbox_h, image_path,
		       COALESCE(recording_id, 0), uploaded, orphaned, created_at
		FROM detections
		WHERE id = ?`, id).Scan(
		&d.ID, &d.FrameTsMs, &d.ClassID, &classLabel, &d.Confidence,
		&d.BboxX, &d.BboxY, &d.BboxW, &d.BboxH, &imagePath,
		&d.RecordingID, &d.Uploaded, &d.Orphaned, &d.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	d.ClassLabel = classLabel.String
	d.ImagePath = imagePath.String

	return &d, nil
}

// InsertDetection inserts a new detection record
func InsertDetection(det *Detection) (int64, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return 0, fmt.Errorf("database not open")
	}

	now := time.Now().UnixMilli()
	result, err := db.Exec(`
		INSERT INTO detections (frame_ts_ms, class_id, class_label, confidence,
		                        bbox_x, bbox_y, bbox_w, bbox_h, image_path,
		                        recording_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		det.FrameTsMs, det.ClassID, det.ClassLabel, det.Confidence,
		det.BboxX, det.BboxY, det.BboxW, det.BboxH, det.ImagePath,
		sql.NullInt64{Int64: det.RecordingID, Valid: det.RecordingID > 0},
		now)

	if err != nil {
		return 0, fmt.Errorf("insert failed: %w", err)
	}

	return result.LastInsertId()
}

// GetStats returns database statistics
func GetStats() (*Stats, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	stats := &Stats{}

	db.QueryRow("SELECT COUNT(*) FROM recordings").Scan(&stats.TotalRecordings)
	db.QueryRow("SELECT COUNT(*) FROM detections").Scan(&stats.TotalDetections)
	db.QueryRow("SELECT COUNT(*) FROM detections WHERE uploaded = 0 AND orphaned = 0").Scan(&stats.PendingUploads)

	return stats, nil
}

// GetDetectionsByRecording returns all detections for a recording
func GetDetectionsByRecording(recordingID int64) ([]Detection, error) {
	mu.Lock()
	defer mu.Unlock()

	if db == nil {
		return nil, fmt.Errorf("database not open")
	}

	rows, err := db.Query(`
		SELECT id, frame_ts_ms, class_id, class_label, confidence,
		       bbox_x, bbox_y, bbox_w, bbox_h, image_path,
		       COALESCE(recording_id, 0), uploaded, orphaned, created_at
		FROM detections
		WHERE recording_id = ?
		ORDER BY frame_ts_ms ASC`, recordingID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var dets []Detection
	for rows.Next() {
		var d Detection
		var classLabel, imagePath sql.NullString
		err := rows.Scan(
			&d.ID, &d.FrameTsMs, &d.ClassID, &classLabel, &d.Confidence,
			&d.BboxX, &d.BboxY, &d.BboxW, &d.BboxH, &imagePath,
			&d.RecordingID, &d.Uploaded, &d.Orphaned, &d.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		d.ClassLabel = classLabel.String
		d.ImagePath = imagePath.String
		dets = append(dets, d)
	}

	return dets, rows.Err()
}
