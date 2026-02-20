package main

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"

	"broken-order-service/internal/modal"
	"broken-order-service/internal/workflows"
)

type uiServer struct {
	tc client.Client
	t  *template.Template
}

type uiTaskRow struct {
	WorkflowID string
	RunID      string
	Task       modal.HumanTask
}

type uiIndexData struct {
	Tab   string
	Query string
	Tasks []uiTaskRow
	Hits  []uiTaskRow // reuse row type for search results
	Error string
}

type uiDetailData struct {
	WorkflowID string
	RunID      string
	CaseFile   modal.CaseFile
	Task       modal.HumanTask
	Audit      []modal.AuditEvent
	Error      string
}

func registerUIRoutes(r chi.Router, tc client.Client) {
	t := template.Must(template.New("base").Parse(uiTemplates))
	s := &uiServer{tc: tc, t: t}

	r.Get("/ui", s.handleIndex)
	r.Get("/ui/wf/{workflowId}", s.handleDetail)
	r.Post("/ui/wf/{workflowId}/decision", s.handleDecision)
}

/*
func (s *uiServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "tasks"
	}
	q := r.URL.Query().Get("q")

	data := uiIndexData{Tab: tab, Query: q}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// List open workflows: CloseTime = missing returns open workflows. In production, we want pagination and better filtering (e.g. by OrderID), but for demo purposes this is sufficient.
	resp, err := s.tc.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		// ry:    "CloseTime is null",
		PageSize: 200,
	})
	if err != nil {
		data.Error = err.Error()
		_ = s.t.ExecuteTemplate(w, "index", data)
		return
	}

	// For MVP: iterate and query each workflow for pending task / casefile.
	for _, ex := range resp.Executions {
		if ex.Execution == nil {
			continue
		}
		wid := ex.Execution.WorkflowId
		rid := ex.Execution.RunId

		task, _ := s.queryPendingTask(r.Context(), wid, rid)

		if tab == "tasks" {
			if task.ID != "" {
				data.Tasks = append(data.Tasks, uiTaskRow{
					WorkflowID: wid,
					RunID:      rid,
					Task:       task,
				})
			}
			continue
		}

		// tab == "search"
		if q == "" {
			continue
		}
		cf, _ := s.queryCaseFile(r.Context(), wid, rid)
		if cf.OrderID == q {
			data.Hits = append(data.Hits, uiTaskRow{
				WorkflowID: wid,
				RunID:      rid,
				Task:       task, // may be empty if no task
			})
		}
	}

	_ = s.t.ExecuteTemplate(w, "index", data)
}
*/

func (s *uiServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "tasks"
	}
	q := r.URL.Query().Get("q")

	data := uiIndexData{Tab: tab, Query: q}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	// Build list filter depending on tab
	var query string
	switch tab {
	case "tasks":
		// Only running workflows (so pending task query is relevant).
		// Optionally scope to workflow type if you want:
		// query = `ExecutionStatus = "Running" AND WorkflowType = "ResolveBrokenOrder"`
		query = `ExecutionStatus = "Running"`
	case "search":
		// Search ALL executions by orderID by leveraging WorkflowId prefix:
		// e.g. resolve-ORDER-42-<timestamp>
		// STARTS_WITH is supported for Keyword attributes like WorkflowId. :contentReference[oaicite:1]{index=1}
		if q == "" {
			// No query => return empty results fast
			_ = s.t.ExecuteTemplate(w, "index", data)
			return
		}
		query = `WorkflowId STARTS_WITH "resolve-` + q + `"`
	default:
		tab = "tasks"
		data.Tab = "tasks"
		query = `ExecutionStatus = "Running"`
	}

	resp, err := s.tc.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		Query:    query,
		PageSize: 200, // MVP: single page
	})
	if err != nil {
		data.Error = err.Error()
		_ = s.t.ExecuteTemplate(w, "index", data)
		return
	}

	// tab=tasks: only return human tasks
	if tab == "tasks" {
		for _, ex := range resp.Executions {
			if ex.Execution == nil {
				continue
			}
			wid := ex.Execution.WorkflowId
			rid := ex.Execution.RunId

			task, err := s.queryPendingTask(r.Context(), wid, rid)
			if err != nil {
				// Ignore noisy workflows / transient query failures in MVP
				continue
			}
			if task.ID == "" {
				continue
			}

			data.Tasks = append(data.Tasks, uiTaskRow{
				WorkflowID: wid,
				RunID:      rid,
				Task:       task,
			})

			// Optional: cap for UI speed
			if len(data.Tasks) >= 100 {
				break
			}
		}

		_ = s.t.ExecuteTemplate(w, "index", data)
		return
	}

	// tab=search: return all executions matched by visibility query
	for _, ex := range resp.Executions {
		if ex.Execution == nil {
			continue
		}
		data.Hits = append(data.Hits, uiTaskRow{
			WorkflowID: ex.Execution.WorkflowId,
			RunID:      ex.Execution.RunId,
			// NOTE: intentionally NOT querying pending_task/casefile here (fast search).
		})
	}

	_ = s.t.ExecuteTemplate(w, "index", data)
}

func (s *uiServer) handleDetail(w http.ResponseWriter, r *http.Request) {
	wid := chi.URLParam(r, "workflowId")
	rid := r.URL.Query().Get("runId")

	data := uiDetailData{WorkflowID: wid, RunID: rid}

	cf, err := s.queryCaseFile(r.Context(), wid, rid)
	if err != nil {
		data.Error = err.Error()
		_ = s.t.ExecuteTemplate(w, "detail", data)
		return
	}
	data.CaseFile = cf

	task, _ := s.queryPendingTask(r.Context(), wid, rid)
	data.Task = task

	audit, _ := s.queryAudit(r.Context(), wid, rid)
	data.Audit = audit

	_ = s.t.ExecuteTemplate(w, "detail", data)
}

func (s *uiServer) handleDecision(w http.ResponseWriter, r *http.Request) {
	wid := chi.URLParam(r, "workflowId")
	rid := r.URL.Query().Get("runId")

	approved := r.FormValue("approved") == "true"
	taskID := r.FormValue("taskId")
	notes := r.FormValue("notes")
	decider := r.FormValue("decider")
	if decider == "" {
		decider = "ops-agent"
	}

	d := modal.TaskDecision{
		TaskID:    taskID,
		Approved:  approved,
		Notes:     notes,
		Decider:   decider,
		DecidedAt: time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := s.tc.SignalWorkflow(ctx, wid, rid, workflows.TaskDecisionSignal, d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/ui/wf/"+wid+"?runId="+rid, http.StatusSeeOther)
}

func (s *uiServer) queryCaseFile(ctx context.Context, wid, rid string) (modal.CaseFile, error) {
	cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	qr, err := s.tc.QueryWorkflow(cctx, wid, rid, "casefile")
	if err != nil {
		return modal.CaseFile{}, err
	}
	var cf modal.CaseFile
	return cf, qr.Get(&cf)
}

func (s *uiServer) queryPendingTask(ctx context.Context, wid, rid string) (modal.HumanTask, error) {
	cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	qr, err := s.tc.QueryWorkflow(cctx, wid, rid, "pending_task")
	if err != nil {
		return modal.HumanTask{}, err
	}
	var t modal.HumanTask
	return t, qr.Get(&t)
}

func (s *uiServer) queryAudit(ctx context.Context, wid, rid string) ([]modal.AuditEvent, error) {
	cctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	qr, err := s.tc.QueryWorkflow(cctx, wid, rid, "audit_log")
	if err != nil {
		return nil, err
	}
	var events []modal.AuditEvent
	return events, qr.Get(&events)
}

func prettyJSON(v any) template.HTML {
	b, _ := json.MarshalIndent(v, "", "  ")
	return template.HTML("<pre>" + template.HTMLEscapeString(string(b)) + "</pre>")
}

const uiTemplates = `
{{define "index"}}
<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>Broken Order Tool</title>
  <style>
    body { font-family: sans-serif; margin: 24px; }
    .tabs a { margin-right: 12px; }
    table { border-collapse: collapse; width: 100%; margin-top: 12px; }
    th, td { border: 1px solid #ddd; padding: 8px; }
    .err { color: #b00020; }
    .muted { color: #666; }
  </style>
</head>
<body>
  <h2>Broken Order Tool (MVP)</h2>

  <div class="tabs">
    <a href="/ui?tab=tasks">Tasks</a>
    <a href="/ui?tab=search">Search</a>
  </div>

  {{if .Error}}<p class="err">{{.Error}}</p>{{end}}

  {{if eq .Tab "tasks"}}
    <h3>Open Human Tasks</h3>
    <p class="muted">List open workflows, query pending task per workflow (UI-grade; not optimized).</p>
    <table>
      <thead><tr><th>Task</th><th>OrderID</th><th>Type</th><th>Workflow</th></tr></thead>
      <tbody>
      {{range .Tasks}}
        <tr>
          <td>{{.Task.ID}}</td>
          <td>{{.Task.OrderID}}</td>
          <td>{{.Task.Type}}</td>
          <td><a href="/ui/wf/{{.WorkflowID}}?runId={{.RunID}}">{{.WorkflowID}}</a></td>
        </tr>
      {{end}}
      </tbody>
    </table>
  {{else}}
    <h3>Search by OrderID</h3>
    <form method="get" action="/ui">
      <input type="hidden" name="tab" value="search"/>
      <input name="q" placeholder="ORDER-42" value="{{.Query}}" style="width: 320px;"/>
      <button type="submit">Search</button>
    </form>

    {{if .Query}}
      <h4>Results</h4>
      <table>
        <thead><tr><th>OrderID</th><th>Workflow</th><th>Has Task?</th></tr></thead>
        <tbody>
        {{range .Hits}}
          <tr>
            <td>{{$.Query}}</td>
            <td><a href="/ui/wf/{{.WorkflowID}}?runId={{.RunID}}">{{.WorkflowID}}</a></td>
            <td>{{if .Task.ID}}Yes{{else}}No{{end}}</td>
          </tr>
        {{end}}
        </tbody>
      </table>
    {{end}}
  {{end}}
</body>
</html>
{{end}}

{{define "detail"}}
<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>Workflow Detail</title>
  <style>
    body { font-family: sans-serif; margin: 24px; }
    .err { color: #b00020; }
    pre { background: #f7f7f7; padding: 12px; overflow: auto; }
    table { border-collapse: collapse; width: 100%; margin-top: 12px; }
    th, td { border: 1px solid #ddd; padding: 8px; }
  </style>
</head>
<body>
  <a href="/ui">← Back</a>
  <h2>Workflow Detail</h2>

  {{if .Error}}<p class="err">{{.Error}}</p>{{end}}

  <p><b>WorkflowID:</b> {{.WorkflowID}}<br/>
     <b>RunID:</b> {{.RunID}}</p>

  <h3>Case File</h3>
  {{.CaseFile | printf "%#v" | html}}

  <h3>Pending Task</h3>
  {{if .Task.ID}}
    <p><b>{{.Task.Title}}</b><br/>{{.Task.Reason}}</p>

    <form method="post" action="/ui/wf/{{.WorkflowID}}/decision?runId={{.RunID}}">
      <input type="hidden" name="taskId" value="{{.Task.ID}}"/>
      <label>Decider: <input name="decider" value="richard"/></label><br/><br/>
      <label>Notes:<br/><textarea name="notes" rows="3" cols="80"></textarea></label><br/><br/>
      <button name="approved" value="true" type="submit">Approve</button>
      <button name="approved" value="false" type="submit">Reject</button>
    </form>
  {{else}}
    <p>(No pending task)</p>
  {{end}}

  <h3>Audit Log</h3>
  <table>
    <thead><tr><th>Time</th><th>Kind</th><th>Message</th></tr></thead>
    <tbody>
      {{range .Audit}}
        <tr>
          <td>{{.At}}</td>
          <td>{{.Kind}}</td>
          <td>{{.Message}}</td>
        </tr>
      {{end}}
    </tbody>
  </table>
</body>
</html>
{{end}}
`
