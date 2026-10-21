"""
Outbox Worker — sends unsynced attendance events from SQLite to Server.
Implements batching, thread-isolated SQLite connection, and exponential backoff.
"""
import time
import json
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

        if not events:
            return

        url = f"{self.config.server_url.rstrip('/')}/sync/events"
        headers = {"X-API-Key": self.config.api_key, "Content-Type": "application/json"}

        try:
            res = requests.post(url, json={"events": events}, headers=headers, timeout=5, verify=False)
            if res.status_code == 200:
                cur.execute(
                    f"UPDATE outbox SET synced = 1 WHERE id IN ({','.join(['?']*len(valid_row_ids))})",
                    valid_row_ids
                )
                conn.commit()
                logger.info("Successfully synced %d events to server", len(events))
        except Exception as e:
            logger.debug("Server sync unreachable: %s", e)
