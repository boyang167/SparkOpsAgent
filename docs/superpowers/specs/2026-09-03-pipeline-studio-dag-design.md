# Pipeline Studio 可视化 DAG 编排设计

> 版本：v1.0 · 2026-09-03  
> 状态：已评审（产品决策已确认）  
> 父文档：`spec/delivery-center-design.md`  
> 范围：前端 Mock 可视化编排，不接后端

---

## 1. 目标

在独立页面 `/delivery/pipelines/[id]/studio` 把现有 Pipeline Studio 升级为可拖拽的 DAG 编排器：从 Step 库拖入节点、在画布上拉线建立 `needs`、为 `fleet.deploy` 配置并行 / 串行 / 分波。

部署向导不在此期改成可编辑画布，只继续选择已保存的 Pipeline，并只读预览 DAG。

## 2. 已确认决策

| 决策 | 选择 |
|------|------|
| 编辑器位置 | 独立 Pipeline Studio，不嵌在部署向导里 |
| 发布方式 | 仅 `fleet.deploy.strategy`：`parallel` / `sequential` / `wave` |
| 持久化 | 前端 Mock，保存只写内存并增加版本号 |
| 画布引擎 | `@xyflow/react` 替换自研 `PipelineCanvas` |
| 领域模型 | 继续用现有 `PipelineVM`（`definition` + `layout`） |
| YAML | 保持「编辑 → 应用到画布」，不做实时双向同步 |

## 3. 非目标

- 后端 `cd` 模块、`pipelines` / `pipeline_versions` API
- `localStorage` 或跨刷新持久化
- Canary / 蓝绿（`progressive`）；表单中隐藏该字段
- 部署向导内的可编辑 DAG
- YAML 输入时自动回写画布
- Run Engine、Fleet Board 运行态高亮（可复用同一套节点，但不在本期做）

经典单集群部署路径（`ProjectDeployModal` + `deployrun` Orchestrator）保持不变。

## 4. 架构

单一数据源是 `PipelineVM`。React Flow 的 nodes/edges 只是视图，禁止把 Flow 内部 state 当成领域模型。

```
Step 库拖入 / 连线 / 改表单 / 应用 YAML
        │
        ▼
  PipelineVM（definition + layout）
        │
        ├── validatePipelineDefinition
        ├── pipelineToYaml / mergePipelineFromYaml
        └── definitionToFlow ──► React Flow 画布
```

写回顺序：画布事件 → 算出下一份 `stages` / `layout` → `validatePipelineDefinition` → 硬校验通过才更新 `PipelineVM` → Flow 从新 VM 渲染。硬校验失败则 definition 不变，顶栏提示，画布保持原状。

### 4.1 映射规则

| 领域 | 画布 |
|------|------|
| `PipelineStage` | React Flow node（`id = stage.id`） |
| `stage.needs` | 有向边：`need → stage.id`（多入边 = AND） |
| `layout.stages[id]` | 节点坐标 `{ x, y }` |
| Step 库一项 | 拖入后新建「单 step 的 Stage」 |

### 4.2 组件边界

均放在 `fronted/apps/web/src/modules/delivery/cd/`。

| 单元 | 路径 | 职责 |
|------|------|------|
| `PipelineStudio` | `components/PipelineStudio/PipelineStudio.tsx` | 状态主人：pipeline、选中节点、校验、Mock 保存 |
| `StepPalette` | `components/PipelineStudio/StepPalette.tsx` | 从现有左侧栏抽出，支持拖拽 + 点击添加 |
| `PipelineCanvas` | `components/PipelineStudio/PipelineCanvas.tsx` | 重写为 React Flow：节点、连线、平移缩放、落点创建 |
| `StageNode` | `components/PipelineStudio/StageNode.tsx` | 自定义节点：标题、主 step、策略徽标、审批高亮 |
| `flow-adapter` | `lib/flow-adapter.ts` | `PipelineDefinition` + `PipelineLayout` ↔ Flow nodes/edges |
| `StepPropertyPanel` | 沿用 | id、`if`、steps、`fleet.deploy` strategy |
| `pipeline-yaml` / `dag-layout` | 沿用并扩展 | YAML、环检测、自动布局 |

新增依赖：`@xyflow/react`（加在 `fronted/apps/web`）。

## 5. 画布交互

三栏布局不变：左 Step 库、中画布、右属性面板。

### 5.1 从库到画布

- 卡片可拖到画布空白处，落点即节点坐标。
- 点击卡片也可添加（无障碍兜底），新节点用自动布局空位，不覆盖已有坐标。
- 新建 Stage **不自动**连接到上一个节点。
- `fleet.deploy` 默认 `with.strategy = "parallel"`。
- 空画布显示「从左侧拖入步骤」。

### 5.2 连线与节点

- Handle：左入、右出。A → B 表示 B 的 `needs` 包含 A。
- 禁止自连；重复边忽略。
- 成环则拒绝写回，提示「依赖不能成环」（现有文案：`Pipeline DAG 存在环，请检查 needs 依赖`）。
- 删边：从目标 Stage 的 `needs` 去掉对应 id。
- 选中节点后按 Delete，或点右侧「删除」：去掉该 Stage，并清理其他 Stage 对它的 `needs` 与 `layout` key。
- 拖节点只更新 `layout.stages[id]`，不改 DAG。
- 工具条保留：可视化 / YAML 切换、缩放与自适应（React Flow `Controls`）、自动布局（复用 `autoLayoutStages`）、保存草稿。

### 5.3 发布方式

只作用于 `uses === "fleet.deploy"` 的主 step。

| `strategy` | 节点徽标 | 右侧补充 |
|------------|----------|----------|
| `parallel` | 并行 | 目标 Fleet 集群同时部署 |
| `sequential` | 顺序 | 按 Fleet 成员顺序 |
| `wave` | 分波 | 只读展示该 Fleet 的 wave 名称链，例如 `canary → primary → edge` |

选 `wave` 但对应 Fleet（`MOCK_FLEETS`）没有 `topology.waves` 时，面板给软警告，画布仍可编辑。

`progressive`（金丝雀 / 蓝绿）从 `fleet.deploy` 的 Catalog 字段中隐藏，不出现在表单和节点上。已有 Mock YAML 里若带该字段，解析后保留但不展示。

审批节点（`approval.manual`）继续使用暂停样式。

### 5.4 YAML

- 切到 YAML Tab 时用当前 `definition` 生成文本。
- 用户编辑后必须点「应用到画布」。
- 应用成功：用新 `definition` 重建 Flow；已有 stage id 尽量保留坐标，新增 id 走自动布局空位。
- 应用失败：保留编辑器原文，显示 `yamlError`，画布仍是旧图。

## 6. 校验与错误处理

在现有 `validatePipelineDefinition` 上扩展，不另起一套规则。

### 6.1 硬校验（失败则拒绝写回）

- Stage `id` 非空且唯一
- 每个 Stage 至少一个 step
- `needs` 引用存在的 id
- DAG 无环
- 每个 `uses` 必须在 `STEP_CATALOG` 中（现有校验尚未检查，本期补上）

改 Stage id 时：同步改写所有 `needs` 与 `layout` key；新 id 与已有 id 冲突则拒绝。

### 6.2 软警告（不拦拖拽、不拦保存草稿）

- `fleet.deploy` 未选 Fleet
- `strategy === "wave"` 但该 Fleet 无 wave 拓扑

### 6.3 保存

Mock「保存草稿」只把 `version + 1` 并提示成功。允许带着软警告保存。硬校验失败时画布事件本身不会写入 definition，因此正常操作下保存不到非法 DAG；YAML 硬校验失败则应用不上，保存的仍是上一份合法 definition。

## 7. 测试

沿用 `pipeline-yaml.test.ts` 的单元测试风格，不引入 E2E。

必须覆盖：

1. `flow-adapter`：`definition` + `layout` ↔ nodes/edges 往返；坐标写入 `layout.stages`。
2. 连接：`onConnect` 等价物给目标 Stage 增加 `needs`。
3. 成环：写回被拒，`stages` 与连线前一致。
4. 删边：去掉对应 `needs`。
5. 拖入 `fleet.deploy`：默认 `strategy: "parallel"`。
6. YAML 成环：`mergePipelineFromYaml` 失败，pipeline 保持原 definition。
7. `uses` 不在 Catalog：硬校验报错。

现有 YAML roundtrip 与环检测测试继续通过。

## 8. 验收标准

- 能从 Step 库拖出「Jenkins 构建 → 人工审批 → Fleet 部署」三条节点，并用连线做成该顺序。
- Fleet 部署节点可选并行 / 顺序 / 分波；选分波且 Fleet 为 `prod-cn` 时，右侧能看到 `canary → primary → edge`。
- 节点徽标能区分三种策略；审批节点有暂停样式。
- 拉成环时连不上，顶栏出现环错误。
- YAML Tab 能看到同一份 DAG；改坏后「应用到画布」失败且可视化图不变。
- 保存草稿只更新内存版本号，不请求后端。
- 经典「单集群部署」入口与 API 行为不变。

---

*本文件只覆盖 Pipeline Studio 可视化一期。后续 Run Engine、真实保存、向导只读预览增强见父文档实施路线图。*
