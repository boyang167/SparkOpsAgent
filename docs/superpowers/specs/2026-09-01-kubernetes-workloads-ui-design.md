# Kubernetes 工作负载页 UI 设计

对齐参考页 `http://localhost:8082/#/layout/kubernetes/workloads` 的布局、页签、列与操作入口。数据与写操作走现有 SparkOps BFF / Kubernetes proxy / Pod 终端 / 日志 / 文件 / 指标 API。不嵌入 Vue，不引入 Element Plus。

## 目标

将 `/plugins/kubernetes/workloads` 从通用 `ResourceTable` + 手工 `OperationsPanel` 改为专用 `WorkloadsPanel`，视觉与交互与参考页一致，技术栈保持现有 React / SparkOps 组件。

集群选择继续由 `KubernetesConsole` 的 `ClusterPicker` 提供。页内命名空间选择、CloudShell、AI 助手、六页签表格与参考页同序。

## 页面结构

自上而下：

1. 工具栏：命名空间下拉（含「All Namespaces」）、CloudShell、AI 助手。
2. 页签（英文标签，固定顺序）：Deployments、DaemonSets、StatefulSets、Jobs、CronJobs、Pods。
3. 当前页签工具栏：名称输入、「查询」、「重置」；「YAML」与「新增」（Pods 仅「新增」）。
4. 数据表（含复选框列）。
5. 分页：`10 / 30 / 50 / 100`，默认 10，显示总数、上一页、页码、下一页。

`ResourceMenuPage` 在 `menu === "workloads"` 时渲染 `WorkloadsPanel`，不再渲染通用表格标题卡、`ResourceTable`，也不再挂载底部 Pod 运维与指标调试面板。`ReplicaSet` 仍保留在 catalog，但不出现在本页页签。

## 数据加载

每个页签独立 `keyword` / `page` / `pageSize` 状态，切页签不互相覆盖。列表走 `resourceCollectionUrl` + `k8sFetch`。

「All Namespaces」对应集群范围 list（proxy 路径不含 `namespaces/{ns}`）。指定命名空间时路径含该 NS。关键词映射现有 `keyword` 查询参数。

切换集群、命名空间、页签、查询条件时重置到第 1 页。

## 控制器页签（Deployments / DaemonSets / StatefulSets / Jobs / CronJobs）

公共列：复选、名称（蓝色可点，旁有标签提示）、命名空间、存活时间、操作。

专属列：

| 页签 | 专属列 |
|------|--------|
| Deployments / StatefulSets | 容器组数量：`readyReplicas / replicas`，全部就绪为成功徽章，否则危险徽章 |
| DaemonSets | 准备就绪、当前调度、期望调度 |
| Jobs | 状态（active / succeeded / failed 徽章）、耗时 |
| CronJobs | 调度、最后调度时间、暂停 |

操作：YAML、编辑、删除。写操作仅 `canManage`；无权限时按钮灰色禁用。删除前确认「确定要删除吗？」。

点击名称打开详情弹窗：浅色 JSON/YAML 只读面板即可（不复刻 GVA 多 Tab 伸缩/镜像/回滚）。YAML 与编辑均打开现有浅色 `YamlEditor`；标题分别为 YAML 与「编辑」。新增打开该 Kind 的空对象模板，保存时 POST 到集合路径。

## Pods 页签

列顺序：复选、名称（图标 + 可点名 + 标签）、CPU 申请/限制、内存 申请/限制、容器就绪点、CPU/内存/TCP、状态、重启次数、命名空间、Pod IP、调度节点、存活时间、操作。

- CPU/内存申请限制复用 `podResourceSummary` / `sumPodResources`；超限标红。
- 容器点：ready 绿、waiting 红、terminated/其他灰，tooltip 含容器名与原因。
- CPU/内存/TCP：调用现有 `POST .../metrics` 的 `pod_cpu` / `pod_memory` 模板；无数据时显示 `-`，不阻塞列表。
- 状态：Running 成功徽章；Succeeded 显示 Completed；其他危险徽章。
- 重启次数复用 `podRestartCount`。

行操作：编辑（浅色 YAML）；「更多」用操作面板 Modal（避免 overflow 裁切），项为：详情、每个容器「{name} 文件浏览」、监控、每个容器「{name} 终端」、每个容器「{name} 终端日志」、删除。

- 终端：现有 `PodTerminal`（黑底交互 WS）。
- 日志：现有 SSE 日志面板。
- 文件：现有 files list/upload/download/delete API，弹层内完成。
- 监控：弹层内用 metrics 模板展示 CPU/内存 JSON 或简单数值，不新做图表库。
- 详情：只读 JSON/YAML 面板。

## 共享能力

- CloudShell / AI 助手：复用 `CloudTTYPage`、`ChatPage`，与节点页相同 Modal。
- YAML 编辑器保持白底；终端保持黑底。
- 存活时间复用 `formatNodeAge` 同类相对时间。
- 错误用 `ApiErrorBanner`。
- 不修改 `NodesPanel`、`PodTerminal` 协议、`YamlEditor` 浅色主题。

## 非目标

- GVA 创建向导表单树、Deployment 伸缩/镜像/暂停/ReplicaSet 回滚 Tab。
- 将日志改为 xterm WebSocket。
- 嵌 Vue / Element Plus / Monaco。
- 复刻参考源码中的 type 笔误（CronJob 当 jobs 等）。
- 为工作负载页新增后端路由（终端、日志、文件、指标已存在）。
