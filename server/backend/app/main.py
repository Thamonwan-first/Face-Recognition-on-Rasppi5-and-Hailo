"""
FastAPI Server Entry Point — Central Attendance System.
"""
import uuid
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from sqlalchemy.orm import Session

from .db import engine, Base, SessionLocal
from .models import Admin, Device, DeviceState
from .security import hash_password
from .api import auth, sync, users, devices, attendance, enrollment, audit

# Initialize database schema
Base.metadata.create_all(bind=engine)


def seed_defaults():
    """Seed initial Admin and Kiosk-01 on first run."""
    db: Session = SessionLocal()
    try:
        # 1. Admin
        admin = db.query(Admin).filter(Admin.username == "admin").first()
        if not admin:
            db.add(Admin(
                id=str(uuid.uuid4()),
                username="admin",
                password_hash=hash_password("admin1234"),
                role="admin",
            ))

        # 2. Default Kiosk
        dev = db.query(Device).filter(Device.id == "kiosk-01").first()
        if not dev:
            db.add(Device(
                id="kiosk-01",
                name="ตู้บันทึกเวลา ประตูหน้า (Main Entrance)",
                location="ชั้น 1 อาคารสำนักงานใหญ่",
                direction="AUTO",
                api_key_hash=hash_password("changeme-generate-with-openssl-rand-hex-32"),
                status="online",
            ))
            db.add(DeviceState(
                device_id="kiosk-01",
                applied_version=1,
                model_version="scrfd_2.5g + arcface_mobilefacenet",
                sw_version="v1.0.0",
                cpu_temp=46.5,
                fps=30.0,
            ))
        db.commit()
    except Exception as e:
        db.rollback()
        print(f"Seed note: {e}")
    finally:
        db.close()


seed_defaults()

app = FastAPI(
    title="Edge AI Face Attendance Central API",
    version="1.0.0",
    docs_url="/docs",
    redoc_url="/redoc",
)

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health")
def health():
    return {"status": "ok", "service": "attendance-backend"}


# Mount routers
app.include_router(auth.router)
app.include_router(sync.router)
app.include_router(users.router)
app.include_router(devices.router)
app.include_router(attendance.router)
app.include_router(enrollment.router)
app.include_router(audit.router)

if __name__ == "__main__":
    import uvicorn
    uvicorn.run("app.main:app", host="0.0.0.0", port=8443, reload=True)
