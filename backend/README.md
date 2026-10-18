# SparkOps Backend

Go · DDD · 模块化单体 · **Gin**

## 路径

`A:\21-Agent\SparkOpsAgent\backend`

## 文档

- [Greenfield Go DDD 架构](../spec/greenfield-go-ddd.md)
- [完整重写实施计划](../spec/backend-implementation-plan.md)
- [OpenAPI 规范](./api/openapi/openapi.yaml)

## 本地开发

```bash
cd backend

# 无 config.yaml 时仅启动 health（不连数据库）
go run ./cmd/server

# 完整配置
copy config.example.yaml config.yaml
go run ./cmd/server
```

默认 API：`http://localhost:8080`

**Swagger UI**：`http://localhost:8080/api/docs`（OpenAPI 规范见 `./api/openapi/openapi.yaml`）

### Docker 一键启动

```bash
cd backend
make docker-up
make docker-logs
make docker-down
```

### 数据库建表

表不在 `public` schema，而是按 BC 分散在独立 schema 中（如 `identity.members`、`tenant.workspaces`）。在 Navicat / DBeaver 中请展开 **Schemas**，并确认连接的是 `config.yaml` 里的库名（默认 `sparkops_agent`）。

```bash
# 仅建 schema + 表（不写入演示数据）
go run ./cmd/migrate
# 或
make migrate

# 建表 + 写入演示数据
go run ./cmd/seed
```

启动 `go run ./cmd/server` 且成功连库时，各模块 Register 也会自动 AutoMigrate。

默认账号：`demo@sparkops.local` / `Demo1234!`（seed 会自动授予 **Super Admin**）

## 权限模型（Phase P0）

| 层级 | 机制 | 说明 |
|------|------|------|
| Platform Super Admin | `identity.system_role_bindings` | 访问 `/api/v1/admin/*`（集群、工具箱、公告等） |
| Org Admin | `tenant.organization_members`（owner/admin） | 在组织下创建工作区 |
| Workspace RBAC | `governance.roles` + 成员绑定 | 工作区内资源权限；bootstrap 不再含 `platform:admin` |
| Workspace 成员 | 所有 `X-Workspace-Id` API | 须为工作区成员（`tenant.workspace_members`） |

`GET /api/v1/auth/me` 返回 `is_super_admin` 字段，前端据此控制平台运维导航项。

授予 Super Admin（示例）：

```sql
INSERT INTO identity.system_role_bindings (member_id, role)
VALUES ('<member-uuid>', 'super_admin')
ON CONFLICT (member_id) DO UPDATE SET role = EXCLUDED.role;
```

或重新执行 `go run ./cmd/seed`（demo 用户会自动获得 super_admin）。

新建 Super Admin 账号（推荐）：

**PowerShell（单行，推荐）：**

```powershell
go run ./cmd/grant-super-admin -email superadmin@sparkops.local -password "SuperAdmin1234!" -display-name "Platform Super Admin" -register
```

**Bash / Linux：**

```bash
go run ./cmd/grant-super-admin \
  -email superadmin@sparkops.local \
  -password 'SuperAdmin1234!' \
  -display-name "Platform Super Admin" \
  -register
```

> PowerShell 不支持 `\` 换行；多行请用反引号 `` ` `` 结尾，或直接用上面单行命令。

已有用户仅授予角色：

```bash
go run ./cmd/grant-super-admin -email existing@example.com
```

## 权限模型（Phase P1 — Org RBAC）

固定组织角色（`tenant.organization_members.role`）：

| 角色 | 权限 |
|------|------|
| `owner` | `org:read`, `org:member:manage`, `org:workspace:create` |
| `admin` | 同 owner（不可移除 owner） |
| `member` | `org:read` |

Org 成员管理 API 需 `org:member:manage`；创建工作区需 `org:workspace:create`。

## 权限模型（Phase P2 — 审计与平台治理）

| 能力 | 路径 | 权限 |
|------|------|------|
| 审计日志（平台） | `GET /api/v1/platform/audit-logs` | Super Admin |
| 审计日志（组织） | `GET /api/v1/orgs/:orgId/audit-logs` | `org:member:manage` |
| 平台组织列表/创建 | `GET/POST /api/v1/platform/orgs` | Super Admin |
| 平台用户列表/创建 | `GET/POST /api/v1/platform/members` | Super Admin |
| 平台用户更新 | `PATCH /api/v1/platform/members/:memberId` | Super Admin |
| 平台用户重置密码 | `PATCH /api/v1/platform/members/:memberId/password` | Super Admin |
| 组织启停 | `PATCH /api/v1/platform/orgs/:orgId/status` | Super Admin |
| Owner 转让（平台） | `POST /api/v1/platform/orgs/:orgId/owners/transfer` | Super Admin |
| Owner 转让（组织） | `POST /api/v1/orgs/:orgId/owners/transfer` | Org Owner |

审计表：`platform.audit_logs`，记录组织创建、成员变更、Owner 转让、平台开户等操作。

## MCP 集成（外部 AI 客户端）

| 能力 | 路径 | 权限 |
|------|------|------|
| 工具列表 | `GET /api/v1/mcp/tools` | 工作区成员 + 各 Tool 自身权限过滤 |
| JSON-RPC | `POST /api/v1/mcp/rpc` | 工作区成员 + 各 Tool 自身权限过滤 |

需在平台配置中启用 **MCP 集成** 模块（`entitlements.mcp`）。鉴权使用工作区 **API Token**（Bearer）+ `X-Workspace-Id` 请求头。

内置 Tools（Registry 单一注册源，列表按 entitlement / RBAC 过滤）：

| Tool | 层级 | 权限 | 模块 |
|------|------|------|------|
| `list_projects` | discovery | `project:read` | — |
| `get_project` | discovery | `project:read` | — |
| `list_knowledge_bases` | discovery | `project:read` | knowledge |
| `list_agents` | discovery | `agent:read` | agent |
| `search_knowledge` | read | `project:read` | knowledge |
| `get_chat_run` | read | `chat:use` | agent |
| `chat_with_agent` | action | `chat:use` | agent |
| `list_products` | discovery | `project:read` | delivery |
| `list_product_workflows` | discovery | `project:read` | delivery |
| `list_sprints` | discovery | `project:read` | delivery |
| `trigger_workflow` | action | `project:write` | delivery |
| `create_work_item` | action | `project:write` | delivery |

插件 Tools（工作区启用对应插件后可见，如 `plugin:kubernetes`）：

| Tool | 层级 | 权限 | 插件 |
|------|------|------|------|
| `list_kubernetes_clusters` | discovery | `project:read` | kubernetes |

插件通过实现 `platform/plugin.MCPPlugin` 在启动时向 Registry 注册 handler；`plugin.yaml` 的 `mcp_tools` 段用于 manifest 声明与校验。

`chat_with_agent` 默认立即返回 `run_id`；可选 `wait_seconds`（≤30）同步等待。轮询请用 `get_chat_run`。

本地 stdio 桥接（供 Cursor / Claude Desktop）：

```bash
cd backend
SPARKOPS_API_URL=http://localhost:8080 \
SPARKOPS_TOKEN=<api-token> \
SPARKOPS_WORKSPACE_ID=<workspace-uuid> \
go run ./cmd/mcp-server
```

前端配置页：`/config/mcp`（需 MCP 模块 entitlement）。

## MVP API 概览

| BC | Method | Path |
|----|--------|------|
| health | GET | `/health` |
| identity | POST | `/api/v1/auth/register`, `/login` |
| identity | GET | `/api/v1/auth/me` |
| tenant | POST | `/api/v1/orgs`, `/api/v1/orgs/:orgId/workspaces` |
| tenant | GET | `/api/v1/orgs`, `/api/v1/orgs/:orgId`, `/api/v1/workspaces` |
| tenant | GET/POST/PATCH/DELETE | `/api/v1/orgs/:orgId/members`（需 `org:member:manage`） |
| tenant | GET | `/api/v1/orgs/:orgId/me/permissions` |
| governance | GET/POST | `/api/v1/workspaces/:wsId/roles` |
| governance | PATCH/DELETE | `/api/v1/workspaces/:wsId/roles/:roleId` |
| governance | GET/POST | `/api/v1/workspaces/:wsId/members`, `/members/:memberId/roles` |
| governance | GET | `/api/v1/workspaces/:wsId/me/permissions` |
| workspace_app | GET/POST | `/api/v1/workspaces/:wsId/projects` |
| workspace_app | GET/PATCH/DELETE | `/api/v1/projects/:projectId` |
| agent | GET/POST | `/api/v1/projects/:projectId/agents` |
| agent | GET/PATCH/DELETE | `/api/v1/agents/:agentId` |
| integration | GET | `/api/v1/models` |
| agent_runtime | POST | `/api/v1/agents/:agentId/chat` |
| agent_runtime | GET | `/api/v1/runs/:runId/stream` (SSE) |
| delivery | GET/POST | `/api/v1/projects/:projectId/builds` |
| delivery | GET | `/api/v1/builds/:buildId` |
| runtime | GET | `/api/v1/projects/:projectId/pods` |
| runtime | GET | `/api/v1/projects/:projectId/pods/:podId` |
| platform_admin | GET/PATCH | `/api/v1/admin/*`（需 Super Admin） |
| platform | GET/POST | `/api/v1/platform/orgs`（Super Admin） |
| platform | GET/POST/PATCH | `/api/v1/platform/members`（Super Admin） |
| platform | PATCH | `/api/v1/platform/members/:memberId/password`（Super Admin） |
| platform | GET | `/api/v1/platform/audit-logs`（Super Admin） |
| tenant | POST | `/api/v1/orgs/:orgId/owners/transfer`（Org Owner） |
| tenant | GET | `/api/v1/orgs/:orgId/audit-logs` |

受保护接口需 `Authorization: Bearer <token>` 与 `X-Workspace-Id`。

## Product v2（发布计划 + 产品工作流）

Product v2 在 `product` schema 下引入 **ReleasePlan**（发布计划）、**Environment**（环境）与 **ProductWorkflow**（产品工作流），替代旧版 `ProductRequirement` 发布流程。

设计文档：[product_desgin.md](../docs/product_desgin.md) · 实施计划：[product_implementation_plan.md](../docs/product_implementation_plan.md)

### 数据库迁移（000045–000052）

| 版本 | 说明 |
|------|------|
| `000045` | Product v2 基础表（`product.products`、项目 `product_id` 等） |
| `000046` | 工作流模板 `product.workflow_templates` + 内置模板 seed |
| `000047` | 发布计划 `product.release_plans` / jobs / audit |
| `000048` | 产品工作流 `product.product_workflows` / runs |
| `000050` | 环境 `product.environments` |
| `000051` | 工作流 deploy 引用 `product.workflow_deploy_refs` |
| `000052` | **幂等**将 `product_requirements` 历史数据迁移至 `release_plans` |

```bash
cd backend
go run ./cmd/migrate    # 含 000052
# 或启动 server（连库后各模块 AutoMigrate）
go run ./cmd/server
```

> 000052 可重复执行：已迁移的 requirement 不会重复插入。回滚见 `000052_migrate_requirements_to_release_plans.down.sql`（仅删除 `migrated_from_requirement` 审计记录，不删 release_plan 本体）。

### 核心 API

| 域 | Method | Path |
|----|--------|------|
| 产品 | GET/POST | `/api/v1/workspaces/:wsId/products` |
| 产品 | GET/PATCH | `/api/v1/products/:productId` |
| 产品 | GET | `/api/v1/products/:productId/overview`、`/projects` |
| 发布计划 | GET/POST | `/api/v1/products/:productId/release-plans` |
| 发布计划 | GET | `/api/v1/products/:productId/release-plans/:planId` |
| 发布计划 | POST | `.../finish-planning`、`.../approve`、`.../reject`、`.../execute` |
| 发布计划 Job | POST | `.../jobs`、`.../jobs/:jobId/done`、`.../skip`、`.../retry` |
| 环境 | GET/POST | `/api/v1/products/:productId/environments` |
| 环境 | PATCH/DELETE/GET | `.../environments/:envId`、`.../status` |
| 工作流模板 | GET/POST | `/api/v1/workspaces/:wsId/workflow-templates` |
| 工作流模板 | GET/PUT/POST | `.../:templateId`、`.../publish`、`.../preview` |
| 产品工作流 | GET/POST | `/api/v1/products/:productId/workflows`、`.../from-template` |
| 产品工作流 Run | POST/GET | `.../workflows/:workflowId/runs`、`.../runs/:runId` |
| 工作流审批 | POST | `.../runs/:runId/jobs/:jobName/approve|reject` |
| 全局工作流 | GET | `/api/v1/workspaces/:wsId/workflows` |

内置系统模板 key：`std-build-deploy-uat`、`std-release-prod`（启动时 `SyncBuiltinTemplates` 写入）。

### Requirement API 废弃

旧 `ProductRequirement` 接口仍可用（兼容层：创建 Requirement 会同步创建 ReleasePlan），但响应带 RFC 废弃头：

| Header | 值 |
|--------|-----|
| `Deprecation` | `true` |
| `Sunset` | `2026-12-31` |
| `Link` | `</api/v1/products/{productId}/release-plans>; rel="successor-version"` |
| `X-SparkOps-Deprecated-Reason` | 说明迁移至 ReleasePlan |

新功能请直接使用 **ReleasePlan** API。

### 验收路径（手动）

1. `go run ./cmd/migrate` → 确认 `product.release_plans` 存在
2. 前端 `/publish/workflows`：查看工作区全局工作流，从模板一键执行
3. 产品详情 → **发布计划**：创建 text Job → 完成规划 → 执行 → 标记完成
4. 产品详情 → **工作流**：从 `std-build-deploy-uat` 创建并 Run（需 UAT 环境）
5. 集成测试：`go test ./internal/modules/product/application/... -run ProductV2`

## 测试与构建

```bash
go test ./...
go build -o bin/server ./cmd/server
go run ./scripts/check-deps.go
```

## Phase 状态

- [x] Phase 0~4 MVP 后端完成
- [x] Phase P0 企业权限（Super Admin / Org / Workspace 成员校验）
- [x] Phase P1 Org RBAC（成员邀请/角色/移除 API + 前端成员管理）
- [x] Phase P2 审计日志、Owner 转让、Super Admin 平台组织治理
- [x] Phase P3 企业特性（SSO/LDAP、Org 配额、Casbin/OPA 策略引擎、项目级 ACL）

### Phase P3 能力

| 能力 | API / 说明 |
|------|------------|
| Org 配额 | `GET/PATCH /api/v1/orgs/:orgId/quota`，`GET .../quota/usage` |
| 配额 enforcement | 创建工作区 / Agent / 项目时校验 Org 与工作区限额 |
| 项目成员 ACL | `GET/POST/PATCH/DELETE /api/v1/projects/:projectId/members` |
| Casbin 策略 | 默认 `policy.driver=casbin`（**Casbin v3** + gorm-adapter v3.41），表 `governance.casbin_rule` |
| OPA 策略 | 可选 `policy.driver=opa` + `policy.opa.url` |
| SSO / OIDC | `GET /api/v1/auth/oidc/login`，`GET /api/v1/auth/oidc/callback` |
| LDAP | `POST /api/v1/auth/ldap/login` |
| 外部身份 | 表 `identity.external_identities` |

`config.yaml` 示例：

```yaml
policy:
  driver: casbin   # 或 opa
  opa:
    url: http://localhost:8181
    data_path: sparkops/authz/allow

auth:
  oidc:
    enabled: false
    issuer: https://keycloak.example/realms/sparkops
    client_id: sparkops-web
    client_secret: secret
    redirect_url: http://localhost:3000/auth/callback
  ldap:
    enabled: false
    url: ldap://ldap.example:389
    bind_dn: cn=admin,dc=example,dc=com
    bind_password: secret
    user_search_base: ou=users,dc=example,dc=com
    user_search_filter: "(uid=%s)"
```

工作流补充能力（需在 `config.yaml` 或环境变量中配置后才真正发出）：

| 能力 | 配置 |
|------|------|
| 对外 Hook 地址 | `server.public_url` 或 `SPARKOPS_PUBLIC_URL`。保存 Git 触发器时用项目仓库密钥注册 GitLab Project Hook |
| 加密变量 | 使用 `security.secret_encryption_key`（否则回退 `jwt.secret`）。界面留空表示保持原密文 |
| Vault | `vault.address` / `vault.token`，或 `VAULT_ADDR` / `VAULT_TOKEN`。变量的 Vault 路径在启动时读取 KV v2 的 `value` |
| 邮件 | `notify.smtp` 或 `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` |
| 飞书成员 | `notify.feishu.app_id` / `app_secret`，或 `FEISHU_APP_ID` / `FEISHU_APP_SECRET`。目标填 open_id，含 `@` 时按邮箱发送 |

超时会把运行标为失败；调试断点让每个阶段在继续前暂停。GitLab 的提交状态和 MR 评论使用同一套项目密钥回写。ConfigMap / Secret YAML 支持 `{{.变量名}}`。