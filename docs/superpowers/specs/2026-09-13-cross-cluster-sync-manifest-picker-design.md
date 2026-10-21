# 跨集群同步：可选资源与 YAML 编辑

> 仅「跨集群同步」入口。弹窗展示将 Apply 的资源，默认勾选现网核心工作负载；用户可增选 ConfigMap / Secret / Ingress，并可编辑 YAML。改动只作用于本次同步到目标集群。

## 目标

操作者在点击「开始同步」前能看到、勾选、修改本次要同步的 K8s 清单；提交后编排器按勾选清单 Apply 到除构建集群外的授权集群。

## 非目标

- 「跨集群发布」弹窗不增加资源选择（发布仍走构建 + 现有 Deployment/Service 渲染）。
- 不把弹窗内 YAML 写回构建集群或项目配置。
- 不同步 PVC、CronJob、任意自定义 CRD。
- 取消勾选不会删除目标集群已有资源。
- 某目标集群失败后，不自动回滚已成功的集群。
- 不在 UI 中展示镜像拉取 Secret 的 dockerconfigjson 明文。

## 背景

当前 `sync_only` 复用最近成功镜像，跳过 Jenkins，对每个同步集群调用 `finalizeKubernetes`：只渲染并 Apply **Deployment + Service**，再由 deployer 按 Harbor 凭据 **ensure imagePullSecret**。deployer 的 `resourceFor` 目前只允许 Deployment/Service，ConfigMap 会返回 `ErrManifestUnsupported`。

跨集群同步确认已是独立 Modal，适合扩成「清单 + YAML」而不改发布路径。

## 决策摘要

1. 只改 **跨集群同步**；交互采用 **同一弹窗左右栏**（方案 1）。
2. 资源来源：**构建集群现网**；按与当前服务的相关性过滤。
3. 默认勾选：Deployment、Service、镜像拉取 Secret。
4. 默列不勾：相关 ConfigMap、Secret、Ingress。
5. YAML 编辑 **仅本次 Apply**；禁止改 `kind` / `metadata.name` / `metadata.namespace`。
6. 勾选清单随 `sync_only` 创建请求提交，持久化在本次 DeploymentRun 上供异步编排读取。
7. deployer 放宽白名单，并允许 sync 路径在未勾选 Deployment/Service 时不强制二者同时存在。

---

## 产品交互

### 入口

服务详情「更多 → 跨集群同步」打开 `CrossClusterConfirmModal`（`mode=sync`），尺寸 `xl`。

### 布局

上半：构建/主部署集群、同步目标、Git 引用、镜像标签（现有文案保留）。

下半：

| 区域 | 行为 |
|------|------|
| 左栏清单 | 按 kind 分组；每项 checkbox + 名称；当前项高亮 |
| 右栏编辑器 | 选中项的 YAML；镜像拉取 Secret 为只读说明，无明文 |
| 页脚 | 取消 / 开始同步；至少勾选 1 项且无校验错误才可提交 |

加载预览时显示骨架。预览失败：错误提示，不允许空清单硬提交。无附属资源：核心三项仍可用（现网 YAML 或项目模板回退），附属分组空态「未发现相关资源」。

### 默认勾选与过滤

**默认勾选**

- Deployment：优先构建集群现网 YAML，否则项目模板；容器镜像 tag 替换为本次同步的 `image_tag`。
- Service：同上。
- 镜像拉取 Secret：Deployment `imagePullSecrets` 中的名称；内容不从构建集群拷贝，目标集群用 Harbor 凭据 ensure。

**列出但不默认勾选（相关）**

满足任一即列出：

1. 被 Deployment 引用：`envFrom`、`env.valueFrom`、`volumes[].configMap|secret`
2. label `app` 或 `app.kubernetes.io/name` 等于项目名
3. 资源名以项目名为前缀
4. Ingress backend 指向本服务 Service 名

排除：`default-token-*`、类型为 `kubernetes.io/service-account-token` 的 Secret、helm 释放类系统 Secret。

### 编辑约束

- 可改 spec / data / annotations 等。
- 提交时校验：YAML 可解析；`kind`、`metadata.name` 与清单项一致；`metadata.namespace` 为空或等于项目 namespace。
- 非法项在清单上标红，整单不可提交。
- 未勾选资源不进入请求体。

---

## 架构

### 预览

`GET /api/v1/projects/:projectId/deployments/sync-preview`

- 权限：`project:read`（与部署选项一致即可）
- 使用构建集群（priority / `is_build_env`）kubeconfig，列出命名空间内相关资源并附 YAML
- 镜像拉取 Secret 项：`kind=Secret`，`managed=true`，`yaml` 为空，`note` 说明将在目标集群由 Harbor 凭据创建

响应项字段：`id`（kind/name）、`kind`、`name`、`related_reason`、`default_selected`、`managed`、`yaml`、`note`

BFF：`/api/bff/v1/workspaces/:wsId/projects/:projectId/deployments/sync-preview`

### 提交

现有 `POST .../deployments`，`sync_only: true` 时增加：

```json
{
  "sync_only": true,
  "git_ref_type": "tag",
  "git_ref_value": "v1.0.1",
  "image_tag": "v1.0.1",
  "manifests": [
    { "kind": "Deployment", "name": "demo", "yaml": "..." },
    { "kind": "Service", "name": "demo", "yaml": "..." },
    { "kind": "Secret", "name": "registry-demo", "managed_pull_secret": true }
  ]
}
```

`kind` 白名单：`Deployment` | `Service` | `ConfigMap` | `Secret` | `Ingress`。

`managed_pull_secret: true` 的项不携带 YAML，编排时走现有 `ensureImagePullSecrets`。

清单持久化在 `deployment_runs.sync_manifests`（JSONB）。普通发布、回滚不写该列。

### Apply

`sync_only` 且 `sync_manifests` 非空时，**不要**只用 `renderManifests` 覆盖用户勾选结果。对每个同步目标集群：

1. 将用户 YAML 解析为 unstructured，强制 `metadata.namespace` = 项目 namespace
2. Apply 顺序：ConfigMap → Secret → Service → Ingress → Deployment
3. 若包含 `managed_pull_secret` 或 Deployment 引用了 pull secret：ensure Harbor imagePullSecret
4. 仅当清单含 Deployment 时 `RolloutRestart`
5. 未勾选 Deployment 或 Service 时，deployer 不得再要求「两者必须同时存在」
6. 普通 `finalizeKubernetes`（发布/回滚）仍要求 Deployment+Service，行为不变

`resourceFor` 增加：

- `ConfigMap`（v1）
- `Secret`（v1）
- `Ingress`（`networking.k8s.io/v1`）

某同步集群失败：记 `cross-cluster sync failed`，整次 run 失败。成功后可追加 `applied: Deployment/foo, ConfigMap/bar`。

主部署集群（构建集群）：`sync_only` 仍先对该集群走现有部署（用户清单同样 Apply），再同步其余集群。用户清单是「本次要落地的内容」，构建集群与目标集群使用同一份勾选 YAML（构建集群也是 Apply 用户清单，而不是只改目标集群）。

澄清：决策「编辑不写回构建集群」指 **不把 YAML 存进项目配置 / 不作为构建集群的持久编辑入口**；本次 run 若仍对构建集群执行 `finalizeKubernetes`，会把同一份清单 Apply 上去。为避免误改构建集群现网，**sync_only 对构建集群跳过 Apply，只对 `cluster.ID != run.ClusterID` 的授权集群 Apply 用户清单**。构建集群已有对应版本镜像与工作负载，同步语义是「推到其余集群」。

若构建集群与目标需要一致且用户改过 YAML：仅目标集群拿到编辑后的 YAML，符合「仅本次同步到目标集群」。

---

## 错误处理

| 场景 | 行为 |
|------|------|
| 预览超时/无权限 | 弹窗展示错误；禁止提交 |
| YAML 非法或 identity 被改 | 该项错误；禁止提交 |
| `manifests` 为空 | 400 |
| kind 非白名单 | 400 |
| 目标集群 Apply 失败 | run 失败；已成功集群不回滚 |
| 镜像拉取 Secret | 目标集群用 Harbor 重建，不复制源集群 dockerconfigjson |

---

## 测试

- 预览过滤：引用、标签、前缀、Ingress backend；排除 SA token
- 创建：拒绝改 name/kind/namespace；允许只有 ConfigMap（无 Deployment）
- deployer：ConfigMap/Secret/Ingress 可 Apply；普通发布仍要求 Deployment+Service
- 编排：sync_only 不 Apply 构建集群，只 Apply 同步目标；失败打 failed 事件
- 前端：默认勾选三项；取消勾选不进 body；编辑后的 YAML 出现在请求里

---

## 主要改动面

- `fronted/.../CrossClusterConfirmModal.tsx`：清单 + YAML
- `fronted/packages/contracts/src/delivery/index.ts`：preview VM、`manifests` 字段
- BFF + web-sdk：`getSyncPreview`、create body
- `backend/.../command/create_deployment.go`：校验并保存 manifests
- migration：`sync_manifests JSONB`
- `orchestrator.go`：sync_only Apply 用户清单
- `runtime/.../deployer.go`：扩展 kind；sync 模式不强制 Deployment+Service
