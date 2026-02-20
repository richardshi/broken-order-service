package modal

import "time"

type CaseFile struct {
	OrderID        string         `json:"orderId"`
	IssueType      IssueType      `json:"issueType"`
	BuyerEmail     string         `json:"buyerEmail"`
	TransferStatus TransferStatus `json:"transferStatus"`
	AttemptCount   int            `json:"attemptCount"`
	GeneratedAt    time.Time      `json:"generatedAt"`
}

type ActionAttemp struct {
	AttemptID      string    `json:"attemptId"`
	OrderID        string    `json:"orderId"`
	ActionType     string    `json:"actionType"`
	IdempotencyKey string    `json:"idempotencyKey"`
	AttemptedAt    time.Time `json:"attemptedAt"`
	Result         string    `json:"result"`
}
