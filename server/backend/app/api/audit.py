"""
Audit Log inquiry API.
"""
from typing import Optional
from datetime import timezone
from fastapi import APIRouter, Depends
from sqlalchemy.orm import Session

from ..db import get_db
from ..models import AuditLog, Admin
from ..security import get_current_admin
from ..utils import format_utc_iso

router = APIRouter(prefix="/api/audit", tags=["Audit"])


@router.get("")
def list_audit_logs(
    action: Optional[str] = None,
    limit: int = 200,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    query = db.query(AuditLog, Admin.username).outerjoin(Admin, AuditLog.admin_id == Admin.id)
    if action:
        query = query.filter(AuditLog.action.contains(action))

    logs = query.order_by(AuditLog.created_at.desc()).limit(limit).all()

    return [
        {
            "id": l.id,
            "admin_id": l.admin_id,
            "admin_username": adm_user,
            "action": l.action,
            "target_type": l.target_type,
            "target_id": l.target_id,
            "detail": l.detail,
            "created_at": format_utc_iso(l.created_at),
        }
        for (l, adm_user) in logs
    ]
