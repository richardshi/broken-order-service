package workflows

import (
	"broken-order-service/internal/modal"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const TaskQueue = "BROKEN_ORDER_TASK_QUEUE"
const TaskDecisionSignal = "TASK_DECISION_SIGNAL"

type workflowState struct {
	CaseFile    modal.CaseFile     `json:"caseFile"`
	PendingTask *modal.HumanTask   `json:"pendingTask,omitempty"`
	Audit       []modal.AuditEvent `json:"audit,omitempty"`
}

func ResolveBrokenOrder(ctx workflow.Context, orderID string) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("workflow started", "orderID", orderID)

	// Initialize workflow state and helper for appending audit events.
	state := &workflowState{
		Audit: make([]modal.AuditEvent, 0),
	}

	appendAudit := func(kind, message string, data map[string]any) {
		state.Audit = append(state.Audit, modal.AuditEvent{
			At:      workflow.Now(ctx),
			Kind:    kind,
			Message: message,
			Data:    data,
		})
	}

	// Queries for API to read casefile/tasks/audit without extra DB
	_ = workflow.SetQueryHandler(ctx, "casefile", func() (modal.CaseFile, error) {
		return state.CaseFile, nil
	})

	_ = workflow.SetQueryHandler(ctx, "pending_task", func() (modal.HumanTask, error) {
		if state.PendingTask == nil {
			return modal.HumanTask{}, nil
		}
		return *state.PendingTask, nil
	})

	_ = workflow.SetQueryHandler(ctx, "audit_log", func() ([]modal.AuditEvent, error) {
		return state.Audit, nil
	})

	// Error and retry policy:
	// Timeout: if activity doesn't complete in 10s, assume it failed and retry.
	// Retries: retry up to 3 times with exponential backoff (1s, 2s, 4s) before failing workflow.
	// Note: this is a simple example. In production, we want more robust error handling and compensation logic.
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Build case file
	var cf modal.CaseFile
	if err := workflow.ExecuteActivity(ctx, "BuildCaseFile", orderID).Get(ctx, &cf); err != nil {
		logger.Error("failed to build casefile", "error", err)
		return "", err
	}
	state.CaseFile = cf
	appendAudit("CASEFILE_BUILT", "Case file built for order", map[string]any{
		"issueType": cf.IssueType,
	})

	// Simple playbook (hardcoded for prototype): if issue is TRANSFER_FAILED, create human task to retry transfer.
	// In a real system, this would be more complex with branching logic, multiple task types, etc.
	if cf.IssueType == modal.IssueTransferFailed {
		const maxAttempts = 3
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if state.CaseFile.TransferStatus == modal.TransferAccepted {
				appendAudit("RESOLVED", "transfer already accepted", nil)
				return "RESOLVED_AUTOMATICALLY", nil
			}

			var status modal.TransferStatus
			if err := workflow.ExecuteActivity(ctx, "RetryTransfer", orderID, attempt).Get(ctx, &status); err != nil {
				appendAudit("ERROR", "RetryTransfer failed", map[string]any{
					"attempt": attempt,
					"error":   err.Error(),
				})
				// Let the workflow retry behavior handle transient errors; keep it simple here
				return "", err
			}

			state.CaseFile.TransferStatus = status
			state.CaseFile.AttemptCount = attempt
			appendAudit("RETRY_TRANSFER", "retry transfer executed", map[string]any{
				"attempt": attempt,
				"status":  status,
			})

			if status == modal.TransferAccepted {
				appendAudit("RESOLVED", "transfer accepted after retries", map[string]any{"attempt": attempt})
				return "RESOLVED_AUTOMATICALLY", nil
			}
		}

		// If Still failing after retries, create human task for manual review (not implemented in this prototype).
		task := &modal.HumanTask{
			ID:        "task-" + orderID,
			OrderID:   orderID,
			Type:      "RETRY_TRANSFER",
			Title:     "Please check failed transfer",
			Reason:    "Automated retries failed to resolve transfer issue. Please investigate and take necessary actions.",
			CreatedAt: workflow.Now(ctx),
		}
		state.PendingTask = task
		appendAudit("HUMAN_TASK_CREATED", "created human task for manual review after retries", nil)
		logger.Info("human task created for manual review after retries", "orderID", orderID)

		var decision modal.TaskDecision
		selector := workflow.NewSelector(ctx)
		sigCh := workflow.GetSignalChannel(ctx, TaskDecisionSignal)

		selector.AddReceive(sigCh, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, &decision)
		})

		for decision.TaskID != task.ID {
			selector.Select(ctx) // <-- yields; no busy-spin
		}
		if decision.Approved {
			appendAudit("DONE", "workflow completed after human decision", map[string]any{"result": "ESCALATED_APPROVED"})
			return "ESCALATED_APPROVED", nil
		}
		return "PENDING_MANUAL_REVIEW", nil

	}

	// For other issue types, we can add more logic here. For now, just return resolved for unsupported issue types.

	appendAudit("DONE", "workflow completed after human decision", map[string]any{"result": "ESCALATED_REJECTED"})
	return "ESCALATED_REJECTED", nil
}
