# Project Review Roles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Workspace 默认角色与项目单级审核，使研发可提交、审核员可通过或驳回，前端卡片展示真实审核进度。

**Architecture:** 治理模块继续使用 `governance.roles`，通过默认角色定义和幂等补种扩展现有 bootstrap。项目聚合持久化独立 `review_status` 状态机；应用命令联合项目 ACL 与 Workspace 权限完成提交/审批授权。前端通过 contracts → bff-modules → BFF route → web-sdk 接入 API，由 `ProjectVM` 的能力字段驱动按钮。

**Tech Stack:** Go 1.x、Gin、GORM、PostgreSQL、Testify、TypeScript、Zod、Next.js 15、React 19、pnpm/Turbo。

## Global Constraints

- 单级审批；提交人不能审批自己，`governance:admin` 不豁免。
- Workspace 默认角色固定为 `admin`、`developer`、`reviewer`、`operator`、`viewer`，均不可删除或改名。
- `review_status` 固定为 `none | pending | approved | rejected`，不得复用项目生命周期 `status`。
- 无项目成员且无 `governance:admin` 的成员不得提交或审批。
- `pending` 时锁定配置类字段；`approved` 后修改配置类字段回到 `none`。
- 驳回意见去除空白后必须非空；本期不提供撤回，不保存审批历史。
- `project:operate` 仅作为角色权限预留，不接部署接口。
- 不提交或清理既有 `logs/sparkops.log` 及 `.next` 产物。

---

### Task 1: 默认 Workspace 角色与权限

**Files:**
- Modify: `backend/internal/platform/authz/permissions.go`
- Modify: `backend/internal/modules/governance/domain/model/role.go`
- Modify: `backend/internal/modules/governance/application/command/role_commands.go`
- Modify: `backend/internal/modules/governance/application/command/bootstrap_test.go`
- Modify: `backend/internal/modules/governance/domain/model/role_test.go`
- Modify: `backend/internal/modules/governance/module.go`
- Modify: `fronted/packages/contracts/src/governance/index.ts`

**Interfaces:**
- Produces: `PermProjectSubmit`, `PermProjectReview`, `PermProjectOperate`
- Produces: `DefaultWorkspaceRoles []DefaultRoleDefinition`
- Produces: `EnsureDefaultWorkspaceRolesHandler.Handle(ctx context.Context) error`

- [ ] **Step 1: 写失败测试**

在 `bootstrap_test.go` 覆盖新 Workspace 创建 5 个默认角色、仅 admin 绑定 owner、已有同名角色权限不被覆盖、缺失角色被补齐；在 `role_test.go` 断言 5 个默认名称均受保护。

- [ ] **Step 2: 验证 RED**

Run: `go test ./internal/modules/governance/application/command ./internal/modules/governance/domain/model`

Expected: FAIL，缺少默认角色定义/补种行为，或 `developer` 等角色尚未受保护。

- [ ] **Step 3: 最小实现**

在 `permissions.go` 增加：

```go
const (
    PermProjectSubmit  = "project:submit"
    PermProjectReview  = "project:review"
    PermProjectOperate = "project:operate"
)
```

把三个权限加入 `AllPermissions`，并加入 admin 默认权限。定义五个默认角色及规格中的权限列表；`BootstrapWorkspaceHandler` 幂等创建全部角色并仅绑定 owner 到 admin；启动 ensure 遍历现有 Workspace 角色集合并补缺。`Role.IsProtected` 改为匹配五个固定名称。前端权限选项增加对应中文项。

- [ ] **Step 4: 验证 GREEN**

Run: `go test ./internal/modules/governance/... ./internal/platform/authz/...`

Expected: PASS。

---

### Task 2: 项目审核领域状态与持久化

**Files:**
- Create: `backend/migrations/000028_project_review.up.sql`
- Create: `backend/migrations/000028_project_review.down.sql`
- Modify: `backend/internal/modules/workspace_app/domain/errors.go`
- Modify: `backend/internal/modules/workspace_app/domain/model/project.go`
- Modify: `backend/internal/modules/workspace_app/domain/model/project_test.go`
- Modify: `backend/internal/modules/workspace_app/infrastructure/persistence/project_repo.go`
- Modify: `backend/internal/modules/workspace_app/application/query/project_queries.go`

**Interfaces:**
- Produces: `type ProjectReviewStatus string`
- Produces: `Project.IsConfigComplete() bool`
- Produces: `Project.SubmitReview(actor kernel.MemberID) error`
- Produces: `Project.DecideReview(actor kernel.MemberID, decision ProjectReviewStatus, comment string) error`

- [ ] **Step 1: 写失败领域测试**

覆盖：新项目为 none；配置不全不可提交；none/rejected 可提交；pending/approved 不可提交；自己不能审批；仅 pending 可审批；驳回意见必填；pending 锁配置；approved 修改配置回 none；pending 仍可改显示名/描述。

- [ ] **Step 2: 验证 RED**

Run: `go test ./internal/modules/workspace_app/domain/model -run 'TestProject_(Submit|Review|Update)' -v`

Expected: FAIL，审核类型和方法不存在。

- [ ] **Step 3: 实现状态机与迁移**

领域字段使用：

```go
ReviewStatus  ProjectReviewStatus
SubmittedBy  *kernel.MemberID
SubmittedAt  *time.Time
ReviewedBy   *kernel.MemberID
ReviewedAt   *time.Time
ReviewComment string
```

迁移增加规格中的 6 列与 `CHECK (review_status IN ('none','pending','approved','rejected'))`。GORM model、Save/Update/rowToProject 和 `ProjectView` 完整映射。配置变化用标准化后的输入逐字段比较，不把 `display_name`、`description` 计入配置变化。

- [ ] **Step 4: 验证 GREEN**

Run: `go test ./internal/modules/workspace_app/domain/model ./internal/modules/workspace_app/infrastructure/persistence`

Expected: PASS。

---

### Task 3: 提交与审批后端 API

**Files:**
- Create: `backend/internal/modules/workspace_app/application/command/project_review.go`
- Create: `backend/internal/modules/workspace_app/application/command/project_review_test.go`
- Modify: `backend/internal/modules/workspace_app/interface/http/handler.go`
- Modify: `backend/internal/modules/workspace_app/module.go`
- Modify: `backend/internal/modules/workspace_app/application/query/project_queries.go`

**Interfaces:**
- Produces: `SubmitProjectReviewHandler.Handle(ctx, wsID, actorID, projectID) (*model.Project, error)`
- Produces: `ReviewProjectHandler.Handle(ctx, wsID, actorID, projectID, decision, comment) (*model.Project, error)`
- Produces: `POST /api/v1/projects/:projectId/submit-review`
- Produces: `POST /api/v1/projects/:projectId/review`

- [ ] **Step 1: 写失败命令测试**

用内存 fake repo、project member reader 与 permission checker 覆盖：developer+member 可提交；仅 ACL 或仅 Workspace 权限均拒绝；reviewer+viewer 可审批；提交人自审拒绝；admin 可越过项目成员限制但不可自审。

- [ ] **Step 2: 验证 RED**

Run: `go test ./internal/modules/workspace_app/application/command -run 'Test(Submit|Review)Project' -v`

Expected: FAIL，命令 handler 尚不存在。

- [ ] **Step 3: 实现命令、能力字段和路由**

命令先验证 workspace/project 归属，再分别检查项目 ACL 与 `project:submit` / `project:review`；`governance:admin` 作为项目准入及功能权限兜底。路由使用 `projectWrite`（提交）与 `projectRead`（审批）中间件，handler body：

```go
type projectReviewRequest struct {
    Decision string `json:"decision" binding:"required,oneof=approved rejected"`
    Comment  string `json:"comment"`
}
```

`ProjectView` 增加审核字段，并基于当前 actor 返回 `can_submit_review`、`can_review`。列表和详情 handler 都使用同一能力计算器，能力计算错误直接返回，不静默放行。

- [ ] **Step 4: 验证 GREEN**

Run: `go test ./internal/modules/workspace_app/...`

Expected: PASS。

---

### Task 4: Contracts、BFF 与 Web SDK

**Files:**
- Modify: `fronted/packages/contracts/src/workspace/index.ts`
- Create: `fronted/packages/bff-modules/src/workspace/projectReview.ts`
- Modify: `fronted/packages/bff-modules/src/workspace/index.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/submit-review/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/review/route.ts`
- Modify: `fronted/packages/web-sdk/src/workspace.ts`
- Modify: `fronted/packages/web-sdk/src/index.ts`（仅当 workspace namespace 需要显式导出）

**Interfaces:**
- Produces: `ProjectReviewStatusSchema`
- Produces: `DecideProjectReviewCommandSchema`
- Produces: `workspace.submitProjectReview(workspaceId, projectId)`
- Produces: `workspace.reviewProject(workspaceId, projectId, input)`

- [ ] **Step 1: 先扩充合约，使消费者类型检查失败**

`ProjectVM` 增加 `review_status`、提交/审批人和时间、意见、`can_submit_review`、`can_review`。新增：

```ts
export const ProjectReviewStatusSchema = z.enum(["none", "pending", "approved", "rejected"]);
export const DecideProjectReviewCommandSchema = z.object({
  decision: z.enum(["approved", "rejected"]),
  comment: z.string().optional().default(""),
}).superRefine((value, ctx) => {
  if (value.decision === "rejected" && !value.comment.trim()) {
    ctx.addIssue({ code: "custom", path: ["comment"], message: "驳回意见不能为空" });
  }
});
```

- [ ] **Step 2: 验证 RED**

Run: `pnpm --filter @sparkops/web typecheck`

Expected: FAIL，API 函数/路由或新增必需字段尚未接齐。

- [ ] **Step 3: 接通完整代理链**

BFF modules 调后端 `/api/v1/projects/:projectId/submit-review` 与 `/review`；Next routes 校验 body 并用 `ok/fail` 返回；web-sdk 调 BFF Workspace 路径并返回 `ProjectVM`。

- [ ] **Step 4: 验证 GREEN**

Run: `pnpm typecheck`

Expected: PASS。

---

### Task 5: 项目卡片、审核交互与详情

**Files:**
- Modify: `fronted/apps/web/src/modules/workspace/components/project/project-progress.ts`
- Create: `fronted/apps/web/src/modules/workspace/components/project/ProjectReviewModal.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectOverviewCard.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectCard.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/ProjectPanel.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectDetailModal.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectConfigurePanel.tsx`（如保存后需刷新项目）
- Modify: `fronted/apps/web/app/globals.css`（沿用实际项目样式入口）

**Interfaces:**
- Consumes: `ProjectVM.review_status/can_submit_review/can_review`
- Consumes: `workspace.submitProjectReview`, `workspace.reviewProject`
- Produces: 真实四步进度、提交按钮、审核弹窗、详情审核信息

- [ ] **Step 1: 写可测试的纯状态规则并让类型检查失败**

`computeProjectProgress` 按规格矩阵返回：none 待提交、pending 第三步 done/第四步 active、rejected 重新提交、approved 全 done。按钮决策抽为纯函数：

```ts
type ProjectPrimaryAction = "configure" | "submit" | "review" | "pending";
export function projectPrimaryAction(project: ProjectVM): ProjectPrimaryAction
```

- [ ] **Step 2: 验证 RED**

Run: `pnpm --filter @sparkops/web typecheck`

Expected: FAIL，组件尚未处理新增状态/动作。

- [ ] **Step 3: 实现交互**

项目列表持有提交/审核 loading 与 modal 状态；API 成功后用返回的 ProjectVM 原位替换列表项并同步详情。审核弹窗含通过、驳回、意见输入；驳回无意见禁用确认。错误显示现有 `InlineAlert`。详情展示审核状态、提交/审批时间与意见；配置保存遇 `PROJECT_REVIEW_LOCKED` 显示「项目审核中，无法修改配置」。

- [ ] **Step 4: 验证 GREEN**

Run: `pnpm --filter @sparkops/web typecheck`

Expected: PASS。

---

### Task 6: 全链路验证与回归

**Files:**
- Verify only; do not modify generated `.next` files or `logs/sparkops.log`

- [ ] **Step 1: 格式化**

Run: `gofmt -w <本计划中修改的 Go 文件>`

Expected: 无输出。

- [ ] **Step 2: 后端回归**

Run: `go test ./...`

Expected: PASS。

- [ ] **Step 3: 前端回归**

Run: `pnpm typecheck && pnpm build`

Expected: PASS。

- [ ] **Step 4: 迁移/API 冒烟**

启动服务后，用 developer 项目成员提交，换 reviewer 项目成员驳回并填写意见，再重新提交后通过；验证 pending 配置更新 409、自审 403、进度条状态与详情字段一致。

- [ ] **Step 5: 检查工作区**

Run: `git status --short`

Expected: 仅规格、计划及本功能源代码/迁移改动；不纳入日志和构建产物。
