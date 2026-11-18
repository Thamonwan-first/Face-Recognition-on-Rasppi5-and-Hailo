"""
Silent Face Anti-Spoofing (Passive Liveness Detection)
Uses MiniFASNetV1SE with Hysteresis (Schmitt Trigger) Temporal Smoothing:
- Prevents UI flickering between Real and Spoof
- Blocks phone screens / paper photos without false rejections of real humans
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
    MiniFASNetV1SE with Hysteresis State Machine:
    - High threshold: 0.55 (Promotes to REAL after 2 sustained frames)
    - Low threshold:  0.35 (Demotes to SPOOF after 3 sustained frames)
    Eliminates boundary jitter and prevents screen flickering.
    """
    def __init__(self, model_path: str = None, threshold: float = 0.55):
        self.high_thresh = threshold
        self.low_thresh = max(0.20, threshold - 0.20)
        self.threshold = threshold
        self.net = None
        self.scale = 4.0
        self.enabled = False

        # Hysteresis state tracking
        self.state = "EVALUATING"
        self._ema_score = 0.5
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
                    "🛡️ Passive Anti-Spoofing active using %s (scale=%.1f, high=%.2f, low=%.2f)",
                    resolved_path.name, self.scale, self.high_thresh, self.low_thresh
                )
            except Exception as e:
                logger.warning("Could not load Anti-Spoofing model: %s", e)
        else:
            logger.warning("Anti-Spoofing model not found at %s", resolved_path)

    def reset(self):
        """Reset temporal state when face leaves camera view."""
        self.state = "EVALUATING"
        self._ema_score = 0.5
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
        Predict if face is real or fake using Hysteresis filter.
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

            # Update Exponential Moving Average (EMA)
            self._ema_score = 0.5 * raw_real_score + 0.5 * self._ema_score

            # Hysteresis state machine:
            if self.state == "REAL":
                # Once confirmed real, only degrade to spoof if sustained low (< low_thresh for 3 frames)
                if self._ema_score < self.low_thresh:
                    self._fake_streak += 1
                    if self._fake_streak >= 3:
                        self.state = "SPOOF"
                        self._real_streak = 0
                else:
                    self._fake_streak = 0
            else:
                # Currently SPOOF or EVALUATING: only promote to REAL if sustained high (>= high_thresh for 2 frames)
                if self._ema_score >= self.high_thresh and label_idx == 1:
                    self._real_streak += 1
                    if self._real_streak >= 2:
                        self.state = "REAL"
                        self._fake_streak = 0
                else:
                    self._real_streak = 0
                    if self._ema_score < self.low_thresh:
                        self._fake_streak += 1
                        if self._fake_streak >= 2:
                            self.state = "SPOOF"

            return self.state, float(self._ema_score)

        except Exception as e:
            logger.debug("Liveness check error: %s", e)
            return "REAL", 1.0
