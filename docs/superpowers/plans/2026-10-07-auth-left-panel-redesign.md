# 登录页左侧品牌区 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将登录页左侧深色功能卡片区改为浅色、克制的产品叙事区。

**Architecture:** 保留现有认证路由和右侧表单，仅替换 `AuthLayout` 左侧的静态展示结构，并在全局样式中重写 `.auth-brand*` 规则。产品流程预览只使用语义化 HTML、内联 SVG 和 CSS，不引入依赖或认证状态。

**Tech Stack:** Next.js 15、React 19、TypeScript、CSS

## Global Constraints

- 使用现有中性色设计 Token，不新增 UI 库。
- 不使用渐变、彩色光晕和大面积深色背景。
- 登录按钮保持页面唯一高对比操作。
- 视口小于 960px 时隐藏左侧品牌区。
- 不修改登录、注册、SSO 或认证数据流。

---

### Task 1: 重构品牌区内容结构

**Files:**
- Modify: `fronted/apps/web/app/(auth)/layout.tsx`

**Interfaces:**
- Consumes: `AuthLayout({ children }: { children: React.ReactNode })`
- Produces: 保持相同组件签名，仅输出新的 `.auth-brand-*` 展示结构

- [ ] **Step 1: 替换左侧品牌区**

移除 `auth-brand-glow` 和三张功能卡，保留品牌标识，并新增：

```tsx
<div className="auth-brand-story">
  <p className="auth-brand-kicker">Private Agent Platform</p>
  <h1 className="auth-brand-title">构建、交付、治理<br />企业级 Agent</h1>
  <p className="auth-brand-desc">
    在一个统一控制台中连接团队、运行环境与交付流程。
  </p>
</div>
<ol className="auth-brand-flow" aria-label="Agent 生命周期">
  {["构建", "发布", "运行", "观测"].map((label, index) => (
    <li className="auth-brand-flow-step" key={label}>
      <span className="auth-brand-flow-index">{String(index + 1).padStart(2, "0")}</span>
      <span>{label}</span>
    </li>
  ))}
</ol>
```

底部输出三个纯文本信任项：“私有化部署”“企业身份接入”“审计治理”。

- [ ] **Step 2: 运行类型检查**

Run: `pnpm --filter @sparkops/web typecheck`

Expected: 不新增 `app/(auth)/layout.tsx` 类型错误。

### Task 2: 重写浅色品牌区样式

**Files:**
- Modify: `fronted/apps/web/app/globals.css:186-354`

**Interfaces:**
- Consumes: Task 1 新增的 `.auth-brand-story`、`.auth-brand-flow`、`.auth-brand-trust` 类名
- Produces: 桌面端浅色双栏布局和小于 960px 的单栏布局

- [ ] **Step 1: 删除旧装饰样式**

删除以下不再使用的规则：

```css
.auth-brand-glow
.auth-brand-glow-a
.auth-brand-glow-b
.auth-brand-features
.auth-brand-feature
.auth-brand-feature-icon
.auth-brand-feature-title
.auth-brand-feature-desc
```

- [ ] **Step 2: 实现浅色表面与文字层级**

品牌区使用 `var(--surface)`、`var(--text)`、`var(--text-secondary)` 和 `var(--muted)`；标题最大宽度 32rem，字号使用 `clamp(2.5rem, 4.2vw, 4.75rem)`，并保持左侧内容的明确对齐轴。

- [ ] **Step 3: 实现生命周期流程**

流程使用四列开放布局；序号为 muted 小字，节点仅在 hover 时出现 `rgba(0, 0, 0, 0.025)` 背景。节点之间使用留白，不使用彩色边框或箭头。

- [ ] **Step 4: 实现响应式与减弱动画**

在现有 `@media (min-width: 960px)` 下显示品牌区；窄屏沿用 `.auth-panel-mobile-brand`。为交互元素添加不超过 180ms 的过渡，并在 `prefers-reduced-motion: reduce` 下关闭过渡。

- [ ] **Step 5: 验证 CSS 与构建**

Run: `pnpm --filter @sparkops/web build`

Expected: PostCSS 不报告语法错误；Next.js 构建不新增本次变更导致的失败。

- [ ] **Step 6: 检查改动范围**

Run: `git diff -- fronted/apps/web/app/(auth)/layout.tsx fronted/apps/web/app/globals.css`

Expected: 仅认证布局左侧结构和对应样式发生变化，右侧认证表单逻辑未修改。
