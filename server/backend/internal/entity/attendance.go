package entity

import "time"

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
