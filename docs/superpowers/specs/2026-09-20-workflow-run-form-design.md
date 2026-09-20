# 工作流执行单 UI 设计

> Date: 2026-09-20  
> Scope: 第一期仅前端交互与布局；后端入参不变  
> Reference: [KodeRover 工作流执行页](https://course.koderover.com/v1/projects/detail/microservice-demo/pipelines)

## Goal

点击工作流「执行」时，打开接近全屏的分段执行单：按构建 / 部署 / 其他任务分组填写，视觉与扫描路径对齐参考图，提交仍走现有 `startProductWorkflowRun` 入参。

## Current behavior

`StartWorkflowRunModal` 是普通小弹窗，字段为：

- 一个全局构建分支
- 每个 `sparkops-deploy` 任务一个环境下拉
- 勾了运行时输入的变量

后端 `input` 契约（保持不变）：

```ts
{
  branch?: string;
  environment_overrides?: Record<string, string>; // job.name -> environment_id
  variables?: Record<string, string>;
}
```

引擎 `ApplyRunInput` 会把 `branch` 覆盖全部 `sparkops-build`（含 `project_builds[].branch`），把 `environment_overrides` 覆盖对应部署任务的 `environment_id`。

## Interaction

入口不变：工作流列表「执行」、更多菜单「执行工作流」。

容器：宽屏约 960–1100px、高度约 80vh 的执行单（沿用现有 `Modal` 的 `size` 扩展或专用 class，禁止新 UI 库）。

结构：

1. **顶栏**：标题「执行工作流」+ 工作流名称；右侧阶段胶囊（仅展示当前工作流存在的阶段名），点击滚动到对应区块；关闭按钮。
2. **正文**：按阶段分块，块标题用阶段名。块内是表格，不是一长串 label。
3. **底栏**：主按钮「执行」，次按钮「取消」。无「调试 / 不使用缓存」。

## Sections

只渲染工作流里真实存在的任务类型，没有则整块不出现。

| 区块 | 何时出现 | 内容 | 写入 input |
|------|----------|------|------------|
| 构建 | 存在 `sparkops-build` | 行：项目/服务名、代码库（只读，有 Git 则仓库名，否则项目名）、分支输入 | 所有构建行共用一个 `branch`，改一处同步 |
| 部署 | 存在 `sparkops-deploy` | 顶部环境选择；下列只读服务/任务名 | 每个部署 job 写入同一所选 `environment_id` 到 `environment_overrides[job.name]` |
| 其他任务 | 存在 approval / freestyle / notify / 测试类 | 只读说明「按编排定义执行」；若 job spec 含 `project_id` / `project_builds` 则列出名称并共用构建分支 | 不新增字段 |
| 运行时变量 | `variables` 中 `runtime_input` 为真 | key + 描述 + 输入框 | `variables` |
| 备注 | 第一期不展示 | — | — |

草稿工作流仍不可打开执行（列表已禁用）。打开时预填：分支默认 `main` 或定义中的 branch；环境默认定义中的 `environment_id`。

校验：运行时必填变量为空时禁用「执行」。环境在有部署任务时必选。

成功后跳转该次运行详情页，行为与现在一致。

## Visual

- Token 只用 `globals.css` 已有变量。
- 表头浅底、行分割线、胶囊用主色填充（参考图顶栏蓝标）。
- 只读单元格灰色；可编辑单元格白底 + 边框。
- 空态文案短句，不堆说明。

## Out of scope

- 按服务/项目填写不同分支或 PR
- 镜像预览、部署顺序上下移
- 调试断点、不使用缓存
- 备注入库
- 改后端 `PrepareRun` / API schema

以上作为后续迭代，本 spec 明确不做。

## Files (expected)

- `fronted/apps/web/src/modules/product/components/StartWorkflowRunModal.tsx`（主改）
- `fronted/apps/web/app/globals.css`（执行单样式）
- 列表入口无需改调用契约；若需项目 Git 展示名，只读已有 `projectsByProduct` / workflow spec，不新增 API

## Testing

- 手工：构建+部署工作流打开执行单，改分支与环境后执行，确认运行使用覆盖值。
- 手工：仅构建、仅部署、带运行时变量、无部署，区块显隐正确。
- 现有 `workflow-list-utils` 测试不要求为本次新增；若抽出 stage 分组纯函数，补 2–3 个单测。
