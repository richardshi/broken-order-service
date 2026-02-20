package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"broken-order-service/internal/modal"
	"broken-order-service/internal/workflows"

	"github.com/go-chi/chi/v5"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

type startReq struct {
	OrderID string `json:"orderId"`
}

type startResp struct {
	WorkflowID string `json:"workflowId"`
	RunID      string `json:"runId"`
}

func main() {
	tc, err := client.Dial(client.Options{HostPort: "localhost:7233"})
	if err != nil {
		log.Fatalf("unable to create Temporal client: %v", err)
	}
	defer tc.Close()

	r := chi.NewRouter()

	// Start a workflow execution for a given orderID.
	// In production, we would want to trigger this from an API call or message queue event rather than a manual HTTP endpoint.
	r.Post("/workflows/start", func(w http.ResponseWriter, r *http.Request) {
		var req startReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OrderID == "" {
			http.Error(w, "invalid body: {\"orderId\":\"...\"}", http.StatusBadRequest)
			return
		}

		// Unique workflow ID: for demo, use "resolve-<orderID>".
		// To keep the idempotency of this endpoint, we set WorkflowExecutionErrorWhenAlreadyStarted: true, so if a workflow with the same ID is already running, it will return an error instead of starting a new one. In production, we would want a more robust strategy for generating unique workflow IDs and handling duplicates (e.g. by checking for existing workflows first, or by allowing multiple workflows per order with different IDs).
		// In production, we might need to handle multiple workflows for the same order (e.g. if we want to allow multiple attempts to resolve the same order issue), in which case we would need a more robust strategy for generating unique workflow IDs and correlating them with orders (e.g. by using a combination of orderID and a timestamp or a UUID).
		// In that case, we can also consider passing event_id(UUID) and order_id as workflow input, and use event_id as workflow ID for uniqueness, and order_id for querying and correlation.
		wid := "resolve-" + req.OrderID

		opts := client.StartWorkflowOptions{
			ID:                                       wid,
			TaskQueue:                                workflows.TaskQueue,
			WorkflowExecutionTimeout:                 1 * time.Minute,
			WorkflowExecutionErrorWhenAlreadyStarted: true,
			WorkflowIDReusePolicy:                    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		we, err := tc.ExecuteWorkflow(ctx, opts, workflows.ResolveBrokenOrder, req.OrderID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, startResp{WorkflowID: we.GetID(), RunID: we.GetRunID()})
	})

	r.Get("/workflows/{workflowId}/casefile", func(w http.ResponseWriter, r *http.Request) {
		workflowID := chi.URLParam(r, "workflowId")
		runID := r.URL.Query().Get("runId")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		qr, err := tc.QueryWorkflow(ctx, workflowID, runID, "casefile")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var cf modal.CaseFile
		if err := qr.Get(&cf); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, cf)
	})

	r.Get("/workflows/{workflowId}/task", func(w http.ResponseWriter, r *http.Request) {
		workflowID := chi.URLParam(r, "workflowId")
		runID := r.URL.Query().Get("runId")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		qr, err := tc.QueryWorkflow(ctx, workflowID, runID, "pending_task")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var task *modal.HumanTask
		if err := qr.Get(&task); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If no task, return null (or you can return {} with 204)
		writeJSON(w, task)
	})

	r.Get("/workflows/{workflowId}/audit", func(w http.ResponseWriter, r *http.Request) {
		workflowID := chi.URLParam(r, "workflowId")
		runID := r.URL.Query().Get("runId")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		qr, err := tc.QueryWorkflow(ctx, workflowID, runID, "audit_log")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var events []modal.AuditEvent
		if err := qr.Get(&events); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, events)
	})

	r.Post("/workflows/{workflowId}/task/decision", func(w http.ResponseWriter, r *http.Request) {
		workflowID := chi.URLParam(r, "workflowId")
		runID := r.URL.Query().Get("runId")

		var d modal.TaskDecision
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil || d.TaskID == "" {
			http.Error(w, "invalid body: {\"taskId\":\"...\",\"approved\":true,\"notes\":\"...\",\"decider\":\"...\"}", http.StatusBadRequest)
			return
		}
		if d.Decider == "" {
			d.Decider = "ops-agent"
		}
		d.DecidedAt = time.Now().UTC()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		if err := tc.SignalWorkflow(ctx, workflowID, runID, workflows.TaskDecisionSignal, d); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"ok": true})
	})

	registerUIRoutes(r, tc)
	log.Println("api listening on :8090")
	log.Fatal(http.ListenAndServe(":8090", r))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
