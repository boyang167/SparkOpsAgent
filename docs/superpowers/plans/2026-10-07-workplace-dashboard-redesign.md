# Workplace Dashboard Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将工作台重构为兼具首版完整感与当前克制视觉的“欢迎区 + 核心概览 + 主工作区 + 连续右侧栏”页面。

**Architecture:** 保留 `WorkplaceDashboard` 现有数据接口和路由，只重组页面语义结构与展示条件。主区域负责四项核心指标和进行中工作，右侧连续面板只提供产品、部署、工作区切换和快捷项目，杜绝指标重复。

**Tech Stack:** Next.js 15 App Router、React 19、TypeScript、现有 SparkOps Web SDK、全局 CSS tokens。

## Global Constraints

- 不引入新 UI 库或依赖。
- 不使用渐变、大色块或巨大装饰字符。
- 项目、可发布、迭代、发布计划四项指标只出现一次。
- 附加接口失败不得阻塞其他区域。
- 不改变现有创建项目、项目详情、产品和发布计划路由。
- 当前 Web 应用没有组件测试运行器；本次以 TypeScript、lint 诊断、生产构建和 HTTP 检查验证。

---

### Task 1: 重组工作台语义结构

**Files:**
- Modify: `fronted/apps/web/src/modules/dashboard/components/WorkplaceDashboard.tsx`

**Interfaces:**
- Consumes: `WorkplaceDashboardProps`、现有 `platform`、`product`、`tenant` SDK。
- Produces: `.workplace-welcome`、`.workplace-overview-panel`、`.workplace-context-panel` 三个稳定布局区域。

- [ ] **Step 1: 建立欢迎区**

使用 `session.name` 或邮箱前缀生成显示名称，标题为 `你好，{displayName}`；副标题显示当前空间和工作状态；右侧保留 `/dashboard?create=1`。

- [ ] **Step 2: 将四项指标移入单一概览面板**

把项目、可发布、迭代、发布计划放入 `.workplace-overview-panel`。无进行中工作时只增加一句引导：

```tsx
{!workLoading && !hasWork ? (
  <p className="workplace-overview-hint">从创建第一个项目开始构建当前空间。</p>
) : null}
```

- [ ] **Step 3: 保留非空工作列表**

可发布项目、迭代、发布计划维持现有条件渲染；最近变更继续只使用 `changes`，不得恢复 GET、HEAD、OPTIONS 记录。

- [ ] **Step 4: 合并右侧上下文**

用一个 `.workplace-context-panel` 包裹补充概览、业务空间、快捷项目，三个分区用内部标题和轻分隔组织，不再各自使用 `PanelCard`。

- [ ] **Step 5: 检查展示去重**

搜索并确认右侧只包含“产品、部署”，主概览只包含“项目、可发布、迭代、发布计划”。

Run:

```powershell
rg '产品|部署|可发布|发布计划' fronted/apps/web/src/modules/dashboard/components/WorkplaceDashboard.tsx
```

Expected: 产品和部署只出现在右侧补充概览；四项核心指标只在主概览定义一次，工作列表标题除外。

### Task 2: 建立完整且克制的视觉层级

**Files:**
- Modify: `fronted/apps/web/app/globals.css`

**Interfaces:**
- Consumes: Task 1 输出的 `.workplace-welcome`、`.workplace-overview-panel`、`.workplace-context-panel`。
- Produces: 桌面左主右辅布局，以及 1100px、640px 两级响应式布局。

- [ ] **Step 1: 设置欢迎区**

欢迎区使用透明背景、紧凑标题和单个高对比按钮；不得添加水印、头像方块、渐变或阴影。

- [ ] **Step 2: 设置核心概览面板**

概览面板使用 `var(--surface)`、`var(--shadow-sm)` 和 `var(--radius-lg)`；四项指标横排，以轻量分隔线和留白形成节奏。

- [ ] **Step 3: 设置连续右侧栏**

右侧只保留一个表面容器；内部三段使用 `.workplace-context-section + .workplace-context-section` 的顶部边框分隔。工作区按钮和快捷项目沿用现有 hover 与禁用状态。

- [ ] **Step 4: 降低主列表卡片重量**

主列表容器使用白色表面和轻阴影，行间依靠留白与 hover，不增加饱和色背景。

- [ ] **Step 5: 完成响应式**

1100px 以下改为单列并让右侧面板落到主内容下方；640px 以下欢迎区纵向排列、指标两列。

### Task 3: 验证页面完整性

**Files:**
- Verify: `fronted/apps/web/src/modules/dashboard/components/WorkplaceDashboard.tsx`
- Verify: `fronted/apps/web/app/globals.css`

**Interfaces:**
- Consumes: Tasks 1–2 的最终实现。
- Produces: 编译、样式语法和开发服务器可访问性证据。

- [ ] **Step 1: 检查编辑器诊断**

读取两个修改文件的诊断，预期没有新增错误。

- [ ] **Step 2: 运行 TypeScript 检查**

```powershell
pnpm --dir fronted --filter @sparkops/web typecheck
```

Expected: exit 0；若仅出现已知 `.next/types` 缓存缺失，先重启前端再重跑。

- [ ] **Step 3: 运行生产构建**

确保开发服务器停止或使用独立构建目录后运行：

```powershell
pnpm --dir fronted --filter @sparkops/web build
```

Expected: Next.js 编译与类型检查完成，exit 0。

- [ ] **Step 4: 检查开发服务**

```powershell
curl.exe -s -o NUL -w "%{http_code}" http://localhost:3000/login
curl.exe -s -o NUL -w "%{http_code}" http://localhost:3000/dashboard/workplace
```

Expected: `/login` 返回 200；未登录访问工作台返回 307 到登录页。

- [ ] **Step 5: 人工验收登录态页面**

在已登录浏览器确认桌面与窄屏布局、工作区切换、创建项目、快捷项目链接，以及空数据和有数据两种状态。
