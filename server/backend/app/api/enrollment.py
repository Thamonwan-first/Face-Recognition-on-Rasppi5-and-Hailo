"""
Face Enrollment Jobs API.
"""
import uuid
import base64
from datetime import datetime, timezone, timedelta
from typing import Optional
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session
from sqlalchemy import func

from ..db import get_db
from ..models import EnrollmentJob, User, UserEmbedding, Admin, Device
from ..security import get_current_admin, validate_device_api_key

router = APIRouter(prefix="/api/enrollment", tags=["Enrollment"])


class CreateEnrollmentJobRequest(BaseModel):
    user_id: str
    image_b64: Optional[str] = None


@router.post("")
def create_enrollment_job(
    req: CreateEnrollmentJobRequest,
    admin: Admin = Depends(get_current_admin),
    db: Session = Depends(get_db),
):
    user = db.query(User).filter(User.id == req.user_id).first()
    if not user:
        raise HTTPException(status_code=404, detail="User not found")

    job = EnrollmentJob(
        id=str(uuid.uuid4()),
        user_id=req.user_id,
        status="pending",
        expires_at=datetime.now(timezone.utc) + timedelta(hours=24),
    )
    db.add(job)
    db.commit()
    return {"message": "Enrollment job created for Hailo hardware", "job_id": job.id}


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
            "created_at": j.created_at.isoformat() if j.created_at else None,
            "processed_at": j.processed_at.isoformat() if j.processed_at else None,
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

    db.commit()
    return {"message": "Embedding saved and user version bumped", "user_id": job.user_id}
