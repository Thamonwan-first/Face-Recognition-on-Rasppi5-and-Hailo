package entity

import "time"

type AttendanceEvent struct {
	EventID     string    `json:"event_id"`
	UserID      string    `json:"user_id"`
	DeviceID    string    `json:"device_id"`
	Direction   string    `json:"direction"`
	Confidence  float64   `json:"confidence"`
	ClockSynced bool      `json:"clock_synced"`
	SnapshotURL string    `json:"snapshot_url,omitempty"`
	SnapshotB64 string    `json:"snapshot_b64,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	SyncedAt    time.Time `json:"synced_at"`
}

type AttendancePair struct {
	ID               int        `json:"id"`
	UserID           string     `json:"user_id"`
	UserName         string     `json:"name,omitempty"`
	EmployeeID       string     `json:"employee_id,omitempty"`
	Department       string     `json:"department,omitempty"`
	CheckInEventID   *string    `json:"check_in_event_id"`
	CheckOutEventID  *string    `json:"check_out_event_id"`
	CheckInDeviceID  *string    `json:"check_in_device_id,omitempty"`
	CheckOutDeviceID *string    `json:"check_out_device_id,omitempty"`
	CheckInTime      *time.Time `json:"check_in_time"`
	CheckOutTime     *time.Time `json:"check_out_time"`
	CheckInSnapshot  *string    `json:"check_in_snapshot,omitempty"`
	CheckOutSnapshot *string    `json:"check_out_snapshot,omitempty"`
	WorkDate         string     `json:"work_date"`
	HoursWorked      *float64   `json:"hours_worked"`
	Status           string     `json:"status"`
	IsManual         bool       `json:"is_manual"`
	AdjustmentReason string     `json:"adjustment_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type UnrecognizedScan struct {
	ID             string     `json:"id"`
	DeviceID       string     `json:"device_id"`
	SnapshotURL    string     `json:"snapshot_url"`
	SnapshotB64    string     `json:"snapshot_b64,omitempty"`
	Confidence     float64    `json:"confidence"`
	Status         string     `json:"status"` // pending | resolved | dismissed
	ResolvedUserID *string    `json:"resolved_user_id,omitempty"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
