# Filesystem 统一工作台 UI 设计

> 日期：2026-11-14  
> 状态：已批准，待实施计划  
> 适用范围：`fronted/apps/web` 的 Filesystem 用户界面及其必要 BFF/后端补充

## 1. 目标

把现有分散的文件浏览页与存储管理长页面整合为一个克制、清晰的 Filesystem 工作台。普通成员可以浏览、上传和管理文件；管理员可以在同一信息架构下查看传输、配置存储和审计事件。

成功标准：

1. `/files` 成为 Filesystem 主入口，第一屏清楚回答“我在哪、能做什么、当前状态如何”。
2. 文件、传输、存储治理、审计四类能力通过一级标签组织。
3. 文件区支持目录树、站内拖拽移动、桌面拖拽上传及网格/列表双视图。
4. 权限决定标签和操作是否可见，后端继续执行最终授权。
5. 不新增实时容量曲线，不把不存在的数据伪装成实时指标。

## 2. 设计原则

- 内容优先：文件和任务是主角，界面使用近乎无色的表面与充分留白。
- 单一焦点：工作台第一屏只有“上传文件”使用主按钮样式。
- 空间分层：使用 `stack-*`、轻阴影和圆角表达层级，避免边框套边框。
- 统一反馈：加载、空、错误、无权限四态分离。
- 可恢复操作：拖拽移动失败时恢复原位置；上传失败进入传输中心供重试。
- 不引入新 UI 组件库，复用现有 Token、`Modal`、`Drawer`、`InlineAlert`、`PageHeader`。

## 3. 信息架构

### 3.1 主入口

`/files` 使用统一工作台骨架：

```text
FilesystemWorkbench
├── PageHeader
│   ├── 标题与说明
│   └── 上传文件（唯一主操作）
├── WorkbenchTabs
│   ├── 文件
│   ├── 传输
│   ├── 存储治理
│   └── 审计
└── 当前标签内容
```

标签状态进入 URL：`/files?tab=files|transfers|governance|audit`。刷新和分享链接后保留当前标签。

### 3.2 文件标签

桌面布局：

```text
┌───────────────┬────────────────────────────────────┐
│ DirectoryTree │ Breadcrumb / Search / View Toggle │
│               ├────────────────────────────────────┤
│ 快捷访问       │ FileBrowser（List 或 Grid）        │
└───────────────┴────────────────────────────────────┘
```

窄屏下目录树收进 Drawer，文件区保持全宽。

URL 保存浏览状态：

- `parent=<nodeId>`：当前目录；
- `view=list|grid`：视图；
- `cursor=<cursor>`：分页位置；
- `q=<query>`：搜索词。

### 3.3 传输标签

展示按钮上传、拖拽上传和续传产生的任务：

- 文件名、目标目录；
- 状态与进度；
- 已传/总大小；
- 失败原因；
- 取消、重试；
- 完成后跳转到文件详情。

小型上传可在文件标签显示紧凑进度，但任务的权威状态统一由 `TransferCenter` 管理。

### 3.4 存储治理标签

把当前 `/files/admin` 的纵向大面板改为二级导航：

1. Provider 与存储池；
2. 作用域绑定；
3. 生命周期策略。

现有 `StoragePoolPanel`、`StorageBindingPanel`、`LifecyclePolicyPanel` 保留领域逻辑，重组页面布局和加载状态。普通成员无管理权限时不显示该一级标签。

### 3.5 审计标签

顶部为紧凑筛选条，主体为事件表格：

- 操作者；
- 动作；
- 资源；
- 决策结果；
- 时间；
- Trace ID。

点击事件打开 Drawer，展示完整 detail。筛选条件同步到 URL，分页继续使用 cursor。

## 4. 文件浏览交互

### 4.1 列表与网格

两种视图共享同一个数据源、选中状态和命令层：

- 列表：名称、修改时间、大小、操作；
- 网格：文件/目录图标、名称、大小或子项数、修改时间；
- 切换视图不重新请求当前页数据；
- 用户选择写入 URL，可选地持久化到 local storage 作为下次默认值；
- 两种视图都通过同一个“···”菜单执行重命名、移动、复制、删除、查看版本。

### 4.2 目录树

`DirectoryTree` 懒加载子目录，不一次性请求整个命名空间。

- 展开节点时请求其直接子目录；
- 当前目录高亮；
- 点击节点导航；
- 根目录、已展开目录均是投放目标；
- 拖拽悬停 700ms 自动展开目标目录；
- 正在加载的节点显示独立加载态。

### 4.3 站内拖拽移动

支持把单个或多选的文件/目录拖到目录树节点。

前端预校验：

- 禁止拖入自身；
- 禁止目录拖入自己的后代；
- 禁止跨 Workspace；
- 无 `file:write` 权限时不启用拖拽；
- 禁止移动已被删除或状态不可用的节点。

提交后先保持原列表，服务端成功才更新；若使用乐观 UI，则失败必须完整回滚原父节点和排序。服务端 `move` API 是最终授权和一致性边界。

### 4.4 桌面拖拽上传

当前目录主区域和目录树节点均可接收桌面文件。

- 首期支持多文件，不递归上传本地文件夹；
- 拖入时显示全区域投放遮罩和目标目录名称；
- 文件进入统一传输队列；
- 并发数受控，默认同时上传 3 个；
- 每个任务可取消或重试；
- 上传完成后刷新目标目录；
- 不要求前端显式选择默认 Provider，未指定时由后端默认 Provider 决策。

## 5. 文件详情

点击文件打开右侧 `Drawer`，避免离开当前目录。内容：

- 基本信息；
- 当前版本；
- 预览（后端可用时）；
- 版本历史；
- 下载；
- 审计摘要。

`/files/[nodeId]?mode=revisions` 保留为可直接访问的深链接和无 JavaScript 降级页面，但主交互使用 Drawer。

## 6. 权限模型

前端只负责隐藏或禁用不适用操作，不替代后端授权。

| 权限 | UI 能力 |
|---|---|
| `file:read` | 浏览、搜索、查看详情和版本 |
| `file:write` | 上传、新建、重命名、移动、复制 |
| `file:delete` | 删除/回收 |
| `storage:read` | 查看存储治理 |
| `storage:admin` | 管理 Provider、Pool 和 Binding |
| `lifecycle:admin` |管理生命周期策略 |
| `file:admin` | 查看审计和高级管理操作 |

权限未加载时不闪现管理入口；无权限页面返回明确空态而不是通用错误。

## 7. 组件边界

建议新增或重组：

```text
src/modules/filesystem/components/
├── FilesystemWorkbench.tsx
├── WorkbenchTabs.tsx
├── FileBrowser.tsx
├── FileListView.tsx
├── FileGridView.tsx
├── DirectoryTree.tsx
├── FileDropZone.tsx
├── FileActionMenu.tsx
├── FileDetailDrawer.tsx
├── TransferCenter.tsx
├── GovernanceWorkspace.tsx
└── AuditWorkspace.tsx
```

保留并逐步迁移：

- `UploadPanel` 的 SDK 调用迁入 `TransferCenter`；
- `FileExplorerPanel` 拆成工作台、树和浏览器，避免继续扩大单文件；
-现有三个治理 Panel 保持业务边界；
- `FileAuditPanel` 拆分筛选、表格和详情 Drawer。

## 8. 数据与 API

### 8.1 可复用能力

- namespace、children、search；
- create directory、rename、move、copy、trash；
- revisions；
- transfer create/get/abort；
- provider/pool/binding/lifecycle；
- filesystem audits。

### 8.2 必要补充

1. 目录树接口需保证可只返回目录，并支持 cursor；优先在现有 children 查询增加 `type=DIRECTORY`，避免新建平行接口。
2. 多选站内移动首期可逐项调用现有 move；若性能不足再新增批量 API，不提前扩展。
3. 传输列表若后端尚无 workspace 级 list endpoint，需要新增：
   `GET /api/v1/workspaces/{wsId}/transfers?status=&cursor=&limit=`。
4. 文件摘要指标只显示可从现有数据库可靠聚合的数据；不实现时序容量 API，不展示实时曲线。

## 9. 状态与错误处理

- 页面首载：骨架屏，不使用“暂无数据”替代加载。
- 空目录：提供“上传文件”和“新建目录”引导。
- API 错误：`InlineAlert` 给出原因和可行动建议。
- 删除等危险动作：使用 `Modal` 二次确认。
- 重命名、新建目录：`Modal`，提交时禁用并显示进行中文案。
- 拖拽冲突：目标节点旁显示拒绝态，释放不会发请求。
- 部分上传失败：成功任务不回滚，失败任务留在队列。
- 网络恢复：可重试任务保留必要 session 信息，继续使用现有续传能力。

## 10. 可访问性与响应式

- 标签符合 tab/tabpanel 语义并支持键盘切换；
- 目录树符合 tree/treeitem 语义，支持方向键展开和导航；
- 拖拽必须有键盘替代方案：“移动到…” Modal；
- 图标按钮提供 `aria-label`；
- Drawer/Modal 管理焦点并支持 Esc；
- 进度使用 `role="progressbar"`；
- 小屏隐藏指标卡，目录树进入 Drawer，操作栏可换行。

## 11. 测试策略

### 单元测试

- URL 状态解析与序列化；
- 目录树懒加载和展开；
- 拖入自身/子孙/跨 Workspace 的拒绝规则；
- 列表/网格共享选择状态；
-权限到标签/操作的映射；
- 多文件上传队列的并发、取消和重试。

### 组件测试

- 工作台四标签；
- 列表/网格切换；
- 拖拽移动成功与失败回滚；
- 桌面拖入显示目标并创建任务；
-详情 Drawer 与版本；
- 管理权限裁剪；
- 审计筛选和详情。

### 浏览器验收

1. 普通成员完成浏览、新建、上传、移动、切换视图；
2. 管理员完成 Provider/Pool/Binding/Lifecycle 操作；
3. 审计可定位上述操作并显示 trace；
4. 窄屏可通过 Drawer 使用目录树；
5. 键盘用户可完成“移动到…”而无需拖拽。

## 12. 明确不做

- 实时容量曲线及 SSE 容量推送；
- 首期递归上传本地文件夹；
- 同屏展示多套重复管理面板；
- 引入新的 UI 库；
- 为视觉效果伪造后端尚未提供的指标。

## 13. 交付顺序

1. 工作台骨架、标签与 URL 状态；
2. FileBrowser 拆分及列表/网格视图；
3. DirectoryTree 和站内拖拽；
4. 桌面拖拽上传与 TransferCenter；
5. 文件详情 Drawer；
6. 存储治理重组；
7. 审计重组；
8. 响应式、可访问性和浏览器验收。
