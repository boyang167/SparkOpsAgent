# SparkOps 第三方插件框架设计

> 从 `A:\21-Agent\gin-vue-admin` 迁移插件**框架**能力到 SparkOpsAgent：第三方可开发插件、上传安装、经审核与 CI 进入主进程。兼容 GVA v2 注册码（`register.go` 空白 import + `plugin.go` 的 `init()` / `Register(*gin.Engine)`）。不迁移 `platformsuite` 业务。

## 目标

- 第三方按 SparkOps SDK 开发插件：后端 Go + 前端 Next.js 页面 + `plugin.yaml`。
- 平台管理员上传 ZIP → 静态检查 → 生成注册代码 → Git PR → 人工审核 → CI 构建部署。
- 平台安装一次；各工作区独立启用并授权。
- 兼容 GVA 插件注册码表面：v2 `Plugin` 接口、进程内 Registry、空白 import 触发 `init()`、启动时调用 `Register(engine)`。
- 提供脚手架：生成含 GVA 风格 `plugin.go` 与 SparkOps Manifest 的包。

## 非目标（本规格不包含）

- GVA `initialize` 五件套（Api / Menu / Dictionary / Gorm / Viper）自动执行。
- 现有 GVA Vue ZIP 无需修改即可安装。
- 运行时热加载（Go `.so`、不重建前端）。
- 插件间依赖解析、semver 自动升级。
- 卸载时 DROP 插件业务表。
- 插件市场（可保留外链，不实现运行时商店）。
- `platformsuite` 或其它 GVA 业务插件的功能迁移。
- 签名发布者体系（可后续叠加；本期为人审 PR）。

## 背景与约束

GVA 插件本质是编译期模块：改源码、重启/重建后生效。SparkOps 已是 Go/Gin 模块化单体 + Next.js App Router + BFF；菜单、API 同步、Casbin、entitlements 已存在。缺口是：插件目录与生命周期、GVA 同形注册表、ZIP→PR 流水线、工作区级启停、可扩展 entitlement、静态前端路由适配。

工作区另有未提交的集群管理改动（governance / platform_admin / entitlements）。实施时应避免无谓改写这些 WIP 文件；插件菜单种子与 entitlement 路径映射用扩展点接入。

## 决策摘要

1. **主路径是 SparkOps Manifest + 编译期模块**，不是照搬 GVA 目录与 Vue 动态 import。
2. **GVA 兼容仅限注册码**：`Plugin` / `Register` / `Registered` / `register.go` blank import / `init()`。`Register(engine)` 只允许挂路由（及插件自管的非 GVA 初始化）；菜单与权限只认 `plugin.yaml`。
3. **安装生效 = Git PR + CI 部署**；部署完成前工作区不可启用。
4. **信任模型**：仅平台管理员上传；强制静态检查；必须人工审核 PR 后合并。
5. **工作区隔离**：全局二进制含插件代码；可见性与 API 授权按工作区 enable + Casbin + entitlement。
6. **插件 `Register` 发生在鉴权中间件之后**，其路由走与核心 API 相同的 JWT / Casbin / entitlements 链。

---

## 架构

### 限界上下文

| 模块 | 职责 |
|------|------|
| `platform/plugin`（横切） | 与 GVA v2 同形的 `Plugin` 接口、`Register`/`Registered`、启动时安全调用 `Register(engine)` |
| `modules/plugin`（新 BC） | 插件目录、安装/卸载任务、PR 状态、工作区绑定、安全扫描结果；HTTP 管理 API |
| `governance` | 工作区启用时按 Manifest 写入菜单、按钮、`platform.apis` 元数据、角色 API 策略；禁用时摘除绑定 |
| `entitlements` | 支持插件模块键 `plugin:{id}`；路径映射由安装生成或运行时从已部署 Manifest 加载 |
| 前端 BFF + 管理 UI | 平台安装/列表/卸载/PR 状态；工作区启用与授权 |
| Git/CI 适配器 | 创建分支、提交生成文件、开 PR；轮询合并与部署状态（实现可先用配置的 GitHub/Git 远程） |

不把插件逻辑塞进 `platform_admin` 的 Tools stub。`platform_admin` 仅保留平台身份校验入口若需要，插件 CRUD 属于 `modules/plugin`。

### 目录约定

后端第三方插件（安装后）：

```text
backend/internal/plugins/{id}/
  plugin.go          # init() + Register(*gin.Engine)，import SparkOps v2 包
  plugin.yaml        # 亦可放包根；安装器校验
  ...                # 插件自有 api/service/model，禁止 import 其它 BC 的 domain/infrastructure
backend/internal/plugins/register.go   # 生成：blank import 各已安装插件
```

前端（安装生成或包内提供）：

```text
fronted/apps/web/app/(platform)/plugins/{id}/**/page.tsx
fronted/apps/web/src/modules/plugins/{id}/
fronted/packages/bff-modules/src/plugins/{id}/
fronted/apps/web/app/api/bff/v1/plugins/{id}/**/route.ts
```

ZIP 规范（SparkOps，非 GVA 原样）：

```text
{id}/
  plugin.yaml
  backend/           # 复制到 backend/internal/plugins/{id}/
  frontend/          # 复制到约定 Next 路径
```

包内 `plugin.go` 必须：

```go
func init() {
    sparkplugin.Register(Plugin) // 包路径为 SparkOps 的 platform/plugin，函数签名与 GVA v2 相同
}
func (p *plugin) Register(engine *gin.Engine) { /* 挂路由 */ }
```

为降低第三方改 import 成本：SparkOps 的接口与函数名与 GVA `server/utils/plugin/v2` 保持一致（`Plugin`、`Register`、`Registered`）。文档与脚手架写明新 import 路径。不提供 `github.com/flipped-aurora/...` 的 go.mod replace 兼容（避免把 GVA 模块拉进主模块）。

### 启动顺序

在 `bootstrap.Run` 中，核心 `module.Register` 完成、JWT/Casbin/entitlements 中间件挂上之后、`routes.Collect` 之前：

1. 空白 import `internal/plugins`（`register.go`）已在进程 init 阶段填充 Registry。
2. 对 `Registered()` 快照逐个 `safeRegister(engine)`：recover panic，打日志，继续下一个。
3. `routes.Collect` 包含插件路由，供 API 同步。

未部署（register.go 无 import）的插件不进入进程。

### Manifest（`plugin.yaml`）

必填：

| 字段 | 说明 |
|------|------|
| `id` | 稳定标识，kebab-case，与目录名一致 |
| `version` | 展示用 semver 字符串；本期不做依赖解析 |
| `display_name` | 管理 UI 名称 |
| `route_prefix` | 后端路由前缀，建议 `/api/v1/plugins/{id}` |
| `entitlement` | `plugin:{id}` |
| `menus` | 工作区菜单树（title、path、对应前端 route） |
| `apis` | 声明的 method+path，供 Casbin / API registry 元数据 |
| `frontend.pages` | Next 页面相对路径列表，供安装器校验存在 |

可选：`buttons`、`description`、`author`。

缺少 Manifest 或 `id` 与目录不一致 → 拒绝安装。纯 GVA 包（仅 `initialize/api.go` 无 yaml）拒绝，提示使用脚手架补 Manifest。

---

## 组件与接口

### GVA 兼容层（`platform/plugin`）

与 GVA 行为对齐：

- `type Plugin interface { Register(group *gin.Engine) }`
- `Register(p Plugin)` / `Registered() []Plugin`（互斥锁 + 快照拷贝）
- 忽略 `nil`

差异（必须写进第三方文档）：

- 不调用插件的 `initialize.Api/Menu/...`。
- 不在 `Register` 内写系统菜单表；若插件代码仍调用 GVA 风格初始化，将因缺少 GVA global/DB API 而无法编译——这是预期，须改为只挂路由。

### 插件 BC

状态机（平台级安装记录）：

`uploaded` → `pr_open` → `merged` → `deployed`  
失败：`pr_failed` / `ci_failed`（可重试开 PR，不改主分支）

工作区绑定：`(workspace_id, plugin_id, enabled, enabled_by, enabled_at)`。仅当平台记录为 `deployed` 时可 enable。

卸载：若存在 enabled 工作区则拒绝（或管理 API 提供 `force` 仅平台超管，仍先 disable 全部再开删除 PR）。删除 PR 去掉目录与 `register.go` import；合并部署后删目录元数据。不 DROP 表。

### 权限

- 安装/卸载/看 PR：平台管理员。
- 工作区启用/授权：工作区管理员。
- 插件业务 API：该工作区已 enable + 角色 Casbin + `plugin:{id}` entitlement。
- 未 enable 的工作区访问插件 API：403（entitlement 或 API policy）。

### 前端

- Next.js 无运行时 `import(component)`。菜单 `component` 可存路径字符串，实际导航到预生成 `page.tsx`。
- `nav-tree.ts` 静态树不手工维护每个插件；命令搜索与 entitlement 过滤以 DB 菜单 + Manifest entitlement 为准。若必须改 nav-tree，由安装器生成片段并纳入同一 PR。
- BFF：平台插件管理走 `/api/bff/v1/admin/plugins*`；工作区启用走 `/api/bff/v1/workspaces/:wsId/plugins*`。

---

## 数据流

### 安装

1. 平台管理员上传 ZIP。
2. 静态检查：ZIP slip、禁止 `.sh/.exe/.bat` 等、必须有 `plugin.yaml` 与 `Register(*gin.Engine)` 符号（AST）、`id` 未占用。
3. 解包到工作树副本（安装器使用临时 clone/worktree，不直接污染开发者脏工作区）。
4. 更新 `register.go`；必要时生成 page/BFF 骨架与 entitlement 路径条目。
5. `git checkout -b plugin/install-{id}`、commit、push、`gh pr create`（或配置的 Git 提供方）。
6. 记录 `pr_open`。合并后 CI 构建前后端镜像并部署；健康检查或发布成功回调将状态置 `deployed`。

### 工作区启用 / 禁用

启用：事务内写入菜单、按钮、API 策略、entitlement 覆盖（sysconfig 或专用表）。失败整笔回滚。  
禁用：删除该工作区绑定与菜单可见性；Casbin 策略按工作区移除；代码仍在二进制中。

### 请求路径

```text
浏览器 → BFF /api/bff/v1/plugins/{id}/... → Go /api/v1/plugins/{id}/...
  → JWT → Casbin → entitlements(plugin:{id}) → 插件 Register 挂载的 handler
```

---

## 错误处理

| 情况 | 行为 |
|------|------|
| ZIP 非法、路径穿越、无 Manifest、无 v2 Register | 400，不写 Git |
| id 冲突 | 409；升级不在本期 |
| Git/PR 失败 | `pr_failed`，可重试 |
| CI 失败 | `ci_failed`，插件不可 enable |
| `Register` panic | recover，记日志，其它插件与核心服务继续 |
| 工作区启用部分失败 | 事务回滚 |
| 卸载时仍有启用 | 409 |

---

## 测试

- 单元：ZIP 校验、`register.go` 生成（含增删 import）、Manifest 解析、enable/disable 事务。
- 集成：示例插件 blank import 后启动调用 `Register`；路由出现在 `routes.Collect`；未 enable 工作区 403。
- 前端：管理列表与 PR 状态；启用后侧栏出现 Manifest 菜单，禁用后消失。
- 安全：恶意路径、脚本文件、无 yaml 的 GVA 包拒绝。
- 启动：故意 panic 的插件不导致 `bootstrap.Run` 退出。

示例插件随仓库提供 `backend/internal/plugins/example`（可默认不 import，测试用 build tag 或测试专用 register），用于证明 GVA 风格 `init`+`Register` 可用。

---

## 安全

- 插件与主进程同权限；人审 PR 是主控制。
- 静态检查不能替代审核；文档明确第三方代码可信度。
- 插件禁止 import 其它 BC 的 `domain`/`infrastructure`；允许 `platform/appctx`、`platform/plugin`、公开 HTTP 中间件。评审清单写入 PR 模板。
- 路由建议限制在 `route_prefix` 下；安装器 AST 检查 `Group` 前缀（尽力，非形式化证明）。

---

## 实施顺序（规格级，非任务拆解）

1. `platform/plugin` 契约 + bootstrap 挂载 + 空 `register.go`。
2. `plugin.yaml` 与 DB 模型、管理 API。
3. 安装器：校验、解包、生成、PR。
4. 工作区 enable + governance/entitlements 扩展。
5. 前端管理与工作区开关。
6. 脚手架与 example 插件、测试与文档。

---

## 验收标准

- 第三方可用脚手架生成插件；`plugin.go` 使用与 GVA 相同的 `init` + `Register(engine)` 写法（import 改为 SparkOps 包）。
- 上传合法 ZIP 后产生 PR；合并部署后工作区可启用；启用后菜单与 API 按 Manifest 生效。
- 禁用后菜单消失且 API 403；卸载走反向 PR。
- 无 Manifest 的 GVA ZIP 被拒绝。
- 单插件 panic 不影响主服务启动。
