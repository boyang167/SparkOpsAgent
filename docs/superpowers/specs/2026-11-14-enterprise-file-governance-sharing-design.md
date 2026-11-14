# SparkOps 企业文件治理与共享设计

> 日期：2026-11-14  
> 状态：已确认  
> 方案：A — 多类型 Namespace + 统一 Resource Grant  
> 关联设计：[企业级统一文件基础设施设计](./2026-11-12-enterprise-unified-file-infrastructure-design.md)

## 1. 目标

在现有文件中心的工作区 Namespace、Node、Revision、Blob、Resource Grant 和 Storage Fabric 基础上，建立租户、工作区和个人三类文件空间，并提供企业级共享、审批、审计、配额和生命周期治理。

目标能力：

1. 每个租户拥有一个由 Tenant Owner 或租户文件管理员治理的租户空间。
2. 租户文件可只读发布给全部工作区，也可授权给指定一个或多个工作区。
3. 每个工作区拥有一个协作空间，顶层目录决定成员可见范围，子目录默认继承。
4. 每个用户在每个租户下拥有一个默认私有的个人空间。
5. 个人文件可共享给用户、工作区或租户；扩大到整个工作区或租户时执行审批策略。
6. 共享采用实时引用，不复制 Blob、不改变文件身份和物理存储位置。
7. 跨租户实时共享禁止；跨租户协作使用受控 Export/Import。

## 2. 当前实现基线

当前实现提供：

- `filesystem.namespaces`，一个工作区对应一个 Namespace。
- Node、Revision、Blob、Replica 分层模型。
- 文件和目录级 `ResourceGrant`，支持权限、Allow/Deny、过期时间。
- `file:read`、`file:write`、`file:delete`、`file:share`、`file:admin` 权限码。
- ORG、WORKSPACE、PROJECT、USER 四类存储池绑定作用域。
- 文件中心只加载当前工作区 Namespace，并显示单棵“全部文件”目录树。

当前缺口：

- Namespace 没有 `ORGANIZATION`、`WORKSPACE`、`USER` 类型。
- Resource Grant 的受让方只有 `grantee_id`，不能区分用户、工作区、组织和角色。
- 没有“与我共享”、租户发布、审批和有效权限解释接口。
- Tenant Owner 没有租户文件治理入口。
- 文件上传 Placement 尚未完整消费 Storage Binding。

本设计扩展现有模型，不建立第二套文件系统，也不改变 Node、Revision、Blob 的身份边界。

## 3. 核心不变量

1. Namespace 是所有权、配额、生命周期和默认策略边界。
2. Node 所属 Namespace 创建后不因共享而改变。
3. 共享不创建 Node、Revision、Blob 或 Replica 副本。
4. 文件共享后仍由源 Namespace 计量配额和执行生命周期。
5. 撤销共享后访问立即失效；缓存不得延长授权有效期。
6. 显式 Deny 优先于任何 Allow。
7. 目录授权默认向子孙节点继承，直接授权优先于较远父目录授权。
8. Tenant Owner 不自动拥有个人 Namespace 的读取权限。
9. 普通用户不得绕过审批将个人或工作区文件发布给整个租户。
10. 跨 Organization 不共享 Node 或 Blob 引用。

## 4. Namespace 模型

### 4.1 类型

新增 `NamespaceType`：

```text
ORGANIZATION
WORKSPACE
USER
```

建议字段：

```text
filesystem.namespaces
- id
- namespace_type
- organization_id
- workspace_id       nullable
- owner_user_id      nullable
- root_node_id
- status
- policy_version
- created_at
- updated_at
```

唯一性约束：

```text
UNIQUE (organization_id)
  WHERE namespace_type = 'ORGANIZATION'

UNIQUE (workspace_id)
  WHERE namespace_type = 'WORKSPACE'

UNIQUE (organization_id, owner_user_id)
  WHERE namespace_type = 'USER'
```

完整性约束：

- `ORGANIZATION`：必须有 `organization_id`，不得有 `workspace_id` 和 `owner_user_id`。
- `WORKSPACE`：必须有 `organization_id` 和 `workspace_id`，不得有 `owner_user_id`。
- `USER`：必须有 `organization_id` 和 `owner_user_id`，不得有 `workspace_id`。

### 4.2 租户空间

每个 Organization 一个 Namespace。

- 默认仅 Tenant Owner 和 `org:file:admin` 可见。
- Tenant Owner 可委派租户文件管理员。
- 对工作区开放必须创建显式 Grant。
- 普通用户只看见授权覆盖的节点，不看见未发布的租户目录。

推荐目录示例：

```text
租户文件
├── 制度与规范      所有工作区只读
├── 公共模板        所有工作区只读
├── 财务资料        财务工作区
├── 研发规范        指定研发工作区
└── 管理区          Tenant Owner / 文件管理员
```

### 4.3 工作区空间

保留现有每个 Workspace 一个 Namespace。

工作区根目录本身不向成员暴露统一的隐含全读权限。每个顶层目录必须由 Workspace Owner 或 `file:admin` 选择策略：

```text
WORKSPACE_MEMBERS
ROLE_LIST
USER_LIST
ADMIN_ONLY
```

子目录默认继承顶层策略。需要收窄时使用显式 Deny 或更具体的受众 Grant。

推荐目录示例：

```text
工作区文件
├── 公共文件        全体工作区成员
├── 项目资料        指定角色或成员
├── 管理资料        Workspace Owner / Admin
└── 受限资料        显式授权
```

个人私密内容不得依赖工作区目录隐藏，应保存在用户个人 Namespace。

### 4.4 个人空间

每个用户在每个 Organization 下一个 Namespace，首次进入“我的文件”时幂等创建。

- 默认仅 `owner_user_id` 可见。
- 用户拥有个人空间内 Node 的管理权，但共享范围仍受组织策略约束。
- Tenant Owner 默认不能浏览个人文件。
- 平台管理员的受控访问必须产生高风险审计事件。
- 用户冻结或离职后，个人 Namespace 进入 `FROZEN`，禁止新写入，等待移交。

## 5. 统一 Resource Grant

### 5.1 受让主体

扩展现有 Resource Grant，新增 `principal_type`：

```text
USER
WORKSPACE
ORGANIZATION
ROLE
SERVICE_ACCOUNT
AGENT
```

`principal_id` 的语义由 `principal_type` 决定。文件域只保存外部主体引用，不复制 identity、tenant 或 governance 主数据。

### 5.2 数据结构

```text
filesystem.resource_grants
- id
- organization_id
- node_id
- principal_type
- principal_id
- actions
- effect
- inherit_to_children
- expires_at
- approval_status
- approved_by
- approved_at
- reason
- created_by
- policy_version
- created_at
- revoked_at
```

`actions` 支持：

```text
READ
DOWNLOAD
WRITE
DELETE
SHARE
ADMIN
```

第一版写入时按单 action 一行保存，以兼容现有 `permission` 字段和唯一约束；API 使用 actions 数组，由应用层拆分或聚合。

### 5.3 共享表达

租户目录发布给全部工作区：

```text
principal_type = ORGANIZATION
principal_id = organization_id
actions = [READ, DOWNLOAD]
```

授权语义为：当前用户必须是该 Organization 下任意有效 Workspace 的成员，且当前请求带有有效 Workspace 上下文。

租户目录发布给指定工作区：

```text
principal_type = WORKSPACE
principal_id = workspace_id
actions = [READ, DOWNLOAD]
```

多个工作区创建多条 Grant，便于单独撤销和审计。

个人文件共享给工作区：

```text
principal_type = WORKSPACE
principal_id = workspace_id
actions = [READ, DOWNLOAD]
```

工作区目录共享给角色：

```text
principal_type = ROLE
principal_id = role_id
actions = [READ]
```

### 5.4 实时引用

共享视图保存 Grant 关系，不创建目标 Namespace 下的别名 Node。

- URL 和 API 使用原 `node_id`。
- 源 Revision 更新后，共享方读取最新 Revision。
- 源节点重命名后，共享视图同步显示新名称。
- 源节点进入回收站后，共享入口标记不可用。
- 源节点恢复后，未撤销且未过期的 Grant 恢复生效。
- 源节点永久清除前必须检查法务保留和共享保留策略。

## 6. 授权决策

每次文件操作依次执行：

1. Authentication：验证 Human、Service Account 或 Agent 身份。
2. Tenant Boundary：请求 Organization 与 Namespace Organization 必须一致。
3. Entitlement：Organization 已启用 filesystem。
4. Platform RBAC：验证对应功能权限码。
5. Namespace Authority：计算个人 Owner、Workspace 管理员或 Tenant 文件管理员权限。
6. Membership：解析用户的 Organization、Workspace 和 Role 关系。
7. Resource Grant：计算节点直接 Grant 和父目录继承 Grant。
8. Classification Policy：验证文件分类允许目标动作和共享范围。
9. Explicit Deny：任一有效 Deny 拒绝请求。
10. Default Deny：没有有效 Allow 时拒绝。

有效权限解释结果必须返回：

```text
decision
effective_actions
reason_code
source_grant_ids
policy_version
trace_id
```

优先级：

```text
Explicit Deny
> 节点直接 Grant
> 最近父目录 Grant
> Namespace 默认管理权
> 平台角色默认能力
```

`file:read` 等平台权限只代表“允许使用该能力”，不能代替资源级授权。

## 7. 分享审批

### 7.1 不需要审批

- 个人文件分享给同一租户内指定用户，只读。
- 文件分享给当前工作区内指定用户，只读。
- Workspace Admin 在已授权顶层目录内发布只读内容。

### 7.2 Workspace Admin 审批

- 个人文件共享给整个工作区。
- 普通成员扩大工作区目录受众。
- 授予整个工作区 WRITE 权限。

### 7.3 Tenant Owner 或租户文件管理员审批

- 个人文件共享给整个 Organization。
- 工作区文件发布给其他工作区。
- 工作区文件发布给整个 Organization。
- `CONFIDENTIAL` 文件扩大访问范围。

### 7.4 默认禁止

- `RESTRICTED` 文件发布给整个 Organization。
- 跨 Organization 实时共享。
- 向 Organization 主体授予 DELETE、SHARE 或 ADMIN。
- 匿名公网链接。

审批实体：

```text
filesystem.share_requests
- id
- organization_id
- node_id
- requested_by
- principal_type
- principal_ids
- actions
- inherit_to_children
- expires_at
- status
- reviewed_by
- reviewed_at
- rejection_reason
- created_at
```

批准操作在同一事务内更新请求状态、创建 Grant 和写入 Outbox。

## 8. 文件中心信息架构

文件 Tab 左侧改为：

```text
文件空间
├── 我的文件
├── 当前工作区
├── 租户文件
└── 与我共享
```

### 8.1 我的文件

- 加载当前用户在当前 Organization 下的 USER Namespace。
- 默认显示“仅自己可见”。
- 已共享节点展示受众数量和最高权限。
- 分享操作支持用户、工作区和 Organization。

### 8.2 当前工作区

- 加载全局 Workspace 上下文对应的 WORKSPACE Namespace。
- 顶层目录展示“全体成员、指定角色、指定成员、仅管理员”策略徽标。
- 普通成员只能看到有效授权覆盖的目录。

### 8.3 租户文件

- Tenant Owner 和租户文件管理员看见完整树及发布状态。
- 普通成员只看见发布给当前 Workspace、当前 Role、当前用户或整个 Organization 的节点。
- 页面明确显示当前 Organization 和 Workspace，避免受众语义不清。

### 8.4 与我共享

聚合 Grant 命中的入口，按来源分组：

```text
租户发布
其他用户共享
工作区共享
Agent / Workflow 产物
```

该页面是查询投影，不创建文件副本。

## 9. API

保留现有 `/api/v1/workspaces/{workspaceId}/namespaces` 兼容接口，其内部解析为 WORKSPACE Namespace。

新增统一接口：

```text
GET  /api/v1/filesystem/namespaces
GET  /api/v1/filesystem/namespaces/{namespaceId}/children
GET  /api/v1/filesystem/shared-with-me
GET  /api/v1/filesystem/nodes/{nodeId}/effective-access

GET  /api/v1/filesystem/nodes/{nodeId}/grants
POST /api/v1/filesystem/nodes/{nodeId}/grants
DELETE /api/v1/filesystem/nodes/{nodeId}/grants/{grantId}

POST /api/v1/filesystem/nodes/{nodeId}/share-requests
POST /api/v1/filesystem/share-requests/{requestId}/approve
POST /api/v1/filesystem/share-requests/{requestId}/reject

POST /api/v1/filesystem/nodes/{nodeId}/adopt
```

`GET /namespaces` 只返回当前主体可进入的空间及能力摘要，不返回不可见 Namespace。

`GET /shared-with-me` 支持：

```text
workspace_id
source_type
cursor
limit
q
```

结果去重规则：同一 `node_id` 只返回一次，合并所有有效权限并附带命中的 Grant 来源。

## 10. 存储、配额与生命周期

### 10.1 Placement

新文件按所属 Namespace 解析 Storage Binding：

```text
USER Namespace:
  USER > ORGANIZATION > SYSTEM

WORKSPACE Namespace:
  WORKSPACE > ORGANIZATION > SYSTEM

ORGANIZATION Namespace:
  ORGANIZATION > SYSTEM
```

PROJECT Binding 只用于明确关联 Project 的 Artifact 或上传请求，不作为普通 Namespace 的隐式覆盖。

共享不触发 Placement、不迁移 Replica。目标 Workspace 即使绑定不同 Pool，也读取源文件已有 Replica。

### 10.2 配额

- 字节和文件数只计入源 Namespace。
- 共享不计入接收方配额。
- 转存或采纳会创建目标 Node/Blob 归属，并计入目标配额。
- 上传预占按源 Namespace 的配额账户执行。

### 10.3 采纳

“采纳为租户文件”是独立于共享的显式操作：

1. 验证源文件读取权和租户空间写入权。
2. 在租户 Namespace 创建新 Node 和 Revision。
3. 同 Organization 且策略允许时可复用 Blob；否则创建副本。
4. 写入来源关系和审计事件。
5. 新文件拥有独立权限、生命周期和配额。

### 10.4 删除与离职

- 有共享关系不阻止进入回收站，但共享入口立即不可读取。
- 永久删除必须满足无 Legal Hold、无保留策略阻止。
- 用户离职时 USER Namespace 进入 `FROZEN`。
- 管理员列出该用户全部对外共享，选择移交、采纳或撤销。
- 移交不允许静默扩大原文件可见范围。

## 11. 审计与分类

必须审计：

- Namespace 创建、冻结和移交。
- Grant 创建、更新、撤销和过期。
- 分享申请、批准和拒绝。
- 顶层目录策略变化。
- `CONFIDENTIAL`、`RESTRICTED` 文件读取和下载。
- 管理员受控访问个人空间。
- 文件采纳、Export/Import 和跨空间复制。

审计事件至少包含：

```text
actor_id
organization_id
workspace_id
namespace_id
node_id
action
principal_type
principal_ids
permission_before
permission_after
reason
trace_id
occurred_at
```

数据分类：

```text
PUBLIC
INTERNAL
CONFIDENTIAL
RESTRICTED
```

分类策略限制共享范围，但不替代 Resource Grant。

## 12. 缓存与一致性

- 授权缓存键必须包含主体、Node、动作、Organization、Workspace 和 `policy_version`。
- Grant、成员关系、角色、分类或 Namespace 策略变化后递增版本并主动失效。
- 缓存故障时回源数据库；不得因缓存不可用默认 Allow。
- `shared-with-me` 第一版实时查询 Resource Grant 和成员关系。
- 达到性能阈值后使用异步授权投影，但数据库授权事实仍是权威源。
- 撤销操作提交成功后，新请求不得继续命中旧授权。

## 13. 迁移兼容

1. 为现有 Namespace 回填 `namespace_type = WORKSPACE`。
2. 保留 `workspace_id NOT NULL` 的旧约束前，先完成 nullable 迁移和类型约束。
3. 为现有 Grant 回填 `principal_type = USER`。
4. 现有工作区根目录创建兼容顶层策略，保持当前可见行为，不静默收紧导致生产故障。
5. 新建 Organization 和 USER Namespace 使用幂等命令，不批量创建空个人空间。
6. 旧 API、BFF 和页面在统一 API 上线前继续使用 WORKSPACE Namespace。
7. 迁移期间授权引擎同时支持旧 Grant 和新 Principal Grant，完成回填后移除兼容读取。

## 14. 分阶段交付

### Phase 1：Namespace 基础

- 增加 Namespace Type 和完整性约束。
- 回填现有 WORKSPACE Namespace。
- 增加 Organization、User Namespace 幂等创建。
- 接通三类 Namespace 的配额和 Storage Binding 解析。

验收：三类空间相互隔离，现有工作区文件无回归。

### Phase 2：统一共享

- Grant 增加 Principal Type。
- 支持 USER、WORKSPACE、ORGANIZATION、ROLE。
- 完成继承、Deny、过期和有效权限解释。
- 实现 `shared-with-me`。

验收：共享不复制文件，源更新实时可见，撤销立即失效。

### Phase 3：文件中心

- 增加“我的文件、当前工作区、租户文件、与我共享”入口。
- 增加顶层目录策略和共享弹窗。
- Tenant Owner 获得租户文件治理视图。

验收：不同身份只能看见授权允许的空间和节点。

### Phase 4：审批与企业治理

- 增加分享审批、分类限制和完整审计。
- 增加用户离职移交和“采纳为租户文件”。
- 完成 Placement 接入、缓存失效和性能投影。

验收：高影响共享不可绕过审批，审计可完整解释一次访问。

## 15. 验收标准

1. 租户 Owner 可创建租户文件，并发布给全部或指定工作区。
2. 未被发布覆盖的工作区不能发现、读取或搜索租户文件。
3. 工作区顶层目录策略可控制全体成员、角色、用户和管理员可见范围。
4. 用户个人文件默认只有本人可见。
5. 个人文件可以实时共享给用户、工作区或 Organization。
6. 向整个 Organization 的共享必须经过 Tenant Owner 或租户文件管理员审批。
7. 撤销 Grant 后，新读取请求立即失败。
8. 共享不增加 Node、Blob、Replica 数量，也不增加接收方配额。
9. Tenant Owner 不能仅凭 Owner 身份读取用户个人文件。
10. 跨 Organization 实时共享请求被拒绝。
11. 文件中心能够明确区分四类入口并显示当前 Organization、Workspace 和共享来源。
12. 每次共享、审批、撤销、敏感读取和采纳都可从审计记录追溯。

## 16. 非目标

- 第一阶段不开放匿名公网链接。
- 不提供跨 Organization 实时共享。
- 不把个人空间实现成工作区下的隐藏目录。
- 不因共享自动复制或迁移物理数据。
- 不在第一阶段引入外部 OpenFGA 或 OPA 依赖。
- 不允许业务模块直接读取 Provider 物理路径。
