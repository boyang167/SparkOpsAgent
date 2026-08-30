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

### `plugin.yaml`

- `id`: `[a-z][a-z0-9-]{1,62}`
- `entitlement`: must be `plugin:{id}`
- `route_prefix`: must be `/api/v1/plugins/{id}`
- At least one API (`method` UPPERCASE, `path` under `route_prefix`)
- Menu paths under `/plugins/{id}`
- At least one frontend page path

Without a valid `plugin.yaml`, install is rejected.

## Scaffold

From the `backend` module:

```bash
go run ./cmd/plugin-scaffold --id demo --name "Demo"
go run ./cmd/plugin-scaffold --id demo --name "Demo" -out /tmp/demo-plugin
```

The generated tree is suitable for ZIP packaging and must pass install `InspectZip`.

## Install & enable

1. **Install via Git PR** — only a **Super Admin** may upload a ZIP. The platform opens a PR that adds backend sources and a blank-import `register.go` entry. Merge/deploy advances install state. There is **no** runtime hot reload of Go plugins.
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
- Uninstall removes plugin metadata and generated files after deploy; it does **not DROP** plugin business tables.

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
	g := engine.Group("/api/v1/plugins/demo")
	g.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
}
```
