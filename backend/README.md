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

```bash
go run ./cmd/grant-super-admin \
  -email superadmin@sparkops.local \
  -password 'SuperAdmin1234!' \
  -display-name "Platform Super Admin" \
  -register
```

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
