# Task 11 Report: Release Blocker Closure

## Scope

- Audited baselines `8b3fd76` and `a9275dc`; completion work started from `a9275dc06fca3edda4bb06f78a578bd7e0267984`.
- The commit is limited to Task 11 messaging, DLQ, migration, rollout, CI, tests, and this report.
- Unrelated plugin, frontend, progress, Task 5 report, log, and business-navigation work remains uncommitted and untouched.

## Release blockers

### 1. Same-runtime RabbitMQ recovery

- The production event transport now owns separate publisher and consumer connections.
- Publisher channel/connection failure retires stale state and redials with bounded exponential backoff.
- Consumer connection loss redials, redeclares the exchange plus all main/retry/DLQ topology, and restores every registered subscription and handler.
- Registration/reconnect state is lock-coordinated; stale reconnect signals are ignored; duplicate queue consumers are rejected; `Close` cancels dial/backoff and waits for the reconnect worker.
- Real RabbitMQ coverage closes both live connections on one transport and proves two subscribers resume on that same instance.

### 2. Queue-targeted DLQ replay

- Replay mandatory-publishes to the default exchange with the selected subscriber main queue as the exact routing key; it does not topic-broadcast.
- Original body, `MessageId`, AMQP properties, and headers including `x-sparkops-attempts` are retained.
- Queue subscriber/event/version identity, envelope structure, and production codec registration are validated before publish.
- A DLQ delivery is ACKed only after publisher confirm and no mandatory return.
- Real two-subscriber coverage proves no cross-delivery; an unroutable target proves return handling leaves the DLQ message available.

### 3. Migration-before-rollout

- Added the packaged `migrate-sql` CLI using the actual `golang-migrate` runner and repository SQL migrations.
- Historical duplicate migration versions are normalized deterministically; the production runner uses the established GORM bootstrap only for an unversioned fresh database and the authoritative SQL 70-75 segment thereafter.
- Docker packages `migrate-schema`, `migrate-sql`, and migration files; Compose gates API startup on migration success.
- Helm supplies `pre-install,pre-upgrade` Secret and Job hooks, `DATABASE_URL`, ordered weights, finite retries/deadline, and the actual packaged runners. Hook failure blocks rollout.
- CI runs `helm lint` and `helm template` and asserts the rendered hook, runner, and secret contract.

### 4. Non-skippable CI integration gates

- Integration helpers Fatal rather than Skip when either `CI=true` or `REQUIRE_INTEGRATION=true` and DSN/AMQP is absent or an isolated PostgreSQL database cannot be created.
- CI probes PostgreSQL, AMQP, and RabbitMQ readiness, then asserts named migration, reconnect, retry, DLQ, and factory-gate tests executed.
- Negative checks were run locally and produced the expected test failures for missing `DATABASE_DSN` and missing `AMQP_URL`.

### 5. Real retry chain

- Temporary failures traverse the actual four retry queues by TTL dead-lettering back to the subscriber main queue.
- Attempt headers increment through `[0,1,2,3,4]`.
- Real PostgreSQL/RabbitMQ coverage proves both eventual success on the final attempt and exhausted delivery to the subscriber DLQ.

### 6. Actual migration runner and startup gate

- Real PostgreSQL tests run the production runner for fresh bootstrap, 69 to 75, and 74 to 75.
- The application factory rejects versions 69 and 74 before migration and starts after the same runner reaches 75.
- No test manually updates a migration version to claim success.

## Fresh verification matrix

Environment:

- Go `1.26.0`.
- PostgreSQL `12.22`, isolated database per test with cleanup.
- Reachable RabbitMQ is `3.6.14`, isolated vhost per run with cleanup.

Passed:

- Core unit/static scope:
  `go test ./cmd/event-dlq ./internal/platform/messaging/... ./internal/platform/dbmigration ./internal/bootstrap ./deployments/helm ./migrations -count=1 -timeout 300s`
- Build:
  `go build ./cmd/server ./cmd/event-dlq ./cmd/migrate ./cmd/migrate-sql`
- Real PostgreSQL:
  - `TestProjectArchiveIntentMigrationUpgradesPhaseSemanticsPostgres`
  - `TestSQLMigrationRunnerFreshReaches75AndPassesStartupGate`
  - `TestSQLMigrationRunnerUpgrades69And74To75WithStartupGate/from-69,from-74`
  - `TestRealMigrationRunnerControlsApplicationFactoryGate/from-69,from-74`
- Real RabbitMQ:
  - `TestRabbitEventTransportSameInstanceReconnectsPublisherAndConsumer` with two restored subscriptions
  - `TestReplayTargetsOnlyRequestedSubscriberAndPreservesHeaders`
  - `TestReplayMandatoryReturnDoesNotAckDLQ`
- Real PostgreSQL plus RabbitMQ:
  - `TestEventPipelineTemporaryFailureUsesEveryRetryTier`
- Full real tagged matrix:
  `go test -tags=integration ./internal/platform/messaging/... ./internal/modules/delivery/deployrun/application/... ./cmd/event-dlq -count=1 -timeout 300s`
- CI fail-closed:
  - missing `DATABASE_DSN` with `CI=true` failed as required
  - missing `AMQP_URL` with `CI=true` failed as required

Attempted full backend regression:

- `go test ./... -count=1 -timeout 600s` reached all packages.
- Task 11 packages passed. Four protected, unrelated working-tree failures remain:
  - plugin persistence schema assertion from current uncommitted plugin changes
  - Kubernetes bundle descriptor missing `diagnosis/agent.go`
  - Kubernetes stream recording assertion
  - config DSN unit test overridden by the local uncommitted `backend/config.yaml`

## Residual risks and environment limits

- RabbitMQ 3.6 verifies real reconnect, confirms, mandatory returns, TTL dead-lettering, attempts, DLQ outcome, and subscription restoration, but it cannot validate quorum queues. CI uses RabbitMQ 3.13 and is the required quorum-declaration gate; staging must still validate three-node quorum node loss.
- Docker and Helm CLIs are unavailable locally. Image build, Compose startup, `helm lint`, and rendered chart checks are therefore CI-owned; Go static Helm contract tests passed locally.
- Windows race builds are not claimed; CI/Linux should run race coverage if required.
- Delivery remains at-least-once across external-success/mark windows, so stable idempotency keys and receiver reconciliation remain required.
# Task 11 Report: Reliable Messaging Fault Windows

## Scope

- Baseline: `77066c3f71f8d871ab477f15256463b4c6220705`; continuation HEAD before this review fix: `a9275dc06fca3edda4bb06f78a578bd7e0267984`.
- This follow-up fixes the six review findings: same-runtime AMQP reconnect, queue-targeted DLQ replay, Helm migration hooks, CI integration gates, real retry TTL routing, and real migration runner/gate coverage.
- Unrelated plugin, frontend, progress, Task 5 report, log, and business-navigation changes were preserved and excluded.

## Delivered

### AMQP reconnect

- One production `amqpEventTransport` now owns bounded, interruptible reconnect loops.
- Publisher connection/channel failures retire the stale connection and redial before publishing.
- Consumer connection loss redials, redeclares the durable topology, and restores every registered subscription/handler.
- Subscription registration is coordinated under locks; stale reconnect signals are ignored; `Close` interrupts retry backoff and waits for reconnect workers.
- A real-broker integration test closes both live publisher and consumer connections on the same transport, then proves publish and consumption recover without constructing a replacement transport.

### Targeted DLQ replay

- `event-dlq replay` no longer calls topic `Publish`.
- It mandatory-publishes the original AMQP delivery to the default exchange with the exact subscriber main queue as routing key.
- Original body, `MessageId`, AMQP properties, and headers including `x-sparkops-attempts` are preserved.
- Queue subscriber/event/version identity and envelope registration are validated before publish.
- The DLQ delivery is ACKed only after publisher confirm and absence of a mandatory return.
- Real integration verifies two subscribers do not cross-receive, headers survive, and an unroutable confirm/return leaves the DLQ message unacked and requeued.

### Retry chain

- A temporary failure now traverses four bounded retry tiers and then reaches DLQ instead of looping forever in the final tier.
- The real integration topology uses short test-only TTLs on the actual retry queue names and dead-letters back to main.
- It observes attempts `[0,1,2,3,4]` for both an eventual-success message and an exhausted message that reaches DLQ.

### SQL migration runner and rollout

- Added a `golang-migrate` based `migrate-sql` production CLI.
- The runner stages a deterministic source, including symmetric handling of historical duplicate migration version 61, and supports the production `70+` migration segment.
- A version-69 no-op baseline lets standard runner semantics cover fresh, 69, and 74 starting states.
- The historical GORM bootstrap runs only for an unversioned fresh database; versioned upgrades do not run AutoMigrate before SQL backfills.
- Fresh bootstrap now includes deployrun persistence and safely handles a missing `identity.members` table.
- Docker packages both migration binaries and SQL files; Compose uses the same conditional-bootstrap + SQL-runner sequence.
- Helm adds idempotent `pre-install,pre-upgrade` Secret and Job hooks. The Job uses the API image, `DATABASE_URL`, bounded retries/deadline, ordered hook weights, and fails the release before API rollout if migration fails.

### CI gates

- Tagged integration helpers fail rather than skip when `CI=true` or `REQUIRE_INTEGRATION=true` and required DSNs are absent.
- CI explicitly probes PostgreSQL and RabbitMQ, runs the packaged migration commands, executes fresh/69/74 migration tests, runs messaging/delivery/DLQ integration packages, and asserts named tests actually executed.
- Helm CI renders the chart and asserts migration Job, hook, runner, and secret output in addition to lint.

## TDD evidence

- AMQP reconnect RED: publisher returned a stale connection `Channel` error; closed consumer delivery channels never redialed.
- DLQ RED: the old publisher interface broadcast by topic and could not satisfy `PublishToQueue`.
- Migration runner RED 1: standard runner rejected duplicate migration version 61.
- Migration runner RED 2: attempting the incomplete historical `1-75` SQL chain failed at migration 23; production was corrected to preserve the established GORM baseline and run the authoritative `70-75` SQL segment.
- Retry RED: the real short-TTL chain never reached DLQ because temporary errors retried the 30-minute tier indefinitely.
- Every RED has a focused regression test that is now green.

## Real verification matrix

Environment:

- Go `1.26.0`.
- PostgreSQL `12.22`, with a random isolated database per integration test and cleanup.
- RabbitMQ `3.6.14`, with a generated isolated vhost per run and successful deletion.
- Credentials were supplied only through process environment variables.

Passed:

- Reconnect and unit scope:
  `go test ./internal/platform/messaging -run 'TestEventTransportCloseInterruptsConsumerReconnectBackoff|TestEventTransportReconnectsConsumerConnectionAndRestoresEverySubscription|TestEventTransportRedialsPublisherWhenConnectionCannotOpenChannel' -count=1`
- DLQ unit scope:
  `go test ./cmd/event-dlq -count=1`
- Real PostgreSQL:
  `TestSQLMigrationRunnerFreshReaches75AndPassesStartupGate`
  and `TestSQLMigrationRunnerUpgrades69And74To75WithStartupGate/from-69,from-74`.
- Real application factory gate:
  `TestRealMigrationRunnerControlsApplicationFactoryGate/from-69,from-74`.
- Real RabbitMQ reconnect:
  `TestRabbitEventTransportSameInstanceReconnectsPublisherAndConsumer`.
- Real retry:
  `TestEventPipelineTemporaryFailureUsesEveryRetryTier`.
- Real DLQ:
  `TestReplayTargetsOnlyRequestedSubscriberAndPreservesHeaders`
  and `TestReplayMandatoryReturnDoesNotAckDLQ`.
- Full real tagged matrix:
  `go test -tags=integration ./internal/platform/messaging/... ./internal/modules/delivery/deployrun/application/... ./cmd/event-dlq -count=1 -timeout 300s`
  passed across all listed packages.
- Build:
  `go build ./cmd/server ./cmd/event-dlq ./cmd/migrate ./cmd/migrate-sql`.
- Helm static contract:
  `go test ./deployments/helm`.
- Full backend:
  `go test ./... -count=1 -timeout 600s` reached all packages. Every Task 11
  package passed; only two protected, unrelated Kubernetes plugin tests failed:
  `TestKubernetesBundledFilesBuildRealInspectableZip` and
  `TestExecPersistsRecordingWithoutCommandOrCredentials`.

## Environment limits and residual risks

- Docker CLI/daemon is not installed, so local image build and Compose startup could not run.
- Helm is not installed, so local `helm lint/template` could not run; CI installs Helm and runs both plus explicit rendered-output assertions.
- Windows `go test -race` could not compile because the installed C compiler reports `64-bit mode not compiled in`; CI/Linux should run race coverage if required.
- RabbitMQ 3.6 validates real AMQP disconnect/reconnect, confirms, returns, TTL dead-lettering, redelivery, and consumer restoration, but cannot prove production three-node quorum availability. CI RabbitMQ 3.13 is authoritative for quorum declaration compatibility; staging must still test node loss and quorum replication.
- Delivery remains at-least-once across external-success/mark windows; stable idempotency keys and receiver reconciliation remain required.
- Live down migration remains prohibited while API writers/consumers are running.
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
