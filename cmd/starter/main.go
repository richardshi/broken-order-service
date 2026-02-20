package main

import (
	"broken-order-service/internal/workflows"
	"context"
	"flag"
	"log"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

// This is a simple starter that starts a workflow execution for demo/testing purposes.
// For protoype/dev purposes, use a command line arg to specify orderID, and start a workflow for that orderID. In production, this would be triggered by an API call or message queue event with the orderID.
// Note: More details being implemented in cmd/api/main.go, this is just a simple example to demonstrate starting a workflow execution.
func main() {
	var orderID string
	flag.StringVar(&orderID, "order", "ORDER-123", "order id")
	flag.Parse()

	c, err := client.Dial(client.Options{HostPort: "localhost:7233"})
	if err != nil {
		log.Fatalf("unable to create Temporal client: %v", err)
	}
	defer c.Close()

	// Start workflow execution.
	// Note: in production, we want to use a more robust correlation strategy rather than just WorkflowID based on orderID.
	opts := client.StartWorkflowOptions{
		ID:                                       "resolve-" + orderID,
		TaskQueue:                                workflows.TaskQueue,
		WorkflowExecutionTimeout:                 1 * time.Minute,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
		WorkflowIDReusePolicy:                    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	we, err := c.ExecuteWorkflow(ctx, opts, workflows.ResolveBrokenOrder, orderID)
	if err != nil {
		log.Fatalf("unable to execute workflow: %v", err)
	}

	log.Printf("started workflow: WorkflowID=%s RunID=%s\n", we.GetID(), we.GetRunID())

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	var result string
	if err := we.Get(ctx2, &result); err != nil {
		log.Fatalf("unable to get workflow result: %v", err)
	}
	log.Printf("workflow result: %s\n", result)
}

func ctxWithTimeout(d time.Duration) (ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return ctx
}
