---
name: fronted
description: Builds SparkOps enterprise frontend UI with refined visual taste, friendly interaction, and consistent design tokens. Use when creating or editing pages, panels, forms, tables, modals, or components under fronted/, or when the user asks for frontend UI, UX polish, or enterprise-grade design.
---

# SparkOps 企业级前端 UI

> **设计哲学：** 先读 [apple-restrained-ui/SKILL.md](../apple-restrained-ui/SKILL.md)（Apple 克制方法论：内容优先、近乎无色、空间层级、呼吸感动效），再按本文档落地组件与交互。

## 设计目标

交付**企业级控制台**体验：信息层次清晰、操作可预期、视觉克制但有品位。避免「后台模板感」和「堆组件感」——每个页面应有明确的视觉焦点与呼吸感。

**品位感来自：**
- 留白与对齐优先于装饰
- 单一主色 + 中性灰阶，语义色仅用于状态
- 微妙的阴影、圆角、hover 过渡（150ms ease）
- 文案简洁、动作动词开头（「新建产品」而非「产品创建」）

## 技术栈与路径

| 项 | 约定 |
|---|---|
| 应用根目录 | `fronted/apps/web/` |
| 页面 | `app/(platform)/**/page.tsx` |
| 业务组件 | `src/modules/<domain>/components/` |
| 通用 UI | `src/components/ui/` |
| 全局样式 / Token | `app/globals.css` |
| K8s 专项 | `src/components/k8s/` + 规则 `.cursor/rules/k8s-enterprise-ui.mdc` |

优先复用现有组件与 CSS 类，**禁止**引入新 UI 库或内联硬编码色值。

## 设计 Token（必须使用）

```css
var(--bg) var(--surface) var(--surface-muted)
var(--text) var(--text-secondary) var(--muted)
var(--primary) var(--primary-hover) var(--primary-soft)
var(--border) var(--border-strong)
var(--shadow-sm) var(--shadow-md) var(--shadow-lg)
var(--radius-sm) var(--radius-md) var(--radius-lg)
```

禁止：`#409eff`、`#f56c6c` 等散落色值；状态色沿用 globals 中已有 badge / alert 语义类。

## 页面骨架

标准列表/管理页结构：

```
stack-lg
├── PageHeader（eyebrow + title + description + actions）
└── Panel / section（工具栏 → 内容 → 分页）
```

```tsx
<div className="stack-lg">
  <PageHeader
    eyebrow="Product"
    title="产品管理"
    description="管理工作空间下的产品及其关联项目"
    actions={<button className="btn btn-primary text-sm">新建</button>}
  />
  <ProductsPanel />
</div>
```

- 平台运营内嵌页用 `PlatformPanelHead`，避免与外层 `PageHeader` 重复层级
- 垂直间距：`stack-lg`（页面）→ `stack-md`（面板内）→ `stack-sm`（表单字段组）

## 共享组件（优先使用）

路径：`src/components/ui/`

| 场景 | 组件 |
|------|------|
| 页头 | `PageHeader` |
| 内嵌标题 | `PlatformPanelHead` |
| 面包屑 | `Breadcrumb` |
| 指标卡 | `MetricCard`（tone: sky/violet/emerald/amber） |
| 对话框 | `Modal` |
| 抽屉 | `Drawer` |
| 行内提示 | `InlineAlert` |
| 占位 | `ComingSoonPanel` |

K8s / 集群资源页必须使用 `src/components/k8s/` 套件，详见 [reference.md](reference.md)。

## 布局与表面

| 类名 | 用途 |
|------|------|
| `.card` / `.section` | 白色面板，带边框与轻阴影 |
| `.card-hover` | 可点击卡片，hover 微抬升 |
| `.section-header` | 区块标题行（标题 + 右侧操作） |
| `.table-wrap` + `.data-table` | 数据表格 |
| `.empty-state` | 空数据（虚线边框 + muted 文案） |

工具栏：主操作 `btn btn-primary`，次要 `btn-secondary`，低优先级 `btn-ghost`。

## 交互规范

### 必须做

1. **加载 / 空 / 错 三态分离** — 禁止「加载中…」与「暂无数据」共用同一文案或组件状态
2. **危险操作二次确认** — 用 `Modal`，禁止 `window.confirm` / `window.alert`
3. **异步反馈** — 提交时 `disabled` + 按钮文案变化（「保存中…」）；失败用 `InlineAlert tone="error"`
4. **键盘与无障碍** — Modal 支持 Esc；加载区 `role="status"`；图标按钮加 `aria-label`
5. **可点击行/卡** — hover 有视觉反馈（边框/阴影/背景），cursor pointer

### 禁止做

- 同一屏多个 competing 主按钮（最多一个 `btn-primary` 视觉焦点）
- 无间距堆叠表单项（用 `stack-sm` 或 `.form-row`）
- 原生浏览器对话框
- 硬编码 px 色值替代 token
- 过度动画（bounce、长 duration、大面积 gradient 背景）

##  typography

| 类名 | 用途 |
|------|------|
| `.eyebrow` | 小标签（全大写、letter-spacing） |
| `.page-title` | 页面 H1 |
| `.section-title` | 区块标题 |
| `.text-muted` | 辅助说明 |

中文界面：标题用名词短语，描述用完整句；错误信息说明原因 + 可行动建议。

## 表单

- 标签在上、输入在下；必填用文案或 `*` 标注，不单靠颜色
- 校验错误贴近字段，提交级错误用 `InlineAlert`
- 长表单分 `section` 区块，每块有 `.section-title`
- Modal 内表单：标题 + 可选 description + 字段 + footer（取消 secondary / 确认 primary）

## 表格

```
table-wrap → data-table
├── thead：muted 背景、左对齐
├── tbody：hover 行背景
└── 空态/加载：td colSpan 居中
```

行内操作：ghost 按钮或文字链，destructive 用现有 danger 语义类，不发明新红色。

## 美感自检（交付前过一遍）

```
- [ ] 第一屏能回答：我在哪？能做什么？关键数据是什么？
- [ ] 视觉层级：eyebrow < description < section-title < page-title
- [ ] 主色只出现在：主按钮、链接、选中态、关键指标
- [ ] 间距是否一致（stack-* 而非随意 margin）
- [ ] hover/focus 状态完整，disabled 态可识别
- [ ] 空态有引导（如「新建产品」按钮），不是冷冰冰一句「无数据」
- [ ] 移动端/窄屏：table-wrap 可横向滚动，工具栏 flex-wrap
```

## 新增页面 Checklist

1. `loadPlatformPageContext` 等 loader 取上下文（服务端 page）
2. `PageHeader` + `stack-lg` 外层
3. 业务逻辑放 `src/modules/*/components/*Panel.tsx`
4. 三态 + 错误处理完整
5. K8s 相关页面额外遵循 `k8s-enterprise-ui` 规则

## 延伸阅读

- **设计方法论（优先）：** [apple-restrained-ui/SKILL.md](../apple-restrained-ui/SKILL.md)
- K8s 与专项组件清单：[reference.md](reference.md)
- 设计 Token 与全局类完整定义：`fronted/apps/web/app/globals.css`
