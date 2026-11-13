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
        url = self.config.get_url(f"/sync/users?since={last_v}") if hasattr(self.config, "get_url") else f"{self.config.server_url.rstrip('/')}/sync/users?since={last_v}"
        headers = self.config.get_headers(json_content=False) if hasattr(self.config, "get_headers") else {"X-API-Key": self.config.api_key}

        try:
            res = requests.get(url, headers=headers, timeout=5, verify=getattr(self.config, 'ssl_verify', False))
            if res.status_code != 200:
                return

            data = res.json()
            users = data.get("users", [])
            latest_v = data.get("latest_version", last_v)

            conn = self._get_connection()
            cur = conn.cursor()

            # Handle server database re-seed or version reset
            if latest_v < last_v:
                logger.warning(
                    "Server version (%d) is lower than kiosk version (%d). Resetting local gallery to re-sync.",
                    latest_v, last_v
                )
                cur.execute("DELETE FROM user_embeddings")
                cur.execute("DELETE FROM users")
                cur.execute("UPDATE sync_state SET value = '0' WHERE key = 'last_user_version'")
                conn.commit()
                last_v = 0
                if self.on_users_updated:
                    self.on_users_updated()
                return

            if not users:
                return

            # Pre-download avatar images outside SQLite transaction to prevent locking the database
            avatars_dir = Path(__file__).resolve().parent.parent / "data" / "avatars"
            avatars_dir.mkdir(parents=True, exist_ok=True)
            user_photo_paths = {}

            for u in users:
                user_id = u["id"]
                photo_url = u.get("photo_url")
                local_img_path = avatars_dir / f"{user_id}.jpg"

                if photo_url:
                    full_photo_url = self.config.get_url(photo_url) if hasattr(self.config, "get_url") else f"{self.config.server_url.rstrip('/')}{photo_url}"
                    try:
                        img_res = requests.get(full_photo_url, headers=headers, timeout=5, verify=getattr(self.config, 'ssl_verify', False))
                        if img_res.status_code == 200:
                            local_img_path.write_bytes(img_res.content)
                            user_photo_paths[user_id] = str(local_img_path)
                    except Exception as e:
                        logger.debug("Failed downloading avatar for %s: %s", user_id, e)
                        if local_img_path.exists():
                            user_photo_paths[user_id] = str(local_img_path)
                elif local_img_path.exists():
                    user_photo_paths[user_id] = str(local_img_path)

            # Rapid atomic SQLite transaction without any blocking network I/O
            conn = self._get_connection()
            cur = conn.cursor()

            for u in users:
                user_id = u["id"]
                deleted_at = u.get("deleted_at")
                status = u.get("status", "active")
                photo_path = user_photo_paths.get(user_id, "")

                if deleted_at or status != "active":
                    cur.execute(
                        """
                        INSERT INTO users (id, employee_id, name, department, status, version, photo_path, deleted_at)
                        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                        ON CONFLICT(id) DO UPDATE SET
                          employee_id=excluded.employee_id,
                          name=excluded.name,
                          department=excluded.department,
                          status=excluded.status,
                          version=excluded.version,
                          photo_path=COALESCE(excluded.photo_path, users.photo_path),
                          deleted_at=excluded.deleted_at
                        """,
                        (user_id, u.get("employee_id"), u["name"], u.get("department"), status, u.get("version", 0), photo_path or None, deleted_at or "suspended")
                    )
                    cur.execute("DELETE FROM user_embeddings WHERE user_id = ?", (user_id,))
                else:
                    cur.execute(
                        """
                        INSERT INTO users (id, employee_id, name, department, status, version, photo_path)
                        VALUES (?, ?, ?, ?, ?, ?, ?)
                        ON CONFLICT(id) DO UPDATE SET
                          employee_id=excluded.employee_id,
                          name=excluded.name,
                          department=excluded.department,
                          status=excluded.status,
                          version=excluded.version,
                          photo_path=COALESCE(excluded.photo_path, users.photo_path)
                        """,
                        (user_id, u.get("employee_id"), u["name"], u.get("department"), "active", u.get("version", 0), photo_path or None)
                    )
                    emb_b64 = u.get("embedding_b64")
                    cur.execute("DELETE FROM user_embeddings WHERE user_id = ?", (user_id,))
                    if emb_b64:
                        emb_bytes = base64.b64decode(emb_b64)
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
