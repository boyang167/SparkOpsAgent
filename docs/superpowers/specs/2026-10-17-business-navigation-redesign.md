# SparkOps 业务控制台菜单重构设计

## 目标

将业务控制台侧栏调整为面向企业用户的任务型导航，让用户按“产品交付、运行排障、集群管理、知识与配置”快速找到入口，同时兼容现有工作区、角色权限、书签和深链接。

## 最终菜单

```text
工作台

产品交付
├── 产品
├── 项目
├── 发布计划
├── 工作流
├── 定时构建
└── 模板中心

运行观测
├── 运行实例
├── 监控概览
└── 日志查询

Kubernetes
├── 集群管理
├── 工作负载
├── 节点管理
├── 命名空间
├── 网络与网关
├── 配置与存储
├── 安全与访问
└── 运维工具

知识库

配置与集成
├── 密钥管理
├── 外部集成
└── MCP 集成
```

## 兼容策略

### 路由

保留全部现有 URL，不重命名业务路由：

- 产品继续使用 `/publish/products`
- 项目继续使用 `/projects`
- 发布计划继续使用 `/publish/release-plans`
- 工作流继续使用 `/products/workflows`
- 定时构建继续使用 `/cronjobs`
- 模板中心继续使用 `/templates`
- 运行观测、知识库、配置与 Kubernetes 现有路径全部保留

这样可保证已有书签、通知链接、页面守卫和 API entitlement 不失效。

### 菜单数据与角色权限

复用已有菜单记录和菜单 ID，只调整 `title`、`parent_id` 与 `sort`。由于角色菜单关系绑定的是菜单 ID，重新挂载不会丢失已有角色授权。

沿用现有根分组 ID，降低已有工作区迁移风险：

- `application` 改为“产品交付”，吸收原“交付”分组内容
- `ops-runtime` 改为“运行观测”，吸收原“观测”分组内容
- `ops-infra` 改为“配置与集成”
- 迁移完成后软删除空的 `ops-delivery` 与 `ops-observability` 根分组

`/knowledge` 提升为根级直接入口。后端菜单树仍负责 RBAC 与 entitlement 过滤，前端不绕过权限硬编码显示。

`/config/mcp` 补入默认菜单与角色策略；无 MCP entitlement 时仍由现有过滤逻辑隐藏。

### 定时构建 entitlement

本次只改变信息架构与显示名称，`/cronjobs` 暂时保留现有 `k8s` entitlement，不在菜单重构中改变许可证语义。后续如需改为 `delivery`，应作为独立授权迁移处理。

## Kubernetes 收敛

侧栏只保留八个高频入口，现有页面路由不删除：

- “网络与网关”以 `/plugins/kubernetes/networks` 为侧栏入口，页面内提供网络与 Gateway 切换
- “配置与存储”以 `/plugins/kubernetes/configs` 为侧栏入口，页面内提供配置与存储切换
- “安全与访问”以 `/plugins/kubernetes/access-controls` 为侧栏入口，页面内提供访问控制与终端审计切换
- “运维工具”以 `/plugins/kubernetes/diagnosis` 为侧栏入口，页面内提供诊断、CloudTTY、备份、AI Chat 与扩展组件入口

原 Gateway、存储、终端审计及各扩展组件路由继续可访问，并映射到对应聚合入口的侧栏选中态。插件 manifest 不再把这些深层页面平铺到侧栏。

## 前端交互

- 工作台、知识库为根级直接入口
- 产品交付默认展开
- 当前路由所属分组自动展开
- 配置与集成默认折叠并位于底部
- Kubernetes 深层页面保持对应聚合入口高亮
- 动态菜单不可用时，静态 fallback 保持同样的信息架构
- 命令搜索、最近访问和路由标签同步使用新名称

## 影响范围

### 中等风险

- 后端治理菜单 seed/repair：已有工作区菜单重挂载、排序及旧根清理
- Kubernetes bundled manifest：启用插件的工作区会在启动时重新绑定菜单
- 顶层知识库：动态菜单转换需要支持根级叶子节点

### 低风险

- 前端 fallback 菜单、图标和显示文案
- 命令搜索与最近访问标签
- 分组默认展开与 active 自动展开

### 不在本次范围

- 不改变业务页面 URL
- 不改变 API 路由
- 不删除 Kubernetes 页面
- 不强制覆盖管理员已有的角色授权
- 不改变 `/cronjobs` entitlement
- 不调整管理控制台与平台运营中心菜单

## 验证

1. 新工作区直接生成最终菜单结构。
2. 旧工作区首次拉取菜单后完成重挂载，无重复分组且角色可见性不变。
3. 动态菜单 API 失败时，fallback 结构与生产菜单一致。
4. Kubernetes 插件启用后只显示八个入口，所有原深层路由仍可访问。
5. 进入 Gateway、存储、终端审计及运维扩展页面时，对应聚合入口保持高亮。
6. 无 knowledge、delivery、k8s、observability 或 mcp entitlement 时，对应入口正确隐藏。
7. 产品、项目、发布计划、工作流和定时构建的原有链接继续有效。
