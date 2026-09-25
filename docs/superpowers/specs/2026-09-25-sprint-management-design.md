# 迭代管理（Sprint Management）详细 Spec

> Date: 2026-09-25  
> Status: **Draft — 待评审**  
> Reference: [Zadig 迭代管理](https://docs.koderover.com/zadig/cn/Zadig%20v4.0/sprint/overview/) · [KodeRover 迭代看板示例](https://course.koderover.com/v1/projects/detail/yaml-poc/sprints?sprintName=v3.2.4&sprintId=68f8500d33a6b255503c7fdc)  
> Contracts: `fronted/packages/contracts/src/product/sprint.ts`  
> OpenAPI: `backend/api/openapi/sprint.yaml`

---

## 1. Goal

在产品（Product）维度交付企业版迭代管理：

1. **流程模板（SprintProcess）** — 配置阶段 + 每阶段关联 ProductWorkflow
2. **迭代实例（Sprint）** — 并行多迭代、生命周期管理、归档
3. **工作项看板（WorkItem）** — 拖拽移列、执行工作流、聚合交付信息
4. **与 ReleasePlan 联动** — 同一迭代可有多条发布计划
5. **Jira 集成（Phase S4）** — Link / Import / Transition

**不在首期范围**：替代 Jira 做完整需求管理、WebSocket 协同编辑、TAPD 双向同步。

---

## 2. 概念与现有模块关系

| 概念 | 职责 | 与现有实体 |
|------|------|-----------|
| SprintProcess | 产品级流程模板 | 新增 |
| Sprint | 一次交付周期 | 演进自 `ProductPlan`；`legacy_plan_id` 兼容 |
| WorkItem | 看板卡片 | 新增 |
| ReleasePlan | 发布执行编排 | 已有；新增 `sprint_id` 外键 |
| ProductWorkflow | DevOps 执行 | 已有；WorkItem.execute 触发 Run |

### 2.1 UI Tab 调整（S0）

| 现 Tab | 现内容 | 调整后 |
|--------|--------|--------|
| `iterations` 迭代管理 | ReleasePlansPanel | **SprintBoardPanel**（看板） |
| （新增）`releases` 发布计划 | — | ReleasePlansPanel 迁入 |

---

## 3. 领域模型

### 3.1 聚合边界

```
SprintProcess (Product 级，版本化)
Sprint (聚合根)
  ├── WorkItem[]
  ├── WorkItemExecution[]  (via WorkItem)
  └── WorkItemActivity[]   (via WorkItem)
```

Sprint 与 ReleasePlan 为 **弱聚合**：通过 `sprint_id` 关联，ReleasePlan 仍归属 product 模块。

### 3.2 Sprint 状态机

```
planning ──POST /start──► active ──POST /complete──► completed ──POST /archive──► archived
```

| 状态 | 允许操作 |
|------|----------|
| planning | 编辑元数据、增删 WorkItem、Import Jira |
| active | 移列、执行工作流、创建 ReleasePlan |
| completed | 只读 + 归档 |
| archived | 只读；企业版可 reopen → active |

**规则**：

- 同一 Product **允许多个 `active` Sprint**（对标 Zadig）
- `(product_id, name)` 唯一
- `complete` 时若有 `status=executing` 的关联 ReleasePlan → `409 SPRINT_BLOCKING_RELEASE`

### 3.3 WorkItem 移列

```
POST /work-items/{id}/move
Body: { stage_id, sort_order, version }
```

处理顺序：

1. 校验 Sprint.status = active
2. 校验 RBAC：`product:developer` 及以上
3. 校验 `version`（乐观锁）→ 冲突返回 `409 WORK_ITEM_VERSION_CONFLICT`
4. 评估 `entry_policy`（Phase S5；MVP 跳过或仅 WIP）
5. 更新 stage_id、sort_order、version++
6. 写 `WorkItemActivity(action=moved_stage)`
7. 若目标阶段 `trigger_mode=auto_on_enter` → 异步触发默认 workflow（Phase S2）

### 3.4 WorkItem 执行

```
POST /work-items/{id}/execute
Body: { workflow_id?, stage_id?, input? }
→ 202 { execution_id, workflow_run_id }
```

1. 解析 workflow：请求指定 > 当前 stage 第一个 binding > 报错 `WORKFLOW_NOT_CONFIGURED`
2. 调用 `product_workflow` port：`StartRun(workflow_id, input)`
3. 创建 `WorkItemExecution(status=running)`
4. 订阅 `on_workflow_run_completed` → 更新 execution + `WorkItem.delivery` + activity

`input` 契约与现有工作流执行一致：

```ts
{ branch?, environment_overrides?, variables? }
```

---

## 4. 数据库 DDL

Migration: `0000xx_sprint_management.up.sql`

```sql
CREATE TABLE product.sprint_processes (
    id            UUID PRIMARY KEY,
    product_id    UUID NOT NULL REFERENCES product.products(id),
    name          VARCHAR(255) NOT NULL,
    version       INT NOT NULL DEFAULT 1,
    is_default    BOOLEAN NOT NULL DEFAULT TRUE,
    stages        JSONB NOT NULL DEFAULT '[]',
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_sprint_process_product_default
    ON product.sprint_processes(product_id) WHERE is_default = TRUE;

CREATE TABLE product.sprints (
    id              UUID PRIMARY KEY,
    product_id      UUID NOT NULL REFERENCES product.products(id),
    process_id      UUID NOT NULL REFERENCES product.sprint_processes(id),
    process_version INT NOT NULL,
    name            VARCHAR(255) NOT NULL,
    goal            TEXT,
    status          VARCHAR(32) NOT NULL DEFAULT 'planning',
    planned_start   TIMESTAMPTZ,
    planned_end     TIMESTAMPTZ,
    actual_start    TIMESTAMPTZ,
    actual_end      TIMESTAMPTZ,
    owner_id        UUID,
    external_ref    JSONB,
    legacy_plan_id  UUID REFERENCES product.product_plans(id),
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    UNIQUE (product_id, name)
);
CREATE INDEX idx_sprints_product_status ON product.sprints(product_id, status);

CREATE TABLE product.work_items (
    id             UUID PRIMARY KEY,
    sprint_id      UUID NOT NULL REFERENCES product.sprints(id) ON DELETE CASCADE,
    stage_id       UUID NOT NULL,
    parent_id      UUID REFERENCES product.work_items(id),
    sort_order     INT NOT NULL DEFAULT 0,
    type           VARCHAR(32) NOT NULL,
    title          VARCHAR(512) NOT NULL,
    description    TEXT,
    priority       VARCHAR(16) NOT NULL DEFAULT 'medium',
    status         VARCHAR(32) NOT NULL DEFAULT 'open',
    assignee_id    UUID,
    reporter_id    UUID,
    project_ids    UUID[] NOT NULL DEFAULT '{}',
    labels         TEXT[] NOT NULL DEFAULT '{}',
    external_ref   JSONB,
    delivery       JSONB NOT NULL DEFAULT '{}',
    blocked_reason TEXT,
    version        INT NOT NULL DEFAULT 0,
    created_by     UUID NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);
CREATE INDEX idx_work_items_board ON product.work_items(sprint_id, stage_id, sort_order);
CREATE INDEX idx_work_items_jira ON product.work_items((external_ref->>'issue_key'));

CREATE TABLE product.work_item_executions (
    id              UUID PRIMARY KEY,
    work_item_id    UUID NOT NULL REFERENCES product.work_items(id) ON DELETE CASCADE,
    stage_id        UUID NOT NULL,
    workflow_id     UUID NOT NULL,
    workflow_run_id UUID NOT NULL,
    triggered_by    UUID NOT NULL,
    status          VARCHAR(32) NOT NULL,
    artifacts       JSONB NOT NULL DEFAULT '{}',
    error_message   TEXT,
    started_at      TIMESTAMPTZ NOT NULL,
    finished_at     TIMESTAMPTZ
);
CREATE INDEX idx_work_item_executions_item ON product.work_item_executions(work_item_id, started_at DESC);

CREATE TABLE product.work_item_activities (
    id           UUID PRIMARY KEY,
    work_item_id UUID NOT NULL REFERENCES product.work_items(id) ON DELETE CASCADE,
    actor_id     UUID NOT NULL,
    action       VARCHAR(64) NOT NULL,
    detail       JSONB NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_work_item_activities_item ON product.work_item_activities(work_item_id, created_at DESC);

ALTER TABLE product.release_plans
    ADD COLUMN IF NOT EXISTS sprint_id UUID REFERENCES product.sprints(id);
CREATE INDEX IF NOT EXISTS idx_release_plans_sprint ON product.release_plans(sprint_id);
```

### 4.1 默认流程 Seed

产品首次 `GET /sprint-process` 若不存在，自动创建：

| key | name | 默认 workflow 绑定 |
|-----|------|-------------------|
| dev | 开发 | （空，手动） |
| test | 测试 | （空） |
| uat | 预发 | （空） |
| release | 发布 | （空） |

---

## 5. API 清单

完整 OpenAPI 见 `backend/api/openapi/sprint.yaml`。

### 5.1 Sprint Process

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/products/{productId}/sprint-process` | 获取默认流程 |
| PUT | `/api/v1/products/{productId}/sprint-process` | 更新（version++） |
| POST | `/api/v1/products/{productId}/sprint-process/preview` | 预览 |

### 5.2 Sprint

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/products/{productId}/sprints` | 列表 |
| POST | `/api/v1/products/{productId}/sprints` | 创建 |
| GET | `/api/v1/products/{productId}/sprints/{sprintId}` | 详情 |
| PATCH | `/api/v1/products/{productId}/sprints/{sprintId}` | 更新 |
| POST | `.../sprints/{sprintId}/start` | 启动 |
| POST | `.../sprints/{sprintId}/complete` | 完成 |
| POST | `.../sprints/{sprintId}/archive` | 归档 |
| GET | `.../sprints/{sprintId}/board` | 看板 |
| GET | `.../sprints/{sprintId}/metrics` | 度量 |
| POST | `.../sprints/{sprintId}/import/jira` | Jira 导入 |

### 5.3 WorkItem

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `.../sprints/{sprintId}/work-items` | 创建 |
| GET | `/api/v1/products/{productId}/work-items/{workItemId}` | 详情 |
| PATCH | `.../work-items/{workItemId}` | 更新 |
| DELETE | `.../work-items/{workItemId}` | 删除 |
| POST | `.../work-items/{workItemId}/move` | 拖拽 |
| POST | `.../work-items/{workItemId}/execute` | 执行工作流 |
| POST | `.../work-items/{workItemId}/link-jira` | 绑定 Jira |
| GET | `.../work-items/{workItemId}/executions` | 执行记录 |
| GET | `.../work-items/{workItemId}/activities` | 活动日志 |

### 5.4 Workspace

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/workspaces/{workspaceId}/sprints/active` | 跨产品活跃迭代 |

### 5.5 错误码

| error_code | HTTP | 说明 |
|------------|------|------|
| SPRINT_NOT_FOUND | 404 | 迭代不存在 |
| SPRINT_NAME_DUPLICATE | 409 | 同名迭代 |
| SPRINT_INVALID_TRANSITION | 409 | 非法状态转换 |
| SPRINT_BLOCKING_RELEASE | 409 | 有未完成发布计划 |
| WORK_ITEM_NOT_FOUND | 404 | 工作项不存在 |
| WORK_ITEM_VERSION_CONFLICT | 409 | 乐观锁冲突 |
| STAGE_GATE_DENIED | 409 | 阶段准入失败 |
| WORKFLOW_NOT_CONFIGURED | 422 | 阶段未绑定工作流 |
| JIRA_NOT_CONFIGURED | 422 | Workspace 未配置 Jira |

---

## 6. Contracts（Zod）

文件：`fronted/packages/contracts/src/product/sprint.ts`

主要导出：

| Schema | 用途 |
|--------|------|
| `SprintProcessVMSchema` | 流程模板 |
| `UpsertSprintProcessCommandSchema` | 更新流程 |
| `SprintSummaryVMSchema` / `SprintVMSchema` | 迭代 |
| `CreateSprintCommandSchema` | 创建迭代 |
| `WorkItemCardVMSchema` / `WorkItemVMSchema` | 工作项 |
| `MoveWorkItemCommandSchema` | 拖拽（含 version） |
| `ExecuteWorkItemCommandSchema` | 执行工作流 |
| `SprintBoardVMSchema` | 看板聚合 |
| `WorkItemExecutionVMSchema` | 执行记录 |
| `SprintMetricsVMSchema` | 度量 |
| `ImportJiraSprintCommandSchema` | Jira 导入 |

BFF 路由建议（与现网一致）：

```
/workspaces/{wsId}/products/{productId}/sprints/...
/workspaces/{wsId}/products/{productId}/sprint-process
```

---

## 7. 后端模块

```
backend/internal/modules/sprint/
├── domain/
│   ├── model/
│   ├── engine/          # board_projector, stage_gate_evaluator
│   ├── port/            # workflow_trigger, pm_connector
│   └── repository/
├── application/
│   ├── command/
│   ├── query/
│   └── subscriber/      # on_workflow_run_completed
├── infrastructure/
│   ├── persistence/
│   └── acl/             # product_workflow_acl, jira_acl
└── interface/http/
```

**BC 间约束**：sprint 模块禁止 import `product_workflow/infrastructure`；通过 port 调用 `StartWorkflowRun`。

---

## 8. 前端页面

路径（App Router）：

```
app/(platform)/products/[productId]/sprints/page.tsx           # 迭代列表
app/(platform)/products/[productId]/sprints/[sprintId]/page.tsx # 看板（默认 Tab）
app/(platform)/products/[productId]/sprints/config/page.tsx     # 流程配置
```

组件：

| 组件 | 职责 |
|------|------|
| `SprintListPanel` | 迭代列表 + 切换 |
| `SprintBoardPanel` | Kanban（列=Stage，卡=WorkItem） |
| `WorkItemDrawer` | 详情 / 执行记录 / 活动 |
| `SprintProcessConfigPanel` | 阶段 + Workflow 绑定 |
| `CreateSprintModal` | 新建迭代 |
| `CreateWorkItemModal` | 新建工作项 |
| `ExecuteWorkItemDrawer` | 复用 StartWorkflowRunModal 交互 |

看板交互：

- 拖拽移列 → `moveWorkItem`（失败 409 提示刷新）
- 卡片点击 → WorkItemDrawer
- 卡片「执行」→ ExecuteWorkItemDrawer
- WIP 超限 → 列头 warning badge

---

## 9. ReleasePlan 联动

`ReleasePlanVM` 扩展（后续 PR）：

```ts
sprint_id?: string;
```

创建 ReleasePlan 时可选 `sprint_id`；Sprint 详情页「发布计划」子 Tab 过滤 `GET /release-plans?sprint_id=`.

---

## 10. Jira 集成（Phase S4）

### 10.1 Workspace 配置

复用 `integration` 模块，新增 `JiraConnector` port：

- `GetSprintIssues(sprintId, issueTypes?)`
- `GetIssue(issueKey)`
- `TransitionIssue(issueKey, targetStatus)`

### 10.2 Import 流程

```
POST .../sprints/{id}/import/jira
{ jira_sprint_id, target_stage_id?, issue_types?, sync_mode }
```

1. 拉取 Jira Sprint Issues
2. 按 `(sprint_id, issue_key)` 去重
3. 创建 WorkItem，`external_ref` 填充，`stage_id` = target 或第一列
4. 返回 `{ imported_count, skipped_count, work_item_ids }`

### 10.3 Workflow 完成 → Jira Transition

`product_workflow` Job 类型扩展（可选）：`sparkops-jira-transition`  
或 sprint subscriber 在 execution success 时调用 connector。

---

## 11. 权限

| 操作 | 最低角色 |
|------|----------|
| 查看看板 | product:viewer |
| 创建/编辑 WorkItem、移列、执行 | product:developer |
| 配置 SprintProcess | product:admin |
| 创建/归档 Sprint | product:admin 或 sprint owner |
| Import Jira | product:admin |

---

## 12. 分期与验收

| Phase | 范围 | 验收 |
|-------|------|------|
| **S0** | Tab 重命名、ReleasePlan 迁 Tab | UI 概念清晰 |
| **S1** | SprintProcess + Sprint + WorkItem CRUD + Board | 创建 v3.2.4，看板 4 列，拖卡片 |
| **S2** | execute + execution/activity 回写 | 工作项执行 build workflow，卡片显示 Run 状态 |
| **S3** | sprint_id ↔ ReleasePlan | 迭代下创建 UAT/生产两条发布计划 |
| **S4** | Jira link + import + transition | 从 Jira 导入 Story，Run 成功更新 Issue |
| **S5** | stage gate、metrics、workspace 视图 | 企业演示 |

**S1+S2 完成定义（MVP）**：对标 KodeRover 迭代页 — 流程配置 → 创建迭代 → 看板管理工作项 → 阶段执行工作流 → 查看执行记录。

---

## 13. 测试策略

| 层级 | 覆盖 |
|------|------|
| 单元 | Sprint 状态机、move 乐观锁、board projector |
| 集成 | 创建 Sprint → 3 WorkItem → move → execute → mock workflow callback |
| 契约 | Zod schema ↔ API 响应抽样 |
| E2E | S2 后 Playwright：看板拖拽 + 执行 |

---

## 14. ProductPlan 兼容

| 旧 API | 新行为 |
|--------|--------|
| `GET/POST /products/{id}/plans` | 6 个月保留；内部可读写 `sprints` 表 `legacy_plan_id` |
| `ReleasePlan.product_plan_id` | 保留；新字段 `sprint_id` 优先 |

迁移脚本（S6）：`product_plans` → `sprints` 一行一迭代。

---

## 15. Out of Scope（本 Spec 明确不做）

- WebSocket 看板协同编辑
- TAPD/PingCode 双向同步（仅预留 `PmProvider` enum）
- 在 SparkOps 内管理 Epic 层级 backlog（以 Jira 为准）
- 修改 ProductWorkflow 引擎契约
- 独立 Project 的 Sprint（仅 product_id 有值时启用）
