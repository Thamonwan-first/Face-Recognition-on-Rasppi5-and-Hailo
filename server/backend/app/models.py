"""
SQLAlchemy ORM Models for Central Attendance Server.
"""
from datetime import datetime, timezone
from sqlalchemy import (
    Column, String, Integer, Float, Boolean, DateTime,
    ForeignKey, LargeBinary, Text, Date
)
from sqlalchemy.orm import relationship
from .db import Base


def utcnow():
    return datetime.now(timezone.utc)


class Device(Base):
    __tablename__ = "devices"

    id = Column(String, primary_key=True)               # e.g. "kiosk-01"
    name = Column(String, nullable=False)
    location = Column(String, nullable=True)
    direction = Column(String, default="AUTO")           # IN | OUT | AUTO
    api_key_hash = Column(String, nullable=False)        # bcrypt hash
    status = Column(String, default="offline")           # online | offline | revoked
    last_seen = Column(DateTime, nullable=True)
    created_at = Column(DateTime, default=utcnow)
    revoked_at = Column(DateTime, nullable=True)

    state = relationship("DeviceState", back_populates="device", uselist=False)


class DeviceState(Base):
    __tablename__ = "device_state"

    device_id = Column(String, ForeignKey("devices.id"), primary_key=True)
    applied_version = Column(Integer, default=0)
    model_version = Column(String, nullable=True)
    sw_version = Column(String, nullable=True)
    cpu_temp = Column(Float, nullable=True)
    fps = Column(Float, nullable=True)
    outbox_pending = Column(Integer, default=0)
    updated_at = Column(DateTime, default=utcnow, onupdate=utcnow)

    device = relationship("Device", back_populates="state")


class User(Base):
    __tablename__ = "users"

    id = Column(String, primary_key=True)               # UUID
    employee_id = Column(String, unique=True, nullable=True)
    title = Column(String, nullable=True)               # คำนำหน้า (นาย, นาง, ดร., ฯลฯ)
    first_name = Column(String, nullable=True)          # ชื่อ
    last_name = Column(String, nullable=True)           # นามสกุล
    name = Column(String, nullable=False)               # ชื่อเต็ม
    department = Column(String, nullable=True)          # ฝ่าย (Division)
    status = Column(String, default="active")           # active | inactive
    version = Column(Integer, default=1)                 # Incremented on updates
    deleted_at = Column(DateTime, nullable=True)        # Soft delete
    created_at = Column(DateTime, default=utcnow)
    updated_at = Column(DateTime, default=utcnow, onupdate=utcnow)

    embeddings = relationship("UserEmbedding", back_populates="user")


class UserEmbedding(Base):
    __tablename__ = "user_embeddings"

    id = Column(String, primary_key=True)               # UUID
    user_id = Column(String, ForeignKey("users.id"), nullable=False)
    embedding = Column(LargeBinary, nullable=False)     # 512 float32 = 2048 bytes
    model_version = Column(String, default="arcface_mobilefacenet_v1")
    source = Column(String, default="kiosk")            # kiosk | dashboard
    device_id = Column(String, nullable=True)
    quality_score = Column(Float, default=1.0)
    created_at = Column(DateTime, default=utcnow)

    user = relationship("User", back_populates="embeddings")


class AttendanceEvent(Base):
    __tablename__ = "attendance_events"

    event_id = Column(String, primary_key=True)         # UUID (Idempotency key)
    user_id = Column(String, ForeignKey("users.id"), nullable=False)
    device_id = Column(String, ForeignKey("devices.id"), nullable=False)
    direction = Column(String, default="AUTO")          # IN | OUT | AUTO
    confidence = Column(Float, nullable=False)
    clock_synced = Column(Boolean, default=False)
    created_at = Column(DateTime, nullable=False)       # Scan time on Kiosk
    synced_at = Column(DateTime, default=utcnow)        # Upload time


class AttendancePair(Base):
    __tablename__ = "attendance_pairs"

    id = Column(Integer, primary_key=True, autoincrement=True)
    user_id = Column(String, ForeignKey("users.id"), nullable=False)
    check_in_event_id = Column(String, ForeignKey("attendance_events.event_id"), nullable=True)
    check_out_event_id = Column(String, ForeignKey("attendance_events.event_id"), nullable=True)
    work_date = Column(Date, nullable=False)
    hours_worked = Column(Float, nullable=True)
    status = Column(String, default="open")             # open | closed | manual_edit
    created_at = Column(DateTime, default=utcnow)
    updated_at = Column(DateTime, default=utcnow, onupdate=utcnow)


class EnrollmentJob(Base):
    __tablename__ = "enrollment_jobs"

    id = Column(String, primary_key=True)               # UUID
    user_id = Column(String, ForeignKey("users.id"), nullable=False)
    image_path = Column(String, nullable=True)
    status = Column(String, default="pending")          # pending | done | failed
    assigned_device_id = Column(String, nullable=True)
    created_at = Column(DateTime, default=utcnow)
    processed_at = Column(DateTime, nullable=True)
    expires_at = Column(DateTime, nullable=True)


class Admin(Base):
    __tablename__ = "admins"

    id = Column(String, primary_key=True)               # UUID
    username = Column(String, unique=True, nullable=False)
    password_hash = Column(String, nullable=False)      # bcrypt
    role = Column(String, default="admin")              # admin | viewer
    created_at = Column(DateTime, default=utcnow)
    last_login = Column(DateTime, nullable=True)


class AuditLog(Base):
    __tablename__ = "audit_log"

    id = Column(Integer, primary_key=True, autoincrement=True)
    admin_id = Column(String, ForeignKey("admins.id"), nullable=True)
    action = Column(String, nullable=False)             # user.create, device.register
    target_type = Column(String, nullable=True)
    target_id = Column(String, nullable=True)
    detail = Column(Text, nullable=True)                # JSON snapshot
    created_at = Column(DateTime, default=utcnow)
