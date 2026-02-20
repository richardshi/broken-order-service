package main

import (
	"broken-order-service/internal/workflows"
	"context"
	"flag"
	"log"
	"time"

	"go.temporal.io/sdk/client"
)

func main() {
	var orderID string
	flag.StringVar(&orderID, "order", "ORDER-123", "order id")
	flag.Parse()

	c, err := client.Dial(client.Options{HostPort: "localhost:7233"})
	if err != nil {
		log.Fatalf("unable to create Temporal client: %v", err)
	}
	defer c.Close()

	opts := client.StartWorkflowOptions{
		ID:                       "resolve-" + orderID,
		TaskQueue:                workflows.TaskQueue,
		WorkflowExecutionTimeout: 1 * time.Minute,
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

	return nil
}
