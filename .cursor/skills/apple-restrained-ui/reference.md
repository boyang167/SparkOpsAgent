# Apple 克制设计 — SparkOps 实现参考

## 视觉基准（White Agency 风格）

- 页面底 `#f5f5f7`，主内容区为 `#ffffff` 大圆角漂浮卡片（`page-canvas`，32px 圆角）
- 几乎无边框，模块靠留白 + 轻阴影分离
- 主色近黑 `#1d1d1f`，仅用于主按钮与关键交互
- 输入框/按钮药丸形（`border-radius: 999px`），内凹浅灰背景
- 侧栏透明融入页面底，导航选中态为 `rgba(0,0,0,0.06)` 灰底

## 设计 Token（近乎无色）

```css
/* 背景层 — 大面积极浅 */
var(--bg)           /* #f5f5f7 页面底 */
var(--surface)      /* #ffffff 卡片/面板 */
var(--surface-muted)/* #fafafa 次级区域 */

/* 文字层 — 层级来自字色与字重，非色块 */
var(--text)              /* 主文案 */
var(--text-secondary)    /* 次级说明 */
var(--muted)             /* 辅助/placeholder */

/* 强调 — 仅关键交互 */
var(--primary)           /* 主按钮、链接、选中 */
var(--primary-soft)      /* 极淡选中背景 */

/* 分离感 — 阴影轻、边框弱 */
var(--shadow-sm)   /* 0 1px 2px rgba(15,23,42,0.04) */
var(--shadow-md)   /* 0 8px 24px rgba(15,23,42,0.06) */
var(--border)      /* #e2e8f0 — 必要时 1px */
```

禁止：`#409eff`、内联 hex、高饱和背景色块。

## 空间层级（Z 轴 > X/Y 轴）

### 页面骨架

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

- 外层 `stack-lg`（1.75rem）— 模块间「空气」
- 面板内 `stack-md`（1.25rem）
- 字段组 `stack-sm`（0.75rem）

### 表面 elevation

| 层级 | 类/Token | 场景 |
|------|----------|------|
| 0 | 无 shadow，融入 `--bg` | 页面背景 |
| 1 | `.card` + `--shadow-sm` | 内容面板 |
| 2 | `.card-hover` hover 微抬升 | 可点击卡片 |
| 3 | `Modal` / `Drawer` + `--shadow-md` | 浮层 |

避免：`.card` 内再套带 border 的 `.card` 三层嵌套。

## 内容优先 — 组件模式

### 单职责卡片

```tsx
<section className="card stack-md">
  <div className="section-header">
    <h2 className="section-title">最近发布</h2>
    <button className="btn btn-ghost text-sm">查看全部</button>
  </div>
  {/* 仅列表内容，无多余装饰 */}
  <ReleaseList items={items} />
</section>
```

### 空态 — 文案 + 一个动作

```tsx
<div className="empty-state stack-sm">
  <p className="text-muted">暂无发布计划</p>
  <button className="btn btn-primary text-sm">新建发布计划</button>
</div>
```

不写：大插画 + 三段说明 + 两个 primary 按钮。

## 克制用色 — MetricCard

```tsx
{/* tone 按语义，但同屏不超过 2 种 accent */}
<MetricCard label="运行中" value="12" tone="sky" />
<MetricCard label="异常" value="1" tone="amber" />
```

指标区 3–4 张即可；数值是主角，卡片背景保持白/浅灰。

## 图标规范

```tsx
import { Plus, ChevronRight } from "lucide-react";

{/* 行内辅助 — muted，不抢文案 */}
<Plus className="h-4 w-4 text-[var(--muted)]" aria-hidden />

{/* 图标按钮 — 必须 aria-label */}
<button className="btn btn-ghost" aria-label="更多操作">
  <ChevronRight className="h-4 w-4" />
</button>
```

- 尺寸：16px（h-4）行内，20px（h-5）工具栏
- 颜色：`--muted` 或 `--text-secondary`，非 `--primary` 除非可点击

## 动效 — 呼吸感

```css
/* globals 已有模式 — 沿用 */
.card-hover {
  transition: box-shadow 150ms ease, transform 150ms ease;
}
.btn {
  transition: background-color 150ms ease, border-color 150ms ease;
}
```

| 属性 | 值 |
|------|-----|
| duration | 150ms（最大 200ms） |
| easing | ease 或 ease-out |
| 禁止 | bounce, spring 夸张, >300ms, 大面积 gradient 动画 |

Modal 打开：淡入 + 轻微 scale(0.98→1)，不用 slide-from-bottom 全屏。

## 边框使用决策树

```
需要分隔两个区域？
├── 是同级模块 → stack-md 间距，不加 border
├── 是表格行 → tbody hover 背景，不用行 border
├── 是输入框 → 1px var(--border)，focus 时 primary 描边
└── 是侧栏/固定 chrome → 允许 1px var(--border)（sidebar 等）
```

## Do / Don't 对照

**Do**
- 白底面板 + 24px 间距 + 轻 shadow 浮在 `--bg` 上
- 标题 `section-title`，说明 `text-muted`，一行即止
- 表格 thead `surface-muted` 背景，无竖线

**Don't**
- 每个 section 不同背景色
- 用 `border-l-4 border-primary` 做左侧强调条
- 状态用 5 种彩色 badge 同屏
- empty-state 放 120px 插画占半屏

## 与 fronted skill 的关系

| 本 skill | fronted skill |
|----------|---------------|
| **为什么** — 设计哲学与审美方向 | **怎么做** — 路径、组件、三态、表单表格 |
| 五维方法论、自检、反模式 | PageHeader、Modal、K8s 套件、Checklist |

两者同时适用：先方法论定调，再 fronted 落地。
