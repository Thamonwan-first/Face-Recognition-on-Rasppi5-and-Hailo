"""
Enrollment Worker Thread — pulls pending enrollment jobs from Server,
computes 512-d Face Embedding on Hailo-8 (or CPU Fallback),
and submits the result back to Server.
"""
import time
import uuid
import json
import shutil
import base64
import sqlite3
import logging
import threading
from pathlib import Path
from datetime import datetime, timezone
import cv2
import numpy as np
import requests

from alignment import align_face

logger = logging.getLogger(__name__)


class EnrollmentWorker:
    def __init__(self, detector, config, on_enrollment_done=None):
        self.detector = detector
        self.config = config
        self.on_enrollment_done = on_enrollment_done
        self._running = False
        self._thread = None

    def start(self):
        self._running = True
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()
        logger.info("EnrollmentWorker started")

    def stop(self):
        self._running = False
        if self._thread:
            self._thread.join(timeout=2.0)

    def _loop(self):
        while self._running:
            try:
                self._check_and_process_job()
            except Exception as e:
                logger.error("EnrollmentWorker error: %s", e)

            try:
                self._check_and_process_local_folder()
            except Exception as e:
                logger.error("Local enrollment folder error: %s", e)

            # Check every 4 seconds
            for _ in range(8):
                if not self._running:
                    break
                time.sleep(0.5)

    def _check_and_process_job(self):
        claim_url = self.config.get_url("/sync/enrollment/claim") if hasattr(self.config, "get_url") else f"{self.config.server_url.rstrip('/')}/sync/enrollment/claim"
        headers = self.config.get_headers(json_content=False) if hasattr(self.config, "get_headers") else {"X-API-Key": self.config.api_key}

        try:
            resp = requests.get(claim_url, headers=headers, timeout=5, verify=getattr(self.config, 'ssl_verify', False))
            if resp.status_code != 200:
                return
            data = resp.json()
            job = data.get("job")
            if not job:
                return

            job_id = job["id"]
            user_id = job["user_id"]
            images_b64 = (
                job.get("images_b64")
                or data.get("images_b64")
                or ([job["image_b64"]] if job.get("image_b64") else [])
                or ([data["image_b64"]] if data.get("image_b64") else [])
            )

            logger.info("Claimed enrollment job %s for user %s (%d image(s))", job_id, user_id, len(images_b64))

            if not images_b64:
                logger.warning("Enrollment job %s has no image data", job_id)
                return

            captured_embeddings = []
            for idx, img_b64 in enumerate(images_b64):
                try:
                    # Decode image
                    img_bytes = base64.b64decode(img_b64)
                    nparr = np.frombuffer(img_bytes, np.uint8)
                    frame_bgr = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
                    if frame_bgr is None:
                        logger.warning("Failed to decode image #%d for job %s", idx, job_id)
                        continue

                    frame_rgb = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2RGB)

                    # Detect face
                    faces = self.detector.detect_faces(frame_rgb)
                    if not faces:
                        logger.warning("No face detected in enrollment image #%d for job %s, using fallback center crop", idx, job_id)
                        h, w = frame_rgb.shape[:2]
                        s = min(h, w)
                        crop = frame_rgb[(h - s) // 2:(h + s) // 2, (w - s) // 2:(w + s) // 2]
                        aligned = cv2.resize(crop, (112, 112))
                    else:
                        best_face = max(faces, key=lambda f: (f["bbox"][2] - f["bbox"][0]) * (f["bbox"][3] - f["bbox"][1]))
                        landmarks = best_face.get("landmarks", [])
                        if len(landmarks) >= 5:
                            aligned = align_face(frame_rgb, np.array(landmarks, dtype=np.float32), 112)
                        else:
                            x1, y1, x2, y2 = best_face["bbox"]
                            h, w = frame_rgb.shape[:2]
                            crop = frame_rgb[max(0, y1):min(h, y2), max(0, x1):min(w, x2)]
                            aligned = cv2.resize(crop, (112, 112))

                    emb = self.detector.extract_embedding(aligned)
                    if emb is not None:
                        captured_embeddings.append(emb)
                except Exception as img_err:
                    logger.error("Error processing enrollment image #%d for job %s: %s", idx, job_id, img_err)

            result_url = self.config.get_url("/sync/enrollment/result") if hasattr(self.config, "get_url") else f"{self.config.server_url.rstrip('/')}/sync/enrollment/result"
            post_headers = self.config.get_headers() if hasattr(self.config, "get_headers") else {"X-API-Key": self.config.api_key, "Content-Type": "application/json"}

            if not captured_embeddings:
                logger.error("Could not extract any embeddings for job %s. Notifying server of failure.", job_id)
                try:
                    requests.post(result_url, json={"job_id": job_id, "embedding_b64": "", "quality_score": 0.0}, headers=post_headers, timeout=5, verify=getattr(self.config, 'ssl_verify', False))
                except Exception as post_err:
                    logger.debug("Failed notifying server of enrollment failure: %s", post_err)
                return

            # Compute average embedding (mean vector) and normalize with L2 norm
            mean_emb = np.mean(captured_embeddings, axis=0)
            norm = np.linalg.norm(mean_emb)
            if norm > 1e-6:
                mean_emb = mean_emb / norm

            emb_bytes = mean_emb.astype(np.float32).tobytes()
            emb_b64 = base64.b64encode(emb_bytes).decode("utf-8")

            # Submit result to Server
            payload = {
                "job_id": job_id,
                "embedding_b64": emb_b64,
                "quality_score": 1.0,
                "images_count": len(captured_embeddings),
            }
            res_post = requests.post(result_url, json=payload, headers=post_headers, timeout=5, verify=getattr(self.config, 'ssl_verify', False))
            if res_post.status_code == 200:
                logger.info("Successfully completed enrollment job %s from %d image(s)! Embedding stored on server.", job_id, len(captured_embeddings))
                if self.on_enrollment_done:
                    self.on_enrollment_done()
            else:
                logger.error("Failed to submit enrollment result: HTTP %d %s", res_post.status_code, res_post.text)

        except Exception as e:
            logger.error("Error executing enrollment job: %s", e)

    def _check_and_process_local_folder(self):
        """
        Scan kiosk/enrollment_images folder for photos or subfolders named by employee_id.
        Computes 512-d ArcFace embeddings, registers users in local kiosk.db,
        caches avatars, pushes to outbox for server sync, and moves images to processed/.
        """
        kiosk_root = Path(__file__).resolve().parent.parent
        candidate_dirs = [
            kiosk_root / "enrollment_images",
            kiosk_root / "employee",
            kiosk_root / "employees",
        ]
        target_dirs = [d for d in candidate_dirs if d.is_dir()]
        if not target_dirs:
            return

        processed_dir = kiosk_root / "enrollment_images" / "processed"
        processed_dir.mkdir(parents=True, exist_ok=True)
        avatars_dir = kiosk_root / "data" / "avatars"
        avatars_dir.mkdir(parents=True, exist_ok=True)

        valid_exts = {".jpg", ".jpeg", ".png", ".webp"}
        generic_names = {"img", "image", "photo", "face", "front", "side", "tilt", "pic", "picture", "1", "2", "3", "4", "5", "employee", "employees", "photos", "images"}
        employee_groups = {}  # emp_id -> list of Path objects

        # Recursively scan candidate directories (handles flat files, subfolders, or nested folders like employee/)
        for enroll_dir in target_dirs:
            for p in sorted(enroll_dir.rglob("*")):
                if not p.is_file() or p.suffix.lower() not in valid_exts:
                    continue
                try:
                    rel_parts = p.relative_to(enroll_dir).parts
                except Exception:
                    continue

                # Ignore processed or hidden folders
                if any(part.startswith(".") or part == "processed" for part in rel_parts):
                    continue

                clean_stem = p.stem.split("_")[0].strip()
                parent_name = p.parent.name.strip()

                # Determine employee ID:
                # 1. If filename is meaningful (not generic), use filename
                # 2. Otherwise if parent folder is meaningful (not generic like "employee" or "photos"), use parent folder
                if clean_stem.lower() not in generic_names:
                    emp_id = clean_stem
                elif parent_name.lower() not in generic_names and parent_name != enroll_dir.name:
                    emp_id = parent_name
                else:
                    emp_id = clean_stem

                if emp_id and emp_id.lower() not in generic_names:
                    employee_groups.setdefault(emp_id, []).append(p)

        if not employee_groups:
            return

        db_path = getattr(self.config, "db_full_path", kiosk_root / "data" / "kiosk.db")
        conn = sqlite3.connect(str(db_path), check_same_thread=False)
        conn.row_factory = sqlite3.Row
        cur = conn.cursor()

        any_enrolled = False

        for emp_id, img_paths in employee_groups.items():
            logger.info("Found local enrollment images for employee %s (%d file(s))", emp_id, len(img_paths))
            captured_embeddings = []
            primary_avatar_bytes = None

            for img_path in sorted(img_paths)[:5]:  # limit to max 5 images per employee
                try:
                    frame_bgr = cv2.imread(str(img_path))
                    if frame_bgr is None:
                        continue
                    if primary_avatar_bytes is None:
                        _, buf = cv2.imencode(".jpg", frame_bgr)
                        primary_avatar_bytes = buf.tobytes()

                    frame_rgb = cv2.cvtColor(frame_bgr, cv2.COLOR_BGR2RGB)
                    faces = self.detector.detect_faces(frame_rgb)
                    if not faces:
                        h, w = frame_rgb.shape[:2]
                        s = min(h, w)
                        crop = frame_rgb[(h - s) // 2:(h + s) // 2, (w - s) // 2:(w + s) // 2]
                        aligned = cv2.resize(crop, (112, 112))
                    else:
                        best_face = max(faces, key=lambda f: (f["bbox"][2] - f["bbox"][0]) * (f["bbox"][3] - f["bbox"][1]))
                        landmarks = best_face.get("landmarks", [])
                        if len(landmarks) >= 5:
                            aligned = align_face(frame_rgb, np.array(landmarks, dtype=np.float32), 112)
                        else:
                            x1, y1, x2, y2 = best_face["bbox"]
                            h, w = frame_rgb.shape[:2]
                            crop = frame_rgb[max(0, y1):min(h, y2), max(0, x1):min(w, x2)]
                            aligned = cv2.resize(crop, (112, 112))

                    emb = self.detector.extract_embedding(aligned)
                    if emb is not None:
                        captured_embeddings.append(emb)
                except Exception as ex:
                    logger.warning("Error processing image %s for %s: %s", img_path.name, emp_id, ex)

            if not captured_embeddings:
                logger.warning("Could not extract any embeddings for %s, moving files to processed", emp_id)
            else:
                mean_emb = np.mean(captured_embeddings, axis=0)
                norm = np.linalg.norm(mean_emb)
                if norm > 1e-6:
                    mean_emb = mean_emb / norm
                emb_bytes = mean_emb.astype(np.float32).tobytes()
                now_iso = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")

                # Check if user already exists
                cur.execute("SELECT id, name FROM users WHERE employee_id = ?", (emp_id,))
                existing_user = cur.fetchone()
                if existing_user:
                    user_id = existing_user["id"]
                    user_name = existing_user["name"]
                    cur.execute("UPDATE users SET updated_at = ? WHERE id = ?", (now_iso, user_id))
                else:
                    user_id = str(uuid.uuid4())
                    user_name = f"พนักงาน {emp_id}"
                    cur.execute(
                        """
                        INSERT INTO users (id, employee_id, name, department, status, version, updated_at)
                        VALUES (?, ?, ?, 'ฝ่ายทั่วไป', 'active', 0, ?)
                        """,
                        (user_id, emp_id, user_name, now_iso),
                    )

                # Save avatar locally
                if primary_avatar_bytes:
                    avatar_path = avatars_dir / f"{user_id}.jpg"
                    avatar_path.write_bytes(primary_avatar_bytes)
                    cur.execute("UPDATE users SET photo_path = ? WHERE id = ?", (str(avatar_path), user_id))

                # Insert / replace user embedding
                emb_id = str(uuid.uuid4())
                cur.execute("DELETE FROM user_embeddings WHERE user_id = ?", (user_id,))
                cur.execute(
                    """
                    INSERT INTO user_embeddings (id, user_id, embedding, model_version, source, created_at)
                    VALUES (?, ?, ?, 'arcface_mobilefacenet_v1', 'local_folder', ?)
                    """,
                    (emb_id, user_id, emb_bytes, now_iso),
                )

                # Queue for server sync via outbox
                outbox_payload = {
                    "user_id": user_id,
                    "name": user_name,
                    "employee_id": emp_id,
                    "embedding_b64": base64.b64encode(emb_bytes).decode("utf-8"),
                    "photo_b64": base64.b64encode(primary_avatar_bytes).decode("utf-8") if primary_avatar_bytes else "",
                    "quality_score": 1.0,
                }
                cur.execute(
                    """
                    INSERT INTO outbox (event_id, type, payload, created_at, synced, retry_count)
                    VALUES (?, 'user_enrollment', ?, ?, 0, 0)
                    """,
                    (user_id, json.dumps(outbox_payload), now_iso),
                )

                conn.commit()
                any_enrolled = True
                logger.info("✅ Enrolled employee %s (%s) from local folder with %d image(s)", emp_id, user_name, len(captured_embeddings))

            # Move processed images to processed directory
            target_dest_dir = processed_dir / emp_id
            target_dest_dir.mkdir(parents=True, exist_ok=True)
            for img_p in img_paths:
                try:
                    dest_file = target_dest_dir / img_p.name
                    if dest_file.exists():
                        dest_file.unlink()
                    shutil.move(str(img_p), str(dest_file))
                except Exception as move_err:
                    logger.debug("Failed moving %s: %s", img_p, move_err)

            # If it was an empty subfolder, remove it
            subfolder = enroll_dir / emp_id
            if subfolder.is_dir() and not any(subfolder.iterdir()):
                try:
                    subfolder.rmdir()
                except Exception:
                    pass

        conn.close()

        if any_enrolled and self.on_enrollment_done:
            self.on_enrollment_done()
