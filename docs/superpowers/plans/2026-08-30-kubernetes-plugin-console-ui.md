# Kubernetes Plugin Console UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 侧栏展开 Kubernetes 子菜单，集群页与资源页对齐 GVA 管理台布局。

**Architecture:** 纯前端：从动态菜单树提升三级插件节点为 NavGroup；集群表与配置弹窗复用现有 BFF；不改平台集群账本。

**Tech Stack:** Next.js App Router、现有 `NavGroup`/`data-table`/`Modal`、Vitest。

## Global Constraints

- 不嵌入 Vue；视觉用 SparkOps token。
- 不在插件内新增/删除集群。
- 配置写操作仅 `governance:admin`。

---

### Task 1: 菜单树转导航分组

**Files:**
- Create: `fronted/apps/web/src/components/layout/menu-tree-nav.ts`
- Create: `fronted/apps/web/src/components/layout/menu-tree-nav.test.ts`
- Modify: `fronted/apps/web/src/components/layout/PlatformNav.tsx`

- [x] Promote nested plugin roots to sidebar groups
- [x] Redirect `/plugins/kubernetes` to clusters
- [x] Clusters table + settings modal
- [x] Compact picker on other pages
