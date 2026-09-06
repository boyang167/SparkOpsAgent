# 发布编排接入现有构建流程

> 版本：v1.0 · 2026-09-03  
> 状态：已评审（产品决策已确认）  
> 父文档：`spec/delivery-center-design.md`  
> 相关：`docs/superpowers/specs/2026-09-03-pipeline-studio-dag-design.md`  
> 范围：把 Pipeline DAG 接到「镜像已生成之后」的发布，而不是重做 Jenkins 构建

---

## 1. 目标

项目已经用 Jenkins 模板生成 Jenkinsfile，构建产出镜像。不同环境、不同发布方式（并行 / 串行 / 分波）由 **发布 Pipeline** 编排。

工作台「发布」打开向导，用户二选一：

- **使用已有镜像**：选最近成功构建的 `{repo}:{tag}`（可手填 tag），直接进入发布 DAG。
- **先构建再发布**：填 Git ref，走现有 `preparing → building`，制品就绪后再进入同一条发布 DAG。

Jenkins 模板只回答「怎么构建」；Pipeline 只回答「怎么发布」。

## 2. 已确认决策

| 决策 | 选择 |
|------|------|
| 编排职责 | Release-only DAG，不替换 Jenkins 模板 / Jenkinsfile |
| 向导制品来源 | 二选一：已有镜像 / 先构建再发布 |
| 编辑器 | 独立 Studio；向导只读预览已保存 Pipeline |
| 发布策略（一期） | `fleet.deploy.strategy`：`parallel` / `sequential` / `wave` |
| 经典单集群 | 兼容期保留，等价于「先构建 + 单集群一条 TaskRun」 |
| 项目绑定 | `jenkinsfile_template_id`（构建）与 `default_pipeline_id`（发布）分开 |

## 3. 非目标

- 本期不实现完整后端 `cd` Run Engine（可先把向导接到真构建 API + Mock 发布 DAG）。
- 不在向导里现场拖拽改 DAG。
- 不把 Jenkinsfile 写回业务 Git。
- 不做 Canary / 蓝绿执行（`progressive` 仍隐藏）。
- 不要求第一期就有 image digest；先用 `image_repository + image_tag`。
- 不删除 `POST .../deployments` 经典路径。

## 4. 今天为什么接不上

当前真实闭环是一条焊死的 `DeploymentRun`：

```
选模板 → 生成 Jenkinsfile → 创建 Run（预先定 image_tag）
  → preparing → building → deploying（同一 tag、一个集群、一次 apply）
```

没有「制品已就绪」停靠点。`delivery.builds` 不存镜像坐标。工作台主按钮「部署」打开的是 Mock 向导，不会调用构建或发布 API。

编排的榫头必须加在 **building 成功之后**，用 ArtifactSet 把构建和发布分开。

## 5. 目标流程

```
配置（已有）
  项目绑定 jenkinsfile_template_id
  生成 / 预览 Jenkinsfile

构建（已有，可独立）
  工作台「构建」或向导「先构建再发布」
  产出 ArtifactSet { image_repository, image_tag }
  可以停在这里，不强制上集群

发布（新增）
  输入：ArtifactSet + Pipeline + Fleet/环境
  artifact.ready 之后只跑 CD DAG
  fleet.deploy 按策略展开为 N 次现有 K8s apply
```

**一条发布 Run 的内部阶段**

```
来源 = 已有镜像:   artifact.ready → 执行发布 DAG
来源 = 先构建:     queued → preparing → building → artifact.ready → 执行发布 DAG
```

## 6. 发布向导

入口：项目工作台主按钮改为「发布」（`ReleaseWizardModal`）。门禁沿用现有 `deployment-gate`（审核、密钥、无冲突 Run）。

### Step 1 制品来源

| 选项 | 表单 | 禁用条件 |
|------|------|----------|
| 使用已有镜像 | 下拉最近成功构建的 `{repo}:{tag}`，允许手填 tag | 没有任何成功构建且未手填 |
| 先构建再发布 | Git ref 类型/值（与 `ProjectDeployModal` 相同）；`image_tag` 可选，空则后端按 Run ID 生成 | 缺 Jenkins / Git / Harbor 密钥 |

两种来源最终都写入同一份 `ArtifactSet` 草稿：已有镜像当场填齐；先构建则在 `building` 成功后回填。

### Step 2 发布编排

- 默认选中项目 `default_pipeline_id`，可换同 Workspace 下 `status=active` 的 Pipeline。
- 只读 DAG 预览（复用 Studio 的 layout / React Flow 只读模式）。
- 预览从 CD 节点起：`fleet.deploy`、审批、验证。不要求用户再选 Jenkins 模板。
- 若选中的 Pipeline 含 `jenkins.build`：仅当来源是「先构建再发布」时允许；来源是「已有镜像」则提示将跳过构建类步骤。

### Step 3 目标环境

- 多选 Fleet（受项目 `allowed_fleet_ids` 约束；未配则列出 Workspace 内 Fleet）。
- 展开 wave 成员，标 Ready / Blocked（集群未授权、凭证未就绪、缺 Profile）。
- Blocked 不禁止进入下一步，但确认页必须展示风险；`blocked=true` 的强制门禁（例如 prod 无审批）在 Preflight 里拦截。

### Step 4 确认

摘要：来源、制品坐标、Pipeline 名/版本、Fleet、各 `fleet.deploy` 的 strategy。  
确认后创建 Run，跳转 `/delivery/runs/:runId`。

## 7. 领域交接

### ArtifactSet（构建 → 发布）

| 字段 | 第一期 | 说明 |
|------|--------|------|
| `image_repository` | 必填 | 与现有部署 ACL 公式一致：`{harborHost}/{namespace}/{projectName}` |
| `image_tag` | 必填 | 已有镜像：用户选/填；先构建：创建时预分配或用户覆盖 |
| `image_digest` | 可选，后补 | 不拦第一期发布 |
| `git_ref_type` / `git_ref_value` | 先构建时必填 | 已有镜像时记录来源构建上的 ref（若有） |
| `build_id` | 有则写 | 关联 `delivery.builds` |

没有 ArtifactSet 不得进入发布 DAG。

### 项目字段

```
jenkinsfile_template_id   // 已有，怎么构建
default_pipeline_id       // 新增，怎么发布
allowed_fleet_ids         // 新增，可发布的 Fleet；空 = 不限制（仍受集群授权）
```

### 与 DeploymentRun 兼容

- 经典 `POST .../deployments`：内部视为「先构建 + 单集群 TaskRun」，状态机仍是 `queued → preparing → building → deploying`。
- 新发布 Run：`fleet.deploy` 每个目标集群一条 TaskRun；兼容期可双写 `deployment_runs`（`release_id` 可空）。
- 同一项目仍只允许一条 `pending/running/paused` 的发布 Run（可与经典 active DeploymentRun 互斥）。

### 现有 K8s apply 的复用

`fleet.deploy` 展开后，每集群调用今天 `deploying` 使用的 `YAMLManifestRenderer` + `ManifestDeployer`：

- 镜像强制为 ArtifactSet 的 `{repository}:{tag}`。
- 凭证第一期仍用项目级 `kubernetes_secret_id`；per-cluster secret 后补。
- `strategy=parallel`：集群并行 apply。
- `strategy=sequential`：按 Fleet 成员顺序。
- `strategy=wave`：按 Fleet `topology.waves` 顺序；一波全部成功（或 `onFailure` 允许）再下一波。

## 8. Studio 默认模板

Workspace 预置一条 **release-only** 模板（项目默认绑定它），不要以 `jenkins.build` 开头。示意：

```yaml
apiVersion: cd.sparkops.io/v1
kind: Pipeline
metadata:
  name: release-staging-then-prod
spec:
  parameters:
    - { name: image_tag, type: string, required: true }
  stages:
    - id: deploy-staging
      steps:
        - uses: fleet.deploy
          with: { fleet: staging-all, strategy: parallel, profile: staging }
    - id: verify
      needs: [deploy-staging]
      steps: [{ uses: k8s.wait_rollout }, { uses: http.smoke }]
    - id: approve-prod
      needs: [verify]
      steps: [{ uses: approval.manual }]
    - id: deploy-prod
      needs: [approve-prod]
      steps:
        - uses: fleet.deploy
          with: { fleet: prod-cn, strategy: wave, profile: prod-cn }
```

`jenkins.build` 仍留在 Step 库，仅用于「从 Git 一键出发」的可选模板。向导选「已有镜像」时跳过这类步骤。

## 9. API 增量（相对现有）

第一期最小集：

```
GET  /projects/:id/artifacts          # 最近成功构建的 repo/tag 列表（可先从 deployment_runs + builds 拼）
GET  /projects/:id/release/preflight  # Pipeline 列表、Fleet Ready、门禁
POST /projects/:id/releases           # source=artifact|build + pipeline + fleets + 参数
GET  /delivery/runs/:runId            # 发布 Run 状态（可先 Mock 观测，后接 Engine）
```

`POST .../builds`、`POST .../deployments`、`GET .../generated-templates` 保持兼容。

`CreateRelease` 当 `source=build` 时：先走现有 CreateDeployment 的 preparing/building（不进入单集群 deploying），成功后写 ArtifactSet 再调度 DAG。当 `source=artifact` 时：校验 tag 存在（或允许手填），直接 `artifact.ready`。

## 10. UI 改动

| 表面 | 变化 |
|------|------|
| `ProjectWorkbenchPage` | 主按钮「发布」→ 真向导；保留「经典单集群部署」 |
| `ReleaseWizardModal` | 按第 6 节四步改；不再 `setTimeout` 假成功 |
| Pipeline Studio | 增加 release-only 默认模板；构建类步骤标为可选 |
| `/delivery/builds` | 成功记录要能被向导 Step 1 选中（需能读到 tag） |
| Fleet Board | 仍可为 Mock，但 Run 创建后必须跳到该页 |

## 11. 落地顺序

1. **制品可读**：构建成功后能列出 `{repo}:{tag}`（可先从 `deployment_runs.image_tag` + 仓库公式推导）。
2. **向导接真 Preflight / 创建**：Step 1 二选一可用；`source=build` 复用现有构建；`source=artifact` 跳过构建。
3. **DAG 替换单集群 applying**：`fleet.deploy` 调现有 deployer，按 strategy 展开。
4. digest lock、per-cluster 凭证、审批回调、Profile 分层。

## 12. 验收

- 有成功构建的项目：选「使用已有镜像」→ 选 release-only Pipeline → 选 Fleet → 确认后 **不再触发 Jenkins**。
- 选「先构建再发布」：先出现 preparing/building，成功后才执行 `fleet.deploy`。
- 项目未绑 Jenkins 模板时，「先构建」被门禁拦住；「已有镜像」仍可用（若有手填或历史 tag）。
- 经典「单集群部署」行为与现在一致。
- Studio 仍只编发布 DAG，不要求用户在画布上配置 Jenkins 模板。

---

*构建归 Jenkins 模板；发布归 Pipeline。中间只交接 ArtifactSet。*
