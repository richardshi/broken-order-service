package activities

import (
	"broken-order-service/internal/modal"
	"context"
	"fmt"
	"strings"
	"time"
)

type Activities struct{}

func (a *Activities) BuildCaseFile(ctx context.Context, orderID string) (modal.CaseFile, error) {
	// Prototype: mock context aggregation.
	// In the real system, this would
	// 1. Get order details from Order service (e.g. order amount, buyer/seller info, etc.)
	// 2. Call adapters: Order, Transfer, Supplier, Payment, etc.

	// For demo purposes, hardcode issue type based on orderID (e.g. if orderID contains "TRANSFER_FAILED", set that as issue type).
	cf := modal.CaseFile{
		OrderID:        orderID,
		IssueType:      modal.IssueTransferFailed,
		BuyerEmail:     "richardshi2342+buyer+test1@gmail.com",
		TransferStatus: modal.TransferAccepted,
		AttemptCount:   0,
		GeneratedAt:    time.Now().UTC(),
	}
	fmt.Printf("[activity] built casefile for order=%s issue=%s\n", orderID, cf.IssueType)
	return cf, nil
}

// RetryTransfer simulates retrying a transfer.
// For demo purposes:
// - If orderID contains "FAIL" => never succeeds.
// - Otherwise succeeds on attempt >= 2.
func (a *Activities) RetryTransfer(ctx context.Context, orderID string, attempt int) (modal.TransferStatus, error) {
	if strings.Contains(strings.ToUpper(orderID), "FAIL") {
		fmt.Printf("[activity] RetryTransfer order=%s attempt=%d => NOT_ACCEPTED (forced)\n", orderID, attempt)
		return modal.TransferNotAccepted, nil
	}

	// Simulate success on 2nd attempt or later
	if attempt >= 2 {
		fmt.Printf("[activity] RetryTransfer order=%s attempt=%d => ACCEPTED\n", orderID, attempt)

		// In a real implementation, this would call the Transfer service adapter to perform the transfer and return the actual status.
		return modal.TransferAccepted, nil
	}

	// Simulate failure on first attempt
	fmt.Printf("[activity] RetryTransfer order=%s attempt=%d => NOT_ACCEPTED\n", orderID, attempt)
	return modal.TransferNotAccepted, nil
}
