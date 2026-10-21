"""
Edge Kiosk Synchronization API.
Protected by X-API-Key header.
"""
import os
import base64
from datetime import datetime, timezone, date
from zoneinfo import ZoneInfo
from typing import List, Optional
from fastapi import APIRouter, Depends
from pydantic import BaseModel
from sqlalchemy.orm import Session
from sqlalchemy import func

from ..db import get_db
from ..models import Device, DeviceState, User, UserEmbedding, AttendanceEvent, AttendancePair
from ..security import validate_device_api_key

LOCAL_TZ = ZoneInfo(os.getenv("SERVER_TIMEZONE", "Asia/Bangkok"))

router = APIRouter(prefix="/sync", tags=["Sync"])


class EventPayload(BaseModel):
    event_id: str
    user_id: str
    device_id: Optional[str] = None
    direction: Optional[str] = "AUTO"
    confidence: Optional[float] = 0.0
    clock_synced: Optional[bool] = False
    created_at: str


class SyncEventsRequest(BaseModel):
    events: List[EventPayload]


@router.post("/events")
def sync_events(
    req: SyncEventsRequest,
    device: Device = Depends(validate_device_api_key),
    db: Session = Depends(get_db),
):
    processed = 0
    now = datetime.now(timezone.utc)

    for ev in req.events:
        # 1. Idempotency check
        existing = db.query(AttendanceEvent).filter(AttendanceEvent.event_id == ev.event_id).first()
        if existing:
            continue

        try:
            ev_dt = datetime.fromisoformat(ev.created_at.replace("Z", "+00:00"))
            if ev_dt.tzinfo is None:
                ev_dt = ev_dt.replace(tzinfo=timezone.utc)
        except Exception:
            ev_dt = now

        # Convert UTC event time to organization local timezone for work_date
        local_dt = ev_dt.astimezone(LOCAL_TZ)
        w_date = local_dt.date()

        # 2. Insert event
        direction = ev.direction or "AUTO"
        event_obj = AttendanceEvent(
            event_id=ev.event_id,
            user_id=ev.user_id,
            device_id=ev.device_id or device.id,
            direction=direction,
            confidence=ev.confidence,
            clock_synced=ev.clock_synced,
            created_at=ev_dt,
            synced_at=now,
        )
        db.add(event_obj)

        # 3. Reconcile attendance pair using First-In, Last-Out for local calendar day
        pair = db.query(AttendancePair).filter(
            AttendancePair.user_id == ev.user_id,
            AttendancePair.work_date == w_date,
        ).first()

        effective_dir = direction
        if not pair:
            # First scan of the day -> Check-in (First-In)
            pair = AttendancePair(
                user_id=ev.user_id,
                check_in_event_id=ev.event_id,
                check_out_event_id=None,
                work_date=w_date,
                status="open",
            )
            db.add(pair)
        else:
            # Subsequent scan of the day -> Update to latest Check-out (Last-Out)
            pair.check_out_event_id = ev.event_id
            if pair.check_in_event_id:
                in_ev = db.query(AttendanceEvent).filter(AttendanceEvent.event_id == pair.check_in_event_id).first()
                if in_ev:
                    in_dt = in_ev.created_at if in_ev.created_at.tzinfo else in_ev.created_at.replace(tzinfo=timezone.utc)
                    diff_hours = (ev_dt - in_dt).total_seconds() / 3600.0
                    pair.hours_worked = round(max(0.0, diff_hours), 2)
            pair.status = "closed"

        processed += 1

    db.commit()
    return {"status": "ok", "received": len(req.events), "processed": processed}


@router.get("/users")
def sync_users(
    since: int = 0,
    device: Device = Depends(validate_device_api_key),
    db: Session = Depends(get_db),
):
    users = db.query(User).filter(User.version > since).order_by(User.version.asc()).all()
    max_v = db.query(func.max(User.version)).scalar() or 0

    user_payloads = []
    for u in users:
        emb_b64 = None
        if not u.deleted_at:
            emb_row = db.query(UserEmbedding).filter(UserEmbedding.user_id == u.id).first()
            if emb_row and emb_row.embedding:
                emb_b64 = base64.b64encode(emb_row.embedding).decode("utf-8")

        user_payloads.append({
            "id": u.id,
            "employee_id": u.employee_id,
            "name": u.name,
            "department": u.department,
            "status": u.status,
            "version": u.version,
            "deleted_at": u.deleted_at.isoformat() if u.deleted_at else None,
            "embedding_b64": emb_b64,
        })

    return {"users": user_payloads, "latest_version": max_v}


class HeartbeatPayload(BaseModel):
    applied_version: Optional[int] = 0
    sw_version: Optional[str] = None
    model_version: Optional[str] = None
    cpu_temp: Optional[float] = None
    fps: Optional[float] = None
    outbox_pending: Optional[int] = 0


@router.post("/heartbeat")
def heartbeat(
    payload: HeartbeatPayload,
    device: Device = Depends(validate_device_api_key),
    db: Session = Depends(get_db),
):
    now = datetime.now(timezone.utc)
    device.status = "online"
    device.last_seen = now

    state = db.query(DeviceState).filter(DeviceState.device_id == device.id).first()
    if not state:
        state = DeviceState(device_id=device.id)
        db.add(state)

    state.applied_version = payload.applied_version
    state.sw_version = payload.sw_version
    state.model_version = payload.model_version
    state.cpu_temp = payload.cpu_temp
    state.fps = payload.fps
    state.outbox_pending = payload.outbox_pending
    state.updated_at = now

    db.commit()
    return {"status": "ok", "server_time": now.isoformat()}
