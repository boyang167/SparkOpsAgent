# Task 11 Report: Release Blocker Closure

## Scope

- Initial release-blocker closure was committed as `cebbc29269c2123474ddf2eab23d3e44f5d77e69`.
- This follow-up closes the topology-normalization, Helm PodSpec, and DLQ-attempt review findings from that HEAD.
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
- Queue parsing is delegated to the exported narrow messaging validator `ValidateEventDeadLetterQueue`; it uses `topologyFor` and `normalizeQueueNamePart` rather than duplicating queue rules in the CLI.
- Queue subscriber/event/version identity, exact `.dlq` suffix, envelope structure, and production codec registration are validated before publish.
- Unit coverage includes valid production codecs whose event names contain underscores: `agent_runtime`, `workspace_app`, and `product_workflow`, plus mismatched event/version rejection.
- A DLQ delivery is ACKed only after publisher confirm and no mandatory return.
- Real two-subscriber coverage proves no cross-delivery; an unroutable target proves return handling leaves the DLQ message available.

### 3. Migration-before-rollout

- Added the packaged `migrate-sql` CLI using the actual `golang-migrate` runner and repository SQL migrations.
- Historical duplicate migration versions are normalized deterministically; the production runner uses the established GORM bootstrap only for an unversioned fresh database and the authoritative SQL 70-75 segment thereafter.
- Docker packages `migrate-schema`, `migrate-sql`, and migration files; Compose gates API startup on migration success.
- Helm supplies `pre-install,pre-upgrade` Secret and Job hooks, `DATABASE_URL`, ordered weights, finite retries/deadline, and the actual packaged runners. Hook failure blocks rollout.
- The migration Job uses the chart service account and inherits `global.imagePullSecrets`, matching the API PodSpec for private registries.
- CI runs `helm lint` and `helm template` and asserts the rendered hook, runner, and secret contract.

### 4. Non-skippable CI integration gates

- Integration helpers Fatal rather than Skip when either `CI=true` or `REQUIRE_INTEGRATION=true` and DSN/AMQP is absent or an isolated PostgreSQL database cannot be created.
- CI probes PostgreSQL, AMQP, and RabbitMQ readiness, then asserts named migration, reconnect, retry, DLQ, and factory-gate tests executed.
- Negative checks were run locally and produced the expected test failures for missing `DATABASE_DSN` and missing `AMQP_URL`.

### 5. Real retry chain

- Temporary failures traverse the actual four retry queues by TTL dead-lettering back to the subscriber main queue.
- Attempt headers increment through `[0,1,2,3,4]`.
- Both permanent and exhausted structured DLQ publications carry the current `x-sparkops-attempts` value.
- Real RabbitMQ coverage proves exhaustion produces DLQ attempts `4`, CLI replay preserves `4` on the exact main queue, and the test does not synthesize that header.

### 6. Actual migration runner and startup gate

- Real PostgreSQL tests run the production runner for fresh bootstrap, 69 to 75, and 74 to 75.
- The application factory rejects versions 69 and 74 before migration and starts after the same runner reaches 75.
- No test manually updates a migration version to claim success.

## Fresh verification matrix

Environment:

- Go `1.26.0`.
- PostgreSQL `12.22`, isolated database per test with cleanup.
- Reachable RabbitMQ is `3.6.14`, isolated vhost per run with cleanup.
- Helm and Docker CLIs are not installed locally; no local Helm version or container-render result is claimed.

Passed:

- Core unit/static scope:
  `go test ./cmd/event-dlq ./internal/platform/messaging/... ./internal/modules ./internal/modules/notification/application/subscriber ./deployments/helm -count=1 -timeout 300s`
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
  - `TestRetryExhaustionDLQAttemptsSurviveCLIReplay`
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
- Docker and Helm CLIs are unavailable locally. Image build, Compose startup, `helm lint`, and rendered chart checks are therefore CI-owned. Go static Helm contract tests passed locally, and CI renders with a private `global.imagePullSecrets` value and asserts the Job output.
- Windows race builds are not claimed; CI/Linux should run race coverage if required.
- Delivery remains at-least-once across external-success/mark windows, so stable idempotency keys and receiver reconciliation remain required.
