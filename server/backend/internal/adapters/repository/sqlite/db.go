package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"attendance-backend/internal/pkg/hasher"
	_ "modernc.org/sqlite"
)

func NewSQLiteDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		dbPath = "data/server.db"
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Enable WAL mode & foreign keys
	if _, err := db.Exec("PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("failed to set pragma: %w", err)
	}

	if err := createSchema(db); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	if err := seedDefaults(db); err != nil {
		return nil, fmt.Errorf("failed to seed defaults: %w", err)
	}

	return db, nil
}

func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS admins (
		id TEXT PRIMARY KEY,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT DEFAULT 'admin',
		created_at DATETIME,
		last_login DATETIME
	);

	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		location TEXT,
		direction TEXT DEFAULT 'AUTO',
		api_key_hash TEXT NOT NULL,
		status TEXT DEFAULT 'offline',
		last_seen DATETIME,
		created_at DATETIME,
		revoked_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS device_state (
		device_id TEXT PRIMARY KEY,
		applied_version INTEGER DEFAULT 0,
		model_version TEXT,
		sw_version TEXT,
		cpu_temp REAL,
		fps REAL,
		outbox_pending INTEGER DEFAULT 0,
		updated_at DATETIME,
		FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		employee_id TEXT UNIQUE,
		title TEXT,
		first_name TEXT,
		last_name TEXT,
		name TEXT NOT NULL,
		department TEXT,
		status TEXT DEFAULT 'active',
		version INTEGER DEFAULT 1,
		deleted_at DATETIME,
		created_at DATETIME,
		updated_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS user_embeddings (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		embedding BLOB NOT NULL,
		model_version TEXT DEFAULT 'arcface_mobilefacenet_v1',
		source TEXT DEFAULT 'kiosk',
		device_id TEXT,
		quality_score REAL DEFAULT 1.0,
		created_at DATETIME,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS attendance_events (
		event_id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		device_id TEXT NOT NULL,
		direction TEXT DEFAULT 'AUTO',
		confidence REAL NOT NULL,
		clock_synced BOOLEAN DEFAULT 0,
		created_at DATETIME NOT NULL,
		synced_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS attendance_pairs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		check_in_event_id TEXT,
		check_out_event_id TEXT,
		work_date DATE NOT NULL,
		hours_worked REAL,
		status TEXT DEFAULT 'open',
		created_at DATETIME,
		updated_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS enrollment_jobs (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		image_path TEXT,
		status TEXT DEFAULT 'pending',
		assigned_device_id TEXT,
		created_at DATETIME,
		processed_at DATETIME,
		expires_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id TEXT,
		action TEXT NOT NULL,
		target_type TEXT,
		target_id TEXT,
		detail TEXT,
		created_at DATETIME
	);
	`
	_, err := db.Exec(schema)
	return err
}

func seedDefaults(db *sql.DB) error {
	// 1. Seed Admin
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM admins WHERE username = 'admin'").Scan(&count)
	if count == 0 {
		hash, _ := hasher.HashPassword("admin1234")
		_, _ = db.Exec(
			"INSERT INTO admins (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)",
			"admin-default-uuid", "admin", hash, "admin",
		)
	}

	// 2. Seed Default Kiosk
	var devCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM devices WHERE id = 'kiosk-01'").Scan(&devCount)
	if devCount == 0 {
		kioskHash, _ := hasher.HashPassword("changeme-generate-with-openssl-rand-hex-32")
		_, _ = db.Exec(
			"INSERT INTO devices (id, name, location, direction, api_key_hash, status, created_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
			"kiosk-01", "kiosk-01", "", "AUTO", kioskHash, "offline",
		)
		_, _ = db.Exec(
			"INSERT OR IGNORE INTO device_state (device_id, applied_version, model_version, sw_version, updated_at) VALUES (?, 0, '', '', CURRENT_TIMESTAMP)",
			"kiosk-01",
		)
	}

	return nil
}
