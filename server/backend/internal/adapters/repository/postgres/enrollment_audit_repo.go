package postgres

import (
	"context"
	"database/sql"

	"attendance-backend/internal/entity"
	"attendance-backend/internal/ports"
)

type enrollmentRepo struct {
	db *sql.DB
}

func NewEnrollmentRepository(db *sql.DB) ports.EnrollmentRepository {
	return &enrollmentRepo{db: db}
}

func (r *enrollmentRepo) CreateJob(ctx context.Context, job *entity.EnrollmentJob) error {
	query := `
	INSERT INTO enrollment_jobs (id, user_id, image_path, status, assigned_device_id, created_at, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		job.ID, job.UserID, job.ImagePath, job.Status, job.AssignedDeviceID, job.CreatedAt, job.ExpiresAt)
	return err
}

func (r *enrollmentRepo) ClaimPendingJob(ctx context.Context, deviceID string) (*entity.EnrollmentJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `
	SELECT id, user_id, image_path, status, assigned_device_id, created_at, expires_at
	FROM enrollment_jobs
	WHERE status = 'pending' AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	ORDER BY created_at ASC LIMIT 1
	FOR UPDATE SKIP LOCKED
	`
	row := tx.QueryRowContext(ctx, query)
	var j entity.EnrollmentJob
	var imgPath, assignedDev sql.NullString
	var expiresAt sql.NullTime

	if err := row.Scan(&j.ID, &j.UserID, &imgPath, &j.Status, &assignedDev, &j.CreatedAt, &expiresAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	j.ImagePath = imgPath.String
	j.AssignedDeviceID = assignedDev.String
	if expiresAt.Valid {
		j.ExpiresAt = &expiresAt.Time
	}

	updateQuery := "UPDATE enrollment_jobs SET status = 'processing', assigned_device_id = $1 WHERE id = $2"
	if _, err := tx.ExecContext(ctx, updateQuery, deviceID, j.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	j.Status = "processing"
	j.AssignedDeviceID = deviceID
	return &j, nil
}

func (r *enrollmentRepo) GetPendingJobs(ctx context.Context) ([]*entity.EnrollmentJob, error) {
	query := `
	SELECT id, user_id, image_path, status, assigned_device_id, created_at, processed_at, expires_at
	FROM enrollment_jobs
	WHERE status IN ('pending', 'processing')
	ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*entity.EnrollmentJob
	for rows.Next() {
		var j entity.EnrollmentJob
		var imgPath, assignedDev sql.NullString
		var processedAt, expiresAt sql.NullTime

		if err := rows.Scan(&j.ID, &j.UserID, &imgPath, &j.Status, &assignedDev, &j.CreatedAt, &processedAt, &expiresAt); err != nil {
			return nil, err
		}
		j.ImagePath = imgPath.String
		j.AssignedDeviceID = assignedDev.String
		if processedAt.Valid {
			j.ProcessedAt = &processedAt.Time
		}
		if expiresAt.Valid {
			j.ExpiresAt = &expiresAt.Time
		}
		jobs = append(jobs, &j)
	}
	return jobs, nil
}

func (r *enrollmentRepo) GetJobByID(ctx context.Context, id string) (*entity.EnrollmentJob, error) {
	query := `
	SELECT id, user_id, image_path, status, assigned_device_id, created_at, processed_at, expires_at
	FROM enrollment_jobs WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	var j entity.EnrollmentJob
	var imgPath, assignedDev sql.NullString
	var processedAt, expiresAt sql.NullTime

	if err := row.Scan(&j.ID, &j.UserID, &imgPath, &j.Status, &assignedDev, &j.CreatedAt, &processedAt, &expiresAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	j.ImagePath = imgPath.String
	j.AssignedDeviceID = assignedDev.String
	if processedAt.Valid {
		j.ProcessedAt = &processedAt.Time
	}
	if expiresAt.Valid {
		j.ExpiresAt = &expiresAt.Time
	}
	return &j, nil
}

func (r *enrollmentRepo) UpdateJobStatus(ctx context.Context, id, status string) error {
	query := "UPDATE enrollment_jobs SET status = $1, processed_at = CURRENT_TIMESTAMP WHERE id = $2"
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

// ---------------- Audit Repo ----------------

type auditRepo struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) ports.AuditRepository {
	return &auditRepo{db: db}
}

func (r *auditRepo) Log(ctx context.Context, log *entity.AuditLog) error {
	query := "INSERT INTO audit_log (admin_id, action, target_type, target_id, detail, created_at) VALUES ($1, $2, $3, $4, $5, $6)"
	_, err := r.db.ExecContext(ctx, query,
		log.AdminID, log.Action, log.TargetType, log.TargetID, log.Detail, log.CreatedAt)
	return err
}

func (r *auditRepo) List(ctx context.Context, limit int) ([]*entity.AuditLog, error) {
	query := "SELECT id, admin_id, action, target_type, target_id, detail, created_at FROM audit_log ORDER BY created_at DESC LIMIT $1"
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*entity.AuditLog
	for rows.Next() {
		var l entity.AuditLog
		var adminID, targetType, targetID, detail sql.NullString
		if err := rows.Scan(&l.ID, &adminID, &l.Action, &targetType, &targetID, &detail, &l.CreatedAt); err != nil {
			return nil, err
		}
		if adminID.Valid {
			l.AdminID = &adminID.String
		}
		l.TargetType = targetType.String
		l.TargetID = targetID.String
		l.Detail = detail.String
		logs = append(logs, &l)
	}
	return logs, nil
}
