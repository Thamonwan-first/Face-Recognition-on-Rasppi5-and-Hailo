"""
Device Management API (Admin Dashboard).
"""
import secrets
from datetime import datetime, timezone
from typing import Optional
from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from sqlalchemy.orm import Session

from ..db import get_db
from ..models import Device, DeviceState, Admin
from ..security import get_current_admin
from ..utils import format_utc_iso

router = APIRouter(prefix="/api/devices", tags=["Devices"])


class RegisterDeviceRequest(BaseModel):
    id: str
    name: str
    location: Optional[str] = None
    direction: Optional[str] = "AUTO"


@router.get("/status")
def list_devices_status(admin: Admin = Depends(get_current_admin), db: Session = Depends(get_db)):
    devices = db.query(Device).all()
    now = datetime.now(timezone.utc)
    results = []

    for d in devices:
        computed_status = "offline"
        if d.revoked_at:
            computed_status = "revoked"
        elif d.last_seen:
            last_seen_aware = d.last_seen if d.last_seen.tzinfo else d.last_seen.replace(tzinfo=timezone.utc)
            if (now - last_seen_aware).total_seconds() <= 30:
                computed_status = "online"

        state = d.state
        results.append({
            "id": d.id,
            "name": d.name,
            "location": d.location,
            "direction": d.direction,
            "status": computed_status,
            "last_seen": format_utc_iso(d.last_seen),
            "created_at": format_utc_iso(d.created_at),
            "applied_version": state.applied_version if state else 0,
            "cpu_temp": state.cpu_temp if state else None,
            "fps": state.fps if state else None,
            "outbox_pending": state.outbox_pending if state else 0,
        })
    return results


@router.post("/register", status_code=status.HTTP_201_CREATED)
def register_device(
    req: RegisterDeviceRequest,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    existing = db.query(Device).filter(Device.id == req.id).first()
    if existing:
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail=f"รหัสอุปกรณ์ '{req.id}' มีอยู่แล้ว")

    plain_api_key = secrets.token_hex(32)
    dev = Device(
        id=req.id,
        name=req.name,
        location=req.location,
        direction=req.direction,
        api_key_hash=hash_password(plain_api_key),
        status="offline",
    )
    db.add(dev)
    db.commit()

    record_audit(db, "device.register", admin_id=admin.id, target_type="device", target_id=dev.id)

    return {
        "message": "Device registered successfully",
        "device_id": dev.id,
        "api_key": plain_api_key,
        "note": "กรุณาคัดลอก API Key นี้ไปใส่ในไฟล์คอนฟิกของ Kiosk ทันที ค่านี้จะไม่แสดงซ้ำอีก",
    }
