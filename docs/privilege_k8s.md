# Kubernetes 与多层级授权方案

本文档说明 **组织 → 工作区 → 项目 → 集群** 的授权关系，并给出 **角色-菜单-API 对照表** 与 **开发 / 测试 / 运维 / 审计** 四套默认 RBAC 模板。

相关设计见 [`privilege_design.md`](./privilege_design.md)。

---

## 1. 总体模型

```text
Platform（平台超管）
  └── Organization（组织）
        └── Workspace（工作区）
              ├── Cluster 授权（workspace_clusters）
              ├── 成员 + RBAC 角色（governance）
              └── Project（项目）
                    └── default_cluster_id + 密钥 → 部署
```

| 层级 | 管什么 | 不管什么 |
|------|--------|----------|
| **组织** | 成员、创建工作区、配额 | 不直接管集群 |
| **工作区** | 集群登记/撤销、RBAC、密钥、模板 | 不能跨工作区用未授权集群 |
| **项目** | 选默认集群、绑 Git/Jenkins/K8s 密钥、部署 | 不能注册集群 |
| **集群** | 全局目录 + 对工作区的授权关系 | 不属于某个项目 |

---

## 2. 两类「集群授权」（勿混淆）

| 类型 | 存储 | 入口 | 含义 |
|------|------|------|------|
| **工作区集群授权** | `platform_admin.workspace_clusters` | 持续交付 → 集群管理；K8s 插件 → 集群管理 | 工作区**能否使用**该集群（硬门禁） |
| **成员 K8s 授权** | `plugin_kubernetes.cluster_user_grants` | K8s 插件 → 集群管理 → **授权** | 给成员签发**个人 kubeconfig**（本地 kubectl） |

规则：

- 未写入 `workspace_clusters` 的集群：列表不可见、proxy 403、项目不可选。
- **读** K8s（proxy GET）：任意工作区成员 + 集群已授权。
- **写** K8s（YAML/删除/变更集群/settings）：需 `governance:admin`（前端 `canManage`）。
- `cluster_user_grants` **不**限制控制台访问，只影响「授权 → 用户」签发的凭证。

---

## 3. 权限判定链

任意操作按顺序检查（AND）：

1. **认证**：JWT 有效
2. **作用域成员**：组织 / 工作区 / 项目成员关系
3. **功能权限码**：`project:write`、`secret:write`、`governance:admin` 等
4. **Entitlement**：`k8s`、`delivery`、`plugin:kubernetes` 模块开关
5. **资源级**：集群是否在 `workspace_clusters`；项目 ACL

---

## 4. 系统内置工作区角色

定义见 `backend/internal/platform/authz/permissions.go`。

| 内置角色 | 权限码 |
|----------|--------|
| **admin** | 全部（含 `governance:admin`、`secret:write`、`project:operate` 等） |
| **developer** | `project:read/write/submit`、`secret:read`、`agent:read`、`chat:use` |
| **reviewer** | `project:read/review`、`secret:read` |
| **operator** | `project:read/operate`、`secret:read` |
| **viewer** | `project:read` |

注意：**developer 默认不能部署**（无 `project:operate`），也**不能写密钥/注册集群**（无 `secret:write`）。

---

## 5. 角色-菜单-API 对照表

图例：**✓** 可用 · **只读** 可看不可改 · **—** 不可见或 403 · **成员** 仅需工作区成员资格

### 5.1 业务控制台侧栏（Entitlement + 菜单策略）

侧栏可见性 = **Entitlement 模块开关** + **治理菜单策略**（`governance.role_menus`）。下表为 Entitlement 满足时的默认预期。

| 菜单 | Entitlement | admin | developer | operator | reviewer | viewer |
|------|-------------|-------|-----------|----------|----------|--------|
| 工作台 | — | ✓ | ✓ | ✓ | ✓ | ✓ |
| 项目管理 / 模板 / 知识库 | agent / knowledge | ✓ | ✓ | 只读 | 只读 | 只读 |
| 构建 / 发布 / Pipeline 等 | delivery | ✓ | ✓ | ✓ | 只读 | 只读 |
| **持续交付 → 集群管理** | 无（始终可见） | ✓ | ✓ | ✓ | ✓ | ✓ |
| **持续交付 → 密钥配置** | delivery | ✓ | 只读 | 只读 | 只读 | — |
| 运行实例 / 定时任务 | k8s | ✓ | ✓ | ✓ | — | — |
| 监控 / 日志 / 统计 | observability | ✓ | ✓ | ✓ | ✓ | 只读 |
| Webhook / MCP | mcp | ✓ | — | — | — | — |

K8s 插件整组菜单需 **`plugin:kubernetes` 已启用** 且工作区已授权至少一个集群（会自动打开 `k8s` entitlement）。

### 5.2 Kubernetes 插件 → 集群管理（截图中的操作列）

前端写操作统一受 `canManage = governance:admin || super_admin` 控制。

| 操作 | admin | developer | operator | reviewer | viewer |
|------|-------|-----------|----------|----------|--------|
| 查看集群列表 / 状态探测 | ✓ | ✓ | ✓ | ✓ | ✓ |
| **新增 / 变更 / 删除** | ✓ | — | — | — | — |
| **授权**（打开弹窗） | ✓ | 只读* | 只读* | 只读* | 只读* |
| 授权 → 连接信息（导出 kubeconfig） | ✓ | ✓ | ✓ | ✓ | ✓ |
| 授权 → 用户（签发 SA Token） | ✓ | — | — | — | — |
| 授权 → 角色（ClusterRole 绑定） | ✓ | — | — | — | — |
| **关联**（Velaux 多集群） | ✓ | — | — | — | — |

\* 无 `governance:admin` 时可打开「授权」弹窗查看连接信息，但用户/角色 Tab 内按钮禁用。

### 5.3 Kubernetes 插件 → 资源页（工作负载 / 配置 / 网络等）

| 能力 | admin | developer | operator | reviewer | viewer |
|------|-------|-----------|----------|----------|--------|
| 列表 / 详情 / 日志（GET proxy） | ✓ | ✓ | ✓ | ✓ | ✓ |
| 创建 / 编辑 / 删除 YAML | ✓ | — | — | — | — |
| Pod 终端 / 文件上传 / 执行命令 | ✓ | — | — | — | — |
| 配置管理 → **Secret** Tab | ✓ | — | — | — | — |
| 应用诊断 / Arthas 安装 | ✓ | — | — | — | — |

### 5.4 关键后端 API

| API | 权限要求 |
|-----|----------|
| `GET /api/v1/workspaces/:wsId/clusters` | 工作区成员 + `project:read` |
| `POST /api/v1/workspaces/:wsId/clusters`（注册并授权） | 工作区成员 + **`secret:write`** |
| `DELETE .../clusters/:clusterId`（撤销工作区授权） | 工作区成员 + **`secret:write`** |
| `POST /api/v1/admin/workspaces/:wsId/clusters`（跨工作区 grant） | **Super Admin** |
| `GET/POST .../plugins/kubernetes/clusters/:id/proxy/*`（读） | 工作区成员 + 集群已授权 |
| `POST/PUT/PATCH/DELETE .../proxy/*`（写） | 工作区成员 + **`governance:admin`** + 集群已授权 |
| `PUT .../clusters/:id/settings` | **`governance:admin`** |
| `POST .../cluster-users` / 发 kubeconfig | **`governance:admin`** |
| `PATCH /api/v1/projects/:id`（绑集群/密钥） | 项目写权限 + 集群在 `workspace_clusters` |
| `POST .../projects/:id/deployments` | **`project:operate`** + 项目 approved |

---

## 6. 四套业务 RBAC 模板（推荐）

在「管理控制台 → 角色管理」中创建自定义角色，或使用内置角色 + 微调绑定。

### 6.1 开发（Developer）

**目标**：写代码、改项目配置、日常看 K8s，**不**改集群、**不**删生产资源。

| 项 | 建议 |
|----|------|
| 工作区角色 | 内置 `developer`，或复制后增加 `project:operate`（若需自行触发部署到 dev） |
| 组织角色 | `member` |
| 项目角色 | `lead` 或 `member` |
| 集群 | 使用 dev；`default_cluster_id` 指向 dev |
| K8s 控制台 | 只读（无 `governance:admin`） |
| 密钥 | 只读 `secret:read`；密钥由运维统一维护 |

### 6.2 测试（QA）

**目标**：验收 builds、只读 K8s/uat、提交审核意见。

| 项 | 建议 |
|----|------|
| 工作区角色 | 内置 `reviewer` + 自定义 `qa`：`project:read`、`project:review`、`secret:read` |
| 项目角色 | `member` 或 `viewer` |
| 集群 | 只看 uat；项目 `default_cluster_id=uat` |
| K8s 控制台 | 只读 |
| 部署 | 一般不授予 `project:operate`；由运维/发布专员执行 |

### 6.3 运维（Ops）

**目标**：集群登记、密钥、部署、K8s 资源变更。

| 项 | 建议 |
|----|------|
| 工作区角色 | 内置 `admin`，或 `operator` + **`governance:admin`** + **`secret:write`** |
| 项目角色 | 按需 `lead` |
| 集群 | 维护 dev/uat；负责「变更 / 授权 / 关联」 |
| K8s 控制台 | 读写（`governance:admin`） |
| 持续交付 | 密钥配置读写、触发部署 |

### 6.4 审计（Audit）

**目标**：只读浏览，不能改任何资源。

| 项 | 建议 |
|----|------|
| 工作区角色 | 内置 `viewer` |
| 组织角色 | `member`（仅需 `org:read` 进组织） |
| 项目角色 | `viewer` |
| K8s 控制台 | 只读 |
| 管理控制台 | 可只开「操作日志」「组织审计」菜单，不绑 `governance:admin` |

---

## 7. 角色 × 权限码 速查矩阵

| 权限码 | admin | developer | operator | reviewer | viewer | 运维模板 | 开发+部署 |
|--------|:-----:|:---------:|:--------:|:--------:|:------:|:--------:|:---------:|
| `governance:admin` | ✓ | — | — | — | — | ✓ | — |
| `secret:write` | ✓ | — | — | — | — | ✓ | — |
| `secret:read` | ✓ | ✓ | ✓ | ✓ | — | ✓ | ✓ |
| `project:write` | ✓ | ✓ | — | — | — | ✓ | ✓ |
| `project:operate` | ✓ | — | ✓ | — | — | ✓ | ✓ |
| `project:review` | ✓ | — | — | ✓ | — | — | — |
| `project:read` | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

---

## 8. 推荐授权流程（Checklist）

1. [ ] 组织成员加入，创建/加入工作区  
2. [ ] 按上表绑定工作区角色（运维 = admin，开发 = developer，审计 = viewer）  
3. [ ] 运维登记 **dev / uat** 集群（`POST .../workspaces/:wsId/clusters`）  
4. [ ] 运维创建 Git / Jenkins / K8s / Harbor 密钥  
5. [ ] 开发配置项目：`default_cluster_id`、模板、密钥引用  
6. [ ] Reviewer 审核项目 → `approved`  
7. [ ] 运维或 operator 执行部署到 uat  
8. [ ] （可选）运维在「授权 → 用户」为需要 kubectl 的开发签发凭证  

---

## 9. 环境分工示例（test-workspace）

| 集群 | 用途 | 主要角色 |
|------|------|----------|
| **dev** | 开发联调、K8s 插件日常只读/运维写 | 开发只读，运维 admin |
| **uat** | 验收与发布 | 项目 `default_cluster_id`，运维/operator 部署 |

---

## 10. 已知限制（规划参考）

1. 组织层无集群 API，集群只在 Platform + Workspace 管理。  
2. K8s 权限较粗：读 = 全体工作区成员，写 = `governance:admin`。  
3. 项目密钥为项目级，尚未做到每集群独立 secret。  
4. `cluster_user_grants` 不充当控制台门禁。  

---

## 11. 关键代码索引

|  topic | 路径 |
|--------|------|
| 权限码定义 | `backend/internal/platform/authz/permissions.go` |
| 工作区集群 grant | `backend/internal/modules/platform_admin/application/command/admin_commands.go` |
| K8s 写门禁 | `backend/internal/plugins/kubernetes/http/write_guard.go` |
| 前端 canManage | `fronted/apps/web/src/server/loaders/governance-context.ts` |
| 集群管理 UI | `fronted/apps/web/src/modules/plugins/kubernetes/specialized/ClustersPanel.tsx` |
| 部署门禁 | `backend/internal/modules/delivery/deployrun/infrastructure/acl/workspace_app_acl.go` |

---

## 12. 菜单策略（`governance.role_menus`）机制

### 12.1 数据表

| 表 | 作用 |
|----|------|
| `governance.menus` | 工作区菜单树（path 如 `/projects`；分组父节点 path 如 `ops-delivery`） |
| `governance.role_menus` | 角色 ↔ 菜单多对多 |
| `governance.menu_buttons` | 页面内按钮定义 |
| `governance.role_menu_buttons` | 角色 ↔ 按钮 |

菜单种子来源：

- 业务菜单：`menu_commands.go` → `defaultMenuGroups`
- K8s 插件菜单：启用 `plugin:kubernetes` 后从 `plugin.yaml` 同步
- 管理控制台（`/admin/*`）：前端静态配置，不走 `role_menus`

### 12.2 可见性规则（重要）

对每个菜单节点独立判断（`FilterMenusForNav`）：

| `role_menus` 绑定情况 | 谁能看到 |
|----------------------|----------|
| **无任何角色绑定** | 所有工作区成员（仍受 Entitlement 过滤） |
| **已绑定若干角色** | 仅绑定角色之一的成员 |

因此：**一旦给某菜单绑了角色，它就变成白名单模式**。未绑定的菜单仍全员可见。

侧栏最终可见性 = **Entitlement** ∩ **role_menus** ∩ **Super Admin 路径规则**。

### 12.3 配置入口

| 方式 | 说明 |
|------|------|
| **UI** | 管理控制台 → 角色管理 → 选中角色 → 权限 / 菜单树勾选 |
| **API** | `PUT /api/v1/workspaces/:wsId/roles/:roleId/menus`，body: `{ "menu_ids": ["uuid", ...] }` |
| **查询** | `GET /api/v1/workspaces/:wsId/roles/:roleId/menus` → `{ "menu_ids": [...] }` |

> API 使用 **menu UUID**（每个工作区 seed 时随机生成）。跨环境复制策略请用 **path → id** 映射（见 §12.6）。

---

## 13. 菜单 Path 目录（策略配置用）

### 13.1 业务控制台分组

| 分组 path | 分组标题 | 叶子 path | 标题 |
|-----------|----------|-----------|------|
| `application` | 应用中心 | `/projects` | 项目管理 |
| | | `/templates` | 模板管理 |
| | | `/knowledge` | 知识库 |
| `ops-delivery` | 持续交付 | `/delivery/builds` | 构建管理 |
| | | `/delivery/pipelines` | Pipeline Studio |
| | | `/delivery/fleets` | Fleet 编辑器 |
| | | `/delivery/profiles` | Profile 配置 |
| | | `/publish/products` | 产品管理 |
| | | `/publish/plans` | 版本计划 |
| | | `/publish/releases` | 发布管理 |
| | | `/config/secrets` | 密钥配置 |
| | | `/cluster/list` | 集群管理 |
| `ops-runtime` | 运行管理 | `/runtime/pods` | 运行实例 |
| | | `/cronjobs` | 定时任务 |
| `ops-observability` | 可观测性 | `/monitor/overview` | 监控概览 |
| | | `/monitor/log-viewer` | 日志查询 |
| | | `/monitor/statistics` | 部署统计 |
| `ops-infra` | 基础设施 | `/config/webhooks` | 外部集成 |

### 13.2 Kubernetes 插件（启用后）

| path | 标题 |
|------|------|
| `/plugins/kubernetes` | Kubernetes管理（分组） |
| `/plugins/kubernetes/clusters` | 集群管理 |
| `/plugins/kubernetes/nodes` | 节点管理 |
| `/plugins/kubernetes/workloads` | 工作负载 |
| `/plugins/kubernetes/namespaces` | 命名空间 |
| `/plugins/kubernetes/networks` | 网络管理 |
| `/plugins/kubernetes/configs` | 配置管理 |
| `/plugins/kubernetes/storages` | 存储管理 |
| `/plugins/kubernetes/access-controls` | 访问控制 |
| `/plugins/kubernetes/records` | 终端审计 |
| `/plugins/kubernetes/diagnosis` | 应用诊断 |
| … | 见 `backend/internal/plugins/kubernetes/plugin.yaml` |

配置 `role_menus` 时，**父分组 path 也要勾选**，否则子菜单无法出现在侧栏树中。

---

## 14. 四套角色菜单策略 Seed 示例

策略文件用 **path** 描述（稳定、可版本管理）；落库时解析为 `menu_ids`。

**符号**：✓ 勾选 · — 不勾选 · （空）= 不绑定（全员可见，若需Restrict则见备注）

### 14.1 运维（ops）— 内置 `admin` 角色

| path | admin |
|------|:-----:|
| `application` + 全部子项 | ✓ |
| `ops-delivery` + 全部子项 | ✓ |
| `ops-runtime` + 全部子项 | ✓ |
| `ops-observability` + 全部子项 | ✓ |
| `ops-infra` + `/config/webhooks` | ✓ |
| `/plugins/kubernetes` + **全部**子项 | ✓ |

> 实践建议：**admin 可不绑 menu**（默认全员可见 + `governance:admin` 控 API）。若需「最小可见面」，按上表显式绑定。

### 14.2 开发（developer）— 内置 `developer` 角色

| path | developer |
|------|:---------:|
| `application` | ✓ |
| `/projects` | ✓ |
| `/templates` | ✓ |
| `/knowledge` | ✓ |
| `ops-delivery` | ✓ |
| `/delivery/builds` | ✓ |
| `/cluster/list` | ✓ |
| `/config/secrets` | — |
| `/publish/*`、`/delivery/pipelines` 等 | — |
| `ops-runtime` | ✓ |
| `/runtime/pods` | ✓ |
| `/cronjobs` | — |
| `ops-observability` | ✓ |
| `/monitor/log-viewer` | ✓ |
| `/plugins/kubernetes` | ✓ |
| `/plugins/kubernetes/clusters` | ✓ |
| `/plugins/kubernetes/workloads` | ✓ |
| `/plugins/kubernetes/configs` | ✓ |
| `/plugins/kubernetes/namespaces` | ✓ |
| `/plugins/kubernetes/access-controls` | — |
| `/plugins/kubernetes/records` | — |

### 14.3 测试（qa）— 建议自定义角色 `qa`（基于 reviewer 权限码）

| path | qa |
|------|:--:|
| `application` → `/projects` | ✓ |
| `ops-delivery` → `/delivery/builds` | ✓ |
| `ops-delivery` → `/publish/releases` | ✓ |
| `ops-observability` → 全部 | ✓ |
| `/plugins/kubernetes` → `/clusters`、`/workloads` | ✓ |
| 其余 | — |

### 14.4 审计（audit）— 内置 `viewer` 角色

| path | viewer |
|------|:------:|
| `application` → `/projects` | ✓ |
| `ops-observability` → 全部 | ✓ |
| `/plugins/kubernetes` → `/clusters`、`/records` | ✓ |
| 写操作类（secrets、pipeline、变更集群） | — |

---

## 15. JSON Seed 模板（path 版）

以下 JSON 可直接放仓库（如 `docs/seeds/menu-policy.test-workspace.json`），由脚本或手工映射为 `menu_ids` 后调用 API。

```json
{
  "workspace_slug": "test-workspace",
  "roles": {
    "developer": {
      "permissions_extra": ["project:operate"],
      "menu_paths": [
        "application",
        "/projects",
        "/templates",
        "/knowledge",
        "ops-delivery",
        "/delivery/builds",
        "/cluster/list",
        "ops-runtime",
        "/runtime/pods",
        "ops-observability",
        "/monitor/log-viewer",
        "/plugins/kubernetes",
        "/plugins/kubernetes/clusters",
        "/plugins/kubernetes/workloads",
        "/plugins/kubernetes/configs",
        "/plugins/kubernetes/namespaces"
      ]
    },
    "operator": {
      "menu_paths": [
        "application",
        "/projects",
        "ops-delivery",
        "/delivery/builds",
        "/publish/releases",
        "/config/secrets",
        "/cluster/list",
        "ops-runtime",
        "/runtime/pods",
        "/cronjobs",
        "ops-observability",
        "/plugins/kubernetes",
        "/plugins/kubernetes/clusters",
        "/plugins/kubernetes/workloads"
      ]
    },
    "viewer": {
      "menu_paths": [
        "application",
        "/projects",
        "ops-observability",
        "/monitor/overview",
        "/monitor/log-viewer",
        "/monitor/statistics",
        "/plugins/kubernetes",
        "/plugins/kubernetes/clusters",
        "/plugins/kubernetes/records"
      ]
    },
    "admin": {
      "menu_paths": ["*"]
    }
  },
  "restricted_paths": {
    "comment": "建议仅这些 path 做 role 白名单绑定；其余保持未绑定=全员可见",
    "paths": [
      "/config/secrets",
      "/delivery/pipelines",
      "/delivery/fleets",
      "/delivery/profiles",
      "/config/webhooks",
      "/plugins/kubernetes/access-controls",
      "/plugins/kubernetes/diagnosis"
    ],
    "allowed_roles": ["admin", "operator"]
  }
}
```

`admin.menu_paths: ["*"]` 表示不限制（不对 admin 写 `role_menus`，或绑定全部节点）。

---

## 16. 落库步骤

### 16.1 SQL：path → menu_id

```sql
-- 替换 :workspace_id
SELECT id, path, title
FROM governance.menus
WHERE workspace_id = :workspace_id
  AND deleted_at IS NULL
  AND path IN (
    '/projects',
    '/config/secrets',
    '/plugins/kubernetes/clusters'
  );
```

### 16.2 API 示例

```http
PUT /api/v1/workspaces/{wsId}/roles/{roleId}/menus
Content-Type: application/json

{
  "menu_ids": [
    "8f2c1a00-....",
    "b91d4e22-...."
  ]
}
```

BFF 等价路径：`PUT /api/bff/v1/workspaces/{wsId}/roles/{roleId}/menus`

### 16.3 Restrict 型菜单（推荐做法）

对敏感菜单（如 `/config/secrets`）：

1. 在「菜单策略」UI 中打开该菜单 → 仅勾选 **admin**、**operator**
2. 其他角色不勾选 → developer/viewer 侧栏不再显示密钥配置
3. **API 仍受 `secret:write` / `secret:read` 保护**（菜单隐藏 ≠ 安全边界）

### 16.4 按钮策略（可选）

页面内按钮（如「删除集群」「批量撤销」）可走 `role_menu_buttons`：

- 未配置按钮绑定 → 当前实现为**按钮全员可见**（仍受 `canManage` / API 权限后端拦截）
- 配置后 → 按角色过滤

K8s 集群写操作以后端 `governance:admin` 为准；按钮策略只做 UI 降噪。

---

## 17. 与管理控制台菜单的关系

| 控制台 | 菜单来源 | 访问控制 |
|--------|----------|----------|
| 业务控制台 | `governance.menus` + Entitlement | `role_menus` + 模块开关 |
| 管理控制台 `/admin/*` | 前端 `ADMIN_NAV_SECTIONS` | 通常需工作区 **`governance:admin`** |
| 平台运营 `/platform/*` | 前端静态 | **Super Admin** |

组织层无独立菜单树；组织权限仅 `org:read` / `org:member:manage` / `org:workspace:create`。

---

## 18. 实施 Checklist（菜单 + 权限码）

1. [ ] 确认工作区已 seed 菜单（首次访问侧栏或调 `GET .../menus/tree` 触发）
2. [ ] 启用 Kubernetes 插件 → 同步 `/plugins/kubernetes/*` 菜单
3. [ ] 创建/确认四套角色（admin / developer / operator / viewer 或自定义 qa）
4. [ ] 按 §14 为各角色 **SetRoleMenus**（含父分组 path）
5. [ ] 对 §15 `restricted_paths` 做菜单级白名单
6. [ ] 校验 Entitlement：无 `k8s` 时运行管理/K8s 分组自动隐藏
7. [ ] 用测试账号登录，确认侧栏与 §5 对照表一致
8. [ ] 验证 API：无权限码时菜单可见但操作 403（双重保障）

