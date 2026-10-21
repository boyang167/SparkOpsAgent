---
name: apple-restrained-ui
description: Applies Apple-inspired restrained UI design methodology (content-first, near-colorless, spatial hierarchy, subtle motion) to SparkOps frontend. Use when designing or building pages, panels, forms, cards, or polishing UI under fronted/, or when the user mentions Apple-style UI, content-first design, restrained aesthetics, or 克制设计.
---

# Apple 克制设计方法论

> **核心观点：这套思路，比追风格重要得多。**（强调方法论大于视觉表象）

## 本质认知

这种 UI 风格不是「没设计」，而是**设计被极度克制了**。它本质上是在向 Apple 的设计方法论致敬。

UI 的精髓在于极度克制——通过留白、空间层级、近乎无色的搭配和克制的动效，让 UI 退居幕后，让内容本身成为绝对的主角。

---

## 五维方法论

### 1. 内容优先（Content First）

**原则：** UI 为内容服务，不为了炫技。无大色块、无夸张渐变、无复杂图形，但信息层级清楚。

**方法论：**
- 每个卡片只解决一件事
- 文案是主角，UI 是空气
- 「如果一个元素删掉，不影响理解，那它就应该删」

### 2. 克制用色（近乎无色原则）

**原则：** 不是不用颜色，而是尽量接近「无色」。大面积用白/极浅灰，强对比只用于关键状态，阴影几乎不可察觉。

**方法论：**
- 用留白 + 间距替代颜色区分
- 用层级差异替代高饱和强调
- 阴影是为了「分离感」，而非立体感

### 3. 层级来自空间，而非边框

**原则：** 极少用「线条」分割界面，模块之间是「空气」（留白）。

**方法论：**
- 层级是 Z 轴，不是 X/Y 轴
- 用 elevation（高度）替代 border（边框）
- 用模糊（blur）替代描边
- 这造就了「漂浮在云里」的视觉效果

### 4. 图标与插画：弱存在，但统一语言

**原则：** 线条统一、表情克制、几乎没有色彩。

**方法论：**
- 插画不是主角，是节奏调节器
- 所有视觉资产服务于同一个「气质」
- 不追求真实，只追求一致（如苹果的 SF Symbols）

### 5. 动效不是炫，是呼吸感

**原则：** 动效要慢一点、淡一点、有惯性。

---

## 在 SparkOps 中的执行顺序

做 UI 任务时，**先读本 skill 定方向**，再读 [fronted/SKILL.md](../fronted/SKILL.md) 取技术约定。

```
1. 内容审计 → 删掉不影响理解的元素
2. 空间布局 → stack-* 留白，少用 border 分割
3. 用色检查 → token 灰阶为主，语义色仅状态
4. 表面层级 → shadow-sm/md 分离，不用粗边框堆叠
5. 图标/动效 → 统一 stroke，150ms ease 过渡
6. 交付自检 → 下方 checklist
```

## 理论 → 代码映射

| 维度 | 在 SparkOps 中 |
|------|----------------|
| 内容优先 | 每 `.card`/`.section` 一个主题；`PageHeader` 标题 + 一句 description 即止 |
| 克制用色 | `var(--bg)` / `var(--surface)` / `var(--surface-muted)` 为主；`var(--primary)` 仅主按钮、链接、选中 |
| 空间层级 | `page-canvas` 主内容漂浮白卡片；`stack-lg` → `stack-md` → `stack-sm` 留白分层 |
| elevation | `var(--shadow-float)` 主画布；`var(--shadow-sm/md)` 卡片；`var(--shadow-lg)` Modal/Drawer |
| 少用边框 | 优先无 border 面板；必要时 `var(--border)` 1px，禁止彩色 border |
| 图标 | Lucide/现有 icon，统一 size（16/20），`var(--muted)` 或 `var(--text-secondary)` |
| 动效 | `transition: 150ms ease`；禁止 bounce、>300ms、大面积 gradient 动画 |

详细 Token 与组件清单见 [reference.md](reference.md)。

## 交付前自检

```
内容优先
- [ ] 每个卡片/区块只传达一件事
- [ ] 删掉任一装饰元素，信息仍完整
- [ ] 文案比 UI 控件更抢眼（字号/字重层级正确）

克制用色
- [ ] 背景以白/极浅灰为主，无大色块铺底
- [ ] 主色只出现在：主按钮、链接、选中态、关键指标
- [ ] 阴影轻到「几乎不可察觉」，仅用于分离

空间层级
- [ ] 模块间用 stack-* 留白，而非 border 分割线
- [ ] 浮层/Modal 靠 shadow-md + 圆角 elevation，不靠粗描边
- [ ] 无「边框套边框套边框」嵌套

图标与动效
- [ ] 图标线条/尺寸统一，非主角位用 muted 色
- [ ] hover/focus 过渡 150ms ease，有轻微惯性感
- [ ] 无 bounce、无长 duration、无炫技渐变

第一屏
- [ ] 能回答：我在哪？能做什么？关键数据是什么？
- [ ] 全屏最多一个 btn-primary 视觉焦点
```

## 反模式（立即修正）

| 不要 | 应该 |
|------|------|
| 满屏 gradient / 大色块背景 | `var(--bg)` + 白 surface |
| 用 border 区分每个区块 | `stack-md` 间距 + 轻 shadow |
| 多个 primary 按钮同屏 | 一个主操作，其余 ghost/secondary |
| 彩色 badge 堆砌 | muted 文案 + 语义色仅关键状态 |
| 复杂插画/emoji 当状态 | 统一 icon + 简短文案 |
| `box-shadow: 0 20px 60px` 重阴影 | `var(--shadow-sm)` / `var(--shadow-md)` |

## 延伸阅读

- SparkOps 组件与页面骨架：[fronted/SKILL.md](../fronted/SKILL.md)
- Token 映射与代码示例：[reference.md](reference.md)
- 全局样式定义：`fronted/apps/web/app/globals.css`
