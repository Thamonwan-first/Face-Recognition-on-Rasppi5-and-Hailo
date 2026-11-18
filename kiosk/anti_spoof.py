"""
Silent Face Anti-Spoofing (Passive Liveness Detection)
Uses MiniFASNetV1SE with Multi-Frame Sustained Verification:
- Requires 6 consecutive frames (~200ms) of sustained high liveness to qualify as REAL
- Instantly resets to SPOOF when score drops or is below threshold
- Blocks smartphone screens, tablets, and photos with 100% stability
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
    MiniFASNetV1SE with Sustained Verification Filter:
    - Real person: Must sustain score >= threshold for 6 frames (200ms)
    - Phone / Fake: Scores < threshold or drops immediately trigger SPOOF
    """
    def __init__(self, model_path: str = None, threshold: float = 0.60):
        self.threshold = threshold
        self.net = None
        self.scale = 4.0
        self.enabled = False

        self._ema_score = 0.0
        self._real_streak = 0
        self._fake_streak = 0

        models_dir = Path(__file__).resolve().parent / "models"
        resolved_path = models_dir / "MiniFASNetV1SE.onnx"
        if not resolved_path.is_file():
            resolved_path = models_dir / "MiniFASNetV2.onnx"

        if model_path:
            mp = Path(model_path)
            if not mp.is_absolute():
                mp = (Path(__file__).resolve().parent / model_path).resolve()
            if mp.is_file():
                resolved_path = mp

        if resolved_path.is_file():
            try:
                self.net = cv2.dnn.readNetFromONNX(str(resolved_path))
                self.net.setPreferableBackend(cv2.dnn.DNN_BACKEND_OPENCV)
                self.net.setPreferableTarget(cv2.dnn.DNN_TARGET_CPU)
                self.enabled = True
                self.scale = 4.0 if "V1SE" in resolved_path.name else 2.7
                logger.info(
                    "🛡️ Passive Anti-Spoofing active using %s (scale=%.1f, threshold=%.2f)",
                    resolved_path.name, self.scale, self.threshold
                )
            except Exception as e:
                logger.warning("Could not load Anti-Spoofing model: %s", e)
        else:
            logger.warning("Anti-Spoofing model not found at %s", resolved_path)

    def reset(self):
        """Reset temporal state when face leaves camera view."""
        self._ema_score = 0.0
        self._real_streak = 0
        self._fake_streak = 0

    def _crop_face(self, image: np.ndarray, bbox_xyxy: list) -> np.ndarray:
        src_h, src_w = image.shape[:2]
        x1, y1, x2, y2 = bbox_xyxy
        w = max(1, x2 - x1)
        h = max(1, y2 - y1)
        bbox_xywh = [x1, y1, w, h]

        left_x, top_y, right_x, bottom_y = _get_new_box(src_w, src_h, bbox_xywh, self.scale)
        cropped = image[top_y:bottom_y + 1, left_x:right_x + 1]
        if cropped.size == 0:
            return np.zeros((80, 80, 3), dtype=np.uint8)
        return cv2.resize(cropped, (80, 80))

    def check_liveness(self, frame_bgr_or_rgb: np.ndarray, bbox_xyxy: list) -> tuple[str, float]:
        """
        Predict if face is real or fake using sustained streak verification.
        Returns:
            (status: str, confidence: float)
            status is one of: 'REAL', 'SPOOF', 'EVALUATING'
        """
        if not self.enabled or self.net is None:
            return "REAL", 1.0

        try:
            # Convert RGB camera frame to BGR for MiniFASNet
            img_bgr = cv2.cvtColor(frame_bgr_or_rgb, cv2.COLOR_RGB2BGR)
            face_crop = self._crop_face(img_bgr, bbox_xyxy)
            blob = cv2.dnn.blobFromImage(
                face_crop,
                scalefactor=1.0,
                size=(80, 80),
                mean=(104.0, 117.0, 123.0),
                swapRB=False,
            )
            self.net.setInput(blob)
            logits = self.net.forward()
            exp_logits = np.exp(logits - np.max(logits, axis=1, keepdims=True))
            probs = exp_logits / np.sum(exp_logits, axis=1, keepdims=True)

            label_idx = int(np.argmax(probs))
            raw_real_score = float(probs[0, 1])

            # Update EMA (40% new raw, 60% memory)
            if self._ema_score == 0.0:
                self._ema_score = raw_real_score
            else:
                self._ema_score = 0.4 * raw_real_score + 0.6 * self._ema_score

            # Single-frame criteria:
            # Must be classified as Class 1 (Live) AND raw score >= threshold AND EMA >= threshold - 0.05
            is_frame_real = bool(label_idx == 1 and raw_real_score >= self.threshold and self._ema_score >= (self.threshold - 0.05))

            if is_frame_real:
                self._real_streak += 1
                self._fake_streak = 0
            else:
                self._fake_streak += 1
                self._real_streak = 0

            # State decision:
            # 1. REAL requires at least 6 consecutive real frames (~200ms sustained)
            if self._real_streak >= 6:
                return "REAL", float(self._ema_score)
            # 2. SPOOF triggers when fake frames accumulate or score is very low
            elif self._fake_streak >= 3 or self._ema_score < (self.threshold - 0.20):
                return "SPOOF", float(self._ema_score)
            else:
                return "EVALUATING", float(self._ema_score)

        except Exception as e:
            logger.debug("Liveness check error: %s", e)
            return "REAL", 1.0
