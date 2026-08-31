# Kubernetes 工作区成员授权

将集群「授权 → 用户」改为给 **当前工作区成员** 授权访问该集群，对齐 GVA 表格与操作，凭据使用 ServiceAccount Token。

## 界面

工具栏：添加、删除、刷新。表格：复选框、邮箱、显示名称、凭据 / 变更 / 删除。

添加：多选工作区成员；选择 ClusterRole；可添加多组「命名空间 + Role（ClusterRole 名）」。

## 后端

- 表 `plugin_kubernetes.cluster_user_grants` 保存成员与角色 JSON，不存 Token。
- 授权时在命名空间 `sparkops` 创建独立 ServiceAccount，并创建托管 ClusterRoleBinding / RoleBinding。
- 凭据：TokenRequest，有效期 24 小时，返回 kubeconfig。
- 删除：删除授权记录与托管 K8s 对象。
- 写操作仅 `governance:admin`。对象带 SparkOps 管理标签。
