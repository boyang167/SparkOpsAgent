# Task 10 Report: External Side Effects via Operation Outbox

## Scope

- Baseline: `d73ddf4c020e6e088108b49abf68e14ef07915cc`.
- Replaced the eight Task 9 retry-sentinel subscriptions with stable Operation Outbox scheduling.
- Kept `inmemory` synchronous: it calls the existing adapters directly and does not require an Operation transaction.
- AMQP subscribers only read/write local state and call `OperationWriter.Schedule`; external protocols run in Operation handlers.
- Added the Governance Casbin member-role sync operation so its Inbox transaction can commit roles, binding, Inbox and Operation together.
- Unrelated plugin, frontend, progress, Task 5 report, binaries and logs were not included.

## Subscription matrix

| Subscription | Local/Inbox action | Operation type | Owner | Target / idempotency | Reconciliation |
| --- | --- | --- | --- | --- | --- |
| `product.workflow-run-completed` | Persist Inbox and completion operation | `product.workflow.trigger.v1` | subscriber | workflow run ID | Reload release-plan/sprint execution state; terminal state guards make replay convergent |
| `product-workflow.build-completed` | Persist Inbox and completion operation | `product-workflow.build.trigger.v1` | subscriber | build ID | Scheduler reloads run/version and skips terminal build refs |
| `product-workflow.deploy-completed` | Persist Inbox and completion operation | `product-workflow.deploy.trigger.v1` | subscriber | deployment run ID | Scheduler reloads run/version and skips terminal deploy refs |
| `workspace-app.deploy-completed.archive-coordinator` | Validate archiving project and persist operation | `workspace-app.project.undeploy.v1` | subscriber | project ID | Handler reloads project, skips non-archiving projects, queries online clusters/active runs before undeploy |
| `delivery.build-completed.deployrun` | Move deployment run to `deploying` and persist queue operation in the same Inbox transaction | `delivery.queue.deploy.publish.v1` | subscriber | deployment run ID | Handler reloads run and skips terminal runs; task consumer state guards duplicate queue deliveries |
| `delivery.build-completed.webhook` | Read enabled hooks and persist one operation per matching hook | `delivery.webhook.send.v1` | subscriber | webhook ID | At-least-once; sends stable event/idempotency headers |
| `delivery.deploy-completed.webhook` | Read enabled hooks and persist one operation per matching hook | `delivery.webhook.send.v1` | subscriber | webhook ID | At-least-once; sends stable event/idempotency headers |
| `delivery.secret-updated.credential-sync` | Persist credential-sync operation | `delivery.jenkins.credentials.sync.v1` | subscriber | secret ID | Jenkins implementation queries folder/credential state and performs idempotent upsert |

Stable keys use `<eventID>:<subscriber>:<operationType>:<target>`. Command-created deploy/rollback/undeploy operations use the deployment run ID as the stable event/target component and owners `delivery.command.create-deployment`, `delivery.command.rollback-deployment`, and `delivery.command.undeploy`.

## Registered operation handlers

- Delivery: webhook send, Jenkins credential sync, deploy queue publish.
- Workspace App: project undeploy, pipeline disable.
- Product Workflow: build completion/trigger, deploy completion/trigger, webhook send, SMTP send, Feishu-user send.
- Product: workflow completion/trigger, Jira transition, release-plan webhook send.
- Governance: Casbin member-role sync.

Registration is guarded by `OperationRegistry.Register`; duplicate types fail startup. A runtime registry test registers all 14 production handler constructors, resolves every scheduled type, and proves duplicate registration fails.

## Idempotency and reconciliation

- Webhooks include `X-SparkOps-Event-ID` and `X-SparkOps-Idempotency-Key`; non-2xx responses retry, permanent 4xx responses fail observably, and delivery remains intentionally at-least-once.
- Deploy queue payloads include event and idempotency identifiers. Before publishing, the handler reloads the deployment run and skips terminal state.
- Jenkins folder creation queries state first and tolerates create conflicts; credential synchronization uses upsert semantics.
- Jira operation queries the issue and skips a transition when the requested state is already present.
- Workflow/release/archive handlers reload persisted state and rely on terminal/version/state guards before repeating work after an external-success/`MarkCompleted` failure window.
- Operation dispatcher tests cover retry, permanent failure, unknown type, lease loss and completed-operation exclusion.

## TDD evidence

RED observed:

- Webhook test failed because `NewDispatcher` had no `OperationWriter`.
- Secret-updated test failed because the handler constructor had no writer and still launched a goroutine.
- Build-completed deploy test failed because the handler still called the queue publisher.
- Governance real-Inbox test initially failed because no Operation row/table existed.

GREEN:

- Webhook and secret handlers schedule stable-key operations without calling their external adapters.
- Build-completed commits deployment state plus deploy-queue operation and does not publish directly.
- Governance real Inbox commits roles, owner binding, Inbox and Casbin operation atomically.
- Webhook operation protocol test verifies both stable idempotency headers and permanent HTTP rejection classification.
- Static subscription test covers all eight former sentinel paths; production registration test covers all operation types.

## Risks

- Webhooks are at-least-once by design; receivers must honor the supplied idempotency key.
- RabbitMQ task queue publication can repeat after publish success but before Operation completion persistence; deployment-run and consumer state guards are the second line of defense.
- External systems cannot participate in the PostgreSQL transaction. Correctness depends on the documented state queries/upserts and Operation retries.
- The local `inmemory` driver remains synchronous and non-durable; production reliability still requires AMQP/outbox enablement in Task 11.

## Task 10 review-fix evidence

### Corrected external-operation matrix

| Path | Operation type | Owner / target | Idempotency and reconciliation |
| --- | --- | --- | --- |
| Product-workflow webhook notification | `product-workflow.webhook.send.v1` | original completion subscriber / notification ID | Stable event key and outbound idempotency header; 408/429/5xx retry |
| Product-workflow SMTP notification | `product-workflow.mail.send.v1` | original completion subscriber / notification ID | Stable event key forwarded as SMTP `Message-ID`; send errors retry |
| Product-workflow Feishu-user notification | `product-workflow.feishu-user.send.v1` | original completion subscriber / notification ID | Stable event key forwarded to the HTTP protocol; 408/429/5xx retry |
| Product Jira transition | `product.jira.transition.v1` | `product.workflow-run-completed` / provider + issue | Query current issue status before transition; an already-converged issue is success |
| Product release-plan hook | `product.release-plan.webhook.send.v1` | `product.workflow-run-completed` / plan ID | Reload current plan at execution and forward `Idempotency-Key`; HTTP failures are observable |
| Workspace archive pipeline disable | `workspace-app.pipeline.disable.v1` | archive subscriber or command / project ID | Query Jenkins job state before disable and again before archive; archived replay is a no-op |
| Delivery build webhook | `delivery.webhook.send.v1` | `delivery.build-completed.webhook` / webhook ID | Payload stores webhook ID only; handler reloads latest URL/token; 408/429/5xx retry |
| Delivery deploy webhook | `delivery.webhook.send.v1` | `delivery.deploy-completed.webhook` / webhook ID | Same protected lookup and retry policy as build webhook |

### RED / GREEN

- RED: workflow notification tests did not compile because mail/Feishu operation types and scheduler writer support were absent. GREEN: three target-specific rows are persisted and duplicate scheduling remains one row per target.
- RED: archive tests showed Jenkins disable failure still returned success and archived the project. GREEN: failed disable leaves `archiving`; the operation queries state, disables, confirms, then archives transactionally.
- RED: delivery webhook constructor lacked runtime lookup and payload assertions exposed URL/token persistence. GREEN: payload contains webhook ID plus non-sensitive event data only and the handler reloads current credentials.
- RED: 408 tests were classified permanent. GREEN: 408, 429 and 5xx are temporary for delivery and product-workflow webhook handlers.
- RED: runtime production-chain coverage relied on source counting. GREEN: actual handler constructors are registered in a real registry; SQLite transaction tests persist Jira, release-plan, notification, archive and delivery webhook rows and prove stable-key deduplication.
- Operation-writer failure test proves local DB and Inbox rows roll back together and the delivery is retried.

### Remaining risk

- SMTP and third-party webhook protocols cannot atomically share PostgreSQL completion state. Stable protocol idempotency metadata plus query-before-write reconciliation reduce the external-success/`MarkCompleted` retry window; receivers that ignore idempotency metadata remain at-least-once.

# Task 10 Report: Prometheus named-template metrics

## Summary

- Implemented `POST /api/v1/plugins/kubernetes/clusters/:clusterId/metrics` under the existing Kubernetes plugin membership group.
- Metrics requests accept only named templates plus validated parameters; arbitrary PromQL request fields are rejected.
- Prometheus settings are read from `plugin_kubernetes.cluster_settings` only after `Clusters.ListMeta(wsID)` confirms the cluster is authorized for the workspace.
- Prometheus upstream calls use a 10s client timeout, fixed API path whitelist, redirect blocking, JSON content-type validation, 8MiB response cap, and fixed error responses that do not echo upstream body, URL, password, or PromQL.

## Templates

- `node_cpu`: requires `node`
- `node_memory`: requires `node`
- `pod_cpu`: requires `namespace`, `pod`
- `pod_memory`: requires `namespace`, `pod`

All provided `node`, `pod`, and `namespace` values must match `^[a-z0-9.-]+$`. Allowed ranges are `5m`, `15m`, `1h`, `6h`, and `24h`.

## TDD Evidence

- RED: `go test ./internal/plugins/kubernetes/metrics` failed for `TestBuildRangeQueryRejectsUnsafeUnusedParameters` because unsafe unused parameters were accepted.
- GREEN: after validating every provided metric parameter, `go test ./internal/plugins/kubernetes/metrics` passed.
- RED: `go test ./internal/plugins/kubernetes/metrics` failed for `TestClientRejectsNonPrometheusAPIPaths` because a non-Prometheus path was requested.
- GREEN: after adding client path whitelist enforcement, `go test ./internal/plugins/kubernetes/metrics` passed.

## Verification

- `go test ./internal/plugins/kubernetes/metrics ./internal/plugins/kubernetes/http ./internal/plugins/kubernetes` passed.
- `ReadLints` on edited Task 10 files reported no linter errors.

## Commit

- Existing Task 10 commit on current branch: `8e336d6 Query Prometheus with named templates from cluster settings.`
- Diff from base `b078033764a468a2fd491582984097f5a1382b96` is limited to Kubernetes metrics route, metrics package, router registration, and bundled descriptor/test updates.
- This report file is intentionally uncommitted.
# Task 10 Report: Prometheus named-template 指标查询

## Status

**DONE**

## Summary

Implemented `POST /api/v1/plugins/kubernetes/clusters/:clusterId/metrics` under the existing workspace membership group. The endpoint accepts only named templates plus validated parameters, reads Prometheus settings from `plugin_kubernetes.cluster_settings`, and proxies only to the configured Prometheus `/api/v1/query_range` endpoint.

## TDD steps

| Step | Result |
|------|--------|
| 1. Write `metrics/query_test.go` first | Asserted named template construction, required params, safe parameter regex, range whitelist, RFC3339/Unix time parsing |
| 2. Run metrics package RED | Build failed as expected: `undefined: BuildRangeQuery`, `undefined: QueryRequest`, sentinel errors missing |
| 3. Write `http/metrics_test.go` first | Asserted route behavior, strict JSON, authorized cluster check before settings lookup, not-configured response, upstream redaction, vector/matrix JSON handling |
| 4. Run HTTP package RED | Failed as expected with 404 because `/metrics` was not mounted |
| 5. Implement metrics package + HTTP handler + router + descriptor updates | Done |
| 6. Re-run tests | PASS |

## Files changed (committed)

| Path | Role |
|------|------|
| `backend/internal/plugins/kubernetes/metrics/query.go` | Named-template PromQL construction, range/time validation, Prometheus client safety limits |
| `backend/internal/plugins/kubernetes/metrics/query_test.go` | RED/GREEN coverage for templates and validation |
| `backend/internal/plugins/kubernetes/http/metrics.go` | Gin handler for auth/settings/query/response mapping |
| `backend/internal/plugins/kubernetes/http/metrics_test.go` | Handler tests for success, rejection, authorization ordering, config, upstream errors |
| `backend/internal/plugins/kubernetes/http/router.go` | Mounted `POST /clusters/:clusterId/metrics` under membership-protected group |
| `backend/internal/plugins/kubernetes/plugin.go` | Bundled descriptor installed files updated |
| `backend/internal/plugins/kubernetes/plugin_test.go` | Descriptor regression updated |

## Verification

```text
cd backend
go test -count=1 ./internal/plugins/kubernetes/metrics ./internal/plugins/kubernetes/http ./internal/plugins/kubernetes
# ok github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/metrics 0.909s
# ok github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes/http 4.463s
# ok github.com/sparkops/sparkops-agent/backend/internal/plugins/kubernetes 3.422s

git diff --check -- <task10 files>
# no whitespace errors; Git printed expected CRLF warnings on Windows
```

## Commit

- `8e336d6` — Query Prometheus with named templates from cluster settings.
- Only the 7 Task 10 files above were committed; this report is intentionally uncommitted.

## Self-review

- No arbitrary PromQL `query` field is accepted; JSON decoder rejects unknown fields.
- Supported templates: `node_cpu`, `node_memory`, `pod_cpu`, `pod_memory`.
- `node` / `pod` / `namespace` are required per template and must match `^[a-z0-9.-]+$`.
- Range whitelist is exactly `5m`, `15m`, `1h`, `6h`, `24h`; unknown template and invalid input return fixed 400 messages.
- Handler checks workspace-authorized cluster via `Clusters.ListMeta(wsID)` before reading settings or calling Prometheus.
- Prometheus URL/basic auth are read from `plugin_kubernetes.cluster_settings`; missing URL returns 400 `PLUGIN_PROMETHEUS_NOT_CONFIGURED`.
- Prometheus client uses 10s timeout, disables redirects, limits response reads to 8MiB, enforces JSON content type and vector/matrix result type, and maps upstream failures to fixed 502 `PLUGIN_PROMETHEUS_UPSTREAM` without echoing body, URL, password, or PromQL.

## Concerns

None blocking. The Cursor shell auto-injected `git commit --trailer`, but installed Git 2.31.1 does not support that option. The repository has no active hooks, so the final commit was created with `git commit-tree` / `git update-ref` after `git commit -F` failed twice for the same injected option.

## Final external-operation review fixes (2026-10-18)

### Seven-item acceptance matrix

1. **Archive ResumeAll** — each archiving project is handled in its own transaction. A UUIDv5 event ID derived from `projectID` and stable subscriber `workspace-app.archive-resumer` deduplicate restart replays. The resumer performs no teardown calls; it schedules `workspace-app.project.undeploy.v1`, whose handler reconciles online/active state and schedules `workspace-app.pipeline.disable.v1`. Pipeline disable is queried, applied, queried again, and only then is the project archived.
2. **Product-workflow notification identity** — keys contain run ID, workflow/job scope, job ID sentinel, event, status, and note ID. SQLite transaction tests prove job/workflow notifications with the same note ID create distinct rows and duplicate delivery remains one row. Webhook payloads contain note/run references only and exclude URL/token values.
3. **Runtime webhook resolution** — `WorkflowWebhookResolver` reloads the run, workflow/job notification, active state, URL, and interpolated body at handler execution. No credential-bearing URL is persisted in the outbox payload.
4. **Release-plan hooks and approval** — webhook and approval delivery are separate operation types. HTTP command transitions run in a database transaction with scheduling; scheduling errors roll state back. AMQP paths no longer call or swallow webhook/approval delivery. Handlers reload the current plan and classify permanent HTTP rejection.
5. **Mail and Feishu errors** — missing SMTP host/recipients and Feishu credentials/recipients return permanent errors. Feishu token and message HTTP-200 responses parse business `code`; nonzero codes fail, with throttling classified retryable.
6. **GitLab status** — scheduler state transitions schedule `product-workflow.gitlab.status.report.v1` transactionally. The handler queries the current context status and skips an already-converged value; the adapter propagates lookup/query/write errors instead of logging and swallowing them.
7. **Production registration** — product and product-workflow expose strongly typed `OperationFactory` values with `Validate` and fail-fast registration. The bootstrap test assembles all 16 Task10 operation types with non-nil production constructor dependencies and verifies every handler plus duplicate rejection.

### RED / GREEN evidence

- RED: product-workflow failed to compile because the old GitLab reporter did not implement `CurrentStatus` or error-returning `Report`. GREEN: adapter protocol tests, reconciliation handler, production factory registration, and module build pass.
- RED: Feishu accepted HTTP 200 with nonzero business code when a token field was present. GREEN: nonzero token/message codes always fail with retry/permanent classification.
- RED: archive resumer and archive command invoked teardown state directly before scheduling. GREEN: tests assert zero teardown calls before the advance operation executes; restart replay persists one stable operation.
- RED: release-plan had no independent approval operation and direct hook/approval calls existed outside state transactions. GREEN: three real outbox rows (Jira, release hook, approval) persist/deduplicate, and an induced schedule failure rolls state back.
- GREEN-only regression evidence retained from the previous session: notification cross-layer key separation, secret-free payload, mail configuration, webhook 408 handling, and archive finalization tests.

### Verification

- `go test -count=1` passed for delivery, governance, product, product-workflow, workspace-app, messaging, messaging persistence, postgres, and bootstrap packages.
- `go test -count=2` passed for product service/command, product-workflow engine, workspace archive command, and bootstrap registration.
- `go build ./...` passed.
- `git diff --check` reported no whitespace errors (Windows LF/CRLF notices only).

### Residual risks

- External protocols remain at-least-once across an external-success/outbox-completion crash window. Stable idempotency headers and query-before-write reconciliation are required at receivers.
- Release-plan approval/webhook endpoints that ignore `Idempotency-Key` can still observe duplicate delivery.
- Inmemory mode intentionally keeps synchronous compatibility and has no durable retry.

## Final Important/Minor review closure (2026-10-18)

### Acceptance evidence

1. **Archive retry lifecycle** — project undeploy performs cluster/run queries and idempotent undeploy start outside database transactions. Online or active state returns a retryable error so the fixed operation remains pending. Only confirmed offline/no-active state opens a short transaction, rechecks `archiving`, and schedules pipeline disable. The integration test covers online → start → retry → lost completion event → offline reconciliation → pipeline disable → archived, plus schedule failure after external convergence.
2. **Deleted webhook configuration** — soft-deleted workflow/config references resolve as inactive and complete as an idempotent no-op. Other repository failures remain retryable; changed active configuration is loaded at execution time.
3. **Release hook/approval validation** — enabled external delivery validates an HTTP(S) URL with host and required token/config before state persistence or operation scheduling. Missing/invalid configuration rolls the state transaction back; operation handlers repeat validation and return permanent errors.
4. **Bounded GitLab I/O** — status reads and writes use the same injected `http.Client`; production defaults to a 12-second timeout. A hanging-server test proves both paths return within the configured deadline even when operation execution uses a cancellation-detached context.
5. **Strong production factories** — delivery, workspace-app, and governance now join product/product-workflow with typed `OperationFactory`, `Validate`, and module-owned registration. The real bootstrap assembly covers all 16 operation types, non-nil handlers, duplicate rejection, and missing-dependency fail-fast.
6. **Feishu token classification** — the bounded token body is read, HTTP status is classified first, and only successful 2xx bodies are decoded. HTML/empty 401 responses are permanent; 408/429/5xx remain retryable.

### Additional RED / GREEN evidence

- RED: archive operation tests failed against the transaction-bound constructor and completed after starting undeploy. GREEN: external calls are outside the transaction and online/active results force retry until reconciliation converges.
- RED: deleted workflow resolution returned not-found and retried forever. GREEN: semantic not-found is inactive/no-op while injected database errors remain retryable.
- RED: missing release hook/approval URLs could create waiting states. GREEN: command tests prove validation occurs before token/state persistence, and notifier tests prove invalid runtime configuration is permanent.
- RED: `Reporter` had no injectable HTTP client. GREEN: hanging read/write tests pass with a 25ms client timeout.
- RED: Feishu 401 HTML/empty bodies surfaced as JSON decode errors. GREEN: status-first tests prove permanent 4xx and retryable throttling/server errors.
- RED: delivery/workspace-app/governance factory tests did not compile because typed factories were absent. GREEN: production modules use their factories and bootstrap registration passes.

## Final convergence review closure (2026-10-18)

1. **Legacy archive rejection** — an AMQP `project.undeploy` operation with `allow_undeploy=false` and an online cluster now opens a short database transaction, reloads the project, aborts `archiving`, persists the restored active state, and returns success. It does not query active runs, start undeploy, or schedule pipeline disable. Replays after restoration are idempotent no-ops; the inmemory command path still returns `ErrProjectHasOnlineEnvironments`.
2. **Release-hook retry convergence** — the durable payload records the pending hook token as a state reference. Before sending, the production handler reloads the plan and requires a matching pending event/token. Cleared, advanced, failed, or superseded hook state completes as a no-op. Only a matching pending state reaches notifier validation, where malformed URL/token configuration remains permanent. The regression simulates successful delivery, callback state clearance, failed completion persistence, and a harmless operation retry without duplicate delivery.
3. **Production operation manifest** — all five typed factories expose `Descriptors()` and use the same descriptors for production registration. Bootstrap assembles the real factories, combines the manifests, verifies all 16 non-nil unique handlers, and checks duplicate rejection. Factory dependency validation remains fail-fast.

### RED / GREEN

- RED: legacy rejection returned `project remains online` forever. GREEN: persisted AMQP lifecycle reaches `completed`, restores the project, and replay performs no external work.
- RED: release hook operations had no persisted token reference or current-state guard. GREEN: matching pending state sends once; callback-cleared or mismatched state converges with nil; truly invalid matching configuration is permanent.
- RED: bootstrap registration tests duplicated operation-type lists and the factories had no shared descriptor API. GREEN: tests and production registration consume the same factory manifests.
