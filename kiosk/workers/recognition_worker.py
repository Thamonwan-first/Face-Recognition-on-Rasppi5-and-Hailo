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

logger = logging.getLogger(__name__)


class RecognitionWorker(QThread):
    """Camera capture and face recognition worker thread."""
    frame_ready = pyqtSignal(QImage)
    face_recognized = pyqtSignal(str, float, str)
    unknown_detected = pyqtSignal()

    def __init__(self, camera, detector, database, config, current_direction="AUTO"):
        super().__init__()
        self.camera = camera
        self.detector = detector
        self.db = database
        self.config = config
        self.current_direction = current_direction
        self._running = True

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

                faces = self.detector.detect_faces(frame_rgb)
                annotated = frame_rgb.copy()

                for f in faces:
                    x1, y1, x2, y2 = f["bbox"]
                    landmarks = f.get("landmarks", [])

                    # 5-point alignment & ArcFace embedding
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

                    if match:
                        user_id, conf = match
                        user_info = self.user_lookup.get(user_id, {})
                        name = user_info.get("name", "Unknown")

                        # Annotate Green
                        cv2.rectangle(annotated, (x1, y1), (x2, y2), (0, 184, 148), 2)
                        cv2.putText(annotated, f"{name} ({conf:.0%})", (x1, max(20, y1 - 8)), cv2.FONT_HERSHEY_SIMPLEX, 0.55, (0, 184, 148), 2)

                        rec = self.db.record_attendance(
                            user_id,
                            conf,
                            self.current_direction,
                            self.config.device_id,
                            self.config.debounce_seconds,
                        )
                        if rec:
                            self.face_recognized.emit(name, conf, rec["direction"])
                    else:
                        cv2.rectangle(annotated, (x1, y1), (x2, y2), (214, 48, 49), 2)
                        cv2.putText(annotated, "Unknown", (x1, max(20, y1 - 8)), cv2.FONT_HERSHEY_SIMPLEX, 0.55, (214, 48, 49), 2)
                        self.unknown_detected.emit()

                    for pt in landmarks:
                        cv2.circle(annotated, (int(pt[0]), int(pt[1])), 3, (9, 132, 227), -1)

                h, w, ch = annotated.shape
                qimg = QImage(annotated.data, w, h, ch * w, QImage.Format_RGB888).copy()
                self.frame_ready.emit(qimg)

            except Exception as e:
                logger.error("Recognition worker error: %s", e)

            self.msleep(30)

    def stop(self):
        self._running = False
        self.wait()
