"""
Security utilities: Password hashing, JWT token authentication,
Device API key verification, and Audit Logging.
"""
import os
import json
from datetime import datetime, timedelta, timezone
import bcrypt
from fastapi import Header, HTTPException, Depends, status
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
from jose import jwt, JWTError
from sqlalchemy.orm import Session

from .db import get_db
from .models import Admin, Device, AuditLog

security_bearer = HTTPBearer(auto_error=False)

JWT_SECRET = os.getenv("JWT_SECRET", "dev-secret-change-me")
ALGORITHM = "HS256"
ACCESS_TOKEN_EXPIRE_HOURS = 8


def hash_password(password: str) -> str:
    salt = bcrypt.gensalt()
    return bcrypt.hashpw(password.encode("utf-8")[:72], salt).decode("utf-8")


def verify_password(plain_password: str, hashed_password: str) -> bool:
    try:
        return bcrypt.checkpw(
            plain_password.encode("utf-8")[:72],
            hashed_password.encode("utf-8")
        )
    except Exception:
        return False


def create_access_token(data: dict) -> str:
    to_encode = data.copy()
    expire = datetime.now(timezone.utc) + timedelta(hours=ACCESS_TOKEN_EXPIRE_HOURS)
    to_encode.update({"exp": expire})
    return jwt.encode(to_encode, JWT_SECRET, algorithm=ALGORITHM)


def get_current_admin(
    credentials: HTTPAuthorizationCredentials = Depends(security_bearer),
    db: Session = Depends(get_db),
) -> Admin:
    """Validate JWT Bearer token and return Admin."""
    if not credentials:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Missing Authorization header")
    try:
        payload = jwt.decode(credentials.credentials, JWT_SECRET, algorithms=[ALGORITHM])
        admin_id: str = payload.get("id")
        if admin_id is None:
            raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Invalid token")
    except JWTError:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Token expired or invalid")

    admin = db.query(Admin).filter(Admin.id == admin_id).first()
    if not admin:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Admin not found")
    return admin


def validate_device_api_key(
    x_api_key: str = Header(None, alias="X-API-Key"),
    db: Session = Depends(get_db),
) -> Device:
    """Validate X-API-Key header against registered Kiosks."""
    if not x_api_key:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Missing X-API-Key header")

    active_devices = db.query(Device).filter(Device.revoked_at.is_(None)).all()
    for dev in active_devices:
        if verify_password(x_api_key, dev.api_key_hash):
            return dev

    raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail="Invalid or revoked API key")


def record_audit(db: Session, action: str, admin_id: str = None, target_type: str = None, target_id: str = None, detail: dict = None):
    """Helper to record audit logs."""
    try:
        entry = AuditLog(
            admin_id=admin_id,
            action=action,
            target_type=target_type,
            target_id=str(target_id) if target_id else None,
            detail=json.dumps(detail, default=str) if detail else None,
        )
        db.add(entry)
        db.commit()
    except Exception as e:
        db.rollback()
        print(f"Audit log failed: {e}")
