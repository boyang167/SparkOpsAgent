# Filesystem Unified Workbench UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有 Filesystem 页面升级为包含文件、传输、存储治理和审计的统一工作台，并支持目录树拖拽、桌面文件拖入以及网格/列表双视图。

**Architecture:** `/files` 作为唯一工作台入口，URL 保存 tab、目录、视图和分页状态。`FilesystemWorkbench` 只编排子组件；目录树、文件浏览器、传输中心、治理和审计各自拥有独立边界。后端仅补充传输列表和目录筛选能力，现有命令 API 继续作为写操作与授权边界。

**Tech Stack:** Next.js App Router、React、TypeScript、Zod、现有 SparkOps UI Token/Modal/Drawer、Vitest、Go、Gin、GORM。

## Global Constraints

- 不引入新的 UI 组件库或拖拽依赖；使用原生 Pointer/Drag Events 和现有组件。
- 主色仅用于主按钮、链接、选中态和关键状态；不使用大面积渐变。
- 每屏最多一个 `btn-primary`，工作台唯一主操作为“上传文件”。
- 加载、空、错误、无权限四态必须分离。
- 拖拽必须提供键盘替代方案“移动到…”。
- 前端权限裁剪不能替代后端授权。
- 首期不实现实时容量曲线、SSE 容量推送或本地目录递归上传。
- 所有新增写操作使用 Idempotency-Key。

---

## File Structure

### Frontend

- `fronted/apps/web/app/(platform)/files/page.tsx`：加载工作台首屏数据。
- `fronted/apps/web/app/(platform)/files/[nodeId]/page.tsx`：保留版本深链接。
- `fronted/apps/web/app/(platform)/files/admin/page.tsx`：兼容重定向到工作台治理标签。
- `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`：工作台编排。
- `fronted/apps/web/src/modules/filesystem/components/workbench-state.ts`：URL 状态解析。
- `fronted/apps/web/src/modules/filesystem/components/DirectoryTree.tsx`：懒加载目录树和投放目标。
- `fronted/apps/web/src/modules/filesystem/components/directory-dnd.ts`：站内拖拽纯规则。
- `fronted/apps/web/src/modules/filesystem/components/FileBrowser.tsx`：选择、排序、命令编排。
- `fronted/apps/web/src/modules/filesystem/components/FileListView.tsx`：列表视图。
- `fronted/apps/web/src/modules/filesystem/components/FileGridView.tsx`：网格视图。
- `fronted/apps/web/src/modules/filesystem/components/FileDropZone.tsx`：桌面拖入反馈。
- `fronted/apps/web/src/modules/filesystem/components/TransferCenter.tsx`：多文件上传队列。
- `fronted/apps/web/src/modules/filesystem/components/transfer-queue.ts`：上传队列状态机。
- `fronted/apps/web/src/modules/filesystem/components/FileDetailDrawer.tsx`：详情与版本。
- `fronted/apps/web/src/modules/filesystem/components/GovernanceWorkspace.tsx`：治理二级导航。
- `fronted/apps/web/src/modules/filesystem/components/AuditWorkspace.tsx`：审计筛选与详情。
- `fronted/apps/web/app/globals.css`：工作台布局与交互 Token 类。
- `fronted/packages/contracts/src/filesystem/index.ts`：传输列表契约。
- `fronted/packages/web-sdk/src/filesystem.ts`：传输列表、移动、复制、删除调用。
- `fronted/packages/upstream/src/filesystem.ts`：上游 API 调用。
- `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/transfers/route.ts`：GET 传输列表 BFF。

### Backend

- `backend/internal/modules/filesystem/application/query/list_transfers.go`：workspace 传输查询。
- `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`：传输列表仓储接口。
- `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`：传输列表持久化。
- `backend/internal/modules/filesystem/interface/http/handler.go`：绑定列表处理器和路由。
- `backend/internal/modules/filesystem/interface/http/transfer.go`：GET transfers 响应。
- `backend/internal/modules/filesystem/module.go`：装配查询处理器。
- `backend/api/openapi/filesystem.yaml`：传输列表 OpenAPI。

---

### Task 1: 工作台 URL 状态与页面骨架

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/workbench-state.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/workbench-state.test.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx`
- Modify: `fronted/apps/web/app/(platform)/files/page.tsx`
- Modify: `fronted/apps/web/app/(platform)/files/admin/page.tsx`

**Interfaces:**
- Produces: `parseWorkbenchState(searchParams): WorkbenchState`
- Produces: `FilesystemWorkbenchProps`
- Consumes: 当前 namespace、root children、workspace ID。

- [ ] **Step 1: 写 URL 状态失败测试**

```ts
import { describe, expect, it } from "vitest";
import { parseWorkbenchState } from "./workbench-state";

describe("parseWorkbenchState", () => {
  it("normalizes unsupported values", () => {
    expect(parseWorkbenchState({ tab: "bad", view: "bad", cursor: "c1" })).toEqual({
      tab: "files",
      view: "list",
      parent: undefined,
      cursor: "c1",
      query: "",
    });
  });
});
```

- [ ] **Step 2: 运行并确认失败**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/workbench-state.test.ts
```

Expected: FAIL，提示找不到 `workbench-state`。

- [ ] **Step 3: 实现 URL 状态**

```ts
export type WorkbenchTab = "files" | "transfers" | "governance" | "audit";
export type FileView = "list" | "grid";

export type WorkbenchState = {
  tab: WorkbenchTab;
  view: FileView;
  parent?: string;
  cursor?: string;
  query: string;
};

export function parseWorkbenchState(input: Record<string, string | undefined>): WorkbenchState {
  const tabs: WorkbenchTab[] = ["files", "transfers", "governance", "audit"];
  return {
    tab: tabs.includes(input.tab as WorkbenchTab) ? (input.tab as WorkbenchTab) : "files",
    view: input.view === "grid" ? "grid" : "list",
    parent: input.parent || undefined,
    cursor: input.cursor || undefined,
    query: input.q?.trim() ?? "",
  };
}
```

- [ ] **Step 4: 写工作台标签组件测试**

```tsx
render(
  <FilesystemWorkbench
    workspaceId="00000000-0000-4000-8000-000000000001"
    namespace={namespace}
    initialNodes={[]}
    initialState={{ tab: "files", view: "list", query: "" }}
    permissions={["file:read"]}
  />,
);
expect(screen.getByRole("tab", { name: "文件" })).toHaveAttribute("aria-selected", "true");
expect(screen.queryByRole("tab", { name: "存储治理" })).not.toBeInTheDocument();
```

- [ ] **Step 5: 实现最小工作台骨架**

`FilesystemWorkbench.tsx` 定义四个 tab；`governance` 仅对 `storage:read` 可见，`audit` 仅对 `file:admin` 可见。使用 `router.replace()` 更新 query string，不重置未修改的状态。

- [ ] **Step 6: 修改页面装配**

`page.tsx` 接收 `searchParams`，调用 `parseWorkbenchState`，加载当前 parent 的节点并传入 `FilesystemWorkbench`。`admin/page.tsx` 改为：

```tsx
import { redirect } from "next/navigation";

export default function FilesystemAdminPage() {
  redirect("/files?tab=governance");
}
```

- [ ] **Step 7: 运行测试**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/workbench-state.test.ts apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx
pnpm --filter web typecheck
```

Expected: PASS。

- [ ] **Step 8: 提交**

```powershell
git add fronted/apps/web/app fronted/apps/web/src/modules/filesystem/components
git commit -m "feat(filesystem-ui): add unified workbench shell"
```

---

### Task 2: 文件浏览器拆分与网格/列表双视图

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/FileBrowser.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileListView.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileGridView.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileBrowser.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileExplorerPanel.tsx`

**Interfaces:**
- Produces: `FileBrowserProps { nodes, view, selectedIds, onSelectionChange, onOpen, onRename, onMove }`
- Consumes: `FileNodeVM[]`。

- [ ] **Step 1: 写双视图共享状态测试**

```tsx
const { rerender } = render(
  <FileBrowser {...props} view="list" />,
);
await user.click(screen.getByRole("checkbox", { name: "选择 规格说明.md" }));
rerender(<FileBrowser {...props} view="grid" />);
expect(screen.getByRole("checkbox", { name: "选择 规格说明.md" })).toBeChecked();
```

- [ ] **Step 2: 运行并确认失败**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/FileBrowser.test.tsx
```

Expected: FAIL，组件不存在。

- [ ] **Step 3: 实现共享浏览器与两个纯视图**

`FileBrowser` 持有 selection，排序规则固定为“目录优先、中文名称升序”。`FileListView` 和 `FileGridView` 只渲染传入数据和回调，不各自维护选择状态。

```ts
export function sortFileNodes(nodes: FileNodeVM[]) {
  return [...nodes].sort((a, b) => {
    if (a.type !== b.type) return a.type === "DIRECTORY" ? -1 : 1;
    return a.name.localeCompare(b.name, "zh-CN");
  });
}
```

- [ ] **Step 4: 接入视图切换**

工作台工具栏添加两个带 `aria-pressed` 的图标按钮。切换仅修改 URL 中的 `view`，不重新请求当前节点集合。

- [ ] **Step 5: 将旧 Explorer 降级为兼容包装**

`FileExplorerPanel` 暂时调用 `FileBrowser`，保留现有测试和导出；Task 7 把新建、重命名 Modal 移入工作台。

- [ ] **Step 6: 运行测试**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/FileBrowser.test.tsx apps/web/src/modules/filesystem/components/FileExplorerPanel.test.ts
pnpm --filter web typecheck
```

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add fronted/apps/web/src/modules/filesystem/components
git commit -m "feat(filesystem-ui): add list and grid file views"
```

---

### Task 3: 懒加载目录树与键盘导航

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/DirectoryTree.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/DirectoryTree.test.tsx`
- Modify: `fronted/packages/web-sdk/src/filesystem.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`
- Modify: `backend/internal/modules/filesystem/application/query/queries.go`
- Modify: `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Modify: `backend/internal/modules/filesystem/interface/http/handler.go`

**Interfaces:**
- Produces: `DirectoryTreeProps { workspaceId, root, currentId, onNavigate, onDropNodes }`
- Consumes: `listFileNodes(workspaceId, { parentId, nodeType: "DIRECTORY" })`。

- [ ] **Step 1: 扩展 SDK 查询测试**

```ts
expect(fetchMock).toHaveBeenCalledWith(
  expect.stringContaining("parent_id=parent-1&type=DIRECTORY"),
  expect.anything(),
);
```

- [ ] **Step 2: 扩展 `ListFilesQuery`**

```ts
type ListFilesQuery = {
  parentId?: string | null;
  cursor?: string;
  limit?: number;
  nodeType?: "FILE" | "DIRECTORY";
};
```

`listFilesQuery` 将 `nodeType` 编码为 `type`。

- [ ] **Step 3: 写目录树懒加载测试**

```tsx
render(<DirectoryTree {...props} />);
await user.click(screen.getByRole("button", { name: "展开 项目资料" }));
await waitFor(() =>
  expect(listFileNodes).toHaveBeenCalledWith(workspaceId, {
    parentId: "directory-1",
    nodeType: "DIRECTORY",
  }),
);
```

- [ ] **Step 4: 后端实现目录类型筛选**

把 `ListChildren`、仓储接口和 HTTP handler 的调用签名扩展为可选 `*model.NodeType`：

```go
type ListChildren struct {
    NamespaceID uuid.UUID
    ParentID    *uuid.UUID
    NodeType    *model.NodeType
    Limit       int
    Cursor      string
}
```

持久化查询在 `nodeType != nil` 时增加：

```go
query = query.Where("type = ?", *nodeType)
```

HTTP 只接受空值、`FILE`、`DIRECTORY`；其他值返回 400。授权过滤和 cursor 扫描继续使用过滤后的结果，避免树请求扫描无关文件。

- [ ] **Step 5: 运行后端筛选测试**

```powershell
cd backend
go test ./internal/modules/filesystem/application/query ./internal/modules/filesystem/infrastructure/persistence ./internal/modules/filesystem/interface/http -count=1
```

Expected: PASS；`type=DIRECTORY` 只返回目录。

- [ ] **Step 6: 实现树状态**

每个节点维护 `expanded/loading/childrenLoaded`。使用 `role="tree"`、`role="treeitem"`、`aria-expanded`；ArrowRight 展开，ArrowLeft 收起，Enter 导航。

- [ ] **Step 7: 添加响应式入口**

桌面显示固定侧栏；窄屏通过现有 `Drawer` 打开目录树。两种容器复用同一个 `DirectoryTree`。

- [ ] **Step 8: 运行前端测试**

```powershell
cd fronted
pnpm vitest run packages/web-sdk/src/filesystem.test.ts apps/web/src/modules/filesystem/components/DirectoryTree.test.tsx
pnpm --filter web typecheck
```

Expected: PASS。

- [ ] **Step 9: 提交**

```powershell
git add backend/internal/modules/filesystem fronted/packages/web-sdk/src/filesystem.ts fronted/apps/web/src/modules/filesystem/components
git commit -m "feat(filesystem-ui): add lazy directory tree"
```

---

### Task 4: 站内拖拽移动与键盘替代操作

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/directory-dnd.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/directory-dnd.test.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/MoveNodesModal.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/MoveNodesModal.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/DirectoryTree.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileBrowser.tsx`
- Modify: `fronted/packages/web-sdk/src/filesystem.ts`

**Interfaces:**
- Produces: `validateNodeDrop(input): DropDecision`
- Produces: `moveFileNode(workspaceId, nodeId, targetParentId)`
- Consumes: 现有后端 `POST /files/:nodeId/move`。

- [ ] **Step 1: 写拖拽规则测试**

```ts
expect(validateNodeDrop({
  draggedIds: ["dir-a"],
  targetId: "dir-a",
  descendantIds: new Set(),
  sameWorkspace: true,
  canWrite: true,
})).toEqual({ allowed: false, reason: "不能移动到自身" });

expect(validateNodeDrop({
  draggedIds: ["dir-a"],
  targetId: "dir-b",
  descendantIds: new Set(["dir-b"]),
  sameWorkspace: true,
  canWrite: true,
})).toEqual({ allowed: false, reason: "不能移动到子目录" });
```

- [ ] **Step 2: 实现纯校验函数**

```ts
export type DropDecision =
  | { allowed: true }
  | { allowed: false; reason: string };

export function validateNodeDrop(input: DropInput): DropDecision {
  if (!input.canWrite) return { allowed: false, reason: "没有移动权限" };
  if (!input.sameWorkspace) return { allowed: false, reason: "不能跨工作区移动" };
  if (input.draggedIds.includes(input.targetId)) return { allowed: false, reason: "不能移动到自身" };
  if (input.descendantIds.has(input.targetId)) return { allowed: false, reason: "不能移动到子目录" };
  return { allowed: true };
}
```

- [ ] **Step 3: 补齐 SDK 移动方法**

```ts
export function moveFileNode(
  workspaceId: string,
  nodeId: string,
  targetParentId: string,
  options: BffFetchOptions & { idempotencyKey?: string } = {},
) {
  return bffFetch<void>(`/workspaces/${workspaceId}/files/${nodeId}/move`, {
    method: "POST",
    workspaceId,
    body: JSON.stringify({ target_parent_id: targetParentId }),
    headers: {
      ...options.headers,
      "Idempotency-Key": options.idempotencyKey ?? mutationIdempotencyKey("move"),
    },
  });
}
```

- [ ] **Step 4: 实现拖拽目标与 700ms 展开**

`DirectoryTree` 在 `dragenter` 启动 700ms timer，`dragleave/drop/unmount` 清理。校验失败显示拒绝样式且不调用 API。

- [ ] **Step 5: 实现键盘“移动到…” Modal**

`FileActionMenu` 和多选工具栏都提供“移动到…”。Modal 内复用 DirectoryTree 的只选模式，确认后逐项调用 `moveFileNode`；任一失败则刷新源目录和目标目录，不保留错误乐观状态。

- [ ] **Step 6: 运行测试**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/directory-dnd.test.ts apps/web/src/modules/filesystem/components/MoveNodesModal.test.tsx
pnpm --filter web typecheck
```

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add fronted/apps/web/src/modules/filesystem/components fronted/packages/web-sdk/src/filesystem.ts
git commit -m "feat(filesystem-ui): add drag and keyboard node moves"
```

---

### Task 5: 多文件拖拽上传与统一传输队列

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/transfer-queue.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/transfer-queue.test.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileDropZone.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileDropZone.test.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/TransferCenter.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/TransferCenter.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/UploadPanel.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`

**Interfaces:**
- Produces: `createTransferQueue({ concurrency: 3, upload }): TransferQueue`
- Produces: `enqueue(files, targetParentId)`
- Consumes: `createTransferSession`、`uploadTransfer`、后端默认 Provider。

- [ ] **Step 1: 写并发队列测试**

```ts
it("runs at most three uploads", async () => {
  let active = 0;
  let peak = 0;
  const queue = createTransferQueue({
    concurrency: 3,
    upload: async () => {
      active += 1;
      peak = Math.max(peak, active);
      await deferred();
      active -= 1;
    },
  });
  queue.enqueue(files(6), "parent-1");
  await queue.whenIdle();
  expect(peak).toBe(3);
});
```

- [ ] **Step 2: 实现队列状态机**

状态固定为 `queued | creating | uploading | verifying | completed | failed | canceled`。任务保存 file、targetParentId、progress、session、error。`retry(id)` 只重置失败任务；`cancel(id)` 对有 session 的任务调用 `abortTransfer`。

- [ ] **Step 3: 写 DropZone 测试**

```tsx
fireEvent.dragEnter(zone, { dataTransfer: { types: ["Files"], files } });
expect(screen.getByText('释放以上传到“交付中心”')).toBeInTheDocument();
fireEvent.drop(zone, { dataTransfer: { files } });
expect(onFiles).toHaveBeenCalledWith(Array.from(files), "target-parent");
```

- [ ] **Step 4: 实现桌面拖入**

只接受 `dataTransfer.files`，忽略站内节点 MIME。目录项通过 `webkitGetAsEntry()?.isDirectory` 拒绝并显示“当前版本不支持目录递归上传”。

- [ ] **Step 5: 迁移 UploadPanel**

`UploadPanel` 不再自己执行上传，只调用工作台提供的 `onFilesSelected(files)`。不再因缺少 `providerId` 禁用按钮，后端默认 Provider 负责放置。

- [ ] **Step 6: 实现 TransferCenter**

传输标签展示队列项、进度、重试和取消。文件标签底部只显示最多三个活跃任务的紧凑摘要和“查看全部传输”链接。

- [ ] **Step 7: 运行测试**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/transfer-queue.test.ts apps/web/src/modules/filesystem/components/FileDropZone.test.tsx apps/web/src/modules/filesystem/components/TransferCenter.test.tsx apps/web/src/modules/filesystem/components/UploadPanel.test.ts
pnpm --filter web typecheck
```

Expected: PASS。

- [ ] **Step 8: 提交**

```powershell
git add fronted/apps/web/src/modules/filesystem/components
git commit -m "feat(filesystem-ui): add multi-file transfer center"
```

---

### Task 6: 后端传输列表与前端持久任务

**Files:**
- Create: `backend/internal/modules/filesystem/application/query/list_transfers.go`
- Create: `backend/internal/modules/filesystem/application/query/list_transfers_test.go`
- Modify: `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Modify: `backend/internal/modules/filesystem/interface/http/handler.go`
- Modify: `backend/internal/modules/filesystem/interface/http/transfer.go`
- Modify: `backend/internal/modules/filesystem/module.go`
- Modify: `backend/api/openapi/filesystem.yaml`
- Modify: `fronted/packages/contracts/src/filesystem/index.ts`
- Modify: `fronted/packages/upstream/src/filesystem.ts`
- Modify: `fronted/packages/web-sdk/src/filesystem.ts`
- Modify: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/transfers/route.ts`

**Interfaces:**
- Produces: `GET /api/v1/workspaces/:wsId/transfers?status=&cursor=&limit=`
- Produces: `TransferSessionPageVM`
- Consumes: workspace authorization and persisted transfer sessions.

- [ ] **Step 1: 写查询处理器失败测试**

```go
func TestListTransfersScopesWorkspaceAndStatus(t *testing.T) {
    rows, next, err := handler.Handle(ctx, ListTransfers{
        WorkspaceID: workspaceID,
        Status: model.TransferFailed,
        Limit: 20,
    })
    require.NoError(t, err)
    require.Len(t, rows, 1)
    require.Equal(t, workspaceID, rows[0].WorkspaceID())
    require.Empty(t, next)
}
```

- [ ] **Step 2: 添加仓储接口与查询**

```go
ListTransfers(
    context.Context,
    uuid.UUID,
    *model.TransferStatus,
    int,
    string,
) ([]*model.TransferSession, string, error)
```

持久化按 `created_at DESC, id DESC` 排序，cursor 编码最后一条的时间和 ID；limit 限制为 1–100。

- [ ] **Step 3: 实现查询处理器**

校验 WorkspaceID，解析 status，调用仓储。跨 workspace 数据永远不返回。

- [ ] **Step 4: 注册 HTTP GET**

在现有 `/transfers` 路径增加 GET，权限使用 `file:read`。响应：

```json
{
  "items": [],
  "next_cursor": ""
}
```

- [ ] **Step 5: 更新 OpenAPI 并运行路由一致性测试**

```powershell
cd backend
go test ./api/openapi -run TestFilesystem -count=1
```

Expected: PASS。

- [ ] **Step 6: 添加前端契约**

```ts
export const TransferSessionPageVMSchema = z.object({
  items: z.array(TransferSessionVMSchema),
  next_cursor: z.string().default(""),
});
export type TransferSessionPageVM = z.infer<typeof TransferSessionPageVMSchema>;
```

- [ ] **Step 7: 接通 upstream、BFF 和 SDK**

SDK 签名：

```ts
export function listTransferSessions(
  workspaceId: string,
  query: { status?: TransferStatus; cursor?: string; limit?: number } = {},
): Promise<TransferSessionPageVM>
```

BFF GET 解析 query 后调用 upstream，POST 保持现有创建行为。

- [ ] **Step 8: 运行后端与前端测试**

```powershell
cd backend
go test ./internal/modules/filesystem/... ./api/openapi -count=1
cd ..\fronted
pnpm vitest run packages/web-sdk/src/filesystem.test.ts
pnpm typecheck
```

Expected: PASS。

- [ ] **Step 9: 提交**

```powershell
git add backend fronted/packages fronted/apps/web/app/api/bff
git commit -m "feat(filesystem): add workspace transfer listing"
```

---

### Task 7: 文件详情 Drawer、治理与审计工作区

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/FileDetailDrawer.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileDetailDrawer.test.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/GovernanceWorkspace.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/GovernanceWorkspace.test.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/AuditWorkspace.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/AuditWorkspace.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileAuditPanel.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`
- Modify: `fronted/apps/web/app/(platform)/files/[nodeId]/page.tsx`

**Interfaces:**
- Produces: `FileDetailDrawerProps { node, workspaceId, open, onClose }`
- Produces: `GovernanceWorkspaceProps`
- Produces: `AuditWorkspaceProps`
- Consumes: 现有 revision、provider、pool、binding、lifecycle、audit SDK。

- [ ] **Step 1: 写详情 Drawer 测试**

```tsx
render(<FileDetailDrawer open node={file} workspaceId={workspaceId} onClose={onClose} />);
expect(await screen.findByText("版本 #2")).toBeInTheDocument();
await user.keyboard("{Escape}");
expect(onClose).toHaveBeenCalled();
```

- [ ] **Step 2: 实现详情 Drawer**

Drawer tabs 为“详情 / 版本 / 审计”。打开时按需加载 revisions；预览 API 不可用时显示明确占位，不伪造内容。下载只在 capability URL 可用时显示。

- [ ] **Step 3: 重组治理工作区**

`GovernanceWorkspace` 使用二级 tab：

```ts
type GovernanceSection = "providers" | "bindings" | "lifecycle";
```

每次只渲染一个现有 Panel，避免四个大区块纵向堆叠。

- [ ] **Step 4: 重组审计工作区**

筛选条默认折叠高级项；表格行点击打开 Drawer，展示 detail JSON、actor、resource、decision、trace。筛选写入 URL。

- [ ] **Step 5: 保留深链接**

`/files/[nodeId]?mode=revisions` 继续返回可访问的版本表格；普通 `/files/[nodeId]` 重定向为 `/files?node=<nodeId>`，由工作台打开 Drawer。

- [ ] **Step 6: 运行测试**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/FileDetailDrawer.test.tsx apps/web/src/modules/filesystem/components/GovernanceWorkspace.test.tsx apps/web/src/modules/filesystem/components/AuditWorkspace.test.tsx
pnpm --filter web typecheck
```

Expected: PASS。

- [ ] **Step 7: 提交**

```powershell
git add fronted/apps/web/app fronted/apps/web/src/modules/filesystem/components
git commit -m "feat(filesystem-ui): add details governance and audit workspaces"
```

---

### Task 8: 视觉、响应式、可访问性与总体验收

**Files:**
- Modify: `fronted/apps/web/app/globals.css`
- Modify: `fronted/apps/web/src/modules/filesystem/components/*.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx`
- Modify: `docs/verification/unified-file-infrastructure-g4.md`
- Modify: `docs/verification/unified-file-infrastructure-v1.0.md`

**Interfaces:**
- Consumes: Tasks 1–7 全部 UI 和 API。
- Produces: 可验收统一工作台及 fresh 证据。

- [ ] **Step 1: 添加工作台样式**

新增语义类，不写散落硬编码颜色：

```css
.filesystem-workbench {
  display: grid;
  gap: var(--space-lg);
}

.filesystem-browser-layout {
  display: grid;
  grid-template-columns: minmax(14rem, 18rem) minmax(0, 1fr);
  gap: var(--space-lg);
}

@media (max-width: 900px) {
  .filesystem-browser-layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .filesystem-directory-sidebar {
    display: none;
  }
}
```

若现有 Token 中没有 `--space-lg`，改用仓库已有 spacing utility，不新增仅供单页使用的 Token。

- [ ] **Step 2: 写无障碍测试**

```tsx
expect(screen.getByRole("tablist", { name: "文件系统功能" })).toBeInTheDocument();
expect(screen.getByRole("tree", { name: "目录" })).toBeInTheDocument();
expect(screen.getByRole("button", { name: "切换到网格视图" })).toHaveAttribute(
  "aria-pressed",
  "false",
);
```

再覆盖键盘移动替代方案、Drawer Esc、上传 progressbar。

- [ ] **Step 3: 运行前端完整矩阵**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts
pnpm typecheck
```

Expected: 所有测试 PASS，无未处理 Promise 或 React act warning。

- [ ] **Step 4: 运行后端回归**

```powershell
cd backend
go test ./internal/modules/filesystem/... ./internal/filesystemgateway/... ./internal/sftpgateway/... ./api/openapi -count=1
```

Expected: PASS。

- [ ] **Step 5: 浏览器手工验收**

按顺序验证：

1. 普通成员只看到“文件”和“传输”；
2. 管理员看到四个标签；
3. 列表选择文件后切换网格，选择保持；
4. 目录树懒加载，拖入自身和子目录被拒绝；
5. 拖动文件到另一目录成功，失败时恢复；
6. 桌面拖入 4 个文件时最大 3 个并发；
7. 失败任务可重试，活跃任务可取消；
8. 文件详情 Drawer 显示版本，Esc 关闭；
9. 窄屏目录树通过 Drawer 使用；
10. 不使用鼠标也能通过“移动到…”完成移动。

- [ ] **Step 6: 更新验收证据**

只把实际执行并保留输出的 UI 项更新为 PASS。实时容量曲线继续明确标注“不在范围”，不得把未执行的 20GiB、200 并发或灾备项改成 PASS。

- [ ] **Step 7: 重启并验证开发服务**

```powershell
cd A:\21-Agent\SparkOpsAgent
.\scripts\dev-restart.ps1
curl.exe -sS -i http://localhost:8080/ready
curl.exe -sS -o NUL -w "%{http_code}" http://localhost:3000/files
```

Expected: 后端 `/ready` 为 HTTP 200；前端返回 200 或未登录时 307。

- [ ] **Step 8: 提交**

```powershell
git add fronted docs/verification
git commit -m "feat(filesystem-ui): complete unified workbench"
```

---

## Final Acceptance

Run:

```powershell
cd A:\21-Agent\SparkOpsAgent\backend
go test ./internal/modules/filesystem/... ./internal/filesystemgateway/... ./internal/sftpgateway/... ./api/openapi -count=1

cd A:\21-Agent\SparkOpsAgent\fronted
pnpm vitest run apps/web/src/modules/filesystem packages/web-sdk/src/filesystem.test.ts
pnpm typecheck
```

通过条件：

- 所有命令 exit 0；
- 四个工作台标签按权限显示；
- 网格/列表共享状态；
- 站内拖拽与键盘移动均可用；
- 桌面多文件上传并发不超过 3；
- 传输任务可查询、取消和重试；
- 窄屏与键盘操作可完成核心流程；
- 文档未将实时容量曲线或无现场证据项标为 PASS。
