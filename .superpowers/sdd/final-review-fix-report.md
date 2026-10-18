# Final Full-Branch Review Fix Report

## Scope and baseline

- Baseline: `05408d7`
- Reviewed/fixed HEAD: `72b4107fe2ade8fb325424739270af17fb7fda6a`
- This change is limited to final-review messaging lifecycle, retry classification,
  outbound HTTP error sanitization, SMTP semantics, configuration, tests, and
  related operational documentation.
- Existing plugin, frontend, progress, Task 5, log, and other unrelated working
  tree changes were not staged or committed.

## Fixes

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
