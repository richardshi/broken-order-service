package modal

import "time"

type HumanTask struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"orderId"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

type TaskDecision struct {
	TaskID    string    `json:"taskId"`
	Approved  bool      `json:"approved"`
	Notes     string    `json:"notes"`
	DecidedAt time.Time `json:"decidedAt"`
	Decider   string    `json:"decider"`
}

type AuditEvent struct {
	At      time.Time      `json:"at"`
	Kind    string         `json:"kind"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}
