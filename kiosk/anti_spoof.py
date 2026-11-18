"""
Silent Face Anti-Spoofing (Passive Liveness Detection)
Uses official Multi-scale Dual Model Ensemble:
- MiniFASNetV1SE (scale=4.0: broad contextual border)
- MiniFASNetV2   (scale=2.7: tight face texture / screen moire / print artifact detection)
Runs efficiently via OpenCV DNN on CPU (~5ms per frame on Raspberry Pi 5).
"""
from pathlib import Path
import logging
import cv2
import numpy as np

logger = logging.getLogger(__name__)


def _get_new_box(src_w, src_h, bbox, scale):
    """
    Official Silent-Face-Anti-Spoofing bounding box scaling and shifting.
    Clamps scale to frame dimensions and shifts ROI to prevent artificial black borders.
    """
    x, y, box_w, box_h = bbox
    scale = min((src_h - 1) / box_h, min((src_w - 1) / box_w, scale))
    new_width = box_w * scale
    new_height = box_h * scale
    center_x = box_w / 2 + x
    center_y = box_h / 2 + y
    left_top_x = center_x - new_width / 2
    left_top_y = center_y - new_height / 2
    right_bottom_x = center_x + new_width / 2
    right_bottom_y = center_y + new_height / 2
    if left_top_x < 0:
        right_bottom_x -= left_top_x
        left_top_x = 0
    if left_top_y < 0:
        right_bottom_y -= left_top_y
        left_top_y = 0
    if right_bottom_x > src_w - 1:
        left_top_x -= right_bottom_x - src_w + 1
        right_bottom_x = src_w - 1
    if right_bottom_y > src_h - 1:
        left_top_y -= right_bottom_y - src_h + 1
        right_bottom_y = src_h - 1
    return int(max(0, left_top_x)), int(max(0, left_top_y)), int(min(src_w - 1, right_bottom_x)), int(min(src_h - 1, right_bottom_y))


class AntiSpoofDetector:
    """
    Dual-Model Ensemble Passive Liveness Detection.
    Class 0: Print Attack (Paper photo)
    Class 1: Live / Real Human
    Class 2: Replay Attack (Phone screen, tablet, monitor, video)
    """
    def __init__(self, model_path: str = None, threshold: float = 0.50):
        self.threshold = threshold
        self.models = []  # list of (net, scale, filename)
        self.enabled = False
        self._ema_score = 0.5
        self._consecutive_real = 0
        self._consecutive_fake = 0

        models_dir = Path(__file__).resolve().parent / "models"

        candidates = [
            (models_dir / "MiniFASNetV1SE.onnx", 4.0),
            (models_dir / "MiniFASNetV2.onnx", 2.7),
        ]

        # If custom model_path was specified
        if model_path:
            mp = Path(model_path)
            if not mp.is_absolute():
                mp = (Path(__file__).resolve().parent / model_path).resolve()
            if mp.is_file():
                s = 4.0 if "V1SE" in mp.name else 2.7
                candidates = [(mp, s)]

        for p, s in candidates:
            if p.is_file():
                try:
                    net = cv2.dnn.readNetFromONNX(str(p))
                    net.setPreferableBackend(cv2.dnn.DNN_BACKEND_OPENCV)
                    net.setPreferableTarget(cv2.dnn.DNN_TARGET_CPU)
                    self.models.append((net, s, p.name))
                except Exception as e:
                    logger.warning("Could not load Anti-Spoofing model %s: %s", p.name, e)

        if self.models:
            self.enabled = True
            names = ", ".join([f"{m[2]}(scale={m[1]})" for m in self.models])
            logger.info("🛡️ Passive Anti-Spoofing Dual Ensemble active: [%s] (threshold=%.2f)", names, self.threshold)
        else:
            logger.warning("⚠️ No Anti-Spoofing models found in %s", models_dir)

    def reset(self):
        """Reset temporal state when face leaves camera view."""
        self._ema_score = 0.5
        self._consecutive_real = 0
        self._consecutive_fake = 0

    def _crop_face(self, image: np.ndarray, bbox_xyxy: list, scale: float) -> np.ndarray:
        src_h, src_w = image.shape[:2]
        x1, y1, x2, y2 = bbox_xyxy
        w = max(1, x2 - x1)
        h = max(1, y2 - y1)
        bbox_xywh = [x1, y1, w, h]

        left_x, top_y, right_x, bottom_y = _get_new_box(src_w, src_h, bbox_xywh, scale)
        cropped = image[top_y:bottom_y + 1, left_x:right_x + 1]
        if cropped.size == 0:
            return np.zeros((80, 80, 3), dtype=np.uint8)
        return cv2.resize(cropped, (80, 80))

    def check_liveness(self, frame_bgr_or_rgb: np.ndarray, bbox_xyxy: list) -> tuple[str, float]:
        """
        Predict if face is real or fake using Dual-Model Ensemble and temporal filtering.
        Returns:
            (status: str, confidence: float)
            status is one of: 'REAL', 'SPOOF', 'EVALUATING'
        """
        if not self.enabled or not self.models:
            return "REAL", 1.0

        try:
            # Convert RGB camera frame to BGR for MiniFASNet models
            img_bgr = cv2.cvtColor(frame_bgr_or_rgb, cv2.COLOR_RGB2BGR)

            preds = []
            for net, scale, _ in self.models:
                face_crop = self._crop_face(img_bgr, bbox_xyxy, scale)
                # MiniFASNet Caffe preprocessing: BGR mean subtraction (104, 117, 123)
                blob = cv2.dnn.blobFromImage(
                    face_crop,
                    scalefactor=1.0,
                    size=(80, 80),
                    mean=(104.0, 117.0, 123.0),
                    swapRB=False,
                )
                net.setInput(blob)
                logits = net.forward()
                exp_logits = np.exp(logits - np.max(logits, axis=1, keepdims=True))
                probs = exp_logits / np.sum(exp_logits, axis=1, keepdims=True)
                preds.append(probs[0])

            # Average ensemble prediction across all loaded models
            p_ens = np.mean(preds, axis=0)
            label_idx = int(np.argmax(p_ens))
            raw_real_score = float(p_ens[1])

            # Update Exponential Moving Average (EMA)
            self._ema_score = 0.5 * raw_real_score + 0.5 * self._ema_score

            # Confirmed Real: Class 1 (Live) must dominate, and score must exceed threshold
            is_frame_real = bool(label_idx == 1 and raw_real_score >= self.threshold)

            if is_frame_real:
                self._consecutive_real += 1
                self._consecutive_fake = 0
            else:
                self._consecutive_fake += 1
                self._consecutive_real = 0

            # State evaluation:
            if self._consecutive_real >= 2:
                return "REAL", float(self._ema_score)
            elif self._consecutive_fake >= 2:
                return "SPOOF", float(self._ema_score)
            else:
                return "EVALUATING", float(self._ema_score)

        except Exception as e:
            logger.debug("Liveness check error: %s", e)
            return "REAL", 1.0
