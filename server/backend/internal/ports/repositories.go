package ports

import (
	"context"
	"time"

	"attendance-backend/internal/entity"
)

type AdminRepository interface {
	GetByUsername(ctx context.Context, username string) (*entity.Admin, error)
	GetByID(ctx context.Context, id string) (*entity.Admin, error)
	List(ctx context.Context) ([]*entity.Admin, error)
	Create(ctx context.Context, admin *entity.Admin) error
	Update(ctx context.Context, admin *entity.Admin) error
	UpdatePassword(ctx context.Context, id string, passwordHash string) error
	UpdateLastLogin(ctx context.Context, id string, lastLogin time.Time) error
	Delete(ctx context.Context, id string) error
}

type DeviceRepository interface {
	GetByID(ctx context.Context, id string) (*entity.Device, error)
	GetByApiKeyHash(ctx context.Context, hash string) (*entity.Device, error)
	List(ctx context.Context) ([]*entity.Device, error)
	Create(ctx context.Context, device *entity.Device) error
	Update(ctx context.Context, device *entity.Device) error
	UpdateStatus(ctx context.Context, id string, status string, lastSeen time.Time) error
	UpdateIPAddress(ctx context.Context, id string, ip string) error
	UpdateStatusAndIP(ctx context.Context, id string, status string, ip string, lastSeen time.Time) error
	UpdateState(ctx context.Context, state *entity.DeviceState) error
	UpdateMode(ctx context.Context, id string, mode string) error
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
	ListPairs(ctx context.Context, startDate, endDate, userID, department, deviceID string, limit int) ([]*entity.AttendancePair, error)
	GetPairByID(ctx context.Context, id int) (*entity.AttendancePair, error)
	FindOpenPair(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error)
	FindLastPairByUserAndDate(ctx context.Context, userID, workDate string) (*entity.AttendancePair, error)
	SavePair(ctx context.Context, pair *entity.AttendancePair) error
	UpdatePair(ctx context.Context, pair *entity.AttendancePair) error
	SaveEventAndPair(ctx context.Context, event *entity.AttendanceEvent, pair *entity.AttendancePair, isUpdate bool) error

	SaveUnrecognized(ctx context.Context, scan *entity.UnrecognizedScan) error
	ListUnrecognized(ctx context.Context, limit int) ([]*entity.UnrecognizedScan, error)
	GetUnrecognizedByID(ctx context.Context, id string) (*entity.UnrecognizedScan, error)
	ResolveUnrecognized(ctx context.Context, id, userID, status string) error
	DeleteUnrecognized(ctx context.Context, id string) error
}

type SettingsRepository interface {
	GetSettings(ctx context.Context) (*entity.SystemSettings, error)
	UpdateSettings(ctx context.Context, settings *entity.SystemSettings) error
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

type MeetingRepository interface {
	Create(ctx context.Context, meeting *entity.Meeting) error
	Update(ctx context.Context, meeting *entity.Meeting) error
	GetByID(ctx context.Context, id string) (*entity.Meeting, error)
	List(ctx context.Context, date string, status string, deviceID string) ([]*entity.Meeting, error)
	Delete(ctx context.Context, id string) error
	FindActiveMeetingByDevice(ctx context.Context, deviceID string, date string, timeStr string) (*entity.Meeting, error)

	RecordAttendee(ctx context.Context, attendee *entity.MeetingAttendee) error
	ListAttendees(ctx context.Context, meetingID string) ([]*entity.MeetingAttendee, error)
	GetAttendee(ctx context.Context, meetingID string, userID string) (*entity.MeetingAttendee, error)
}
