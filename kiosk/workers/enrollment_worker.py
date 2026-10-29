"""
Enrollment Worker Thread — pulls pending enrollment jobs from Server,
computes 512-d Face Embedding on Hailo-8 (or CPU Fallback),
and submits the result back to Server.
"""
import time
import base64
import logging
import threading
from pathlib import Path
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

            # Check every 4 seconds
            for _ in range(8):
                if not self._running:
                    break
                time.sleep(0.5)

    def _check_and_process_job(self):
        claim_url = f"{self.config.server_url.rstrip('/')}/sync/enrollment/claim"
        headers = {"X-API-Key": self.config.api_key}

        try:
            resp = requests.get(claim_url, headers=headers, timeout=5, verify=self.config.ssl_verify)
            if resp.status_code != 200:
                return
            data = resp.json()
            job = data.get("job")
            if not job:
                return

            job_id = job["id"]
            user_id = job["user_id"]
            images_b64 = job.get("images_b64") or ([job["image_b64"]] if job.get("image_b64") else [])

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

            if not captured_embeddings:
                logger.error("Could not extract any embeddings for job %s", job_id)
                return

            # Compute average embedding (mean vector) and normalize with L2 norm
            mean_emb = np.mean(captured_embeddings, axis=0)
            norm = np.linalg.norm(mean_emb)
            if norm > 1e-6:
                mean_emb = mean_emb / norm

            emb_bytes = mean_emb.astype(np.float32).tobytes()
            emb_b64 = base64.b64encode(emb_bytes).decode("utf-8")

            # Submit result to Server
            result_url = f"{self.config.server_url.rstrip('/')}/api/enrollment/{job_id}/result"
            payload = {
                "embedding_b64": emb_b64,
                "quality_score": 1.0,
            }
            res_post = requests.post(result_url, json=payload, headers={"X-API-Key": self.config.api_key}, timeout=5, verify=self.config.ssl_verify)
            if res_post.status_code == 200:
                logger.info("Successfully completed enrollment job %s from %d image(s)! Embedding stored on server.", job_id, len(captured_embeddings))
                if self.on_enrollment_done:
                    self.on_enrollment_done()
            else:
                logger.error("Failed to submit enrollment result: HTTP %d %s", res_post.status_code, res_post.text)

        except Exception as e:
            logger.error("Error executing enrollment job: %s", e)
