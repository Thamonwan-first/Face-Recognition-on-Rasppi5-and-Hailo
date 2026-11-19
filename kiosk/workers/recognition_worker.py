"""
Recognition Worker Thread — runs real-time pipeline and emits PyQt5 signals.
"""
import time
import base64
import logging
from pathlib import Path
import cv2
import numpy as np
from PyQt5.QtCore import QThread, pyqtSignal
from PyQt5.QtGui import QImage

from alignment import align_face
from matcher import find_best_match
from metrics import MetricsTracker
from anti_spoof import AntiSpoofDetector

logger = logging.getLogger(__name__)


def draw_corner_brackets(img, x1, y1, x2, y2, color=(255, 255, 255), thickness=2, corner_len=24):
    """Draw sleek minimalist corner brackets at the 4 face corners."""
    h_img, w_img = img.shape[:2]
    x1, y1 = max(0, int(x1)), max(0, int(y1))
    x2, y2 = min(w_img - 1, int(x2)), min(h_img - 1, int(y2))

    w = x2 - x1
    h = y2 - y1
    cl = max(12, min(corner_len, w // 4, h // 4))

    # Top-Left
    cv2.line(img, (x1, y1), (x1 + cl, y1), color, thickness)
    cv2.line(img, (x1, y1), (x1, y1 + cl), color, thickness)
    # Top-Right
    cv2.line(img, (x2, y1), (x2 - cl, y1), color, thickness)
    cv2.line(img, (x2, y1), (x2, y1 + cl), color, thickness)
    # Bottom-Left
    cv2.line(img, (x1, y2), (x1 + cl, y2), color, thickness)
    cv2.line(img, (x1, y2), (x1, y2 - cl), color, thickness)
    # Bottom-Right
    cv2.line(img, (x2, y2), (x2 - cl, y2), color, thickness)
    cv2.line(img, (x2, y2), (x2, y2 - cl), color, thickness)


def crop_face_snapshot_b64(frame_rgb, bbox) -> str:
    """Crop and encode 250x250 face snapshot only when an event is actually recorded."""
    try:
        x1, y1, x2, y2 = bbox
        h_frame, w_frame = frame_rgb.shape[:2]
        bw, bh = x2 - x1, y2 - y1
        pad_w, pad_h = int(bw * 0.15), int(bh * 0.15)
        cx1 = max(0, int(x1 - pad_w))
        cy1 = max(0, int(y1 - pad_h))
        cx2 = min(w_frame, int(x2 + pad_w))
        cy2 = min(h_frame, int(y2 + pad_h))
        face_crop = frame_rgb[cy1:cy2, cx1:cx2]
        if face_crop.size > 0:
            face_resized = cv2.resize(face_crop, (250, 250))
            face_bgr = cv2.cvtColor(face_resized, cv2.COLOR_RGB2BGR)
            _, buf = cv2.imencode('.jpg', face_bgr, [int(cv2.IMWRITE_JPEG_QUALITY), 80])
            return base64.b64encode(buf).decode('utf-8')
    except Exception as crop_err:
        logger.debug("Failed creating face snapshot: %s", crop_err)
    return ""


class RecognitionWorker(QThread):
    """Camera capture and face recognition worker thread."""
    frame_ready = pyqtSignal(QImage)
    face_recognized = pyqtSignal(str, float, str, str, str, str, bool)
    unknown_detected = pyqtSignal()
    spoof_detected = pyqtSignal()

    def __init__(self, camera, detector, database, config, current_direction="AUTO", on_attendance_recorded=None):
        super().__init__()
        self.camera = camera
        self.detector = detector
        self.db = database
        self.config = config
        self.current_direction = current_direction
        self.on_attendance_recorded = on_attendance_recorded
        self._running = True

        # Initialize Passive Anti-Spoofing Detector (MiniFASNetV1SE)
        if getattr(config, "anti_spoof_enabled", True):
            model_p = getattr(config, "anti_spoof_model", getattr(config, "anti_spoof_model_path", None))
            thresh = getattr(config, "anti_spoof_threshold", 0.60)
            self.anti_spoof = AntiSpoofDetector(model_path=model_p, threshold=thresh)
        else:
            self.anti_spoof = None

        self.gallery_cache = []
        self._recent_scans = {}
        self.reload_gallery()

    def reload_gallery(self):
        """Reload user embeddings from database into RAM with pre-stacked matrix for O(1) matching overhead."""
        active = self.db.load_active_gallery()
        user_ids = [u["user_id"] for u in active]
        if active:
            matrix = np.asarray([u["embedding"] for u in active], dtype=np.float32)
            self.gallery_cache = (user_ids, matrix)
        else:
            self.gallery_cache = ([], None)
        self.user_lookup = {u["user_id"]: u for u in active}
        logger.info("Loaded %d face embedding(s) into active recognition gallery", len(user_ids))

    def set_direction(self, direction):
        self.current_direction = direction

    def _is_clock_synced(self) -> bool:
        """Verify if system clock is synchronized via RTC, NTP, or valid modern epoch."""
        if getattr(self.config, "has_rtc", False) or Path("/dev/rtc").exists() or Path("/dev/rtc0").exists():
            return True
        try:
            from datetime import datetime
            return datetime.now().year >= 2024
        except Exception:
            return False

    def run(self):
        while self._running:
            try:
                frame_rgb = self.camera.capture()
                if frame_rgb is None:
                    self.msleep(10)
                    continue

                MetricsTracker.record_frame()

                faces = self.detector.detect_faces(frame_rgb)
                annotated = frame_rgb.copy()

                for f in faces:
                    x1, y1, x2, y2 = f["bbox"]
                    landmarks = f.get("landmarks", [])

                    # 1. Passive Anti-Spoofing Check (Dual Ensemble)
                    if self.anti_spoof and self.anti_spoof.enabled:
                        liveness_status, spoof_score = self.anti_spoof.check_liveness(frame_rgb, [x1, y1, x2, y2])
                        if liveness_status == "SPOOF":
                            # Reset unknown face accumulation so phone screens never trigger upload to dashboard
                            self._unknown_face_start = None

                            self.spoof_detected.emit()

                            # Rate-limit log output so terminal is not flooded
                            now_t = time.time()
                            if now_t - getattr(self, '_last_spoof_log', 0) >= 1.5:
                                self._last_spoof_log = now_t
                                logger.info("🛡️ ตรวจพบ Anti-Spoof รูปถ่าย/หน้าจอ (ความมั่นใจคนจริง: %.2f / เกณฑ์: %.2f)", spoof_score, self.anti_spoof.threshold)

                            # วาดกรอบสีส้ม-แดงแจ้งเตือนว่าเป็นภาพปลอม และข้ามการบันทึกเวลา
                            draw_corner_brackets(annotated, x1, y1, x2, y2, color=(255, 69, 0), thickness=2)
                            cv2.putText(annotated, "Spoof Detected", (x1, max(20, y1 - 8)), cv2.FONT_HERSHEY_SIMPLEX, 0.55, (255, 69, 0), 2)
                            continue
                        elif liveness_status == "EVALUATING":
                            # Multi-frame warmup: draw clean white brackets and continue evaluating (takes ~30-60ms)
                            draw_corner_brackets(annotated, x1, y1, x2, y2, color=(255, 255, 255), thickness=1)
                            continue
                        elif not self.anti_spoof.is_recognition_allowed():
                            draw_corner_brackets(annotated, x1, y1, x2, y2, color=(255, 255, 255), thickness=1)
                            continue
                    else:
                        if not getattr(self, '_anti_spoof_warned', False):
                            logger.warning("⚠️ Anti-Spoofing model is NOT loaded (file missing or disabled) — checking skipped!")
                            self._anti_spoof_warned = True

                    # 2. 5-point alignment & ArcFace embedding
                    if len(landmarks) >= 5:
                        aligned = align_face(frame_rgb, np.array(landmarks, dtype=np.float32), 112)
                    else:
                        h, w = frame_rgb.shape[:2]
                        crop = frame_rgb[max(0, y1):min(h, y2), max(0, x1):min(w, x2)]
                        if crop.size == 0:
                            continue
                        aligned = cv2.resize(crop, (112, 112))

                    emb = self.detector.extract_embedding(aligned)
                    match = find_best_match(emb, self.gallery_cache, self.config.recognition_threshold)

                    # 3. Real Person: Draw normal white corner brackets
                    draw_corner_brackets(annotated, x1, y1, x2, y2, color=(255, 255, 255), thickness=2)

                    if match:
                        user_id, conf = match
                        user_info = self.user_lookup.get(user_id, {})
                        name = user_info.get("name", "Unknown")
                        emp_id = user_info.get("employee_id", "")
                        dept = user_info.get("department", "")
                        photo_path = user_info.get("photo_path", "")
                        user_status = user_info.get("status", "active")
                        end_date_str = user_info.get("end_date")

                        # ตรวจสอบการหมดอายุสัญญาจ้าง
                        is_contract_expired = False
                        if user_status != "active":
                            is_contract_expired = True
                        elif end_date_str:
                            try:
                                from datetime import date
                                end_d = date.fromisoformat(end_date_str[:10])
                                if date.today() > end_d:
                                    is_contract_expired = True
                            except Exception:
                                pass

                        if is_contract_expired:
                            draw_corner_brackets(annotated, x1, y1, x2, y2, color=(0, 0, 255), thickness=2)

                        # Fast in-memory debounce check before querying SQLite in 30fps loop
                        now = time.time()
                        last_scan = self._recent_scans.get(user_id, 0)
                        if (now - last_scan >= self.config.debounce_seconds) and not self.db.is_debounced(user_id, self.config.debounce_seconds):
                            snapshot_b64 = crop_face_snapshot_b64(frame_rgb, (x1, y1, x2, y2))

                            if is_contract_expired:
                                # พนักงานหมดอายุสัญญาจ้าง: ส่งภาพถ่ายและข้อมูลเข้าเว็บ แต่ไม่บันทึกเวลาเข้า-ออก
                                rec = self.db.record_attendance(
                                    user_id,
                                    conf,
                                    "EXPIRED",
                                    self.config.device_id,
                                    self.config.debounce_seconds,
                                    clock_synced=self._is_clock_synced(),
                                    snapshot_b64=snapshot_b64,
                                    notes="หมดอายุสัญญาจ้าง",
                                )
                                self._recent_scans[user_id] = now
                                if len(self._recent_scans) > 50:
                                    cutoff = now - (2 * self.config.debounce_seconds)
                                    self._recent_scans = {uid: t for uid, t in self._recent_scans.items() if t > cutoff}

                                self.face_recognized.emit(name, conf, "EXPIRED", emp_id, dept, photo_path, True)
                                logger.warning("⚠️ พนักงาน %s (%s) หมดอายุสัญญาจ้าง — ส่งข้อมูลเข้าเว็บโดยไม่บันทึกเวลาเข้า-ออก (หมายเหตุ: หมดอายุสัญญาจ้าง)", name, emp_id)
                                if self.on_attendance_recorded:
                                    try:
                                        self.on_attendance_recorded()
                                    except Exception as cb_err:
                                        logger.debug("on_attendance_recorded failed: %s", cb_err)
                            else:
                                rec = self.db.record_attendance(
                                    user_id,
                                    conf,
                                    self.current_direction,
                                    self.config.device_id,
                                    self.config.debounce_seconds,
                                    clock_synced=self._is_clock_synced(),
                                    snapshot_b64=snapshot_b64,
                                    notes="",
                                )
                                if rec:
                                    logger.info("✅ บันทึกเวลาสำเร็จ: %s (%s) [%s] (ความมั่นใจ: %.2f)", name, emp_id, rec["direction"], conf)
                                    self._recent_scans[user_id] = now
                                    # Bound memory: purge stale entries older than 2x debounce window
                                    if len(self._recent_scans) > 50:
                                        cutoff = now - (2 * self.config.debounce_seconds)
                                        self._recent_scans = {uid: t for uid, t in self._recent_scans.items() if t > cutoff}
                                    self.face_recognized.emit(name, conf, rec["direction"], emp_id, dept, photo_path, False)
                                    if self.on_attendance_recorded:
                                        try:
                                            self.on_attendance_recorded()
                                        except Exception as cb_err:
                                            logger.debug("on_attendance_recorded failed: %s", cb_err)
                        else:
                            # Debounced: user was already recorded within debounce_seconds
                            now_t = time.time()
                            if now_t - getattr(self, f'_last_debounced_log_{user_id}', 0) >= 3.0:
                                setattr(self, f'_last_debounced_log_{user_id}', now_t)
                                remaining = int(self.config.debounce_seconds - (now - last_scan))
                                logger.info("⏳ พนักงาน %s เพิ่งบันทึกเวลาไปแล้ว (ระบบป้องกันการสแกนซ้ำ อีก %d วินาที)", name, max(0, remaining))
                    else:
                        now = time.time()
                        if not hasattr(self, '_unknown_face_start') or self._unknown_face_start is None:
                            self._unknown_face_start = now
                            self._unknown_last_seen = now
                        else:
                            if now - self._unknown_last_seen < 0.5:
                                self._unknown_last_seen = now
                                if (now - self._unknown_face_start >= 1.0) and (now - getattr(self, '_last_unrec_push_time', 0) >= 15.0):
                                    self._last_unrec_push_time = now
                                    # Only crop & encode snapshot JPEG when unknown face actually triggers
                                    snap_b64 = crop_face_snapshot_b64(frame_rgb, (x1, y1, x2, y2))
                                    if snap_b64:
                                        try:
                                            self.db.record_unrecognized(
                                                device_id=self.config.device_id,
                                                confidence=0.0,
                                                snapshot_b64=snap_b64,
                                            )
                                            logger.info("Recorded unrecognized face scan for dashboard review")
                                        except Exception as ur_err:
                                            logger.debug("Failed recording unrecognized scan: %s", ur_err)
                            else:
                                self._unknown_face_start = now
                                self._unknown_last_seen = now
                        self.unknown_detected.emit()

                if not faces:
                    self._unknown_face_start = None
                    if self.anti_spoof:
                        self.anti_spoof.reset()

                h, w, ch = annotated.shape
                qimg = QImage(annotated.data, w, h, ch * w, QImage.Format_RGB888).copy()
                self.frame_ready.emit(qimg)

            except Exception as e:
                logger.error("Recognition worker error: %s", e)

            self.msleep(30)

    def stop(self):
        self._running = False
        self.wait()
