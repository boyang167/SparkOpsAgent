# 工作流执行单 Implementation Plan

> **For agentic workers:** Execute in the current session. Do not commit unless the user asks.

**Goal:** 把「执行工作流」改成接近全屏的分段执行单，提交仍走现有 `branch` / `environment_overrides` / `variables`。

**Architecture:** 抽出纯函数 `buildWorkflowRunForm` 把 stages 编成构建行、部署任务、其他任务；`StartWorkflowRunModal` 用全屏 Modal 渲染分段表格；列表把当前产品的 `projects` 传进去只读展示名称。

**Tech Stack:** React 19、现有 Modal、`globals.css` tokens、Vitest。

## Global Constraints

- 不改后端 API / `PrepareRun`
- 不新增 UI 库；色值只用 CSS 变量
- 不做按服务分支、PR、镜像预览、调试/不缓存、备注
- 不 git commit（除非用户要求）

---

### Task 1: 表单模型纯函数 + 测试

**Files:**
- Create: `fronted/apps/web/src/modules/product/components/workflow-run-form.ts`
- Create: `fronted/apps/web/src/modules/product/components/workflow-run-form.test.ts`

**Produces:**
- `buildWorkflowRunForm(stages, projects) => { stages: RunFormStage[]; pills: { id: string; name: string }[] }`
- `repoLabelFromGitUrl(gitUrl, fallback)`
- `defaultRunBranch(stages)`
- `defaultDeployEnvironmentId(stages)`

- [ ] 测试：构建+部署两阶段、仅构建、无阶段、git_url 解析
- [ ] 实现分组：sparkops-build → buildRows；sparkops-deploy → deployJobs；其余 → otherJobs；空阶段不进 pills

### Task 2: Modal 全屏尺寸 + 执行单样式

**Files:**
- Modify: `fronted/apps/web/src/components/ui/Modal.tsx`（`size` 增加 `"full"`）
- Modify: `fronted/apps/web/app/globals.css`

- [ ] `.modal-panel-full`：max-width min(1100px, 96vw)，height 80vh，flex column
- [ ] `.wf-run-*`：顶栏胶囊、阶段块、表格、底栏

### Task 3: 重写 StartWorkflowRunModal 并接线

**Files:**
- Modify: `fronted/apps/web/src/modules/product/components/StartWorkflowRunModal.tsx`
- Modify: `fronted/apps/web/src/modules/product/components/WorkflowListPanel.tsx`

- [ ] 传入 `projects: ProductProjectVM[]`
- [ ] 胶囊滚动、共用分支、单一环境应用到全部 deploy jobs
- [ ] 运行时变量必填、有部署时环境必选
- [ ] 提交契约不变
