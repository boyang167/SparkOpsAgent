# SparkOps Kubernetes 管理插件设计

> 用现有插件框架交付 `plugin:kubernetes`：功能对齐 `plugins/kubernetes.zip`（GVA Kubernetes 插件），前后端按 SparkOps（Gin + Next.js BFF + 现有设计系统）重写。不嵌入 Vue，不执行 GVA initialize 五件套。

## 目标

- 工作区启用后提供完整 Kubernetes 控制台：集群选择、原生资源、生态 CRD、监控、终端/文件、审计、CloudTTY、NodeTTY、Arthas、Velaux、AI Chat。
- 集群连接与授权复用平台 `cluster_info` + `workspace_clusters`，插件不维护第二套 kubeconfig 账本。
- 仓库内置即可用，同时可导出符合 `InspectZip` 的 ZIP，走上传 → PR → 部署通道。
- UI 功能、字段、交互对齐 GVA；视觉使用 SparkOps 组件与导航。

## 非目标

- 原样安装 GVA Vue ZIP。
- 像素级复刻 Element Plus 皮肤。
- Karmada 占位页。
- 插件热加载。
- 卸载时 DROP `plugin_kubernetes` 业务表。
- 替代或删除平台已有的 runtime Pod 观测、部署流水线、工作区集群登记页。
- 普通成员的细粒度资源级 RBAC（本期写操作仅 `governance:admin`）。

## 决策摘要

1. 交付形态：bundled + ZIP。`id=kubernetes`，entitlement=`plugin:kubernetes`。
2. 实现路线：安全 apiserver Proxy + 资源目录驱动的 Next 页面 + 专项实时模块。
3. 集群范围：仅工作区已授权集群；选中集群后可见全部 Namespace 与集群级资源。
4. 缺 CRD：菜单保留，请求展示 apiserver 错误。
5. 权限：enable 后成员只读；创建/更新/删除、Exec、文件、诊断、特权壳需要 `governance:admin`。
6. SDK：保留 `Register(*gin.Engine)`；新增可选 `RegisterWithContext(engine, PluginContext)`。
7. 数据：独立 schema `plugin_kubernetes`，平台统一 AutoMigrate；卸载不 DROP。
8. GVA 缺陷在本期修复：挂上终端/日志 Web 通道、补根落地页、注册 Velero 备份中心菜单。

## 架构

### 限界上下文

| 单元 | 职责 |
|------|------|
| `platform/plugin` | 注册表、`InstallAll`、可选 Context 注入 |
| `modules/plugin` | 安装/启用生命周期；bundled 种子 `deployed` |
| `internal/plugins/kubernetes` | 插件实现：Proxy、专项服务、迁移声明 |
| `platform_admin` 集群表 | 只读消费：已授权集群及证书字段 |
| 前端 `app/(platform)/plugins/kubernetes/**` | 控制台页面 |
| BFF `app/api/bff/v1/plugins/kubernetes/**` | 鉴权转发与流式代理 |

插件禁止 import 其它 BC 的 `domain`/`infrastructure`。允许 `platform/plugin`、`httpx`、`authn`，以及 Context 注入的端口（K8s client 工厂、权限、审计、DB），不得直接 import `modules/runtime` 或 `modules/platform_admin` 内部包。

### PluginContext（新增）

```go
type PluginContext struct {
    Logger      *zap.Logger
    DB          *gorm.DB
    Audit       AuditLogger // 操作审计，禁止写入 multipart 原文
}

type ClusterClientFactory interface {
    // 仅当 workspace 已授权该 cluster 时返回 client。
    ForWorkspace(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (kubernetes.Interface, dynamic.Interface, *rest.Config, *ClusterMeta, error)
}

type ClusterMeta struct {
    ID, Name, Server string
    PrometheusURL, PrometheusUser, PrometheusPassword string
    VelauxURL, VelauxUser, VelauxPassword string
    LLMBaseURL, LLMAPIKey, LLMModel string
}
```

`ClusterMeta` 的监控/Velaux/LLM 字段：若 `cluster_info` 尚无这些列，用 `plugin_kubernetes.cluster_settings` 按 `cluster_id` 覆盖存储，**不**复制 kubeconfig。凭证仍只来自平台集群表。

旧插件只实现 `Plugin` 时，`InstallAll` 行为不变。实现 `ContextPlugin` 的插件优先走 `RegisterWithContext`。

### 路由

前缀固定 `/api/v1/plugins/kubernetes`。

| 类别 | 路径 | 说明 |
|------|------|------|
| 集群 | `GET /clusters` | 列出当前工作区已授权集群 + settings |
| 设置 | `PUT /clusters/:clusterId/settings` | admin：Prometheus/Velaux/LLM 覆盖 |
| Proxy | `ANY /clusters/:clusterId/proxy/*path` | 转发 apiserver |
| 指标 | `POST /clusters/:clusterId/metrics` | Prometheus 查询 |
| 文件 | `POST|GET .../pods/.../files` | ls/upload/delete/download |
| 流 | `GET .../pods/.../logs` `.../exec` `.../arthas-ws` | SSE 或 WebSocket，见下 |
| 审计 | `GET|DELETE /records` | 终端录像 |
| CloudTTY | `POST /clusters/:clusterId/cloudtty` | |
| NodeTTY | `POST /clusters/:clusterId/nodetty` | 特权，仅 admin |
| Arthas | `/clusters/:clusterId/diagnosis/*` | |
| Velaux | `ANY /clusters/:clusterId/velaux/*path` | |
| Chat | `POST /clusters/:clusterId/chat/query` | SSE |

实时通道：优先 SSE（与现有 Pod 终端一致），GVA 使用 WebSocket 的能力用 SSE 或同源 WS 二选一，**同一功能只保留一种**，前端不直连后端端口绕过 BFF。流式请求必须带 JWT（query 仅作降级且与 cookie/header 同源校验），禁止只靠 `x-token` 且跳过 Casbin。

### Manifest 与 Casbin

`plugin.yaml` 的 `apis` 允许：

- 精确路径 + 大写 method（现有规则）
- **前缀规则**：`method: ANY` 且 path 以 `/*` 结尾，表示该前缀下所有方法（仅本插件需要；框架扩展校验：`ANY` 只能用于 `route_prefix` 下带 `/*` 的 path）

enable 时把前缀策略写入 Casbin，避免为每个 GVK 手写上百条 API。只读 GET/HEAD 赋给工作区成员可见角色；写方法与流式 Exec 仅赋 `governance:admin` 角色。

菜单为分组树：父菜单 `/plugins/kubernetes`，子项对齐 GVA `initialize/menu.go`，并增加 Velero（`/plugins/kubernetes/backup`）。governance bind 支持一层 parent（插件根 + 分组 + 叶子），避免全部扁平成无法在 `PlatformNav` 展示的根叶子。

### 前端

- 页面根：`fronted/apps/web/app/(platform)/plugins/kubernetes/`
- 资源目录：`src/modules/plugins/kubernetes/catalog.ts` 声明 GVK、列、namespaced、CRD group
- 通用页：列表、详情、YAML（Monaco 或现有编辑器）、Namespace 选择器、创建向导（对齐 GVA 常用工作负载表单）
- 专项页：集群选择与设置、节点监控、Pod 终端/日志/文件、审计播放、诊断、Chat、Velaux 入口
- BFF：`app/api/bff/v1/plugins/kubernetes/**`，流式用现有 `proxyUpstreamStream` 同类机制

ZIP 前端白名单保持现有前缀。若 catalog + 页面文件数超过 1000 条 ZIP 限制，安装器将 `maxZipEntries` 提高到至少 2000（kubernetes 包规模），单文件仍 ≤50 MiB（容纳 `arthas-bin.tar`）。允许插件 ZIP 内 `.tar` 作为已声明资源（路径必须在 `backend/` 下且 manifest 或固定文件名列出），禁止 `.sh/.exe` 等现有拒绝列表。

## 数据流

1. BFF 带 workspace 调用插件 API。
2. 中间件：登录 → entitlement `plugin:kubernetes` → 工作区成员；写/Exec 再 `governance:admin`。
3. `ClusterClientFactory.ForWorkspace` 校验授权后构建 client。
4. Proxy 将 `*path` 转到 apiserver；分页/keyword/多 ns 合并/旧版 Ingress CronJob 改写对齐 GVA proxy 行为。
5. 专项模块使用同一 client 或集群 settings 中的 Prometheus/Velaux/LLM。
6. 审计写入 `plugin_kubernetes.records`，平台 operation log 对 multipart 记 `[multipart omitted]`。

## 错误处理

| 条件 | 行为 |
|------|------|
| 插件未 enable | 403 `ENTITLEMENT_MODULE_DISABLED` |
| 集群未授权 | 403，不泄漏其它工作区集群 |
| 上游 404 / 无 CRD | 原样或规范化消息展示在页面 |
| 上游不可达 | 502，错误体无证书/token |
| 流中断 | 关闭并标记审计结束 |
| NodeTTY/Arthas 失败 | 删除临时 Pod/Service/port-forward |

## 数据模型（`plugin_kubernetes`）

- `cluster_settings`：`cluster_id` PK，prometheus/velaux/llm 字段，无 kubeconfig。
- `records`：终端审计（会话元数据 + 录像 blob 或对象路径）。
- `diagnosis`：Arthas 会话状态（集群、pod、端口、过期时间）。

不镜像 GVA `k8s_clusters` / `k8s_users`。不绑定 GVA `SysUser`。

平台 `cluster_info` 若缺证书导致无法连集群：插件返回明确错误，引导管理员在平台集群页补全凭证；插件 UI 不提供第二套「粘贴 kubeconfig」主路径。

## 功能范围（一次性交付）

对齐 GVA 矩阵并修复已知缺口：

- 原生：Node（含标签污点）、Workload（Deploy/DS/STS/Job/CronJob/Pod/RS）、Namespace、Network、Gateway API、Config、Storage、RBAC/PSP、Events、YAML、创建向导
- 生态：OpenKruise、Kruise Rollouts、KubeBlocks、KubeVela（proxy CRD + Velaux HTTP）、Velero 备份中心
- 可观测：Prometheus 节点/Pod 监控（查询模板对齐 GVA）
- 运行时：日志流、Exec、文件、审计播放
- 扩展：CloudTTY、NodeTTY（文档标明 HostPID/特权风险）、Arthas（二进制随插件 `backend/resource/arthas-bin.tar`）、AI Chat（集群 settings 或平台 LLM）
- 不交付：Karmada 占位

NodeTTY 镜像固定为可配置默认 `alpine:latest` + nsenter，与 GVA 行为一致。

## 与现有 runtime 的关系

`/runtime/pods` 等平台页保持不变。本插件是工作区级全集群控制台，不强制替换 runtime 的项目 Namespace 观测。二者可并存；插件不调用 runtime 内部服务。

## 测试

- `PluginContext` 与旧 `Register` 回归。
- 授权矩阵（enable、集群授权、admin 写）。
- Proxy 契约测试（fake client）。
- 专项模块 mock 测试（含失败清理）。
- schema AutoMigrate。
- 脚手架/bundled 树 `InspectZip`。
- 前端 catalog 与权限按钮、错误态组件测试。
- CI 默认不连真实集群。

## 验收

1. 启用后菜单树完整（含 Velero），SparkOps 视觉，功能对齐 GVA。
2. 仅列出已授权集群，选中后可操作全部 Namespace。
3. 原生与生态资源可列表/YAML/变更；缺 CRD 报错不藏菜单。
4. 日志、Exec、文件、CloudTTY、NodeTTY、Arthas、Velaux、Chat 可用。
5. 成员只读，governance admin 可写。
6. 内置即可用；ZIP 可通过安装静态检查。
7. 错误与审计不泄漏集群证书。
