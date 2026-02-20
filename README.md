# Design Doc: Broken Order Automation Service (Prototype)

### **Problem framing**
During peak events, OPS agents face a backlog of "broken orders" that require repeated, multiple-step investigation across multiple interval system (order dtails, supplier comms, transfer status, attemps, refund policy). The work is often deterministic but time-consuming due to context switching and lack of a  standardized, auditable workflow.

### Hypotheses

| What wastes time today | Hypothesis |  MVP feature | Measurement |
| ------------- | ------------- | ------------- | ------------- |
| Agents hop between tools to build a mental model | Case file will reduce context gathering time | Generate Order Case File with timeline + evidence in One UI | Time spent before first meaningful action |
| Many issues are repetitive (e.g., transfer retries) | Auto-remediation will resolve a meaningful chunk | Safe auto-retry transfer with backoff + stop conditions | retry success rate, and % of resolved order without human |
| Refund/adjustments need policy&judgment | Human-in-loop gates will reduce risky mistakes | Create human tasks for risky actions(e.g. refund) | Refund leakage, escalation rate, time-to-approve |



### Goals
1. Reduce context gathering time by generating case file(Both in UI and Orchestraction). 
2. Reduce time-to-resuolution by automating repetitive steps (context aggregation, standard checks, safe retries).
3. Reduce human touches per order by escalating only uncertain/high-risk decisions.
4. Provide duable orchestration(retries/timeouts), auditability, and observability so the system is safe and debuggable during the spikes.

### Not included in prototype
- Actual integrations with ticket/order/payment processors.
- Full auth and production security
- LLM based classification; We will improve the system with hooks for LLM later.



## **Architecture**
### **High-level flow**
1. Broken order event is received with idemptency key (mocked in prototype, using kafka/SQS in production for retries and balancing)
2. Orchestrator starts a workflow per order
3. Workflow calls activities(tool adapters) to:
   1. fetch context (gather all context in prototype. UI can be optimized depending on the issue type).
   2. classify issue types (with Tier).
   3. execute safe remeditaions.
   4. if necessary, create human tasks for gated actinos
4. Workflow updates status + appends to audit log

### **Components**
Workflow Orchestrator (Using Temporal for prototype)
Temporal is used to represent the resolution process as a durable workflow per order. It provides:
- Durable state across restarts
- Built-in retries/timeouts
- A clear human-in-the-loop pattern (signal to resume)
- Visibility into workflow history and execution state
In some cases, this could also be implemented as a DB-backed state machine, but Temporal reduces edge cases and simplifies operational correctness.

Playbook Engine (hardcoded for prototype)
Playbook logic is currently hardcoded (minimal MVP), but structured so it can evolve into a configurable system:
- Issue type → ordered steps and guardrails
- Bounded retries and stop conditions
- Escalation to human tasks on uncertainty or risk


Activities
Activities represent “tool calls” the workflow can execute. In production these would wrap internal services or external providers. Using adapters provides:
- Clean boundaries
- Testability via mocks
- Centralized reliability policies per downstream dependency
Example adapters (mocked in this prototype):
- OrderAdapter: purchase details, listing, seat info
- TransferAdapter: transfer status, retry transfe
- SupplierAdapter: comms history, send ping
- PaymentAdapter: compute refund/adjustment, issue refund

Case File Store
- Materialized summary of the order context (single view of truth for ops). (Currently saved in Temporal execution for prototype)
Human Task Queue
- Tasks created by workflows to request approval/decision. (Currently saved in Temporal execution for prototype)
- Approvals resume workflows with structured inputs. (Continue and completethe Temporal execution)

### Service Architecture
Both the API/UI and the worker communicate with the Temporal Server:
`[API/CLI] ----> [Temporal Server] <---- [Temporal Worker]`
`[UI/Tools] ----> [Temporal Server] <---- [Temporal Worker]`
- API starts workflows, queries workflow state (case_file/pending_task/audit_log), and sends signals for task decisions.
- Worker executes workflow and activity code.


# Setting up
### Prerequisites
- Go (1.22+ recommended)
- Docker + Docker Compose
- (Optional) PostgreSQL installed locally is not required — Postgres is started via Docker for Temporal persistence.
- Go dependencies are managed by `go mod`(temporal and chi. No separate package install step needed beyond go mod tidy)

### Run temporal and postgres in docker
From the repo root:
1. Start the Temporal stack (Temporal Server + Postgres + Temporal UI): `sudo docker compose up -d` 
2. To check temporal and postgres status, run: `sudo docker compose ps`
3. Note: The Docker Compose file uses a named volume, so Temporal/Postgres data persists across container restarts. To fully reset data, run: `sudo docker compose down -v`

### Run temporal worker and API
1. In terminal 1, run `go run ./cmd/worker`
2. In terminal 2, run `go un ./cmd/api`

### Trigger demo workflows(Sample events) 
In terminal 3, run event test. For example: 
   1. Success request: `curl -s -X POST localhost:8090/workflows/start \
   -H 'Content-Type: application/json' \
   -d '{"orderId":"ORDER-42"}'`
   2. Failed request: `curl -s -X POST localhost:8090/workflows/start \
  -H 'Content-Type: application/json' \
  -d '{"orderId":"ORDER-FAIL-1"}'`


### UI tools
This repo exposes two UIs:
1. Temporal Web UI (workflow visibility and debugging)
View workflow executions in the default namespace: `http://localhost:8080/namespaces/default/workflows`
2. MVP Ops Dashboard (prototype internal tool): `http://localhost:8090/ui`
   1. Task tab: that we have tried, but still require human review/actions.
   2. Search tab: find workflow executions by order id.
   3. Workflow detail view: shows detail case file(aggregated order context) and the audit logs.

## Future Improvements
### 1. Real integrations and reliability policies
Replace mock adapters with real integrations and add reliability patterns:
- Per-adapter timeouts, retries, circuit breakers
- Rate limiting / backpressure to avoid hammering providers during spikes
- Error classification (retryable vs non-retryable) and escalation rules

### 2. Richer playbooks and more issue types
Add more broken-order types and configurable playbooks:
- Seat mismatch / partial fulfillment
- Supplier cannot fulfill / replacement sourcing
- Delivery method constraints (transfer disabled until event, etc.)
- Move from hardcoded branching to config-driven playbooks (JSON/YAML) once stable. 

### 3.AI/agentic assist (optional and gated)
Add an LLM layer for:
- Data extaction and classification using LLM agents
- Case summary drafting (“what happened / what we tried / recommended next step”)
- Suggested actions within strict tool permission boundaries
- Include an evaluation harness (golden cases + rubric) to ensure correctness and safety.

### 4.Security and access control
- Auth/RBAC for Ops tools and approvals
- Audit immutability requirements
- Policy enforcement for high-risk actions (refund thresholds, VIP handling, etc.)