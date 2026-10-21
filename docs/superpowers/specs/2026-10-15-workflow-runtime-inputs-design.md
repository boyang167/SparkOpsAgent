# 工作流运行输入契约与发布参数 设计

日期：2026-10-15
状态：已确认，待实施

## 1. 背景

发布计划 `deploy1` 的工作流 `dev` 在第三阶段「创建版本 · archive」失败：

> 版本号 "{{version}}" 包含未替换的占位符，请在执行时填写真实版本号

根因有三层：

1. **两种占位符语法并存。**
   - `{{.KEY}}`：运行变量，每次运行由 `engine.InterpolateStages` 替换。
   - `{{key}}`：模板参数，仅在「从模板创建工作流」时由 `keyval.Resolve` 替换一次；创建时未赋值就会原样残留在工作流定义里。
   `dev` 的 `version_tag` 残留了 `{{version}}`，运行时不会被任何变量替换。
2. **缺失输入发现太晚。** `engine.mergeRunVariables` 只检查「已声明 + runtime_input」的变量；残留占位符要等执行到对应任务才报错，前面的构建、部署已经跑完（本例 1 分 22 秒）。
3. **发布计划没有填写入口。** 后端已支持把发布项参数中非 `_` 前缀的键作为 `variables` 传入运行，但前端既不知道需要哪些输入，也没有地方填写。工作流运行弹窗靠专门检测版本号占位符 + `version_tag_overrides` 绕过，发布计划没有这条路径。

后续会有更多工作流（`{{env}}`、`{{release_note}}`、`{{ticket}}` …），需要通用机制而不是版本号特例。

## 2. 已确认的决策

| 决策 | 结论 |
|------|------|
| 填写时机 | 规划时填写；「完成规划」强制校验齐全，审批人看到的即最终参数 |
| 未声明占位符 | 自动视为必填运行输入；工作流编辑器提示「建议声明为变量」，不拦截保存 |
| 作用范围 | 计划级「发布参数」统一填写，所有工作流发布项继承，单个发布项可覆盖 |

## 3. 设计

### 3.1 运行输入清单（product_workflow 引擎）

新增 `engine/runtime_inputs.go`：

```go
type RuntimeInputRef struct {
    StageIndex int    `json:"stage_index"`
    StageName  string `json:"stage_name"`
    JobName    string `json:"job_name"`
    JobType    string `json:"job_type"`
    Field      string `json:"field"` // 任务 spec 的顶层字段，如 version_tag
}

type RuntimeInput struct {
    Key      string            `json:"key"`
    Label    string            `json:"label"`            // 已声明变量取其说明，否则为 key
    Declared bool              `json:"declared"`         // 是否已在工作流「变量」中声明
    Secret   bool              `json:"secret,omitempty"`
    Refs     []RuntimeInputRef `json:"refs,omitempty"`   // 引用位置
}

func RuntimeInputs(wf *model.ProductWorkflow, input map[string]any) []RuntimeInput
```

收集规则：

- 先把 `input`（分支、环境、项目、版本号覆盖等，不含 `variables`）通过 `ApplyRunInput` 套到阶段定义上，再扫描。被覆盖掉的占位符（如发布项已指定分支时的 `{{build_branch}}`）不再算作所需输入。`input=nil` 时扫描原始定义。
- 已声明且 `runtime_input=true` 的变量 → 必填，`Declared=true`。
- 扫描所有阶段任务 `spec`（递归字符串），匹配 `{{.KEY}}` 与 `{{key}}`（key 规则 `[A-Za-z][A-Za-z0-9_]*`）。
  - 跳过原样内容字段：键名为 `content`、`yaml`、`manifest`、`template` 的值（部署 YAML、配置资源正文等可能自带 `{{name}}` 文本）。这些字段里只识别 `{{.KEY}}`，不识别 `{{key}}`。
  - 排除系统变量（`systemVariables` 的键：`WORKFLOW_NAME`、`RUN_ID`、`GIT_BRANCH` …）。
  - 排除已声明但非 runtime_input 的变量（无论默认值是否为空，运行时都会被替换为默认值）。
  - 其余 → 必填，`Declared=false`。
- 结果按 key 去重并合并 `Refs`，按首次出现排序（字段按名称排序遍历），结果稳定。

`GET /products/:id/workflows/:workflowId` 与工作流保存响应新增 `runtime_inputs` 字段（`input=nil`）；前端各处只读这一份，不再自行扫描。

### 3.2 运行时统一替换与启动前拦截

- `Interpolate` 同时替换 `{{.key}}` 与 `{{key}}`，值均取自合并后的运行变量。未提供值的保持原样。3.1 中的原样内容字段只替换 `{{.key}}`。
  - 模板创建阶段仍由 `keyval.Resolve` 先处理，二者不冲突：能走到运行时的 `{{key}}` 必然是模板未赋值的残留。
- `PrepareRun` 在替换变量、`ApplyRunInput`、按 `stage_indices` 过滤之后调用 `UnresolvedPlaceholders(stages)`（与 3.1 相同的扫描规则）；非空即返回 `model.FormatMissingVariables(keys)` 错误，运行不会被创建。
  - 覆盖所有入口：工作流运行弹窗、Git 触发器、发布计划。
  - 只检查本次实际执行的阶段，部分阶段运行不受未选阶段影响。
- 现有 `version_tag_overrides` 保留，优先级高于变量替换后的值，兼容运行弹窗。
- `create_version_executor` 中的占位符检查保留作为兜底。
- 运行弹窗：除已声明的运行时变量外，额外列出 `runtime_inputs` 中 `Declared=false` 的项作为必填变量；所有引用都在 `version_tag` 字段的项除外（已有版本号输入框负责）。

### 3.3 发布计划：计划级发布参数 + 发布项覆盖

**数据模型**

- `ReleasePlan` 新增 `Parameters map[string]string`；表 `product.release_plans` 新增 `parameters jsonb`（AutoMigrate 字段 + 迁移文件 `000068_release_plan_parameters`）。
- 计划版本快照（`release_plan_version.go`）、模板（`release_plan_template.go`）、复制计划一并携带 `parameters`。版本差异只显示「发布参数 · key」已填写 / 已修改 / 未填写，不显示取值。
- 发布项覆盖沿用 `WorkflowRefSpec.Parameters` 中非 `_` 前缀的键，不新增结构。`branch`、`environment_id` 是保留键，同名输入只能在计划级填写。

**合并顺序**

`ToRunInput(planParams, required)`：发布项自身的非保留参数全部作为变量传入（保持现有行为）；所需输入中发布项未填写的键，再从计划参数取值。计划参数只传入该工作流需要的键，避免注入其他工作流。

**接口**

- 计划更新请求与详情响应新增 `parameters`（创建请求本期不加，规划阶段在详情页填写）。
- 计划详情响应中每个工作流发布项附带 `runtime_inputs`（由 3.4 的端口按该发布项的 input 计算），前端据此渲染并判断缺失。

**界面**

- 计划详情「发布项」Tab 顶部新增「发布参数」区块：列出所有工作流发布项所需输入的并集。每行显示键名、显示名、引用它的发布项（如「dev · 阶段3 · archive」），以及输入框；有发布项因此缺参时以警示色标出。没有任何所需输入时不显示该区块。
- 发布项卡片新增「运行参数」字段：逐项显示所需输入。未覆盖时显示「继承：1.2.0」或「未填写」（警示色），可点击「单独填写」，已覆盖时可「恢复继承」。
- 规划中可编辑；执行中仅失败的工作流发布项可编辑运行参数；其他状态只读。

### 3.4 校验门：完成规划

- `appctx` 新增端口 `WorkflowRuntimeInputReader`：`RuntimeInputs(ctx, productID, workflowID, input) ([]WorkflowRuntimeInput, error)`，由 product_workflow 模块实现并注入 `Container.WorkflowRuntimeInputs`；工作流不存在时返回 `appctx.ErrWorkflowNotFound`。
- `FinishReleasePlanPlanningHandler` 在任何状态变化（含进入 `finish_planning` 外部检查钩子等待）之前，对每个未跳过的工作流发布项计算缺失键（所需输入 − 发布项覆盖 − 计划参数，空白值视为缺失）。钩子回调只做状态流转，因此钩子路径同样被覆盖。
- 有缺失时返回 `RELEASE_PLAN_MISSING_INPUTS`（HTTP 422），消息形如「发布项缺少运行参数：「dev」缺少 version；「prod」缺少 ticket」。
- 前端在调用接口前用同一规则（基于各发布项的 `runtime_inputs`）预先计算：有缺失时不发请求，展示同样的提示，切到「发布项」Tab，展开并滚动到第一个缺参发布项。

### 3.5 工作流编辑器提示

- 编辑器「变量」面板读取最近一次加载或保存得到的 `runtime_inputs` 中 `Declared=false`、且当前未声明的项，显示「未声明的占位符」列表：键名、引用位置，说明「运行时将作为必填参数」。
- 每项提供「声明为变量」快捷操作：新增变量 `key=<key>, runtime_input=true`。仅提示，不拦截保存。

## 4. 错误处理

| 场景 | 行为 |
|------|------|
| 完成规划时缺参数 | 前端预检拦截；绕过前端时后端返回 422 + 文字清单，计划保持「规划中」 |
| 工作流在规划后被修改、新增了输入 | 启动运行时 `PrepareRun` 拦截，不创建运行；`ReleasePlanExecutor.Advance` 把启动阶段的领域错误记录为发布项失败（`execution_ref.error_message`），不再作为接口错误抛出。用户补填本项运行参数后点「重试」 |
| 读取工作流运行输入失败（工作流被删除等） | 完成规划返回「发布项「X」关联的工作流不存在」；执行时同样把发布项标为失败 |
| 敏感变量（Secret） | 输入框为密码样式；详情响应中计划参数与发布项覆盖按 `******` 掩码返回；更新时收到 `******` 保留原值 |

## 5. 测试

- `engine`：`RuntimeInputs` 覆盖已声明 / 未声明 / 系统变量 / 已声明固定值 / 原样内容字段 / 套用 input 后消失 / 去重合并 `Refs`；`Interpolate` 两种语法；`PrepareRun` 残留占位符拦截、版本号覆盖放行、未选阶段不拦截。
- `product/domain/model`：`ToRunInput` 合并顺序与按所需键过滤；缺失键计算；保留键；掩码还原；启动失败与重试流转；执行中编辑权限；版本差异不泄露取值。
- `product/application/service`：缺参返回 `RELEASE_PLAN_MISSING_INPUTS` 与清单；齐全时通过；跳过的发布项不校验；工作流不存在的错误信息。
- 前端：所需输入并集、继承 / 覆盖 / 缺失判定、缺失清单、运行弹窗额外输入过滤、`mergeWorkflowJobParameters` 写入与清除覆盖。
- 现有 `run_input_projects_test.go`、版本号覆盖、`release_plan_test.go` 保持通过。

## 6. 不在本期范围

- 参数类型与格式校验（如版本号必须 semver）、下拉选项、按环境区分的取值 —— 后续可在变量声明上扩展。
- 执行时再补填参数的弹窗（已决定规划时填写）。
- 计划参数的加密存储（本期仅接口掩码）。
- 创建发布计划时直接填写参数。
