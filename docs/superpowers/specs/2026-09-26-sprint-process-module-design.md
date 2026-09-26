# sprint_process 独立模块设计

## 目标

将「迭代流程定义」从 `product` 模块拆分为独立模块 `sprint_process`，对标 `product_workflow` 的 template/instance 模式。

## 实施阶段（已完成）

| 阶段 | 内容 | 状态 |
|------|------|------|
| P1 | domain + persistence 拆出 | ✅ |
| P2 | commands + HTTP + 路由注册 | ✅ |
| P3 | product 瘦身 + `SprintProcessPort` | ✅ |
| P4 | schema 迁移至 `sprint_process.*` | ✅ |
| P5 | 前端独立 Tab + 工作区模板中心 | ✅ |

## 边界

**sprint_process 负责：**
- WorkspaceSprintProcessTemplate CRUD（含 UPDATE）
- SprintProcess（产品级流程模板）CRUD + seed
- 从工作空间模板导入到产品

**product 保留：**
- Sprint / WorkItem 执行态
- 看板、stage gate、PM 集成
- `PlanSprintStageUpdate`（依赖 WorkItem）

## 模块结构

```
backend/internal/modules/sprint_process/
├── module.go
├── domain/
│   ├── model/          SprintProcess, WorkspaceSprintProcessTemplate, SprintStage
│   ├── repository/     ProcessRepository, WorkspaceTemplateRepository
│   ├── port/           ProductChecker, SprintUsageChecker
│   └── errors.go
├── application/command/
├── infrastructure/
│   ├── persistence/    process_repo.go, migrate.go
│   └── acl/            product_checker, sprint_usage
└── interface/http/     handler.go
```

## 跨模块接口

```go
type SprintProcessPort interface {
    GetOrSeedDefault(ctx, productID, actor) (*SprintProcess, error)
    FindByID(ctx, productID, processID) (*SprintProcess, error)
}
```

注册顺序：`product_workflow` → `sprint_process` → `product`

product 通过 `domain/model/process_alias.go` 类型别名过渡，减少大面积 import 改动。

## API

保持现有 REST 路径不变，路由注册在 `sprint_process/module.go`：

| 路径 | 说明 |
|------|------|
| `GET/PUT /api/v1/products/:id/sprint-process` | 默认流程 get/upsert |
| `GET/POST /api/v1/products/:id/sprint-processes` | 产品流程列表/创建 |
| `PUT/DELETE /api/v1/products/:id/sprint-processes/:processId` | 更新/删除 |
| `GET/POST /api/v1/workspaces/:wsId/sprint-process-templates` | 工作区模板列表/创建 |
| `PUT/DELETE /api/v1/workspaces/:wsId/sprint-process-templates/:templateId` | 工作区模板更新/删除 |
| `POST .../sprint-processes/import-workspace` | 从工作区模板导入 |

## 数据库

表位于独立 schema：

- `sprint_process.sprint_processes`
- `sprint_process.workspace_sprint_process_templates`

迁移方式：
1. **GORM AutoMigrate**（`sprint_process/infrastructure/persistence/migrate.go`）：创建 schema，若旧表仍在 `product.*` 则 `ALTER TABLE ... SET SCHEMA sprint_process`
2. **SQL 参考文件**：`backend/migrations/000058_sprint_process_schema.up.sql`（供手动迁移）

`bootstrap/migrate.go` 与模块 `Register` 均会调用 `sprint_process` AutoMigrate。

## 前端入口

| 场景 | 路径 |
|------|------|
| 产品流程模板 | 产品详情 → Tab「流程模板」（`?tab=process`） |
| 工作区共享模板 | `/templates?tab=sprint-process` |
