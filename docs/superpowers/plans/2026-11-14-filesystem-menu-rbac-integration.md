# 文件中心菜单与 RBAC 集成 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让所有启用 Filesystem 的工作区自动获得“文件中心”一级菜单，并使菜单、页面入口和后端 API 使用一致的 entitlement 与 RBAC。

**Architecture:** 复用治理模块现有的幂等启动修复，为新旧工作区创建 `/files` 菜单并增量修复默认角色权限；扩展前后端 entitlement 路径映射；在 `/files` 服务端页面加载文件数据前执行真实 `file:read` 检查。后端 API 继续作为最终安全边界。

**Tech Stack:** Go 1.26、Gin、GORM、Testify、Next.js 15、React 19、TypeScript、Vitest。

## Global Constraints

- 根菜单标题为“文件中心”，路径和组件为 `/files`，排序为 `96`，图标为 `Folder`，`hidden` 为 `false`。
- `/files` 内保留文件、传输、存储治理、审计四个 Tab，不新增侧边子菜单。
- `developer`、`operator` 增量获得 `file:read`、`file:write`、`file:delete`。
- `reviewer`、`viewer` 和自定义角色不自动获得文件权限。
- `admin` 保留完整文件、Provider、Storage Pool、Lifecycle、Audit 权限。
- 已有非空菜单角色策略不得覆盖；显式 `filesystem: false` 不得被强制改为 `true`。
- 前端权限读取失败时必须失败关闭，不得虚构 `file:read`。
- 不新增实时容量曲线，不引入新 UI 库，不使用硬编码色值。

---

### Task 1: 幂等创建“文件中心”菜单并绑定默认菜单角色

**Files:**
- Modify: `backend/internal/modules/governance/application/command/menu_commands.go`
- Modify: `backend/internal/modules/governance/application/command/menu_commands_test.go`
- Modify: `backend/internal/modules/governance/application/command/menu_policy.go`
- Modify: `backend/internal/modules/governance/application/command/apply_menu_role_policies_test.go`

**Interfaces:**
- Consumes: `SeedWorkspaceMenusHandler.Handle(ctx, workspaceID)` 和 `ApplyMenuRolePoliciesHandler.Handle(...)`。
- Produces: `/files` 根菜单；`DefaultMenuRolePolicies["/files"] == []string{"admin", "developer", "operator"}`。

- [ ] **Step 1: 写失败测试**

在 `menu_commands_test.go` 将默认根菜单断言改为：

```go
require.Equal(t, []defaultDirectMenu{
    {Href: "/knowledge", Label: "知识库", Sort: 95},
    {Href: "/files", Label: "文件中心", Icon: "Folder", Sort: 96},
}, defaultDirectMenus)
```

在 `TestRepairBusinessNavigation_preservesLeafIDsAndMovesMenus` 的历史菜单中加入 `/files`，执行修复后断言：

```go
files, err := store.FindByPath(ctx, workspaceID, "/files")
require.NoError(t, err)
require.Equal(t, "menu-files", files.ID)
require.Nil(t, files.ParentID)
require.Equal(t, "文件中心", files.Title)
require.Equal(t, "Folder", files.Icon)
require.Equal(t, "/files", files.Component)
require.Equal(t, 96, files.Sort)
require.False(t, files.Hidden)
```

在 `apply_menu_role_policies_test.go` 加入：

```go
func TestDefaultMenuRolePolicies_filesRoles(t *testing.T) {
    require.Equal(t, []string{"admin", "developer", "operator"},
        command.DefaultMenuRolePolicies["/files"])
}
```

- [ ] **Step 2: 运行测试并确认按预期失败**

Run:

```powershell
go test ./internal/modules/governance/application/command
```

Working directory: `backend`

Expected: FAIL，原因是 `defaultDirectMenu` 尚无 `Icon`，且 `/files` 尚未进入默认菜单和角色策略。

- [ ] **Step 3: 实现最小菜单变更**

给根菜单定义增加图标并写入种子和修复：

```go
type defaultDirectMenu struct {
    Href  string
    Label string
    Icon  string
    Sort  int
}

var defaultDirectMenus = []defaultDirectMenu{
    {Href: "/knowledge", Label: "知识库", Sort: 95},
    {Href: "/files", Label: "文件中心", Icon: "Folder", Sort: 96},
}
```

`seedAll` 和 `repairBusinessNavigation` 保存直接菜单时设置：

```go
Icon: item.Icon,
```

以及：

```go
menu.Icon = item.Icon
menu.Hidden = false
```

将标题同步表和菜单角色策略补齐：

```go
"/files": "文件中心",
```

```go
"/files": {"admin", "developer", "operator"},
```

- [ ] **Step 4: 运行测试并确认通过**

Run:

```powershell
go test ./internal/modules/governance/application/command
```

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add backend/internal/modules/governance/application/command/menu_commands.go backend/internal/modules/governance/application/command/menu_commands_test.go backend/internal/modules/governance/application/command/menu_policy.go backend/internal/modules/governance/application/command/apply_menu_role_policies_test.go
git commit -m "feat(filesystem): seed file center menu"
```

---

### Task 2: 为默认成员角色增量补齐文件操作权限

**Files:**
- Modify: `backend/internal/platform/authz/permissions.go`
- Modify: `backend/internal/platform/authz/permissions_test.go`
- Modify: `backend/internal/modules/governance/application/command/role_commands.go`
- Modify: `backend/internal/modules/governance/application/command/bootstrap_test.go`

**Interfaces:**
- Consumes: `authz.DefaultWorkspaceRoles`、`authz.MergePermissions`。
- Produces: `authz.RequiredDefaultRolePermissions(name string) []string`，供启动修复决定哪些现有默认角色需要增量权限。

- [ ] **Step 1: 写失败测试**

扩展 `TestFilesystemPermissionsByWorkspaceRole`，要求：

```go
{
    role: "developer",
    want: map[string]bool{
        PermFileRead: true, PermFileWrite: true, PermFileDelete: true,
        PermFileAdmin: false, PermStorageAdmin: false, PermLifecycleAdmin: false,
    },
},
{
    role: "operator",
    want: map[string]bool{
        PermFileRead: true, PermFileWrite: true, PermFileDelete: true,
        PermFileAdmin: false, PermStorageAdmin: false, PermLifecycleAdmin: false,
    },
},
```

在 `bootstrap_test.go` 增加现有角色修复用例：

```go
func TestEnsureDefaultWorkspaceRolesAddsRequiredFilesystemPermissions(t *testing.T) {
    ctx := context.Background()
    wsID := kernel.NewWorkspaceID()
    roles := newMemoryRoleRepo()
    developer := model.NewRole(wsID, "developer", []string{authz.PermProjectRead}, nil)
    operator := model.NewRole(wsID, "operator", []string{authz.PermProjectOperate}, nil)
    reviewer := model.NewRole(wsID, "reviewer", []string{authz.PermProjectReview}, nil)
    custom := model.NewRole(wsID, "custom", []string{authz.PermProjectRead}, nil)
    for _, role := range []*model.Role{developer, operator, reviewer, custom} {
        require.NoError(t, roles.Save(ctx, role))
    }

    require.NoError(t, command.NewEnsureDefaultWorkspaceRolesHandler(roles).Handle(ctx))

    items, err := roles.ListByWorkspace(ctx, wsID)
    require.NoError(t, err)
    byName := make(map[string]*model.Role, len(items))
    for _, role := range items {
        byName[role.Name] = role
    }
    for _, name := range []string{"developer", "operator"} {
        require.Contains(t, byName[name].Permissions, authz.PermFileRead)
        require.Contains(t, byName[name].Permissions, authz.PermFileWrite)
        require.Contains(t, byName[name].Permissions, authz.PermFileDelete)
    }
    require.NotContains(t, byName["reviewer"].Permissions, authz.PermFileRead)
    require.NotContains(t, byName["custom"].Permissions, authz.PermFileRead)
}
```

- [ ] **Step 2: 运行测试并确认按预期失败**

Run:

```powershell
go test ./internal/platform/authz ./internal/modules/governance/application/command
```

Working directory: `backend`

Expected: FAIL，developer 缺少删除权限、operator 缺少三项文件权限，启动修复只合并 admin。

- [ ] **Step 3: 实现最小角色权限变更**

在 `permissions.go` 定义：

```go
var defaultMemberFilesystemPermissions = []string{
    PermFileRead,
    PermFileWrite,
    PermFileDelete,
}

func RequiredDefaultRolePermissions(name string) []string {
    switch name {
    case "admin":
        return DefaultAdminPermissions
    case "developer", "operator":
        return defaultMemberFilesystemPermissions
    default:
        return nil
    }
}
```

让 `DefaultWorkspaceRoles` 中 developer、operator 包含三项权限。将 `EnsureDefaultWorkspaceRolesHandler` 中仅处理 admin 的分支改为：

```go
required := authz.RequiredDefaultRolePermissions(definition.Name)
if len(required) == 0 {
    continue
}
merged := authz.MergePermissions(role.Permissions, required)
if len(merged) != len(role.Permissions) {
    role.UpdatePermissions(merged)
    if err := h.roles.Save(ctx, role); err != nil {
        return err
    }
}
```

- [ ] **Step 4: 运行测试并确认通过**

Run:

```powershell
go test ./internal/platform/authz ./internal/modules/governance/application/command
```

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add backend/internal/platform/authz/permissions.go backend/internal/platform/authz/permissions_test.go backend/internal/modules/governance/application/command/role_commands.go backend/internal/modules/governance/application/command/bootstrap_test.go
git commit -m "feat(filesystem): grant member file operations"
```

---

### Task 3: 完整覆盖 Filesystem entitlement 路径

**Files:**
- Modify: `backend/internal/platform/entitlements/entitlements.go`
- Modify: `backend/internal/platform/entitlements/entitlements_test.go`
- Modify: `backend/internal/platform/entitlements/api_paths.go`
- Modify: `backend/internal/platform/entitlements/api_paths_test.go`

**Interfaces:**
- Consumes: `PathEntitlement(path string)`、`APIPathEntitlement(path string)`。
- Produces: 页面 `/files` 和 Filesystem 工作区 API 均返回模块键 `filesystem`。

- [ ] **Step 1: 写失败测试**

在 `entitlements_test.go` 加入：

```go
func TestFilesystemMenuPathEntitlement(t *testing.T) {
    require.Equal(t, "filesystem", PathEntitlement("/files"))
    require.Equal(t, "filesystem", PathEntitlement("/files/node-1"))
}
```

扩展 `TestFilesystemAPIPathEntitlement` 的路径表：

```go
paths := []string{
    "/api/v1/workspaces/ws-1/namespaces",
    "/api/v1/workspaces/ws-1/files",
    "/api/v1/workspaces/ws-1/directories",
    "/api/v1/workspaces/ws-1/transfers",
    "/api/v1/workspaces/ws-1/storage-providers",
    "/api/v1/workspaces/ws-1/storage-pools",
    "/api/v1/workspaces/ws-1/storage-bindings",
    "/api/v1/workspaces/ws-1/lifecycle-policies",
    "/api/v1/workspaces/ws-1/filesystem-audits",
    "/api/v1/workspaces/ws-1/artifacts",
    "/api/v1/workspaces/ws-1/capabilities",
    "/api/v1/workspaces/ws-1/exports",
}
```

- [ ] **Step 2: 运行测试并确认按预期失败**

Run:

```powershell
go test ./internal/platform/entitlements
```

Working directory: `backend`

Expected: FAIL，`/files` 页面路径及治理类 API 尚未全部映射到 `filesystem`。

- [ ] **Step 3: 实现最小路径映射**

在 `PathEntitlement` 的 switch 中加入：

```go
case strings.HasPrefix(path, "/files"):
    return "filesystem"
```

将 `isWorkspaceFilesystemAPI` 的资源集合扩展为：

```go
switch parts[1] {
case "files", "directories", "namespaces", "transfers",
    "storage-providers", "storage-pools", "storage-bindings",
    "lifecycle-policies", "filesystem-audits", "artifacts",
    "capabilities", "exports":
    return true
default:
    return false
}
```

- [ ] **Step 4: 运行测试并确认通过**

Run:

```powershell
go test ./internal/platform/entitlements
```

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add backend/internal/platform/entitlements/entitlements.go backend/internal/platform/entitlements/entitlements_test.go backend/internal/platform/entitlements/api_paths.go backend/internal/platform/entitlements/api_paths_test.go
git commit -m "fix(filesystem): enforce entitlement on all entry points"
```

---

### Task 4: 统一前端名称并让 `/files` 入口失败关闭

**Files:**
- Modify: `fronted/apps/web/src/components/layout/nav-tree.ts`
- Modify: `fronted/apps/web/src/components/layout/nav-tree.test.ts`
- Modify: `fronted/apps/web/src/components/layout/route-labels.ts`
- Modify: `fronted/apps/web/src/components/layout/route-labels.test.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/workbench-permissions.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/workbench-permissions.test.ts`
- Modify: `fronted/apps/web/app/(platform)/files/page.tsx`

**Interfaces:**
- Consumes: `loadMyPermissions(workspaceId)`、`resolveWorkbenchPermissions(...)`。
- Produces: `hasFilesystemPageAccess(permissions: string[]): boolean`；无真实 `file:read` 时在任何文件 API 调用前返回拒绝页面。

- [ ] **Step 1: 写失败测试**

将 fallback 和路由标签断言改为“文件中心”：

```ts
expect(FILES_NAV_ITEM).toEqual({
  href: "/files",
  label: "文件中心",
  entitlement: "filesystem",
});
```

```ts
expect(resolveRouteVisitMeta("/files")).toEqual({
  label: "文件中心",
  group: "文件中心",
});
```

将权限测试改为失败关闭：

```ts
it("fails closed when workspace permissions are unavailable", () => {
  expect(resolveWorkbenchPermissions(undefined, false)).toEqual([]);
});

it("preserves real workspace permissions without inventing file read", () => {
  expect(resolveWorkbenchPermissions(["storage:read"], false)).toEqual(["storage:read"]);
});

it("grants explicit super admin access to the complete workbench", () => {
  expect(resolveWorkbenchPermissions(undefined, true)).toEqual([
    "file:read",
    "storage:read",
    "file:admin",
  ]);
});

it("requires file read for page access", () => {
  expect(hasFilesystemPageAccess([])).toBe(false);
  expect(hasFilesystemPageAccess(["file:write"])).toBe(false);
  expect(hasFilesystemPageAccess(["file:read"])).toBe(true);
});
```

- [ ] **Step 2: 运行测试并确认按预期失败**

Run:

```powershell
pnpm vitest run apps/web/src/components/layout/nav-tree.test.ts apps/web/src/components/layout/route-labels.test.ts apps/web/src/modules/filesystem/components/workbench-permissions.test.ts
```

Working directory: `fronted`

Expected: FAIL，名称仍为“文件”，权限解析仍无条件加入 `file:read`，`hasFilesystemPageAccess` 尚不存在。

- [ ] **Step 3: 实现最小前端变更**

将 fallback 和 route labels 的 `/files` 文案统一为“文件中心”。

在 `workbench-permissions.ts` 实现：

```ts
const SUPER_ADMIN_PERMISSIONS = ["file:read", "storage:read", "file:admin"];

export function resolveWorkbenchPermissions(
  workspacePermissions: string[] | undefined,
  isSuperAdmin: boolean,
): string[] {
  return [
    ...new Set([
      ...(workspacePermissions ?? []),
      ...(isSuperAdmin ? SUPER_ADMIN_PERMISSIONS : []),
    ]),
  ];
}

export function hasFilesystemPageAccess(permissions: string[]): boolean {
  return permissions.includes("file:read");
}
```

在 `page.tsx` 导入 `InlineAlert` 和 `hasFilesystemPageAccess`，并在 `loadFileNamespace` 之前返回：

```tsx
if (!hasFilesystemPageAccess(permissions)) {
  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="Filesystem"
        title="文件中心"
        description="当前角色没有访问工作区文件的权限。"
      />
      <InlineAlert tone="error">
        无权访问文件中心。请联系工作区管理员授予文件读取权限。
      </InlineAlert>
    </div>
  );
}
```

该判断必须位于 `loadFileNamespace`、`loadFileNodes`、`loadFileNode` 之前。

- [ ] **Step 4: 运行定向测试和类型检查**

Run:

```powershell
pnpm vitest run apps/web/src/components/layout/nav-tree.test.ts apps/web/src/components/layout/route-labels.test.ts apps/web/src/modules/filesystem/components/workbench-permissions.test.ts
pnpm --filter @sparkops/web typecheck
```

Expected: 全部 PASS，TypeScript 返回 exit 0。

- [ ] **Step 5: 提交**

```powershell
git add fronted/apps/web/src/components/layout/nav-tree.ts fronted/apps/web/src/components/layout/nav-tree.test.ts fronted/apps/web/src/components/layout/route-labels.ts fronted/apps/web/src/components/layout/route-labels.test.ts fronted/apps/web/src/modules/filesystem/components/workbench-permissions.ts fronted/apps/web/src/modules/filesystem/components/workbench-permissions.test.ts 'fronted/apps/web/app/(platform)/files/page.tsx'
git commit -m "fix(filesystem-ui): guard file center entry"
```

---

### Task 5: 全量验证集成结果

**Files:**
- Verify only; no production file changes expected.

**Interfaces:**
- Consumes: Tasks 1–4 的提交。
- Produces: 可复现的后端测试、前端测试、类型检查和差异检查证据。

- [ ] **Step 1: 运行后端相关测试**

Run:

```powershell
go test ./internal/platform/authz ./internal/platform/entitlements ./internal/modules/governance/application/command ./internal/modules/filesystem/interface/http
```

Working directory: `backend`

Expected: PASS，0 failures。

- [ ] **Step 2: 运行前端相关测试**

Run:

```powershell
pnpm vitest run apps/web/src/components/layout apps/web/src/modules/filesystem/components apps/web/src/server/loaders
```

Working directory: `fronted`

Expected: PASS，0 failures。

- [ ] **Step 3: 运行前端类型检查**

Run:

```powershell
pnpm --filter @sparkops/web typecheck
```

Working directory: `fronted`

Expected: exit 0。

- [ ] **Step 4: 检查差异与仓库状态**

Run:

```powershell
git diff --check
git status --short
```

Working directory: repository root

Expected: `git diff --check` 无输出；除计划文档外无未提交实现文件。

- [ ] **Step 5: 提交实施计划**

```powershell
git add docs/superpowers/plans/2026-11-14-filesystem-menu-rbac-integration.md
git commit -m "docs(filesystem): add menu RBAC implementation plan"
```
