# Release Flow（发布流程设计图）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将多集群发布抽象为 Workspace 级可复用线性 **ReleaseFlow 设计图**（节点=具体 K8s 集群 + 三态人工审核）；项目绑定后每次发布只选制品来源并按图执行。

**Architecture:** 新增 `ReleaseFlow` 产品层（Studio 编辑器 + 项目绑定 + 简化发布向导）；底层通过 `release-flow-compile.ts` 将线性链编译为 release-only Pipeline DAG（对用户透明）。第一期前端 Mock 持久化 + sessionStorage Run 推进；第二期再接 `cd` schema 与 Run Engine。

**Tech Stack:** Next.js 15、React 19、TypeScript 5.8、`@xyflow/react`、Vitest、pnpm 9；复用现有 `fronted/apps/web/src/modules/delivery/cd/` 模块与 Jenkins `triggerBuild` API。

## Global Constraints

- 集群绑定方式：**写死** Workspace 内已授权的具体集群（非占位）。
- 拓扑：**只允许一条链**（线性顺序，无分叉、无并行）。
- 审核：每节点 `none` / `before_deploy` / `after_deploy`。
- 构建仍归 Jenkins 模板；ReleaseFlow 只回答「怎么发布」；中间交接 `ArtifactSet { image_repository, image_tag }`。
- 项目字段：`release_flow_id`（发布）与 `jenkinsfile_template_id`（构建）分开。
- 绑定设计图后发布向导 **不再选 Pipeline/Fleet**；只选制品来源 + 确认。
- 保留经典 `POST .../deployments` 单集群路径；不替换 Pipeline Studio 通用 DAG（一期可保留菜单）。
- 第一期不做：并行分叉、Canary/蓝绿、向导内改链、image digest、完整后端 Run Engine。
- 节点 failed 时停止整条链（一期不做 skip/onFailure）。
- 本计划不含任何自动 `git commit` 步骤。

---

## File map

| 路径 | 职责 |
|------|------|
| `fronted/apps/web/src/modules/delivery/cd/types.ts` | 新增 `ReleaseFlowVM`、`ReleaseFlowNode`、`NodeApproval` 等类型 |
| `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-validate.ts` | 线性链校验（单链、无重复 cluster） |
| `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-validate.test.ts` | 校验单测 |
| `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-compile.ts` | Flow → `PipelineStage[]` 编译 |
| `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-compile.test.ts` | 三态审核编译单测 |
| `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-store.ts` | Mock 持久化（localStorage） |
| `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-binding-store.ts` | 项目 ↔ Flow 绑定 Mock |
| `fronted/apps/web/src/modules/delivery/cd/mock/data.ts` | 预置 Flow 模板、Mock 集群 |
| `fronted/apps/web/src/modules/delivery/cd/components/ReleaseFlowStudio/` | 线性链编辑器（React Flow） |
| `fronted/apps/web/app/(platform)/delivery/release-flows/` | 列表页 + Studio 路由 |
| `fronted/apps/web/src/modules/delivery/cd/components/ReleaseWizard/ReleaseWizardModal.tsx` | 简化为 2 步向导 |
| `fronted/apps/web/src/modules/delivery/cd/lib/release-run-store.ts` | 改为 Flow 节点状态 |
| `fronted/apps/web/src/modules/delivery/cd/components/ReleaseRunBoard/` | 按设计图节点展示 Run |
| `fronted/apps/web/src/modules/workspace/components/project/deployment-gate.ts` | 发布门禁要求 `release_flow_id` |
| `fronted/apps/web/src/modules/workspace/components/project/ProjectReleaseFlowBinding.tsx` | 项目绑定 UI |
| `fronted/apps/web/src/components/layout/nav-tree.ts` | 新增「发布流程」菜单 |
| `fronted/packages/contracts/src/workspace/index.ts` | `ProjectVM.release_flow_id` 可选字段 |
| `backend/migrations/0000xx_release_flow.up.sql` | Phase 2：表结构 |
| `backend/internal/modules/cd/` | Phase 2：领域 + HTTP |

---

### Task 1: ReleaseFlow 类型与线性链校验

**Files:**
- Modify: `fronted/apps/web/src/modules/delivery/cd/types.ts`
- Create: `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-validate.ts`
- Create: `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-validate.test.ts`

**Interfaces:**
- Consumes: 无。
- Produces:
  - `type NodeApproval = "none" | "before_deploy" | "after_deploy"`
  - `interface ReleaseFlowNodeVM { id: string; clusterId: string; clusterName: string; displayName: string; approval: NodeApproval; profileId?: string; sortOrder: number }`
  - `interface ReleaseFlowVM { id: string; name: string; description: string; status: "draft" | "active" | "archived"; version: number; nodes: ReleaseFlowNodeVM[]; layout: Record<string, { x: number; y: number }>; updatedAt: string }`
  - `function validateReleaseFlowNodes(nodes: ReleaseFlowNodeVM[]): string[]`

- [ ] **Step 1: 写失败测试**

```typescript
// release-flow-validate.test.ts
import { describe, expect, it } from "vitest";
import { validateReleaseFlowNodes } from "./release-flow-validate";
import type { ReleaseFlowNodeVM } from "../types";

function node(partial: Partial<ReleaseFlowNodeVM> & Pick<ReleaseFlowNodeVM, "sortOrder" | "clusterId">): ReleaseFlowNodeVM {
  return {
    id: `n-${partial.sortOrder}`,
    clusterName: partial.clusterName ?? partial.clusterId,
    displayName: partial.displayName ?? partial.clusterId,
    approval: partial.approval ?? "none",
    ...partial,
  };
}

describe("validateReleaseFlowNodes", () => {
  it("accepts a non-empty linear chain with unique clusters", () => {
    const errors = validateReleaseFlowNodes([
      node({ sortOrder: 0, clusterId: "c1" }),
      node({ sortOrder: 1, clusterId: "c2" }),
    ]);
    expect(errors).toEqual([]);
  });

  it("rejects empty chain", () => {
    expect(validateReleaseFlowNodes([])).toContain("至少添加一个集群节点");
  });

  it("rejects duplicate cluster_id", () => {
    const errors = validateReleaseFlowNodes([
      node({ sortOrder: 0, clusterId: "c1" }),
      node({ sortOrder: 1, clusterId: "c1" }),
    ]);
    expect(errors.some((e) => e.includes("重复"))).toBe(true);
  });

  it("rejects non-contiguous sort_order", () => {
    const errors = validateReleaseFlowNodes([
      node({ sortOrder: 0, clusterId: "c1" }),
      node({ sortOrder: 2, clusterId: "c2" }),
    ]);
    expect(errors.some((e) => e.includes("顺序"))).toBe(true);
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd fronted && pnpm vitest run apps/web/src/modules/delivery/cd/lib/release-flow-validate.test.ts`  
Expected: FAIL — `validateReleaseFlowNodes is not defined`

- [ ] **Step 3: 实现 types + validate**

```typescript
// types.ts 追加
export type NodeApproval = "none" | "before_deploy" | "after_deploy";

export interface ReleaseFlowNodeVM {
  id: string;
  clusterId: string;
  clusterName: string;
  displayName: string;
  approval: NodeApproval;
  profileId?: string;
  sortOrder: number;
}

export interface ReleaseFlowVM {
  id: string;
  name: string;
  description: string;
  status: "draft" | "active" | "archived";
  version: number;
  nodes: ReleaseFlowNodeVM[];
  layout: Record<string, { x: number; y: number }>;
  updatedAt: string;
}

export const APPROVAL_LABELS: Record<NodeApproval, string> = {
  none: "不审核",
  before_deploy: "部署前审核",
  after_deploy: "部署后审核",
};
```

```typescript
// release-flow-validate.ts
import type { ReleaseFlowNodeVM } from "../types";

export function validateReleaseFlowNodes(nodes: ReleaseFlowNodeVM[]): string[] {
  const errors: string[] = [];
  if (nodes.length === 0) {
    errors.push("至少添加一个集群节点");
    return errors;
  }
  const sorted = [...nodes].sort((a, b) => a.sortOrder - b.sortOrder);
  const clusterIds = new Set<string>();
  sorted.forEach((node, index) => {
    if (node.sortOrder !== index) errors.push("节点顺序必须从 0 连续递增");
    if (clusterIds.has(node.clusterId)) errors.push(`集群 ${node.clusterName} 重复出现`);
    clusterIds.add(node.clusterId);
    if (!node.clusterId.trim()) errors.push("存在未选择集群的节点");
  });
  return [...new Set(errors)];
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd fronted && pnpm vitest run apps/web/src/modules/delivery/cd/lib/release-flow-validate.test.ts`  
Expected: PASS (4 tests)

---

### Task 2: ReleaseFlow → Pipeline 编译器

**Files:**
- Create: `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-compile.ts`
- Create: `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-compile.test.ts`

**Interfaces:**
- Consumes: `ReleaseFlowNodeVM`, `PipelineStage` from `../types`
- Produces: `function compileReleaseFlowToStages(nodes: ReleaseFlowNodeVM[]): PipelineStage[]`

- [ ] **Step 1: 写三态审核编译测试**

```typescript
import { describe, expect, it } from "vitest";
import { compileReleaseFlowToStages } from "./release-flow-compile";
import type { ReleaseFlowNodeVM } from "../types";

function node(i: number, approval: ReleaseFlowNodeVM["approval"]): ReleaseFlowNodeVM {
  return {
    id: `n${i}`,
    clusterId: `cluster-${i}`,
    clusterName: `cluster-${i}`,
    displayName: `node-${i}`,
    approval,
    sortOrder: i,
  };
}

describe("compileReleaseFlowToStages", () => {
  it("expands none approval to deploy + wait_rollout only", () => {
    const stages = compileReleaseFlowToStages([node(0, "none")]);
    expect(stages.map((s) => s.id)).toEqual(["deploy-0"]);
    expect(stages[0]!.steps.map((s) => s.uses)).toEqual(["k8s.deploy", "k8s.wait_rollout"]);
  });

  it("expands before_deploy with approve stage first", () => {
    const stages = compileReleaseFlowToStages([node(0, "before_deploy")]);
    expect(stages.map((s) => s.id)).toEqual(["approve-0", "deploy-0"]);
    expect(stages[0]!.steps[0]!.uses).toBe("approval.manual");
    expect(stages[1]!.needs).toEqual(["approve-0"]);
  });

  it("expands after_deploy with approve after deploy", () => {
    const stages = compileReleaseFlowToStages([node(0, "after_deploy")]);
    expect(stages.map((s) => s.id)).toEqual(["deploy-0", "approve-0"]);
    expect(stages[1]!.needs).toEqual(["deploy-0"]);
  });

  it("chains node i-1 last stage to node i first stage", () => {
    const stages = compileReleaseFlowToStages([node(0, "none"), node(1, "before_deploy")]);
    expect(stages[1]!.needs).toEqual(["deploy-0"]);
  });
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd fronted && pnpm vitest run apps/web/src/modules/delivery/cd/lib/release-flow-compile.test.ts`  
Expected: FAIL

- [ ] **Step 3: 实现编译器**

```typescript
import type { PipelineStage, ReleaseFlowNodeVM } from "../types";

function deployStage(node: ReleaseFlowNodeVM, needs?: string[]): PipelineStage {
  return {
    id: `deploy-${node.sortOrder}`,
    needs,
    steps: [
      { uses: "k8s.deploy", with: { cluster: node.clusterId, profile: node.profileId } },
      { uses: "k8s.wait_rollout", with: { cluster: node.clusterId } },
    ],
  };
}

function approveStage(node: ReleaseFlowNodeVM, needs?: string[]): PipelineStage {
  return {
    id: `approve-${node.sortOrder}`,
    needs,
    steps: [
      {
        uses: "approval.manual",
        with: { title: `发布到 ${node.displayName}` },
      },
    ],
  };
}

function lastStageId(node: ReleaseFlowNodeVM): string {
  return node.approval === "before_deploy" ? `deploy-${node.sortOrder}` : `approve-${node.sortOrder}`;
}

function firstStageId(node: ReleaseFlowNodeVM): string {
  return node.approval === "before_deploy" ? `approve-${node.sortOrder}` : `deploy-${node.sortOrder}`;
}

export function compileReleaseFlowToStages(nodes: ReleaseFlowNodeVM[]): PipelineStage[] {
  const sorted = [...nodes].sort((a, b) => a.sortOrder - b.sortOrder);
  const stages: PipelineStage[] = [];
  sorted.forEach((node, index) => {
    const prevNeeds = index === 0 ? undefined : [lastStageId(sorted[index - 1]!)];
    if (node.approval === "before_deploy") {
      stages.push(approveStage(node, prevNeeds));
      stages.push(deployStage(node, [`approve-${node.sortOrder}`]));
      return;
    }
    stages.push(deployStage(node, prevNeeds));
    if (node.approval === "after_deploy") {
      stages.push(approveStage(node, [`deploy-${node.sortOrder}`]));
    }
  });
  return stages;
}

export function flowChainSummary(nodes: ReleaseFlowNodeVM[]): string {
  const sorted = [...nodes].sort((a, b) => a.sortOrder - b.sortOrder);
  return sorted
    .map((n) => {
      const label = n.approval === "none" ? n.displayName : `${n.displayName}(${APPROVAL_SHORT[n.approval]})`;
      return label;
    })
    .join(" → ");
}

const APPROVAL_SHORT = {
  none: "",
  before_deploy: "部署前审",
  after_deploy: "部署后审",
} as const;
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd fronted && pnpm vitest run apps/web/src/modules/delivery/cd/lib/release-flow-compile.test.ts`  
Expected: PASS (4 tests)

---

### Task 3: Mock 数据与 Flow 持久化

**Files:**
- Modify: `fronted/apps/web/src/modules/delivery/cd/mock/data.ts`
- Create: `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-store.ts`
- Create: `fronted/apps/web/src/modules/delivery/cd/lib/release-flow-binding-store.ts`

**Interfaces:**
- Consumes: `ReleaseFlowVM`, `MOCK_CLUSTERS`
- Produces:
  - `getMockReleaseFlows(): ReleaseFlowVM[]`
  - `getMockReleaseFlow(id: string): ReleaseFlowVM | undefined`
  - `saveReleaseFlow(flow: ReleaseFlowVM): void`
  - `getProjectReleaseFlowId(projectId: string): string | undefined`
  - `setProjectReleaseFlowId(projectId: string, flowId: string): void`

- [ ] **Step 1: 在 mock/data.ts 追加两个预置 Flow**

```typescript
export const MOCK_RELEASE_FLOWS: ReleaseFlowVM[] = [
  {
    id: "flow-staging-prod",
    name: "Staging → Prod",
    description: "预发验证后，生产部署前人工审核",
    status: "active",
    version: 1,
    updatedAt: "2026-09-03T00:00:00Z",
    layout: { "n0": { x: 0, y: 0 }, "n1": { x: 280, y: 0 } },
    nodes: [
      { id: "n0", clusterId: "staging-hz-1", clusterName: "staging-hangzhou-1", displayName: "staging", approval: "after_deploy", sortOrder: 0 },
      { id: "n1", clusterId: "prod-bj-1", clusterName: "prod-beijing-1", displayName: "prod", approval: "before_deploy", sortOrder: 1, profileId: "profile-prod-cn" },
    ],
  },
  {
    id: "flow-dev-staging-prod",
    name: "Dev → Staging → Prod",
    description: "三环境线性发布",
    status: "active",
    version: 1,
    updatedAt: "2026-09-03T00:00:00Z",
    layout: { "n0": { x: 0, y: 0 }, "n1": { x: 280, y: 0 }, "n2": { x: 560, y: 0 } },
    nodes: [
      { id: "n0", clusterId: "staging-hz-2", clusterName: "staging-hangzhou-2", displayName: "dev", approval: "none", sortOrder: 0 },
      { id: "n1", clusterId: "staging-hz-1", clusterName: "staging-hangzhou-1", displayName: "staging", approval: "none", sortOrder: 1 },
      { id: "n2", clusterId: "prod-bj-1", clusterName: "prod-beijing-1", displayName: "prod", approval: "before_deploy", sortOrder: 2 },
    ],
  },
];
```

- [ ] **Step 2: 实现 localStorage store**

```typescript
// release-flow-store.ts
const KEY = "sparkops.release-flows";

export function listReleaseFlows(): ReleaseFlowVM[] { /* merge MOCK + localStorage */ }
export function getReleaseFlow(id: string): ReleaseFlowVM | undefined { /* ... */ }
export function saveReleaseFlow(flow: ReleaseFlowVM): void {
  // version +1 on save; validateReleaseFlowNodes first
}

// release-flow-binding-store.ts
const BIND_KEY = "sparkops.project-release-flow";

export function getProjectReleaseFlowId(projectId: string): string | undefined { /* ... */ }
export function setProjectReleaseFlowId(projectId: string, flowId: string): void { /* ... */ }
```

- [ ] **Step 3: 手动验证**

1. 在浏览器 console 调用 `setProjectReleaseFlowId("project-id", "flow-staging-prod")`
2. 刷新后 `getProjectReleaseFlowId` 返回相同 id

---

### Task 4: Release Flow Studio（线性链编辑器）

**Files:**
- Create: `fronted/apps/web/src/modules/delivery/cd/components/ReleaseFlowStudio/ClusterNode.tsx`
- Create: `fronted/apps/web/src/modules/delivery/cd/components/ReleaseFlowStudio/ClusterPalette.tsx`
- Create: `fronted/apps/web/src/modules/delivery/cd/components/ReleaseFlowStudio/ReleaseFlowCanvas.tsx`
- Create: `fronted/apps/web/src/modules/delivery/cd/components/ReleaseFlowStudio/NodePropertyPanel.tsx`
- Create: `fronted/apps/web/src/modules/delivery/cd/components/ReleaseFlowStudio/ReleaseFlowStudio.tsx`
- Create: `fronted/apps/web/src/modules/delivery/cd/components/ReleaseFlowStudio/ReleaseFlowListPanel.tsx`
- Create: `fronted/apps/web/app/(platform)/delivery/release-flows/page.tsx`
- Create: `fronted/apps/web/app/(platform)/delivery/release-flows/[flowId]/studio/page.tsx`
- Modify: `fronted/apps/web/src/components/layout/nav-tree.ts`
- Modify: `fronted/apps/web/app/globals.css`（`.cd-release-flow-*` 样式）

**Interfaces:**
- Consumes: `MOCK_CLUSTERS`, `validateReleaseFlowNodes`, `saveReleaseFlow`, `@xyflow/react`
- Produces: 可编辑并保存线性 ReleaseFlow 的 Studio 页面

- [ ] **Step 1: 列表页 + 路由 + 菜单**

`nav-tree.ts` Delivery 分组追加：

```typescript
{ href: "/delivery/release-flows", label: "发布流程", entitlement: "delivery" },
```

`/delivery/release-flows/page.tsx` 渲染 `ReleaseFlowListPanel`（卡片列表 + 「新建」按钮）。

- [ ] **Step 2: ClusterPalette + ClusterNode**

- 左侧 `ClusterPalette`：列出 `MOCK_CLUSTERS`（ready/blocked 标签）
- 拖入画布创建节点；节点显示 `displayName`、审核 badge（`APPROVAL_LABELS`）

- [ ] **Step 3: ReleaseFlowCanvas 限制单链**

复用 `PipelineCanvas` 模式，但连线规则：

```typescript
function canConnect(source: Node, target: Node, edges: Edge[]): boolean {
  // 每个节点最多 1 条入边、1 条出边
  // 禁止成环；保存时按拓扑排序写 sortOrder
}
```

保存时：拓扑排序 → `sortOrder` 0..N-1 → `validateReleaseFlowNodes` → `saveReleaseFlow`。

- [ ] **Step 4: NodePropertyPanel**

选中节点编辑：`displayName`、`approval`（select 三态）、`profileId`（可选，Mock profiles 下拉）。

- [ ] **Step 5: 手动验收**

1. 打开 `http://127.0.0.1:3000/delivery/release-flows`
2. 进入 Studio，拖入 2 个集群，连线，prod 设 `before_deploy`
3. 保存后刷新，布局与审核模式仍在

---

### Task 5: 项目绑定 ReleaseFlow

**Files:**
- Modify: `fronted/packages/contracts/src/workspace/index.ts`
- Create: `fronted/apps/web/src/modules/workspace/components/project/ProjectReleaseFlowBinding.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectConfigurePanel.tsx`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/deployment-gate.ts`
- Create: `fronted/apps/web/src/modules/workspace/components/project/release-flow-gate.test.ts`

**Interfaces:**
- Consumes: `getProjectReleaseFlowId`, `setProjectReleaseFlowId`, `listReleaseFlows`, `flowChainSummary`
- Produces: `releaseEntryAction` 在未绑定 Flow 时 disabled + reason

- [ ] **Step 1: 写门禁测试**

```typescript
import { describe, expect, it } from "vitest";
import { releaseFlowGate } from "./deployment-gate";

describe("releaseFlowGate", () => {
  it("blocks release when project has no release_flow_id", () => {
    const result = releaseFlowGate({ releaseFlowId: undefined });
    expect(result.allowed).toBe(false);
    expect(result.missing).toContain("发布流程");
  });

  it("allows when release_flow_id is set", () => {
    expect(releaseFlowGate({ releaseFlowId: "flow-staging-prod" }).allowed).toBe(true);
  });
});
```

- [ ] **Step 2: 扩展 ProjectVM（可选字段，后端未就绪前用 binding-store 兜底）**

```typescript
// contracts/workspace/index.ts
release_flow_id: z.string().optional(),
```

- [ ] **Step 3: ProjectReleaseFlowBinding 组件**

在项目配置 Modal 增加第 4 区块或独立 Tab：

- 下拉 active ReleaseFlow
- 只读链预览：`flowChainSummary(flow.nodes)`
- 保存调用 `setProjectReleaseFlowId(project.id, flowId)`

- [ ] **Step 4: 更新 releaseEntryAction**

```typescript
export function resolveReleaseFlowId(project: ProjectVM): string | undefined {
  return project.release_flow_id ?? getProjectReleaseFlowId(project.id);
}

export function releaseEntryAction(/* ... */): DeploymentEntryAction {
  const flowId = resolveReleaseFlowId(project);
  if (!flowId) {
    return { disabled: true, label: "发布", reason: "请先绑定发布流程" };
  }
  // 其余沿用现有 releaseEntryAction（审核通过即可，不在 Step1 拦集群）
}
```

- [ ] **Step 5: 运行测试 + typecheck**

Run:
```bash
cd fronted && pnpm vitest run apps/web/src/modules/workspace/components/project/release-flow-gate.test.ts
pnpm --filter @sparkops/web typecheck
```

---

### Task 6: 简化发布向导（2 步）

**Files:**
- Modify: `fronted/apps/web/src/modules/delivery/cd/components/ReleaseWizard/ReleaseWizardModal.tsx`
- Modify: `fronted/apps/web/src/modules/delivery/cd/lib/release-draft.ts`
- Modify: `fronted/apps/web/src/modules/delivery/cd/lib/release-draft.test.ts`
- Modify: `fronted/apps/web/src/modules/delivery/cd/lib/release-run-store.ts`
- Modify: `fronted/apps/web/src/modules/delivery/cd/types.ts`

**Interfaces:**
- Consumes: `resolveReleaseFlowId`, `getReleaseFlow`, `flowChainSummary`, `compileReleaseFlowToStages`
- Produces: 2 步向导；`StoredReleaseRun` 含 `releaseFlowId`, `flowVersion`, `nodes[]` 状态

- [ ] **Step 1: 更新 release-draft 测试 — 不再要求 pipelineId/fleetIds**

```typescript
it("validates artifact tag and bound release flow only", () => {
  const errors = validateReleaseDraft(
    draft({ pipelineId: "", fleetIds: [], imageTag: "v1" }),
    { releaseFlowId: "flow-staging-prod" },
  );
  expect(errors).toEqual([]);
});

it("requires release flow binding", () => {
  const errors = validateReleaseDraft(draft(), { releaseFlowId: undefined });
  expect(errors.some((e) => e.includes("发布流程"))).toBe(true);
});
```

- [ ] **Step 2: 重写 ReleaseWizardModal 步骤**

```typescript
const STEPS = ["制品来源", "确认发布"] as const;
```

- Step 0：保留现有制品来源（artifact / build），去掉 Pipeline/Fleet
- Step 1：只读展示 `flowChainSummary` + 各节点审核 + 制品坐标；确认后 `saveReleaseRun`
- 打开向导时若无 `releaseFlowId`：InlineAlert + 禁用下一步

- [ ] **Step 3: 扩展 StoredReleaseRun**

```typescript
export interface StoredReleaseRun {
  id: string;
  projectId: string;
  releaseFlowId: string;
  releaseFlowName: string;
  flowVersion: number;
  nodes: Array<{
    nodeId: string;
    displayName: string;
    clusterId: string;
    approval: NodeApproval;
    status: "pending" | "waiting_approval" | "deploying" | "succeeded" | "failed";
  }>;
  source: ArtifactSource;
  imageTag: string;
  imageRepository: string;
  gitRefType: string;
  gitRefValue: string;
  buildId?: string;
  createdAt: string;
}
```

- [ ] **Step 4: 手动验收**

1. 项目绑定 `flow-staging-prod`
2. 点「发布」→ 仅 2 步
3. 确认页显示 `staging(部署后审) → prod(部署前审)`
4. 确认后跳转 `/delivery/runs/rel-xxx`

---

### Task 7: Release Run 按设计图节点观测（Mock 推进）

**Files:**
- Create: `fronted/apps/web/src/modules/delivery/cd/components/ReleaseRunBoard/ReleaseRunBoard.tsx`
- Create: `fronted/apps/web/src/modules/delivery/cd/lib/release-run-engine.mock.ts`
- Modify: `fronted/apps/web/src/modules/delivery/cd/components/FleetBoard/FleetBoardMock.tsx`
- Modify: `fronted/apps/web/app/(platform)/delivery/runs/[runId]/page.tsx`

**Interfaces:**
- Consumes: `StoredReleaseRun`, `NodeApproval`
- Produces:
  - `function advanceMockReleaseRun(runId: string): StoredReleaseRun | undefined`
  - `function approveMockNode(runId: string, nodeId: string): StoredReleaseRun | undefined`

- [ ] **Step 1: Mock 节点状态机**

```typescript
// release-run-engine.mock.ts
export function approveMockNode(runId: string, nodeId: string): StoredReleaseRun | undefined {
  const run = getReleaseRun(runId);
  if (!run) return undefined;
  const node = run.nodes.find((n) => n.nodeId === nodeId);
  if (!node || node.status !== "waiting_approval") return run;
  node.status = "deploying";
  // 模拟 deploy 完成后 mark succeeded 并推进下一节点
  return persist(run);
}
```

状态转移规则（spec 8.2）：

| approval | 进入节点 | 需审批 | 部署后 |
|----------|----------|--------|--------|
| none | deploying | — | succeeded |
| before_deploy | waiting_approval | → deploying | succeeded |
| after_deploy | deploying | waiting_approval | succeeded |

- [ ] **Step 2: ReleaseRunBoard UI**

横向节点链（复用 `.cd-release-flow-node` 样式）：

- 当前节点高亮
- `waiting_approval` 显示「通过 / 拒绝」按钮（Mock）
- 不展示 Pipeline YAML

- [ ] **Step 3: 替换 Run 详情页**

`FleetBoardMock` 改为读新 `StoredReleaseRun` 结构；PageHeader 改为「发布 Run」。

- [ ] **Step 4: 手动验收**

1. 创建 Run 后打开 `/delivery/runs/rel-xxx`
2. prod 节点 `before_deploy`：初始为 `waiting_approval`，未点通过不应变 `deploying`
3. staging `after_deploy`：先 `deploying` 再 `waiting_approval`

---

### Task 8: 全量前端验证

**Files:** 无新增

- [ ] **Step 1: 运行 CD + workspace 相关测试**

```bash
cd fronted
pnpm vitest run apps/web/src/modules/delivery/cd/lib apps/web/src/modules/workspace/components/project/release-flow-gate.test.ts
pnpm --filter @sparkops/web typecheck
```

Expected: 全部 PASS

- [ ] **Step 2: 走通 spec 11 验收（Mock 范围）**

| 验收项 | 操作 |
|--------|------|
| 创建三节点 Flow | Studio 保存 dev→staging→prod |
| 两项目共用 Flow | 两个项目绑定同一 flowId |
| 未绑定不可发布 | 按钮 disabled |
| 向导 2 步 | 无 Pipeline/Fleet 步骤 |
| 已有镜像不触发 Jenkins | source=artifact 时不调 triggerBuild |
| Run 按节点展示 | ReleaseRunBoard 可见三节点状态 |

---

## Phase 2（后续计划，本期不实施细节）

### Task 9: 后端 migration + cd 模块

- `backend/migrations/0000xx_release_flow.up.sql`：`release_flows`, `release_flow_nodes`, `release_runs`
- `ALTER TABLE workspace_app.projects ADD COLUMN release_flow_id uuid`
- Go 领域：`ReleaseFlow`, `ReleaseRun`, 编译器 port

### Task 10: BFF + contracts + web-sdk

- `GET/POST /workspaces/:ws/release-flows`
- `PUT /projects/:id/release-flow`
- `POST /projects/:id/releases`
- `POST /delivery/runs/:runId/nodes/:nodeId/approve`

### Task 11: 真 K8s deploy + workflow 审批

- 节点 deploy 调现有 `ManifestDeployer`
- `approval.manual` 接 `workflow` 模块
- ReleaseRun 持久化到 PostgreSQL，替换 sessionStorage

---

## Spec coverage self-review

| Spec 章节 | Task |
|-----------|------|
| 4 核心模型 | Task 1–2 |
| 5.1 Studio | Task 4 |
| 5.2 项目绑定 | Task 5 |
| 5.3 简化向导 | Task 6 |
| 5.4 Run 观测 | Task 7 |
| 6 数据模型 | Task 9 (Phase 2) |
| 7 API | Task 10 (Phase 2) |
| 8 执行流程 | Task 7 Mock + Task 11 真执行 |
| 11 验收标准 | Task 8 |

无 TBD / TODO 占位。Phase 1 完整可交付 Mock 闭环；Phase 2 单独计划可基于 Task 9–11 展开。

---

**Plan complete and saved to `docs/superpowers/plans/2026-09-03-release-flow.md`.**

两种执行方式：

1. **Subagent-Driven（推荐）** — 每个 Task 派独立 subagent，Task 间做 review，迭代快  
2. **Inline Execution** — 本会话按 Task 顺序直接实现，批次间设检查点  

你选哪一种？若直接开始，建议从 **Task 1（类型 + 校验）** 做起。
