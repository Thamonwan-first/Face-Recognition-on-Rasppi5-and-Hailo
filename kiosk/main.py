"""
Kiosk Main Entry Point — Raspberry Pi 5 + Hailo-8 AI HAT+.
Starts Database, Camera, Hailo Detector, Background Workers, and PyQt5 UI.
"""
import sys
import logging

from config import config
from database import init_db, Database
from camera import Camera
from detector import HailoDetector
from ui import create_app
from workers.outbox_worker import OutboxWorker
from workers.user_sync import UserSyncWorker
from workers.heartbeat import HeartbeatWorker
from workers.enrollment_worker import EnrollmentWorker

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
)
logger = logging.getLogger("kiosk")


def main() -> int:
    logger.info("==================================================")
    logger.info("🚀 Starting Face Recognition Attendance Kiosk")
    logger.info("Device ID: %s | Mode: %s", config.device_id, config.device_direction)
    logger.info("==================================================")

    # 1. Database
    conn = init_db(config.db_full_path)
    db = Database(conn)
    logger.info("Database initialized at %s", config.db_full_path)

    # 2. Camera
    camera = Camera(
        width=config.camera_width,
        height=config.camera_height,
        fps=config.camera_fps,
        rotation=config.camera_rotation,
    )
    camera.start()
    logger.info("Camera online using backend: [%s]", camera.backend_name)

    # 3. Hailo-8 AI Detector
    detector = HailoDetector(
        detect_hef_path=config.detect_model_path,
        embed_hef_path=config.embedding_model_path,
        conf_threshold=config.recognition_threshold,
    )
    detector.start()
    logger.info("Detector online using backend: [%s]", detector.backend_name)

    # 4. Background Sync Workers (each uses thread-isolated connection)
    outbox_worker = OutboxWorker(config.db_full_path, config)
    outbox_worker.start()

    heartbeat_worker = HeartbeatWorker(config.db_full_path, config)
    heartbeat_worker.start()

    # 5. Launch PyQt5 Touchscreen UI
    app, window = create_app(config, camera, detector, db)

    # Connect user sync updates to refresh UI gallery
    user_sync_worker = UserSyncWorker(config.db_full_path, config, on_users_updated=lambda: window.worker.reload_gallery())
    user_sync_worker.start()

    # Background Enrollment Worker (claims jobs from server, computes ArcFace embeddings, auto-updates gallery)
    enrollment_worker = EnrollmentWorker(detector, config, on_enrollment_done=lambda: window.worker.reload_gallery())
    enrollment_worker.start()

    logger.info("Kiosk UI initialized — showing window (Fullscreen: %s)", config.fullscreen)
    window.show()
    exit_code = app.exec_()

    # Clean shutdown
    logger.info("Shutting down background workers and camera...")
    enrollment_worker.stop()
    outbox_worker.stop()
    user_sync_worker.stop()
    heartbeat_worker.stop()
    detector.stop()
    camera.stop()
    conn.close()
    logger.info("Kiosk clean shutdown complete.")

    return exit_code


if __name__ == "__main__":
    sys.exit(main())
