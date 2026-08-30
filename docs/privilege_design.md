# 权限设计

**Platform / Organization / Workspace / Project** 是作用域（权限在哪一层生效）。

**User / Role / API / Menu / Button / Data** 是权限面（控制什么）。

工作区里 **Role 是枢纽**：人只绑角色，角色再挂菜单、按钮、API、数据。

---

## 1. 纵轴：四层作用域（包含关系）

```text
Platform
  └── Organization
        └── Workspace
              └── Project
```


| 层            | 人怎么挂上去                                | 角色长什么样                              |
| ------------ | ------------------------------------- | ----------------------------------- |
| Platform     | `system_role_bindings`                | 只有 `super_admin`，不是工作区 Role         |
| Organization | `organization_members`                | 固定 `owner` / `admin` / `member`     |
| Workspace    | `workspace_members` + `role_bindings` | 可配置 Role（`admin` / `developer` / …） |
| Project      | `project_members`                     | 固定 `lead` / `member` / `viewer`     |


User（Member）是全局账号，在每一层可以有不同绑定。

- 不能用工作区 `admin` 代替 Super Admin。
- 不能用项目 `lead` 代替 `governance:admin`。

---

## 2. 横轴：六类权限面（工作区 Role 打包）

```text
User ──绑定──► Role ──┬── permissions[]     功能权限码（后端主闸）
                      ├── menus             导航可见
                      ├── buttons           按钮可见
                      ├── API path+method   HTTP（Casbin，可选）
                      └── data scope        行/列（已建模，查询未强制）
```


| 面      | 回答的问题        | 和 Role 的关系                     | 能否当安全边界                      |
| ------ | ------------ | ------------------------------ | ---------------------------- |
| User   | 你是谁          | 被绑到各层角色                        | 认证必须                         |
| Role   | 你在这个工作区是什么包  | 权限包本身                          | 是                            |
| Menu   | 侧栏有没有这个页     | Role ↔ 菜单                      | 否，只控 UI                      |
| Button | 页上有没有这个按钮    | Role ↔ 按钮 code                 | 否，前端 `MenuButtonGuard`       |
| API    | 这个 HTTP 能不能打 | Role ↔ path+method → Casbin    | 有策略才强制；`governance:admin` 旁路 |
| Data   | 能看到哪些行/列     | Role ↔ resource + `data_scope` | 设计上是；当前主要靠项目 ACL             |


组织层没有这套菜单 / 按钮 / API 树，只有三个固定权限码。平台层用 Super Admin 中间件，不走工作区 Casbin。

---

## 3. 一次操作应怎么判定（AND，不是 OR）

建议固定成：

1. **认证**：JWT / 工作区 Token 对应 User。
2. **作用域**：在对应层有成员关系（平台超管 / 组织成员 / 工作区成员 / 项目成员）。
3. **功能权限码**：工作区 Role 并集里有 `project:write` 这类码（路由上的 `RequirePermission`）。
4. **API 策略**（若该工作区配了）：Casbin 允许 `(user, workspace, path, method)`。
5. **数据范围**（若启用）：行在 `all` / `self` / `custom` 内；项目资源再叠 `project_members`。
6. **Entitlement**：模块开关，与角色正交。

菜单、按钮只做第 0 步体验：藏入口。真正写数据必须过第 3 步（和第 4 步）。

按钮没配角色时，当前实现是「未绑定则全员可见该按钮」，配置后才按角色过滤；Super Admin 与 `governance:admin` 看全部按钮。

---

## 4. 各面应该落在哪一层（设计原则）

### Menu / Button

只挂 Workspace 菜单树（按工作区 seed）。平台页用 Super Admin 过滤路径，不要塞进普通工作区角色。组织没有独立菜单体系。

### API


| 接口范围  | 判定              |
| ----- | --------------- |
| 平台接口  | Super Admin     |
| 组织接口  | `org:`*         |
| 工作区接口 | 权限码 + 可选 Casbin |
| 项目接口  | 权限码 + 项目 ACL    |


### Data


| 级别   | 机制                                                    |
| ---- | ----------------------------------------------------- |
| 工作区级 | `workspace_id` 隔离（所有业务表都应有）                           |
| 角色级  | `role_data_policies`（seed 了 project / agent / member） |
| 项目级  | `project_members`，不要和工作区 Data 策略混成一张表                 |


权限码（`project:read`）表示**能力**；Data 表示**范围**；项目 ACL 表示**哪几个项目**。三者叠加：有 `project:read` 才能列表，Data=`self` 只看到自己建的，再被项目成员表裁一层。

---

## 5. 和现状的对齐

**已经按上面枢纽模型做了的：** 工作区 Role 绑定、菜单/按钮、API 策略同步 Casbin、项目 ACL、组织固定角色、平台 Super Admin。

**还没闭环的：** Data 策略只存 `governance.role_data_policies`，列表/查询没有按 `data_scope` 过滤；行级目前主要靠项目成员。若要把「data 权限」做成真安全边界，需要在仓储查询里统一注入 scope（以及字段脱敏），而不是只做管理页配置。

**设计口诀：** 作用域分层、角色打包、UI 与 API 分离、数据范围另算；User 不直接绑 API/菜单，只绑 Role（平台/组织用固定角色例外）。