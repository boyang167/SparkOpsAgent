# Storage Pool Member Creation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 允许存储管理员在创建存储池时配置多个 Provider 成员、角色和优先级，并阻止提交不能参与 Placement 的无效池。

**Architecture:** 在现有 `storage-pool-panel-view.ts` 中新增纯函数，负责默认草稿、校验与 API 命令构造；`StoragePoolPanel.tsx` 只负责成员行交互和调用现有 `createStoragePool`。后端及契约保持不变。

**Tech Stack:** React、TypeScript、Vitest、现有 `@sparkops/contracts/filesystem` 与 `@sparkops/web-sdk/filesystem`

## Global Constraints

- 仅允许选择状态为 `ACTIVE` 的 Provider。
- 至少配置一个 `PRIMARY`。
- 同一 Provider 不可重复。
- 多个 `PRIMARY` 的优先级不可重复。
- 不新增依赖，不修改后端 API，不实现创建后的成员编辑。

---

### Task 1: 存储池草稿、校验与命令构造

**Files:**
- Modify: `fronted/apps/web/src/modules/filesystem/components/storage-pool-panel-view.ts`
- Test: `fronted/apps/web/src/modules/filesystem/components/StoragePoolPanel.test.tsx`

**Interfaces:**
- Produces: `StoragePoolDraftMember`
- Produces: `createDefaultStoragePoolMembers(providers): StoragePoolDraftMember[]`
- Produces: `validateCreateStoragePoolForm(name, members, providers): string | null`
- Produces: `normalizeCreateStoragePoolCommand(name, members): CreateStoragePoolCommand`

- [ ] **Step 1: Write failing tests**

测试默认选择第一个 ACTIVE Provider 为 `PRIMARY/100`，并覆盖空名称、空成员、重复 Provider、缺少 PRIMARY、重复 PRIMARY 优先级以及成功命令构造。

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/StoragePoolPanel.test.tsx
```

Expected: FAIL because the new functions are not exported.

- [ ] **Step 3: Implement minimal pure functions**

草稿成员结构：

```ts
export type StoragePoolDraftMember = {
  provider_id: string;
  mode: StoragePoolMemberVM["mode"];
  priority: number;
};
```

校验顺序固定为名称、可用成员、重复 Provider、PRIMARY 存在性、PRIMARY 优先级冲突。命令构造只做名称 trim 和成员字段复制。

- [ ] **Step 4: Run the focused test and verify GREEN**

Expected: all `StoragePoolPanel.test.tsx` tests pass.

---

### Task 2: 新增存储池多成员表单

**Files:**
- Modify: `fronted/apps/web/src/modules/filesystem/components/StoragePoolPanel.tsx`

**Interfaces:**
- Consumes Task 1 pure functions.
- Submits the existing `CreateStoragePoolCommand` through `createStoragePool`.

- [ ] **Step 1: Replace `poolName` with pool draft state**

维护 `poolName`、`poolMembers` 和 `poolFormError`；打开弹窗时调用 `createDefaultStoragePoolMembers(providers)`。

- [ ] **Step 2: Render member rows**

每行提供 Provider 下拉、角色下拉、数字优先级和删除按钮；仅列出 ACTIVE Provider。提供“添加成员”次要按钮，新增行默认选尚未使用的 Provider，角色为 `REPLICA`，优先级为 `0`。

- [ ] **Step 3: Wire validation and submission**

提交时调用 `validateCreateStoragePoolForm`；通过后调用 `normalizeCreateStoragePoolCommand`。无 ACTIVE Provider 时显示“请先注册可用 Provider”，且创建按钮禁用。

- [ ] **Step 4: Verify component tests and typecheck**

Run:

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/StoragePoolPanel.test.tsx
pnpm --filter web typecheck
```

Expected: tests and typecheck exit 0.

- [ ] **Step 5: Check edited-file lint diagnostics**

Expected: no new diagnostics in `StoragePoolPanel.tsx` and `storage-pool-panel-view.ts`.

---

### Task 3: Final regression verification

**Files:**
- Verify only; do not expand scope.

- [ ] **Step 1: Run filesystem component tests**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components
```

Expected: all filesystem component test files pass.

- [ ] **Step 2: Inspect diff**

Confirm every changed line traces to Provider registration UX, semantic scope labels, or storage pool member creation; report the pre-existing Git 2.31 `--trailer` incompatibility if commit remains unavailable.
