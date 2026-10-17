# SSO Runtime Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让超级管理员在平台设置页安全配置、测试并热更新 OIDC 与 LDAP，且不向前端、日志或通用系统配置暴露密钥。

**Architecture:** identity 模块新增独立 SSO 配置仓储、加密服务、连接测试器和原子运行时快照。管理 API 负责脱敏读取、测试及事务保存，公开登录接口只消费当前快照；前端通过专用 contracts、BFF 和设置表单管理配置。

**Tech Stack:** Go 1.26、Gin、GORM、AES-GCM、`golang.org/x/oauth2`、`go-ldap/ldap/v3`、Next.js 15、React 19、TypeScript 5.8、Zod、Vitest。

## Global Constraints

- OIDC 与 LDAP 可同时配置和启用；登录页只展示默认提供商。
- 启用提供商前，当前配置指纹必须具有成功测试记录。
- OIDC 测试验证 Discovery、端点与字段，不声称在真实授权码回调前验证 Client Secret 真伪。
- 敏感字段使用 `platform/crypto.SecretBox` 加密；读取 API 只返回密钥存在标记。
- 空密钥输入保留原值，只有显式 `clear_client_secret` / `clear_bind_password` 才清除。
- 保存失败不得替换上一份运行时快照；保存成功后当前实例立即生效。
- `config.yaml` 仅在数据库无 SSO 配置时作为兼容来源。
- 管理接口仅超级管理员可访问，日志和错误中不得包含密钥。
- 前端使用现有组件、CSS 类和设计 Token，不新增 UI 库或硬编码色值。

---

## File Map

### Backend

- Create `backend/internal/modules/identity/domain/model/sso_config.go`: 配置类型、规范化、指纹与启用规则。
- Create `backend/internal/modules/identity/domain/model/sso_config_test.go`: 配置规则单元测试。
- Create `backend/internal/modules/identity/domain/repository/sso_config_repository.go`: 仓储接口。
- Create `backend/internal/modules/identity/infrastructure/persistence/sso_config_repo.go`: GORM 模型、迁移、加密读写和乐观版本控制。
- Create `backend/internal/modules/identity/infrastructure/persistence/sso_config_repo_test.go`: SQLite 持久化与密钥泄露测试。
- Modify `backend/internal/modules/identity/infrastructure/auth/oidc.go`: Discovery、端点覆盖和可注入 HTTP Client。
- Modify `backend/internal/modules/identity/infrastructure/auth/ldap.go`: 可注入连接测试与超时。
- Create `backend/internal/modules/identity/application/service/sso_runtime.go`: 原子快照、配置测试、刷新与 YAML 回退。
- Create `backend/internal/modules/identity/application/service/sso_runtime_test.go`: 热更新、失败回滚与测试指纹测试。
- Create `backend/internal/modules/identity/application/command/sso_config.go`: 读取、测试和保存用例。
- Create `backend/internal/modules/identity/interface/http/sso_admin_handler.go`: 管理 API 请求/响应映射。
- Modify `backend/internal/modules/identity/interface/http/sso_handler.go`: 公开接口改为运行时快照。
- Modify `backend/internal/modules/identity/module.go`: 组装依赖、迁移、路由和刷新协程。

### Frontend

- Modify `fronted/packages/contracts/src/identity/index.ts`: SSO 管理 Zod schema 与类型。
- Modify `fronted/packages/upstream/src/identity.ts`: 上游管理 API。
- Modify `fronted/packages/bff-modules/src/identity/sso.ts`: BFF 用例。
- Add `fronted/apps/web/app/api/bff/v1/admin/sso/route.ts`: GET/PUT。
- Add `fronted/apps/web/app/api/bff/v1/admin/sso/test/route.ts`: POST。
- Modify `fronted/packages/web-sdk/src/identity.ts`: 浏览器 SDK。
- Create `fronted/apps/web/src/modules/platform_admin/components/sso-config-state.ts`: 草稿、指纹失效和请求映射纯函数。
- Create `fronted/apps/web/src/modules/platform_admin/components/sso-config-state.test.ts`: 纯函数测试。
- Create `fronted/apps/web/src/modules/platform_admin/components/SSOSettingsSection.tsx`: 三段式 SSO 表单。
- Modify `fronted/apps/web/src/modules/platform_admin/components/PlatformSettingsPanel.tsx`: 接入专用 SSO 面板，移除旧的简化区块。
- Modify `fronted/apps/web/app/globals.css`: 只补充现有平台设置命名空间下的响应式布局样式。

---

### Task 1: SSO Domain Model and Validation

**Files:**
- Create: `backend/internal/modules/identity/domain/model/sso_config.go`
- Test: `backend/internal/modules/identity/domain/model/sso_config_test.go`

**Interfaces:**
- Produces:
  - `type SSOProvider string`
  - `type OIDCSettings struct`
  - `type LDAPSettings struct`
  - `type SSOConfiguration struct`
  - `func (c SSOConfiguration) Normalize() SSOConfiguration`
  - `func (c SSOConfiguration) ProviderFingerprint(provider SSOProvider) string`
  - `func (c SSOConfiguration) ValidateForSave() error`

- [ ] **Step 1: Write failing normalization, fingerprint and lockout tests**

```go
func TestSSOConfigurationFingerprintChangesOnlyForProviderInputs(t *testing.T) {
    cfg := validSSOConfiguration()
    first := cfg.ProviderFingerprint(model.SSOProviderOIDC)
    cfg.LoginUI.ShowPasswordLogin = !cfg.LoginUI.ShowPasswordLogin
    require.Equal(t, first, cfg.ProviderFingerprint(model.SSOProviderOIDC))
    cfg.OIDC.Issuer = "https://idp.example.com/tenant"
    require.NotEqual(t, first, cfg.ProviderFingerprint(model.SSOProviderOIDC))
}

func TestSSOConfigurationRejectsPasswordLockout(t *testing.T) {
    cfg := validSSOConfiguration()
    cfg.LoginUI.Enabled = true
    cfg.LoginUI.ShowPasswordLogin = false
    cfg.OIDC.Enabled = false
    cfg.LDAP.Enabled = false
    require.ErrorContains(t, cfg.ValidateForSave(), "至少启用一个")
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `cd backend; go test ./internal/modules/identity/domain/model -run 'TestSSOConfiguration' -count=1`

Expected: FAIL because `SSOConfiguration` and its methods do not exist.

- [ ] **Step 3: Implement canonical configuration and SHA-256 fingerprints**

```go
type SSOProvider string

const (
    SSOProviderOIDC SSOProvider = "oidc"
    SSOProviderLDAP SSOProvider = "ldap"
)

type LoginUISettings struct {
    Enabled bool `json:"enabled"`
    Provider SSOProvider `json:"provider"`
    ShowPasswordLogin bool `json:"show_password_login"`
}

type OIDCSettings struct {
    Enabled bool `json:"enabled"`
    Issuer string `json:"issuer"`
    ClientID string `json:"client_id"`
    ClientSecret string `json:"-"`
    RedirectURL string `json:"redirect_url"`
    Scopes string `json:"scopes"`
    AuthorizationEndpoint string `json:"authorization_endpoint"`
    TokenEndpoint string `json:"token_endpoint"`
    UserInfoEndpoint string `json:"userinfo_endpoint"`
}

type LDAPSettings struct {
    Enabled bool `json:"enabled"`
    URL string `json:"url"`
    BindDN string `json:"bind_dn"`
    BindPassword string `json:"-"`
    UserSearchBase string `json:"user_search_base"`
    UserSearchFilter string `json:"user_search_filter"`
    EmailAttribute string `json:"email_attribute"`
    NameAttribute string `json:"name_attribute"`
}

type SSOConfiguration struct {
    LoginUI LoginUISettings
    OIDC OIDCSettings
    LDAP LDAPSettings
    OIDCTestedFingerprint string
    LDAPTestedFingerprint string
    Version int64
}
```

Normalize whitespace, lowercase provider values, sort scopes, apply defaults, marshal only the selected provider’s normalized connection fields, and hash with `sha256.Sum256`.

- [ ] **Step 4: Run domain tests**

Run: `cd backend; go test ./internal/modules/identity/domain/model -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the domain model**

```powershell
git add backend/internal/modules/identity/domain/model/sso_config.go backend/internal/modules/identity/domain/model/sso_config_test.go
git commit -m "feat(identity): define SSO configuration rules"
```

---

### Task 2: Encrypted SSO Configuration Repository

**Files:**
- Create: `backend/internal/modules/identity/domain/repository/sso_config_repository.go`
- Create: `backend/internal/modules/identity/infrastructure/persistence/sso_config_repo.go`
- Test: `backend/internal/modules/identity/infrastructure/persistence/sso_config_repo_test.go`

**Interfaces:**
- Consumes: `model.SSOProvider`, `model.SSOConfiguration`, `crypto.SecretBox`.
- Produces:

```go
type SSOConfigRepository interface {
    Load(ctx context.Context) (*model.SSOConfiguration, error)
    Save(ctx context.Context, expectedVersion int64, cfg model.SSOConfiguration) (*model.SSOConfiguration, error)
}
```

- [ ] **Step 1: Write failing repository tests**

Tests must prove:

```go
saved, err := repo.Save(ctx, 0, cfg)
require.NoError(t, err)
require.Equal(t, int64(1), saved.Version)

var rows []map[string]any
require.NoError(t, db.Table("identity.sso_configs").Find(&rows).Error)
raw, _ := json.Marshal(rows)
require.NotContains(t, string(raw), "oidc-secret")
require.NotContains(t, string(raw), "ldap-secret")

loaded, err := repo.Load(ctx)
require.NoError(t, err)
require.Equal(t, "oidc-secret", loaded.OIDC.ClientSecret)
require.Equal(t, "ldap-secret", loaded.LDAP.BindPassword)
```

Add a second test where two writers save the same version and the second returns domain code `IDENTITY_SSO_VERSION_CONFLICT`.

- [ ] **Step 2: Run persistence tests and confirm failure**

Run: `cd backend; go test ./internal/modules/identity/infrastructure/persistence -run SSOConfig -count=1`

Expected: FAIL because the repository does not exist.

- [ ] **Step 3: Implement two provider rows and encrypted secret blobs**

Use this storage shape:

```go
type ssoConfigModel struct {
    Provider string `gorm:"primaryKey;size:16"`
    Enabled bool
    Config string `gorm:"type:text;not null"`
    SecretBlob string `gorm:"type:text"`
    TestedFingerprint string `gorm:"size:64"`
    TestedAt *time.Time
    Version int64 `gorm:"not null"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (ssoConfigModel) TableName() string { return "identity.sso_configs" }
```

`Save` must execute inside `db.Transaction`, compare both stored versions with `expectedVersion`, encrypt `{client_secret}` and `{bind_password}`, then upsert both rows with one incremented version.

In the same transaction, load the latest non-deleted `platform_admin.sys_cfg` row, preserve every existing top-level key, replace only its `sso` object with:

```go
map[string]any{
    "enabled": cfg.LoginUI.Enabled,
    "provider": string(cfg.LoginUI.Provider),
    "show_password_login": cfg.LoginUI.ShowPasswordLogin,
}
```

`Load` reconstructs `LoginUI` from that same object and applies `provider=oidc`, `show_password_login=true` defaults when the object is absent. Add a test proving an unrelated `features` key survives `Save`.

- [ ] **Step 4: Run repository tests**

Run: `cd backend; go test ./internal/modules/identity/infrastructure/persistence -run SSOConfig -count=1`

Expected: PASS and no secret appears in persisted JSON/config columns.

- [ ] **Step 5: Commit encrypted persistence**

```powershell
git add backend/internal/modules/identity/domain/repository/sso_config_repository.go backend/internal/modules/identity/infrastructure/persistence/sso_config_repo.go backend/internal/modules/identity/infrastructure/persistence/sso_config_repo_test.go
git commit -m "feat(identity): persist encrypted SSO settings"
```

---

### Task 3: Provider Discovery, Connection Tests, and Runtime Snapshot

**Files:**
- Modify: `backend/internal/modules/identity/infrastructure/auth/oidc.go`
- Modify: `backend/internal/modules/identity/infrastructure/auth/ldap.go`
- Create: `backend/internal/modules/identity/application/service/sso_runtime.go`
- Test: `backend/internal/modules/identity/application/service/sso_runtime_test.go`

**Interfaces:**
- Consumes: repository from Task 2 and YAML `config.Auth`.
- Produces:

```go
type ProviderTestResult struct {
    Provider model.SSOProvider
    Fingerprint string
    TestedAt time.Time
    Message string
}

type SSOSnapshot struct {
    Configuration model.SSOConfiguration
    OIDC *auth.OIDCClient
    LDAP *auth.LDAPAuthenticator
}

type SSORuntime struct {
    repo repository.SSOConfigRepository
    fallback config.AuthConfig
    httpClient *http.Client
    snapshot atomic.Pointer[SSOSnapshot]
    now func() time.Time
}

func NewSSORuntime(repo repository.SSOConfigRepository, fallback config.AuthConfig, httpClient *http.Client) *SSORuntime
func (r *SSORuntime) Load(ctx context.Context) error
func (r *SSORuntime) Snapshot() SSOSnapshot
func (r *SSORuntime) TestProvider(ctx context.Context, cfg model.SSOConfiguration, provider model.SSOProvider) (ProviderTestResult, error)
func (r *SSORuntime) Replace(cfg model.SSOConfiguration) error
func (r *SSORuntime) RefreshIfChanged(ctx context.Context) error
```

- [ ] **Step 1: Write failing runtime tests**

Cover:

```go
func TestRuntimeKeepsOldSnapshotWhenReplacementIsInvalid(t *testing.T) {
    runtime := loadedRuntime(t)
    before := runtime.Snapshot()
    invalid := before.Configuration
    invalid.OIDC.Issuer = "://bad"
    require.Error(t, runtime.Replace(invalid))
    require.Equal(t, before.Configuration.OIDC.Issuer, runtime.Snapshot().Configuration.OIDC.Issuer)
}

func TestOIDCTestUsesDiscoveryAndAdvancedOverride(t *testing.T) {
    // httptest server returns discovery JSON; assert override token endpoint wins.
}
```

LDAP tests use an injected `LDAPDialer` fake so no external directory is required.

- [ ] **Step 2: Run service tests and confirm failure**

Run: `cd backend; go test ./internal/modules/identity/application/service -run 'SSO|OIDC|LDAP' -count=1`

Expected: FAIL because runtime types do not exist.

- [ ] **Step 3: Add OIDC Discovery and injectable transports**

Add:

```go
type OIDCDiscovery struct {
    Issuer string `json:"issuer"`
    AuthorizationEndpoint string `json:"authorization_endpoint"`
    TokenEndpoint string `json:"token_endpoint"`
    UserInfoEndpoint string `json:"userinfo_endpoint"`
}

func DiscoverOIDC(ctx context.Context, client *http.Client, issuer string) (OIDCDiscovery, error)
func NewOIDCClientFromSettings(settings model.OIDCSettings, discovery OIDCDiscovery) (*OIDCClient, error)
```

Limit discovery responses with `io.LimitReader(resp.Body, 1<<20)`, require status 200, validate absolute HTTP(S) URLs, and apply non-empty advanced overrides after discovery.

- [ ] **Step 4: Add LDAP dialer abstraction and bind test**

```go
type LDAPConnection interface {
    Bind(username, password string) error
    Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error)
    Close() error
}

type LDAPDialer interface {
    Dial(ctx context.Context, rawURL string) (LDAPConnection, error)
}

func (a *LDAPAuthenticator) TestConnection(ctx context.Context) error
```

Production dialer uses `ldap.DialURL` with a bounded network timeout; test connection performs service-account bind and validates search configuration without a user password.

- [ ] **Step 5: Implement atomic runtime**

Use `atomic.Pointer[SSOSnapshot]`. Build the complete next snapshot before calling `Store`; `Load` uses repository data when present and normalized YAML fallback otherwise. `RefreshIfChanged` compares repository version before replacing.

- [ ] **Step 6: Run runtime and auth tests**

Run: `cd backend; go test ./internal/modules/identity/application/service ./internal/modules/identity/infrastructure/auth -count=1`

Expected: PASS, including timeout and rollback cases.

- [ ] **Step 7: Commit runtime management**

```powershell
git add backend/internal/modules/identity/infrastructure/auth/oidc.go backend/internal/modules/identity/infrastructure/auth/ldap.go backend/internal/modules/identity/application/service/sso_runtime.go backend/internal/modules/identity/application/service/sso_runtime_test.go
git commit -m "feat(identity): hot reload tested SSO providers"
```

---

### Task 4: SSO Admin Use Cases and HTTP API

**Files:**
- Create: `backend/internal/modules/identity/application/command/sso_config.go`
- Create: `backend/internal/modules/identity/interface/http/sso_admin_handler.go`
- Create: `backend/internal/modules/identity/interface/http/sso_admin_handler_test.go`
- Modify: `backend/internal/modules/identity/interface/http/sso_handler.go`
- Modify: `backend/internal/modules/identity/module.go`

**Interfaces:**
- Consumes: `SSORuntime`, `SSOConfigRepository`, `SecretBox`, `RequireSuperAdmin`.
- Produces:
  - `GET /api/v1/admin/sso`
  - `POST /api/v1/admin/sso/test`
  - `PUT /api/v1/admin/sso`

- [ ] **Step 1: Write failing handler tests**

Test the exact response properties:

```go
require.NotContains(t, recorder.Body.String(), "stored-client-secret")
require.NotContains(t, recorder.Body.String(), "encrypted-blob")
require.Contains(t, recorder.Body.String(), `"has_client_secret":true`)
```

Also test:
- non-super-admin receives 403;
- modified fingerprint cannot be enabled and returns `IDENTITY_SSO_TEST_REQUIRED`;
- password disabled with no enabled provider returns `IDENTITY_SSO_LOGIN_LOCKOUT`;
- version mismatch returns 409;
- successful PUT changes `/api/v1/auth/providers` without process restart.

- [ ] **Step 2: Run handler tests and confirm failure**

Run: `cd backend; go test ./internal/modules/identity/interface/http -run SSOAdmin -count=1`

Expected: FAIL because the admin handler and routes do not exist.

- [ ] **Step 3: Implement explicit DTOs and secret merge semantics**

The update DTO must include:

```go
type updateSSORequest struct {
    Version int64 `json:"version"`
    LoginUI loginUIDTO `json:"login_ui" binding:"required"`
    OIDC oidcConfigDTO `json:"oidc" binding:"required"`
    LDAP ldapConfigDTO `json:"ldap" binding:"required"`
}

type oidcConfigDTO struct {
    Enabled bool `json:"enabled"`
    Issuer string `json:"issuer"`
    ClientID string `json:"client_id"`
    ClientSecret string `json:"client_secret"`
    ClearClientSecret bool `json:"clear_client_secret"`
    RedirectURL string `json:"redirect_url"`
    Scopes string `json:"scopes"`
    AuthorizationEndpoint string `json:"authorization_endpoint"`
    TokenEndpoint string `json:"token_endpoint"`
    UserInfoEndpoint string `json:"userinfo_endpoint"`
    TestedFingerprint string `json:"tested_fingerprint"`
}
```

Create the corresponding LDAP DTO. Map domain errors to 400, test-required/lockout to 422, and version conflict to 409.

- [ ] **Step 4: Wire runtime-backed public login**

Replace fixed `oidc`, `ldap`, and `states` handler dependencies with `runtime *service.SSORuntime`. Each request obtains one snapshot and uses only that snapshot for the entire request.

- [ ] **Step 5: Wire migration, routes, fallback, polling, and audit**

In `identity.Module.Register`:
- create `SecretBox`;
- migrate `identity.sso_configs`;
- construct repository and runtime;
- call `runtime.Load(c.Context)`;
- register admin routes behind `authn.RequireAuth` and `RequireSuperAdmin`;
- start a context-bound 30-second `RefreshIfChanged` ticker.

Record only action, provider, success/failure and version through `platform/operationlog`; never pass the raw request body.

- [ ] **Step 6: Run identity tests**

Run: `cd backend; go test ./internal/modules/identity/... -count=1`

Expected: PASS.

- [ ] **Step 7: Commit backend API**

```powershell
git add backend/internal/modules/identity
git commit -m "feat(identity): expose secure SSO administration API"
```

---

### Task 5: Frontend Contracts, BFF, and SDK

**Files:**
- Modify: `fronted/packages/contracts/src/identity/index.ts`
- Modify: `fronted/packages/upstream/src/identity.ts`
- Modify: `fronted/packages/bff-modules/src/identity/sso.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/sso/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/admin/sso/test/route.ts`
- Modify: `fronted/packages/web-sdk/src/identity.ts`

**Interfaces:**
- Consumes the Task 4 JSON API.
- Produces:

```ts
export type SSOAdminConfigVM = z.infer<typeof SSOAdminConfigVMSchema>;
export type UpdateSSOConfigCommand = z.infer<typeof UpdateSSOConfigCommandSchema>;
export type TestSSOProviderCommand = z.infer<typeof TestSSOProviderCommandSchema>;
export type TestSSOProviderResultVM = z.infer<typeof TestSSOProviderResultVMSchema>;
```

- [ ] **Step 1: Add failing contract tests**

Create schema tests in `fronted/packages/contracts/src/identity/sso.test.ts` that reject a response containing `client_secret` or `bind_password`, accept `has_client_secret`, and validate provider literals.

- [ ] **Step 2: Run contract tests and confirm failure**

Run: `cd fronted; pnpm vitest run packages/contracts/src/identity/sso.test.ts`

Expected: FAIL because the schemas do not exist.

- [ ] **Step 3: Add schemas and typed client methods**

Add:

```ts
export function getSSOAdminConfig(): Promise<SSOAdminConfigVM>;
export function testSSOProvider(input: TestSSOProviderCommand): Promise<TestSSOProviderResultVM>;
export function updateSSOAdminConfig(input: UpdateSSOConfigCommand): Promise<SSOAdminConfigVM>;
```

The BFF routes forward only authenticated cookies through the existing BFF context; no secret is logged or cached.

- [ ] **Step 4: Run contract and type checks**

Run: `cd fronted; pnpm vitest run packages/contracts/src/identity/sso.test.ts; pnpm typecheck`

Expected: PASS.

- [ ] **Step 5: Commit the frontend data path**

```powershell
git add fronted/packages/contracts/src/identity fronted/packages/upstream/src/identity.ts fronted/packages/bff-modules/src/identity/sso.ts fronted/packages/web-sdk/src/identity.ts fronted/apps/web/app/api/bff/v1/admin/sso
git commit -m "feat(frontend): add SSO administration data path"
```

---

### Task 6: SSO Form State and Test Gates

**Files:**
- Create: `fronted/apps/web/src/modules/platform_admin/components/sso-config-state.ts`
- Test: `fronted/apps/web/src/modules/platform_admin/components/sso-config-state.test.ts`

**Interfaces:**
- Consumes: `SSOAdminConfigVM`.
- Produces:

```ts
export type SSOConfigDraft = {
  version: number;
  loginUI: {
    enabled: boolean;
    provider: "oidc" | "ldap";
    showPasswordLogin: boolean;
  };
  oidc: {
    enabled: boolean;
    issuer: string;
    clientId: string;
    clientSecret: string;
    hasClientSecret: boolean;
    clearClientSecret: boolean;
    redirectUrl: string;
    scopes: string;
    authorizationEndpoint: string;
    tokenEndpoint: string;
    userInfoEndpoint: string;
    testedFingerprint: string;
  };
  ldap: {
    enabled: boolean;
    url: string;
    bindDn: string;
    bindPassword: string;
    hasBindPassword: boolean;
    clearBindPassword: boolean;
    userSearchBase: string;
    userSearchFilter: string;
    emailAttribute: string;
    nameAttribute: string;
    testedFingerprint: string;
  };
};
export function createSSODraft(config: SSOAdminConfigVM): SSOConfigDraft;
export function updateOIDCDraft(draft: SSOConfigDraft, patch: Partial<SSOConfigDraft["oidc"]>): SSOConfigDraft;
export function updateLDAPDraft(draft: SSOConfigDraft, patch: Partial<SSOConfigDraft["ldap"]>): SSOConfigDraft;
export function canEnableProvider(draft: SSOConfigDraft, provider: "oidc" | "ldap"): boolean;
export function toUpdateCommand(draft: SSOConfigDraft): UpdateSSOConfigCommand;
```

- [ ] **Step 1: Write failing state tests**

```ts
it("invalidates only OIDC test state when an OIDC connection field changes", () => {
  const draft = testedDraft();
  const next = updateOIDCDraft(draft, { issuer: "https://new.example.com" });
  expect(next.oidc.testedFingerprint).toBe("");
  expect(next.ldap.testedFingerprint).toBe(draft.ldap.testedFingerprint);
});

it("preserves an existing secret when the input remains empty", () => {
  const command = toUpdateCommand(createSSODraft(configWithSecrets()));
  expect(command.oidc.client_secret).toBe("");
  expect(command.oidc.clear_client_secret).toBe(false);
});
```

- [ ] **Step 2: Run state tests and confirm failure**

Run: `cd fronted; pnpm vitest run apps/web/src/modules/platform_admin/components/sso-config-state.test.ts`

Expected: FAIL because the state module does not exist.

- [ ] **Step 3: Implement immutable state transitions**

Connection-field sets must be explicit constants. UI-only fields and enabled toggles do not accidentally invalidate the other provider. Clearing or replacing a secret invalidates that provider’s tested fingerprint.

- [ ] **Step 4: Run state tests**

Run: `cd fronted; pnpm vitest run apps/web/src/modules/platform_admin/components/sso-config-state.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit form state**

```powershell
git add fronted/apps/web/src/modules/platform_admin/components/sso-config-state.ts fronted/apps/web/src/modules/platform_admin/components/sso-config-state.test.ts
git commit -m "feat(frontend): model SSO configuration state"
```

---

### Task 7: Platform SSO Settings UI

**Files:**
- Create: `fronted/apps/web/src/modules/platform_admin/components/SSOSettingsSection.tsx`
- Modify: `fronted/apps/web/src/modules/platform_admin/components/PlatformSettingsPanel.tsx`
- Modify: `fronted/apps/web/app/globals.css`

**Interfaces:**
- Consumes Task 5 SDK methods and Task 6 state helpers.
- Produces one SSO settings section with login policy, OIDC, LDAP, test actions and one primary save action.

- [ ] **Step 1: Extract the old SSO section and load dedicated configuration**

`PlatformSettingsPanel` keeps tab selection but delegates all SSO rendering and persistence to:

```tsx
<SSOSettingsSection platformStyle={isPlatform} />
```

Remove SSO provider state from the generic `sysconfig` save payload; continue using `sysconfig.sso` only through the dedicated backend save operation.

- [ ] **Step 2: Implement three restrained sections**

`SSOSettingsSection` must render:
- login policy switches and default provider select;
- OIDC status, enabled switch, basic fields, `<details>` advanced endpoints and test button;
- LDAP status, enabled switch, connection/search fields and test button;
- one `btn btn-primary` save button;
- `InlineAlert` for page-level success/error;
- `role="status"` for loading and test progress.

Secret inputs use `type="password"` and adjacent text “已配置；留空将保留”。Clearing requires a secondary explicit button that toggles the relevant `clear_*` flag.

- [ ] **Step 3: Enforce interaction gates**

Disable provider enable controls until `canEnableProvider` returns true. Disable save during tests/saving. On connection-field changes, show “配置已变化，请重新测试”。If version conflict occurs, show “配置已被其他管理员更新，请重新加载后再保存” and a reload action.

- [ ] **Step 4: Add responsive token-based styles**

Add only `.platform-sso-*` selectors. Use existing variables such as `var(--surface-muted)`, `var(--border)`, `var(--radius-md)` and existing `stack-*` spacing; use a two-column grid above 900px and one column below.

- [ ] **Step 5: Run frontend verification**

Run: `cd fronted; pnpm vitest run apps/web/src/modules/platform_admin/components/sso-config-state.test.ts; pnpm typecheck`

Expected: PASS.

Run: `cd fronted; pnpm --filter @sparkops/web build`

Expected: Next.js build completes without type or route errors.

- [ ] **Step 6: Commit the UI**

```powershell
git add fronted/apps/web/src/modules/platform_admin/components/SSOSettingsSection.tsx fronted/apps/web/src/modules/platform_admin/components/PlatformSettingsPanel.tsx fronted/apps/web/app/globals.css
git commit -m "feat(frontend): add complete SSO settings UI"
```

---

### Task 8: End-to-End Verification and Runtime Restart

**Files:**
- Modify only files required by failures found in this task.

**Interfaces:**
- Verifies the complete feature against the design acceptance criteria.

- [ ] **Step 1: Run all identity tests with race detection**

Run: `cd backend; go test -race ./internal/modules/identity/... -count=1`

Expected: PASS with no race report.

- [ ] **Step 2: Run backend-wide tests**

Run: `cd backend; go test ./... -count=1`

Expected: PASS.

- [ ] **Step 3: Run frontend tests, typecheck and build**

Run: `cd fronted; pnpm vitest run; pnpm typecheck; pnpm --filter @sparkops/web build`

Expected: all commands PASS.

- [ ] **Step 4: Restart both development services**

Run: `.\scripts\dev-restart.ps1`

Expected: backend responds on `http://localhost:8080` and frontend responds on `http://localhost:3000`.

- [ ] **Step 5: Smoke-test the protected and public APIs**

Without a token:

```powershell
Invoke-WebRequest http://localhost:8080/api/v1/admin/sso -SkipHttpErrorCheck
```

Expected: 401.

Public status:

```powershell
Invoke-RestMethod http://localhost:8080/api/v1/auth/providers | ConvertTo-Json -Depth 8
```

Expected: response contains `oidc_enabled`, `ldap_enabled`, and `sso_ui`, with no secret fields.

- [ ] **Step 6: Review changed files for secret leakage**

Run:

```powershell
rg -n "client_secret|bind_password|secret_blob" backend/internal/modules/identity fronted/packages fronted/apps/web/src/modules/platform_admin
```

Expected: secret names appear only in write DTOs, encryption code, password inputs and tests; no read DTO serializes secret values and no logger receives them.

- [ ] **Step 7: Commit verification fixes if needed**

```powershell
git add backend/internal/modules/identity fronted/packages/contracts/src/identity fronted/packages/upstream/src/identity.ts fronted/packages/bff-modules/src/identity/sso.ts fronted/packages/web-sdk/src/identity.ts fronted/apps/web/app/api/bff/v1/admin/sso fronted/apps/web/src/modules/platform_admin/components fronted/apps/web/app/globals.css
git commit -m "test: verify runtime SSO configuration"
```
