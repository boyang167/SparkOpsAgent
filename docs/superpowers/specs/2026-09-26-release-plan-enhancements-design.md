# 发布计划增强设计（A + B + C）

> Date: 2026-09-26  
> Status: **Implemented — A+B+C 首版**  
> Reference: [Zadig 发布计划 v4.3](https://docs.koderover.com/zadig/cn/Zadig%20v4.3/release/plan/) · [KodeRover 示例](https://course.koderover.com/v1/releasePlan/index)  
> 前置: `2026-09-25-sprint-management-design.md` · 现有 `ReleasePlansPanel` / `release_plan.go`

---

## 1. 目标

在现有 Release Plan 后端骨架上，分三块交付对标 Zadig 的体验：

| 代号 | 范围 | 成功标准 |
|------|------|----------|
| **A** | 核心发布流程 UX | 规划→审批→执行全链路可在产品/迭代内完成，无需改 API 手工调 |
| **B** | 工作区视图 | 工作区有跨产品发布计划列表 + 日历视图，可跳转产品详情 |
| **C** | 迭代 / JIRA 联动 | 发布计划与 Sprint/WorkItem/JIRA 信息可见、可操作，完成迭代阻塞有明确引导 |

**本期不做**（留阶段 D）：飞书/钉钉审批、外部 Hook、定时自动执行调度器、发布计划模板复制。

---

## 2. 方案对比

### 2.1 工作区聚合 API

| 方案 | 说明 | 优劣 |
|------|------|------|
| **A1 专用 workspace 查询（推荐）** | `GET /workspaces/:wsId/release-plans`，JOIN product 表返回 `product_name` | 一次请求、权限清晰、与 `listWorkspaceWorkflows` 一致 |
| A2 前端 N+1 | 列产品后逐个 `listReleasePlans` | 无后端改动，产品多时慢、权限难统一 |
| A3 复用 dashboard 扩展 | 在 dashboard API 塞 release plans | 耦合 dashboard，不利于独立页 |

**推荐 A1**。

### 2.2 日历视图

| 方案 | 说明 | 优劣 |
|------|------|------|
| **B1 自研月视图 + 列表切换（推荐）** | CSS Grid 月历，按 `release_window_start/end` 渲染色块 | 零依赖、风格统一、满足 Zadig 80% 场景 |
| B2 引入 react-big-calendar | 成熟组件 | 新依赖、主题适配成本 |
| B3 仅时间轴 List | 按周分组列表 | 实现快但不对标 Zadig「日历模式」 |

**推荐 B1**：列表/日历 Tab 切换，日历点击条目跳转产品发布计划详情。

### 2.3 需求 / JIRA 关联

| 方案 | 说明 | 优劣 |
|------|------|------|
| **C1 Sprint 派生展示（推荐）** | Release Plan 已有 `sprint_id`；详情页展示关联 Sprint 的 WorkItem 清单 + PM Link | 无新表、与现有 PM 集成一致 |
| C2 独立 requirement_refs | `release_plans.requirement_refs JSONB` | 可跨 Sprint，但重复 sprint 已有数据 |
| C3 双向 JIRA Sprint ID | Release Plan 存 JIRA sprint key | 与 WorkItem PM link 重复 |

**推荐 C1**，附加轻量能力：**从 Sprint WorkItem 一键生成 text 检查项**（规划阶段）。

---

## 3. 架构

```
┌─────────────────────────────────────────────────────────────┐
│  Workspace: /publish/release-plans                          │
│  WorkspaceReleasePlansPanel (list | calendar)               │
└──────────────────────────┬──────────────────────────────────┘
                           │ GET /workspaces/:wsId/release-plans
┌──────────────────────────▼──────────────────────────────────┐
│  Product: ReleasePlansPanel (enhanced)                      │
│  ├── ReleasePlanForm (create/edit metadata)                 │
│  ├── ReleasePlanJobList (reorder, owner, wf params)         │
│  └── SprintWorkItemsSummary (C: linked sprint items)          │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────┐
│  product module (existing + extensions)                     │
│  ReleasePlanExecutor · CompleteSprint blocking              │
└─────────────────────────────────────────────────────────────┘
```

组件拆分（A 避免 `ReleasePlansPanel` 继续膨胀）：

| 新组件 | 职责 |
|--------|------|
| `ReleasePlanForm.tsx` | 创建/编辑：名称、描述、窗口、负责人、审批开关、sprint |
| `ReleasePlanJobEditor.tsx` | 发布项列表：排序、执行人、text/workflow、参数 |
| `ReleasePlanStatusSteps.tsx` | 状态步骤条 |
| `SprintWorkItemsSummary.tsx` | C：展示 sprint 下 WorkItem + JIRA chip |
| `WorkspaceReleasePlansPanel.tsx` | B：工作区列表 + 日历 |
| `ReleasePlanCalendar.tsx` | B：月视图 |

复用：`WorkItemAssigneeField`（负责人/执行人）、`ProductMasterDetail`（主从布局）、`WorkspaceWorkflowsPanel`（跨产品列表模式）。

---

## 4. 后端变更（A）

### 4.1 新增 API

| Method | Path | 说明 |
|--------|------|------|
| `PATCH` | `/api/v1/products/:productId/release-plans/:planId` | 仅 `planning` 可改元数据 |
| `POST` | `/api/v1/products/:productId/release-plans/:planId/cancel` | 调用 `ReleasePlan.Cancel()` |
| `PUT` | `/api/v1/products/:productId/release-plans/:planId/jobs/reorder` | body: `{ job_ids: string[] }` |
| `PATCH` | `/api/v1/products/:productId/release-plans/:planId/jobs/:jobId` | 仅 `planning`；name/owner/text/workflow_ref/sort_order |
| `DELETE` | `/api/v1/products/:productId/release-plans/:planId/jobs/:jobId` | 仅 `planning` |

已有 `POST .../reject` — 补 BFF/SDK/前端。

### 4.2 UpdateReleasePlan 字段

```go
type UpdateReleasePlanInput struct {
    Name               *string
    Description        *string
    ManagerID          *kernel.MemberID
    ApproverID         *kernel.MemberID
    ReleaseWindowStart *time.Time
    ReleaseWindowEnd   *time.Time
    ApprovalEnabled    *bool
    SprintID           *kernel.SprintID // 可改关联 sprint；nil 表示清除需 explicit 约定
}
```

规则：
- 仅 `status=planning` 可 PATCH
- `manager_id` 默认创建者，可改
- `approver_id` 审批启用时可选
- 拒绝（`denied`）后允许 PATCH 回到可编辑态：新增 `ReleasePlan.ReopenForPlanning()` → `status=planning`

### 4.3 Reject 后重新规划

`Reject()` 保持 `denied`；新增 `ReopenPlanning()`：
- `denied` → `planning`
- 清空审批时间类字段（若有）
- audit: `reopen_planning`

前端：拒绝后展示「重新编辑」按钮 → PATCH + 继续规划。

### 4.4 List 增强

`GET /api/v1/products/:productId/release-plans` 增加 query：
- `status` — 逗号分隔过滤
- 已有 `sprint_id`

---

## 5. 后端变更（B）

### 5.1 Workspace 聚合列表

```
GET /api/v1/workspaces/:wsId/release-plans
  ?status=planning,executing
  ?from=2026-09-01T00:00:00Z   // 窗口重叠过滤（可选）
  ?to=2026-09-30T23:59:59Z
```

Response item（`WorkspaceReleasePlanVM`）：

```typescript
{
  id, product_id, product_name,
  sprint_id?, sprint_name?,
  name, status,
  release_window_start, release_window_end,
  manager_id?, created_at
}
```

实现：`release_plan_repo.ListByWorkspace(ctx, wsID, filter)` JOIN `products` ON product_id，权限 `PermProjectRead` 过滤可见产品。

### 5.2 路由注册

在 `product/module.go` 或 workspace 路由组注册，与 `GET /workspaces/:wsId/sprints/active` 同级。

---

## 6. 后端变更（C）

### 6.1 查询增强（无新表）

`GET release-plans/:planId` 响应扩展：

```typescript
sprint?: {
  id, name, status,
  external_ref?: Record<string, unknown>  // JIRA sprint 元数据
}
linked_work_items?: Array<{
  id, title, stage_id, status,
  pm_links?: Array<{ provider, external_key, url? }>
}>
```

实现：`GetReleasePlanHandler` 若 `plan.SprintID != nil`，加载 Sprint + `ListWorkItemsBySprint`。

### 6.2 从 WorkItem 生成 Jobs

```
POST /api/v1/products/:productId/release-plans/:planId/jobs/from-work-items
Body: { work_item_ids: string[], mode: "text_checklist" }
```

- 仅 `planning`
- 每个 WorkItem 生成一条 `type=text` job：`name=WorkItem.title`，`text_content=PM key + title`
- `sort_order` 追加在末尾
- audit: `add_jobs_from_work_items`

### 6.3 Sprint 完成阻塞 UX（后端已有）

`CompleteSprint` → `409 SPRINT_BLOCKING_RELEASE`。响应 body 增加：

```json
{
  "code": "SPRINT_BLOCKING_RELEASE",
  "blocking_plans": [{ "id", "name", "status" }]
}
```

便于前端展示可点击 plan 列表。

---

## 7. 前端变更（A）

### 7.1 ReleasePlanForm

创建 Modal 扩展字段：
- 名称、描述
- 发布窗口（datetime-local ×2）
- 发布负责人（`WorkItemAssigneeField`）
- 启用审批（switch）
- 关联迭代（select，embedded 时默认当前 sprint 且 disabled）

编辑：planning / denied 态在详情页「编辑」打开同一 Form。

### 7.2 ReleasePlanJobEditor

- 规划态：拖拽排序（HTML5 DnD 或上下箭头，调用 reorder API）
- 每项：执行人选择、text 内容 textarea、workflow 选择 + parameters key-value
- 删除 job（planning only）

### 7.3 执行态

| 能力 | 行为 |
|------|------|
| Workflow Run 链接 | `href=/products/{pid}/workflows/{wfId}/runs/{runId}` |
| Text 完成 | Modal 输入备注 → `done` API（后端 `MarkTextJobDone` 可扩展存 remark） |
| 审批 | 通过 / **驳回**（reason Modal，调 reject API） |
| 取消 | planning/wait_for_approval/executing 显示取消，confirm 后调 cancel API |
| 拒绝后 | 「重新编辑」→ reopen API 或 PATCH 触发 reopen |

### 7.4 状态步骤条中文化

`planning`→规划中 · `wait_for_approval`→待审批 · `executing`→执行中 · `success`→已完成 · `denied`→已驳回 · `cancel`→已取消

---

## 8. 前端变更（B）

### 8.1 路由与导航

- 新页：`fronted/apps/web/app/(platform)/publish/release-plans/page.tsx`
- 侧栏 Publish 菜单增加「发布计划」→ `/publish/release-plans`
- 旧 `/publish/plans`（ProductPlan）保留，标注 legacy 或后续迁移

### 8.2 WorkspaceReleasePlansPanel

- 顶部：搜索、状态筛选、产品筛选
- 视图切换：**列表** | **日历**
- 列表列：计划名、产品、迭代、状态、窗口、负责人、操作（查看 → 深链 `/products/{pid}?tab=releases&planId={id}`）
- 日历：`ReleasePlanCalendar` 月视图，条目按窗口着色，点击打开详情

### 8.3 深链

产品页 `ProductDetailPanel` / `ReleasePlansPanel` 支持 query `planId` 自动选中。

---

## 9. 前端变更（C）

### 9.1 SprintWorkItemsSummary

在 Release Plan 详情（有关联 sprint 时）展示：
- Sprint 名称 + 状态 + ExternalRef（JIRA sprint 名若有）
- WorkItem 表格：标题、阶段、PM Link chip、执行状态
- 规划态按钮：**「从工作项生成检查项」**（多选 → from-work-items API）

### 9.2 SprintBoardPanel 联动

- 嵌入「发布」Tab 已有 `sprintId`
- 完成迭代失败 `SPRINT_BLOCKING_RELEASE`：InlineAlert 列出 blocking plans，链接到对应 plan
- Sprint 头部展示 active release plan 数量 badge

### 9.3 JIRA 信息来源

不新建 ReleasePlan↔JIRA 直连；通过：
1. Sprint `external_ref`（import PM sprint 时写入）
2. WorkItem `pm_links`（已有 link-jira API）

---

## 10. Contracts / BFF / SDK

新增/扩展 `fronted/packages/contracts/src/product/release-plan.ts`：

- `UpdateReleasePlanCommand`
- `RejectReleasePlanCommand` `{ reason?: string }`
- `ReorderReleasePlanJobsCommand`
- `UpdateReleasePlanJobCommand`
- `WorkspaceReleasePlanVM`
- `ReleasePlanVM` 扩展 `sprint`, `linked_work_items`

BFF 路由镜像 backend；SDK `product.ts` 补方法。

---

## 11. 权限

沿用产品级：
- Read: `PermProjectRead`
- Write: `PermProjectWrite`

Workspace 列表：仅返回用户有 read 权限的产品下的 plans。

执行/审批：本期不强制「仅 manager 可执行」（Zadig 行为）；UI 展示负责人，后续可加校验。

---

## 12. 实施顺序

| 阶段 | 内容 | 预估 |
|------|------|------|
| **P1** | 后端：update/cancel/reorder/job CRUD/reject BFF/workspace list/sprint work items in GET | 2d |
| **P2** | 前端 A：Form、JobEditor、reject/cancel/run link | 2d |
| **P3** | 前端 B：Workspace 页 + Calendar | 2d |
| **P4** | 前端 C：WorkItemsSummary、from-work-items、blocking UX | 1d |
| **P5** | 联调 + 阻塞 sprint E2E 验证 | 0.5d |

---

## 13. 测试要点

- planning 态 CRUD job + reorder
- 审批开：planning → finish → wait → approve → executing → 串行 workflow
- 审批关：planning → finish → executing
- reject → reopen → 再 finish
- cancel 各态
- workspace list 跨产品 + 日历渲染窗口重叠
- sprint 关联 plan + from-work-items
- complete sprint with executing plan → 409 + blocking_plans

---

## 14. 阶段 D（已实现）

| 能力 | 说明 |
|------|------|
| 飞书/钉钉审批 | `ApprovalConfig` 扩展 provider/webhook/instance；完成规划发卡片；公开 token 回调 approve/reject |
| 定时自动执行 | `schedule_execute_time` + 分钟级调度器；到期自动推进首个 workflow 项 |
| 复制 / 模板 | `POST .../copy`；工作区模板 CRUD；工作区页「从模板创建」向导 |
| 富文本检查项 | `RichTextField`（HTML）；`text_format` 字段 |

**仍不在范围：**

- Release Plan 级细粒度 RBAC（仅 manager 可执行等）
- TipTap 等重型编辑器替换（当前为轻量 contenteditable）
