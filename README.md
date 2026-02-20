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
1. Broken order event is received (mocked in prototype, using kafka/SQS in production for retries and balancing)
2. Orchestrator starts a workflow per order
3. Workflow calls activities(tool adapters) to:
   1. fetch context (gather all context in prototype. UI can be optimized depending on the issue type).
   2. classify issue types (with Tier).
   3. execute safe remeditaions.
   4. if necessary, create human tasks for gated actinos
4. Workflow updates status + appends to audit log

### **Components**
- Workflow Orchestrator (Using Temporal for prototype can be simplified if needed)
  - Better than db state machine, for the built-in features, durable state when restarted, visibility, clean pattern and fewer edge-cases
- Playbook Engine
  - Easier to scale and manage than a hardcoded switch case.
  - Maps issue types → ordered steps + guardrails
  - Designed to be configurable
- Tool Adapters (Activities)
  - Better boundaries and easier to test. Examples:
    - OrderAdapter: purchase details, listing, seats
    - TransferAdapter: transfer status, retry transfer
    - SupplierAdapter: communication history, send ping
    - PaymentAdapter: compute refund/adjustment, execute refund (mocked)
- Case File Store
  - Materialized summary of the order context (single view of truth for ops)
- Human Task Queue
  - Tasks created by workflows to request approval/decision
  - Approvals resume workflows with structured inputs