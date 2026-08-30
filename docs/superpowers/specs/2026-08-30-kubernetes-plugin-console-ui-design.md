# Kubernetes 插件控制台 UI

对齐 GVA `http://localhost:8082/#/layout/kubernetes/clusters` 的信息架构与管理台布局，使用 SparkOps 组件，不嵌入 Vue。

## 目标

- 左侧出现 **Kubernetes管理** 分组，展开全部插件子菜单（集群、节点、工作负载等）。
- 集群管理页为搜索 + 表格 + 分页 + 行操作。
- 其它 Kubernetes 页使用同一套工具栏/表格风格。
- 集群登记、删除、工作区授权仍走平台页，插件内不造不可用按钮。

## 导航

动态菜单树当前是「插件 → Kubernetes管理」，前端只渲染一层子节点，导致子菜单丢失。

规则：若某节点的子项仍有可见子菜单，则把该节点提升为侧栏分组，孙子节点作为分组叶子。`/plugins` 仅在还有无子菜单的插件叶子时保留。

`/plugins/kubernetes` 重定向到 `/plugins/kubernetes/clusters`。分组标题不可点进总览。

## 集群页

列：ID、名称、Prometheus、操作。

工具栏：名称搜索、查询/重置、**新增**（治理管理员）。

**新增**弹窗对齐 GVA：名称、KubeConfig/Token 凭证、API 地址、AI / Velaux / Prometheus。提交时调用工作区注册集群 API（自动授权当前工作区），再写入插件 settings。

操作：

- **配置**：编辑 Prometheus / Velaux / LLM。
- **删除**：撤销当前工作区授权。

## 非目标

- 像素级复刻 Element Plus。
- GVA 式集群内 RBAC「授权」弹窗。
- 强制废弃平台 `/cluster/list`（仍可作平台总览）。
