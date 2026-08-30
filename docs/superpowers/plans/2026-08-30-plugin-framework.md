# SparkOps Plugin Framework Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地第三方插件框架：GVA v2 同形 `init`/`Register(*gin.Engine)`、ZIP 经静态检查与 Git PR 安装、工作区独立启用，菜单/Casbin/entitlement 只认 `plugin.yaml`。

**Architecture:** 横切包 `platform/plugin` 提供与 GVA v2 相同的接口与注册表；启动时在 JWT/Casbin/entitlements 中间件之后、`routes.Collect` 之前调用 `InstallAll`。新 BC `modules/plugin` 管目录、安装任务与工作区绑定。安装器把包写入 `backend/internal/plugins/{id}` 并改 `register.go` 空白 import，经 `GitPublisher` 开 PR。插件业务路由前缀固定为 `/api/v1/plugins/{id}`。

**Tech Stack:** Go 1.26、Gin、GORM/PostgreSQL、`gopkg.in/yaml.v3`、`archive/zip`、`go/parser`、Next.js 15、Zod、现有 BFF 分层。GitHub PR 通过可替换的 `GitPublisher`（测试用内存实现）。

## Global Constraints

- 兼容面仅限：`Plugin` 接口、`Register`/`Registered`、`register.go` 空白 import、`init()`。不执行 GVA `initialize` 五件套。
- 无 `plugin.yaml` 或无 `Register(*gin.Engine)` 的 ZIP 一律拒绝。
- `Register(engine)` 必须发生在 `OptionalAuth`、`APIAccessMiddleware`、`entitlements.Middleware` **之后**，`routes.Collect` **之前**。
- 单个插件 `Register` panic：recover、打日志、继续；不得让 `bootstrap.Run` 失败。
- 平台安装：`authz.RequireSuperAdmin`。工作区启用/禁用：`authz.PermGovernanceAdmin`。
- 仅 `status=deployed` 的插件可被工作区 enable。卸载时若仍有 enabled 工作区返回 409（本期无 force）。
- 卸载不 DROP 插件业务表。
- 插件代码禁止 import 其它 BC 的 `domain`/`infrastructure`；允许 `platform/plugin`、`platform/httpx`、`platform/authn`。
- 不把逻辑塞进 `platform_admin` Tools stub；不迁移 platformsuite 业务。
- 不修改当前工作区里未提交的集群管理 WIP，除非 entitlement 路径映射必须加 `/api/v1/plugins/`。
- 每个任务完成并审查通过后，只提交该任务的插件相关文件；不得纳入工作区现有的其它改动。

---

## File map

- `backend/internal/platform/plugin/{plugin.go,registry.go,install.go}`：GVA v2 契约与安全安装。
- `backend/internal/plugins/register.go`：生成的空白 import 清单。
- `backend/internal/plugins/example/`：随仓库示例插件。
- `backend/internal/modules/plugin/`：Manifest、聚合、安装/启停、ZIP、Git、HTTP。
- `backend/migrations/000033_plugin_framework.{up,down}.sql`
- `backend/internal/bootstrap/app.go` + `migrate.go`
- `backend/internal/platform/entitlements/{entitlements.go,api_paths.go,middleware.go}`
- `backend/cmd/plugin-scaffold/main.go` + `backend/resource/plugin-template/`
- `fronted/packages/{contracts,upstream,bff-modules,web-sdk}` 与 `fronted/apps/web` 管理/启用 UI。
- `spec/plugin-sdk.md`：第三方开发说明。

---

### Task 1: GVA v2 注册表与 panic 隔离

**Files:**
- Create: `backend/internal/platform/plugin/plugin.go`
- Create: `backend/internal/platform/plugin/registry.go`
- Create: `backend/internal/platform/plugin/install.go`
- Create: `backend/internal/platform/plugin/registry_test.go`
- Create: `backend/internal/platform/plugin/install_test.go`

**Interfaces:**
- Consumes: `*gin.Engine`、`*zap.Logger`（可为 nil）。
- Produces: `type Plugin interface { Register(group *gin.Engine) }`；`func Register(p Plugin)`；`func Registered() []Plugin`；`func ResetRegistryForTest()`（仅测试）；`func InstallAll(engine *gin.Engine, logger *zap.Logger)`。

- [ ] **Step 1: 写注册表与 panic 隔离测试**

```go
package plugin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

type pingPlugin struct{}

func (pingPlugin) Register(engine *gin.Engine) {
	engine.GET("/api/v1/plugins/example/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
}

type panicPlugin struct{}

func (panicPlugin) Register(*gin.Engine) { panic("boom") }

func TestRegisterIgnoresNilAndSnapshots(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	sparkplugin.Register(nil)
	sparkplugin.Register(pingPlugin{})
	got := sparkplugin.Registered()
	require.Len(t, got, 1)
	sparkplugin.Register(pingPlugin{})
	require.Len(t, sparkplugin.Registered(), 2)
	require.Len(t, got, 1)
}

func TestInstallAllRecoversPanicAndKeepsLaterPlugins(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	sparkplugin.Register(panicPlugin{})
	sparkplugin.Register(pingPlugin{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sparkplugin.InstallAll(engine, zap.NewNop())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins/example/ping", nil)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/platform/plugin/ -count=1`（工作目录 `backend`）

Expected: FAIL，包不存在或符号未定义。

- [ ] **Step 3: 实现最小契约**

`plugin.go`：

```go
package plugin

import "github.com/gin-gonic/gin"

// Plugin matches gin-vue-admin utils/plugin/v2.
type Plugin interface {
	Register(group *gin.Engine)
}
```

`registry.go`：互斥锁切片；`Register` 忽略 nil；`Registered` 拷贝快照；`ResetRegistryForTest` 清空切片。

`install.go`：

```go
func InstallAll(engine *gin.Engine, logger *zap.Logger) {
	if engine == nil {
		return
	}
	for _, p := range Registered() {
		installOne(engine, logger, p)
	}
}

func installOne(engine *gin.Engine, logger *zap.Logger, p Plugin) {
	defer func() {
		if rec := recover(); rec != nil {
			if logger != nil {
				logger.Error("plugin register panicked", zap.Any("panic", rec))
			}
		}
	}()
	if p != nil {
		p.Register(engine)
	}
}
```

- [ ] **Step 4: 再跑测试**

Run: `go test ./internal/platform/plugin/ -count=1`

Expected: PASS。

---

### Task 2: example 插件、register.go、bootstrap 挂载顺序

**Files:**
- Create: `backend/internal/plugins/example/plugin.go`
- Create: `backend/internal/plugins/example/plugin.yaml`
- Create: `backend/internal/plugins/register.go`
- Create: `backend/internal/bootstrap/plugin_install_test.go`
- Modify: `backend/internal/bootstrap/app.go`（blank import `internal/plugins`；在 entitlements 中间件之后、`routes.Collect` 之前调用 `plugin.InstallAll`）

**Interfaces:**
- Consumes: Task 1 的 `InstallAll`、`Register`。
- Produces: example 插件 `init()` 调用 `sparkplugin.Register`；进程启动后 `GET /api/v1/plugins/example/ping` 存在于 Engine。

- [ ] **Step 1: 写启动顺序测试**

在 `plugin_install_test.go` 用 `gin.New()` 模拟：先 `Use` 一个中间件把 `X-MW` 设为 `auth`，再 `InstallAll`，handler 读取该值。断言插件路由能读到中间件写入的值。另断言：若先 `InstallAll` 再 `Use` 中间件，插件路由读不到（用两个 engine 对比，防止以后把调用顺序写反）。

```go
func TestPluginRoutesSeeAuthMiddlewareWhenInstalledAfter(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	sparkplugin.Register(mwProbePlugin{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("mw", "auth")
		c.Next()
	})
	sparkplugin.InstallAll(engine, zap.NewNop())
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/probe", nil))
	require.Equal(t, "auth", w.Body.String())
}
```

`mwProbePlugin.Register` 注册 `GET /api/v1/plugins/probe`，响应 `c.GetString("mw")`。

- [ ] **Step 2: 跑测试确认失败或顺序未接线**

Run: `go test ./internal/bootstrap/ -count=1 -run TestPluginRoutesSeeAuthMiddlewareWhenInstalledAfter`

Expected: 当前 FAIL（无测试或未实现 probe）。

- [ ] **Step 3: 实现 example 与 bootstrap**

`example/plugin.go`：

```go
package example

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

type plugin struct{}

func init() {
	sparkplugin.Register(&plugin{})
}

func (p *plugin) Register(engine *gin.Engine) {
	g := engine.Group("/api/v1/plugins/example")
	g.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
}
```

`plugin.yaml`：

```yaml
id: example
version: "0.1.0"
display_name: Example Plugin
route_prefix: /api/v1/plugins/example
entitlement: plugin:example
menus:
  - title: Example
    path: /plugins/example
    component: plugins/example/page
apis:
  - method: GET
    path: /api/v1/plugins/example/ping
frontend:
  pages:
    - app/(platform)/plugins/example/page.tsx
```

`register.go`：

```go
package plugins

import (
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins/example"
)
```

`app.go`：

```go
import (
	_ "github.com/sparkops/sparkops-agent/backend/internal/plugins"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)
```

在 `if c.DB != nil { c.Engine.Use(entitlements.Middleware(c.DB)) }` **之后**：

```go
sparkplugin.InstallAll(c.Engine, logger)
c.RouteRegistry = routes.Collect(c.Engine)
```

删除原来单独的 `routes.Collect` 行，避免收集两次。

- [ ] **Step 4: 跑测试**

Run:

```
go test ./internal/platform/plugin/ ./internal/bootstrap/ -count=1
```

Expected: PASS。手工：`go run ./cmd/server` 后 `GET /api/v1/plugins/example/ping` 返回 401/403 或 JSON（视鉴权），但路由必须存在（非 404）。

---

### Task 3: Manifest 解析与校验

**Files:**
- Create: `backend/internal/modules/plugin/domain/manifest/manifest.go`
- Create: `backend/internal/modules/plugin/domain/manifest/manifest_test.go`

**Interfaces:**
- Consumes: YAML 字节。
- Produces: `type Manifest struct { ID, Version, DisplayName, RoutePrefix, Entitlement string; Menus []MenuSpec; APIs []APISpec; Frontend FrontendSpec }`；`MenuSpec{Title, Path, Component string}`；`APISpec{Method, Path string}`；`FrontendSpec{Pages []string}`；`func Parse(raw []byte) (*Manifest, error)`；`func (m *Manifest) Validate() error`。

校验规则（失败返回 `kernel.NewDomainError("PLUGIN_MANIFEST_INVALID", msg)`）：

- `id` 匹配 `^[a-z][a-z0-9-]{1,62}$`
- `entitlement` 必须等于 `plugin:{id}`
- `route_prefix` 必须等于 `/api/v1/plugins/{id}`
- 至少一个 `apis`；每个 method 为 GET|POST|PUT|PATCH|DELETE；path 必须以 `route_prefix` 为前缀
- 每个 menu.path 必须以 `/plugins/{id}` 为前缀
- `frontend.pages` 至少一条

- [ ] **Step 1: 先写合法 yaml、缺字段、错误 entitlement、错误前缀测试**

覆盖：example 的 yaml 能 Parse；无 id 失败；`entitlement: delivery` 失败；path `/other` 失败。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/plugin/domain/manifest/ -count=1`

Expected: FAIL。

- [ ] **Step 3: 用 `gopkg.in/yaml.v3` 实现 Parse/Validate**（yaml 字段用 `yaml:"id"` 等 snake_case）。若 `go.mod` 尚无该依赖则 `go get gopkg.in/yaml.v3`。

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/modules/plugin/domain/manifest/ -count=1`

Expected: PASS。

---

### Task 4: 数据模型、迁移、仓储

**Files:**
- Create: `backend/migrations/000033_plugin_framework.up.sql`
- Create: `backend/migrations/000033_plugin_framework.down.sql`
- Modify: `backend/internal/shared/kernel/ids.go`（`PluginInstallID`）
- Create: `backend/internal/modules/plugin/domain/model/plugin.go`
- Create: `backend/internal/modules/plugin/domain/model/plugin_test.go`
- Create: `backend/internal/modules/plugin/domain/repository/repository.go`
- Create: `backend/internal/modules/plugin/domain/errors.go`
- Create: `backend/internal/modules/plugin/infrastructure/persistence/store.go`
- Create: `backend/internal/modules/plugin/infrastructure/persistence/store_test.go`
- Modify: `backend/internal/bootstrap/migrate.go`（加入 plugin AutoMigrate）

**Interfaces:**
- Produces 状态：`uploaded` `pr_open` `merged` `deployed` `pr_failed` `ci_failed`。
- `model.PluginRecord`：`ID string`（等于 manifest id，主键）、`Version`、`DisplayName`、`ManifestYAML`、`Status`、`PRURL`、`CreatedBy kernel.MemberID`、`CreatedAt`、`UpdatedAt`。
- `model.WorkspaceBinding`：`WorkspaceID`、`PluginID`、`Enabled bool`、`EnabledBy`、`EnabledAt`。
- `(*PluginRecord).MarkPROpen(url string)`；`MarkDeployed()`；`MarkPRFailed()`；`MarkCIFailed()`；仅允许 `uploaded→pr_open|pr_failed`，`pr_open→merged|pr_failed`，`merged→deployed|ci_failed`，`pr_failed→pr_open`，`ci_failed→pr_open`。
- `CanEnable() bool` 当且仅当 `Status==deployed`。
- Repository：`SavePlugin`、`FindPlugin(id)`、`ListPlugins`、`SaveBinding`、`FindBinding(ws, pluginID)`、`ListBindingsByPlugin`、`ListEnabledByWorkspace`、`DeleteBinding`。

`000033_plugin_framework.up.sql`：

```sql
CREATE SCHEMA IF NOT EXISTS plugin;

CREATE TABLE plugin.plugins (
    id            TEXT PRIMARY KEY,
    version       TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    manifest_yaml TEXT NOT NULL,
    status        TEXT NOT NULL,
    pr_url        TEXT NOT NULL DEFAULT '',
    created_by    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plugin.workspace_bindings (
    workspace_id UUID NOT NULL,
    plugin_id    TEXT NOT NULL REFERENCES plugin.plugins(id),
    enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    enabled_by   UUID,
    enabled_at   TIMESTAMPTZ,
    PRIMARY KEY (workspace_id, plugin_id)
);
```

down：`DROP TABLE IF EXISTS plugin.workspace_bindings; DROP TABLE IF EXISTS plugin.plugins; DROP SCHEMA IF EXISTS plugin;`

GORM 模型表名 `plugin.plugins` / `plugin.workspace_bindings`，`AutoMigrate` 先 `CREATE SCHEMA IF NOT EXISTS plugin`。

仓储测试可用 sqlite 或项目现有 testdb 工具；若模块惯用纯单元测试则只测状态机，GORM 测放到 `store_test.go` 并用 `gorm.Open(sqlite)`——**若仓库测试一律 PostgreSQL，则跟随现有 `persistence` 测试方式，不要引入新的 sqlite 依赖。** 没有集成 DB 时状态机单测必须有，store 测试用 `t.Skip` 仅当 `DATABASE_DSN` 为空。

- [ ] **Step 1: 状态机非法跳跃测试**
- [ ] **Step 2: 跑测失败**
- [ ] **Step 3: 实现模型 + SQL + store + migrate.go 注册 `pluginpersistence.AutoMigrate`**
- [ ] **Step 4: `go test ./internal/modules/plugin/domain/model/ -count=1` PASS**

---

### Task 5: ZIP 静态检查（含 GVA Register 符号）

**Files:**
- Create: `backend/internal/modules/plugin/application/install/zip.go`
- Create: `backend/internal/modules/plugin/application/install/zip_test.go`
- Create: `backend/internal/modules/plugin/application/install/ast.go`
- Create: `backend/internal/modules/plugin/application/install/ast_test.go`

**Interfaces:**
- Produces: `type InspectedZip struct { Manifest *manifest.Manifest; Files map[string][]byte }`（key 为清洗后相对路径）。
- `func InspectZip(raw []byte) (*InspectedZip, error)`
- `func FileDeclaresRegisterEngine(src []byte) bool`

ZIP 规则：

- 拒绝 `..`、绝对路径、反斜杠、空文件名
- 拒绝扩展名 `.sh` `.exe` `.bat` `.cmd` `.ps1` `.dll` `.so`
- 必须存在 `{id}/plugin.yaml` 或 `{id}/backend/plugin.yaml`
- yaml 的 `id` 必须等于顶层目录名
- 必须存在 `{id}/backend/plugin.go` 或 `{id}/backend/**/plugin.go`；对该文件 AST 检查：存在方法 `Register`，接收者任意，唯一参数类型为 `*gin.Engine`（SelectorExpr `gin` + `Engine`）
- 将 `{id}/backend/` 下文件映射为安装根相对路径（去掉 `{id}/backend/` 前缀）
- 将 `{id}/frontend/` 映射到前端相对路径（原样保留 `frontend/` 之后的部分）

- [ ] **Step 1: 测试 ZIP slip、脚本文件、无 yaml、无 Register、合法包**

合法包最小内容：`example/plugin.yaml`（Task 3 合法内容）+ `example/backend/plugin.go`（含 `func (p *plugin) Register(engine *gin.Engine)`）+ `example/frontend/app/(platform)/plugins/example/page.tsx`（任意 `export default function Page(){return null}`）。

- [ ] **Step 2: 跑测失败**
- [ ] **Step 3: 实现 InspectZip + go/parser AST**
- [ ] **Step 4: `go test ./internal/modules/plugin/application/install/ -count=1` PASS**

---

### Task 6: register.go 生成与 GitPublisher

**Files:**
- Create: `backend/internal/modules/plugin/application/install/registerfile.go`
- Create: `backend/internal/modules/plugin/application/install/registerfile_test.go`
- Create: `backend/internal/modules/plugin/application/install/git.go`
- Create: `backend/internal/modules/plugin/application/install/git_memory.go`
- Create: `backend/internal/modules/plugin/application/install/git_test.go`

**Interfaces:**
- `func RewriteRegisterGo(existing []byte, modulePath string, pluginIDs []string) ([]byte, error)`  
  `modulePath` 固定 `github.com/sparkops/sparkops-agent/backend`。  
  输出包名 `plugins`，import `_ "{modulePath}/internal/plugins/{id}"`，id 排序稳定。无插件时仍生成只有 `package plugins` 的合法文件。
- `type GitPublisher interface { Publish(ctx context.Context, req PublishRequest) (prURL string, err error) }`
- `type PublishRequest struct { Branch string; Message string; Files map[string][]byte }`（路径相对仓库根，如 `backend/internal/plugins/register.go`）
- `MemoryPublisher`：保存最后一次 `PublishRequest`，返回 `https://example.local/pr/1`。
- 生产必须使用真实 `GitCLIPublisher`：用 `os/exec` 调用 `git`/`gh`，工作目录为环境变量 `PLUGIN_GIT_ROOT`（必须是干净 Git 仓库）；未配置或校验失败时模块启动失败，禁止退回 Memory 或生成假 PR URL。`MemoryPublisher` 仅用于测试。`GitCLIPublisher` 执行：创建隔离 worktree/分支、写入或删除文件、`git add`、`git commit`、`git push -u`、`gh pr create --fill`；单测覆盖命令执行器接口与参数序列，不实际 push。

`RewriteRegisterGo` 用 `go/parser` 解析已有文件并替换 import 列表，而不是字符串拼接乱序；若 existing 为空则从模板新建。

- [ ] **Step 1: 测试从空文件生成 example import；再添加 second；再删除 example 只留 second**
- [ ] **Step 2: 跑测失败**
- [ ] **Step 3: 实现 RewriteRegisterGo、MemoryPublisher 与生产 GitCLIPublisher**
- [ ] **Step 4: 测试 PASS**

---

### Task 7: 安装/卸载命令与平台 HTTP

**Files:**
- Create: `backend/internal/modules/plugin/application/command/install.go`
- Create: `backend/internal/modules/plugin/application/command/install_test.go`
- Create: `backend/internal/modules/plugin/application/command/uninstall.go`
- Create: `backend/internal/modules/plugin/application/query/list.go`
- Create: `backend/internal/modules/plugin/interface/http/handler.go`
- Create: `backend/internal/modules/plugin/module.go`
- Modify: `backend/internal/bootstrap/app.go`（`modules` 切片追加 `pluginmod.New(tokens)`）

**Interfaces:**
- `InstallHandler.Handle(ctx, zip []byte, actor kernel.MemberID) (*model.PluginRecord, error)`
  1. `InspectZip`
  2. `FindPlugin` 若已存在 → `PLUGIN_ID_CONFLICT` 409
  3. 组装 Files：`backend/internal/plugins/{id}/` + 前端路径 `fronted/apps/web/` + 前缀相对 `frontend/` 的文件；读当前 `register.go`（安装根来自 `PLUGIN_GIT_ROOT` 或 `os.Getwd()` 上溯含 `backend/go.mod` 的目录），`RewriteRegisterGo` 加入新 id
  4. `SavePlugin` status=`uploaded`
  5. `publisher.Publish` branch=`plugin/install-{id}`；成功 `MarkPROpen`；失败 `MarkPRFailed` 仍保存
- `MarkMergedHandler` / `MarkDeployedHandler`：仅 Super Admin HTTP `POST /api/v1/admin/plugins/:id/merged` 与 `.../deployed`，供 CI 或人工推进（本期不写 GitHub webhook）。
- `UninstallHandler.Handle(ctx, id, actor)`：`ListBindingsByPlugin` 若有 `enabled=true` → `PLUGIN_IN_USE`；否则 Publish 删除目录文件（`PublishRequest` 增加 `Delete []string`），`RewriteRegisterGo` 去掉该 id，并将记录状态置为 `uninstalling`、保存卸载 PR URL。删除 PR 合并后置 `uninstall_merged`；部署成功回调 `MarkDeployedHandler` 在该状态下才删除 PluginRecord。创建 PR 后不得立即清元数据。测试覆盖 in-use 拒绝、状态转换、MemoryPublisher 收到 Delete 路径以及部署回调后删除记录。
- HTTP（均 `RequireSuperAdmin`）：
  - `POST /api/v1/admin/plugins/install` `multipart` 字段 `file`
  - `GET /api/v1/admin/plugins`
  - `GET /api/v1/admin/plugins/:id`
  - `POST /api/v1/admin/plugins/:id/merged`
  - `POST /api/v1/admin/plugins/:id/deployed`
  - `POST /api/v1/admin/plugins/:id/uninstall`
- `module.go`：`AutoMigrate`、组装 publisher、注册路由。`Name() string` 返回 `"plugin"`。

错误码：`PLUGIN_ZIP_INVALID`、`PLUGIN_MANIFEST_INVALID`、`PLUGIN_ID_CONFLICT`、`PLUGIN_IN_USE`、`PLUGIN_NOT_FOUND`、`PLUGIN_NOT_DEPLOYED`。在 `httpx.HandleError` 能映射的前提下使用 `kernel.NewDomainError`；若 HandleError 只认部分前缀，在 handler 里对 conflict/in-use 显式 `httpx.Fail(..., 409)`。

- [ ] **Step 1: InstallHandler 单测用 MemoryPublisher + 合法 zip；冲突第二次安装失败；Uninstall 在 enabled binding 时失败**
- [ ] **Step 2: 跑测失败**
- [ ] **Step 3: 实现 command + HTTP + module 注册**
- [ ] **Step 4: `go test ./internal/modules/plugin/... -count=1` PASS**

---

### Task 8: 工作区启用与 entitlement `plugin:{id}`

**Files:**
- Create: `backend/internal/modules/plugin/application/command/enable.go`
- Create: `backend/internal/modules/plugin/application/command/enable_test.go`
- Create: `backend/internal/modules/plugin/infrastructure/acl/governance_bind.go`
- Modify: `backend/internal/platform/entitlements/entitlements.go`（`Plugins []string`）
- Modify: `backend/internal/platform/entitlements/api_paths.go`（`APIPathEntitlement`、`AllowsModule`）
- Modify: `backend/internal/platform/entitlements/middleware.go` 无需改签名，依赖 AllowsModule
- Modify: entitlements 解析处（`ResolveCachedForWorkspace` 所在文件）：加载 `plugin.workspace_bindings` 中 `enabled=true` 的 `plugin_id`，填入 `ents.Plugins`
- Modify: `backend/internal/modules/plugin/interface/http/handler.go` 增加工作区路由
- Modify: `fronted/packages/bff-core/src/entitlements.ts` 与 `fronted/packages/contracts/src/platform/index.ts` 镜像

**Interfaces:**
- `type WorkspaceBinder interface { Bind(ctx, wsID, m *manifest.Manifest) error; Unbind(ctx, wsID, pluginID string) error }`
- `GovernanceBinder`：用 **GORM 直写** `governance.menus`（列与 `MenuStore.Save` 一致）插入 `path=/plugins/{id}` 菜单，`component` 来自 Manifest；`Unbind` 删除该 workspace 下 `path` 前缀 `/plugins/{id}` 的菜单。**不要** import `governance/domain`。查现有 `menus` 表结构（`000003`/`000023`）对齐列名。按钮：Manifest 无 buttons 则跳过。
- Casbin：enable 时把 Manifest `apis` 写入 `platform.apis`（`FirstOrCreate` 按 path+method）；给拥有 `governance:admin` 的角色加 `role_api_policies`。若现有 `APIRegistryStore` 在 governance persistence，plugin ACL 可用同一 GORM 表 `platform.apis` 直接 upsert，避免 import governance domain。禁用时删除该 plugin route_prefix 下的 `role_api_policies` 行（按 path like `/api/v1/plugins/{id}%`），**保留** `platform.apis` 全局记录。
- `EnableHandler`：插件必须 `CanEnable()`；事务：SaveBinding enabled=true + Binder.Bind。失败回滚。
- `DisableHandler`：Binder.Unbind + SaveBinding enabled=false。
- HTTP（`RequireAuth` + workspace 成员 + `PermGovernanceAdmin`）：
  - `GET /api/v1/workspaces/:wsId/plugins`（已部署列表 + enabled 标记）
  - `POST /api/v1/workspaces/:wsId/plugins/:id/enable`
  - `POST /api/v1/workspaces/:wsId/plugins/:id/disable`
- `APIPathEntitlement`：若 path 匹配 `/api/v1/plugins/{id}/` 则返回 `plugin:{id}`。`/api/v1/workspaces/:wsId/plugins` 管理接口 **跳过** entitlement（加入 `isGovernanceWorkspaceAPI` 的 `"/plugins"` 会误伤业务 path——**不要**用 Contains `/plugins` 配工作区。改为：`strings.Contains(path, "/workspaces/") && regexp 或 Trim 后 suffix 为 `/plugins` 或 `/plugins/{id}/enable|disable`）。
- `AllowsModule`：`strings.HasPrefix(key, "plugin:")` 时，id=`strings.TrimPrefix(key,"plugin:")`，`id` 在 `e.Plugins` 中则 true，否则 false。未知非 plugin key 仍 `return true`（保持旧行为）。
- `PathEntitlement`：`strings.HasPrefix(path, "/plugins/")` → `plugin:{第二段}`。
- `WorkspaceEntitlements` JSON 增加 `plugins []string`。前端 schema `plugins: z.array(z.string()).optional().default([])`。
- `isModuleEnabled`：若 `moduleKey.startsWith("plugin:")` 则 `entitlements.plugins.includes(id)`。`apiPathEntitlement` 返回类型改为 `string`（不要再强制 `keyof WorkspaceEntitlementsVM`）。

Enable 单测：未 deployed 失败；Memory/fake Binder 记录 Bind 调用；Disable 调用 Unbind。Entitlement 单测：`APIPathEntitlement("/api/v1/plugins/example/ping")=="plugin:example"`；Plugins 不含 example 时 `AllowsModule` false。

- [ ] **Step 1: 写 enable/entitlement 测试**
- [ ] **Step 2: 跑测失败**
- [ ] **Step 3: 实现 Binder、Resolve 填 Plugins、HTTP**
- [ ] **Step 4: `go test ./internal/modules/plugin/... ./internal/platform/entitlements/ -count=1` 与前端 `pnpm --filter @sparkops/bff-core test`（若无测试脚本则 `pnpm --filter web typecheck`）**

---

### Task 9: 前端管理 UI、工作区开关、example 页面

**Files:**
- Create: `fronted/packages/contracts/src/plugin/index.ts`
- Modify: contracts 包导出
- Modify: `fronted/packages/upstream/src/platform.ts` **或** 新建 `fronted/packages/upstream/src/plugin.ts`（推荐新文件，避免与集群 WIP 冲突）
- Create: `fronted/packages/bff-modules/src/plugins/index.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/plugins/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/plugins/install/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/plugins/[id]/merged/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/plugins/[id]/deployed/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/plugins/[id]/uninstall/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/plugins/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/plugins/[id]/enable/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/workspaces/[wsId]/plugins/[id]/disable/route.ts`
- Modify: `fronted/packages/web-sdk/src/platform.ts` 或新建 `web-sdk/src/plugin.ts`
- Create: `fronted/apps/web/src/modules/plugin/components/PlatformPluginsPanel.tsx`
- Create: `fronted/apps/web/app/(platform)/platform/plugins/page.tsx`
- Create: `fronted/apps/web/src/modules/plugin/components/WorkspacePluginsPanel.tsx`
- Create: `fronted/apps/web/app/(platform)/admin/plugins/page.tsx`（工作区管理员启用；若现有 admin 路由惯例不同，放到 `settings` 下已有权限页——优先 `(platform)/admin/plugins/page.tsx`）
- Create: `fronted/apps/web/app/(platform)/plugins/example/page.tsx`
- Modify: `nav-tree.ts` 仅平台树增加 `{ href: "/platform/plugins", label: "插件" }`；**不要**把 example 写进 BUSINESS_NAV_TREE（侧栏以 DB 菜单为准）
- Modify: `governance` `IsSuperAdminOnlyPath` 增加 `/platform/plugins`

合约 Zod：

```ts
export const PluginRecordVMSchema = z.object({
  id: z.string(),
  version: z.string(),
  display_name: z.string(),
  status: z.string(),
  pr_url: z.string().optional(),
  enabled: z.boolean().optional(),
});
```

PlatformPluginsPanel：上传 zip、表格展示 status/pr_url、按钮「标记已合并」「标记已部署」「卸载」。  
WorkspacePluginsPanel：列出 deployed 插件与启用开关。  
example 页：调用 BFF `GET /api/bff/v1/plugins/example/ping`——需再加 BFF 代理 `apps/web/app/api/bff/v1/plugins/example/ping/route.ts` 转发 upstream。upstream `getExamplePing()` → `GET /api/v1/plugins/example/ping`。

浏览器验证（有 dev server 时）：平台管理员打开 `/platform/plugins`；工作区管理员打开 `/admin/plugins` 启用 example 后侧栏出现菜单（需 enable 写入的菜单被 `PlatformNav` 拉取）；访问 `/plugins/example` 能 ping。无浏览器工具时用 curl 调 BFF。

- [ ] **Step 1: 合约 + upstream + bff-modules 函数**
- [ ] **Step 2: BFF route 与 web-sdk**
- [ ] **Step 3: 两个 Panel + 三个 page + example ping 页**
- [ ] **Step 4: `pnpm --filter web typecheck` PASS**

---

### Task 10: 脚手架、SDK 文档、端到端测试包

**Files:**
- Create: `backend/resource/plugin-template/plugin.yaml`
- Create: `backend/resource/plugin-template/backend/plugin.go.tmpl`
- Create: `backend/resource/plugin-template/frontend/app/(platform)/plugins/{{id}}/page.tsx.tmpl`
- Create: `backend/cmd/plugin-scaffold/main.go`
- Create: `spec/plugin-sdk.md`
- Create: `backend/internal/modules/plugin/application/install/scaffold_test.go`（断言模板替换后 yaml Validate 通过且 plugin.go 含 Register）

脚手架：`go run ./cmd/plugin-scaffold --id demo --name "Demo"` 在 stdout 或 `-out` 目录写出可被 `InspectZip` 接受的树（再 zip）。`plugin.go.tmpl` 必须是：

```go
package {{.Pkg}}

import (
	"github.com/gin-gonic/gin"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

type plugin struct{}

func init() { sparkplugin.Register(&plugin{}) }

func (p *plugin) Register(engine *gin.Engine) {
	g := engine.Group("/api/v1/plugins/{{.ID}}")
	g.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
}
```

`spec/plugin-sdk.md` 写清：import 路径不是 GVA；必须提供 plugin.yaml；安装走 PR；工作区 enable；禁止 initialize 五件套。

- [ ] **Step 1: 模板 + scaffold 生成后 InspectZip 测试**
- [ ] **Step 2: 跑测失败**
- [ ] **Step 3: 实现 cmd 与文档**
- [ ] **Step 4: `go test ./internal/modules/plugin/... ./internal/platform/plugin/ -count=1` 与 `go run ./cmd/plugin-scaffold --id demo -out $TMP` PASS**

---

## Spec coverage

| 规格条目 | 任务 |
|----------|------|
| GVA Plugin/Register/Registered/blank import | 1, 2 |
| InstallAll 在鉴权之后、Collect 之前 | 2 |
| Register panic 隔离 | 1 |
| plugin.yaml 为菜单/权限唯一来源 | 3, 8 |
| ZIP 检查、拒绝纯 GVA 包 | 5 |
| register.go 生成 + Git PR | 6, 7 |
| Super Admin 安装、人审、CI 用 merged/deployed 推进 | 7 |
| 工作区 enable + Casbin/菜单/entitlement | 8 |
| 前端管理与工作区开关 | 9 |
| 脚手架 + 第三方文档 | 10 |
| 不执行 initialize 五件套、不热加载、不 DROP 表 | 全局约束 |
| example 证明 init+Register | 2, 9 |

## 风险说明

- 真实 `gh pr create` 依赖本机 git 干净工作区；开发机有 WIP 时必须用 `PLUGIN_GIT_ROOT` 指向干净 clone。
- governance 菜单表列需对照迁移 SQL；ACL 写错列会导致 enable 失败，enable 测试在无 DB 时只测 Binder 接口假对象。
- 前端 `EntitlementModuleKey` 从 keyof 改为 string 后，所有 `isModuleEnabled` 调用仍应编译；typecheck 为门禁。
