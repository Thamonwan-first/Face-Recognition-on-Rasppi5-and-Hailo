"""
Face Enrollment Jobs API.
"""
import uuid
import base64
import os
import shutil
from pathlib import Path
from datetime import datetime, timezone, timedelta
from typing import Optional
from fastapi import APIRouter, Depends, HTTPException, Request
from pydantic import BaseModel
from sqlalchemy.orm import Session
from sqlalchemy import func

from ..db import get_db
from ..models import EnrollmentJob, User, UserEmbedding, Admin, Device
from ..security import get_current_admin, validate_device_api_key
from ..utils import format_utc_iso

router = APIRouter(prefix="/api/enrollment", tags=["Enrollment"])


@router.post("")
async def create_enrollment_job(
    request: Request,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    content_type = request.headers.get("content-type", "")
    user_id = None
    image_bytes_list = []

    if "multipart/form-data" in content_type:
        form = await request.form()
        user_id = form.get("user_id")
        # Support multiple image uploads (up to 5 images)
        files = form.getlist("images") or form.getlist("image")
        for file in files:
            if file and hasattr(file, "read"):
                b = await file.read()
                if b:
                    image_bytes_list.append(b)
    else:
        try:
            body = await request.json()
            user_id = body.get("user_id")
            if body.get("images_b64") and isinstance(body["images_b64"], list):
                for b64 in body["images_b64"]:
                    image_bytes_list.append(base64.b64decode(b64))
            elif body.get("image_b64"):
                image_bytes_list.append(base64.b64decode(body["image_b64"]))
        except Exception:
            pass

    if not user_id:
        raise HTTPException(status_code=400, detail="กรุณาเลือกพนักงาน (user_id is required)")

    user = db.query(User).filter(User.id == user_id).first()
    if not user:
        raise HTTPException(status_code=404, detail="ไม่พบข้อมูลพนักงานในระบบ (User not found)")

    job_id = str(uuid.uuid4())
    img_path = None
    if image_bytes_list:
        enroll_dir = Path(os.getenv("ENROLLMENT_IMAGE_DIR", "./data/enrollment_images"))
        job_dir = enroll_dir / job_id
        job_dir.mkdir(parents=True, exist_ok=True)
        for idx, img_data in enumerate(image_bytes_list):
            img_file = job_dir / f"{idx}.jpg"
            with open(img_file, "wb") as f:
                f.write(img_data)
        img_path = str(job_dir.resolve())

    job = EnrollmentJob(
        id=job_id,
        user_id=user_id,
        image_path=img_path,
        status="pending",
        expires_at=datetime.now(timezone.utc) + timedelta(hours=24),
    )
    db.add(job)
    db.commit()
    return {"message": f"Enrollment job created ({len(image_bytes_list)} images)", "job_id": job.id}


@router.get("/pending")
def list_pending_jobs(
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    jobs = db.query(EnrollmentJob, User.name).outerjoin(User, EnrollmentJob.user_id == User.id).order_by(
        EnrollmentJob.created_at.desc()
    ).limit(50).all()

    return [
        {
            "id": j.id,
            "user_id": j.user_id,
            "user_name": u_name,
            "status": j.status,
            "assigned_device_id": j.assigned_device_id,
            "created_at": format_utc_iso(j.created_at),
            "processed_at": format_utc_iso(j.processed_at),
        }
        for (j, u_name) in jobs
    ]


class SubmitEmbeddingResultRequest(BaseModel):
    embedding_b64: str
    quality_score: Optional[float] = 1.0


@router.post("/{job_id}/result")
def submit_job_result(
    job_id: str,
    req: SubmitEmbeddingResultRequest,
    device: Device = Depends(validate_device_api_key),
    db: Session = Depends(get_db),
):
    job = db.query(EnrollmentJob).filter(EnrollmentJob.id == job_id).first()
    if not job:
        raise HTTPException(status_code=404, detail="Job not found")

    raw_emb = base64.b64decode(req.embedding_b64)
    now = datetime.now(timezone.utc)

    emb_row = UserEmbedding(
        id=str(uuid.uuid4()),
        user_id=job.user_id,
        embedding=raw_emb,
        source="dashboard",
        device_id=device.id,
        quality_score=req.quality_score,
        created_at=now,
    )
    db.add(emb_row)

    job.status = "done"
    job.assigned_device_id = device.id
    job.processed_at = now

    # Bump user version
    user = db.query(User).filter(User.id == job.user_id).first()
    if user:
        max_v = db.query(func.max(User.version)).scalar() or 0
        user.version = max_v + 1

    # PDPA: Delete temporary enrollment image(s) once embedding is extracted
    if job.image_path and os.path.exists(job.image_path):
        try:
            if os.path.isdir(job.image_path):
                shutil.rmtree(job.image_path, ignore_errors=True)
            else:
                os.remove(job.image_path)
            job.image_path = None
        except Exception:
            pass

    db.commit()
    return {"message": "Embedding saved and user version bumped", "user_id": job.user_id}
