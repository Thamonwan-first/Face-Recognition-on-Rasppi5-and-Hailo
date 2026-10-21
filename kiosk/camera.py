"""
Camera capture module with Picamera2 support on Raspberry Pi 5
and OpenCV / Synthetic fallback for cross-platform development and testing.
"""
import logging
import cv2
import numpy as np

logger = logging.getLogger(__name__)

try:
    from picamera2 import Picamera2
    PICAMERA_AVAILABLE = True
except ImportError:
    PICAMERA_AVAILABLE = False


class Camera:
    """Manages Camera lifecycle for continuous frame capture."""

    def __init__(
        self,
        width: int = 640,
        height: int = 480,
        fps: int = 30,
        camera_index: int = 0,
        force_opencv: bool = False,
        rotation: int = 0,
    ):
        self.width = width
        self.height = height
        self.fps = fps
        self.camera_index = camera_index
        self.force_opencv = force_opencv
        self.rotation = rotation

        self._picam = None
        self._cv_cap = None
        self._is_active = False
        self._backend = "none"

    def start(self):
        """Initialise and start the best available camera backend."""
        # 1. Try Picamera2 on Raspberry Pi 5 (Camera Module 3 / IMX708)
        if PICAMERA_AVAILABLE and not self.force_opencv:
            try:
                self._picam = Picamera2()
                config = self._picam.create_preview_configuration(
                    main={"size": (self.width, self.height), "format": "RGB888"},
                )
                self._picam.configure(config)
                self._picam.start()
                self._backend = "picamera2"
                self._is_active = True
                logger.info(
                    "Picamera2 initialized successfully: %dx%d @ %dfps (IMX708)",
                    self.width, self.height, self.fps
                )
                return
            except Exception as e:
                logger.warning("Picamera2 init failed (%s), falling back to OpenCV", e)

        # 2. Try OpenCV VideoCapture (V4L2 / USB camera / local webcam)
        try:
            self._cv_cap = cv2.VideoCapture(self.camera_index)
            self._cv_cap.set(cv2.CAP_PROP_FRAME_WIDTH, self.width)
            self._cv_cap.set(cv2.CAP_PROP_FRAME_HEIGHT, self.height)
            self._cv_cap.set(cv2.CAP_PROP_FPS, self.fps)

            if self._cv_cap.isOpened():
                self._backend = "opencv"
                self._is_active = True
                logger.info(
                    "OpenCV VideoCapture initialized: device %d (%dx%d)",
                    self.camera_index, self.width, self.height
                )
                return
        except Exception as e:
            logger.warning("OpenCV VideoCapture failed: %s", e)

        # 3. Fallback to Synthetic / Mock frame generator for testing
        self._backend = "mock"
        self._is_active = True
        logger.warning(
            "No hardware camera detected. Running in synthetic mock camera mode."
        )

    def capture(self) -> np.ndarray:
        """
        Capture a single frame as a numpy array in RGB format (H, W, 3) uint8.
        """
        if not self._is_active:
            raise RuntimeError("Camera is not active. Call start() first.")

        frame = None
        if self._backend == "picamera2":
            try:
                raw = self._picam.capture_array()
                # Picamera2 libcamera memory is BGR; convert to RGB for accurate skin tones
                frame = cv2.cvtColor(raw, cv2.COLOR_BGR2RGB)
            except Exception as e:
                logger.warning("Picamera2 capture error: %s", e)
                return None

        elif self._backend == "opencv":
            if self._cv_cap and self._cv_cap.isOpened():
                ret, raw = self._cv_cap.read()
                if ret and raw is not None:
                    frame = cv2.cvtColor(raw, cv2.COLOR_BGR2RGB)
            if frame is None:
                logger.warning("Failed to read frame from OpenCV capture")
                return None

        elif self._backend == "mock":
            frame = self._generate_mock_frame()

        if frame is not None and self.rotation != 0:
            if self.rotation == 90:
                frame = cv2.rotate(frame, cv2.ROTATE_90_CLOCKWISE)
            elif self.rotation == 180:
                frame = cv2.rotate(frame, cv2.ROTATE_180)
            elif self.rotation == 270:
                frame = cv2.rotate(frame, cv2.ROTATE_90_COUNTERCLOCKWISE)

        return frame

    def _generate_mock_frame(self) -> np.ndarray:
        """Generate a realistic test frame with an animated face for testing."""
        frame = np.full((self.height, self.width, 3), 40, dtype=np.uint8)

        # Draw grid background
        for x in range(0, self.width, 40):
            cv2.line(frame, (x, 0), (x, self.height), (60, 60, 60), 1)
        for y in range(0, self.height, 40):
            cv2.line(frame, (0, y), (self.width, y), (60, 60, 60), 1)

        # Draw a synthetic face avatar in center
        cx, cy = self.width // 2, self.height // 2
        cv2.ellipse(frame, (cx, cy), (80, 110), 0, 0, 360, (200, 180, 160), -1)
        # Eyes
        cv2.circle(frame, (cx - 30, cy - 20), 12, (255, 255, 255), -1)
        cv2.circle(frame, (cx + 30, cy - 20), 12, (255, 255, 255), -1)
        cv2.circle(frame, (cx - 30, cy - 20), 5, (50, 50, 50), -1)
        cv2.circle(frame, (cx + 30, cy - 20), 5, (50, 50, 50), -1)
        # Nose
        cv2.line(frame, (cx, cy - 10), (cx, cy + 15), (150, 130, 110), 3)
        # Mouth
        cv2.ellipse(frame, (cx, cy + 45), (35, 15), 0, 0, 180, (80, 80, 180), -1)

        cv2.putText(
            frame, "MOCK CAMERA STREAM", (20, 40),
            cv2.FONT_HERSHEY_SIMPLEX, 0.8, (0, 255, 255), 2
        )
        return cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)

    def stop(self):
        """Stop camera and free resources."""
        if self._picam:
            try:
                self._picam.stop()
                self._picam.close()
            except Exception:
                pass
            self._picam = None

        if self._cv_cap:
            self._cv_cap.release()
            self._cv_cap = None

        self._is_active = False
        self._backend = "none"
        logger.info("Camera stopped successfully")

    @property
    def backend_name(self) -> str:
        return self._backend
