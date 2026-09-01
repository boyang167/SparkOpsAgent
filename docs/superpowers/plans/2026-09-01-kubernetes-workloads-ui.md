# Kubernetes 工作负载页 UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `/plugins/kubernetes/workloads` 做成对标 GVA 工作负载页的专用 `WorkloadsPanel`（六页签、专属列、YAML/编辑/删除、Pods 更多操作），接入现有列表与写 API。

**Architecture:** 与 `NodesPanel` 同模式：`ResourceMenuPage` 按 menu 分支。列表用 `resourceCollectionUrl`；All Namespaces 走集群范围 list（路径不含 `namespaces/{ns}`）。Pod 终端/日志/文件/指标复用节点详情与 `PodTerminal`。不新增后端路由。

**Tech Stack:** Next.js App Router、React、现有 `k8sFetch` / `YamlEditor` / `PodTerminal` / vitest。

## Global Constraints

- 页签顺序固定：Deployments → DaemonSets → StatefulSets → Jobs → CronJobs → Pods。
- ReplicaSet 不出现在本页页签。
- 不嵌入 Vue / Element Plus / Monaco。
- 不复刻创建向导、伸缩、镜像、暂停、ReplicaSet 回滚 Tab。
- YAML 保持浅色白底；终端保持黑底 `PodTerminal`。
- 不修改 `NodesPanel` 节点逻辑、`PodTerminal` 协议、`.k8s-yaml-editor` 主题。
- 工作负载页不再挂载底部 `OperationsPanel`（pods/metrics）。
- 用户未要求时不执行 git commit。

## File Structure

- Create: `fronted/apps/web/src/modules/plugins/kubernetes/workload-view.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/workload-view.test.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadsPanel.tsx`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadPodActions.tsx`（从节点详情抽出的 Pod 更多操作 Modal：终端/日志/文件/监控/删除）
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/resource-url.ts`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/resource-url.test.ts`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/ResourceMenuPage.tsx`
- Modify: `fronted/apps/web/app/globals.css`（仅工作负载表徽章/页签/操作链接，不改 YAML/终端主题）
- Optional reuse (do not fork): `YamlEditor.tsx`, `PodTerminal.tsx`, `CloudTTYPage.tsx`, `ChatPage.tsx`, `node-view.ts` 的 `formatNodeAge` / `podResourceSummary` / `podContainerNames` / `podRestartCount`

---

### Task 1: All Namespaces 列表 URL

**Files:**
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/resource-url.ts`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/resource-url.test.ts`

**Interfaces:**
- Change `resourceCollectionUrl`：namespaced 资源在 `namespace` 为空时返回集群范围路径（`proxyPath` 不带 ns），不再 `return null`。
- `ResourceTable` 仍应在未选手动命名空间时避免误拉全量：由调用方决定是否传空。工作负载页会显式使用空 NS = All Namespaces。若 `ResourceTable` 依赖 `null` 表示「请先选 NS」，则给 `resourceCollectionUrl` 增加可选 `allowAllNamespaces?: boolean`，仅 WorkloadsPanel 传 `true`。

- [ ] **Step 1: Write the failing test**

在 `resource-url.test.ts` 增加：

```ts
it("lists namespaced resources cluster-wide when all namespaces is allowed", () => {
  const deployment = mustResource("Deployment");
  expect(
    resourceCollectionUrl("prod", deployment, {
      page: 1,
      pageSize: 10,
      allowAllNamespaces: true,
    }),
  ).toBe(
    "/api/bff/v1/plugins/kubernetes/clusters/prod/proxy/apis/apps/v1/deployments?page=1&pageSize=10",
  );
});
```

保留原测试：不传 `allowAllNamespaces` 时 namespaced 无 NS 仍为 `null`。

- [ ] **Step 2: Run test to verify it fails**

```powershell
cd A:\21-Agent\SparkOpsAgent\fronted\apps\web
pnpm exec vitest run src/modules/plugins/kubernetes/resource-url.test.ts
```

- [ ] **Step 3: Implement**

```ts
export function resourceCollectionUrl(
  clusterId: string,
  def: ResourceDef,
  options: ResourceCollectionOptions,
): string | null {
  const namespace = options.namespace?.trim();
  if (def.namespaced && !namespace && !options.allowAllNamespaces) return null;
  // ...
  return `${proxyPath(clusterId, def, namespace)}?${params.toString()}`;
}
```

- [ ] **Step 4: Re-run tests to verify they pass**

```powershell
pnpm exec vitest run src/modules/plugins/kubernetes/resource-url.test.ts
```

- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 2: 工作负载视图纯函数

**Files:**
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/workload-view.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/workload-view.test.ts`

**Interfaces:**

```ts
export const WORKLOAD_TABS = [
  "Deployment",
  "DaemonSet",
  "StatefulSet",
  "Job",
  "CronJob",
  "Pod",
] as const;

export function replicaBadge(ready?: number, desired?: number): { text: string; tone: "ok" | "bad" };
export function daemonSetCounts(obj: Record<string, unknown>): { ready: number; current: number; desired: number };
export function jobStatus(obj: Record<string, unknown>): { active: number; succeeded: number; failed: number; duration: string };
export function cronJobView(obj: Record<string, unknown>): { schedule: string; lastSchedule: string; suspend: boolean };
export function podPhaseBadge(phase?: string): { text: string; tone: "ok" | "bad" | "muted" };
export function emptyWorkloadTemplate(kind: string, namespace?: string): Record<string, unknown>;
```

`podPhaseBadge`：`Running` → 成功 + Running；`Succeeded` → 成功 + Completed；其余危险。Job 耗时用 `status.startTime` 与 `completionTime` 或 now。模板含 `apiVersion`/`kind`/`metadata`，Deployment 含 `spec.replicas: 1` 与空 `selector`/`template` 占位即可。

- [ ] **Step 1: Write failing tests** covering replica 0/1 vs 1/1、Job 三计数、CronJob suspend、Pod Completed、六页签常量不含 ReplicaSet、empty template kind。
- [ ] **Step 2: Run tests, expect fail**
- [ ] **Step 3: Implement `workload-view.ts`**
- [ ] **Step 4: Re-run tests, expect pass**
- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 3: WorkloadsPanel 骨架与控制器表

**Files:**
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadsPanel.tsx`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/ResourceMenuPage.tsx`
- Modify: `fronted/apps/web/app/globals.css`

**Requirements:**

`ResourceMenuPage`：

```tsx
{menu === "nodes" ? <NodesPanel ... /> :
 menu === "workloads" ? <WorkloadsPanel clusterId={clusterId} canManage={canManage} /> :
 <ResourceTable ... />}
```

工作负载分支不渲染标题卡、不渲染两个 `OperationsPanel`。

`WorkloadsPanel` 对标 `NodesPanel`：

- 命名空间：`GET namespaces`，选项含 All Namespaces（值 `""`）。
- CloudShell / AI：与 `NodesPanel` 相同 Modal + `CloudTTYPage` / `ChatPage`。
- 页签状态 `tab: typeof WORKLOAD_TABS[number]`，默认 Deployment。
- 列表：`resourceCollectionUrl(..., { namespace, page, pageSize, keyword, allowAllNamespaces: !namespace })`。
- 分页：`PAGE_SIZES = [10, 30, 50, 100]`。
- 控制器列：名称（可点详情 + labels tooltip）、命名空间、专属列、存活时间、YAML / 编辑 / 删除。
- 无权限：`.k8s-action-link:disabled`。
- YAML/编辑/新增：浅色 `YamlEditor`；保存 PUT 单项或 POST 集合（与 `ResourceTable` 相同）。
- 删除：确认后 DELETE。
- 详情：只读 `YamlEditor` 或 `<pre>` JSON。

样式：`.k8s-tabs`、`.k8s-badge-ok`、`.k8s-badge-bad`、`.k8s-action-link`（可复用节点页已有 action-link）。

- [ ] **Step 1: Wire ResourceMenuPage branch and panel shell (tabs + empty table)**
- [ ] **Step 2: Load list and render Deployment/DaemonSet/StatefulSet/Job/CronJob columns**
- [ ] **Step 3: YAML / 编辑 / 新增 / 删除**
- [ ] **Step 4: Browser check `/plugins/kubernetes/workloads`**：六页签切换、选 NS、分页、禁用态。无浏览器工具则用 curl 打列表 URL 并说明未做 UI 点击。
- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 4: Pods 页签与更多操作

**Files:**
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadPodActions.tsx`
- Modify: `fronted/apps/web/src/modules/plugins/kubernetes/specialized/WorkloadsPanel.tsx`
- Optional extract from `NodeDetailView.tsx`：日志 SSE、files API、metrics POST；**不要**改节点页行为，只抽共享组件或在 WorkloadPodActions 内复制调用路径。

**Requirements:**

Pods 列：名称（图标可省略或用现有 SVG）、CPU 申请/限制、内存申请/限制、容器色点、CPU/内存/TCP（metrics 有则显示摘要否则 `-`）、状态、重启、命名空间、IP、节点、存活时间、编辑 + 更多。

更多 Modal：详情、每容器文件、监控、每容器终端、每容器终端日志、删除。

- 终端：`<PodTerminal clusterId namespace pod container />`
- 日志：现有 `.../pods/{pod}/logs?container=&follow=true` EventSource
- 文件：现有 files list/upload/download/delete
- 监控：`POST .../metrics` `pod_cpu` / `pod_memory`

Pods 工具栏无 YAML 按钮，仅「新增」（空 Pod 模板）。

- [ ] **Step 1: Render Pods table from list API using `podResourceSummary` / `podRestartCount` / `podContainerNames`**
- [ ] **Step 2: Wire more-actions Modal + PodTerminal / logs / files / metrics / YAML edit / delete**
- [ ] **Step 3: Optional best-effort metrics column; failure must not empty the table**
- [ ] **Step 4: Browser：打开 Pods，点更多 → 终端（黑底）、YAML（白底）、删除确认。核对节点页终端与 YAML 未回归。**
- [ ] **Step 5: Do not commit unless the user asks**

---

### Task 5: 回归测试

**Files:** none new

- [ ] **Step 1:** `pnpm exec vitest run src/modules/plugins/kubernetes`

期望：含 resource-url、workload-view、既有 yaml/pod-terminal 测试全部通过。

- [ ] **Step 2:** 浏览器核对工作负载六页签 + 节点页仍可用。

- [ ] **Step 3:** Do not commit unless the user asks
