"""
Attendance Log and Summary API (Admin Dashboard).
"""
from typing import Optional
from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from ..db import get_db
from ..models import AttendanceEvent, AttendancePair, User, Admin
from ..security import get_current_admin

router = APIRouter(prefix="/api/attendance", tags=["Attendance"])


@router.get("")
def list_attendance(
    start: Optional[str] = None,
    end: Optional[str] = None,
    user_id: Optional[str] = None,
    device_id: Optional[str] = None,
    limit: int = 200,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    query = db.query(AttendanceEvent, User.name, User.employee_id).outerjoin(
        User, AttendanceEvent.user_id == User.id
    )

    if start:
        query = query.filter(AttendanceEvent.created_at >= f"{start} 00:00:00")
    if end:
        query = query.filter(AttendanceEvent.created_at <= f"{end} 23:59:59")
    if user_id:
        query = query.filter(AttendanceEvent.user_id == user_id)
    if device_id:
        query = query.filter(AttendanceEvent.device_id == device_id)

    rows = query.order_by(AttendanceEvent.created_at.desc()).limit(limit).all()

    return [
        {
            "event_id": ev.event_id,
            "user_id": ev.user_id,
            "user_name": u_name,
            "employee_id": u_emp,
            "device_id": ev.device_id,
            "direction": ev.direction,
            "confidence": ev.confidence,
            "created_at": ev.created_at.isoformat() if ev.created_at else None,
            "synced_at": ev.synced_at.isoformat() if ev.synced_at else None,
        }
        for (ev, u_name, u_emp) in rows
    ]


@router.get("/summary")
def get_attendance_summary(
    user_id: Optional[str] = None,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    query = db.query(AttendancePair, User.name, User.employee_id).outerjoin(
        User, AttendancePair.user_id == User.id
    )
    if user_id:
        query = query.filter(AttendancePair.user_id == user_id)

    pairs = query.order_by(AttendancePair.work_date.desc()).limit(300).all()

    return [
        {
            "id": p.id,
            "user_id": p.user_id,
            "user_name": u_name,
            "employee_id": u_emp,
            "work_date": p.work_date.isoformat(),
            "hours_worked": p.hours_worked,
            "status": p.status,
        }
        for (p, u_name, u_emp) in pairs
    ]
