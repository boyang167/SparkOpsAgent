# Final Full-Branch Review Fix Report

## Scope and baseline

- Baseline: `05408d7`
- Follow-up parent HEAD: `c5cb4e482a384feba9c8bf20431daed1da36c03c`
- This change is limited to final-review messaging lifecycle, retry classification,
  outbound HTTP error sanitization, SMTP semantics, configuration, tests, and
  related operational documentation.
- Existing plugin, frontend, progress, Task 5, log, and other unrelated working
  tree changes were not staged or committed.

## Fixes

### Final review P2 follow-up

- Runtime Outbox dispatch claims one record per loop instead of leasing the
  configured batch before serial publication. A successful non-empty pass
  immediately claims again; empty passes and errors wait for `PollInterval`.
  The public `DispatchOnce` batch behavior remains available for explicit use
  and tests. A deterministic short-lease/slow-publish two-owner test proves an
  unstarted later record is not leased by the first runtime worker. Existing
  owner fencing and shutdown draining remain unchanged.
- AMQP `x-sparkops-attempts` parsing rejects negative and unsupported values
  as raw poison before invoking a handler, saturates large `int64` values at
  `MaxInt32`, and saturates published retry/DLQ headers. Inbox retries also use
  saturating increment and remain on the 30-minute tier at `MaxInt32`.
- Request-construction errors now use the same URL sanitizer as `Client.Do`
  errors throughout delivery and product-workflow HTTP paths. Invalid escapes
  and control characters cannot expose full URLs or secret query parameters in
  returned, logged, or persisted errors.

### Bounded Outbox shutdown

- Added `messaging.outbox.publish_timeout` / `OUTBOX_PUBLISH_TIMEOUT`, defaulting
  to 30 seconds and validated as positive in AMQP mode.
- Every Outbox Publish, MarkPublished, and Reschedule attempt now receives its
  own bounded context. Runtime dispatch keeps request values while decoupling an
  in-flight confirm from intake cancellation.
- Shutdown cancels claim intake, lets the current confirm drain, stops before
  processing another claimed record, and cannot wait forever on RabbitMQ
  redial/confirm.
- ReliableBus and Factory continue to stop intake and wait before closing the
  transport; the transport is not used as the mechanism that breaks an
  unbounded dispatcher wait.

### Temporary retry and DLQ classification

- Only errors explicitly classified with `messaging.Permanent` enter the
  subscriber DLQ.
- Temporary attempts use 5s, 30s, 5m, 30m for attempts 0..3 and continue at
  30m thereafter without an exhaustion DLQ.
- `x-sparkops-attempts` continues increasing across retries.
- The former Task 11 “retry exhaustion produces DLQ at attempt 4” behavior and
  report wording are superseded by this final review fix. Attempt 4 replay
  coverage now starts from an explicitly permanent error.

### HTTP URL sanitization

- Added one network-error sanitizer for delivery and product-workflow HTTP
  clients.
- `*url.Error` request URL/query data is excluded from `Error()`, while timeout,
  temporary, and `errors.Is` behavior are retained through the non-URL cause.
- Delivery webhook return values, structured logs, and persisted Operation
  Outbox `last_error` are covered with secret-query tests.
- All `http.Client.Do` network errors in delivery and product-workflow use the
  sanitizer, including webhook, GitLab, Harbor, Vault, Feishu, and notification
  paths.

### SMTP protocol boundary

- SMTP notification operations are at-least-once, non-destructive external
  side effects, not exactly-once.
- Standard SMTP cannot resolve an accepted-message/process-crash-before-
  MarkCompleted window. A retry can therefore deliver a duplicate email.
- SparkOps derives a stable `Message-ID` from the Operation Outbox idempotency
  key. This can help a receiver deduplicate but is not an SMTP exactly-once
  guarantee.
- The design and Helm operations guide now state this protocol limitation and
  the possibility of duplicate receipt.

## TDD evidence

Observed RED before implementation:

- Runtime Outbox used the configured batch size and then slept after a
  successful non-empty pass.
- Attempts parser tests did not compile with an error-returning contract;
  MaxInt32 consecutive retries overflowed, and published headers wrapped.
- Invalid-escape and control-character webhook URLs exposed secret query
  values in request-construction errors.
- Outbox tests failed to compile because `PublishTimeout` did not exist.
- Factory propagation test failed because `config.OutboxConfig.PublishTimeout`
  did not exist.
- Infinite temporary retry test expected six retry publications but observed
  four followed by the old DLQ branch.
- Delivery and product-workflow secret-query tests returned complete webhook
  URLs from `url.Error`.
- HTTP sanitizer test failed because `SanitizeNetwork` did not exist.
- SMTP stable Message-ID test failed because the message builder did not exist.
- Config validation accepted a negative publish timeout.

Observed GREEN after the minimal fixes:

- Bounded shutdown, normal in-flight confirm drain, continuous publisher redial
  close, context value/deadline, retry saturation, permanent attempt-4 DLQ,
  URL sanitization, persisted `last_error`, stable Message-ID, and config
  validation tests all pass.

## Verification

Passed:

- `go test ./internal/platform/messaging/... -count=3`
- Runtime single-claim/short-lease tests with `-count=10`
- `go test ./internal/modules/delivery/... -count=3`
- `go test ./internal/modules/product_workflow/... -count=3`
- All non-plugin backend packages with `-count=1`
- `go test ./internal/platform/messaging -count=3`
- Critical shutdown/retry tests with `-count=10`
- Delivery outbound/GitLab/Harbor package tests
- `go test ./internal/modules/product_workflow/... -count=1`
- Config and HTTP sanitizer tests
- Task 11 tagged messaging, DLQ, and delivery integration package matrices
- `go test ./deployments/helm -count=1`
- `go build ./...`
- `git diff --check` for the final-fix candidate files

Environment-limited:

- The named real RabbitMQ/PostgreSQL TTL, reconnect, and permanent attempt-4
  replay tests compile under `-tags=integration` but skip locally because
  `AMQP_URL` and `DATABASE_DSN` are not set.
- Helm CLI lint was not run because `helm` is not installed; the Go Helm static
  contract tests pass.
- The literal `go test ./... -count=1` is blocked only by unrelated Kubernetes
  plugin working-tree changes that this fix was forbidden to touch:
  `TestKubernetesBundledFilesBuildRealInspectableZip` reports a missing bundled
  `diagnosis/agent.go`, and `TestExecPersistsRecordingWithoutCommandOrCredentials`
  reports an existing stream-recording assertion mismatch. The same regression
  excluding `internal/plugins/*` passes.
