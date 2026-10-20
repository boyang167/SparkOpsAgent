# 开源 README 设计

## 目标

为准备开源的 SparkOps Agent 提供仓库首页文档：中文 `README.md`、英文 `README_EN.md`，以及 Apache-2.0 `LICENSE`。首页让第一次打开仓库的人知道这是什么、依赖什么、如何在本地或 Docker 中跑起来。

## 文件

| 路径 | 动作 |
|------|------|
| `README.md` | 新建中文首页，页首链接 `README_EN.md` |
| `README_EN.md` | 新建英文首页，页首链接 `README.md` |
| `LICENSE` | 新建 Apache License 2.0 全文 |
| `READEMD.md` | 删除。其中含真实外部系统口令与 Token，不得迁入新 README |

两份 README 章节顺序、事实和命令一致，只是语言不同。目录名保持仓库现状 `fronted/`。

版权行使用 `Copyright 2026 SparkOps contributors`。

不修改 `backend/README.md`、`fronted/README.md`、Helm 文档、历史 spec/plan，也不改测试夹具里的 `../READEMD.md` 字符串。

## 首页大纲

两份文件都按以下顺序书写。

1. 标题与一句话定位：多租户 Agent 控制面，带可编排的交付与 Kubernetes 运行面；对外提供 REST、MCP 和 Webhook。
2. 语言切换链接。
3. 能力概览：组织与工作空间、RBAC、项目与可选产品组合、Agent 对话、知识库、Jenkins 构建与部署、Kubernetes 运行、MCP、OIDC/LDAP、通知与审计。
4. 架构：Go 模块化单体 API（`:8080`）与 Next.js BFF（`:3000`）。浏览器访问 BFF，BFF 访问 Go API。数据依赖 PostgreSQL 与 Redis。消息可用内存驱动，生产使用 RabbitMQ。Jenkins、GitLab、Harbor、Kubernetes、SonarQube 为可选外部系统。
5. 环境要求：Go 1.26、Node.js 20+、pnpm 9.15.9、PostgreSQL 15、Redis 7。Docker 路径另需 Docker Compose。Helm 路径要求见 Helm 文档。
6. 快速开始，两条路径：
   - Docker：在仓库根目录执行 `make docker-up`。Compose 会启动 PostgreSQL、Redis、RabbitMQ、API 和 Web。该 Compose 默认 `RUNTIME_DRIVER=mock`、`DELIVERY_DRIVER=mock`。界面为 `http://localhost:3000`，API 为 `http://localhost:8080`。
   - 本地：复制 `backend/config.example.yaml` 为 `backend/config.yaml` 并填写数据库与 Redis；执行 `go run ./cmd/migrate`、`go run ./cmd/seed`、`go run ./cmd/server`。前端在 `fronted/` 执行 `pnpm install`，复制 `apps/web/.env.example` 为 `apps/web/.env.local`，再执行 `pnpm dev`。
7. 本地演示账号仅写 seed 账号：`demo@sparkops.local` / `Demo1234!`。说明该账号由 `go run ./cmd/seed` 创建，并具有 Super Admin。不写其他角色口令，不写 `grant-super-admin` 的默认密码。
8. 目录表：`backend/`、`fronted/`、`deployments/helm/sparkops/`、`spec/`、`docs/`。
9. 配置入口：`backend/config.example.yaml`、`fronted/apps/web/.env.example`。说明密码含特殊字符时使用结构化 database 配置。生产环境设置 `APP_ENV=production`，消息驱动使用 `amqp`，并配置项目级 Jenkins 与 Kubernetes 凭据。本地可显式设置 `DELIVERY_DRIVER=mock` 与 `RUNTIME_DRIVER=mock`。
10. 开发与测试：`make test`、`make backend-test`、`make frontend-test`。Swagger UI 为 `http://localhost:8080/api/docs`。
11. 部署：在仓库根目录构建镜像，标签由读者替换：

```bash
docker build -t your-registry/sparkops/api:1.0.0 -f backend/deployments/docker/Dockerfile backend
docker build -t your-registry/sparkops/web:1.0.0 -f fronted/deployments/docker/Dockerfile fronted
```

Helm 安装只链接 `deployments/helm/sparkops/README.md`，不复制整段 Helm 参数。
12. 文档链接：`backend/README.md`、`fronted/README.md`、`deployments/helm/sparkops/README.md`、`backend/api/openapi/openapi.yaml`。
13. 安全：不要提交 `config.yaml`、`.env.local`、kubeconfig、API Token 或外部系统口令。示例配置中的 `sparkops` / `change-me-in-production` 只用于本地演示。
14. 贡献：欢迎 Issue 与 Pull Request；提交前运行 `make test`。不另建 `CONTRIBUTING.md`。
15. 许可证：Apache License 2.0，链接 `LICENSE`。

## 明确排除

- 真实 GitLab、Jenkins、Harbor、RabbitMQ、集群地址、个人访问令牌。
- 本机绝对路径、旧系统目录、阶段验收清单、完整 API 表、权限阶段表。
- Git 历史改写。删除当前文件不能消除已推送的历史凭据；那是独立的密钥轮换与历史清理工作。

## 验收

- 仓库根目录同时存在 `README.md`、`README_EN.md`、`LICENSE`。
- `READEMD.md` 不存在。
- 两份 README 的命令、端口、版本和演示账号一致。
- 全文检索两份 README 与 `LICENSE`，不包含已从 `READEMD.md` 识别出的真实口令、Token 或内网地址。
- `LICENSE` 为 Apache-2.0 正文，版权行为 `Copyright 2026 SparkOps contributors`。
