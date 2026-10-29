"""
Configuration loader for Kiosk.
Reads from config.yaml (preferred) or .env fallback.
"""
import os
from pathlib import Path
from dataclasses import dataclass, field

KIOSK_ROOT = Path(__file__).resolve().parent

try:
    import yaml
    YAML_AVAILABLE = True
except ImportError:
    YAML_AVAILABLE = False


def load_raw_config() -> dict:
    cfg_file = KIOSK_ROOT / "config.yaml"
    if not cfg_file.exists():
        cfg_file = KIOSK_ROOT / "config.example.yaml"

    if YAML_AVAILABLE and cfg_file.exists():
        try:
            with open(cfg_file, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f) or {}
                return data
        except Exception as e:
            print(f"Warning: Failed to parse {cfg_file}: {e}")

    return {}


_RAW = load_raw_config()


@dataclass(frozen=True)
class KioskConfig:
    device_id: str = _RAW.get("device", {}).get("id", os.getenv("DEVICE_ID", "kiosk-01"))
    device_name: str = _RAW.get("device", {}).get("name", "ตู้บันทึกเวลา")
    device_direction: str = _RAW.get("device", {}).get("direction", os.getenv("DEVICE_DIRECTION", "AUTO"))

    server_url: str = _RAW.get("server", {}).get("url", os.getenv("SERVER_URL", "https://localhost:8443"))
    api_key: str = _RAW.get("server", {}).get("api_key", os.getenv("API_KEY", ""))
    ca_cert: str = _RAW.get("server", {}).get("ca_cert", os.getenv("CA_CERT", ""))

    @property
    def ssl_verify(self):
        """Return CA bundle path if configured and exists, otherwise False."""
        if self.ca_cert and Path(self.ca_cert).exists():
            return self.ca_cert
        return False

    recognition_threshold: float = float(_RAW.get("recognition", {}).get("threshold", 0.55))
    debounce_seconds: int = int(_RAW.get("recognition", {}).get("debounce_seconds", 30))
    min_enroll_images: int = int(_RAW.get("recognition", {}).get("min_enroll_images", 3))
    max_enroll_images: int = int(_RAW.get("recognition", {}).get("max_enroll_images", 5))

    detect_model: str = _RAW.get("models", {}).get("detect_model", "models/scrfd_2.5g.hef")
    embedding_model: str = _RAW.get("models", {}).get("embedding_model", "models/arcface_mobilefacenet.hef")
    anti_spoof_model: str = _RAW.get("models", {}).get("anti_spoof_model", "models/MiniFASNetV2.onnx")
    anti_spoof_enabled: bool = bool(_RAW.get("recognition", {}).get("anti_spoof_enabled", True))
    anti_spoof_threshold: float = float(_RAW.get("recognition", {}).get("anti_spoof_threshold", 0.85))

    camera_width: int = int(_RAW.get("camera", {}).get("width", 640))
    camera_height: int = int(_RAW.get("camera", {}).get("height", 480))
    camera_fps: int = int(_RAW.get("camera", {}).get("fps", 30))
    camera_rotation: int = int(_RAW.get("camera", {}).get("rotation", 0))

    db_path: str = _RAW.get("database", {}).get("path", "data/kiosk.db")
    fullscreen: bool = bool(_RAW.get("ui", {}).get("fullscreen", True))
    orientation: str = _RAW.get("ui", {}).get("orientation", "portrait")
    display_width: int = int(_RAW.get("ui", {}).get("width", 480))
    display_height: int = int(_RAW.get("ui", {}).get("height", 800))

    has_rtc: bool = bool(_RAW.get("clock", {}).get("has_rtc", False))

    @property
    def detect_model_path(self) -> Path:
        p = Path(self.detect_model)
        return p if p.is_absolute() else KIOSK_ROOT / p

    @property
    def embedding_model_path(self) -> Path:
        p = Path(self.embedding_model)
        return p if p.is_absolute() else KIOSK_ROOT / p

    @property
    def anti_spoof_model_path(self) -> Path:
        p = Path(self.anti_spoof_model)
        return p if p.is_absolute() else KIOSK_ROOT / p

    @property
    def db_full_path(self) -> Path:
        p = Path(self.db_path)
        return p if p.is_absolute() else KIOSK_ROOT / p


config = KioskConfig()
