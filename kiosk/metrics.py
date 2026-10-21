"""
System metrics monitoring for Raspberry Pi 5 & Hailo-8.
Measures CPU temperature, FPS, and outbox queue size.
"""
import os
import time
import logging

logger = logging.getLogger(__name__)


class MetricsTracker:
    """Tracks application FPS and system metrics."""

    def __init__(self):
        self._frame_count = 0
        self._last_fps_time = time.time()
        self.current_fps = 0.0

    def tick_frame(self) -> float:
        """Call on every processed frame to calculate rolling FPS."""
        self._frame_count += 1
        now = time.time()
        elapsed = now - self._last_fps_time
        if elapsed >= 1.0:
            self.current_fps = round(self._frame_count / elapsed, 1)
            self._frame_count = 0
            self._last_fps_time = now
        return self.current_fps

    @staticmethod
    def get_cpu_temperature() -> float:
        """Read Raspberry Pi 5 SoC temperature."""
        thermal_path = "/sys/class/thermal/thermal_zone0/temp"
        if os.path.exists(thermal_path):
            try:
                with open(thermal_path, "r") as f:
                    return round(float(f.read().strip()) / 1000.0, 1)
            except Exception:
                pass
        return 45.0  # Simulated default on non-Pi systems

    @staticmethod
    def get_outbox_pending_count(conn) -> int:
        """Count unsynced records in outbox."""
        try:
            cur = conn.cursor()
            cur.execute("SELECT COUNT(*) AS c FROM outbox WHERE synced = 0")
            row = cur.fetchone()
            return int(row["c"]) if row else 0
        except Exception:
            return 0
