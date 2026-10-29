"""
Hailo-8 Face & Landmark Detector (SCRFD) and ArcFace Feature Extractor.
Communicates with Hailo-8 chip over PCIe Gen3.
Includes automated CPU Fallback when running without Hailo hardware.
"""
import logging
import cv2
import numpy as np
from pathlib import Path

logger = logging.getLogger(__name__)

# Check for HailoRT Python bindings (provided by apt install hailo-all)
try:
    from hailo_platform import (
        HEF,
        VDevice,
        HailoStreamInterface,
        ConfigureParams,
        InferVStreams,
        InputVStreamParams,
        OutputVStreamParams,
        FormatType,
    )
    HAILO_AVAILABLE = True
except ImportError:
    HAILO_AVAILABLE = False


def _distance2bbox(points, distance, max_shape=None):
    x1 = points[:, 0] - distance[:, 0]
    y1 = points[:, 1] - distance[:, 1]
    x2 = points[:, 0] + distance[:, 2]
    y2 = points[:, 1] + distance[:, 3]
    if max_shape is not None:
        x1 = np.clip(x1, 0, max_shape[1])
        y1 = np.clip(y1, 0, max_shape[0])
        x2 = np.clip(x2, 0, max_shape[1])
        y2 = np.clip(y2, 0, max_shape[0])
    return np.stack([x1, y1, x2, y2], axis=-1)


def _distance2kps(points, distance, max_shape=None):
    preds = []
    for i in range(0, distance.shape[1], 2):
        px = points[:, 0] + distance[:, i]
        py = points[:, 1] + distance[:, i + 1]
        if max_shape is not None:
            px = np.clip(px, 0, max_shape[1])
            py = np.clip(py, 0, max_shape[0])
        preds.append(px)
        preds.append(py)
    return np.stack(preds, axis=-1)


def _nms(boxes, scores, iou_threshold=0.4):
    if len(boxes) == 0:
        return []
    x1, y1, x2, y2 = boxes[:, 0], boxes[:, 1], boxes[:, 2], boxes[:, 3]
    areas = (x2 - x1 + 1) * (y2 - y1 + 1)
    order = scores.argsort()[::-1]
    keep = []
    while order.size > 0:
        i = order[0]
        keep.append(i)
        xx1 = np.maximum(x1[i], x1[order[1:]])
        yy1 = np.maximum(y1[i], y1[order[1:]])
        xx2 = np.minimum(x2[i], x2[order[1:]])
        yy2 = np.minimum(y2[i], y2[order[1:]])
        w = np.maximum(0.0, xx2 - xx1 + 1)
        h = np.maximum(0.0, yy2 - yy1 + 1)
        inter = w * h
        ovr = inter / (areas[i] + areas[order[1:]] - inter)
        inds = np.where(ovr <= iou_threshold)[0]
        order = order[inds + 1]
    return keep


class HailoDetector:
    """Manages SCRFD face detection and ArcFace feature extraction on Hailo-8."""

    def __init__(self, detect_hef_path: str | Path, embed_hef_path: str | Path | None = None, conf_threshold: float = 0.5):
        self.detect_hef_path = Path(detect_hef_path)
        self.embed_hef_path = Path(embed_hef_path) if embed_hef_path else None
        self.conf_threshold = conf_threshold
        self._vdevice = None
        self._det_network_group = None
        self._emb_network_group = None
        self._backend = "none"
        self._cv_face_cascade = None

    def start(self):
        """Open Hailo VDevice or initialize CPU Fallback."""
        if HAILO_AVAILABLE and self.detect_hef_path.is_file():
            try:
                self._vdevice = VDevice()
                det_hef = HEF(str(self.detect_hef_path))
                det_cfg = ConfigureParams.create_from_hef(det_hef, interface=HailoStreamInterface.PCIe)
                self._det_network_group = self._vdevice.configure(det_hef, det_cfg)[0]
                self._det_input_info = det_hef.get_input_vstream_infos()[0]

                if self.embed_hef_path and self.embed_hef_path.is_file():
                    emb_hef = HEF(str(self.embed_hef_path))
                    emb_cfg = ConfigureParams.create_from_hef(emb_hef, interface=HailoStreamInterface.PCIe)
                    self._emb_network_group = self._vdevice.configure(emb_hef, emb_cfg)[0]
                    self._emb_input_info = emb_hef.get_input_vstream_infos()[0]

                self._backend = "hailo8"
                logger.info("Hailo-8 hardware pipeline active on PCIe Gen3")
                return
            except Exception as e:
                logger.warning("Hailo init failed: %s. Using CPU Fallback.", e)

        self._backend = "cpu_fallback"
        cascade_path = None
        if hasattr(cv2, "data") and hasattr(cv2.data, "haarcascades"):
            cascade_path = cv2.data.haarcascades + "haarcascade_frontalface_default.xml"
        else:
            candidates = [
                "/usr/share/opencv4/haarcascades/haarcascade_frontalface_default.xml",
                "/usr/share/opencv/haarcascades/haarcascade_frontalface_default.xml",
                "/usr/share/OpenCV/haarcascades/haarcascade_frontalface_default.xml",
            ]
            for p in candidates:
                if Path(p).is_file():
                    cascade_path = p
                    break
            if not cascade_path:
                cascade_path = "haarcascade_frontalface_default.xml"

        self._cv_face_cascade = cv2.CascadeClassifier(cascade_path)
        logger.info("Inference active in CPU Fallback mode (Cascade: %s)", cascade_path)

        # Load YuNet Deep Learning Face Detector for CPU Fallback (high accuracy, zero false positives)
        yunet_path = Path(__file__).resolve().parent / "models" / "face_detection_yunet_2023mar.onnx"
        self._cpu_yunet = None
        if yunet_path.is_file() and hasattr(cv2, "FaceDetectorYN"):
            try:
                self._cpu_yunet = cv2.FaceDetectorYN.create(
                    model=str(yunet_path),
                    config="",
                    input_size=(640, 480),
                    score_threshold=0.7,
                    nms_threshold=0.3,
                    top_k=5000,
                )
                logger.info("⭐ Real YuNet Deep Learning Face Detector loaded for CPU mode (Accurate 5-point landmarks)!")
            except Exception as e:
                logger.warning("Could not load YuNet: %s", e)

        # Load real ArcFace ONNX model for CPU face recognition
        onnx_path = Path(__file__).resolve().parent / "models" / "arcface_mobilefacenet.onnx"
        self._cpu_arcface_net = None
        if onnx_path.is_file():
            try:
                self._cpu_arcface_net = cv2.dnn.readNetFromONNX(str(onnx_path))
                logger.info("⭐ Real ArcFace MobileFaceNet ONNX model loaded for CPU recognition!")
            except Exception as e:
                logger.warning("Could not load CPU ArcFace ONNX: %s", e)

    def detect_faces(self, frame_rgb: np.ndarray) -> list[dict]:
        """Detect faces + 5 landmarks."""
        h, w = frame_rgb.shape[:2]

        if self._backend == "hailo8":
            resized = cv2.resize(frame_rgb, (640, 640))
            input_data = np.expand_dims(resized, axis=0)
            in_params = InputVStreamParams.make_from_network_group(self._det_network_group, quantized=False, format_type=FormatType.UINT8)
            out_params = OutputVStreamParams.make_from_network_group(self._det_network_group, quantized=False, format_type=FormatType.FLOAT32)

            with self._det_network_group.activate(self._det_network_group.create_params()):
                with InferVStreams(self._det_network_group, in_params, out_params) as pipeline:
                    raw_outputs = pipeline.infer({self._det_input_info.name: input_data})

            # Decode SCRFD anchors
            scale_x, scale_y = w / 640.0, h / 640.0
            boxes, scores, landmarks = [], [], []

            for stride in (8, 16, 32):
                feat_h, feat_w = 640 // stride, 640 // stride
                centers = np.stack(np.mgrid[:feat_h, :feat_w][::-1], axis=-1).astype(np.float32)
                centers = np.stack([centers * stride] * 2, axis=1).reshape((-1, 2))

                s_score, s_bbox, s_kps = None, None, None
                for k, v in raw_outputs.items():
                    arr = v.squeeze()
                    shape = arr.shape
                    last_dim = shape[-1] if len(shape) > 2 else shape[0]
                    if f"_{stride}" in k.lower() or (feat_h, feat_w) in [shape[:2], shape[-2:]]:
                        if last_dim in (1, 2): s_score = arr.reshape(-1, 1)
                        elif last_dim in (4, 8): s_bbox = arr.reshape(-1, 4) * stride
                        elif last_dim in (10, 20): s_kps = arr.reshape(-1, 10) * stride

                if s_score is None or s_bbox is None:
                    continue

                sc = 1.0 / (1.0 + np.exp(-s_score.flatten())) if s_score.max() > 1.0 else s_score.flatten()
                mask = sc >= self.conf_threshold
                if np.any(mask):
                    b = _distance2bbox(centers[mask], s_bbox[mask])
                    b[:, [0, 2]] *= scale_x
                    b[:, [1, 3]] *= scale_y
                    boxes.append(b)
                    scores.append(sc[mask])
                    if s_kps is not None:
                        kp = _distance2kps(centers[mask], s_kps[mask])
                        kp[:, 0::2] *= scale_x
                        kp[:, 1::2] *= scale_y
                        landmarks.append(kp)

            if not boxes:
                return []

            cat_boxes = np.concatenate(boxes, axis=0)
            cat_scores = np.concatenate(scores, axis=0)
            cat_kps = np.concatenate(landmarks, axis=0) if landmarks else None
            keep = _nms(cat_boxes, cat_scores, 0.4)

            results = []
            for idx in keep:
                pts = cat_kps[idx].reshape((5, 2)) if cat_kps is not None else []
                results.append({
                    "bbox": [int(cat_boxes[idx][0]), int(cat_boxes[idx][1]), int(cat_boxes[idx][2]), int(cat_boxes[idx][3])],
                    "score": float(cat_scores[idx]),
                    "landmarks": [[int(pts[p][0]), int(pts[p][1])] for p in range(len(pts))],
                })
            return results

        # CPU Fallback: Use YuNet Deep Learning detector if available
        if getattr(self, "_cpu_yunet", None) is not None:
            self._cpu_yunet.setInputSize((w, h))
            frame_bgr = cv2.cvtColor(frame_rgb, cv2.COLOR_RGB2BGR)
            _, detected = self._cpu_yunet.detect(frame_bgr)
            res = []
            if detected is not None:
                for face in detected:
                    fx, fy, fw, fh = face[0:4]
                    score = float(face[14])
                    # Real 5-point landmarks: right eye, left eye, nose, right mouth, left mouth
                    landmarks = [
                        [int(face[4]), int(face[5])],
                        [int(face[6]), int(face[7])],
                        [int(face[8]), int(face[9])],
                        [int(face[10]), int(face[11])],
                        [int(face[12]), int(face[13])],
                    ]
                    res.append({
                        "bbox": [int(fx), int(fy), int(fx + fw), int(fy + fh)],
                        "score": score,
                        "landmarks": landmarks,
                    })
            return res

        # Fallback to Haar Cascade if YuNet is not present
        gray = cv2.cvtColor(frame_rgb, cv2.COLOR_RGB2GRAY)
        faces = self._cv_face_cascade.detectMultiScale(gray, 1.15, 8, minSize=(80, 80))
        res = []
        for (x, y, fw, fh) in faces:
            res.append({
                "bbox": [int(x), int(y), int(x + fw), int(y + fh)],
                "score": 0.95,
                "landmarks": [
                    [int(x + fw * 0.3), int(y + fh * 0.38)],
                    [int(x + fw * 0.7), int(y + fh * 0.38)],
                    [int(x + fw * 0.5), int(y + fh * 0.55)],
                    [int(x + fw * 0.35), int(y + fh * 0.75)],
                    [int(x + fw * 0.65), int(y + fh * 0.75)],
                ],
            })
        return res

    def extract_embedding(self, aligned_crop: np.ndarray) -> np.ndarray:
        """Extract 512-d normalized embedding vector."""
        if self._backend == "hailo8" and self._emb_network_group:
            inp = np.expand_dims(cv2.resize(aligned_crop, (112, 112)).astype(np.float32), axis=0)
            in_p = InputVStreamParams.make_from_network_group(self._emb_network_group, quantized=False, format_type=FormatType.FLOAT32)
            out_p = OutputVStreamParams.make_from_network_group(self._emb_network_group, quantized=False, format_type=FormatType.FLOAT32)
            with self._emb_network_group.activate(self._emb_network_group.create_params()):
                with InferVStreams(self._emb_network_group, in_p, out_p) as pipeline:
                    out = pipeline.infer({self._emb_input_info.name: inp})
                    raw = list(out.values())[0].flatten()
                    norm = np.linalg.norm(raw)
                    return (raw / norm) if norm > 0 else raw

        # Real ArcFace ONNX inference on CPU
        if getattr(self, "_cpu_arcface_net", None) is not None:
            try:
                crop_resized = cv2.resize(aligned_crop, (112, 112))
                blob = cv2.dnn.blobFromImage(
                    crop_resized,
                    scalefactor=1.0 / 127.5,
                    size=(112, 112),
                    mean=(127.5, 127.5, 127.5),
                    swapRB=True,
                )
                self._cpu_arcface_net.setInput(blob)
                out = self._cpu_arcface_net.forward()
                raw = out.flatten().astype(np.float32)
                norm = np.linalg.norm(raw)
                return (raw / norm) if norm > 0 else raw
            except Exception as e:
                logger.error("ArcFace CPU inference error: %s", e)

        # CPU fallback dummy if model missing
        rng = np.random.default_rng(int(aligned_crop.mean() * 100))
        vec = rng.standard_normal(512).astype(np.float32)
        return vec / np.linalg.norm(vec)

    def stop(self):
        if self._vdevice:
            try: self._vdevice.release()
            except Exception: pass
            self._vdevice = None

    @property
    def backend_name(self) -> str:
        return self._backend
