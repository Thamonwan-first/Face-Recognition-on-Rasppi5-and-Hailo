"""
User Management API (Admin Dashboard).
"""
import uuid
from datetime import datetime, timezone
from typing import Optional
from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from sqlalchemy.orm import Session
from sqlalchemy import func

from ..db import get_db
from ..models import User, Admin
from ..security import get_current_admin, record_audit

router = APIRouter(prefix="/api/users", tags=["Users"])


def next_version(db: Session) -> int:
    max_v = db.query(func.max(User.version)).scalar() or 0
    return max_v + 1


class CreateUserRequest(BaseModel):
    name: str
    employee_id: Optional[str] = None
    department: Optional[str] = None


class UpdateUserRequest(BaseModel):
    name: Optional[str] = None
    department: Optional[str] = None
    status: Optional[str] = None
    version: Optional[int] = None


@router.get("")
def list_users(
    search: Optional[str] = None,
    department: Optional[str] = None,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    query = db.query(User).filter(User.deleted_at.is_(None))
    if search:
        query = query.filter((User.name.contains(search)) | (User.employee_id.contains(search)))
    if department:
        query = query.filter(User.department == department)
    return query.order_by(User.created_at.desc()).all()


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

    new_user = User(
        id=str(uuid.uuid4()),
        name=req.name,
        employee_id=req.employee_id,
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

    if req.name is not None: user.name = req.name
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
    db.commit()

    record_audit(db, "user.delete", admin_id=admin.id, target_type="user", target_id=user.id)
    return {"message": "User deleted successfully", "version": user.version}
