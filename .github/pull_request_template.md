## Summary

-

## Plugin review (if this PR installs or changes a plugin)

- [ ] N/A — not a plugin change
- [ ] `plugin.yaml` is present and is the only source of menus / APIs / entitlement
- [ ] Backend uses `Register(*gin.Engine)` + `init()` blank-import registration; no GVA initialize 五件套
- [ ] Plugin code does not import other BCs' `domain` or `infrastructure` (allowed: `platform/plugin`, `platform/httpx`, `platform/authn`)
- [ ] Frontend files stay under `app/(platform)/plugins/{id}/` (and optional `src/modules/plugins/{id}/` or BFF plugin routes)
- [ ] Install is Git PR + CI rebuild; no hot reload; uninstall must not DROP plugin business tables
