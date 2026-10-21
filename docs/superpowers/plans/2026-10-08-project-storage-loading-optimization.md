# Project Storage Loading Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将项目持久化存储页从两个独立请求改为单个聚合请求，并通过 stale-while-revalidate 缓存消除重复请求和返回页签时的空白加载。

**Architecture:** 后端运行时模块新增 `storage-overview` 聚合查询，一次授权、一次打开项目运行时会话，并行获取 PVC 和 StorageClass；PVC 失败为致命错误，StorageClass 失败降级为 warning。前端新增对应契约、BFF 链路和轻量缓存模块，`ProjectStorageTab` 优先显示缓存并后台刷新，所有变更操作成功后使缓存失效并拉取权威数据。

**Tech Stack:** Go、Gin、client-go、TypeScript、Next.js App Router、React 19、Zod、Vitest

## Global Constraints

- 不引入新的前端数据请求库。
- 保留现有 PVC 与 StorageClass API，兼容现有调用方。
- 缓存 TTL 固定为 30 秒。
- PVC 查询失败时聚合接口失败；StorageClass 查询失败时返回 PVC、空 classes 和 warnings。
- 只有无缓存的首次加载使用整页加载态；有缓存刷新时内容保持可见。

---

### Task 1: 后端 Storage Overview 聚合查询

**Files:**
- Modify: `backend/internal/modules/runtime/infrastructure/k8s/project_runtime_pvcs.go`
- Modify: `backend/internal/modules/runtime/infrastructure/k8s/project_runtime_pvcs_test.go`
- Modify: `backend/internal/modules/runtime/application/command/runtime_config.go`
- Modify: `backend/internal/modules/runtime/interface/http/handler.go`
- Modify: `backend/internal/modules/runtime/module.go`

**Interfaces:**
- Produces: `StorageOverviewVM{Items []ProjectPVCVM, Classes []StorageClassVM, Warnings []string}`
- Produces: `(*ProjectRuntimeConfigurator).GetStorageOverview(ctx, wsID, projectID) (StorageOverviewVM, error)`
- Produces: `(*RuntimeConfigHandler).GetStorageOverview(ctx, wsID, projectID) (StorageOverviewVM, error)`
- Produces: `GET /api/v1/projects/:projectId/runtime/storage-overview`

- [ ] **Step 1: Write failing configurator tests**

在 `project_runtime_pvcs_test.go` 增加测试，直接测试接收 `ProjectClusterSession` 的私有聚合函数，避免构造数据库和密钥：

```go
func TestGetStorageOverviewReturnsPVCsAndStorageClasses(t *testing.T) {
    client := fake.NewSimpleClientset(
        &corev1.PersistentVolumeClaim{
            ObjectMeta: metav1.ObjectMeta{
                Name: "orders-data", Namespace: "team-a",
                Labels: map[string]string{"app": "orders"},
            },
        },
        &storagev1.StorageClass{
            ObjectMeta: metav1.ObjectMeta{Name: "fast"},
            Provisioner: "example.com/fast",
        },
    )
    session := &ProjectClusterSession{
        Client: client, Namespace: "team-a", ProjectName: "orders",
    }
    overview, err := getStorageOverviewWithSession(context.Background(), session)
    require.NoError(t, err)
    require.Len(t, overview.Items, 1)
    require.Len(t, overview.Classes, 1)
    require.Empty(t, overview.Warnings)
}

func TestGetStorageOverviewKeepsPVCsWhenStorageClassesFail(t *testing.T) {
    client := fake.NewSimpleClientset(&corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name: "orders-data", Namespace: "team-a",
            Labels: map[string]string{"app": "orders"},
        },
    })
    client.PrependReactor(
        "list", "storageclasses",
        func(action k8stesting.Action) (bool, runtime.Object, error) {
            return true, nil, errors.New("storage API unavailable")
        },
    )
    session := &ProjectClusterSession{
        Client: client, Namespace: "team-a", ProjectName: "orders",
    }
    overview, err := getStorageOverviewWithSession(context.Background(), session)
    require.NoError(t, err)
    require.Len(t, overview.Items, 1)
    require.Empty(t, overview.Classes)
    require.Equal(t, []string{"StorageClass 加载失败：storage API unavailable"}, overview.Warnings)
}
```

测试文件新增 `context`、`errors`、`storagev1`、`runtime`、`k8stesting`、`fake` 与 `testify/require` imports。

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd backend
go test ./internal/modules/runtime/infrastructure/k8s -run TestGetStorageOverview -count=1
```

Expected: FAIL because `GetStorageOverview` and `StorageOverviewVM` do not exist.

- [ ] **Step 3: Implement one-session parallel aggregation**

在 `project_runtime_pvcs.go` 中提取接收 `*ProjectClusterSession` 的私有查询函数，保留原公开方法行为：

```go
type StorageOverviewVM struct {
    Items    []ProjectPVCVM  `json:"items"`
    Classes  []StorageClassVM `json:"classes"`
    Warnings []string        `json:"warnings"`
}

func (c *ProjectRuntimeConfigurator) GetStorageOverview(
    ctx context.Context,
    wsID kernel.WorkspaceID,
    projectID kernel.ProjectID,
) (StorageOverviewVM, error) {
    session, err := c.access.Open(ctx, wsID, projectID)
    if err != nil {
        return StorageOverviewVM{}, err
    }
    pvcCh := make(chan struct {
        items []ProjectPVCVM
        err   error
    }, 1)
    classCh := make(chan struct {
        items []StorageClassVM
        err   error
    }, 1)
    go func() {
        items, err := listPVCsWithSession(ctx, session)
        pvcCh <- struct {
            items []ProjectPVCVM
            err   error
        }{items, err}
    }()
    go func() {
        items, err := listStorageClassesWithSession(ctx, session)
        classCh <- struct {
            items []StorageClassVM
            err   error
        }{items, err}
    }()
    pvcResult, classResult := <-pvcCh, <-classCh
    if pvcResult.err != nil {
        return StorageOverviewVM{}, pvcResult.err
    }
    overview := StorageOverviewVM{Items: pvcResult.items, Classes: classResult.items, Warnings: []string{}}
    if classResult.err != nil {
        overview.Classes = []StorageClassVM{}
        overview.Warnings = append(overview.Warnings, "StorageClass 加载失败："+classResult.err.Error())
    }
    return overview, nil
}
```

`ListPVCs`、`ListStorageClasses` 改为打开 session 后调用同一私有函数，避免逻辑复制。

- [ ] **Step 4: Add application and HTTP wiring**

在 `RuntimeConfigHandler` 增加：

```go
func (h *RuntimeConfigHandler) GetStorageOverview(
    ctx context.Context,
    wsID kernel.WorkspaceID,
    projectID kernel.ProjectID,
) (runtimek8s.StorageOverviewVM, error) {
    if err := h.authorize(ctx, wsID, projectID); err != nil {
        return runtimek8s.StorageOverviewVM{}, err
    }
    return h.configurator.GetStorageOverview(ctx, wsID, projectID)
}
```

在 HTTP handler 增加参数解析、nil 检查与 `httpx.OK(c, overview)`；在 `module.go` 注册：

```go
c.Engine.GET("/api/v1/projects/:projectId/runtime/storage-overview",
    auth, ws, wsMember,
    authz.RequirePermission(c.PermissionChecker, authz.PermProjectRead),
    handler.GetStorageOverview)
```

- [ ] **Step 5: Verify backend**

Run:

```powershell
cd backend
gofmt -w internal/modules/runtime/infrastructure/k8s/project_runtime_pvcs.go `
  internal/modules/runtime/infrastructure/k8s/project_runtime_pvcs_test.go `
  internal/modules/runtime/application/command/runtime_config.go `
  internal/modules/runtime/interface/http/handler.go `
  internal/modules/runtime/module.go
go test ./internal/modules/runtime/... -count=1
go build ./...
```

Expected: all runtime tests PASS and build exits 0.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/modules/runtime
git -c alias.ci=commit ci -m "feat(runtime): add project storage overview"
```

---

### Task 2: 前端契约与单请求 BFF 链路

**Files:**
- Modify: `fronted/packages/contracts/src/cluster/index.ts`
- Modify: `fronted/packages/upstream/src/cluster.ts`
- Modify: `fronted/packages/bff-modules/src/cluster/index.ts`
- Modify: `fronted/packages/web-sdk/src/cluster.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/projects/[projectId]/runtime/storage-overview/route.ts`

**Interfaces:**
- Consumes: backend `GET /api/v1/projects/:projectId/runtime/storage-overview`
- Produces: `ProjectStorageOverviewVM`
- Produces: `cluster.getStorageOverview(projectId, workspaceId)`

- [ ] **Step 1: Add contract schema**

在 cluster contracts 的 PVC 类型后增加：

```ts
export const ProjectStorageOverviewVMSchema = z.object({
  items: z.array(ProjectPVCVMSchema),
  classes: z.array(StorageClassVMSchema),
  warnings: z.array(z.string()),
});

export type ProjectStorageOverviewVM = z.infer<typeof ProjectStorageOverviewVMSchema>;
```

- [ ] **Step 2: Add upstream and BFF module methods**

Upstream：

```ts
async getStorageOverview(workspaceId: string, projectId: string) {
  return upstreamFetch<ProjectStorageOverviewVM>(
    `/api/v1/projects/${projectId}/runtime/storage-overview`,
    { method: "GET", options: { ...options, workspaceId } },
  );
}
```

BFF module：

```ts
export async function getStorageOverview(ctx: BffContext, workspaceId: string, projectId: string) {
  return ctx.upstream.cluster.getStorageOverview(workspaceId, projectId);
}
```

- [ ] **Step 3: Add BFF route and SDK method**

Route：

```ts
import { createBffHandler } from "@sparkops/bff-core";
import { getStorageOverview } from "@sparkops/bff-modules/cluster";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) throw new Error("project id and workspace required");
  return getStorageOverview(ctx, wsId, projectId);
});
```

SDK：

```ts
export function getStorageOverview(projectId: string, workspaceId: string) {
  return bffFetch<ProjectStorageOverviewVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/storage-overview`,
    { workspaceId },
  );
}
```

- [ ] **Step 4: Typecheck changed packages**

Run:

```powershell
cd fronted
pnpm --filter @sparkops/contracts typecheck
pnpm --filter @sparkops/upstream typecheck
pnpm --filter @sparkops/bff-modules typecheck
pnpm --filter @sparkops/web-sdk typecheck
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit**

```powershell
git add fronted/packages fronted/apps/web/app/api/bff/v1/workspaces
git -c alias.ci=commit ci -m "feat(web): expose project storage overview"
```

---

### Task 3: Storage Overview SWR 缓存与请求合并

**Files:**
- Create: `fronted/apps/web/src/modules/workspace/components/project/project-storage-data.ts`
- Create: `fronted/apps/web/src/modules/workspace/components/project/project-storage-data.test.ts`

**Interfaces:**
- Consumes: `cluster.getStorageOverview(projectId, workspaceId)`
- Produces: `readProjectStorageCache(key)`
- Produces: `loadProjectStorage(key, fetcher, { force? })`
- Produces: `invalidateProjectStorage(key)`
- Produces: `projectStorageKey(workspaceId, projectId)`

- [ ] **Step 1: Write failing cache tests**

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearProjectStorageCache,
  invalidateProjectStorage,
  loadProjectStorage,
  projectStorageKey,
  readProjectStorageCache,
} from "./project-storage-data";

describe("project storage data", () => {
  beforeEach(clearProjectStorageCache);

  it("deduplicates concurrent requests", async () => {
    const fetcher = vi.fn().mockResolvedValue({ items: [], classes: [], warnings: [] });
    const key = projectStorageKey("ws", "project");
    const [a, b] = await Promise.all([
      loadProjectStorage(key, fetcher),
      loadProjectStorage(key, fetcher),
    ]);
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(a).toEqual(b);
  });

  it("returns fresh cache without refetching", async () => {
    const fetcher = vi.fn().mockResolvedValue({ items: [], classes: [], warnings: [] });
    const key = projectStorageKey("ws", "project");
    await loadProjectStorage(key, fetcher);
    await loadProjectStorage(key, fetcher);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("force refreshes and invalidation removes freshness", async () => {
    const fetcher = vi.fn().mockResolvedValue({ items: [], classes: [], warnings: [] });
    const key = projectStorageKey("ws", "project");
    await loadProjectStorage(key, fetcher);
    await loadProjectStorage(key, fetcher, { force: true });
    invalidateProjectStorage(key);
    expect(readProjectStorageCache(key)).toBeNull();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
cd fronted/apps/web
pnpm exec vitest run src/modules/workspace/components/project/project-storage-data.test.ts
```

Expected: FAIL because the module does not exist.

- [ ] **Step 3: Implement the cache**

```ts
import type { ProjectStorageOverviewVM } from "@sparkops/contracts/cluster";

const TTL_MS = 30_000;
type Entry = { value: ProjectStorageOverviewVM; writtenAt: number };
const cache = new Map<string, Entry>();
const inflight = new Map<string, Promise<ProjectStorageOverviewVM>>();

export const projectStorageKey = (workspaceId: string, projectId: string) =>
  `${workspaceId}:${projectId}`;

export function readProjectStorageCache(key: string) {
  return cache.get(key)?.value ?? null;
}

export function invalidateProjectStorage(key: string) {
  cache.delete(key);
}

export function clearProjectStorageCache() {
  cache.clear();
  inflight.clear();
}

export function loadProjectStorage(
  key: string,
  fetcher: () => Promise<ProjectStorageOverviewVM>,
  options: { force?: boolean } = {},
) {
  const cached = cache.get(key);
  if (!options.force && cached && Date.now() - cached.writtenAt < TTL_MS) {
    return Promise.resolve(cached.value);
  }
  const pending = inflight.get(key);
  if (pending) return pending;
  const request = fetcher()
    .then((value) => {
      cache.set(key, { value, writtenAt: Date.now() });
      return value;
    })
    .finally(() => inflight.delete(key));
  inflight.set(key, request);
  return request;
}
```

- [ ] **Step 4: Run tests and verify GREEN**

Run:

```powershell
cd fronted/apps/web
pnpm exec vitest run src/modules/workspace/components/project/project-storage-data.test.ts
```

Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```powershell
git add fronted/apps/web/src/modules/workspace/components/project/project-storage-data*
git -c alias.ci=commit ci -m "feat(web): cache project storage overview"
```

---

### Task 4: 将 ProjectStorageTab 接入聚合数据与静默刷新

**Files:**
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectStorageTab.tsx`
- Modify: `fronted/apps/web/app/globals.css` only if no existing compact refresh style can be reused

**Interfaces:**
- Consumes: Task 2 `cluster.getStorageOverview`
- Consumes: Task 3 cache helpers

- [ ] **Step 1: Replace dual requests with cached aggregate load**

初始化时同步读取缓存：

```ts
const cacheKey = useMemo(
  () => projectStorageKey(workspaceId, project.id),
  [workspaceId, project.id],
);
const cached = readProjectStorageCache(cacheKey);
const [items, setItems] = useState<ProjectPVCVM[]>(cached?.items ?? []);
const [classes, setClasses] = useState<StorageClassVM[]>(cached?.classes ?? []);
const [warnings, setWarnings] = useState<string[]>(cached?.warnings ?? []);
const [loading, setLoading] = useState(!cached);
const [refreshing, setRefreshing] = useState(false);
```

`load` 接收 `{ force?: boolean }`，有内容时只设置 `refreshing`：

```ts
const load = useCallback(async ({ force = false } = {}) => {
  const hasVisibleData = readProjectStorageCache(cacheKey) !== null;
  hasVisibleData ? setRefreshing(true) : setLoading(true);
  setError(null);
  try {
    const next = await loadProjectStorage(
      cacheKey,
      () => cluster.getStorageOverview(project.id, workspaceId),
      { force },
    );
    setItems(next.items);
    setClasses(next.classes);
    setWarnings(next.warnings);
    setSelectedName((current) =>
      current && next.items.some((item) => item.name === current)
        ? current
        : next.items[0]?.name ?? null,
    );
  } catch (err) {
    setError(formatStorageError(err, "持久化存储加载失败"));
  } finally {
    setLoading(false);
    setRefreshing(false);
  }
}, [cacheKey, project.id, workspaceId]);
```

Effect 中使用普通加载；手动刷新使用 `load({ force: true })`。

- [ ] **Step 2: Invalidate after mutations**

在创建、挂载、卸载、删除成功后，将原 `await load()` 替换为：

```ts
invalidateProjectStorage(cacheKey);
await load({ force: true });
```

操作期间保留 `items`，不要清空列表。

- [ ] **Step 3: Update loading and warning UI**

- `loading && items.length === 0` 时显示 `K8sLoadingState`。
- `refreshing` 时刷新按钮文案改为「刷新中…」并禁用。
- `warnings.length > 0` 时在工具栏下显示非阻塞提示。
- `error && items.length > 0` 时提示“刷新失败，当前为缓存数据”，保留内容。
- 空态条件必须包含 `!loading && !error && items.length === 0`。

- [ ] **Step 4: Run focused and project tests**

Run:

```powershell
cd fronted/apps/web
pnpm exec vitest run src/modules/workspace/components/project/project-storage-data.test.ts
pnpm exec vitest run src/modules/workspace/components/project
```

Expected: cache tests and existing project tests PASS.

- [ ] **Step 5: Run typecheck and inspect only changed-file diagnostics**

Run:

```powershell
cd fronted
pnpm --filter @sparkops/web typecheck
```

If the repository-wide pre-existing errors remain, verify that output contains no error for `ProjectStorageTab.tsx` or `project-storage-data.ts`, and run IDE diagnostics for changed files.

- [ ] **Step 6: Commit**

```powershell
git add fronted/apps/web/src/modules/workspace/components/project/ProjectStorageTab.tsx `
  fronted/apps/web/src/modules/workspace/components/project/project-storage-data.ts `
  fronted/apps/web/src/modules/workspace/components/project/project-storage-data.test.ts `
  fronted/apps/web/app/globals.css
git -c alias.ci=commit ci -m "perf(web): optimize project storage loading"
```

---

### Task 5: 集成验证与服务重启

**Files:**
- No new production files expected.

- [ ] **Step 1: Run final backend verification**

```powershell
cd backend
go test ./internal/modules/runtime/... -count=1
go build ./...
```

Expected: PASS and exit 0.

- [ ] **Step 2: Run final frontend verification**

```powershell
cd fronted
pnpm --filter @sparkops/contracts typecheck
pnpm --filter @sparkops/upstream typecheck
pnpm --filter @sparkops/bff-modules typecheck
pnpm --filter @sparkops/web-sdk typecheck
pnpm --filter @sparkops/web exec vitest run src/modules/workspace/components/project
```

Expected: package typechecks and project tests PASS.

- [ ] **Step 3: Restart affected services**

```powershell
cd A:\21-Agent\SparkOpsAgent
.\scripts\dev-restart.ps1 -Backend
.\scripts\dev-restart.ps1 -Frontend
```

Expected: ports 8080 and 3000 are listening with new PIDs.

- [ ] **Step 4: Verify the live aggregate endpoint**

With a valid local session, request:

```text
GET /api/bff/v1/workspaces/{wsId}/projects/{projectId}/runtime/storage-overview
```

Expected: HTTP 200 with `items`, `classes`, and `warnings`.

- [ ] **Step 5: Verify request count and UX**

In browser Network:

1. Enter the storage tab: exactly one `storage-overview` request.
2. Leave and return within 30 seconds: cached content appears immediately; no full-page loading.
3. Trigger simultaneous loads in development Strict Mode: one upstream request due to inflight deduplication.
4. Click refresh: one forced request while existing content remains visible.
5. Create/bind/unbind/delete: mutation succeeds, cache invalidates, one overview request refreshes authoritative state.

- [ ] **Step 6: Final commit if verification required fixes**

Stage only files changed for verification fixes and commit:

```powershell
git -c alias.ci=commit ci -m "test: verify project storage loading optimization"
```
