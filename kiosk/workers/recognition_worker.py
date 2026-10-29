"""
Recognition Worker Thread — runs real-time pipeline and emits PyQt5 signals.
"""
import logging
import cv2
import numpy as np
from PyQt5.QtCore import QThread, pyqtSignal
from PyQt5.QtGui import QImage

from alignment import align_face
from matcher import find_best_match
from metrics import MetricsTracker
from anti_spoof import AntiSpoofDetector

logger = logging.getLogger(__name__)


def draw_corner_brackets(img, x1, y1, x2, y2, color=(255, 255, 255), thickness=2, corner_len=18):
    """Draw 4 corner brackets around the face box instead of a full rectangle."""
    w = x2 - x1
    h = y2 - y1
    cl = max(8, min(corner_len, w // 4, h // 4))
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


class RecognitionWorker(QThread):
    """Camera capture and face recognition worker thread."""
    frame_ready = pyqtSignal(QImage)
    face_recognized = pyqtSignal(str, float, str, str, str)
    unknown_detected = pyqtSignal()
    spoof_detected = pyqtSignal()

    def __init__(self, camera, detector, database, config, current_direction="AUTO"):
        super().__init__()
        self.camera = camera
        self.detector = detector
        self.db = database
        self.config = config
        self.current_direction = current_direction
        self._running = True

        # Initialize Passive Anti-Spoofing Detector (MiniFASNetV2)
        if getattr(config, "anti_spoof_enabled", True):
            model_p = getattr(config, "anti_spoof_model_path", None)
            thresh = getattr(config, "anti_spoof_threshold", 0.85)
            self.anti_spoof = AntiSpoofDetector(model_path=model_p, threshold=thresh)
        else:
            self.anti_spoof = None

        self.gallery_cache = []
        self.reload_gallery()

    def reload_gallery(self):
        """Reload user embeddings from database into RAM."""
        active = self.db.load_active_gallery()
        self.gallery_cache = [(u["user_id"], u["embedding"]) for u in active]
        self.user_lookup = {u["user_id"]: u for u in active}

    def set_direction(self, direction):
        self.current_direction = direction

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

                    # 1. Passive Anti-Spoofing Check (MiniFASNetV2)
                    if self.anti_spoof and self.anti_spoof.enabled:
                        is_real, spoof_score = self.anti_spoof.check_liveness(frame_rgb, [x1, y1, x2, y2])
                        if not is_real:
                            # ป้องกันภาพถ่าย/หน้าจอมือถือแบบเงียบๆ ไม่แสดงข้อความกวนสายตา และไม่บันทึกเวลา
                            draw_corner_brackets(annotated, x1, y1, x2, y2, color=(255, 255, 255), thickness=2)
                            continue

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

                        cv2.putText(annotated, name, (x1, max(20, y1 - 8)), cv2.FONT_HERSHEY_SIMPLEX, 0.55, (255, 255, 255), 2)

                        rec = self.db.record_attendance(
                            user_id,
                            conf,
                            self.current_direction,
                            self.config.device_id,
                            self.config.debounce_seconds,
                        )
                        if rec:
                            self.face_recognized.emit(name, conf, rec["direction"], emp_id, dept)
                    else:
                        self.unknown_detected.emit()

                h, w, ch = annotated.shape
                qimg = QImage(annotated.data, w, h, ch * w, QImage.Format_RGB888).copy()
                self.frame_ready.emit(qimg)

            except Exception as e:
                logger.error("Recognition worker error: %s", e)

            self.msleep(30)

    def stop(self):
        self._running = False
        self.wait()
