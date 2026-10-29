package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrInvalidInput   = errors.New("invalid input")
	ErrAlreadyExists  = errors.New("already exists")
	ErrInternalServer = errors.New("internal server error")
)

type Admin struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLogin    *time.Time `json:"last_login"`
}

type Device struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Location   string       `json:"location"`
	Direction  string       `json:"direction"`
	ApiKeyHash string       `json:"-"`
	Status     string       `json:"status"`
	LastSeen   *time.Time   `json:"last_seen"`
	CreatedAt  time.Time    `json:"created_at"`
	RevokedAt  *time.Time   `json:"revoked_at"`
	State      *DeviceState `json:"state,omitempty"`

	// Flattened fields for UI compatibility
	CPUTemp       *float64 `json:"cpu_temp,omitempty"`
	FPS           *float64 `json:"fps,omitempty"`
	OutboxPending int      `json:"outbox_pending"`
}

type DeviceState struct {
	DeviceID       string    `json:"device_id"`
	AppliedVersion int       `json:"applied_version"`
	ModelVersion   string    `json:"model_version"`
	SwVersion      string    `json:"sw_version"`
	CPUTemp        *float64  `json:"cpu_temp"`
	FPS            *float64  `json:"fps"`
	OutboxPending  int       `json:"outbox_pending"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type User struct {
	ID          string     `json:"id"`
	EmployeeID  string     `json:"employee_id"`
	Title       string     `json:"title"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	Name        string     `json:"name"`
	Department  string     `json:"department"`
	Status      string     `json:"status"`
	Version     int        `json:"version"`
	DeletedAt   *time.Time `json:"deleted_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	HasEmbedding bool      `json:"has_embedding"`
	RawEmbedding []byte    `json:"-"`
}

type UserEmbedding struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Embedding    []byte    `json:"embedding"`
	ModelVersion string    `json:"model_version"`
	Source       string    `json:"source"`
	DeviceID     string    `json:"device_id"`
	QualityScore float64   `json:"quality_score"`
	CreatedAt    time.Time `json:"created_at"`
}

type AttendanceEvent struct {
	EventID     string    `json:"event_id"`
	UserID      string    `json:"user_id"`
	DeviceID    string    `json:"device_id"`
	Direction   string    `json:"direction"`
	Confidence  float64   `json:"confidence"`
	ClockSynced bool      `json:"clock_synced"`
	CreatedAt   time.Time `json:"created_at"`
	SyncedAt    time.Time `json:"synced_at"`
}

type AttendancePair struct {
	ID              int        `json:"id"`
	UserID          string     `json:"user_id"`
	UserName        string     `json:"name,omitempty"`
	EmployeeID      string     `json:"employee_id,omitempty"`
	Department      string     `json:"department,omitempty"`
	CheckInEventID  *string    `json:"check_in_event_id"`
	CheckOutEventID *string    `json:"check_out_event_id"`
	CheckInTime     *time.Time `json:"check_in_time"`
	CheckOutTime    *time.Time `json:"check_out_time"`
	WorkDate        string     `json:"work_date"`
	HoursWorked     *float64   `json:"hours_worked"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type EnrollmentJob struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	ImagePath        string     `json:"image_path"`
	Status           string     `json:"status"`
	AssignedDeviceID string     `json:"assigned_device_id"`
	CreatedAt        time.Time  `json:"created_at"`
	ProcessedAt      *time.Time `json:"processed_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
}

type AuditLog struct {
	ID         int       `json:"id"`
	AdminID    *string   `json:"admin_id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}
