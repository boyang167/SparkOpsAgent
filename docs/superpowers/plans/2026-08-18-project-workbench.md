# Project Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 第 1 步创建项目后跳转到项目工作台详情页；详情页采用参考站风格的进度卡片，并保留项目成员、Agents、Workflows。

**Architecture:** 创建向导第 1 步成功后立即 `createProject` 并跳转 `/projects/{id}`。详情页顶部新增 `ProjectWorkbenchCard`（进度/统计/继续配置），配置通过 `?configure=1` 打开可复用的配置面板（基于现有 Step2 + 模板选择），保存走 `updateProject`。成员/Agents/Workflows 保持现有 section。

**Tech Stack:** Next.js App Router、React Client Components、`@sparkops/web-sdk`、现有 `globals.css`

## Global Constraints

- 不新增审核状态持久化；「提交审核」「审核完成」仅展示为未开始。
- 配置完成判定：`git_url` + `git_ref_value` + `dockerfile_path` + `template_id` 均已填写。
- 保留现有 `ProjectMembersPanel`、`AgentList`/`CreateAgentForm`、`WorkflowList`/`CreateWorkflowForm`/`WorkflowSettings`。
- 第 1 步后不再在创建弹窗内进入第 2/3 步。
- UI 文案使用中文。

---

### Task 1: 进度计算工具

**Files:**
- Create: `fronted/apps/web/src/modules/workspace/components/project/project-progress.ts`
- Modify: `fronted/apps/web/src/modules/workspace/components/project/project-utils.ts`（导出进度相关常量如需）

**Interfaces:**
- Produces:
  - `ProjectProgressStep = { key: string; title: string; subtitle: string; status: "done" | "active" | "pending" }`
  - `computeProjectProgress(project: ProjectVM): { steps: ProjectProgressStep[]; configComplete: boolean; statusLabel: string }`

- [ ] **Step 1: 新增进度计算函数**

```ts
import type { ProjectVM } from "@sparkops/contracts/workspace";

export type ProgressStatus = "done" | "active" | "pending";

export interface ProjectProgressStep {
  key: "created" | "configured" | "submitted" | "approved";
  title: string;
  subtitle: string;
  status: ProgressStatus;
}

export function isProjectConfigComplete(project: Pick<ProjectVM, "git_url" | "git_ref_value" | "dockerfile_path" | "template_id">) {
  return Boolean(
    project.git_url?.trim() &&
      project.git_ref_value?.trim() &&
      (project.dockerfile_path?.trim() || "./Dockerfile") &&
      project.template_id?.trim(),
  );
}

export function computeProjectProgress(project: ProjectVM): {
  steps: ProjectProgressStep[];
  configComplete: boolean;
  statusLabel: string;
} {
  const configComplete = isProjectConfigComplete(project);
  const steps: ProjectProgressStep[] = [
    { key: "created", title: "创建项目", subtitle: "基本信息", status: "done" },
    {
      key: "configured",
      title: "配置完成",
      subtitle: "项目设置",
      status: configComplete ? "done" : "active",
    },
    { key: "submitted", title: "提交审核", subtitle: "等待审核", status: "pending" },
    { key: "approved", title: "审核完成", subtitle: "审核通过", status: "pending" },
  ];
  return {
    steps,
    configComplete,
    statusLabel: configComplete ? "已配置" : "创建中",
  };
}
```

- [ ] **Step 2: 手工校验（无独立单测文件时）**

在 Node/浏览器控制台或临时脚本验证：

```ts
// empty git -> configComplete false, step2 active
// all fields filled -> configComplete true, step2 done
```

Expected: 逻辑符合上表。

- [ ] **Step 3: Commit（仅当用户要求提交时执行；否则跳过）**

```bash
git add fronted/apps/web/src/modules/workspace/components/project/project-progress.ts
git commit -m "feat: add project progress helpers for workbench"
```

---

### Task 2: 项目工作台卡片组件

**Files:**
- Create: `fronted/apps/web/src/modules/workspace/components/project/ProjectWorkbenchCard.tsx`
- Modify: `fronted/apps/web/app/globals.css`（追加 `.project-workbench-*` 样式）

**Interfaces:**
- Consumes: `computeProjectProgress` from Task 1
- Produces: `ProjectWorkbenchCard({ project, onContinueConfig }: { project: ProjectVM; onContinueConfig: () => void })`

- [ ] **Step 1: 实现卡片 UI（对齐参考图）**

结构：

```tsx
"use client";

import type { ProjectVM } from "@sparkops/contracts/workspace";
import {
  languageColor,
  languageLabel,
  projectDisplayName,
} from "./project-utils";
import { computeProjectProgress } from "./project-progress";
import { formatDateTime } from "./project-utils";

interface ProjectWorkbenchCardProps {
  project: ProjectVM;
  onContinueConfig: () => void;
}

export function ProjectWorkbenchCard({ project, onContinueConfig }: ProjectWorkbenchCardProps) {
  const { steps, statusLabel } = computeProjectProgress(project);
  const langColor = languageColor(project.language);
  const initial = languageLabel(project.language).charAt(0).toUpperCase() || "P";

  return (
    <article className="project-workbench-card">
      <header className="project-workbench-header">
        <div className="project-workbench-title-wrap">
          <div className="project-lang-icon" style={{ backgroundColor: langColor }}>{initial}</div>
          <div>
            <h2>{projectDisplayName(project)}</h2>
            <p className="project-card-subtitle">{project.name}</p>
            <div className="project-card-tags">
              {project.namespace ? <span className="project-tag project-tag-gray">{project.namespace}</span> : null}
              <span className="project-tag project-tag-gray">草稿</span>
              <span className={`project-tag ${statusLabel === "创建中" ? "project-tag-orange" : "project-tag-green"}`}>
                {statusLabel}
              </span>
            </div>
          </div>
        </div>
      </header>

      <section className="project-workbench-progress">
        <div className="project-workbench-progress-title">项目进度</div>
        <ol className="project-workbench-steps">
          {steps.map((step) => (
            <li key={step.key} className={`project-workbench-step is-${step.status}`}>
              <div className="project-workbench-step-circle">
                {step.status === "done" ? "✓" : steps.findIndex((s) => s.key === step.key) + 1}
              </div>
              <div>
                <strong>{step.title}</strong>
                <span>{step.subtitle}</span>
              </div>
            </li>
          ))}
        </ol>
      </section>

      <div className="project-workbench-stats">
        <div><span>部署次数</span><strong>0</strong></div>
        <div><span>构建次数</span><strong>0</strong></div>
        <div><span>最后部署</span><strong>N/A</strong></div>
      </div>

      <footer className="project-workbench-footer">
        <span className="text-muted">创建于 {formatDateTime(project.created_at)}</span>
        <div className="project-workbench-actions">
          <a href="#project-relations" className="btn-secondary">查看详情</a>
          <button type="button" className="btn project-btn-primary" onClick={onContinueConfig}>
            继续配置
          </button>
        </div>
      </footer>
    </article>
  );
}
```

- [ ] **Step 2: 追加样式**

在 `globals.css` 增加：

```css
.project-tag-orange {
  background: #fdf6ec;
  color: #e6a23c;
}
.project-workbench-card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 1.25rem 1.25rem 1rem;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
}
.project-workbench-progress {
  margin-top: 1rem;
  padding: 0.875rem 1rem;
  background: #f5f7fa;
  border-radius: 6px;
}
.project-workbench-steps {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.5rem;
  list-style: none;
  margin: 0.75rem 0 0;
  padding: 0;
}
.project-workbench-step.is-done { color: #67c23a; }
.project-workbench-step.is-active { color: #303133; font-weight: 600; }
.project-workbench-step.is-pending { color: #909399; }
.project-workbench-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  margin-top: 1rem;
  text-align: center;
}
.project-workbench-stats strong { display: block; color: #409eff; font-size: 1.25rem; }
.project-workbench-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 1rem;
  padding-top: 0.75rem;
  border-top: 1px solid #ebeef5;
}
```

- [ ] **Step 3: 类型检查**

Run: `cd fronted/apps/web; pnpm exec tsc --noEmit`
Expected: 无新增错误

---

### Task 3: 项目配置面板（继续配置）

**Files:**
- Create: `fronted/apps/web/src/modules/workspace/components/project/ProjectConfigurePanel.tsx`
- Reuse: `ProjectCreateStep2.tsx`、`project-form-shared.tsx`、`product.listTemplates` / `secret.listSecrets`

**Interfaces:**
- Consumes: `ProjectVM`, `workspace.updateProject`, `ProjectCreateStep2`
- Produces: `ProjectConfigurePanel({ workspaceId, project, open, onClose, onSaved })`

- [ ] **Step 1: 实现配置面板**

行为：

1. `open=true` 时用 Modal 或页面内 panel 展示。
2. 表单从 `project` 初始化（`git_*`、`resources`、`template_id` 等）。
3. 两段 UI：配置信息（复用 `ProjectCreateStep2`）+ 模板选择。
4. 保存按钮调用：

```ts
await workspace.updateProject(workspaceId, project.id, {
  name: project.name,
  display_name: project.display_name,
  description: project.description,
  namespace: project.namespace,
  language: form.language,
  git_url: form.git_url,
  secret_id: form.secret_id || undefined,
  git_ref_type: form.git_ref_type,
  git_ref_value: form.git_ref_value,
  dockerfile_path: form.dockerfile_path || "./Dockerfile",
  template_id: form.template_id || undefined,
  resources: form.resources,
});
```

5. 成功后 `onSaved()` → `router.refresh()` 并关闭。

推荐用现有 `Modal`，`size="xl"`，标题「继续配置」。

- [ ] **Step 2: 校验**

- Git URL / 引用值必填才可保存（与创建第 2 步一致）。
- 模板可选，但「配置完成」进度需要模板；可在提示中说明。

- [ ] **Step 3: Typecheck**

Run: `cd fronted/apps/web; pnpm exec tsc --noEmit`
Expected: PASS

---

### Task 4: 创建向导第 1 步后跳转

**Files:**
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectCreateWizard.tsx`

**Interfaces:**
- Consumes: `workspace.createProject`
- Produces: 第 1 步成功后 `router.push(/projects/{id})`，关闭弹窗

- [ ] **Step 1: 修改 `handleNext`**

当 `step === 1`：

```ts
async function handleNext() {
  if (step === 1) setNameTouched(true);
  const message = validateStep(step);
  if (message) {
    setError(message);
    return;
  }
  setSubmitting(true);
  setError(null);
  try {
    if (step === 1) {
      const project = await persistProject();
      onSuccess?.(project);
      onClose();
      router.push(`/projects/${project.id}`);
      router.refresh();
      return;
    }
    await persistProject();
    setStep((s) => Math.min(3, s + 1));
  } catch (err) {
    setError(err instanceof BffError ? err.message : "保存失败，请稍后重试");
  } finally {
    setSubmitting(false);
  }
}
```

- [ ] **Step 2: 简化向导**

创建弹窗只保留第 1 步 UI（或仍渲染 STEPS 但隐藏第 2/3 步入口）。推荐：

- STEPS 可改为仅显示「填写项目基本信息」或保留三步视觉但第 1 步按钮文案改为「创建并进入」/「下一步」。
- 按规格：按钮仍可叫「下一步」，成功后跳转。

删除弹窗内对第 2/3 步的依赖路径（`ProjectCreateStep2` 可仍被配置面板引用）。

- [ ] **Step 3: 手动验证**

1. 打开创建项目，填合法英文名，点下一步。
2. 应创建成功并进入 `/projects/{uuid}`。
3. 非法英文名仍阻断。

---

### Task 5: 重组项目详情页

**Files:**
- Modify: `fronted/apps/web/app/(platform)/projects/[projectId]/page.tsx`
- Create (可选 client shell): `fronted/apps/web/src/modules/workspace/components/project/ProjectWorkbenchPage.tsx`

**Interfaces:**
- Consumes: `ProjectWorkbenchCard`, `ProjectConfigurePanel`, 现有 members/agents/workflows
- Produces: 完整工作台页面

- [ ] **Step 1: 新增客户端壳组件**

```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import type { AgentVM } from "@sparkops/contracts/agent"; // 按实际类型
import type { WorkflowVM } from "@sparkops/contracts/workflow"; // 按实际类型
import { ProjectWorkbenchCard } from "./ProjectWorkbenchCard";
import { ProjectConfigurePanel } from "./ProjectConfigurePanel";
import { ProjectMembersPanel } from "../ProjectMembersPanel";
import { EditProjectForm, DeleteProjectButton } from "../ProjectSettings";
// AgentList / CreateAgentForm / Workflow* 从原 page 迁移 props

export function ProjectWorkbenchPage(props: {
  workspaceId: string;
  project: ProjectVM;
  agents: /* existing */;
  workflows: /* existing */;
  initialConfigure?: boolean;
}) {
  const router = useRouter();
  const [configureOpen, setConfigureOpen] = useState(Boolean(props.initialConfigure));

  return (
    <div className="stack-lg">
      <ProjectWorkbenchCard
        project={props.project}
        onContinueConfig={() => setConfigureOpen(true)}
      />
      <div className="flex flex-wrap gap-2 justify-end">
        <EditProjectForm workspaceId={props.workspaceId} project={props.project} />
        <DeleteProjectButton
          workspaceId={props.workspaceId}
          projectId={props.project.id}
          projectName={props.project.name}
        />
      </div>
      <div id="project-relations" className="stack-lg">
        <ProjectMembersPanel workspaceId={props.workspaceId} projectId={props.project.id} />
        {/* Agents / Workflows sections 原样保留 */}
      </div>
      <ProjectConfigurePanel
        open={configureOpen}
        workspaceId={props.workspaceId}
        project={props.project}
        onClose={() => setConfigureOpen(false)}
        onSaved={() => {
          setConfigureOpen(false);
          router.refresh();
        }}
      />
    </div>
  );
}
```

- [ ] **Step 2: 改写 `page.tsx`**

Server Component 继续 `loadProject` / `loadAgents` / `loadWorkflows`，渲染 `ProjectWorkbenchPage`。返回链接改为 `/projects`。支持 searchParams `configure=1` 初始打开配置。

- [ ] **Step 3: Typecheck + 手动联调**

Run: `cd fronted/apps/web; pnpm exec tsc --noEmit`

手动：

1. 创建第 1 步 → 进入工作台，进度「创建项目」完成、「配置完成」进行中，标签「创建中」。
2. 点「继续配置」保存 Git+模板 → 刷新后进度「配置完成」完成，标签「已配置」。
3. 项目成员可添加；Agents/Workflows 可创建与列表。

---

### Task 6: 列表卡片状态对齐（轻量）

**Files:**
- Modify: `fronted/apps/web/src/modules/workspace/components/project/ProjectCard.tsx`

- [ ] **Step 1: 列表卡片增加「创建中/已配置」标签**

使用 `computeProjectProgress(project).statusLabel`，与工作台一致；按钮「进入项目」保持；可选增加「继续配置」链接：`/projects/${id}?configure=1`。

- [ ] **Step 2: Typecheck**

Run: `cd fronted/apps/web; pnpm exec tsc --noEmit`

---

## Spec Coverage Checklist

| Spec 要求 | Task |
|-----------|------|
| 第 1 步保存后跳转详情 | Task 4 |
| 工作台卡片 UI（进度/统计/操作） | Task 2 |
| 继续配置 updateProject | Task 3 |
| 进度计算规则 | Task 1 |
| 成员/Agents/Workflows 保留 | Task 5 |
| 创建失败不跳转 | Task 4 |
| 列表侧状态可选对齐 | Task 6 |

## Self-Review Notes

- 无 TBD。
- 「提交审核/审核完成」仅 UI pending，符合规格。
- `ProjectCreateStep2` 从创建弹窗解耦后由配置面板复用，避免重复表单。
