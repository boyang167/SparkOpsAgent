# Business Navigation Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将业务控制台侧栏升级为“产品交付、运行观测、Kubernetes、知识库、配置与集成”，并安全迁移已有工作区菜单与权限。

**Architecture:** 保留所有业务 URL 和已有菜单 ID，由治理菜单 repair 原地修改标题、父级和排序；前端同时支持治理 API 动态菜单与静态 fallback。Kubernetes 仅收敛侧栏入口，原深层页面继续保留，并通过页面内分类导航和 active alias 访问。

**Tech Stack:** Go、GORM、React 19、Next.js 15、TypeScript、Vitest、YAML

## Global Constraints

- 不改变现有业务页面 URL、API 路由和 Kubernetes 深层页面。
- 不改变 `/cronjobs` 当前的 `k8s` entitlement。
- 已有角色菜单绑定必须通过保留菜单 ID 延续。
- 管理控制台与平台运营中心菜单不在本次范围。
- 不引入新的 UI 库，沿用现有设计 token 和导航样式。

---

### Task 1: 治理菜单定义与旧工作区迁移

**Files:**
- Modify: `backend/internal/modules/governance/application/command/menu_commands.go`
- Modify: `backend/internal/modules/governance/application/command/menu_commands_test.go`
- Modify: `backend/internal/modules/governance/application/command/menu_policy.go`

**Interfaces:**
- Produces: `defaultMenuGroups` 中的新业务分组定义。
- Produces: `repairBusinessNavigation(ctx, workspaceID) error`，原地迁移已有菜单。
- Preserves: 叶子菜单原 path 与 ID。

- [ ] **Step 1: 写失败测试**

在 `menu_commands_test.go` 增加结构断言：

```go
func TestDefaultMenuGroups_matchBusinessNavigation(t *testing.T) {
	require.Equal(t, "产品交付", defaultMenuGroups[0].Label)
	require.Equal(t,
		[]string{"/publish/products", "/projects", "/publish/release-plans", "/products/workflows", "/cronjobs", "/templates"},
		menuGroupPaths(defaultMenuGroups[0]),
	)
	require.Equal(t, "运行观测", defaultMenuGroups[1].Label)
	require.Equal(t,
		[]string{"/runtime/pods", "/monitor/overview", "/monitor/log-viewer"},
		menuGroupPaths(defaultMenuGroups[1]),
	)
	require.Equal(t, "配置与集成", defaultMenuGroups[2].Label)
	require.Equal(t,
		[]string{"/config/secrets", "/config/webhooks", "/config/mcp"},
		menuGroupPaths(defaultMenuGroups[2]),
	)
}
```

并增加 sqlite 迁移测试：准备旧 `application`、`ops-delivery`、`ops-runtime`、`ops-observability` 菜单，执行 `Handle` 后断言叶子菜单 ID 不变、父级和排序正确、`/knowledge` 成为根节点、旧根被软删除。

- [ ] **Step 2: 运行测试确认失败**

Run:

```powershell
cd backend
go test ./internal/modules/governance/application/command -run "TestDefaultMenuGroups|TestRepairBusinessNavigation" -count=1
```

Expected: FAIL，旧分组仍为“构建/交付/运行/观测”。

- [ ] **Step 3: 实现新定义与原地迁移**

保留根 ID `application`、`ops-runtime`、`ops-infra`，更新分组：

```go
var defaultMenuGroups = []defaultMenuGroup{
	{
		ID: "application", Label: "产品交付",
		Items: []struct{ Href, Label string }{
			{"/publish/products", "产品"},
			{"/projects", "项目"},
			{"/publish/release-plans", "发布计划"},
			{"/products/workflows", "工作流"},
			{"/cronjobs", "定时构建"},
			{"/templates", "模板中心"},
		},
	},
	{
		ID: "ops-runtime", Label: "运行观测",
		Items: []struct{ Href, Label string }{
			{"/runtime/pods", "运行实例"},
			{"/monitor/overview", "监控概览"},
			{"/monitor/log-viewer", "日志查询"},
		},
	},
	{
		ID: settingsGroupID, Label: settingsGroupTitle, Sort: settingsGroupSort,
		Items: []struct{ Href, Label string }{
			{"/config/secrets", "密钥管理"},
			{"/config/webhooks", "外部集成"},
			{"/config/mcp", "MCP 集成"},
		},
	},
}
```

新增 `defaultDirectMenus`，以 `Sort: 95` 创建或迁移 `/knowledge`。新增通用 repair：先确保目标根存在，再按 path 更新 title、parent、sort；迁移完成后仅软删除空的 `ops-delivery` 与 `ops-observability`。删除会与新结构冲突的旧 `repairProductWorkflowsMenu`、`repairReleasePlansMenu` 调用。为 `/config/mcp` 增加默认角色策略。

- [ ] **Step 4: 运行治理菜单测试**

```powershell
cd backend
go test ./internal/modules/governance/application/command ./internal/modules/governance/infrastructure/persistence -count=1
```

Expected: PASS。

---

### Task 2: 前端导航模型与折叠行为

**Files:**
- Modify: `fronted/apps/web/src/components/layout/nav-tree.ts`
- Modify: `fronted/apps/web/src/components/layout/menu-tree-nav.ts`
- Modify: `fronted/apps/web/src/components/layout/menu-tree-nav.test.ts`
- Modify: `fronted/apps/web/src/components/layout/PlatformNav.tsx`
- Modify: `fronted/apps/web/src/components/layout/NavGroup.tsx`

**Interfaces:**
- Produces: `KNOWLEDGE_NAV_ITEM`。
- Produces: `menuTreeToNavEntries(tree): SidebarNavEntry[]`，按根节点顺序返回 group 或 direct link。
- Produces: `NavLeafItem.activeHrefs?: string[]`，用于 Kubernetes 聚合页面高亮。

- [ ] **Step 1: 写动态菜单转换失败测试**

增加根级知识库与 Kubernetes alias 测试：

```ts
it("keeps root leaf menus as direct entries", () => {
  const entries = menuTreeToNavEntries([
    node({ path: "/knowledge", title: "知识库", sort: 95 }),
  ]);
  expect(entries).toEqual([
    { type: "link", href: "/knowledge", label: "知识库" },
  ]);
});

it("maps kubernetes deep routes to consolidated entries", () => {
  const groups = menuTreeToNavGroups(kubernetesTree);
  expect(groups[0].items.find((item) => item.href.endsWith("/networks"))?.activeHrefs)
    .toContain("/plugins/kubernetes/gateway");
});
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd fronted
pnpm vitest run apps/web/src/components/layout/menu-tree-nav.test.ts
```

Expected: FAIL，当前转换器丢弃根级叶子且无 alias。

- [ ] **Step 3: 实现静态与动态结构**

`BUSINESS_NAV_TREE` 改为：

```ts
[
  { id: "application", label: "产品交付", items: [
    { href: "/publish/products", label: "产品", entitlement: "delivery" },
    { href: "/projects", label: "项目", entitlement: "agent" },
    { href: "/publish/release-plans", label: "发布计划", entitlement: "delivery" },
    { href: "/products/workflows", label: "工作流", entitlement: "delivery" },
    { href: "/cronjobs", label: "定时构建", entitlement: "k8s" },
    { href: "/templates", label: "模板中心", entitlement: "agent" },
  ]},
  { id: "ops-runtime", label: "运行观测", items: [
    { href: "/runtime/pods", label: "运行实例", entitlement: "k8s" },
    { href: "/monitor/overview", label: "监控概览", entitlement: "observability" },
    { href: "/monitor/log-viewer", label: "日志查询", entitlement: "observability" },
  ]},
  { id: "ops-infra", label: "配置与集成", defaultCollapsed: true, items: [
    { href: "/config/secrets", label: "密钥管理", entitlement: "delivery" },
    { href: "/config/webhooks", label: "外部集成", entitlement: "mcp" },
    { href: "/config/mcp", label: "MCP 集成", entitlement: "mcp" },
  ]},
]
```

将知识库作为 entitlement 过滤后的直接项渲染。`PlatformNav` 按 `SidebarNavEntry` 顺序渲染动态菜单；fallback 在运行观测后、配置与集成前插入知识库。`NavGroup` 初始化规则为：产品交付默认展开、配置与集成默认折叠、active 分组自动展开。

- [ ] **Step 4: 运行前端导航测试与类型检查**

```powershell
cd fronted
pnpm vitest run apps/web/src/components/layout/menu-tree-nav.test.ts
pnpm --filter @sparkops/web typecheck
```

Expected: PASS。

---

### Task 3: Kubernetes 八入口与页面内分类导航

**Files:**
- Modify: `backend/internal/plugins/kubernetes/plugin.yaml`
- Modify: `backend/internal/modules/plugin/infrastructure/acl/bundled_sync_test.go`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/KubernetesSectionNav.tsx`
- Modify: `fronted/apps/web/app/(platform)/plugins/kubernetes/layout.tsx`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/kubernetes-section-nav.test.ts`

**Interfaces:**
- Produces: `KUBERNETES_NAV_SECTIONS`，定义聚合入口、深层 routes 与页面内链接。
- Preserves: 所有原 `/plugins/kubernetes/*` 页面。

- [ ] **Step 1: 写 manifest 同步与分类映射失败测试**

后端测试断言同步后仅有根菜单加八个直接子菜单；前端测试断言：

```ts
expect(sectionForPath("/plugins/kubernetes/gateway")?.href)
  .toBe("/plugins/kubernetes/networks");
expect(sectionForPath("/plugins/kubernetes/storages")?.href)
  .toBe("/plugins/kubernetes/configs");
expect(sectionForPath("/plugins/kubernetes/records")?.href)
  .toBe("/plugins/kubernetes/access-controls");
expect(sectionForPath("/plugins/kubernetes/cloudtty")?.href)
  .toBe("/plugins/kubernetes/diagnosis");
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd backend
go test ./internal/modules/plugin/infrastructure/acl -run TestSyncBundledPlugins -count=1
cd ..\fronted
pnpm vitest run apps/web/src/modules/plugins/kubernetes/kubernetes-section-nav.test.ts
```

Expected: FAIL，manifest 仍平铺全部功能且分类函数不存在。

- [ ] **Step 3: 收敛 manifest 并增加页面内导航**

manifest 保留八个子菜单：

```yaml
- title: 集群管理
  path: /plugins/kubernetes/clusters
- title: 工作负载
  path: /plugins/kubernetes/workloads
- title: 节点管理
  path: /plugins/kubernetes/nodes
- title: 命名空间
  path: /plugins/kubernetes/namespaces
- title: 网络与网关
  path: /plugins/kubernetes/networks
- title: 配置与存储
  path: /plugins/kubernetes/configs
- title: 安全与访问
  path: /plugins/kubernetes/access-controls
- title: 运维工具
  path: /plugins/kubernetes/diagnosis
```

每项保留原 `component` 与 root parent。`KubernetesSectionNav` 在聚合分类中显示原深层页面链接；layout 根据 pathname 渲染当前分类的紧凑页签。侧栏 alias 让深层 route 保持聚合入口高亮。

- [ ] **Step 4: 运行插件与前端测试**

```powershell
cd backend
go test ./internal/modules/plugin/infrastructure/acl ./internal/modules/plugin/domain/manifest -count=1
cd ..\fronted
pnpm vitest run apps/web/src/modules/plugins/kubernetes/kubernetes-section-nav.test.ts apps/web/src/components/layout/menu-tree-nav.test.ts
pnpm --filter @sparkops/web typecheck
```

Expected: PASS。

---

### Task 4: 搜索、最近访问与路由标签同步

**Files:**
- Modify: `fronted/apps/web/src/components/layout/nav-search.ts`
- Modify: `fronted/apps/web/src/components/layout/route-labels.ts`
- Modify: `fronted/apps/web/src/components/layout/command-search-dynamic.ts`
- Create: `fronted/apps/web/src/components/layout/route-labels.test.ts`

**Interfaces:**
- Produces: 新分组名称与 Kubernetes 深层路由标签。

- [ ] **Step 1: 写标签失败测试**

```ts
it("labels consolidated kubernetes routes", () => {
  expect(resolveRouteVisitMeta("/plugins/kubernetes/gateway"))
    .toEqual({ label: "Gateway", group: "Kubernetes" });
  expect(resolveRouteVisitMeta("/cronjobs"))
    .toEqual({ label: "定时构建", group: "产品交付" });
});
```

- [ ] **Step 2: 运行测试确认失败**

```powershell
cd fronted
pnpm vitest run apps/web/src/components/layout/route-labels.test.ts
```

Expected: FAIL，仍返回旧分组或通用“页面”。

- [ ] **Step 3: 同步搜索与标签**

让静态搜索显式加入知识库；更新硬编码动态搜索分组；为所有 Kubernetes 深层路径增加 label，并将 group 统一为 `Kubernetes`。

- [ ] **Step 4: 运行布局测试**

```powershell
cd fronted
pnpm vitest run apps/web/src/components/layout
pnpm --filter @sparkops/web typecheck
```

Expected: PASS。

---

### Task 5: 全量验证与运行时迁移

**Files:**
- Verify only: all files changed by Tasks 1–4

- [ ] **Step 1: 格式化**

```powershell
cd backend
gofmt -w internal/modules/governance/application/command/menu_commands.go internal/modules/governance/application/command/menu_commands_test.go internal/modules/governance/application/command/menu_policy.go internal/modules/plugin/infrastructure/acl/bundled_sync_test.go
```

- [ ] **Step 2: 运行后端相关测试**

```powershell
cd backend
go test ./internal/modules/governance/... ./internal/modules/plugin/... -count=1
```

Expected: PASS。

- [ ] **Step 3: 运行前端相关测试与类型检查**

```powershell
cd fronted
pnpm vitest run apps/web/src/components/layout apps/web/src/modules/plugins/kubernetes
pnpm --filter @sparkops/web typecheck
```

Expected: PASS。

- [ ] **Step 4: 重启后端触发菜单 repair 与 bundled sync**

```powershell
cd A:\21-Agent\SparkOpsAgent
.\scripts\dev-restart.ps1 -Backend
```

Expected: 后端启动成功，`http://localhost:8080` 可访问。

- [ ] **Step 5: 检查改动诊断与工作区状态**

确认编辑文件无新增 lint/type 错误，且不覆盖工作区内与本任务无关的改动。
