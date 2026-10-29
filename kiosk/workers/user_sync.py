"""
User Sync Worker — pulls user changes (new, updated, deleted) from Server.
Uses version-based query: GET /sync/users?since=<last_version>
Implements thread-isolated SQLite connection.
"""
import time
import base64
import logging
import threading
from pathlib import Path
import requests

from database import get_connection

logger = logging.getLogger(__name__)


class UserSyncWorker:
    def __init__(self, db_source, config, on_users_updated=None):
        self.db_source = db_source
        self.config = config
        self.on_users_updated = on_users_updated
        self.conn = None
        self._running = False
        self._thread = None

    def start(self):
        self._running = True
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()
        logger.info("UserSyncWorker started")

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
                self._pull_user_updates()
            except Exception as e:
                logger.error("User sync error: %s", e)
            time.sleep(30)

    def _get_last_version(self) -> int:
        conn = self._get_connection()
        cur = conn.cursor()
        cur.execute("SELECT value FROM sync_state WHERE key = 'last_user_version'")
        row = cur.fetchone()
        return int(row["value"]) if row else 0

    def _pull_user_updates(self):
        last_v = self._get_last_version()
        url = f"{self.config.server_url.rstrip('/')}/sync/users?since={last_v}"
        headers = {"X-API-Key": self.config.api_key}

        try:
            res = requests.get(url, headers=headers, timeout=5, verify=getattr(self.config, 'ssl_verify', False))
            if res.status_code != 200:
                return

            data = res.json()
            users = data.get("users", [])
            latest_v = data.get("latest_version", last_v)

            if not users:
                return

            conn = self._get_connection()
            cur = conn.cursor()
            for u in users:
                user_id = u["id"]
                deleted_at = u.get("deleted_at")

                if deleted_at:
                    cur.execute("UPDATE users SET deleted_at = ?, status = 'inactive' WHERE id = ?", (deleted_at, user_id))
                    cur.execute("DELETE FROM user_embeddings WHERE user_id = ?", (user_id,))
                else:
                    cur.execute(
                        """
                        INSERT INTO users (id, employee_id, name, department, status, version)
                        VALUES (?, ?, ?, ?, ?, ?)
                        ON CONFLICT(id) DO UPDATE SET
                          employee_id=excluded.employee_id,
                          name=excluded.name,
                          department=excluded.department,
                          status=excluded.status,
                          version=excluded.version
                        """,
                        (user_id, u.get("employee_id"), u["name"], u.get("department"), u.get("status", "active"), u.get("version", 0))
                    )
                    emb_b64 = u.get("embedding_b64")
                    if emb_b64:
                        emb_bytes = base64.b64decode(emb_b64)
                        # Ensure no duplicate embeddings exist for this user
                        cur.execute("DELETE FROM user_embeddings WHERE user_id = ?", (user_id,))
                        cur.execute(
                            """
                            INSERT INTO user_embeddings (id, user_id, embedding, model_version, source)
                            VALUES (?, ?, ?, 'arcface_mobilefacenet_v1', 'server_sync')
                            """,
                            (f"emb-{user_id}", user_id, emb_bytes)
                        )

            cur.execute("UPDATE sync_state SET value = ? WHERE key = 'last_user_version'", (str(latest_v),))
            conn.commit()
            logger.info("Synced %d user update(s) from server (new version: %d)", len(users), latest_v)

            if self.on_users_updated:
                self.on_users_updated()

        except Exception as e:
            logger.debug("User sync server unreachable: %s", e)
