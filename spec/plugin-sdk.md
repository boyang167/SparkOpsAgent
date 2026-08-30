# SparkOps Plugin SDK

Third-party and internal plugins for SparkOps use the **SparkOps** plugin contract — not GVA’s initialize 五件套.

## Import path

Use SparkOps packages only:

```go
import (
	"github.com/gin-gonic/gin"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)
```

Do **not** import GVA plugin helpers or call GVA-style initialize routines.

## Required layout

A plugin ZIP must contain a single top-level directory whose name equals the plugin `id`, with:

| Path | Required |
|------|----------|
| `{id}/plugin.yaml` or `{id}/backend/plugin.yaml` | Yes — sole source of menus, APIs, entitlement |
| `{id}/backend/plugin.go` | Yes — must `init` `sparkplugin.Register` and implement `Register(*gin.Engine)` |
| `{id}/frontend/...` | At least one page listed in `frontend.pages` |

The ZIP inspector rejects packages with more than one manifest, more than 2000 entries, any single expanded entry over 50 MiB, or total expanded size over 80 MiB. Do not include `node_modules`, `.next`, `*.tsbuildinfo`, tests, or generated build output. Backend `.tar` assets are allowed only when their exact `backend/.../*.tar` path is declared in `assets`; undeclared or mismatched tar paths are rejected.

### `plugin.yaml`

- `id`: `[a-z][a-z0-9-]{1,62}`
- `entitlement`: must be `plugin:{id}`
- `route_prefix`: must be `/api/v1/plugins/{id}`
- At least one API (`method` UPPERCASE, `path` under `route_prefix`)
- Menu paths under `/plugins/{id}`
- At least one frontend page path
- Menu `parent` references must point to another menu in the same manifest and the tree depth must stay at 3 levels or less.
- `frontend.pages` entries are relative to `fronted/apps/web` and must exist under `{id}/frontend/...` in ZIP uploads.

Without a valid `plugin.yaml`, install is rejected.

### APIs and authorization

Manifest API paths use Casbin `keyMatch2` syntax: path parameters are written as `:param`. Wildcards are only supported as a trailing `/*` segment, for example `/api/v1/plugins/kubernetes/clusters/:clusterId/proxy/*`. Do not use deprecated `*` string replacement examples.

`method: ANY` is only valid with a trailing `/*` path. Workspace members may call read methods (`GET`, `HEAD`) through those routes. Mutating methods require the platform writer permission. A `read_only: true` exception is only valid for an exact `POST` route, not for wildcard `ANY` routes.

Plugin routes run behind the normal platform chain: JWT authentication, workspace context, Casbin when policies are present, plugin entitlement, and plugin-specific member/write checks. Error responses must use the platform envelope with `code`, `message`, `trace_id`, and `data.error_code`.

### Frontend, BFF, SSE, and assets

Frontend pages are installed under `fronted/apps/web/app/(platform)/plugins/{id}/...`; shared plugin UI/runtime helpers live under `fronted/apps/web/src/modules/plugins/{id}/...`. BFF routes for plugins live under `fronted/apps/web/app/api/bff/v1/plugins/{id}/...` when needed and must forward the platform JWT/session context instead of bypassing it.

Plugins that ship BFF routes require the Task17 platform baseline: `@sparkops/bff-core` plus the generic plugin proxy helper from `@sparkops/bff-modules/plugins`. Those platform helpers are not plugin descriptor files. The plugin descriptor should include the plugin-owned route file, such as `fronted/apps/web/app/api/bff/v1/plugins/kubernetes/[...path]/route.ts`, and rely on the platform baseline for shared BFF runtime code.

Server-sent event endpoints should keep the normal JWT and workspace checks, then stream through the route handler. Frontend SSE clients should parse platform/plugin error envelopes before switching to stream handling.

## Scaffold

From the `backend` module:

```bash
go run ./cmd/plugin-scaffold --id demo --name "Demo"
go run ./cmd/plugin-scaffold --id demo --name "Demo" -out /tmp/demo-plugin
```

The generated tree is suitable for ZIP packaging and must pass install `InspectZip`.

## Runtime entrypoint

Prefer `ContextPlugin` for new plugins:

```go
type plugin struct{}

func init() { sparkplugin.Register(&plugin{}) }

func (p *plugin) Register(engine *gin.Engine) {
	p.RegisterWithContext(engine, sparkplugin.PluginContext{})
}

func (p *plugin) RegisterWithContext(engine *gin.Engine, ctx sparkplugin.PluginContext) {
	g := engine.Group("/api/v1/plugins/demo")
	g.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
}
```

The legacy `Plugin.Register(*gin.Engine)` entrypoint remains compatible, but platform installation prefers `ContextPlugin.RegisterWithContext` when implemented. `PluginContext` exposes neutral platform ports:

- `Logger`: plugin-scoped logger.
- `DB`: shared database handle for plugin-owned schema/tables.
- `Audit`: audit event writer.
- `Clusters`: workspace-scoped Kubernetes cluster client factory.
- `Writer`: workspace write authorizer.
- `Members`: workspace membership authorizer.

`ClusterClientFactory.ForWorkspace(ctx, workspaceID, clusterID)` returns the typed Kubernetes client, dynamic client, `*rest.Config`, and neutral cluster metadata only after the platform confirms that the workspace may access that cluster. `ListMeta(ctx, workspaceID)` returns the same metadata shape without exposing kubeconfig material to plugin UI code.

## Install & enable

1. **Install via Git PR** — only a **Super Admin** may upload a ZIP. The platform opens a PR that adds backend sources, frontend files, assets, and a blank-import `register.go` entry. Merge/deploy advances install state. There is **no** runtime hot reload of Go plugins.
2. **Workspace enable** — a workspace member with `governance:admin` enables the plugin after status is `deployed`. That binds Casbin, menus, and entitlements for that workspace only.
3. Plugin HTTP routes under `/api/v1/plugins/{id}/` require a valid JWT (same chain as core APIs: auth → Casbin when policies exist → entitlements).
4. Frontend surfaces respect the `plugin:{id}` entitlement.

## Import restrictions

Plugin Go files **must not** import other bounded contexts' `domain` or `infrastructure` packages. Allowed SparkOps imports:

- `backend/internal/platform/plugin`
- `backend/internal/platform/httpx`
- `backend/internal/platform/authn`

## Uninstall

- Uninstall returns **409** while any workspace still has the plugin enabled (no force in this release).
- Uninstall uses the install receipt to remove generated files after deploy.
- Uninstall removes plugin metadata, but it does **not DROP** plugin-owned schemas or business tables.

## Explicitly out of scope

- **Do not** run GVA initialize 五件套 (`Initialize`, router/menu/Casbin auto-wiring, etc.).
- **No hot reload** — code lands through merge + deploy only.
- **Do not DROP** plugin business tables or mutate core schema from plugin install.

## Minimal `plugin.go`

```go
package demo

import (
	"github.com/gin-gonic/gin"
	sparkplugin "github.com/sparkops/sparkops-agent/backend/internal/platform/plugin"
)

type plugin struct{}

func init() { sparkplugin.Register(&plugin{}) }

func (p *plugin) Register(engine *gin.Engine) {
	p.RegisterWithContext(engine, sparkplugin.PluginContext{})
}

func (p *plugin) RegisterWithContext(engine *gin.Engine, ctx sparkplugin.PluginContext) {
	g := engine.Group("/api/v1/plugins/demo")
	g.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
}
```
