package postgres

import (
	"database/sql"
	"fmt"
	"os"

	"attendance-backend/internal/pkg/hasher"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func NewPostgresDB(databaseURL string) (*sql.DB, error) {
	if databaseURL == "" {
		host := getEnv("DB_HOST", "localhost")
		port := getEnv("DB_PORT", "5432")
		user := getEnv("DB_USER", "attendance_app")
		pass := getEnv("DB_PASSWORD", "changeme")
		dbName := getEnv("DB_NAME", "attendance")
		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbName)
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	if err := createSchema(db); err != nil {
		return nil, fmt.Errorf("failed to create postgres schema: %w", err)
	}

	if err := seedDefaults(db); err != nil {
		return nil, fmt.Errorf("failed to seed postgres defaults: %w", err)
	}

	return db, nil
}

func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS admins (
		id VARCHAR(64) PRIMARY KEY,
		username VARCHAR(128) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		role VARCHAR(32) DEFAULT 'admin',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		last_login TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS devices (
		id VARCHAR(64) PRIMARY KEY,
		name VARCHAR(128) NOT NULL,
		location VARCHAR(255),
		direction VARCHAR(32) DEFAULT 'AUTO',
		api_key_hash VARCHAR(255) NOT NULL,
		status VARCHAR(32) DEFAULT 'offline',
		last_seen TIMESTAMPTZ,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		revoked_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS device_state (
		device_id VARCHAR(64) PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
		applied_version INTEGER DEFAULT 0,
		model_version VARCHAR(128),
		sw_version VARCHAR(128),
		cpu_temp DOUBLE PRECISION,
		fps DOUBLE PRECISION,
		outbox_pending INTEGER DEFAULT 0,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(64) PRIMARY KEY,
		employee_id VARCHAR(128) UNIQUE,
		title VARCHAR(64),
		first_name VARCHAR(128),
		last_name VARCHAR(128),
		name VARCHAR(255) NOT NULL,
		department VARCHAR(128),
		status VARCHAR(32) DEFAULT 'active',
		version INTEGER DEFAULT 1,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS user_embeddings (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		embedding BYTEA NOT NULL,
		model_version VARCHAR(128) DEFAULT 'arcface_mobilefacenet_v1',
		source VARCHAR(64) DEFAULT 'kiosk',
		device_id VARCHAR(64),
		quality_score DOUBLE PRECISION DEFAULT 1.0,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT uq_user_embeddings_user_id UNIQUE (user_id)
	);

	CREATE TABLE IF NOT EXISTS attendance_events (
		event_id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL,
		device_id VARCHAR(64) NOT NULL,
		direction VARCHAR(32) DEFAULT 'AUTO',
		confidence DOUBLE PRECISION NOT NULL,
		clock_synced BOOLEAN DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL,
		synced_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS attendance_pairs (
		id SERIAL PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL,
		check_in_event_id VARCHAR(64),
		check_out_event_id VARCHAR(64),
		work_date DATE NOT NULL,
		hours_worked DOUBLE PRECISION,
		status VARCHAR(32) DEFAULT 'open',
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS enrollment_jobs (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL,
		image_path VARCHAR(512),
		status VARCHAR(32) DEFAULT 'pending',
		assigned_device_id VARCHAR(64),
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
		processed_at TIMESTAMPTZ,
		expires_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id SERIAL PRIMARY KEY,
		admin_id VARCHAR(64),
		action VARCHAR(128) NOT NULL,
		target_type VARCHAR(64),
		target_id VARCHAR(64),
		detail TEXT,
		created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	);

	-- Performance Indexes for O(1) and O(log N) lookups
	CREATE INDEX IF NOT EXISTS idx_pg_devices_api_key_hash ON devices(api_key_hash);
	CREATE INDEX IF NOT EXISTS idx_pg_users_version ON users(version);
	CREATE INDEX IF NOT EXISTS idx_pg_user_embeddings_user_id ON user_embeddings(user_id);
	CREATE INDEX IF NOT EXISTS idx_pg_pairs_user_workdate_status ON attendance_pairs(user_id, work_date, status);
	CREATE INDEX IF NOT EXISTS idx_pg_pairs_workdate ON attendance_pairs(work_date);
	CREATE INDEX IF NOT EXISTS idx_pg_attendance_events_user_created ON attendance_events(user_id, created_at);
	`
	_, err := db.Exec(schema)
	return err
}

func seedDefaults(db *sql.DB) error {
	isProd := os.Getenv("ENV") == "production" || os.Getenv("GIN_MODE") == "release"

	// 1. Seed Admin
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM admins WHERE username = 'admin'").Scan(&count)
	if count == 0 {
		defaultPass := os.Getenv("ADMIN_DEFAULT_PASSWORD")
		if defaultPass == "" {
			if isProd {
				return fmt.Errorf("ADMIN_DEFAULT_PASSWORD environment variable is required in production mode")
			}
			defaultPass = "admin1234"
			fmt.Println("⚠️ NOTICE: Created default admin 'admin' with initial password 'admin1234'. Change this immediately.")
		}
		hash, _ := hasher.HashPassword(defaultPass)
		_, _ = db.Exec(
			"INSERT INTO admins (id, username, password_hash, role, created_at) VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)",
			"admin-default-uuid", "admin", hash, "admin",
		)
	}

	// 2. Seed Default Kiosk (only if initial development mode or key provided)
	var devCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM devices WHERE id = 'kiosk-01'").Scan(&devCount)
	if devCount == 0 && !isProd {
		kioskKey := os.Getenv("DEFAULT_KIOSK_API_KEY")
		if kioskKey == "" {
			kioskKey = "changeme-generate-with-openssl-rand-hex-32"
		}
		kioskHash := hasher.HashAPIKey(kioskKey)
		_, _ = db.Exec(
			"INSERT INTO devices (id, name, location, direction, api_key_hash, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)",
			"kiosk-01", "kiosk-01", "", "AUTO", kioskHash, "offline",
		)
		_, _ = db.Exec(
			"INSERT INTO device_state (device_id, applied_version, model_version, sw_version, updated_at) VALUES ($1, 0, '', '', CURRENT_TIMESTAMP) ON CONFLICT(device_id) DO NOTHING",
			"kiosk-01",
		)
	}

	return nil
}
