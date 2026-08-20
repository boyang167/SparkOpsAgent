# 项目审核角色与审批流程设计

> 替代 `2026-08-18-project-workbench-design.md` 中「提交审核 / 审核完成不持久化」的约定。工作台卡片与配置完成判定仍沿用该文档。

## 目标

在 Workspace 治理角色中提供可复用的默认角色，并给项目增加单级审核状态，使配置完成后的「提交审核」能走真实审批：研发提交、审核员通过或驳回。

## 非目标（本规格不包含）

- 多级会签、会签超时、Workflow `approval` 节点。
- 审核通过后的构建 / 部署落地（仅预留 `project:operate`）。
- 组织级 `owner/admin/member`、平台 `super_admin`、项目 ACL `lead/member/viewer` 的语义变更。
- 生成模板内容落库。

## 背景

当前三层角色互不覆盖审批：

| 层级 | 现有能力 | 缺口 |
|------|----------|------|
| 平台 | `super_admin` | 不参与项目审批 |
| 组织 | `owner` / `admin` / `member` | 只管组织成员 |
| Workspace 治理 | 创建 Workspace 只种 `admin` | 无研发 / 审核员 / 运维 / 只读 |
| 项目成员 | `lead` / `member` / `viewer` | 只做项目 ACL，无审批权限码 |

项目 `status` 仅有 `active` / `archived`。卡片第四步「审核完成」目前恒为 pending。

## 决策摘要

1. **审批权限挂在 Workspace 治理角色上**，不新增第四套角色表。
2. **项目成员身份仍是准入条件**：无项目成员且无 `governance:admin` 的人不能提交或审批该项目。
3. **单级审批**：一次提交、一人审批；提交人不能审自己。
4. **审核状态独立字段**，不占用 `projects.status`（生命周期仍为 active/archived）。
5. **待审核期间锁定配置类字段**，避免边审边改。

---

## 默认角色

每个 Workspace 必须存在下列 5 个角色（`governance.roles.name`，英文标识，不可删除）。创建 Workspace 时创建人仍只绑定 `admin`；其余角色只创建、不自动绑人。

| name | 界面展示 | 职责 |
|------|----------|------|
| `admin` | 管理员 | 空间管理；含提交与审批 |
| `developer` | 研发 | 创建/配置项目并提交审核 |
| `reviewer` | 审核员 | 查看配置并审批，不能改项目配置 |
| `operator` | 运维 | 查看项目；后续部署/构建使用 |
| `viewer` | 只读 | 仅查看 |

### 默认权限

与现有常量风格一致，新增三个权限码：

- `project:submit` — 提交审核、重新提交
- `project:review` — 通过 / 驳回
- `project:operate` — 预留给部署/构建，本规格不接业务接口

| 角色 | 权限 |
|------|------|
| `admin` | 现有 `DefaultAdminPermissions` **加上** `project:submit`、`project:review`、`project:operate` |
| `developer` | `project:read`、`project:write`、`project:submit`、`secret:read`、`agent:read`、`chat:use` |
| `reviewer` | `project:read`、`project:review`、`secret:read` |
| `operator` | `project:read`、`project:operate`、`secret:read` |
| `viewer` | `project:read` |

`governance:admin` 仅 `admin` 拥有。`project:write` 不隐含提交或审批。

前端 `PERMISSION_OPTIONS` 增加上述三项中文标签：项目 · 提交审核、项目 · 审批、项目 · 运维操作。

### 保护与补种规则

- `IsProtected()`：上述 5 个 `name` 均不可删除、不可改名。
- 权限：`admin` 启动时仍与 `DefaultAdminPermissions` 做并集合并（现有 `EnsureDefaultAdminPermissionsHandler` 扩展新权限）。
- 其余默认角色：**仅当该 Workspace 缺少该 name 时插入默认权限**；已存在则不覆盖用户改过的权限列表。
- 触发点：
  1. 创建 Workspace（扩展 `BootstrapWorkspaceHandler`）
  2. 治理模块启动时遍历已有 Workspace 补种缺失角色（扩展现有 ensure 逻辑）
- 种子数据 / 已有租户：走启动补种，不要求手工 SQL 绑人。

管理员需在「权限 / 角色」页把成员绑到 `reviewer` 等角色，并在项目里加为成员，审批才能发生。

---

## 项目审核状态

### 字段（`workspace_app.projects`）

| 列 | 类型 | 说明 |
|----|------|------|
| `review_status` | `TEXT NOT NULL DEFAULT 'none'` | `none` / `pending` / `approved` / `rejected` |
| `submitted_by` | `UUID NULL` | 提交人 member_id |
| `submitted_at` | `TIMESTAMPTZ NULL` | |
| `reviewed_by` | `UUID NULL` | 审批人 member_id |
| `reviewed_at` | `TIMESTAMPTZ NULL` | |
| `review_comment` | `TEXT NOT NULL DEFAULT ''` | 驳回时建议必填；通过可空 |

不另建审核历史表（本规格）。再次提交覆盖上一轮提交/审批字段。

`review_status` 与 `status`（active/archived）正交：已归档项目禁止提交与审批。

### 状态机

```
none ──submit──► pending ──approve──► approved
                   │
                   └──reject──► rejected ──submit──► pending

approved ──配置类字段被修改──► none
rejected ──配置类字段被修改──► rejected（可继续改，再 submit）
pending  ──禁止改配置类字段──► （只能等审批或由 governance:admin 撤回，见下）
```

**本规格不提供「撤回提交」接口。** 待审核项目只能审批通过或驳回。

配置类字段：`git_url`、`secret_id`、`git_ref_type`、`git_ref_value`、`dockerfile_path`、`template_id`、`resources`、`namespace`、`language`。  
非配置类：`display_name`、`description`。待审核时仍允许改显示名与描述。

`PATCH` 项目：

- `review_status = pending` 且请求改了配置类字段 → `409 PROJECT_REVIEW_LOCKED`
- `review_status = approved` 且改了配置类字段 → 保存成功并将 `review_status` 置为 `none`，清空本轮审批人/时间（保留 comment 可选，建议清空）
- `archived` → 维持现有不可更新

### 提交前置条件（全部满足）

1. 项目 `status = active`
2. 配置完成：与前端 `isProjectConfigComplete` 一致——`git_url`、`git_ref_value` 非空，Dockerfile 有值（缺省 `./Dockerfile`），且 `template_id` 或 resources 中三个模板 ID 至少其一非空
3. `review_status` 为 `none` 或 `rejected`（已 pending / approved 不可再提交）
4. 调用者有项目写权限（Casbin/项目成员 `lead`/`member`，或 `governance:admin`）
5. 调用者 Workspace 角色含 `project:submit`（`governance:admin` 视为通过）

### 审批前置条件

1. 项目 `status = active` 且 `review_status = pending`
2. 调用者有项目读权限（项目成员含 `viewer`，或 `governance:admin`）
3. 调用者含 `project:review`（`governance:admin` 视为通过）
4. 调用者 `member_id != submitted_by`（自己不能审自己；`governance:admin` **不豁免** 这条）

决策：`approved` | `rejected`。`rejected` 时 `review_comment` 去除空白后长度 ≥ 1。

---

## HTTP API

沿用现有项目路由前缀（需 Workspace 上下文中间件与项目访问校验）。

### `POST /api/v1/projects/:projectId/submit-review`

无 body。成功 `200`，返回更新后的 `ProjectVM`（含审核字段）。

失败：

| 条件 | code | HTTP |
|------|------|------|
| 未登录 | `AUTH_UNAUTHORIZED` | 401 |
| 无项目写权限 | `PROJECT_ACCESS_DENIED` | 403 |
| 无 `project:submit` | `AUTH_FORBIDDEN` | 403 |
| 配置未完成 | `PROJECT_CONFIG_INCOMPLETE` | 400 |
| 状态不允许提交 | `PROJECT_REVIEW_INVALID_STATE` | 409 |
| 已归档 | 现有项目非 active 错误 | 409 |

### `POST /api/v1/projects/:projectId/review`

```json
{ "decision": "approved" | "rejected", "comment": "可选；驳回必填" }
```

成功 `200`，返回更新后的 `ProjectVM`。

失败：无 `project:review` → `AUTH_FORBIDDEN` 403；自己审自己 → `PROJECT_REVIEW_SELF` 403；状态非 pending → `PROJECT_REVIEW_INVALID_STATE` 409；驳回无意见 → `INVALID_REQUEST` 400。

### `GET` / 列表

`ProjectVM` 增加只读审核字段，供卡片与详情使用。列表不必联表查显示名；详情/卡片可用已有成员接口补全名字，本规格不强制 join。

---

## 领域模型

`Project` 增加审核字段与方法（示意）：

- `SubmitReview(actor MemberID) error`
- `DecideReview(actor MemberID, approved bool, comment string) error`
- `ApplyUpdate(...)` 在现有 `UpdateDetails` 中嵌入锁定与 approved→none 规则

配置完成判定放在 workspace_app 领域或 application 层一处，前后端规则必须一致。

---

## 前端

### 进度条（`computeProjectProgress`）

| `review_status` | 配置完成 | 第 2 步 | 第 3 步 提交审核 | 第 4 步 审核完成 | `statusLabel` |
|-----------------|----------|---------|------------------|------------------|---------------|
| 任意 | 否 | active | pending | pending | 创建完成 / 配置中 |
| `none` | 是 | done | active | pending | 待提交审核 |
| `pending` | 是 | done | done | active | 审核中 |
| `rejected` | 是 | done | active | pending | 审核驳回 |
| `approved` | 是 | done | done | done | 审核通过 |

第 3 步在 `rejected` 时 subtitle 用「重新提交」；`pending` 用「等待审核」。

### 按钮

| 条件 | 主操作 |
|------|--------|
| 未配置完成 | 继续配置 |
| 已配置且 `none`/`rejected`，当前用户可 submit | 提交审核 |
| `pending`，当前用户可 review 且不是提交人 | 审核（打开通过/驳回对话框） |
| `pending`，当前用户是提交人或无可 review | 不显示提交；可显示「审核中」禁用按钮 |
| `approved` | 继续配置（改配置会使状态回到 none） |

当前用户权限：前端用已有治理权限查询（或项目详情附带 `can_submit_review` / `can_review` 布尔字段）。**推荐 API 在 ProjectVM 上返回这两个布尔值**，避免前端自己拼角色。计算规则与后端前置条件一致（含「不能审自己」）。

### 详情弹窗

「基本信息」展示：审核状态、提交时间、审批时间、审批意见。占位「未审核」改为真实 `review_status` 文案。

### 角色管理页

默认角色显示中文名；删除按钮对受保护角色禁用（已有 admin 保护，扩展到 5 个 name）。

---

## 合约与 BFF

- `fronted/packages/contracts`：`ProjectVM` 增加审核字段与 `can_submit_review`、`can_review`；新增 `SubmitProjectReview`（空）与 `DecideProjectReview` schema。
- Web SDK + BFF：`POST .../projects/:id/submit-review`、`POST .../projects/:id/review`。
- 治理合约：`PERMISSION_OPTIONS` 与 AllPermissions 对齐。

---

## 错误与文案（界面）

- 配置未完成：请先完成 Git 与模板配置
- 待审核锁定：项目审核中，无法修改配置
- 不能审自己：不能审批自己提交的项目
- 无审核员：提交成功后卡片进入「审核中」；若无人绑定 `reviewer`，由空间管理员在角色页分配（产品不在提交时阻断）

---

## 验证

1. 新 Workspace 角色列表出现 5 个默认角色；创建人仅绑定 admin。
2. 老 Workspace 重启后端后自动补齐缺失默认角色，不覆盖已改权限的同名角色。
3. 仅 developer（加项目 member）可提交；仅 reviewer 可通过；admin 两者都可以，但不能审自己提交的单。
4. 配置未完成提交返回 400；pending 时改 Git 返回 409。
5. 驳回后可改配置并再次提交；通过后改进程条第 4 步为 done；再改 Git 后第 3/4 步回到待提交。
6. 卡片按钮与 `can_submit_review` / `can_review` 一致。

## 修订关系

- 工作台进度条四步含义以本文「前端 / 进度条」为准。
- `operator` 与 `project:operate` 只建角色与权限，不接发布流水线。
