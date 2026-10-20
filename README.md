# SparkOps Agent

[English](README_EN.md)

SparkOps Agent 是多租户 Agent 控制面，带可编排的交付与 Kubernetes 运行面。系统通过 REST 与 OpenAPI 对接业务系统，通过 MCP 对接 AI 客户端，通过 Webhook 对接事件驱动流程。

## 能力

- 组织、工作空间与基于角色的访问控制
- 项目，以及可选的产品组合
- Agent 对话与知识库
- Jenkins 构建、镜像交付与部署
- Kubernetes 集群、工作负载与 Pod 运维
- 面向外部 AI 客户端的 MCP
- OIDC 与 LDAP 登录
- 站内通知与审计日志

## 架构

| 部分 | 说明 |
|------|------|
| API | Go 模块化单体，Gin，默认 `:8080` |
| Web | Next.js BFF，默认 `:3000` |
| 数据 | PostgreSQL、Redis |
| 消息 | 本地可使用内存驱动；生产使用 RabbitMQ |
| 外部系统 | Jenkins、GitLab、Harbor、Kubernetes、SonarQube，按项目按需接入 |

浏览器访问 Web。Web 的 BFF 再访问 Go API。

## 环境要求

- Go 1.26
- Node.js 20+
- pnpm 9.15.9
- PostgreSQL 15
- Redis 7

使用 Docker 启动时需要 Docker Compose。使用 Helm 部署时的集群要求见 [Helm 文档](deployments/helm/sparkops/README.md)。

## 快速开始

### Docker Compose

在仓库根目录执行：

```bash
make docker-up
```

Compose 会启动 PostgreSQL、Redis、RabbitMQ、API 和 Web。这套本地编排默认 `RUNTIME_DRIVER=mock`、`DELIVERY_DRIVER=mock`，用于演示部署与运行流程。

- 界面：http://localhost:3000
- API：http://localhost:8080

停止：

```bash
make docker-down
```

### 本地开发

准备 PostgreSQL 与 Redis 后，启动后端：

```bash
cd backend
cp config.example.yaml config.yaml
go run ./cmd/migrate
go run ./cmd/seed
go run ./cmd/server
```

在 `config.yaml` 中填写数据库与 Redis。密码包含 `@` 等特殊字符时，使用配置文件里的结构化 `database` 字段。

启动前端：

```bash
cd fronted
pnpm install
cp apps/web/.env.example apps/web/.env.local
pnpm dev
```

Windows PowerShell 可将 `cp` 换成 `Copy-Item`。

### 演示账号

`go run ./cmd/seed` 会创建 Super Admin：

| 邮箱 | 密码 |
|------|------|
| `demo@sparkops.local` | `Demo1234!` |

该账号只用于本地演示。

## 目录

| 路径 | 说明 |
|------|------|
| [`backend/`](backend/) | Go API |
| [`fronted/`](fronted/) | Next.js Web 与 BFF |
| [`deployments/helm/sparkops/`](deployments/helm/sparkops/) | Helm Chart |
| [`spec/`](spec/) | 架构说明 |
| [`docs/`](docs/) | 产品与设计文档 |

## 配置

- 后端：[`backend/config.example.yaml`](backend/config.example.yaml)
- 前端：[`fronted/apps/web/.env.example`](fronted/apps/web/.env.example)

生产环境设置 `APP_ENV=production`，将消息驱动设为 `amqp`，并在项目上配置 Jenkins 与 Kubernetes 凭据。本地演示可以设置：

```bash
export DELIVERY_DRIVER=mock
export RUNTIME_DRIVER=mock
```

## 开发与测试

```bash
make test
make backend-test
make frontend-test
```

后端 OpenAPI 界面：http://localhost:8080/api/docs

## 部署

在仓库根目录构建镜像，替换为自己的仓库与标签：

```bash
docker build -t your-registry/sparkops/api:1.0.0 -f backend/deployments/docker/Dockerfile backend
docker build -t your-registry/sparkops/web:1.0.0 -f fronted/deployments/docker/Dockerfile fronted
```

Kubernetes 安装步骤见 [Helm 文档](deployments/helm/sparkops/README.md)。

## 文档

- [后端](backend/README.md)
- [前端](fronted/README.md)
- [Helm](deployments/helm/sparkops/README.md)
- [OpenAPI](backend/api/openapi/openapi.yaml)

## 安全

不要提交 `config.yaml`、`.env.local`、kubeconfig、API Token 或外部系统口令。示例配置中的 `sparkops` 和 `change-me-in-production` 只用于本地演示。

## 贡献

欢迎提交 Issue 与 Pull Request。提交前请运行 `make test`。

## 许可证

本项目采用 [Apache License 2.0](LICENSE)。
