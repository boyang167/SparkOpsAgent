# Kubernetes Pod 详情 UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将工作负载页 Pod 的两处详情入口统一为参考 GVA 的结构化详情弹窗。

**Architecture:** `WorkloadsPanel` 负责获取最新 Pod 和 Events、管理取消与错误；无状态 `PodDetailView` 负责展示；`pod-detail-view.ts` 提供可测试的格式化函数。`WorkloadPodActions` 只把详情点击上抛给父组件。

**Tech Stack:** React、Next.js、TypeScript、现有 `Modal` / `ApiErrorBanner` / `k8sFetch`、Vitest。

## Global Constraints

- 详情只读，不要求 `canManage`。
- 页签顺序固定：容器、事件、创建者、初始化容器、存储。
- 名称点击与「更多 → 详情」必须进入同一个详情组件。
- 每次打开获取最新 Pod；Events 失败不阻止其他详情。
- 不修改监控、文件、终端、日志、YAML 编辑与删除行为。
- 不新增后端路由，不修改 `PodTerminal` 协议或 YAML/终端主题。
- 用户未要求时不执行 git commit。

## File Structure

- Create: `fronted/apps/web/src/modules/plugins/kubernetes/pod-detail-view.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/pod-detail-view.test.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/PodDetailView.tsx`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadsPanel.tsx`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadPodActions.tsx`
- Modify: `fronted/apps/web/app/globals.css`

---

### Task 1: Pod 详情格式化函数

**Files:**
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/pod-detail-view.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/pod-detail-view.test.ts`

**Interfaces:**

```ts
export type PodEvent = Record<string, unknown>;
export function sortPodEvents(events: PodEvent[]): PodEvent[];
export function formatContainerPorts(container: Record<string, unknown>): string;
export function formatContainerCommand(container: Record<string, unknown>): string;
export function formatEnvironment(container: Record<string, unknown>): string[];
export function volumeSourceView(volume: Record<string, unknown>): { type: string; detail: string };
export function eventTimestamp(event: PodEvent): string | undefined;
```

- [ ] **Step 1: Write failing tests**

覆盖事件按 `lastTimestamp` / `eventTime` / creationTimestamp 倒序、TCP/UDP 端口格式、command+args、普通与 valueFrom 环境变量，以及 hostPath/configMap/secret/PVC/emptyDir/未知 volume。

- [ ] **Step 2: Verify RED**

```powershell
cd A:\21-Agent\SparkOpsAgent\fronted
pnpm exec vitest run apps/web/src/modules/plugins/kubernetes/pod-detail-view.test.ts
```

期望：模块缺失或导出缺失导致失败。

- [ ] **Step 3: Implement minimal pure functions**

所有未知值使用安全字符串转换；不得输出 HTML。

- [ ] **Step 4: Verify GREEN**

运行同一命令，期望全部通过。

- [ ] **Step 5: Do not commit unless requested**

---

### Task 2: 结构化 PodDetailView

**Files:**
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/PodDetailView.tsx`
- Modify: `fronted/apps/web/app/globals.css`

**Interfaces:**

```ts
export function PodDetailView(props: {
  pod: PodObject;
  events: PodEvent[];
  eventsError?: ApiErrorInfo | null;
}): JSX.Element;
```

- [ ] **Step 1: Build the read-only view**

渲染基本信息两列区域、Conditions 表，以及固定五页签。容器和初始化容器行可展开，展示镜像策略、环境变量、命令参数、requests/limits 和挂载点。事件、ownerReferences、volumes 使用 Task 1 函数。

- [ ] **Step 2: Add scoped styles**

只新增 `.k8s-pod-detail-*` 样式；不修改 `.k8s-yaml-*`、`.xterm*` 或 Pod 终端样式。

- [ ] **Step 3: Verify TypeScript**

```powershell
cd A:\21-Agent\SparkOpsAgent\fronted
pnpm --filter @sparkops/web typecheck
```

期望退出码 0。

- [ ] **Step 4: Do not commit unless requested**

---

### Task 3: 统一详情数据流与入口

**Files:**
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadsPanel.tsx`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadPodActions.tsx`

**Interfaces:**

```ts
// WorkloadPodActions new prop
onOpenDetail: (pod: PodObject) => void;
```

- [ ] **Step 1: Replace WorkloadPodActions local JSON detail**

「详情」ActionItem 调用 `onOpenDetail(pod)`，删除本地 detail view 与 JSON Modal；其他操作保持不变。

- [ ] **Step 2: Add unified loader in WorkloadsPanel**

以 Pod identity 保存目标；并行 GET 最新 Pod 与 namespaced Events。Events URL 只带 URL 编码后的 `fieldSelector=involvedObject.name={pod}`。用 AbortController + sequence 防止关闭、切集群、切 namespace、切页签和连续点 Pod 时的旧响应覆盖。

- [ ] **Step 3: Render the shared Modal**

Pod 获取成功后用大尺寸 `Modal` + `PodDetailView`；Pod 失败显示现有全局错误且不打开空详情；Events 失败保存局部 `eventsError`。

- [ ] **Step 4: Route both entry points**

Pod 名称和 `WorkloadPodActions` 的详情操作均调用同一个 loader。控制器原有 JSON 详情保持不变。

- [ ] **Step 5: Verify**

```powershell
cd A:\21-Agent\SparkOpsAgent\fronted
pnpm exec vitest run apps/web/src/modules/plugins/kubernetes
pnpm --filter @sparkops/web typecheck
```

期望测试、类型检查全部通过。

- [ ] **Step 6: Do not commit unless requested**

---

### Task 4: 回归与页面验收

- [ ] **Step 1: Run fresh full verification**

```powershell
cd A:\21-Agent\SparkOpsAgent\fronted
pnpm exec vitest run apps/web/src/modules/plugins/kubernetes
pnpm --filter @sparkops/web typecheck
```

- [ ] **Step 2: Verify page response**

```powershell
(Invoke-WebRequest -UseBasicParsing "http://localhost:3000/plugins/kubernetes/workloads").StatusCode
```

期望 `200`。

- [ ] **Step 3: Manual acceptance**

确认 Pod 名称和「更多 → 详情」打开同一结构化弹窗；五页签顺序正确；Events 局部错误不影响基本信息；关闭后网络流停止；终端、日志、文件、监控入口仍可打开。

- [ ] **Step 4: Do not commit unless requested**
