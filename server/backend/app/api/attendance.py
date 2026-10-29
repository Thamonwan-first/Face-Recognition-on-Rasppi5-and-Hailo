"""
Attendance Log and Summary API (Admin Dashboard).
"""
from typing import Optional
from datetime import timezone
from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session, aliased

from ..db import get_db
from ..models import AttendanceEvent, AttendancePair, User, Admin
from ..security import get_current_admin
from ..utils import format_utc_iso

router = APIRouter(prefix="/api/attendance", tags=["Attendance"])


@router.get("/pairs")
@router.get("/summary")
def list_attendance_pairs(
    start: Optional[str] = None,
    end: Optional[str] = None,
    user_id: Optional[str] = None,
    department: Optional[str] = None,
    limit: int = 300,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    InEvent = aliased(AttendanceEvent)
    OutEvent = aliased(AttendanceEvent)

    query = (
        db.query(
            AttendancePair,
            User.name,
            User.employee_id,
            User.department,
            InEvent.created_at.label("check_in_time"),
            OutEvent.created_at.label("check_out_time"),
        )
        .outerjoin(User, AttendancePair.user_id == User.id)
        .outerjoin(InEvent, AttendancePair.check_in_event_id == InEvent.event_id)
        .outerjoin(OutEvent, AttendancePair.check_out_event_id == OutEvent.event_id)
    )

    if start:
        query = query.filter(AttendancePair.work_date >= start)
    if end:
        query = query.filter(AttendancePair.work_date <= end)
    if user_id:
        query = query.filter(AttendancePair.user_id == user_id)
    if department:
        query = query.filter(User.department == department)

    rows = query.order_by(AttendancePair.work_date.desc(), AttendancePair.id.desc()).limit(limit).all()

    return [
        {
            "id": p.id,
            "user_id": p.user_id,
            "employee_id": u_emp or "-",
            "name": u_name or "-",
            "department": u_dept or "-",
            "work_date": p.work_date.isoformat() if p.work_date else "-",
            "check_in_time": format_utc_iso(in_t),
            "check_out_time": format_utc_iso(out_t),
            "hours_worked": p.hours_worked,
            "status": p.status,
        }
        for (p, u_name, u_emp, u_dept, in_t, out_t) in rows
    ]


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
            "created_at": format_utc_iso(ev.created_at),
            "synced_at": format_utc_iso(ev.synced_at),
        }
        for (ev, u_name, u_emp) in rows
    ]



