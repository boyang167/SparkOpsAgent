# Kubernetes Pod 详情 UI 设计

## 目标

将工作负载页中的 Pod 详情从原始 JSON 展示改为与参考 GVA 页面一致的结构化只读弹窗。Pod 名称点击和「更多 → 详情」必须进入同一个详情组件，展示最新 Pod 数据与关联事件。

参考页面：`http://localhost:8082/#/layout/kubernetes/workloads`

## 范围

本次只实现 Pod 详情弹窗：

- 基本信息
- Conditions
- 容器
- 事件
- 创建者
- 初始化容器
- 存储

不修改现有监控、文件、终端、日志、YAML 编辑和删除能力；不新增后端 API，不增加 Network、Filesystem 或 TCP 指标。

## 交互

- 点击 Pods 表格中的名称打开详情。
- 点击「更多 → 详情」打开同一个详情。
- 弹窗标题为「详情」，宽度使用现有 `Modal` 的大尺寸，视觉宽度接近参考页的 60%。
- 每次打开均获取最新 Pod，而不是直接使用列表行快照。
- 关闭弹窗时取消仍在进行的请求。
- 切换集群、命名空间、页签或 Pod 时，旧请求不得覆盖新详情。

## 数据流

打开详情时并行请求：

1. Pod：
   `GET /api/bff/v1/plugins/kubernetes/clusters/{clusterId}/proxy/api/v1/namespaces/{namespace}/pods/{pod}`
2. Events：
   `GET /api/bff/v1/plugins/kubernetes/clusters/{clusterId}/proxy/api/v1/namespaces/{namespace}/events?fieldSelector=involvedObject.name={pod}`

Pod 请求失败时显示错误，不打开空详情。Events 请求失败时仍显示 Pod 详情，并在事件页签显示局部错误。

## 页面结构

### 基本信息

两列描述布局：

- 名称 / 命名空间
- 状态 / 创建时间
- 节点 / Pod IP
- 标签 / 注解

标签和注解以可换行的键值徽章展示；无数据时显示 `-`。

### Conditions

表格列：

- 类型
- 状态
- 更新时间
- 内容（reason）
- 消息（message）

### 页签

固定顺序：

1. 容器
2. 事件
3. 创建者
4. 初始化容器
5. 存储

#### 容器与初始化容器

表格列：名称、镜像、端口。每行可展开显示：

- 镜像拉取策略
- 环境变量
- 启动命令与参数
- CPU / 内存 requests
- CPU / 内存 limits
- 挂载点

#### 事件

表格列：

- 类型
- 对象（kind:name）
- 信息（message）
- 内容（reason）
- 时间

事件按 `lastTimestamp`、`eventTime` 或 `metadata.creationTimestamp` 倒序显示。

#### 创建者

读取 `metadata.ownerReferences`，表格列为名称、类型。

#### 存储

读取 `spec.volumes`，表格列为名称、类型、详情。支持识别并展示常见来源：

- hostPath
- configMap
- secret
- persistentVolumeClaim
- emptyDir
- downwardAPI
- projected

未知来源显示第一个来源键及其 JSON 摘要。

## 组件边界

- 新建 `PodDetailView.tsx`：只负责结构化渲染，不自行请求。
- 新建 `pod-detail-view.ts`：Pod 详情格式化、事件排序、存储来源识别等纯函数。
- `WorkloadsPanel`：统一管理详情 Modal、Pod 与 Events 请求、取消和错误。
- `WorkloadPodActions`：移除自身 JSON 详情 Modal，通过 `onOpenDetail(pod)` 通知父组件打开统一详情。

## 错误与权限

- 详情为只读，不要求 `canManage`。
- Pod 请求错误使用现有 `ApiErrorBanner`。
- Events 错误限制在事件页签内，不影响其他页签。
- 所有文本由 React 转义；未知对象只通过 `JSON.stringify` 显示，不注入 HTML。

## 测试

- 纯函数测试：事件时间排序、容器端口/资源格式、ownerReferences、常见 volume 类型与详情。
- 组件类型检查：`PodDetailView` 的可选字段与空状态。
- 回归测试：Kubernetes 前端测试集。
- 页面验收：名称与「更多 → 详情」进入同一弹窗，页签顺序正确，Events 失败时其他详情仍可用。

## 非目标

- 独立详情路由或深链接。
- 编辑、删除等写操作搬入详情。
- 监控图表、文件浏览器、终端或日志 UI 重做。
- 新增或修改后端路由。
