"""
System metrics monitoring for Raspberry Pi 5 & Hailo-8.
Measures real CPU temperature, camera FPS, and outbox queue size.
"""
import glob
import time
import logging
import threading

logger = logging.getLogger(__name__)


class MetricsTracker:
    """Tracks real application FPS and hardware system metrics."""

    _lock = threading.Lock()
    _latest_fps = 0.0
    _frame_count = 0
    _last_fps_time = time.time()

    @classmethod
    def record_frame(cls) -> float:
        """Call on every processed frame to calculate real rolling FPS."""
        with cls._lock:
            cls._frame_count += 1
            now = time.time()
            elapsed = now - cls._last_fps_time
            if elapsed >= 1.0:
                cls._latest_fps = round(cls._frame_count / elapsed, 1)
                cls._frame_count = 0
                cls._last_fps_time = now
            return cls._latest_fps

    @classmethod
    def get_fps(cls) -> float:
        """Return the latest measured real FPS."""
        with cls._lock:
            return cls._latest_fps

    @staticmethod
    def get_cpu_temperature():
        """Read real hardware CPU temperature (works on both Pi 5 and Linux PC)."""
        # 1. Thermal zones (Raspberry Pi & modern Linux)
        for p in sorted(glob.glob("/sys/class/thermal/thermal_zone*/temp")):
            try:
                with open(p, "r") as f:
                    val = float(f.read().strip())
                    if val > 1000:
                        val /= 1000.0
                    if 10.0 <= val <= 115.0:
                        return round(val, 1)
            except Exception:
                pass

        # 2. Hwmon hardware sensors (common on PC / Laptop)
        for p in sorted(glob.glob("/sys/class/hwmon/hwmon*/temp*_input")):
            try:
                with open(p, "r") as f:
                    val = float(f.read().strip())
                    if val > 1000:
                        val /= 1000.0
                    if 10.0 <= val <= 115.0:
                        return round(val, 1)
            except Exception:
                pass

        return None

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
