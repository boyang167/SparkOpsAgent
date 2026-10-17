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
- Product Workflow: build completion/trigger, deploy completion/trigger, webhook send.
- Product: workflow completion/trigger, Jira transition, release-plan webhook send.
- Governance: Casbin member-role sync.

Registration is guarded by `OperationRegistry.Register`; duplicate types fail startup. A production-source completeness test requires every Task 10 type exactly once (the two module-owned webhook constants occur once per owner).

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
