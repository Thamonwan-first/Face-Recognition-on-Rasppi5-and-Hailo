package ports

import (
	"context"
	"time"

	"attendance-backend/internal/core/domain"
)

type AdminRepository interface {
	GetByUsername(ctx context.Context, username string) (*domain.Admin, error)
	GetByID(ctx context.Context, id string) (*domain.Admin, error)
	Create(ctx context.Context, admin *domain.Admin) error
	UpdateLastLogin(ctx context.Context, id string, lastLogin time.Time) error
}

type DeviceRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Device, error)
	List(ctx context.Context) ([]*domain.Device, error)
	Create(ctx context.Context, device *domain.Device) error
	UpdateStatus(ctx context.Context, id string, status string, lastSeen time.Time) error
	UpdateState(ctx context.Context, state *domain.DeviceState) error
	Delete(ctx context.Context, id string) error
}

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmployeeID(ctx context.Context, employeeID string) (*domain.User, error)
	List(ctx context.Context, unenrolledOnly bool) ([]*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id string) error
	GetLatestVersion(ctx context.Context) (int, error)
	ListUpdatedSince(ctx context.Context, sinceVersion int) ([]*domain.User, error)

	SaveEmbedding(ctx context.Context, embedding *domain.UserEmbedding) error
	GetEmbedding(ctx context.Context, userID string) (*domain.UserEmbedding, error)
	DeleteEmbedding(ctx context.Context, userID string) error
}

type AttendanceRepository interface {
	SaveEvent(ctx context.Context, event *domain.AttendanceEvent) error
	GetEventByID(ctx context.Context, eventID string) (*domain.AttendanceEvent, error)
	ListEvents(ctx context.Context, limit int) ([]*domain.AttendanceEvent, error)
	ListPairs(ctx context.Context, startDate, endDate, userID, department string, limit int) ([]*domain.AttendancePair, error)
	FindOpenPair(ctx context.Context, userID, workDate string) (*domain.AttendancePair, error)
	SavePair(ctx context.Context, pair *domain.AttendancePair) error
	UpdatePair(ctx context.Context, pair *domain.AttendancePair) error
}

type EnrollmentRepository interface {
	CreateJob(ctx context.Context, job *domain.EnrollmentJob) error
	ClaimPendingJob(ctx context.Context, deviceID string) (*domain.EnrollmentJob, error)
	GetPendingJobs(ctx context.Context) ([]*domain.EnrollmentJob, error)
	GetJobByID(ctx context.Context, id string) (*domain.EnrollmentJob, error)
	UpdateJobStatus(ctx context.Context, id, status string) error
}

type AuditRepository interface {
	Log(ctx context.Context, log *domain.AuditLog) error
	List(ctx context.Context, limit int) ([]*domain.AuditLog, error)
}
