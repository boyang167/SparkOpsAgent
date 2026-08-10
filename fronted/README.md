# SparkOps Frontend

Next.js monorepo · BFF 三层架构

## 路径

`A:\21-Agent\SparkOpsAgent\fronted`

## 文档

- [前端架构（含 BFF）](../spec/frontend-architecture.md)

## 结构

```
fronted/
├── apps/web/                 # Next.js 主应用
├── packages/
│   ├── contracts/            # BFF 对外契约 (zod + TS)
│   ├── bff-core/             # BffContext、Handler、Envelope
│   ├── bff-modules/          # 屏幕用例（login/register/session）
│   ├── upstream/             # Go API 客户端（仅服务端）
│   └── web-sdk/              # 浏览器调 /api/bff/v1
└── pnpm-workspace.yaml
```

## 本地开发

```bash
# 1. 启动 Go 后端（另开终端）
cd ../backend
go run ./cmd/server

# 2. 启动前端
cd fronted
pnpm install
copy apps/web/.env.example apps/web/.env.local
pnpm dev
```

- UI: http://localhost:3000
- BFF: http://localhost:3000/api/bff/v1/*
- Upstream: http://localhost:8080/api/v1/*（仅服务端访问）

## Phase 0 验收

- [x] 登录页 → BFF → Go，浏览器不直连 Go API
- [x] contracts 类型贯穿 web-sdk → page
- [x] httpOnly Cookie 保存 JWT

## Phase 1 验收

- [x] Workspace 列表 + 切换（Cookie 持久化）
- [x] Project 列表（Dashboard）
- [x] Project 详情 + Agent 列表
- [x] 新建 Project / Agent 表单

## Phase 2 验收

- [x] Agent Chat 页面（SSE 流式）
- [x] BFF 代理 `/runs/:runId/stream`
- [x] Agent 发布（draft → published）
- [x] 对话 conversationId URL 持久化

## Phase 3 验收

- [x] 对话历史列表（按 Agent 侧边栏）
- [x] 创建 Organization / Workspace UI
- [x] RBAC 管理 UI（角色 CRUD / 成员绑定 / 邀请 / 解绑）
- [x] BFF: governance 路由（roles、members、permissions）

## Phase 4 验收

- [x] 模型列表（Integration `/models` → BFF → 创建 Agent 下拉选择）
- [x] Agent 版本管理（新建版本、版本历史表格）
- [x] BFF: `/models`、`/agents/:id/versions`

## Phase 5 验收

- [x] Project 编辑（PATCH 名称/描述）
- [x] Project 删除（归档，列表不再显示）
- [x] BFF: `PATCH/DELETE /workspaces/:wsId/projects/:projectId`

## Phase 6 验收

- [x] Agent 编辑（PATCH 名称）
- [x] Agent 删除（归档，列表不再显示）
- [x] BFF: `PATCH/DELETE /agents/:agentId`

## Phase 7 验收

- [x] 个人设置页（修改显示名称、修改密码）
- [x] BFF: `PATCH /auth/me`、`PATCH /auth/password`

## Phase 8 验收

- [x] Workflow 列表/创建/编辑/删除（项目详情页）
- [x] 知识库列表/创建/编辑/删除 + 文档 CRUD
- [x] BFF: workflows、knowledge-bases、documents 路由
- [x] Sidebar：知识库、个人设置入口

## Phase 9 验收

- [x] Workflow 详情页 + 激活（draft → active）
- [x] 知识库关键词检索 API + 检索测试 UI
- [x] Agent 版本绑定知识库 + Chat RAG 上下文注入

## Phase 10 验收

- [x] Notification 站内信（列表 / 标已读 / 全部已读）
- [x] 订阅 `RunCompleted` 事件自动生成通知
- [x] BFF: `/notifications` 路由 + Sidebar 入口

## Phase 11 验收

- [x] Workflow Run：触发运行 + 运行历史列表
- [x] 简易执行器（log / noop steps）
- [x] 订阅 `WorkflowRunCompleted` → 站内信通知

## Phase 12 验收

- [x] Workflow Run 详情页（步骤日志时间线）
- [x] BFF: `GET /workflow-runs/:runId`
- [x] 运行记录列表跳转详情

## Phase 13 验收（PlatformSuite Phase 3 起步）

- [x] **delivery BC**：Jenkins Mock 构建触发 / 历史 / 详情
- [x] **runtime BC**：K8s Mock Pod 列表
- [x] 构建完成 → 站内信通知
- [x] 前端 `/delivery/builds`、`/runtime/pods` + Sidebar 入口

## Phase 14 验收（PlatformSuite P1 深化）

- [x] **delivery**：构建日志 SSE 流、中止构建、回滚构建
- [x] **runtime**：Pod 日志、Web 终端 Mock SSE
- [x] BFF 代理 stream / abort / rollback / pod logs / terminal
- [x] 构建详情页实时日志 + 操作按钮；Pod 行点击查看详情

## 数据库审计字段

所有业务表含 `created_at`、`updated_at`、`deleted_at`（软删除）。迁移脚本：

`backend/migrations/000008_audit_columns.up.sql`

## 默认演示账号

运行 `backend` 的 `go run ./cmd/seed` 后可使用：

- Email: `demo@sparkops.local`
- Password: `Demo1234!`
