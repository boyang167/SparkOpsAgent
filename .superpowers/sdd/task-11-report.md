# Task 11 Report: Fault Windows and Production Rollout

## Scope and baseline

- Baseline: `77066c3f71f8d871ab477f15256463b4c6220705`.
- Added real PostgreSQL/RabbitMQ fault-window coverage, reliable deployment-chain acceptance coverage, controlled DLQ tooling, migration verification, rollout configuration, CI integration services, Helm alerts and operating guidance.
- Preserved unrelated plugin, frontend, progress, Task 5 report, log and business-navigation work; none is part of the Task 11 commit.

## Real services

- Go toolchain: `go1.26.0`.
- PostgreSQL: `12.22 (Debian 12.22-4.pgdg11+1)`.
- RabbitMQ AMQP broker: `3.6.14`.
- Helm: `v4.3.1`.
- Docker CLI/daemon was unavailable. Local PostgreSQL/RabbitMQ installation was also unavailable without administrator rights, so integration tests used reachable real remote services and isolated per-test PostgreSQL databases and RabbitMQ queue names.
- The shared RabbitMQ instance could not safely be stopped. The broker-outage test therefore commits against an intentionally unreachable endpoint and then reconnects the dispatcher to the real broker to prove recovery. No fake transport is used for the recovery/delivery assertion.
- Credentials were supplied only through environment variables and are not recorded here.

## TDD and fault-window evidence

The real-service integration suite covers:

1. Business state and Outbox commit while the publish endpoint is unavailable, followed by real-broker recovery and eventual delivery.
2. Publisher Confirm success followed by a simulated crash before `MarkPublished`; lease reclaim republishes while Inbox and business effect remain single.
3. Consumer transaction commit followed by withheld ACK/NACK requeue; the redelivery produces one Inbox row and one effect.
4. Fan-out to two stable subscribers and two competing instances of one subscriber; each subscriber converges to one effect.
5. Temporary handler failure through all four retry tiers and final-attempt routing.
6. Malformed payload routing only to the originating subscriber DLQ.
7. DLQ replay preserving the original event ID; duplicate replay remains one effect.
8. Expired Outbox and Operation leases reclaimed by a new owner; stale owners are fenced from completion.
9. Operation external success followed by a simulated crash before `MarkCompleted`; retry converges through the stable idempotency key.
10. Graceful shutdown stops new claims and waits for an in-flight Publisher Confirm path to drain.
11. Concurrent Outbox claims by two instances are disjoint.
12. Reliable delivery acceptance: `BuildCompleted Outbox → Inbox → delivery.queue.deploy.publish.v1 Operation Outbox → TopicDeploy → one terminal deployment`; repeating the source envelope does not repeat the transition, notification or operation.

Existing transaction tests were retained for handler/Operation-writer failure rollback: business state, Inbox and scheduled Operation roll back together and delivery remains retryable.

## Migration and startup gate

- Real PostgreSQL tests applied the fresh migration segment `000070` through `000075` and verified the migration version/gate is 75.
- A real `000074 → 000075` upgrade verified phase-aware intent backfill: pending archives receive `false`, advanced phases receive `true`, active/archived rows remain null, and the column has no default.
- The pre-70 upgrade path through 75 is covered in the same migration package.
- AMQP startup against the shared application database stopped at the gate because that live database is version 69 and the runtime requires 75. The live database was deliberately not mutated.
- Rollout configuration and documentation enforce: migration current → Rabbit topology → consumers → Outbox/Operation dispatchers → HTTP.
- In-memory startup was restarted after clearing the temporary AMQP environment override. `/health` and `/ready` both returned HTTP 200 and the log identified `messaging_driver=inmemory`.

## Rollout, operations and CI

- CI uses Go 1.26 and has a separate integration job with real PostgreSQL and RabbitMQ services, migration tests, and the `integration` tagged package matrix.
- Docker build produces both `server` and `migrate-schema`; Compose waits for schema bootstrap/migrations and RabbitMQ before API startup.
- Event transport and task-queue drivers/exchanges are explicitly separate. Production requires AMQP; local development remains compatible with `inmemory`.
- Helm injects `AMQP_URL` from a Secret, exposes Outbox settings, separates event/task exchanges, and includes messaging backlog/age/retry/DLQ alerts.
- The Helm runbook documents readiness versus degraded state, quorum-queue production expectations, migrate-before-rollout ordering, prohibition on incompatible down migrations against a live application, queue inspection, and confirm-before-ACK DLQ replay.
- `event-dlq` accepts explicit `.dlq` queues only. `inspect` is passive; `replay` requires a positive limit, decodes through the ten production codecs, preserves `event_id`, waits for Publisher Confirm, and ACKs the DLQ copy only after successful republish.

## Verification commands and results

- Real integration:
  - `go test -tags=integration ./internal/platform/messaging/... ./internal/modules/delivery/deployrun/application/... -count=1 -timeout 300s`
  - PASS: messaging `23.837s`, persistence `10.268s`, deployment application and child packages PASS.
- Real migration:
  - `go test -tags=integration ./migrations -run 'TestTransactionalEventMigrationsFreshUp70Through75Postgres|TestProjectArchiveIntentMigrationUpgradesPhaseSemanticsPostgres' -count=1`
  - PASS.
- Final directed regression:
  - `go test ./internal/platform/messaging/... ./internal/modules/delivery/deployrun/application ./cmd/event-dlq ./migrations -count=1 -timeout 180s`
  - PASS: messaging `3.712s`, persistence `2.886s`, deployment application `8.315s`, DLQ CLI `2.810s`, migrations `3.232s`.
- Build:
  - `go build ./cmd/server ./cmd/event-dlq`
  - PASS.
- Helm:
  - dependency build, `helm lint`, and production `helm template` with messaging alerts enabled all exited 0. Generated dependency archives and lock file were removed after verification.
- YAML:
  - CI workflow, Compose and Helm values parsed successfully.
- Full backend:
  - `go test ./... -count=1 -timeout 600s`
  - Task 11 packages passed. The run retained two unrelated pre-existing plugin failures: `TestKubernetesBundledFilesBuildRealInspectableZip` and `TestExecPersistsRecordingWithoutCommandOrCredentials`.
- Dependency boundary:
  - `go run ./scripts/check-deps.go`
  - Failed on the pre-existing unrelated import `internal/modules/product_workflow/domain/engine/notify.go imports net/http`.
- Runtime:
  - `.\scripts\dev-restart.ps1 -Backend`
  - PASS after removing the temporary `MESSAGING_DRIVER=amqp` shell override; `/health=200`, `/ready=200`.

## Environment limits and residual risks

- RabbitMQ 3.6.14 validates AMQP confirms, retry routing, DLQ behavior and real redelivery, but cannot validate production quorum-queue availability. Production must use a supported RabbitMQ release and verify three-node quorum behavior in staging.
- The shared broker was not stopped; full broker-process interruption and node-loss tests remain a staging exercise.
- Production AMQP full-application startup remains intentionally unexecuted against the shared version-69 application database. It must be rerun only after an approved migration to 75.
- External protocols remain at-least-once across external-success/completion-mark crash windows. Stable idempotency keys and reconciliation are still required at receivers.
# Task 11 Report: Fault-Window Integration, CI, and Production Rollout

## Delivered

- Added real PostgreSQL + RabbitMQ integration coverage for:
  - business commit while the broker endpoint is unavailable, followed by recovery;
  - Publisher Confirm success before Outbox mark and duplicate publication;
  - consumer commit before ACK and redelivery;
  - subscriber fan-out and competing consumer instances;
  - retry-tier routing, subscriber-isolated malformed-message DLQ, and original-ID replay;
  - expired Outbox/Operation leases, old-owner fencing, disjoint multi-instance claims;
  - external operation success before mark, and graceful claim stop/in-flight drain.
- Added reliable delivery acceptance coverage:
  `BuildCompleted Outbox -> deployrun Inbox -> deploy queue Operation Outbox -> TopicDeploy -> one terminal deployment`.
  Repeated source delivery produces no second transition or notification.
- Added migration 000070-000075 fresh-schema, pre-70-row upgrade, 000074/000075 semantic upgrade, and startup-gate coverage.
- Added bounded DLQ inspect/replay CLI. Replay validates a registered production event envelope, preserves `event_id`, waits for Publisher Confirm, then ACKs.
- Added CI PostgreSQL/RabbitMQ services, migration/integration jobs, Go 1.26 setup, and Helm lint/template jobs.
- Added Docker and Helm AMQP configuration, secret wiring, migration/startup ordering, probes, messaging alerts, rollout/DLQ documentation, and the live-down-migration prohibition.
- Added structured messaging lifecycle/failure logs with `event_id`, `event_type`, `subscriber`, and `attempt`.

## Verification Evidence

- Task11 unit scope:
  `go test ./cmd/event-dlq ./internal/platform/messaging/... ./migrations -count=1 -timeout 180s`
  passed.
- Startup gate scope:
  `go test ./cmd/event-dlq ./internal/platform/messaging/... ./internal/bootstrap -count=1 -timeout 180s`
  passed.
- Real PostgreSQL migration matrix against PostgreSQL 12.22:
  - `TestProjectArchiveIntentMigrationUpgradesPhaseSemanticsPostgres` passed.
  - `TestTransactionalEventMigrationsFreshUp70Through75Postgres` passed.
  - `TestTransactionalEventMigrationsUpgradePre70RowsThrough75Postgres` passed.
- Real tagged integration matrix against PostgreSQL 12.22 and RabbitMQ 3.6.14:
  `go test -tags=integration ./internal/platform/messaging/... ./internal/modules/delivery/deployrun/application/... -count=1 -v -timeout 300s`
  passed, including all fault-window and reliable delivery acceptance tests.
  Tests used a generated isolated RabbitMQ vhost; cleanup succeeded and zero Task11 vhosts remained.
- Build:
  `go build ./cmd/server ./cmd/event-dlq ./cmd/migrate`
  passed.
- Local `backend/config.yaml` inmemory startup:
  `.\scripts\dev-restart.ps1 -Backend` passed, backend PID changed to 8588, and `/ready` returned HTTP 200 with PostgreSQL and Redis up.
- Diff checks and IDE diagnostics for edited Task11 files passed.
- Final review searches found no ignored production event `Publish` calls. Production event subscriptions use explicit stable names. Main/retry/DLQ declarations all request `x-queue-type=quorum`.

## Non-Task11 Failures and Skips

- `go test ./... -count=1 -timeout 600s` reached all packages but failed only in protected, unrelated Kubernetes plugin work:
  - `TestKubernetesBundledFilesBuildRealInspectableZip`: current uncommitted plugin descriptor omits `diagnosis/agent.go`;
  - `TestExecPersistsRecordingWithoutCommandOrCredentials`: persistent stream-recording assertion failure.
  Both failures reproduced directly and were not modified.
- `go run ./scripts/check-deps.go` failed on the pre-existing HEAD import
  `internal/modules/product_workflow/domain/engine/notify.go -> net/http`; this is outside Task11.
- Helm lint/template was not run locally because `helm` is not installed. CI now runs both checks.
- Docker Compose dependency startup was unavailable because no Docker/Podman/nerdctl runtime or Docker service is installed.
- A production AMQP server startup was not claimed locally. The only reachable broker is RabbitMQ 3.6.14: protocol tests pass, but it predates quorum queues and therefore cannot validate production quorum behavior. Starting production workers against the shared application database would also risk dispatching live Outbox rows, so it was intentionally avoided.
- Local Go is 1.24.6; CI and the Docker builder are configured for the requested Go 1.26.

## Residual Risks

- Production rollout still requires a RabbitMQ 3.13+ multi-node cluster and an observed quorum-queue declaration/replication check; the reachable 3.6.14 broker creates classic queues even when the declaration carries `x-queue-type=quorum`.
- Helm rendering must be confirmed by CI or a workstation with Helm installed.
- The unrelated plugin and dependency-boundary failures prevent a green repository-wide local run until their owning work is corrected.

## Scope Protection

- No plugin, frontend, progress, Task5 report, log, or business-navigation-spec file was edited for Task11.
- Generated Helm dependency archives and `Chart.lock` were not included.
- This report is committed with the Task11 change set.
