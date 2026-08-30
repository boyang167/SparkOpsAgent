# SparkOps Kubernetes Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付内置 `plugin:kubernetes` 工作区控制台（Proxy + 资源目录 UI + 专项实时能力），并补齐规格要求的框架扩展（PluginContext、ANY 前缀 Casbin、菜单树、ZIP 规模与 `.tar`）。

**Architecture:** 插件只通过 `platform/plugin.PluginContext` 拿到 DB、审计与 `ClusterClientFactory`；工厂在 bootstrap 用平台集群表证书建 client，插件不 import `platform_admin`/`runtime` 的 domain/infrastructure。apiserver 流量走安全 Proxy；Prometheus/Velaux/LLM 存在 `plugin_kubernetes.cluster_settings`。前端用 catalog 驱动通用列表/详情/YAML，专项页走 BFF 流式代理。

**Tech Stack:** Go 1.26、Gin、GORM/PostgreSQL、client-go 0.32、Casbin keyMatch、Next.js 15 App Router、现有 BFF `proxyUpstreamStream`、SparkOps 设计系统组件。

## Global Constraints

- 不嵌入 Vue，不执行 GVA initialize 五件套；不交付 Karmada；不卸载 DROP `plugin_kubernetes`。
- 插件 Go 禁止 import 其它 BC 的 `domain`/`infrastructure`；允许 `platform/plugin`、`platform/httpx`、`platform/authn` 及 Context 注入的端口。
- 集群连接只来自 `cluster_info` 证书；不提供第二套粘贴 kubeconfig 主路径；settings 不存 kubeconfig。
- enable 后成员只读（GET/HEAD + 只读指标查询）；创建/更新/删除、Exec、文件写、诊断、NodeTTY、Chat 写路径仅 `governance:admin`。
- 实时通道只保留 SSE（或同源 WS 二选一，本计划统一 SSE）；前端不直连后端端口；流式必须带 JWT。
- 缺 CRD：菜单保留，页面展示上游错误。选中已授权集群后可见全部 Namespace。
- 不修改无关的集群管理 WIP；每任务只提交该任务相关文件。
- 工作目录跑 Go 测试时用 `backend`：`go test ./... -count=1` 的包路径均相对 `backend`。
- Windows Git 提交：`C:\1-install\13-git\Git\bin\git.exe` + `-F` 消息文件，不要 `--trailer`。

---

## File map

- `backend/internal/platform/plugin/{plugin.go,context.go,install.go,registry.go}`：Plugin、ContextPlugin、InstallAllWithContext。
- `backend/internal/platform/plugin/cluster/{factory.go,factory_test.go}`：`ClusterClientFactory` 与凭据端口（无其它 BC import）。
- `backend/internal/bootstrap/{app.go,migrate.go,plugin_cluster.go}`：注入 factory、改为 `InstallAllWithContext`、调用 kubernetes AutoMigrate。
- `backend/internal/modules/plugin/domain/manifest/manifest.go`：`ANY`、`parent`、`assets`。
- `backend/internal/modules/plugin/infrastructure/acl/governance_bind.go`：菜单树、只读/写策略拆分。
- `backend/internal/platform/policy/model.conf`：分组策略对 path 使用 `keyMatch`。
- `backend/internal/modules/plugin/application/install/zip.go`：`maxZipEntries=2000`、允许声明的 `.tar`。
- `backend/internal/plugins/kubernetes/`：插件实现（http、proxy、metrics、files、stream、tty、diagnosis、velaux、chat、schema）。
- `backend/internal/plugins/register.go`：空白 import kubernetes。
- `backend/migrations/000035_plugin_kubernetes_seed.{up,down}.sql`（可选；优先 GORM seed，与 example 一致）。
- `fronted/apps/web/src/modules/plugins/kubernetes/catalog.ts`：GVK 目录。
- `fronted/apps/web/app/(platform)/plugins/kubernetes/**`：控制台页面。
- `fronted/apps/web/app/api/bff/v1/plugins/kubernetes/**`：BFF。
- `fronted/packages/{upstream,bff-modules,web-sdk}`：kubernetes 客户端封装。
- `spec/plugin-sdk.md`：ContextPlugin、ANY、assets。

---

### Task 1: PluginContext 与 ContextPlugin

**Files:**
- Create: `backend/internal/platform/plugin/context.go`
- Modify: `backend/internal/platform/plugin/install.go`
- Modify: `backend/internal/platform/plugin/registry_test.go`（或新 `install_context_test.go`）
- Modify: `backend/internal/bootstrap/app.go`
- Modify: `backend/internal/bootstrap/plugin_install_test.go`

**Interfaces:**
- Consumes: 现有 `Plugin`、`InstallAll`。
- Produces:

```go
package plugin

type AuditEvent struct {
	WorkspaceID string
	MemberID    string
	Action      string
	Detail      map[string]any
}

type AuditLogger interface {
	Record(ctx context.Context, event AuditEvent) error
}

type PluginContext struct {
	Logger   *zap.Logger
	DB       *gorm.DB
	Audit    AuditLogger
	Clusters ClusterClientFactory // 本任务可为 nil；Task 2 补全类型
}

type ContextPlugin interface {
	Plugin
	RegisterWithContext(engine *gin.Engine, ctx PluginContext)
}

func InstallAllWithContext(engine *gin.Engine, pctx PluginContext)
```

- [ ] **Step 1: 写失败测试**

`backend/internal/platform/plugin/install_context_test.go`：

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

type legacyOnly struct{}

func (legacyOnly) Register(engine *gin.Engine) {
	engine.GET("/api/v1/plugins/legacy/ping", func(c *gin.Context) { c.String(http.StatusOK, "legacy") })
}

type withCtx struct{ sawDB bool }

func (p *withCtx) Register(*gin.Engine) {}

func (p *withCtx) RegisterWithContext(engine *gin.Engine, ctx sparkplugin.PluginContext) {
	p.sawDB = ctx.DB != nil
	engine.GET("/api/v1/plugins/ctx/ping", func(c *gin.Context) { c.String(http.StatusOK, "ctx") })
}

func TestInstallAllWithContextPrefersRegisterWithContext(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	p := &withCtx{}
	sparkplugin.Register(legacyOnly{})
	sparkplugin.Register(p)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sparkplugin.InstallAllWithContext(engine, sparkplugin.PluginContext{Logger: zap.NewNop(), DB: &gorm.DB{}})
	require.True(t, p.sawDB)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/legacy/ping", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "legacy", w.Body.String())
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/ctx/ping", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "ctx", w.Body.String())
}

func TestInstallAllDelegatesToInstallAllWithContext(t *testing.T) {
	sparkplugin.ResetRegistryForTest()
	t.Cleanup(sparkplugin.ResetRegistryForTest)
	sparkplugin.Register(legacyOnly{})
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sparkplugin.InstallAll(engine, zap.NewNop())
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/plugins/legacy/ping", nil))
	require.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/platform/plugin/ -count=1`（cwd `backend`）

Expected: FAIL，`InstallAllWithContext` 未定义。

- [ ] **Step 3: 实现**

`context.go`：定义 `AuditEvent`、`AuditLogger`、`PluginContext`、`ContextPlugin`。`ClusterClientFactory` 先写成：

```go
type ClusterClientFactory interface{}
```

空接口占位，Task 2 立刻替换为完整接口（同 PR 内不要合并空接口到 master：本任务实现完整接口签名但 factory 可返回 `errNotWired`，或 Task 1+2 由同一执行者连续完成。**执行时将 Task 1 与 Task 2 视为同一提交批次若单独 Task 1 无法编译。**）

为可独立编译，Task 1 即写入 Task 2 的完整 `ClusterClientFactory` 接口（见 Task 2 Produces），`Clusters` 字段允许 nil；插件在 nil 时返回 503。

`install.go`：

```go
func InstallAll(engine *gin.Engine, logger *zap.Logger) {
	InstallAllWithContext(engine, PluginContext{Logger: logger})
}

func InstallAllWithContext(engine *gin.Engine, pctx PluginContext) {
	if engine == nil {
		return
	}
	for _, p := range Registered() {
		installOne(engine, pctx, p)
	}
}

func installOne(engine *gin.Engine, pctx PluginContext, p Plugin) {
	defer func() {
		if rec := recover(); rec != nil && pctx.Logger != nil {
			pctx.Logger.Error("plugin register panicked", zap.Any("panic", rec))
		}
	}()
	if p == nil {
		return
	}
	if cp, ok := p.(ContextPlugin); ok {
		cp.RegisterWithContext(engine, pctx)
		return
	}
	p.Register(engine)
}
```

`app.go`：将 `sparkplugin.InstallAll(c.Engine, logger)` 换成 `InstallAllWithContext`，传入 `Logger`、`DB`（`Clusters`/`Audit` 在 Task 2/13 接线）。

`plugin_install_test.go`：断言源码含 `InstallAllWithContext`，且仍位于 `RequireMemberForPluginAPIs` 之后、`routes.Collect` 之前。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/platform/plugin/ ./internal/bootstrap/ -count=1`

Expected: PASS（bootstrap 中与 DB 无关的源码测试）。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/plugin backend/internal/bootstrap/app.go backend/internal/bootstrap/plugin_install_test.go
git commit -m "Add plugin RegisterWithContext so plugins can receive platform ports."
```

---

### Task 2: ClusterClientFactory 与 bootstrap 适配器

**Files:**
- Create: `backend/internal/platform/plugin/cluster.go`
- Create: `backend/internal/platform/plugin/cluster_test.go`
- Create: `backend/internal/bootstrap/plugin_cluster.go`
- Create: `backend/internal/bootstrap/plugin_cluster_test.go`
- Modify: `backend/internal/bootstrap/app.go`（注入 factory）

**Interfaces:**
- Consumes: `kernel.WorkspaceID`、`kernel.ClusterInfoID`；平台表字段与 `repository.ClusterInfo` 相同（adapter 内映射，插件包不引用该类型）。
- Produces:

```go
type ClusterMeta struct {
	ID, Name, Server                                  string
	PrometheusURL, PrometheusUser, PrometheusPassword string
	VelauxURL, VelauxUser, VelauxPassword             string
	LLMBaseURL, LLMAPIKey, LLMModel                   string
}

type ClusterCredential struct {
	ID                       kernel.ClusterInfoID
	Name                     string
	Server                   string
	CertificateAuthorityData string
	ClientCertificateData    string
	ClientKeyData            string
}

type ClusterCredentialSource interface {
	ListAuthorized(ctx context.Context, wsID kernel.WorkspaceID) ([]ClusterCredential, error)
	GetAuthorized(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (*ClusterCredential, error)
}

type ClusterClientFactory interface {
	ForWorkspace(ctx context.Context, wsID kernel.WorkspaceID, clusterID kernel.ClusterInfoID) (
		kubernetes.Interface, dynamic.Interface, *rest.Config, *ClusterMeta, error,
	)
	ListMeta(ctx context.Context, wsID kernel.WorkspaceID) ([]ClusterMeta, error)
}

func NewFactory(src ClusterCredentialSource) ClusterClientFactory
```

`ListMeta`/`ClusterMeta` 的 Prometheus 等字段本任务留空字符串；Task 8 在插件层用 `cluster_settings` 覆盖。

- [ ] **Step 1: 写失败测试**

`cluster_test.go`：假 `ClusterCredentialSource`。未授权 `GetAuthorized` 返回 domain 风格错误（用 `kernel.NewDomainError("PLUGIN_CLUSTER_FORBIDDEN", "cluster is not granted to workspace")`）。授权且 PEM 有效时 `ForWorkspace` 返回非 nil rest.Config.Host。缺证书三件套返回 `PLUGIN_CLUSTER_CREDENTIALS_INCOMPLETE`。

用 `encoding/pem` + `crypto/ecdsa` 在测试里生成自签证书（或最小假 PEM 若 client-go 在 NewForConfig 才校验——`ForWorkspace` 应在缺字段时失败，不必真连 apiserver）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/platform/plugin/ -count=1 -run Cluster`

Expected: FAIL。

- [ ] **Step 3: 实现 `NewFactory`**

用 `rest.Config{Host: cred.Server, TLSClientConfig: rest.TLSClientConfig{CAData, CertData, KeyData: []byte(...)}}` + `kubernetes.NewForConfig` + `dynamic.NewForConfig`。禁止把 CA/Cert/Key 写入返回错误字符串。

`plugin_cluster.go`：实现 `ClusterCredentialSource`，**仅在 bootstrap 包** 调用 `platform_admin` persistence 的 `ListWorkspaceClusters` / `WorkspaceClusterGranted` + 按 ID 取证书（可直接用现有 `AdminRepository`，bootstrap 已依赖该模块）。

未授权：`GetAuthorized` 返回 forbidden，不列举其它工作区集群。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/platform/plugin/ ./internal/bootstrap/ -count=1 -run 'Cluster|Plugin'`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/plugin/cluster.go backend/internal/platform/plugin/cluster_test.go backend/internal/bootstrap/plugin_cluster.go backend/internal/bootstrap/plugin_cluster_test.go backend/internal/bootstrap/app.go
git commit -m "Inject authorized cluster clients into plugins without copying kubeconfig."
```

---

### Task 3: Manifest 允许 ANY 前缀、菜单 parent、assets

**Files:**
- Modify: `backend/internal/modules/plugin/domain/manifest/manifest.go`
- Modify: `backend/internal/modules/plugin/domain/manifest/manifest_test.go`

**Interfaces:**
- Consumes: 现有 `Parse`/`Validate`。
- Produces: `APISpec.Method` 可为 `ANY`；`MenuSpec.Parent string`；`Manifest.Assets []string`。

校验规则（逐条实现，不要放宽）：

1. `ANY` 仅当 `path` 以 `/*` 结尾，且 `strings.HasPrefix(path, m.RoutePrefix)`。
2. `ANY` 不能用于不带 `/*` 的精确路径。
3. 非 ANY 方法集合仍为 GET/POST/PUT/PATCH/DELETE。
4. `parent` 为空或必须是本插件某条 `menus[].path`，禁止指向其它插件；禁止环（实现：parent 链深度 ≤ 3，且 parent ≠ 自身）。
5. `assets` 每项必须是相对路径且前缀 `backend/`，后缀 `.tar`，无 `..`。

- [ ] **Step 1: 写失败测试**

在 `manifest_test.go` 增加：合法 `ANY` + `/*`；非法 `ANY` 精确路径；非法 `parent`；合法两层 parent；非法 `assets` 指向 `frontend/`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/plugin/domain/manifest/ -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现 Validate 扩展**

- [ ] **Step 4: 跑测试确认通过**

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/plugin/domain/manifest
git commit -m "Allow plugin manifest prefix APIs and nested menus."
```

---

### Task 4: Enable 时 Casbin 前缀匹配与成员只读

**Files:**
- Modify: `backend/internal/platform/policy/model.conf`
- Create: `backend/internal/platform/policy/matcher_test.go`
- Modify: `backend/internal/modules/plugin/infrastructure/acl/governance_bind.go`
- Modify: `backend/internal/modules/plugin/infrastructure/acl/governance_bind_test.go`

**Interfaces:**
- Consumes: Task 3 `ANY`、`parent`。
- Produces: Bind 后 Casbin/role_api_policies 行为如下。

菜单：按 `path` upsert；`parent` 空则挂现有 `/plugins` 根；`parent` 非空则 `parent_id` 为同工作区该 path 的菜单 id（先创建父再创建子，YAML 顺序必须父在子前；测试覆盖乱序时返回 `PLUGIN_MANIFEST_INVALID` 或 binder 两轮扫描——**实现选两轮扫描**：第一轮全部 upsert，第二轮填 parent_id）。

API 展开：

- `ANY` + `/*`：写入 `platform.apis` 时 method 存 `*` 或展开为 GET/HEAD/POST/PUT/PATCH/DELETE（选 **展开六种 method**，避免改 apis 表唯一约束语义）。Casbin object 存 **去掉末尾 `*` 后的前缀再加 `*`**，例如 path `/api/v1/plugins/kubernetes/clusters/:clusterId/proxy/*` 规范化为 `/api/v1/plugins/kubernetes/clusters/*/proxy/*`（把 `:param` 换成 `*`）。
- 精确 API 仍精确 path + method。

角色：

- 工作区**所有** role：绑定 **GET、HEAD** 的展开策略（只读）。
- 仅 `governance:admin` role：再绑定 POST/PUT/PATCH/DELETE。
- 另外：精确路径若 method 为 POST 且 path 包含 `/metrics` 的后缀 `/metrics`，视为只读，也赋给全部 role。
- 精确路径包含 `/exec`、`/files`、`/cloudtty`、`/nodetty`、`/diagnosis`、`/chat`、`/velaux` 的 **非 GET/HEAD**：仅 admin。
- Proxy 的 GET/HEAD 给全部 role；Proxy 的写 method 仅 admin。

Casbin `model.conf` matchers 改为：

```
m = (r.sub == p.sub && r.dom == p.dom && r.obj == p.obj && r.act == p.act) || (g(r.sub, p.sub, r.dom) && r.dom == p.dom && (keyMatch(r.obj, p.obj) || keyMatch2(r.obj, p.obj)) && r.act == p.act)
```

`matcher_test.go`：内存 enforcer，policy `(role, ws, /api/v1/plugins/kubernetes/clusters/*/proxy/*, GET)` + grouping member→role，请求 path `/api/v1/plugins/kubernetes/clusters/abc/proxy/api/v1/pods` method GET 允许；同一 path POST 无策略则拒绝。

- [ ] **Step 1: 写 matcher 与 binder 测试（含 viewer GET、viewer POST 拒绝、admin 不测 Casbin skip）**

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/platform/policy/ ./internal/modules/plugin/infrastructure/acl/ -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现 bind 与 matcher**

- [ ] **Step 4: 跑测试确认通过**

Expected: PASS。现有 example bind 测试仍通过（精确 GET 只给 admin 的旧断言必须改：example GET 现在也给 viewer）。**更新** `TestGovernanceBinderBindsAndUnbindsPluginResources`：viewer 应有 GET ping 策略。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/platform/policy/model.conf backend/internal/platform/policy/matcher_test.go backend/internal/modules/plugin/infrastructure/acl
git commit -m "Bind plugin prefix APIs so members can read without listing every GVK."
```

---

### Task 5: ZIP 2000 条目、声明的 `.tar`、总量上限

**Files:**
- Modify: `backend/internal/modules/plugin/application/install/zip.go`
- Modify: `backend/internal/modules/plugin/application/install/zip_test.go`

**Interfaces:**
- Consumes: Task 3 `Manifest.Assets`。
- Produces: `maxZipEntries = 2000`；`maxZipExpandedBytes = 80 << 20`（单文件仍 `50 << 20`）；`.tar` 仅当 `name` 映射后的 backend 相对路径出现在 `manifest.Assets` 中（assets 写 `backend/resource/arthas-bin.tar`，zip 条目 `kubernetes/backend/resource/arthas-bin.tar`）。未声明 `.tar` 拒绝。`.sh/.exe` 等仍拒绝。

- [ ] **Step 1: 改 `TestInspectZipRejectsMoreThanOneThousandEntries` 为 2000：构造 1998 个额外空文件使其失败；新增 1997 额外文件应成功（加上必填 3 个条目低于 2000）。新增 `TestInspectZipAcceptsDeclaredTarUnderBackend`；`TestInspectZipRejectsUndeclaredTar`。**

`makeSizedZip` 的条目上限参数按新常量调整。`TestInspectZipRejectsMoreThanFiftyMiBUncompressed` 改为超过 80MiB（例如九个 9MiB）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/modules/plugin/application/install/ -count=1 -run InspectZip`

Expected: FAIL。

- [ ] **Step 3: 实现常量与 `.tar` 检查**（在读完所有文件并 parse manifest 之后校验 assets；未声明 tar 在第一轮可用 `pendingTar` 列表第二轮核对）。

- [ ] **Step 4: 跑测试确认通过**

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/modules/plugin/application/install/zip.go backend/internal/modules/plugin/application/install/zip_test.go
git commit -m "Raise plugin zip limits so the kubernetes package and arthas tarball can install."
```

---

### Task 6: `plugin_kubernetes` schema 与 AutoMigrate

**Files:**
- Create: `backend/internal/plugins/kubernetes/schema/models.go`
- Create: `backend/internal/plugins/kubernetes/schema/migrate.go`
- Create: `backend/internal/plugins/kubernetes/schema/migrate_test.go`
- Modify: `backend/internal/bootstrap/migrate.go`

**Interfaces:**
- Produces:

```go
type ClusterSettings struct {
	ClusterID           string `gorm:"primaryKey;size:36"`
	PrometheusURL       string
	PrometheusUser      string
	PrometheusPassword  string
	VelauxURL           string
	VelauxUser          string
	VelauxPassword      string
	LLMBaseURL          string
	LLMAPIKey           string
	LLMModel            string
}

func (ClusterSettings) TableName() string { return "plugin_kubernetes.cluster_settings" }

type Record struct {
	ID          string `gorm:"primaryKey;size:36"`
	WorkspaceID string `gorm:"index;size:36"`
	ClusterID   string `gorm:"size:36"`
	Kind        string
	StartedAt   time.Time
	EndedAt     *time.Time
	Blob        []byte
	ObjectPath  string
}

func (Record) TableName() string { return "plugin_kubernetes.records" }

type Diagnosis struct {
	ID          string `gorm:"primaryKey;size:36"`
	WorkspaceID string `gorm:"size:36"`
	ClusterID   string `gorm:"size:36"`
	Namespace   string
	Pod         string
	Port        int
	ExpiresAt   time.Time
}

func (Diagnosis) TableName() string { return "plugin_kubernetes.diagnosis" }

func AutoMigrate(db *gorm.DB) error
```

SQLite 测试：跳过 `CREATE SCHEMA`（与 plugin persistence 相同，`db.Dialector.Name() == "postgres"` 才建 schema）。

- [ ] **Step 1: sqlite AutoMigrate 测试创建三表**

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/plugins/kubernetes/schema/ -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现 AutoMigrate；`migrate.go` 增加一步 `"plugin_kubernetes", k8sschema.AutoMigrate`（import 插件 schema 包，bootstrap 允许）。**

- [ ] **Step 4: 跑测试确认通过**

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/plugins/kubernetes/schema backend/internal/bootstrap/migrate.go
git commit -m "Add plugin_kubernetes tables without storing kubeconfig."
```

---

### Task 7: 内置 kubernetes 插件骨架、manifest、deployed 种子

**Files:**
- Create: `backend/internal/plugins/kubernetes/plugin.go`
- Create: `backend/internal/plugins/kubernetes/plugin.yaml`
- Create: `backend/internal/plugins/kubernetes/http/router.go`（空路由组 + 健康 `GET /ready`）
- Modify: `backend/internal/plugins/register.go`
- Modify: `backend/internal/modules/plugin/infrastructure/persistence/store.go`（`seedBundledKubernetesPlugin`，镜像 example）

**Interfaces:**
- Consumes: `RegisterWithContext`、完整 `plugin.yaml`（菜单树对齐 GVA `initialize/menu.go` + Velero `/plugins/kubernetes/backup` + 根落地 `/plugins/kubernetes`）。
- Produces: `init` 注册；`GET /api/v1/plugins/kubernetes/ready` 返回 `{"ok":true}`。

`plugin.yaml` 必须包含：

- 根菜单 `/plugins/kubernetes`
- 子菜单（parent 均为根或分组）：clusters、nodes、nodelabelstaints（可 hidden: 本期 MenuSpec 无 hidden 则仍注册可见或 path 仍存在）、workloads、openkruise、kruise-rollouts、kubeblocks、kubevela、namespaces、networks、gateway、configs、storages、accessControls、records、diagnosis、backup、chat、cloudtty（若 GVA 无独立菜单则挂在诊断/集群页入口，**仍要有路由页** `/plugins/kubernetes/cloudtty`）、velaux 入口页。
- apis：`GET .../ready`；`GET .../clusters`；`PUT .../clusters/:clusterId/settings`；`ANY .../clusters/:clusterId/proxy/*`；`POST .../clusters/:clusterId/metrics`；文件/日志/exec/records/cloudtty/nodetty/diagnosis/velaux/chat 精确或前缀条（与规格路由表一致）。
- frontend.pages：至少列出将在 Task 16 创建的页面路径；本任务可先建占位 `page.tsx` 以免 InspectZip 失败。
- assets: `backend/resource/arthas-bin.tar`——本任务用 **最小 tar 文件**（几个零字节文件打成 tar）放 `backend/internal/plugins/kubernetes/resource/arthas-bin.tar`，正式二进制在 Task 15 替换；测试不要求 19MiB。

- [ ] **Step 1: 写 `plugin_test.go` httptest：InstallAllWithContext 后 GET ready=200。写 persistence 测试或复用 store 测试断言 kubernetes 种子 status=deployed。**

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/plugins/kubernetes/ ./internal/modules/plugin/infrastructure/persistence/ -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现骨架、空白 import、seed。占位前端页：`fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx` 返回简单标题。**

- [ ] **Step 4: 跑测试确认通过**

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/plugins/kubernetes backend/internal/plugins/register.go backend/internal/modules/plugin/infrastructure/persistence/store.go fronted/apps/web/app/(platform)/plugins/kubernetes/page.tsx
git commit -m "Bundle the kubernetes plugin skeleton as a deployed workspace plugin."
```

---

### Task 8: 集群列表与 settings

**Files:**
- Create: `backend/internal/plugins/kubernetes/http/clusters.go`
- Create: `backend/internal/plugins/kubernetes/http/clusters_test.go`
- Create: `backend/internal/plugins/kubernetes/http/write_guard.go`

**Interfaces:**
- Consumes: `PluginContext.Clusters.ListMeta`、`schema.ClusterSettings`。
- Produces:
  - `GET /api/v1/plugins/kubernetes/clusters`：当前工作区已授权集群；合并 settings；**响应中 password/apiKey 用 `***` 替换，永不回传原文**。
  - `PUT /api/v1/plugins/kubernetes/clusters/:clusterId/settings`：仅 admin（`write_guard`：`authz` PermissionChecker 无法从插件 import governance——用 gin context 已有成员 + 注入的 `WriteAuthorizer` 接口：

```go
type WriteAuthorizer interface {
	CanWrite(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error)
}
```

在 PluginContext 增加 `Writer WriteAuthorizer`，bootstrap 用 `permChecker.HasPermission(..., PermGovernanceAdmin)` 实现。成员 PUT → 403 `GOVERNANCE_API_ACCESS_DENIED`。未授权 clusterId → 403 `PLUGIN_CLUSTER_FORBIDDEN`。

工作区 ID：`httpx.WorkspaceIDFromContext` / `X-Workspace-Id`（与平台一致）。

- [ ] **Step 1: 用 fake factory + sqlite settings 写 GET/PUT 测试**

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/plugins/kubernetes/http/ -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现；GET 合并 meta 空 settings 与 DB 覆盖。**

- [ ] **Step 4: 跑测试确认通过**

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/plugins/kubernetes/http backend/internal/platform/plugin/context.go backend/internal/bootstrap/app.go
git commit -m "List granted clusters and store Prometheus Velaux LLM settings per cluster."
```

---

### Task 9: 安全 apiserver Proxy

**Files:**
- Create: `backend/internal/plugins/kubernetes/proxy/rewrite.go`
- Create: `backend/internal/plugins/kubernetes/proxy/rewrite_test.go`
- Create: `backend/internal/plugins/kubernetes/proxy/handler.go`
- Create: `backend/internal/plugins/kubernetes/proxy/handler_test.go`

**Interfaces:**
- Consumes: `ForWorkspace` 得到 `rest.Config`。
- Produces: `ANY /clusters/:clusterId/proxy/*path` 转发到 apiserver。

`rewrite.go` 纯函数（对齐 GVA `compatibleClusterVersion`）：

```go
func CompatibleAPIPath(minor int, path string) string
```

- minor ≤ 18：`networking.k8s.io/v1` + `ingresses` → `v1beta1`
- minor > 18：`/apis/batch/v1beta1` + `cronjobs` → `/apis/batch/v1`

`handler.go`：

- 用 `httputil.NewSingleHostReverseProxy`，Director 设置 URL 为 `restCfg.Host` + `*path`（保证以 `/` 开头），TLS 用 restCfg。
- Query `page`/`pageSize`/`keyword`/`ns`（多 ns 逗号分隔）：若为 list GET，则对每个 ns 请求上游，合并 `items`，keyword 过滤 `metadata.name`，再分页（逻辑对齐 GVA `pageFilter`/`fieldFilter`）。无这些 query 则原样流式拷贝（大对象 watch 不在本期：`watch=true` 直接流式，不做合并）。
- 上游错误：状态码透传；响应体若含 `certificate`/`token`/`client-key` 则替换为 `redacted`。
- 写方法走 `WriteAuthorizer`。

测试：`httptest.NewServer` 模拟 apiserver；factory 返回指向该 server 的 rest.Config（`Host: srv.URL`，`TLSClientConfig: Insecure` 仅测试）。

- [ ] **Step 1: rewrite 单测 + handler 多 ns 合并单测**

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/plugins/kubernetes/proxy/ -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现并挂到 router**

- [ ] **Step 4: 跑测试确认通过**

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/plugins/kubernetes/proxy backend/internal/plugins/kubernetes/http/router.go
git commit -m "Proxy workspace-authorized clusters to the apiserver with GVA list filters."
```

---

### Task 10: Prometheus 指标查询

**Files:**
- Create: `backend/internal/plugins/kubernetes/metrics/query.go`
- Create: `backend/internal/plugins/kubernetes/metrics/query_test.go`
- Create: `backend/internal/plugins/kubernetes/http/metrics.go`

**Interfaces:**
- Produces: `POST /clusters/:clusterId/metrics` JSON `{ "query": "up", "time": optional }`。用 settings 中 Prometheus URL + basic auth。无 URL → 400 `PLUGIN_PROMETHEUS_NOT_CONFIGURED`。上游失败 502，body 不含密码。

查询模板（与 GVA 节点/Pod 图对齐，作为服务端允许的 named query，防任意 PromQL 注入）：请求 `{ "template": "node_cpu", "range": "1h" }`，白名单 map 内插 `__node__`/`__pod__`/`__namespace__` 必须匹配 `^[a-z0-9.-]+$`。未知 template → 400。

- [ ] **Step 1: httptest Prometheus 返回 vector；非法 template 失败**

- [ ] **Step 2–4: TDD 实现、通过、Commit**

```bash
git commit -m "Query Prometheus with named templates from cluster settings."
```

---

### Task 11: Pod 文件 ls/upload/download/delete

**Files:**
- Create: `backend/internal/plugins/kubernetes/files/handler.go`
- Create: `backend/internal/plugins/kubernetes/files/handler_test.go`

**Interfaces:**
- Produces:
  - `GET /clusters/:clusterId/namespaces/:ns/pods/:pod/files?path=`
  - `POST` multipart upload
  - `DELETE`
  - `GET .../files/download`

实现：`kubernetes.Interface` exec `ls`/`cat`/`rm`（与 runtime podtool 同类，但代码放本插件包，禁止 import runtime）。写操作 `WriteAuthorizer`。路径拒绝 `..`。

测试：fake exec（接口 `PodExecutor`，生产实现 SPDY，测试假数据）。

- [ ] **Step 1–5: TDD、Commit**

```bash
git commit -m "Add pod file list and transfer behind workspace cluster auth."
```

---

### Task 12: 日志与 Exec SSE + JWT

**Files:**
- Create: `backend/internal/plugins/kubernetes/stream/logs.go`
- Create: `backend/internal/plugins/kubernetes/stream/exec.go`
- Create: `backend/internal/plugins/kubernetes/stream/stream_test.go`
- Modify: `backend/internal/platform/authn/optional.go` 或现有 JWT 中间件：query `access_token` **仅当** 与 `Authorization`/`cookie` 将解析出同一 member 才接受；禁止单独 `x-token`。若现有 `OptionalAuth` 已读 Bearer，则增加：无 header 时读 `access_token` query，调用同一 parse。
- Create: `backend/internal/platform/authn/query_token_test.go`

**Interfaces:**
- Produces: `GET .../logs` 与 `GET .../exec` `text/event-stream`。Exec 必须 `WriteAuthorizer`。日志 GET 成员只读允许。断开连接时若有 `Audit` 则 `Record` ended。

测试：fake stream 写两条 SSE；query token 与 header 不一致 → 401。

- [ ] **Step 1–5: TDD、Commit**

```bash
git commit -m "Stream pod logs and exec over SSE with JWT on every request."
```

---

### Task 13: 终端审计 records

**Files:**
- Create: `backend/internal/plugins/kubernetes/records/handler.go`
- Create: `backend/internal/plugins/kubernetes/records/handler_test.go`
- Modify: `context.go`：`NopAudit` 与 bootstrap 接到 operationlog 或插件表（审计元数据写 `plugin_kubernetes.records`，**禁止**把 exec 原文当 multipart 记入平台 operation log）。

**Interfaces:**
- `GET /records` 分页列表（无 blob）；`GET /records/:id` 返回播放数据；`DELETE /records/:id` 仅 admin。Exec 会话结束写入 blob（asciicast 简化 JSON 行）。

- [ ] **Step 1–5: TDD、Commit**

```bash
git commit -m "Store kubernetes terminal recordings in plugin_kubernetes.records."
```

---

### Task 14: CloudTTY 与 NodeTTY

**Files:**
- Create: `backend/internal/plugins/kubernetes/tty/cloudtty.go`
- Create: `backend/internal/plugins/kubernetes/tty/nodetty.go`
- Create: `backend/internal/plugins/kubernetes/tty/tty_test.go`

**Interfaces:**
- `POST /clusters/:clusterId/cloudtty` JSON `{namespace, pod, container}` → 创建临时 Pod/Service（对齐 GVA CloudTTY 资源名 prefix `sparkops-cloudtty-`），返回 `{id}`；失败删除已创建对象。
- `POST /clusters/:clusterId/nodetty` JSON `{node}`：镜像默认 `alpine:latest`，`nsenter`，HostPID；仅 admin；文档注释标明特权风险。失败清理。

测试：fake dynamic/kubernetes reactor 记录 create/delete；注入错误时断言 delete 被调用。

- [ ] **Step 1–5: TDD、Commit**

```bash
git commit -m "Provision CloudTTY and privileged NodeTTY with cleanup on failure."
```

---

### Task 15: Arthas、Velaux、Chat

**Files:**
- Create: `backend/internal/plugins/kubernetes/diagnosis/arthas.go`
- Create: `backend/internal/plugins/kubernetes/diagnosis/arthas_test.go`
- Create: `backend/internal/plugins/kubernetes/velaux/proxy.go`
- Create: `backend/internal/plugins/kubernetes/velaux/proxy_test.go`
- Create: `backend/internal/plugins/kubernetes/chat/query.go`
- Create: `backend/internal/plugins/kubernetes/chat/query_test.go`

**Interfaces:**
- Arthas：从 `embed` 或 `os.DirFS` 读取 `resource/arthas-bin.tar`（路径相对插件模块，禁止 `./resource/` 相对进程 cwd）；拷入目标容器、起会话写入 `diagnosis` 表；过期清理；失败删临时资源。SSE `GET .../diagnosis/:id/ws` 实际用 SSE。
- Velaux：`ANY /clusters/:clusterId/velaux/*path` 反代 settings.VelauxURL，basic auth；无 URL → 400。
- Chat：`POST /clusters/:clusterId/chat/query` SSE；LLM 来自 settings，缺省 400 `PLUGIN_LLM_NOT_CONFIGURED`；prompt 注入集群只读摘要（用 client 列 namespace 数量，禁止把 kubeconfig 放进 prompt）。仅 admin。

- [ ] **Step 1–5: TDD（fake HTTP LLM/Velaux、fake copy-to-pod）、Commit**

```bash
git commit -m "Add Arthas diagnosis, Velaux proxy, and cluster-aware chat SSE."
```

---

### Task 16: 前端 catalog、通用资源页、集群选择

**Files:**
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/catalog.ts`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/catalog.test.ts`（`import { describe, expect, it } from "vitest"`，与 `cluster-authorization-state.test.ts` 相同）
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/ClusterPicker.tsx`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/ResourceTable.tsx`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/YamlEditor.tsx`
- Create: `fronted/apps/web/src/modules/plugins/kubernetes/ApiErrorBanner.tsx`
- Create: `fronted/apps/web/app/(platform)/plugins/kubernetes/layout.tsx`
- Create: 各菜单路由 `page.tsx`：clusters、nodes、workloads、openkruise、kruise-rollouts、kubeblocks、kubevela、namespaces、networks、gateway、configs、storages、access-controls、backup、records、diagnosis、chat、cloudtty、velaux、create。
- Modify: `plugin.yaml` `frontend.pages` 与上述路径一致。

**Interfaces:**
- `catalog.ts` 导出：

```ts
export type ResourceDef = {
  id: string;
  group: string;
  version: string;
  kind: string;
  plural: string;
  namespaced: boolean;
  columns: { key: string; title: string }[];
  menu: string;
};

export const CATALOG: ResourceDef[];
export function resourcesForMenu(menu: string): ResourceDef[];
export function proxyPath(clusterId: string, def: ResourceDef, ns?: string, name?: string): string;
```

`CATALOG` 必须覆盖规格原生 + OpenKruise + Kruise Rollouts + KubeBlocks + KubeVela Application + Velero Backup/Restore/Schedule。`proxyPath` 生成 `/api/bff/v1/plugins/kubernetes/clusters/${clusterId}/proxy/...`。

通用页：`ResourceTable` 调 BFF GET proxy；403/404/`ENTITLEMENT_MODULE_DISABLED` 用 `ApiErrorBanner`。写按钮仅当 entitlements/session 表明 admin（复用现有 governance 标志，不要发明新 RBAC）。YAML 保存 PUT proxy。创建向导：常见 workload 表单字段对齐 GVA（name、image、replicas、ports），提交构造 Deployment JSON。

集群选择：localStorage key `sparkops.plugin.kubernetes.clusterId`，仅允许当前 GET /clusters 返回的 id。

视觉：只用现有 layout/table/button，不引入 Element Plus。

- [ ] **Step 1: `resourcesForMenu`/`proxyPath` 单测**

- [ ] **Step 2: 跑测试确认失败**

Run（cwd `fronted`）: `pnpm exec vitest run apps/web/src/modules/plugins/kubernetes/catalog.test.ts`

Expected: FAIL，模块不存在。

- [ ] **Step 3: 实现 catalog 与页面后再次跑 vitest；再跑 `pnpm --filter @sparkops/web typecheck`（cwd `fronted`）。无浏览器工具时不要声称 UI 已手工点过。**

- [ ] **Step 4: Commit**

```bash
git commit -m "Add kubernetes console pages driven by a GVK catalog."
```

---

### Task 17: BFF 与 SDK 转发（含流式）

**Files:**
- Create: `fronted/apps/web/app/api/bff/v1/plugins/kubernetes/[...path]/route.ts`
- Create: `fronted/packages/upstream/src/plugin_kubernetes.ts`（或扩展 `plugin.ts`）
- Modify: `fronted/packages/bff-modules/src/plugins/index.ts`
- Modify: `fronted/packages/web-sdk/src/plugin.ts`
- Modify: `fronted/packages/bff-core` 仅当需要 `proxyUpstreamStream` 支持任意 method/path（现有函数只 GET：为 kubernetes 增加 `proxyUpstreamStreamRequest(ctx, path, init)`，转发 method/headers/body，SSE 时复制 `text/event-stream`）。

**Interfaces:**
- Catch-all 将 `/api/bff/v1/plugins/kubernetes/${path}` 转到上游 `/api/v1/plugins/kubernetes/${path}`，带 workspace cookie/token。
- 流式 path 后缀 `logs`、`exec`、`chat/query`、`diagnosis/` 用 stream helper。

- [ ] **Step 1: 若有 bff 单测则补；否则最小 route 与类型导出。**

- [ ] **Step 2–4: 实现、`pnpm` typecheck、Commit**

```bash
git commit -m "Proxy kubernetes plugin APIs and SSE through the BFF."
```

---

### Task 18: InspectZip 夹具、SDK 文档、验收测试清单落地

**Files:**
- Create: `backend/internal/plugins/kubernetes/zip_inspect_test.go`（把 backend+frontend 映射成 zip 调 `InspectZip`）
- Modify: `spec/plugin-sdk.md`（ContextPlugin、ANY、assets、`.tar`、菜单 parent）
- Create: `backend/internal/plugins/kubernetes/http/authz_matrix_test.go`：同一套路由，member GET proxy 200、member POST proxy 403、未 enable entitlement 中间件已由平台覆盖则可 mock 403 码。

**Interfaces:**
- InspectZip 成功；zip_inspect 使用真实 `plugin.yaml` 与占位/实现文件。

- [ ] **Step 1: InspectZip 测试失败（缺文件）→ 补齐 manifest pages**

- [ ] **Step 2: `go test ./internal/plugins/kubernetes/... ./internal/modules/plugin/application/install/ -count=1` PASS**

- [ ] **Step 3: 更新 `spec/plugin-sdk.md`**

- [ ] **Step 4: Commit**

```bash
git commit -m "Prove the kubernetes plugin zip inspects cleanly and document ContextPlugin."
```

---

## Spec coverage

| 规格项 | 任务 |
|--------|------|
| bundled + ZIP，id=kubernetes | 7, 18 |
| PluginContext / Register 兼容 | 1 |
| ClusterClientFactory，不复制 kubeconfig | 2, 8 |
| 路由表 Proxy/metrics/files/stream/records/tty/arthas/velaux/chat | 9–15 |
| ANY `/*` Casbin + 成员只读 | 3, 4 |
| 菜单树 + Velero | 3, 4, 7, 16 |
| SSE + JWT | 12, 17 |
| ZIP 2000 + tar + 50MiB 单文件 | 5 |
| plugin_kubernetes 三表，卸载不 DROP | 6 |
| 缺 CRD 展示错误 | 9, 16 ApiErrorBanner |
| 不交付 Karmada | 7/16 不建菜单 |
| 与 runtime 并存 | 无删除 runtime 任务 |
| GVA 缺陷：落地页、Velero、终端通道 | 7, 12, 16 |
| 错误不泄漏证书 | 9, 10, 2 |
| CI 不连真集群 | 全程 fake/httptest |

## 执行说明

任务 1 必须带上 Task 2 的 `ClusterClientFactory` 完整接口才能编译；执行者应连续完成 1–2 再提交，或单提交包含两者。
