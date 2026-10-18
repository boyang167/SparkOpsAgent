# SparkOps Helm Chart

Deploy SparkOps API + Web to Kubernetes with Ingress, HPA, and secrets.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.10+
- Ingress controller (e.g. nginx-ingress)
- External PostgreSQL, Redis, and a production RabbitMQ cluster with at least
  three nodes for quorum queues
- Container images built and pushed to your registry

## Build images

```bash
# API
docker build -t your-registry/sparkops/api:1.0.0 -f backend/deployments/docker/Dockerfile backend

# Web
docker build -t your-registry/sparkops/web:1.0.0 -f fronted/deployments/docker/Dockerfile fronted
```

## Install

```bash
helm upgrade --install sparkops ./deployments/helm/sparkops \
  -f deployments/helm/sparkops/values.yaml \
  -f deployments/helm/sparkops/values-production.yaml \
  --set api.image.repository=your-registry/sparkops/api \
  --set api.image.tag=1.0.0 \
  --set web.image.repository=your-registry/sparkops/web \
  --set web.image.tag=1.0.0 \
  --set secrets.databaseDsn="$DATABASE_DSN" \
  --set-string secrets.amqpUrl="$AMQP_URL" \
  --set secrets.jwtSecret="$JWT_SECRET" \
  --set secrets.webhookSecret="$WEBHOOK_SECRET" \
  --set api.jenkins.url="$JENKINS_URL" \
  --set-string kubeconfig.content="$KUBECONFIG_B64"
```

## Verify

```bash
kubectl get pods -l app.kubernetes.io/instance=sparkops
curl https://api.sparkops.example.com/ready
curl https://app.sparkops.example.com/
```

## Components

| Resource | Description |
|----------|-------------|
| `*-api` Deployment | Go API server, probes on `/health` and `/ready` |
| `*-web` Deployment | Next.js standalone, `UPSTREAM_URL` → API Service |
| Ingress | Split app (UI/BFF) and api (webhooks/metrics) hosts |
| HPA | CPU-based autoscaling for api and web |
| Secret | DSN, JWT, webhook, tokens |

## Reliable messaging rollout

`MESSAGING_DRIVER=amqp` controls the transactional domain-event bus. It does
not replace the existing asynchronous task queue: `sparkops.events` is a topic
exchange for Outbox events, while `sparkops` remains the direct task queue
exchange. `AMQP_URL` is rendered only from `secrets.amqpUrl`; supply it through
an external secret manager or `--set-string`, never a committed values file.

Use this order for every first install and upgrade:

1. Back up PostgreSQL and verify the RabbitMQ cluster is healthy.
2. Run the documented `helm upgrade --install` command above. The chart's
   `pre-install,pre-upgrade` Job conditionally bootstraps an unversioned fresh
   database, then runs `/app/migrate-sql -path /app/migrations -min-version 70`
   from the API image with `DATABASE_URL`; Helm aborts before API rollout if
   migrations do not reach version 75.
3. The API rollout declares `sparkops.events`, all quorum main/retry/
   DLQ queues, and then starts the 15 named consumers.
4. After consumers exist, startup enables Event Outbox and Operation Outbox
   dispatchers.
5. Only after those stages succeed does the HTTP server begin listening.

The application startup gate rejects AMQP mode when migration 75 is not
current. Do not run an incompatible down migration against a live application:
stop writers/consumers first, deploy compatible code, and use a tested restore
or forward migration. Never roll back schema independently under running pods.

The local `inmemory` driver remains supported for development and unit tests,
but production validation rejects it. RabbitMQ outages leave HTTP readiness
healthy because committed Outbox rows can safely accumulate; treat the service
as degraded and alert on backlog/age. PostgreSQL failure makes `/ready` fail.
Outbox publish/confirm and completion bookkeeping use
`api.outbox.publishTimeout` (default `30s`), so shutdown cannot wait forever on
an unreachable broker.

Single-node RabbitMQ (including CI and Docker Compose) validates protocol and
recovery behavior only. It does not validate quorum availability.

SMTP workflow notifications are at-least-once, non-destructive side effects.
If an SMTP server accepts a message and the process crashes before the
Operation Outbox row is marked completed, recovery can send the same message
again. SparkOps supplies a stable `Message-ID` derived from the operation
idempotency key, but standard SMTP cannot guarantee receiver-side deduplication;
operators must allow for duplicate email delivery.

## Metrics and alerts

Monitor at least:

- `sparkops_event_outbox_pending` and
  `sparkops_event_outbox_oldest_seconds`;
- `sparkops_event_publish_total{result}` and
  `sparkops_event_dispatcher_error_total{stage}`;
- `sparkops_event_consume_total{subscriber,result}`,
  `sparkops_event_retry_total{subscriber,delay}`,
  `sparkops_event_dlq_total{subscriber}`, and
  `sparkops_event_inbox_duplicate_total{subscriber}`;
- `sparkops_operation_outbox_pending`,
  `sparkops_operation_outbox_oldest_seconds`, and
  `sparkops_operation_dispatcher_error_total{stage}`.

Set `observability.messagingAlerts.enabled=true` only when the Prometheus
Operator CRD is installed. The chart then creates alerts for stalled/high
backlogs, dispatcher errors, and any new DLQ entry. Operators should correlate
alerts with structured `event_id`, `event_type`, `subscriber`, and `attempt`
fields.

## DLQ inspection and safe replay

Each subscriber has an isolated queue named
`sparkops.event.<subscriber>.<event>.v1.dlq`. Inspect before changing data:

```bash
AMQP_URL="$AMQP_URL" go run ./cmd/event-dlq inspect \
  --queue sparkops.event.<subscriber>.<event>.v1.dlq
```

Safe replay procedure:

1. Identify and fix the permanent decode/business cause.
2. Back up or export the affected DLQ and record its count.
3. Replay a small bounded batch:

   ```bash
   AMQP_URL="$AMQP_URL" go run ./cmd/event-dlq replay \
     --queue sparkops.event.<subscriber>.<event>.v1.dlq --limit 100
   ```

4. Confirm Inbox duplicates/effects, Outbox backlog, retry rate, and DLQ count
   are converging before increasing the limit.

Replay validates one of the 10 production codecs, preserves the original
`event_id`, waits for Publisher Confirm, and only then ACKs the DLQ copy.
Stopping the command is safe; an unconfirmed copy remains in the DLQ. Do not
purge a DLQ or rewrite event IDs as part of replay.

## Lint

```bash
helm lint deployments/helm/sparkops
helm template sparkops deployments/helm/sparkops -f deployments/helm/sparkops/values-production.yaml
```
