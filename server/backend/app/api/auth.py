"""
Admin Authentication API.
"""
from datetime import datetime, timezone
from fastapi import APIRouter, Depends, HTTPException, status
from pydantic import BaseModel
from sqlalchemy.orm import Session

from ..db import get_db
from ..models import Admin
from ..security import verify_password, create_access_token, get_current_admin, record_audit

router = APIRouter(prefix="/auth", tags=["Auth"])


class LoginRequest(BaseModel):
    username: str
    password: str


@router.post("/login")
def login(req: LoginRequest, db: Session = Depends(get_db)):
    admin = db.query(Admin).filter(Admin.username == req.username).first()
    if not admin or not verify_password(req.password, admin.password_hash):
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="ชื่อผู้ใช้หรือรหัสผ่านไม่ถูกต้อง")

    admin.last_login = datetime.now(timezone.utc)
    db.commit()

    token = create_access_token({"id": admin.id, "username": admin.username, "role": admin.role})
    record_audit(db, "admin.login", admin_id=admin.id, target_type="admin", target_id=admin.id)

    return {
        "token": token,
        "admin": {"id": admin.id, "username": admin.username, "role": admin.role},
    }


@router.get("/me")
def get_me(admin: Admin = Depends(get_current_admin)):
    return {"id": admin.id, "username": admin.username, "role": admin.role}
