# Task 9 Report: Named Subscribers and Local Database Effects

## Scope

- Baseline: `aa55c526ce78528d01dde68893645b894d4b89f1`
- Migrated only named subscriptions and local PostgreSQL effects.
- Preserved synchronous `inmemory` behavior.
- Did not implement Task 10 Operation Outbox writers/executors.
- Did not add a temporary AMQP configuration gate; production enablement remains Task 11 after Task 10 closes the operation gap.
- Unrelated plugin, frontend, progress, Task 5 report, generated files, binaries, and logs are excluded from the Task 9 commit.

## Subscription matrix

| Subscriber | Event | Task 9 DB effect | Deferred to Task 10 |
| --- | --- | --- | --- |
| `product.build-completed` | `delivery.BuildCompleted` | Mark matching running requirement completed. | None |
| `product.workflow-run-completed` | `product_workflow.WorkflowRunCompleted` | Release-plan and sprint repositories use Inbox tx. | Workflow/Jira/webhook work; Inbox delivery retryably defers the mixed handler |
| `product.deploy-completed` | `delivery.DeployCompleted` | Update environment-project state; stale events cannot overwrite newer state. | None |
| `product-workflow.build-completed` | `delivery.BuildCompleted` | Workflow-run repositories use Inbox tx. | Build/deploy/webhook work; Inbox delivery retryably defers the mixed handler |
| `product-workflow.deploy-completed` | `delivery.DeployCompleted` | Workflow-run repositories use Inbox tx. | Build/deploy/webhook work; Inbox delivery retryably defers the mixed handler |
| `workspace-app.workspace-created.seed-default-templates` | `integration.WorkspaceCreated` | Seed 14 templates; extracted named subscriber. | None |
| `workspace-app.deploy-completed.archive-coordinator` | `delivery.DeployCompleted` | Project repository uses Inbox tx. | Kubernetes/Jenkins teardown; Inbox delivery retryably defers the mixed handler |
| `delivery.build-completed.deployrun` | `delivery.BuildCompleted` | Deployment-run repository uses Inbox tx. | Deploy queue publish; Inbox delivery retryably defers the mixed handler |
| `delivery.build-completed.webhook` | `delivery.BuildCompleted` | None | HTTP webhook; Inbox delivery defers before I/O |
| `delivery.deploy-completed.webhook` | `delivery.DeployCompleted` | None | HTTP webhook; Inbox delivery defers before I/O |
| `delivery.secret-updated.credential-sync` | `integration.SecretUpdated` | None | Jenkins sync; Inbox delivery defers before goroutine/I/O |
| `notification.run-completed` | `agent_runtime.RunCompleted` | Agent read and one notification insert use Inbox tx. | None |
| `notification.workflow-run-completed` | `workflow.WorkflowRunCompleted` | One notification insert. | None |
| `notification.build-completed` | `delivery.BuildCompleted` | One notification insert. | None |
| `governance.workspace-created.bootstrap` | `integration.WorkspaceCreated` | Attempts role and owner-binding seed in Inbox tx, then returns the retry sentinel so all writes roll back. | Casbin sync must be represented by an Operation before DB state may commit |

All 15 names are asserted exactly once across the six production module registration files. Duplicate `(eventName, subscriber)` registration remains rejected by both buses.

## Transaction semantics

- `InboxConsumer` marks the handler context.
- Pure DB handlers use `postgres.DB(ctx, fallback)` for consistency reads and writes.
- Handler failure rolls back Inbox and DB effects. A confirmed retry copy allows the original delivery to be acknowledged; if retry publication is unconfirmed, the original is not acknowledged.
- Repeated `(event_id, subscriber)` delivery skips the handler and acknowledges without repeating effects.
- The eight mixed/external subscriptions return retryable `messaging.ErrOperationOutboxRequired` before any external call. Their Inbox and interim DB work do not commit until Task 10 can atomically schedule an Operation.
- Governance returns the same sentinel after its transactional role/binding writes but before Casbin sync, rolling the Inbox and governance rows back together. The non-Inbox/inmemory path still calls `SyncMemberRole`.
- This is an intentionally unavailable, retrying safety state and is not production-ready AMQP behavior.

## TDD evidence

Observed RED before implementation:

- Notification, governance, template, product, product-workflow, and deployment-run repository transaction tests failed while repositories bypassed tx context.
- Product stale deployment event test failed because older state overwrote newer state.
- Inbox context test failed before the marker API existed.
- Review RED: the real Governance `InboxConsumer` object graph committed Inbox, roles, and owner binding instead of retrying; expected retry count was 1, actual was 0.

GREEN coverage:

- Real repository rollback tests for notification, governance, templates, product requirement/environment, product-workflow run, and deployment-run storage.
- Real DB `InboxConsumer` test with `postgres.Transactor`, `InboxStore`, `Registry`, and the production Governance handler verifies sentinel rollback leaves no Inbox, role, or binding row.
- Real Inbox success/duplicate tests verify template, product build, and product deploy DB effects commit once.
- Production subscription completeness/uniqueness test.
- Static safety test covers all eight deferred subscriber names and requires the Inbox sentinel guard before their first DB/external side-effect call.
- Existing Inbox rollback, duplicate, retry-confirm, and ACK-boundary tests.

## Risks

- Eight mixed/external subscriptions intentionally retry under AMQP until Task 10 replaces deferral with atomic DB progression plus Operation scheduling. Governance bootstrap also retries until Casbin sync can be scheduled atomically.
- Task 9 must not be treated as production-ready AMQP. Production rollout remains blocked on Task 10 and the planned Task 11 enablement work.
- `inmemory` remains direct/synchronous and is not a durable production mode.
- RabbitMQ fault windows, DLQ replay, and rollout verification remain Task 11.

## Verification

- Task 9 module regression: PASS.
- Messaging, PostgreSQL helper, and production registration regression: PASS.
- Review regression for Governance sentinel rollback, non-Inbox Casbin sync, real Inbox success/duplicate behavior, and all eight deferred paths: PASS.
- `go build ./cmd/server`: PASS.
- Backend restarted; `GET /health`: HTTP 200.
- `go test ./... -count=1 -timeout 600s`: Task 9 packages passed; the command failed only in unrelated, pre-existing Kubernetes plugin work:
  - bundled descriptor missing `diagnosis/agent.go`;
  - stream recording assertion mismatch;
  - Windows cleanup could not unlink a concurrently used `diagnosis.test.exe`.
