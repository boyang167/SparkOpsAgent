# 项目侧边栏克制优化

## 目标

优化项目详情页左侧导航区域，使用户更清楚地识别当前项目与所在模块，同时保持 SparkOps 的克制控制台气质。改动只覆盖截图中的项目侧边栏，不调整平台导航、管理导航、顶栏或主内容区。

## 视觉方向

- 以内容层级为主：返回入口、当前项目、项目模块三层清晰区分。
- 当前项目卡片使用轻 surface 层级与更柔和圆角，避免形成厚重灰块。
- 项目导航保持 pill 形态，选中态通过轻背景、字重和细微空间感表达，不使用高饱和彩色强调。
- 图标保持弱存在，统一尺寸与线性语言，不抢占文字主导地位。
- hover 与 focus 使用现有 motion token，过渡短而轻，不添加弹跳、渐变或复杂动画。

## 实现范围

- 调整 `fronted/apps/web/src/components/layout/ProjectNav.tsx` 中项目导航项的图标结构与可访问性标记。
- 调整 `fronted/apps/web/app/globals.css` 中 `.nav-back-link`、`.nav-project-head`、`.nav-project-*`、`.nav-link` 与 `.nav-icon` 的项目侧边栏相关视觉状态。
- 复用现有设计 token：颜色、圆角、阴影和 motion，不引入新 UI 库或硬编码品牌色。
- 不改变路由、权限、项目名称解析或导航 active 判定逻辑。

## 交互与响应式

- 当前页面的导航项设置明确的 active 样式，并保留 `aria-current` 以辅助识别。
- 可点击项提供 hover 与 keyboard focus 反馈。
- 长项目名称保持单行省略，避免撑开侧边栏。
- 侧边栏宽度与 AppShell 布局不变。

## 验收标准

- 第一眼能回答：当前在哪个项目、位于哪个模块、如何返回全部项目。
- 当前项目卡片不再像重灰色块，整体更接近白/浅灰的空间层级。
- 全屏没有新增 competing 主色焦点；主色仍只用于选中、链接或关键操作。
- CSS 能由 Next.js/PostCSS 正常解析，前端 lint/typecheck 不新增错误。
