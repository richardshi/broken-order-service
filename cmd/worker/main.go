package main

import (
	"broken-order-service/internal/activities"
	"broken-order-service/internal/workflows"
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalf("unable to create Temporal client: %v", err)
	}
	defer c.Close()

	w := worker.New(c, workflows.TaskQueue, worker.Options{})
	// Register workflow + activities (core worker pattern). :contentReference[oaicite:8]{index=8}
	w.RegisterWorkflow(workflows.ResolveBrokenOrder)

	a := &activities.Activities{}
	w.RegisterActivity(a)

	log.Printf("worker started (taskQueue=%s)\n", workflows.TaskQueue)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("worker exited: %v", err)
	}
}
