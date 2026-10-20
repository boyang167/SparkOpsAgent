# SparkOps Agent

[中文](README.md)

SparkOps Agent is a multi-tenant control plane for agents, with an orchestrated delivery and Kubernetes runtime plane. It integrates business systems through REST and OpenAPI, AI clients through MCP, and event-driven workflows through webhooks.

## Capabilities

- Organizations, workspaces, and role-based access control
- Projects, plus an optional product grouping
- Agent chat and knowledge bases
- Jenkins builds, image delivery, and deployment
- Kubernetes clusters, workloads, and pod operations
- MCP for external AI clients
- OIDC and LDAP sign-in
- In-app notifications and audit logs

## Architecture

| Part | Description |
|------|-------------|
| API | Modular Go monolith, Gin, `:8080` by default |
| Web | Next.js BFF, `:3000` by default |
| Data | PostgreSQL and Redis |
| Messaging | In-memory driver for local development; RabbitMQ in production |
| External systems | Jenkins, GitLab, Harbor, Kubernetes, and SonarQube, connected per project when needed |

The browser talks to the web app. The web BFF calls the Go API.

## Requirements

- Go 1.26
- Node.js 20+
- pnpm 9.15.9
- PostgreSQL 15
- Redis 7

Docker Compose is required for the Docker path. Cluster requirements for Helm are in the [Helm guide](deployments/helm/sparkops/README.md).

## Quick start

### Docker Compose

From the repository root:

```bash
make docker-up
```

Compose starts PostgreSQL, Redis, RabbitMQ, the API, and the web app. This local stack defaults to `RUNTIME_DRIVER=mock` and `DELIVERY_DRIVER=mock` so the deployment and runtime flow can be demonstrated locally.

- UI: http://localhost:3000
- API: http://localhost:8080

Stop the stack:

```bash
make docker-down
```

### Local development

With PostgreSQL and Redis available, start the API:

```bash
cd backend
cp config.example.yaml config.yaml
go run ./cmd/migrate
go run ./cmd/seed
go run ./cmd/server
```

Fill in the database and Redis settings in `config.yaml`. When a password contains `@` or another reserved character, use the structured `database` fields in that file.

Start the web app:

```bash
cd fronted
pnpm install
cp apps/web/.env.example apps/web/.env.local
pnpm dev
```

On Windows PowerShell, use `Copy-Item` instead of `cp`.

### Demo account

`go run ./cmd/seed` creates a Super Admin:

| Email | Password |
|-------|----------|
| `demo@sparkops.local` | `Demo1234!` |

Use this account for local demonstration only.

## Layout

| Path | Description |
|------|-------------|
| [`backend/`](backend/) | Go API |
| [`fronted/`](fronted/) | Next.js web app and BFF |
| [`deployments/helm/sparkops/`](deployments/helm/sparkops/) | Helm chart |
| [`spec/`](spec/) | Architecture notes |
| [`docs/`](docs/) | Product and design documents |

## Configuration

- API: [`backend/config.example.yaml`](backend/config.example.yaml)
- Web: [`fronted/apps/web/.env.example`](fronted/apps/web/.env.example)

In production, set `APP_ENV=production`, set the messaging driver to `amqp`, and configure project-level Jenkins and Kubernetes credentials. For a local demonstration:

```bash
export DELIVERY_DRIVER=mock
export RUNTIME_DRIVER=mock
```

## Development

```bash
make test
make backend-test
make frontend-test
```

OpenAPI UI: http://localhost:8080/api/docs

## Deployment

Build the images from the repository root and replace the registry and tag:

```bash
docker build -t your-registry/sparkops/api:1.0.0 -f backend/deployments/docker/Dockerfile backend
docker build -t your-registry/sparkops/web:1.0.0 -f fronted/deployments/docker/Dockerfile fronted
```

Kubernetes install steps are in the [Helm guide](deployments/helm/sparkops/README.md).

## Documentation

- [Backend](backend/README.md)
- [Frontend](fronted/README.md)
- [Helm](deployments/helm/sparkops/README.md)
- [OpenAPI](backend/api/openapi/openapi.yaml)

## Security

Keep `config.yaml`, `.env.local`, kubeconfig files, API tokens, and external-system passwords out of Git. The sample values `sparkops` and `change-me-in-production` are for local demonstration.

## Contributing

Issues and pull requests are welcome. Run `make test` before submitting.

## License

SparkOps Agent is licensed under the [Apache License 2.0](LICENSE).
