# 文件中心菜单与权限集成设计

## 目标

将现有 Filesystem 工作台作为“文件中心”集成到业务控制台：

- 左侧一级菜单位于“知识库”下方，路径 `/files`，排序 `96`，图标 `Folder`。
- `/files` 内继续保留文件、传输、存储治理、审计四个 Tab，不拆分侧边菜单。
- 所有启用 `filesystem` entitlement 的现有及新工作区自动获得菜单，无需逐工作区配置。
- `developer`、`operator` 可读取、上传、移动和删除文件；`admin` 额外管理 Provider、Storage Pool、Lifecycle 与 Audit。
- 菜单隐藏、页面入口和后端 API 使用一致的 entitlement 与 RBAC 规则。

## 菜单集成

扩展治理模块现有的幂等菜单种子，不新增直接操作治理表的 SQL：

- 在根菜单默认项中加入：
  - 标题：`文件中心`
  - 路径及组件：`/files`
  - 排序：`96`
  - 图标：`Folder`
  - `hidden`：`false`
- 新工作区在 bootstrap 时创建该菜单。
- 现有工作区在治理模块启动修复时补齐该菜单。
- 若菜单已存在，保留其 ID 和已有角色绑定；同步标准标题、根级位置、排序、图标与组件。
- 菜单默认角色策略为 `admin`、`developer`、`operator`。已有非空菜单角色策略不覆盖。
- 前端 fallback、路由标签和命令搜索统一使用“文件中心”。

## Entitlement

`filesystem` 默认仍为 `true`，但显式配置为 `false` 的平台或工作区不被迁移强制开启。

- 前后端路径映射均将 `/files` 识别为 `filesystem`。
- 动态菜单和前端 fallback 均按 entitlement 过滤。
- 服务端页面布局在模块关闭时跳转到“模块不可用”页面。
- Filesystem 后端 API 继续以 entitlement 中间件作为模块级安全边界。

## RBAC

默认角色权限如下：

- `developer`、`operator`：`file:read`、`file:write`、`file:delete`。
- `admin`：保留完整文件权限，并包含 `storage:read`、`storage:admin`、`lifecycle:admin`、`file:admin`。
- `reviewer`、`viewer`：不自动授予文件权限。

启动修复对现有 `developer`、`operator` 仅增量加入上述文件权限，不删除其他权限。自定义角色不修改。

前端不再在权限读取失败时虚构 `file:read`：

- 超级管理员继续获得工作台管理视图权限。
- 普通用户必须实际拥有 `file:read` 才能进入 `/files`。
- 缺少权限时页面显示明确的 403/无权访问状态，且不调用文件数据 API。
- 管理 Tab 继续按 `file:admin` 和存储权限控制；后端 API 始终执行最终授权。

## 验证

采用测试先行覆盖：

1. 菜单默认定义、现有工作区修复、标题/排序/图标及 ID 保留。
2. `/files` 默认菜单角色策略及已有策略保留。
3. 新角色默认权限和现有角色增量权限，确保不修改 reviewer、viewer、自定义角色。
4. 后端 `/files` entitlement 路径映射。
5. 前端 fallback 名称、路由标签、命令搜索和权限解析。
6. `/files` 无 `file:read` 时显示拒绝状态且不加载 namespace/files；有权限时保持现有四 Tab。

## 非目标

- 不拆分文件、传输、治理、审计为多个侧边菜单。
- 不新增实时容量曲线。
- 不改变文件工作台现有四 Tab 的交互设计。
- 不覆盖管理员已配置的非空菜单角色策略。
