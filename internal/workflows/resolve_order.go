package workflows

import (
	"broken-order-service/internal/activities"
	"time"

	//"github.com/richardshi/broken-order-temporal/internal/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const TaskQueue = "BROKEN_ORDER_TASK_QUEUE"

func ResolveBrokenOrder(ctx workflow.Context, orderID string) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("workflow started", "orderID", orderID)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var cf activities.CaseFile
	if err := workflow.ExecuteActivity(ctx, "BuildCaseFile", orderID).Get(ctx, &cf); err != nil {
		logger.Error("failed to build casefile", "error", err)
		return "", err
	}

	// Prototype: pretend remediation succeeded once casefile exists.
	// Next step (challenge): branch on cf.IssueType, create human task, etc.
	result := "RESOLVED_AUTOMATICALLY"
	logger.Info("workflow completed", "orderID", orderID, "result", result)
	return result, nil
}
