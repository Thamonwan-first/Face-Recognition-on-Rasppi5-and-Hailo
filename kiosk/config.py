"""
Configuration loader for Kiosk.
Reads from .env (primary for device credentials) and config.yaml (optional overrides).
"""
import os
from pathlib import Path
from dataclasses import dataclass, field

KIOSK_ROOT = Path(__file__).resolve().parent

# 1. Automatically load .env file if present
def _load_env_file():
    env_file = KIOSK_ROOT / ".env"
    if env_file.is_file():
        try:
            with open(env_file, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line or line.startswith("#") or "=" not in line:
                        continue
                    k, v = line.split("=", 1)
                    k = k.strip()
                    v = v.strip().strip("'\"")
                    # Set in os.environ so os.getenv can access it
                    if k:
                        os.environ.setdefault(k, v)
        except Exception as e:
            print(f"Warning: Failed to parse {env_file}: {e}")

_load_env_file()

try:
    import yaml
    YAML_AVAILABLE = True
except ImportError:
    YAML_AVAILABLE = False


def load_raw_config() -> dict:
    cfg_file = KIOSK_ROOT / "config.yaml"
    if YAML_AVAILABLE and cfg_file.is_file():
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
    device_id: str = os.getenv("DEVICE_ID") or _RAW.get("device", {}).get("id", "kiosk-01")
    device_name: str = os.getenv("DEVICE_NAME") or _RAW.get("device", {}).get("name", "ตู้บันทึกเวลา")
    device_direction: str = os.getenv("DEVICE_DIRECTION") or _RAW.get("device", {}).get("direction", "AUTO")

    server_url: str = os.getenv("SERVER_URL") or _RAW.get("server", {}).get("url", "https://localhost:8443")
    api_key: str = os.getenv("API_KEY") or _RAW.get("server", {}).get("api_key", "")
    ca_cert: str = os.getenv("CA_CERT") or _RAW.get("server", {}).get("ca_cert", "")

    @property
    def ssl_verify(self):
        """Return CA bundle path if configured and exists, otherwise False."""
        if self.ca_cert and Path(self.ca_cert).exists():
            return self.ca_cert
        return False

    def get_headers(self, json_content: bool = True) -> dict:
        """Centralized HTTP headers for Kiosk-to-Server requests."""
        headers = {"X-API-Key": self.api_key}
        if json_content:
            headers["Content-Type"] = "application/json"
        return headers

    def get_url(self, endpoint: str) -> str:
        """Safely format server URL for an endpoint."""
        return f"{self.server_url.rstrip('/')}/{endpoint.lstrip('/')}"

    recognition_threshold: float = float(_RAW.get("recognition", {}).get("threshold", os.getenv("RECOGNITION_THRESHOLD", 0.68)))
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
    camera_mirror: bool = bool(_RAW.get("camera", {}).get("mirror", os.getenv("CAMERA_MIRROR", "false").lower() == "true"))

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
