"""
User Management API (Admin Dashboard).
"""
import uuid
from datetime import datetime, timezone
from typing import Optional
from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from sqlalchemy.orm import Session
from sqlalchemy import func, exists

from ..db import get_db
from ..models import User, UserEmbedding, Admin
from ..security import get_current_admin, record_audit

router = APIRouter(prefix="/api/users", tags=["Users"])


def next_version(db: Session) -> int:
    max_v = db.query(func.max(User.version)).scalar() or 0
    return max_v + 1


class CreateUserRequest(BaseModel):
    name: Optional[str] = None
    title: Optional[str] = None
    first_name: Optional[str] = None
    last_name: Optional[str] = None
    employee_id: Optional[str] = None
    department: Optional[str] = None


class UpdateUserRequest(BaseModel):
    name: Optional[str] = None
    title: Optional[str] = None
    first_name: Optional[str] = None
    last_name: Optional[str] = None
    employee_id: Optional[str] = None
    department: Optional[str] = None
    status: Optional[str] = None
    version: Optional[int] = None


@router.get("")
def list_users(
    search: Optional[str] = None,
    department: Optional[str] = None,
    unenrolled_only: bool = False,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    query = db.query(User).filter(User.deleted_at.is_(None))
    if search:
        query = query.filter((User.name.contains(search)) | (User.employee_id.contains(search)))
    if department:
        query = query.filter(User.department == department)
    # Subquery: does this user have at least one face embedding?
    has_face_subq = (
        exists()
        .where(UserEmbedding.user_id == User.id)
        .correlate(User)
    )

    if unenrolled_only:
        query = query.filter(~has_face_subq)

    users = query.order_by(User.created_at.desc()).all()

    # Batch-load enrolled user IDs for the result set only
    user_ids = [u.id for u in users]
    enrolled_user_ids = set(
        r[0] for r in db.query(UserEmbedding.user_id)
        .filter(UserEmbedding.user_id.in_(user_ids))
        .distinct()
        .all()
    ) if user_ids else set()

    return [
        {
            "id": u.id,
            "employee_id": u.employee_id,
            "title": u.title or "",
            "first_name": u.first_name or "",
            "last_name": u.last_name or "",
            "name": u.name,
            "department": u.department,
            "status": u.status,
            "version": u.version,
            "created_at": u.created_at.isoformat() if u.created_at else None,
            "has_face": u.id in enrolled_user_ids,
        }
        for u in users
    ]


@router.post("", status_code=status.HTTP_201_CREATED)
def create_user(
    req: CreateUserRequest,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    if req.employee_id:
        existing = db.query(User).filter(User.employee_id == req.employee_id, User.deleted_at.is_(None)).first()
        if existing:
            raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail=f"รหัสพนักงาน '{req.employee_id}' มีอยู่แล้ว")

    # Compute full name if prefix/first/last are provided
    full_name = req.name
    t = req.title.strip() if req.title else ""
    fn = req.first_name.strip() if req.first_name else ""
    ln = req.last_name.strip() if req.last_name else ""
    prefix = f"{t} " if t else ""
    if fn and ln:
        full_name = f"{prefix}{fn} {ln}".strip()
    elif fn:
        full_name = f"{prefix}{fn}".strip()

    if not full_name:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="กรุณาระบุชื่อพนักงาน")

    new_user = User(
        id=str(uuid.uuid4()),
        employee_id=req.employee_id,
        title=req.title,
        first_name=req.first_name,
        last_name=req.last_name,
        name=full_name,
        department=req.department,
        status="active",
        version=next_version(db),
    )
    db.add(new_user)
    db.commit()
    db.refresh(new_user)

    record_audit(db, "user.create", admin_id=admin.id, target_type="user", target_id=new_user.id, detail={"name": new_user.name})
    return new_user


@router.get("/{user_id}")
def get_user(user_id: str, admin: Admin = Depends(get_current_admin), db: Session = Depends(get_db)):
    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")
    return user


@router.put("/{user_id}")
def update_user(
    user_id: str,
    req: UpdateUserRequest,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")

    # Optimistic locking
    if req.version is not None and user.version != req.version:
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="ข้อมูลถูกแก้ไขโดยผู้อื่นก่อนหน้า กรุณารีเฟรชแล้วลองใหม่")

    if req.title is not None: user.title = req.title
    if req.first_name is not None: user.first_name = req.first_name
    if req.last_name is not None: user.last_name = req.last_name

    # If first_name / last_name updated, recompute name
    if req.first_name is not None or req.last_name is not None or req.title is not None:
        t = (user.title or "").strip()
        fn = (user.first_name or "").strip()
        ln = (user.last_name or "").strip()
        prefix = f"{t} " if t else ""
        if fn and ln:
            user.name = f"{prefix}{fn} {ln}".strip()
        elif fn:
            user.name = f"{prefix}{fn}".strip()
    elif req.name is not None:
        user.name = req.name

    if req.employee_id is not None:
        if req.employee_id != user.employee_id:
            dup = db.query(User).filter(User.employee_id == req.employee_id, User.id != user_id, User.deleted_at.is_(None)).first()
            if dup:
                raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail=f"รหัสพนักงาน '{req.employee_id}' มีอยู่แล้ว")
            user.employee_id = req.employee_id
    if req.department is not None: user.department = req.department
    if req.status is not None: user.status = req.status

    user.version = next_version(db)
    user.updated_at = datetime.now(timezone.utc)
    db.commit()
    db.refresh(user)

    record_audit(db, "user.update", admin_id=admin.id, target_type="user", target_id=user.id)
    return user


@router.delete("/{user_id}")
def delete_user(user_id: str, admin: Admin = Depends(get_current_admin), db: Session = Depends(get_db)):
    user = db.query(User).filter(User.id == user_id, User.deleted_at.is_(None)).first()
    if not user:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="User not found")

    user.deleted_at = datetime.now(timezone.utc)
    user.status = "inactive"
    user.version = next_version(db)

    # PDPA: ลบเวกเตอร์ใบหน้าทั้งหมดของพนักงานคนนี้ออกจากฐานข้อมูล Server ทันที
    db.query(UserEmbedding).filter(UserEmbedding.user_id == user_id).delete()

    db.commit()

    record_audit(db, "user.delete", admin_id=admin.id, target_type="user", target_id=user.id)
    return {"message": "User deleted successfully", "version": user.version}
