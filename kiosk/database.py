"""
Database management for Kiosk.
Loads schema.sql, sets WAL mode, and provides repository operations
for user embeddings, attendance events, and offline outbox.
"""
import json
import uuid
import sqlite3
import logging
from pathlib import Path
from datetime import datetime, timezone
import numpy as np

logger = logging.getLogger(__name__)

KIOSK_ROOT = Path(__file__).resolve().parent
SCHEMA_FILE = KIOSK_ROOT / "schema.sql"


def get_connection(db_path: str | Path) -> sqlite3.Connection:
    """Create a new SQLite connection with WAL mode enabled."""
    conn = sqlite3.connect(str(db_path), check_same_thread=False)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA journal_mode = WAL;")
    conn.execute("PRAGMA foreign_keys = ON;")
    return conn


def init_db(db_path: str | Path) -> sqlite3.Connection:
    """Initialize SQLite database schema and return connection."""
    db_path = Path(db_path)
    db_path.parent.mkdir(parents=True, exist_ok=True)

    conn = get_connection(db_path)

    if SCHEMA_FILE.exists():
        with open(SCHEMA_FILE, "r", encoding="utf-8") as f:
            conn.executescript(f.read())
    else:
        logger.warning("schema.sql not found at %s", SCHEMA_FILE)

    return conn


class Database:
    """High-level repository for Face Gallery and Attendance."""

    def __init__(self, conn: sqlite3.Connection):
        self.conn = conn

    def load_active_gallery(self) -> list[dict]:
        """Load all active users and 512-d embeddings into RAM."""
        sql = """
            SELECT u.id AS user_id, u.name, u.employee_id, e.embedding
            FROM users u
            JOIN user_embeddings e ON u.id = e.user_id
            WHERE u.status = 'active' AND u.deleted_at IS NULL
        """
        cursor = self.conn.cursor()
        cursor.execute(sql)
        rows = cursor.fetchall()

        gallery = []
        for r in rows:
            emb = np.frombuffer(r["embedding"], dtype=np.float32)
            if emb.shape[0] == 512:
                norm = np.linalg.norm(emb)
                if norm > 0:
                    emb = emb / norm
                gallery.append({
                    "user_id": r["user_id"],
                    "name": r["name"],
                    "employee_id": r["employee_id"] or "",
                    "embedding": emb,
                })
        logger.info("Loaded %d face embeddings into RAM gallery", len(gallery))
        return gallery

    def is_debounced(self, user_id: str, debounce_seconds: int = 30) -> bool:
        """Check if user scanned within the last N seconds."""
        sql = """
            SELECT created_at FROM attendance_events
            WHERE user_id = ?
            ORDER BY created_at DESC LIMIT 1
        """
        cursor = self.conn.cursor()
        cursor.execute(sql, (user_id,))
        row = cursor.fetchone()
        if not row:
            return False

        try:
            last_dt = datetime.fromisoformat(row["created_at"].replace("Z", "+00:00"))
            now_dt = datetime.now(timezone.utc)
            return (now_dt - last_dt).total_seconds() < debounce_seconds
        except Exception:
            return False

    def determine_direction(self, user_id: str, default_dir: str = "AUTO") -> str:
        """
        First-In, Last-Out logic:
        - If default_dir is explicitly 'IN' or 'OUT', return it.
        - If 'AUTO':
            - If user has no scan recorded today -> 'IN' (First-In)
            - If user already scanned today -> 'OUT' (Last-Out)
        """
        if default_dir in ("IN", "OUT"):
            return default_dir

        today_start_iso = datetime.now().astimezone().replace(
            hour=0, minute=0, second=0, microsecond=0
        ).astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S")

        cursor = self.conn.cursor()
        cursor.execute(
            """
            SELECT event_id FROM attendance_events
            WHERE user_id = ? AND created_at >= ?
            LIMIT 1
            """,
            (user_id, today_start_iso),
        )
        row = cursor.fetchone()
        return "OUT" if row else "IN"

    def record_attendance(
        self,
        user_id: str,
        confidence: float,
        direction: str,
        device_id: str,
        debounce_seconds: int = 30,
        clock_synced: bool = False,
    ) -> dict | None:
        """Atomically record event in attendance_events and outbox using First-In, Last-Out."""
        if self.is_debounced(user_id, debounce_seconds):
            return None

        event_id = str(uuid.uuid4())
        now_iso = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")
        actual_direction = self.determine_direction(user_id, direction)

        payload = {
            "event_id": event_id,
            "user_id": user_id,
            "device_id": device_id,
            "direction": actual_direction,
            "confidence": round(float(confidence), 4),
            "clock_synced": bool(clock_synced),
            "created_at": now_iso,
        }

        cursor = self.conn.cursor()
        try:
            cursor.execute(
                """
                INSERT INTO attendance_events
                (event_id, user_id, device_id, direction, confidence, clock_synced, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                (event_id, user_id, device_id, actual_direction, confidence, 1 if clock_synced else 0, now_iso),
            )
            cursor.execute(
                """
                INSERT INTO outbox (event_id, type, payload, created_at, synced, retry_count)
                VALUES (?, 'attendance', ?, ?, 0, 0)
                """,
                (event_id, json.dumps(payload), now_iso),
            )
            self.conn.commit()
            return payload
        except Exception as e:
            self.conn.rollback()
            logger.error("Failed to record attendance: %s", e)
            raise e

    def register_user(
        self,
        name: str,
        employee_id: str,
        embedding: np.ndarray,
        department: str = "",
        model_version: str = "arcface_mobilefacenet_v1",
        device_id: str = "kiosk-01",
    ) -> str:
        """Register new user and save 512-d embedding."""
        user_id = str(uuid.uuid4())
        emb_id = str(uuid.uuid4())
        now_iso = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")

        emb = embedding.astype(np.float32)
        norm = np.linalg.norm(emb)
        if norm > 0:
            emb = emb / norm
        emb_bytes = emb.tobytes()

        cursor = self.conn.cursor()
        try:
            cursor.execute(
                """
                INSERT INTO users (id, employee_id, name, department, status, version, updated_at)
                VALUES (?, ?, ?, ?, 'active', 0, ?)
                """,
                (user_id, employee_id, name, department, now_iso),
            )
            cursor.execute(
                """
                INSERT INTO user_embeddings
                (id, user_id, embedding, model_version, source, device_id, quality_score, created_at)
                VALUES (?, ?, ?, ?, 'kiosk', ?, 1.0, ?)
                """,
                (emb_id, user_id, emb_bytes, model_version, device_id, now_iso),
            )
            cursor.execute(
                """
                INSERT INTO outbox (event_id, type, payload, created_at, synced, retry_count)
                VALUES (?, 'user_enrollment', ?, ?, 0, 0)
                """,
                (user_id, json.dumps({"user_id": user_id, "name": name, "employee_id": employee_id}), now_iso),
            )
            self.conn.commit()
            return user_id
        except Exception as e:
            self.conn.rollback()
            logger.error("Failed to register user: %s", e)
            raise e
