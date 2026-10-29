package ports

import (
	"context"
	"time"

	"attendance-backend/internal/entity"
)

type AdminRepository interface {
	GetByUsername(ctx context.Context, username string) (*entity.Admin, error)
	GetByID(ctx context.Context, id string) (*entity.Admin, error)
	Create(ctx context.Context, admin *entity.Admin) error
	UpdateLastLogin(ctx context.Context, id string, lastLogin time.Time) error
}

type DeviceRepository interface {
	GetByID(ctx context.Context, id string) (*entity.Device, error)
	GetByApiKeyHash(ctx context.Context, hash string) (*entity.Device, error)
	List(ctx context.Context) ([]*entity.Device, error)
	Create(ctx context.Context, device *entity.Device) error
	UpdateStatus(ctx context.Context, id string, status string, lastSeen time.Time) error
	UpdateState(ctx context.Context, state *entity.DeviceState) error
	Delete(ctx context.Context, id string) error
}

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*entity.User, error)
	GetByEmployeeID(ctx context.Context, employeeID string) (*entity.User, error)
	List(ctx context.Context, unenrolledOnly bool) ([]*entity.User, error)
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id string) error
	GetLatestVersion(ctx context.Context) (int, error)
	ListUpdatedSince(ctx context.Context, sinceVersion int) ([]*entity.User, error)

	SaveEmbedding(ctx context.Context, embedding *entity.UserEmbedding) error
	GetEmbedding(ctx context.Context, userID string) (*entity.UserEmbedding, error)
	DeleteEmbedding(ctx context.Context, userID string) error
}

type AttendanceRepository interface {
	SaveEvent(ctx context.Context, event *entity.AttendanceEvent) error
	GetEventByID(ctx context.Context, eventID string) (*entity.AttendanceEvent, error)
	ListEvents(ctx context.Context, limit int) ([]*entity.AttendanceEvent, error)
	ListPairs(ctx context.Context, startDate, endDate, userID, department string, limit int) ([]*entity.AttendancePair, error)
	FindOpenPair(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error)
	SavePair(ctx context.Context, pair *entity.AttendancePair) error
	UpdatePair(ctx context.Context, pair *entity.AttendancePair) error
}

type EnrollmentRepository interface {
	CreateJob(ctx context.Context, job *entity.EnrollmentJob) error
	ClaimPendingJob(ctx context.Context, deviceID string) (*entity.EnrollmentJob, error)
	GetPendingJobs(ctx context.Context) ([]*entity.EnrollmentJob, error)
	GetJobByID(ctx context.Context, id string) (*entity.EnrollmentJob, error)
	UpdateJobStatus(ctx context.Context, id, status string) error
}

type AuditRepository interface {
	Log(ctx context.Context, log *entity.AuditLog) error
	List(ctx context.Context, limit int) ([]*entity.AuditLog, error)
}
