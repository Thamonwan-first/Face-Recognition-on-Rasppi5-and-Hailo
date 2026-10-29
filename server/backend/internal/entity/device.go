package entity

import "time"

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
