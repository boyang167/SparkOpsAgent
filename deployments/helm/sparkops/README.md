# SparkOps Helm Chart

Deploy SparkOps API + Web to Kubernetes with Ingress, HPA, and secrets.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.10+
- Ingress controller (e.g. nginx-ingress)
- External PostgreSQL, Redis, RabbitMQ (or in-cluster equivalents)
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
  --set secrets.jwtSecret="$JWT_SECRET" \
  --set secrets.webhookSecret="$WEBHOOK_SECRET" \
  --set api.amqp.url="$AMQP_URL" \
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

## Lint

```bash
helm lint deployments/helm/sparkops
helm template sparkops deployments/helm/sparkops -f deployments/helm/sparkops/values-production.yaml
```
