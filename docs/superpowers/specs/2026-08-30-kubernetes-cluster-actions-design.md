# Kubernetes 集群授权、关联与删除

对齐 GVA 集群表操作：**授权、关联、变更、删除**。写操作仅 `governance:admin`。

## 表格

搜索区与操作区分离。表头含复选框。行操作：授权、关联、变更、删除。工具栏：新增、批量删除。

## 授权

弹窗三页：

- **连接信息**：导出当前工作区可用 kubeconfig（复制/下载）。数据来自平台已存凭证，不另存私钥副本。
- **用户**：列出 ClusterRoleBinding；可创建 ServiceAccount + Binding，可删除 Binding。
- **角色**：列出 ClusterRole；可用 YAML 创建/更新/删除。

## 关联

弹窗通过现有 Velaux 代理访问 `/api/v1/clusters`。未配置 Velaux 时提示去「变更」填写地址。支持刷新、添加（名称 + kubeconfig）、删除。

## 变更

同一创建表单。可改 API 地址/凭证（同名复用登记）及 AI/Velaux/Prometheus settings。

## 删除

仅撤销当前工作区授权，不删除 `cluster_info`。批量删除同语义。

## 非目标

- GVA 证书签发 CSR 流水线与独立用户库。
- 全局物理删除平台集群。
