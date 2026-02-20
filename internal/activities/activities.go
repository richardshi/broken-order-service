package activities

import (
	"context"
	"fmt"
	"time"
)

type CaseFile struct {
	OrderID     string
	IssueType   string
	Evidence    map[string]any
	GeneratedAt time.Time
}

type Activities struct{}

func (a *Activities) BuildCaseFile(ctx context.Context, orderID string) (CaseFile, error) {
	// Prototype: mock context aggregation.
	// In the real system, this would call adapters: Order, Transfer, Supplier, Payment, etc.
	cf := CaseFile{
		OrderID:   orderID,
		IssueType: "TRANSFER_FAILED",
		Evidence: map[string]any{
			"transfer_status": "NOT_ACCEPTED",
			"buyer_email":     "buyer@example.com",
			"attempts":        1,
		},
		GeneratedAt: time.Now().UTC(),
	}
	fmt.Printf("[activity] built casefile for order=%s issue=%s\n", orderID, cf.IssueType)
	return cf, nil
}
