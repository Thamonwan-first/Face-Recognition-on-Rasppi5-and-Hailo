"""
Silent Face Anti-Spoofing (Passive Liveness Detection)
Uses MiniFASNetV2 ONNX model (Multi-scale face crop + Texture/Moiré/Paper detection)
Runs efficiently via OpenCV DNN.
"""
from pathlib import Path
import logging
import cv2
import numpy as np

logger = logging.getLogger(__name__)


class AntiSpoofDetector:
    """
    MiniFASNetV2 Passive Liveness Detection.
    Class 1: Real Face (Live Person)
    Class 0 / 2: Fake Face (Paper photo, Screen replay, Video)
    """
    def __init__(self, model_path: str = None, scale: float = 2.7, threshold: float = 0.85):
        self.scale = scale
        self.threshold = threshold
        self.net = None
        self.enabled = False

        if model_path is None:
            model_path = Path(__file__).resolve().parent / "models" / "MiniFASNetV2.onnx"
        else:
            model_path = Path(model_path)

        if model_path.is_file():
            try:
                self.net = cv2.dnn.readNetFromONNX(str(model_path))
                self.net.setPreferableBackend(cv2.dnn.DNN_BACKEND_OPENCV)
                self.net.setPreferableTarget(cv2.dnn.DNN_TARGET_CPU)
                self.enabled = True
                logger.info("🛡️ Passive Anti-Spoofing active using MiniFASNetV2 (%s, threshold=%.2f)", model_path.name, self.threshold)
            except Exception as e:
                logger.warning("Could not load Anti-Spoofing model: %s", e)
        else:
            logger.warning("Anti-Spoofing model not found at %s", model_path)

    def _crop_face(self, image: np.ndarray, bbox_xyxy: list) -> np.ndarray:
        """
        Crop face region with scale expansion (2.7x) maintaining square aspect ratio
        and zero padding, matching official Silent-Face-Anti-Spoofing MiniFASNet.
        """
        src_h, src_w = image.shape[:2]
        x1, y1, x2, y2 = bbox_xyxy
        w = max(1, x2 - x1)
        h = max(1, y2 - y1)

        new_w = int(w * self.scale)
        new_h = int(h * self.scale)
        cx = int(x1 + w / 2)
        cy = int(y1 + h / 2)

        crop_x1 = cx - new_w // 2
        crop_y1 = cy - new_h // 2
        crop_x2 = crop_x1 + new_w
        crop_y2 = crop_y1 + new_h

        left_pad = max(0, -crop_x1)
        top_pad = max(0, -crop_y1)
        right_pad = max(0, crop_x2 - src_w)
        bottom_pad = max(0, crop_y2 - src_h)

        in_x1 = max(0, crop_x1)
        in_y1 = max(0, crop_y1)
        in_x2 = min(src_w, crop_x2)
        in_y2 = min(src_h, crop_y2)

        cropped = image[in_y1:in_y2, in_x1:in_x2]
        if cropped.size == 0:
            return np.zeros((80, 80, 3), dtype=np.uint8)

        if left_pad > 0 or top_pad > 0 or right_pad > 0 or bottom_pad > 0:
            cropped = cv2.copyMakeBorder(cropped, top_pad, bottom_pad, left_pad, right_pad, cv2.BORDER_CONSTANT, value=0)

        return cv2.resize(cropped, (80, 80))

    def check_liveness(self, frame_bgr_or_rgb: np.ndarray, bbox_xyxy: list) -> tuple[bool, float]:
        """
        Predict if face is real or fake.
        Returns:
            (is_real: bool, confidence: float)
        """
        if not self.enabled or self.net is None:
            return True, 1.0

        try:
            # Convert RGB camera frame to BGR for MiniFASNet
            img_bgr = cv2.cvtColor(frame_bgr_or_rgb, cv2.COLOR_RGB2BGR)
            face_crop = self._crop_face(img_bgr, bbox_xyxy)
            blob = cv2.dnn.blobFromImage(face_crop, scalefactor=1.0, size=(80, 80), swapRB=False)
            self.net.setInput(blob)
            logits = self.net.forward()

            # Softmax
            exp_logits = np.exp(logits - np.max(logits, axis=1, keepdims=True))
            probs = exp_logits / np.sum(exp_logits, axis=1, keepdims=True)

            label_idx = int(np.argmax(probs))
            real_score = float(probs[0, 1])

            # Class 1 is Real Live person
            is_real = bool(label_idx == 1 and real_score >= self.threshold)
            return is_real, real_score
        except Exception as e:
            logger.debug("Liveness check error: %s", e)
            return True, 1.0
