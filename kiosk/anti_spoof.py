"""
Silent Face Anti-Spoofing (Passive Liveness Detection)
Uses MiniFASNet (V1SE / V2) ONNX model (Multi-scale face crop + Texture/Moiré/Paper detection)
Runs efficiently via OpenCV DNN.
"""
from pathlib import Path
import logging
import cv2
import numpy as np

logger = logging.getLogger(__name__)


class AntiSpoofDetector:
    """
    MiniFASNet Passive Liveness Detection.
    Class 1: Real Face (Live Person)
    Class 0 / 2: Fake Face (Paper photo, Screen replay, Video)
    """
    def __init__(self, model_path: str = None, scale: float = None, threshold: float = 0.85):
        self.threshold = threshold
        self.net = None
        self.enabled = False

        if model_path is None:
            model_path = Path(__file__).resolve().parent / "models" / "MiniFASNetV1SE.onnx"
            if not model_path.is_file():
                model_path = Path(__file__).resolve().parent / "models" / "MiniFASNetV2.onnx"
        else:
            model_path = Path(model_path)
            if not model_path.is_absolute():
                candidates = [
                    (Path(__file__).resolve().parent / model_path).resolve(),
                    (Path(__file__).resolve().parent / "models" / model_path.name).resolve(),
                    Path.cwd() / model_path,
                ]
                for cand in candidates:
                    if cand.is_file():
                        model_path = cand
                        break

        # Determine scale: MiniFASNetV1SE uses 4.0 (broad border context), V2 uses 2.7
        if scale is not None:
            self.scale = scale
        elif "V1SE" in model_path.name or "4.0" in model_path.name or "4_0" in model_path.name:
            self.scale = 4.0
        else:
            self.scale = 2.7

        if model_path.is_file():
            try:
                self.net = cv2.dnn.readNetFromONNX(str(model_path))
                self.net.setPreferableBackend(cv2.dnn.DNN_BACKEND_OPENCV)
                self.net.setPreferableTarget(cv2.dnn.DNN_TARGET_CPU)
                self.enabled = True
                logger.info(
                    "🛡️ Passive Anti-Spoofing active using %s (scale=%.1f, threshold=%.2f)",
                    model_path.name, self.scale, self.threshold
                )
            except Exception as e:
                logger.warning("Could not load Anti-Spoofing model: %s", e)
        else:
            logger.warning("Anti-Spoofing model not found at %s", model_path)

    def _crop_face(self, image: np.ndarray, bbox_xyxy: list) -> np.ndarray:
        """
        Crop face region with dynamic scale clamping matching official Silent-Face-Anti-Spoofing.
        Clamps scale to frame dimensions and shifts ROI to prevent artificial black borders.
        """
        src_h, src_w = image.shape[:2]
        x1, y1, x2, y2 = bbox_xyxy
        box_w = max(1, x2 - x1)
        box_h = max(1, y2 - y1)

        clamped_scale = min((src_h - 1) / box_h, min((src_w - 1) / box_w, self.scale))
        new_w = box_w * clamped_scale
        new_h = box_h * clamped_scale
        cx = box_w / 2 + x1
        cy = box_h / 2 + y1

        left_top_x = cx - new_w / 2
        left_top_y = cy - new_h / 2
        right_bottom_x = cx + new_w / 2
        right_bottom_y = cy + new_h / 2

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

        x1_c = int(max(0, left_top_x))
        y1_c = int(max(0, left_top_y))
        x2_c = int(min(src_w - 1, right_bottom_x))
        y2_c = int(min(src_h - 1, right_bottom_y))

        cropped = image[y1_c:y2_c, x1_c:x2_c]
        if cropped.size == 0:
            return np.zeros((80, 80, 3), dtype=np.uint8)
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
            # MiniFASNet was trained with RetinaFace Caffe preprocessing: BGR mean subtraction (104, 117, 123)
            blob = cv2.dnn.blobFromImage(
                face_crop,
                scalefactor=1.0,
                size=(80, 80),
                mean=(104.0, 117.0, 123.0),
                swapRB=False,
            )
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
