"""
Heartbeat Worker — reports device telemetry to Server every 60 seconds.
Implements thread-isolated SQLite connection.
"""
import time
import logging
import threading
from pathlib import Path
import requests

from database import get_connection
from metrics import MetricsTracker

logger = logging.getLogger(__name__)


class HeartbeatWorker:
    def __init__(self, db_source, config):
        self.db_source = db_source
        self.config = config
        self.conn = None
        self._running = False
        self._thread = None

    def start(self):
        self._running = True
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()
        logger.info("HeartbeatWorker started")

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
                self._send_heartbeat()
            except Exception as e:
                logger.error("Heartbeat error: %s", e)

            # Sleep 15 seconds (checks _running every 0.5s for clean shutdown)
            for _ in range(30):
                if not self._running:
                    break
                time.sleep(0.5)

    def _send_heartbeat(self):
        url = self.config.get_url("/sync/heartbeat") if hasattr(self.config, "get_url") else f"{self.config.server_url.rstrip('/')}/sync/heartbeat"
        headers = self.config.get_headers() if hasattr(self.config, "get_headers") else {"X-API-Key": self.config.api_key, "Content-Type": "application/json"}

        conn = self._get_connection()
        # Get latest sync version
        cur = conn.cursor()
        cur.execute("SELECT value FROM sync_state WHERE key = 'last_user_version'")
        row = cur.fetchone()
        applied_v = int(row["value"]) if row else 0

        payload = {
            "applied_version": applied_v,
            "sw_version": "v1.0.0",
            "model_version": "scrfd_2.5g + arcface_mobilefacenet",
            "cpu_temp": MetricsTracker.get_cpu_temperature(),
            "fps": MetricsTracker.get_fps(),
            "outbox_pending": MetricsTracker.get_outbox_pending_count(conn),
        }

        try:
            requests.post(url, json=payload, headers=headers, timeout=5, verify=getattr(self.config, 'ssl_verify', False))
            logger.debug("Heartbeat sent successfully")
        except Exception as e:
            logger.debug("Heartbeat server unreachable: %s", e)
