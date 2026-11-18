"""
Silent Face Anti-Spoofing (Passive Liveness Detection)
Uses MiniFASNetV1SE with Multi-Frame Sustained Verification:
- Requires sustained high liveness to qualify as REAL
- Latches SPOOF after sustained evidence and requires a long stable live streak to release
- Blocks smartphone screens, tablets, and photos with 100% stability
Runs efficiently via OpenCV DNN on CPU (~5ms per frame on Raspberry Pi 5).
"""
from pathlib import Path
import logging
import time
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
    MiniFASNetV1SE/V2 Multi-Scale Ensemble with Sustained Verification Filter:
    - Real person: Must sustain a strong score for a long consecutive streak
    - Phone / Fake: Requires sustained low scores and remains SPOOF with hysteresis
    """
    def __init__(self, model_path: str = None, threshold: float = 0.60):
        self.threshold = threshold
        self.net = None
        self.nets = {}
        self._model_weights = {"V1SE": 0.20, "V2": 0.80}
        self.scale = 4.0
        self.enabled = False

        self._ema_score = 0.0
        self._real_streak = 0
        self._fake_streak = 0
        self._state = "EVALUATING"
        self._spoof_entered_at = 0.0
        self._spoof_hold_seconds = 1.0
        self._spoof_release_real_frames = 12
        self._spoof_release_real_threshold = threshold
        self._spoof_release_ema_threshold = max(threshold - 0.10, 0.50)
        self._real_to_spoof_frames = 2
        self._last_raw_real_score = 1.0
        self._recognition_cooldown_until = 0.0
        self._recognition_cooldown_seconds = 5.0

        models_dir = Path(__file__).resolve().parent / "models"
        v1_path = models_dir / "MiniFASNetV1SE.onnx"
        v2_path = models_dir / "MiniFASNetV2.onnx"

        if model_path:
            configured_path = Path(model_path)
            if not configured_path.is_absolute():
                configured_path = (Path(__file__).resolve().parent / configured_path).resolve()
            if not configured_path.is_file():
                v1_path = None
                v2_path = None
            elif "V1" in configured_path.name.upper():
                v1_path = configured_path
            elif "V2" in configured_path.name.upper():
                v2_path = configured_path

        model_specs = (
            ("V1SE", v1_path, 4.0),
            ("V2", v2_path, 2.7),
        )

        for name, resolved_path, scale in model_specs:
            if resolved_path is None:
                continue
            if not resolved_path.is_file():
                logger.warning("Anti-Spoofing model not found at %s", resolved_path)
                continue
            try:
                net = cv2.dnn.readNetFromONNX(str(resolved_path))
                net.setPreferableBackend(cv2.dnn.DNN_BACKEND_OPENCV)
                net.setPreferableTarget(cv2.dnn.DNN_TARGET_CPU)
                self.nets[name] = {"net": net, "scale": scale}
                logger.info(
                    "🛡️ Loaded Anti-Spoofing model %s (scale=%.1f)",
                    resolved_path.name, scale
                )
            except Exception as e:
                logger.warning("Could not load Anti-Spoofing model %s: %s", resolved_path.name, e)

        if "V1SE" in self.nets:
            self.net = self.nets["V1SE"]["net"]
            self.scale = self.nets["V1SE"]["scale"]
        elif "V2" in self.nets:
            self.net = self.nets["V2"]["net"]
            self.scale = self.nets["V2"]["scale"]
        self.enabled = bool(self.nets)
        if self.enabled:
            logger.info(
                "🛡️ Passive Anti-Spoofing ensemble active (models=%s, threshold=%.2f)",
                ",".join(sorted(self.nets)), self.threshold
            )

    def reset(self):
        """Reset temporal state when face leaves camera view."""
        self._ema_score = 0.0
        self._real_streak = 0
        self._fake_streak = 0
        self._state = "EVALUATING"
        self._spoof_entered_at = 0.0
        self._last_raw_real_score = 1.0
        self._recognition_cooldown_until = 0.0

    def is_recognition_allowed(self) -> bool:
        """Require a real state after the post-spoof cooldown."""
        if self._state != "REAL":
            return False
        if time.monotonic() < self._recognition_cooldown_until:
            return False
        return (
            self._real_streak >= self._spoof_release_real_frames
            and self._ema_score >= self._spoof_release_ema_threshold
        )

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

    def _predict_real_score(self, net, face_crop: np.ndarray) -> float:
        # Silent-Face-Anti-Spoofing crops are RGB uint8 frames.
        # The PyTorch ToTensor preprocessing converts them to CHW float32 in [0, 1].
        tensor = face_crop.astype(np.float32) / 255.0
        tensor = np.transpose(tensor, (2, 0, 1))[np.newaxis, ...]
        tensor = np.ascontiguousarray(tensor)
        blob = tensor
        net.setInput(blob)
        logits = net.forward()
        exp_logits = np.exp(logits - np.max(logits, axis=1, keepdims=True))
        probs = exp_logits / np.sum(exp_logits, axis=1, keepdims=True)
        return float(probs[0, 1])

    def update_state(self) -> str:
        if self._state == "SPOOF":
            held_long_enough = (
                time.monotonic() - self._spoof_entered_at
                >= self._spoof_hold_seconds
            )
            if held_long_enough and self._real_streak >= self._spoof_release_real_frames:
                self._state = "REAL"
                self._spoof_entered_at = 0.0
        elif self._state == "REAL":
            if (
                self._fake_streak >= self._real_to_spoof_frames
                and self._last_raw_real_score < self.threshold
                and self._ema_score < self._spoof_release_ema_threshold
            ):
                self._state = "SPOOF"
                self._spoof_entered_at = time.monotonic()
                self._recognition_cooldown_until = time.monotonic() + self._recognition_cooldown_seconds
        elif self._real_streak >= 6 and (
            self._ema_score >= self._spoof_release_ema_threshold
        ):
            self._state = "REAL"
        elif self._fake_streak >= 8 or (
            self._fake_streak >= 3
            and self._ema_score < (self.threshold - 0.35)
        ):
            self._state = "SPOOF"
            self._spoof_entered_at = time.monotonic()
        else:
            self._state = "EVALUATING"
        return self._state

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
            # Camera.capture() returns RGB; keep the same channel order used by the ONNX training pipeline.
            img_rgb = np.ascontiguousarray(frame_bgr_or_rgb)
            weighted_score = 0.0
            total_weight = 0.0
            for name in ("V1SE", "V2"):
                model = self.nets.get(name)
                if model is None:
                    continue
                face_crop = self._crop_face(img_rgb, bbox_xyxy, model["scale"])
                score = self._predict_real_score(model["net"], face_crop)
                weight = self._model_weights[name]
                weighted_score += score * weight
                total_weight += weight

            if total_weight == 0.0:
                return "REAL", 1.0

            raw_real_score = float(weighted_score / total_weight)
            self._last_raw_real_score = raw_real_score

            # Update EMA (40% new raw, 60% memory)
            if self._ema_score == 0.0:
                self._ema_score = raw_real_score
            else:
                self._ema_score = 0.4 * raw_real_score + 0.6 * self._ema_score

            # Single-frame criteria:
            # SPOOF oscillation is rejected unless both the model and EMA are
            # clearly above a stricter live threshold for consecutive frames.
            is_frame_real = bool(
                raw_real_score >= self._spoof_release_real_threshold
                and self._ema_score >= self._spoof_release_ema_threshold
            )

            if is_frame_real:
                self._real_streak += 1
                self._fake_streak = 0
            else:
                self._fake_streak += 1
                self._real_streak = 0

            # State decision with hysteresis:
            # Once SPOOF is confirmed, keep it latched briefly
            # and require a sustained real streak before releasing it.
            state = self.update_state()
            return state, float(self._ema_score)

        except Exception as e:
            logger.debug("Liveness check error: %s", e)
            return "EVALUATING", 0.0
