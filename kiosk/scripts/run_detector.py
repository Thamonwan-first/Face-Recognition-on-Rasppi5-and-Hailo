#!/usr/bin/env python3
"""
Standalone Face & Person Detection Runner for Raspberry Pi 5 + Hailo-8.
Runs real-time camera inference with SCRFD:
- Displays live preview (or saves preview frames in headless SSH mode)
- Draws bounding boxes and 5 facial landmarks
- Tracks and displays FPS & latency

Usage:
  python kiosk/scripts/run_detector.py [--headless] [--save-interval 30]
"""
import sys
import time
import argparse
import logging
from pathlib import Path
import cv2
import numpy as np

# Ensure project root is in sys.path
PROJECT_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(PROJECT_ROOT))

from src.pipeline.camera import Camera
from src.pipeline.hailo_infer import HailoInferenceEngine
from src.config import config

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger("detector")


def draw_detections(frame_bgr: np.ndarray, faces: list[dict], fps: float, backend_info: str) -> np.ndarray:
    """Draw bounding boxes, landmarks, and telemetry on image."""
    annotated = frame_bgr.copy()

    # Draw telemetry bar at the top
    cv2.rectangle(annotated, (0, 0), (annotated.shape[1], 40), (30, 30, 30), -1)
    status_text = f"FPS: {fps:.1f} | Faces: {len(faces)} | Backend: {backend_info}"
    cv2.putText(
        annotated, status_text, (10, 26),
        cv2.FONT_HERSHEY_SIMPLEX, 0.65, (0, 255, 255), 2
    )

    # Landmark colors: [Left Eye, Right Eye, Nose, Mouth Left, Mouth Right]
    colors = [(255, 0, 0), (0, 0, 255), (0, 255, 0), (255, 255, 0), (0, 255, 255)]

    for face in faces:
        bbox = face["bbox"]
        score = face.get("score", 0.0)
        landmarks = face.get("landmarks", [])

        # Bounding box
        x1, y1, x2, y2 = bbox
        cv2.rectangle(annotated, (x1, y1), (x2, y2), (0, 255, 0), 2)

        # Confidence label
        label = f"Face: {score:.1%}"
        cv2.putText(
            annotated, label, (x1, max(20, y1 - 8)),
            cv2.FONT_HERSHEY_SIMPLEX, 0.55, (0, 255, 0), 2
        )

        # 5 Facial Landmarks
        for i, pt in enumerate(landmarks):
            color = colors[i % len(colors)]
            cv2.circle(annotated, (int(pt[0]), int(pt[1])), 4, color, -1)

    return annotated


def main():
    parser = argparse.ArgumentParser(description="Real-time Face Detector on Raspberry Pi 5 + Hailo-8")
    parser.add_argument("--headless", action="store_true", help="Run without X11 GUI (saves snapshot files instead)")
    parser.add_argument("--save-interval", type=int, default=15, help="Save snapshot every N frames in headless mode")
    parser.add_argument("--threshold", type=float, default=0.5, help="Confidence threshold (0.0 to 1.0)")
    args = parser.parse_args()

    models_dir = PROJECT_ROOT / "models"
    detect_hef = models_dir / "scrfd_2.5g.hef"
    if not detect_hef.exists():
        detect_hef = models_dir / "scrfd_10g.hef"

    logger.info("Starting Face Detection System...")
    logger.info("Using Detection Model: %s", detect_hef)

    # 1. Initialize Camera
    camera = Camera(
        width=config.camera_width,
        height=config.camera_height,
        fps=config.camera_fps,
    )
    camera.start()
    logger.info("Camera online using backend: [%s]", camera.backend_name)

    # 2. Initialize Hailo Inference Engine
    engine = HailoInferenceEngine(
        detect_hef_path=detect_hef,
        conf_threshold=args.threshold,
    )
    engine.start()
    logger.info("Inference Engine online using backend: [%s]", engine.backend_name)

    logger.info("Ready! Processing video stream. Press Ctrl+C to stop.")
    if not args.headless:
        cv2.namedWindow("Raspberry Pi 5 + Hailo-8 Face Detector", cv2.WINDOW_AUTOSIZE)

    frame_count = 0
    start_time = time.time()
    fps = 0.0

    output_dir = PROJECT_ROOT / "data"
    output_dir.mkdir(exist_ok=True)
    snapshot_path = output_dir / "preview_detected.jpg"

    try:
        while True:
            # Capture frame (RGB)
            frame_rgb = camera.capture()
            if frame_rgb is None:
                time.sleep(0.01)
                continue

            frame_count += 1

            # Run Face & Landmark Detection on Hailo-8
            t0 = time.time()
            faces = engine.detect_faces(frame_rgb)
            infer_ms = (time.time() - t0) * 1000

            # Calculate FPS
            elapsed = time.time() - start_time
            if elapsed >= 1.0:
                fps = frame_count / elapsed
                frame_count = 0
                start_time = time.time()
                logger.info(
                    "Telemetry: %.1f FPS | Latency: %.1f ms | Faces Detected: %d",
                    fps, infer_ms, len(faces)
                )

            # Draw annotations (Convert RGB to BGR for OpenCV display/saving)
            frame_bgr = cv2.cvtColor(frame_rgb, cv2.COLOR_RGB2BGR)
            backend_info = f"{engine.backend_name} ({infer_ms:.0f}ms)"
            annotated = draw_detections(frame_bgr, faces, fps, backend_info)

            # Display or Save
            if args.headless:
                if frame_count % args.save_interval == 0:
                    cv2.imwrite(str(snapshot_path), annotated)
                    logger.info("Saved preview snapshot to: %s", snapshot_path)
            else:
                cv2.imshow("Raspberry Pi 5 + Hailo-8 Face Detector", annotated)
                key = cv2.waitKey(1) & 0xFF
                if key in (27, ord('q')):  # ESC or Q to quit
                    break

    except KeyboardInterrupt:
        logger.info("Stopping on user request (SIGINT)...")
    finally:
        camera.stop()
        engine.stop()
        if not args.headless:
            cv2.destroyAllWindows()
        logger.info("Clean shutdown complete.")


if __name__ == "__main__":
    main()
