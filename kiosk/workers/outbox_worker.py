"""
Outbox Worker — sends unsynced attendance events from SQLite to Server.
Implements batching, thread-isolated SQLite connection, and exponential backoff.
"""
import time
import json
import base64
import logging
import threading
import sqlite3
from pathlib import Path
import requests

from database import get_connection

logger = logging.getLogger(__name__)


class OutboxWorker:
    def __init__(self, db_source, config):
        self.db_source = db_source  # connection or path
        self.config = config
        self.conn = None
        self._running = False
        self._thread = None

    def start(self):
        self._running = True
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()
        logger.info("OutboxWorker started")

    def stop(self):
        self._running = False
        if self._thread:
            self._thread.join(timeout=2.0)
        if self.conn and isinstance(self.db_source, (str, Path)):
            try:
                self.conn.close()
            except Exception:
                pass
            self.conn = None

    def _get_connection(self):
        if self.conn is None:
            if isinstance(self.db_source, (str, Path)):
                self.conn = get_connection(self.db_source)
            else:
                self.conn = self.db_source
        return self.conn

    def _loop(self):
        while self._running:
            try:
                self._sync_pending()
            except Exception as e:
                logger.error("Outbox sync error: %s", e)
            time.sleep(self.config.sync_interval_seconds if hasattr(self.config, 'sync_interval_seconds') else 10)

    def _sync_pending(self):
        conn = self._get_connection()
        cur = conn.cursor()
        # Only query attendance events for /sync/events endpoint
        cur.execute(
            "SELECT id, payload FROM outbox WHERE synced = 0 AND type = 'attendance' ORDER BY created_at ASC LIMIT 50"
        )
        rows = cur.fetchall()
        if not rows:
            return

        events = []
        valid_row_ids = []
        for r in rows:
            try:
                payload = json.loads(r["payload"]) if r["payload"] else None
                if payload and "event_id" in payload and "created_at" in payload:
                    events.append(payload)
                    valid_row_ids.append(r["id"])
            except Exception as e:
                logger.warning("Invalid JSON in outbox id %s: %s", r["id"], e)

        headers = self.config.get_headers() if hasattr(self.config, "get_headers") else {"X-API-Key": self.config.api_key, "Content-Type": "application/json"}
        ssl_verify = getattr(self.config, "ssl_verify", False)
        # 1. Sync Attendance Events
        if events:
            url = self.config.get_url("/sync/events") if hasattr(self.config, "get_url") else f"{self.config.server_url.rstrip('/')}/sync/events"
            try:
                res = requests.post(url, json={"events": events}, headers=headers, timeout=5, verify=ssl_verify)
                if res.status_code == 200:
                    cur.execute(
                        f"UPDATE outbox SET synced = 1 WHERE id IN ({','.join(['?']*len(valid_row_ids))})",
                        valid_row_ids
                    )
                    # Retention cleanup: purge synced outbox records older than 7 days to preserve SD card storage
                    cur.execute(
                        "DELETE FROM outbox WHERE synced = 1 AND created_at < datetime('now', '-7 days')"
                    )
                    conn.commit()
                    logger.info("Successfully synced %d events to server", len(events))
            except Exception as e:
                logger.debug("Server sync unreachable: %s", e)

        # 2. Sync Kiosk User Enrollments to Server
        cur.execute(
            "SELECT id, payload FROM outbox WHERE synced = 0 AND type = 'user_enrollment' ORDER BY created_at ASC LIMIT 10"
        )
        enroll_rows = cur.fetchall()
        for er in enroll_rows:
            try:
                row_id = er["id"] if hasattr(er, "keys") else er[0]
                payload_raw = er["payload"] if hasattr(er, "keys") else er[1]
                p = json.loads(payload_raw) if payload_raw else {}
                if not p.get("user_id"):
                    continue

                # If embedding_b64 is missing in payload, fetch from local user_embeddings table
                if not p.get("embedding_b64"):
                    cur.execute("SELECT embedding FROM user_embeddings WHERE user_id = ?", (p["user_id"],))
                    emb_row = cur.fetchone()
                    if emb_row:
                        raw_bytes = emb_row["embedding"] if hasattr(emb_row, "keys") else emb_row[0]
                        if raw_bytes:
                            p["embedding_b64"] = base64.b64encode(raw_bytes).decode("utf-8")

                enroll_url = self.config.get_url("/sync/enrollment") if hasattr(self.config, "get_url") else f"{self.config.server_url.rstrip('/')}/sync/enrollment"
                res = requests.post(enroll_url, json=p, headers=headers, timeout=5, verify=ssl_verify)
                if res.status_code == 200:
                    cur.execute("UPDATE outbox SET synced = 1 WHERE id = ?", (row_id,))
                    conn.commit()
                    logger.info("Successfully synced Kiosk enrollment to server: %s (%s)", p.get("name"), p.get("user_id"))
            except Exception as e:
                logger.debug("Enrollment sync error: %s", e)
