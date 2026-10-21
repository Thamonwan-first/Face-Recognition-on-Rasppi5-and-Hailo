-- ─────────────────────────────────────────────────────────────
-- SQLite Schema for Edge AI Face Recognition Attendance Kiosk
-- WAL Mode for concurrent reads & writes
-- ─────────────────────────────────────────────────────────────

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

-- 1. Users (synced down from server)
CREATE TABLE IF NOT EXISTS users (
    id              TEXT PRIMARY KEY,                  -- UUID
    employee_id     TEXT,
    name            TEXT NOT NULL,
    department      TEXT,
    status          TEXT NOT NULL DEFAULT 'active',   -- active | pending_approval | inactive
    version         INTEGER NOT NULL DEFAULT 0,       -- server version for sync tracking
    deleted_at      TEXT,                             -- ISO-8601; soft-delete
    updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- 2. Embeddings (created by Hailo-8 hardware)
CREATE TABLE IF NOT EXISTS user_embeddings (
    id              TEXT PRIMARY KEY,                  -- UUID
    user_id         TEXT NOT NULL REFERENCES users(id),
    embedding       BLOB NOT NULL,                    -- float32 × 512 = 2048 bytes
    model_version   TEXT NOT NULL,                    -- e.g. "arcface_mobilefacenet_v1"
    source          TEXT NOT NULL DEFAULT 'kiosk',    -- kiosk | dashboard
    device_id       TEXT,                             -- kiosk id
    quality_score   REAL DEFAULT 1.0,
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_emb_user ON user_embeddings(user_id);

-- 3. Attendance Events (raw scans generated locally)
CREATE TABLE IF NOT EXISTS attendance_events (
    event_id        TEXT PRIMARY KEY,                  -- UUID (Idempotency key)
    user_id         TEXT NOT NULL,
    device_id       TEXT NOT NULL,
    direction       TEXT NOT NULL DEFAULT 'AUTO',      -- IN | OUT | AUTO
    confidence      REAL NOT NULL,
    clock_synced    INTEGER NOT NULL DEFAULT 0,        -- 1 = NTP was synced at scan time
    created_at      TEXT NOT NULL,                     -- ISO-8601 scan timestamp
    synced_at       TEXT                               -- set upon successful upload
);
CREATE INDEX IF NOT EXISTS idx_att_user ON attendance_events(user_id);
CREATE INDEX IF NOT EXISTS idx_att_created ON attendance_events(created_at);

-- 4. Outbox (Offline-first transaction queue for sync to server)
CREATE TABLE IF NOT EXISTS outbox (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id        TEXT NOT NULL,                     -- UUID linking to event
    type            TEXT NOT NULL,                     -- 'attendance' | 'user_enrollment' | 'heartbeat'
    payload         TEXT NOT NULL,                     -- JSON string
    created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    synced          INTEGER NOT NULL DEFAULT 0,
    retry_count     INTEGER NOT NULL DEFAULT 0,
    last_retry_at   TEXT
);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox(synced, created_at);

-- 5. Sync State (bookkeeping)
CREATE TABLE IF NOT EXISTS sync_state (
    key             TEXT PRIMARY KEY,
    value           TEXT NOT NULL
);
INSERT OR IGNORE INTO sync_state(key, value) VALUES ('last_user_version', '0');
INSERT OR IGNORE INTO sync_state(key, value) VALUES ('schema_version', '1');

-- 6. Enrollment Jobs (pulled from server for local Hailo embedding)
CREATE TABLE IF NOT EXISTS enrollment_jobs (
    id              TEXT PRIMARY KEY,                  -- UUID from server
    user_id         TEXT NOT NULL,
    image_url       TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',  -- pending | processing | done | failed
    created_at      TEXT NOT NULL,
    processed_at    TEXT
);
