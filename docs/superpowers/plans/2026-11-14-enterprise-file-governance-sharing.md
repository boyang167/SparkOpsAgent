# Enterprise File Governance and Sharing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing workspace-only file center into organization, workspace, and personal file spaces with live-reference sharing, scoped principals, approval, audit, and the approved four-entry file UI.

**Architecture:** Keep Node, Revision, Blob, Replica, transfers, and the four top-level workbench tabs intact. Add typed Namespaces as ownership boundaries, generalize Resource Grant from user-only grantees to typed principals, expose organization-level filesystem APIs alongside compatibility workspace routes, and render the new spaces as navigation over existing explorer components.

**Tech Stack:** Go 1.x, Gin, GORM, PostgreSQL/SQLite migration fallback, React/Next.js, TypeScript, Zod, Vitest, Testing Library.

## Global Constraints

- Sharing is a live reference to the original Node; it never copies Blob or Replica data.
- Existing workspace Namespace and `/api/v1/workspaces/{workspaceId}/...` APIs remain compatible.
- Explicit Deny wins over every Allow.
- Tenant Owner does not automatically receive read access to USER Namespaces.
- Cross-Organization live sharing is rejected.
- Existing file/transfer/storage-governance/audit top-level tabs remain unchanged.
- File-space URL values are exactly `my`, `workspace`, `organization`, and `shared`.
- All new write APIs require `Idempotency-Key`.
- Each task uses TDD and ends with an independently reviewable commit.

---

## File Structure

### Backend additions

- `backend/migrations/000083_filesystem_namespace_types.{up,down}.sql` — typed Namespace schema and compatibility backfill.
- `backend/migrations/000084_filesystem_grant_principals.{up,down}.sql` — typed principals and soft revocation.
- `backend/migrations/000085_filesystem_share_requests.{up,down}.sql` — approval records.
- `backend/internal/modules/filesystem/application/command/ensure_namespace.go` — idempotent Organization/User Namespace creation.
- `backend/internal/modules/filesystem/application/authorization/principal_resolver.go` — user, workspace, organization, and role subject expansion.
- `backend/internal/modules/filesystem/application/query/list_namespaces.go` — accessible spaces.
- `backend/internal/modules/filesystem/application/query/shared_with_me.go` — deduplicated shared projection.
- `backend/internal/modules/filesystem/application/query/effective_access.go` — permission explanation.
- `backend/internal/modules/filesystem/interface/http/governance.go` — unified `/api/v1/filesystem` routes.
- `backend/internal/modules/filesystem/application/command/share_request.go` — request/approve/reject workflow.
- `backend/internal/modules/filesystem/application/command/adopt_file.go` — explicit copy into Organization Namespace.

### Frontend additions

- `fronted/apps/web/src/modules/filesystem/components/FileSpaceNav.tsx` — four space entries and current tree.
- `fronted/apps/web/src/modules/filesystem/components/SharedWithMePanel.tsx` — shared projection.
- `fronted/apps/web/src/modules/filesystem/components/ShareFileModal.tsx` — audience, actions, expiry, and approval result.
- `fronted/apps/web/src/modules/filesystem/components/file-space-view.ts` — pure labels, URL, and permission helpers.
- `fronted/apps/web/app/api/bff/v1/filesystem/**/route.ts` — organization-level BFF routes.

Existing large files remain orchestration points; new space/share behavior is placed in focused modules.

---

### Task 1: Typed Namespace database and domain model

**Files:**
- Create: `backend/migrations/000083_filesystem_namespace_types.up.sql`
- Create: `backend/migrations/000083_filesystem_namespace_types.down.sql`
- Create: `backend/migrations/filesystem_namespace_types_test.go`
- Modify: `backend/internal/modules/filesystem/domain/model/types.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/models.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/migrate.go`

**Interfaces:**
- Produces: `model.NamespaceType`, expanded `model.Namespace`.
- Preserves: existing `WorkspaceID uuid.UUID` callers by representing absent values as `uuid.Nil` in the domain and nullable pointers only in persistence rows.

- [ ] **Step 1: Write failing migration and domain tests**

```go
func TestFilesystemNamespaceTypesMigrationBackfillsWorkspaceRows(t *testing.T) {
    db := migrationTestDB(t, 82)
    workspaceID := uuid.New()
    insertLegacyNamespace(t, db, workspaceID)
    migrateTo(t, db, 83)

    var got struct {
        NamespaceType string
        WorkspaceID   *uuid.UUID
        OwnerUserID   *uuid.UUID
        PolicyVersion int64
    }
    require.NoError(t, db.Raw(`
        SELECT namespace_type, workspace_id, owner_user_id, policy_version
        FROM filesystem.namespaces WHERE workspace_id = ?
    `, workspaceID).Scan(&got).Error)
    require.Equal(t, "WORKSPACE", got.NamespaceType)
    require.Equal(t, workspaceID, *got.WorkspaceID)
    require.Nil(t, got.OwnerUserID)
    require.EqualValues(t, 1, got.PolicyVersion)
}
```

Add model tests that reject invalid Organization/User/Workspace field combinations.

- [ ] **Step 2: Run tests and verify failure**

Run:

```powershell
cd backend
go test ./migrations -run NamespaceTypes -count=1
go test ./internal/modules/filesystem/domain/model -run Namespace -count=1
```

Expected: FAIL because migration 83 and `NamespaceType` do not exist.

- [ ] **Step 3: Add Namespace types**

Add to `domain/model/types.go`:

```go
type NamespaceType string

const (
    NamespaceTypeOrganization NamespaceType = "ORGANIZATION"
    NamespaceTypeWorkspace    NamespaceType = "WORKSPACE"
    NamespaceTypeUser         NamespaceType = "USER"
)

type Namespace struct {
    ID             uuid.UUID
    NamespaceType  NamespaceType
    OrganizationID uuid.UUID
    WorkspaceID    uuid.UUID
    OwnerUserID    uuid.UUID
    RootNodeID     *uuid.UUID
    Status         string
    PolicyVersion  int64
}
```

Add `Validate()` enforcing the combinations from the approved spec.

- [ ] **Step 4: Add SQL migration and fallback schema**

Migration 83 must:

1. add `namespace_type`, `owner_user_id`, and `policy_version`;
2. backfill all existing rows as `WORKSPACE`;
3. make `workspace_id` nullable;
4. replace the old workspace-only uniqueness rule with partial unique indexes;
5. preserve the transfer composite FK requirements;
6. add type/ownership check constraints.

Mirror the schema in `models.go` and `migrate.go` for local SQLite/PostgreSQL fallback.

- [ ] **Step 5: Run tests and migration rollback check**

```powershell
go test ./migrations -run NamespaceTypes -count=1
go test ./internal/modules/filesystem/domain/model -run Namespace -count=1
go test ./internal/modules/filesystem/infrastructure/persistence -run Namespace -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add backend/migrations/000083* backend/migrations/filesystem_namespace_types_test.go backend/internal/modules/filesystem/domain/model/types.go backend/internal/modules/filesystem/infrastructure/persistence/models.go backend/internal/modules/filesystem/infrastructure/persistence/migrate.go
git commit -m "feat(filesystem): add typed namespaces"
```

---

### Task 2: Namespace repository and idempotent provisioning

**Files:**
- Create: `backend/internal/modules/filesystem/application/command/ensure_namespace.go`
- Create: `backend/internal/modules/filesystem/application/command/ensure_namespace_test.go`
- Modify: `backend/internal/modules/filesystem/domain/repository/filesystem_repository.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo_test.go`
- Modify: `backend/internal/modules/filesystem/application/command/commands.go`
- Modify: `backend/internal/modules/filesystem/application/subscriber/on_workspace_created.go`

**Interfaces:**
- Produces:

```go
GetNamespaceByOrganization(ctx context.Context, organizationID uuid.UUID) (*model.Namespace, error)
GetNamespaceByUser(ctx context.Context, organizationID, userID uuid.UUID) (*model.Namespace, error)
ListNamespacesForOrganization(ctx context.Context, organizationID uuid.UUID) ([]*model.Namespace, error)
```

- Produces:

```go
type EnsureNamespace struct {
    OrganizationID uuid.UUID
    WorkspaceID    uuid.UUID
    OwnerUserID    uuid.UUID
    NamespaceType  model.NamespaceType
    ActorID         uuid.UUID
}
```

- [ ] **Step 1: Write repository tests**

Cover:

- Organization lookup.
- User lookup scoped by Organization.
- Existing `GetNamespaceByWorkspace` behavior.
- Two concurrent ensure calls return one Namespace and one root.
- Quota account is created with the Namespace.

- [ ] **Step 2: Verify failures**

```powershell
cd backend
go test ./internal/modules/filesystem/infrastructure/persistence -run Namespace -count=1
go test ./internal/modules/filesystem/application/command -run EnsureNamespace -count=1
```

Expected: FAIL with missing methods and command.

- [ ] **Step 3: Implement repository methods**

Use Namespace type in every query:

```go
func (r *Repository) GetNamespaceByUser(
    ctx context.Context,
    organizationID, userID uuid.UUID,
) (*model.Namespace, error) {
    var row namespaceRow
    err := postgres.DB(ctx, r.db).
        Where("namespace_type = ? AND organization_id = ? AND owner_user_id = ?",
            model.NamespaceTypeUser, organizationID, userID).
        First(&row).Error
    return namespaceFromRow(&row, err)
}
```

Keep `GetNamespaceByWorkspace` as a compatibility method and require `namespace_type = WORKSPACE`.

- [ ] **Step 4: Implement idempotent ensure**

The handler must create Namespace, `.root` Node, Namespace root link, and quota account in one transaction. Resolve duplicate-key races by re-reading the unique Namespace.

Workspace-created subscription now invokes Ensure with `NamespaceTypeWorkspace`; do not change its event contract.

- [ ] **Step 5: Run tests**

```powershell
go test ./internal/modules/filesystem/application/command -run EnsureNamespace -count=1
go test ./internal/modules/filesystem/application/subscriber -count=1
go test ./internal/modules/filesystem/infrastructure/persistence -run Namespace -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/modules/filesystem
git commit -m "feat(filesystem): provision organization and user namespaces"
```

---

### Task 3: Principal-aware Resource Grants

**Files:**
- Create: `backend/migrations/000084_filesystem_grant_principals.up.sql`
- Create: `backend/migrations/000084_filesystem_grant_principals.down.sql`
- Create: `backend/migrations/filesystem_grant_principals_test.go`
- Modify: `backend/internal/modules/filesystem/domain/model/types.go`
- Modify: `backend/internal/modules/filesystem/domain/port/grant.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/models.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Modify: `backend/internal/modules/filesystem/application/authorization/grants.go`
- Modify: `backend/internal/modules/filesystem/application/authorization/grants_test.go`

**Interfaces:**
- Produces:

```go
type PrincipalType string

const (
    PrincipalUser           PrincipalType = "USER"
    PrincipalWorkspace      PrincipalType = "WORKSPACE"
    PrincipalOrganization   PrincipalType = "ORGANIZATION"
    PrincipalRole           PrincipalType = "ROLE"
    PrincipalServiceAccount PrincipalType = "SERVICE_ACCOUNT"
    PrincipalAgent          PrincipalType = "AGENT"
)
```

- `ResourceGrant` gains `OrganizationID`, `PrincipalType`, `PrincipalID`, `InheritToChildren`, `RevokedAt`, `PolicyVersion`.
- [ ] **Step 1: Write migration tests**

Assert old grants are backfilled as:

```text
principal_type = USER
principal_id = old grantee_id
organization_id = namespace organization
inherit_to_children = true
policy_version = 1
```

Assert uniqueness is `(node_id, principal_type, principal_id, permission, effect)` for active grants.

- [ ] **Step 2: Write GrantService tests**

Cover:

- one typed principal;
- same UUID with different principal types does not conflict;
- revoke sets `revoked_at` instead of deleting;
- cross-Organization principal rejected;
- existing USER Grant API compatibility.

- [ ] **Step 3: Run and verify failures**

```powershell
cd backend
go test ./migrations -run GrantPrincipal -count=1
go test ./internal/modules/filesystem/application/authorization -run Grant -count=1
```

- [ ] **Step 4: Implement migration, model, and repository**

Replace grantee-only repository filters with:

```go
type PrincipalRef struct {
    Type model.PrincipalType
    ID   uuid.UUID
}

ListResourceGrants(
    ctx context.Context,
    nodeIDs []uuid.UUID,
    principals []repository.PrincipalRef,
    permission model.ResourcePermission,
    at time.Time,
) ([]model.ResourceGrant, error)
```

Active means `revoked_at IS NULL` and `expires_at IS NULL OR expires_at > at`.

- [ ] **Step 5: Keep compatibility input**

If old `grantee_id` is supplied without `principal_type`, normalize to `USER`. New responses always include both typed fields.

- [ ] **Step 6: Run tests**

```powershell
go test ./migrations -run GrantPrincipal -count=1
go test ./internal/modules/filesystem/application/authorization -count=1
go test ./internal/modules/filesystem/infrastructure/persistence -run Grant -count=1
```

- [ ] **Step 7: Commit**

```powershell
git add backend/migrations/000084* backend/migrations/filesystem_grant_principals_test.go backend/internal/modules/filesystem
git commit -m "feat(filesystem): support typed grant principals"
```

---

### Task 4: Principal resolution and effective authorization

**Files:**
- Create: `backend/internal/modules/filesystem/application/authorization/principal_resolver.go`
- Create: `backend/internal/modules/filesystem/application/authorization/principal_resolver_test.go`
- Modify: `backend/internal/modules/filesystem/application/authorization/service.go`
- Modify: `backend/internal/modules/filesystem/application/authorization/grants.go`
- Modify: `backend/internal/modules/filesystem/domain/service/authorization.go`
- Modify: `backend/internal/modules/filesystem/domain/service/authorization_test.go`
- Modify: `backend/internal/platform/appctx/container.go`
- Modify: `backend/internal/modules/filesystem/module.go`

**Interfaces:**
- Consumes tenant membership and governance role checks through small ports; filesystem must not import their infrastructure packages.
- Produces:

```go
type PrincipalResolver interface {
    Resolve(ctx context.Context, input ResolvePrincipalsInput) ([]repository.PrincipalRef, error)
}
```

- [ ] **Step 1: Write resolver tests**

For a user in workspace A and Organization O, resolve:

```go
[]repository.PrincipalRef{
    {Type: model.PrincipalUser, ID: userID},
    {Type: model.PrincipalWorkspace, ID: workspaceA},
    {Type: model.PrincipalOrganization, ID: organizationO},
    {Type: model.PrincipalRole, ID: roleID},
}
```

Do not include Organization principal when Workspace membership is inactive.

- [ ] **Step 2: Write authorization matrix tests**

Cover:

- USER owner can access personal Namespace.
- Tenant Owner cannot read another user's USER Namespace without Grant.
- Organization Grant works only with valid Workspace context.
- Workspace Grant covers active members.
- direct Deny overrides inherited Organization Allow.
- nearest inherited Grant wins only when no Deny exists.
- cross-Organization request is denied.

- [ ] **Step 3: Run and verify failures**

```powershell
cd backend
go test ./internal/modules/filesystem/application/authorization -count=1
go test ./internal/modules/filesystem/domain/service -run Decide -count=1
```

- [ ] **Step 4: Implement resolver and authorization chain**

Remove the old unconditional `file:admin` resource bypass. Replace it with Namespace-aware authority:

```text
USER: owner only
WORKSPACE: Workspace file admin
ORGANIZATION: org:file:admin
```

Platform permission remains a capability gate; it is not a resource Allow.

- [ ] **Step 5: Add policy version to cache inputs**

Any authorization cache key must include:

```text
organizationID, workspaceID, actorID, nodeID, action, namespace.policyVersion
```

- [ ] **Step 6: Run tests**

```powershell
go test ./internal/modules/filesystem/application/authorization -count=1
go test ./internal/modules/filesystem/domain/service -run Decide -count=1
go test ./internal/modules/filesystem/interface/http -run Access -count=1
```

- [ ] **Step 7: Commit**

```powershell
git add backend/internal/modules/filesystem backend/internal/platform/appctx/container.go
git commit -m "feat(filesystem): resolve organization workspace and role grants"
```

---

### Task 5: Unified namespace, shared, and effective-access queries

**Files:**
- Create: `backend/internal/modules/filesystem/application/query/list_namespaces.go`
- Create: `backend/internal/modules/filesystem/application/query/list_namespaces_test.go`
- Create: `backend/internal/modules/filesystem/application/query/shared_with_me.go`
- Create: `backend/internal/modules/filesystem/application/query/shared_with_me_test.go`
- Create: `backend/internal/modules/filesystem/application/query/effective_access.go`
- Create: `backend/internal/modules/filesystem/application/query/effective_access_test.go`
- Create: `backend/internal/modules/filesystem/interface/http/governance.go`
- Create: `backend/internal/modules/filesystem/interface/http/governance_test.go`
- Modify: `backend/internal/modules/filesystem/module.go`
- Modify: `backend/api/openapi/filesystem.yaml`

**Interfaces:**
- Produces the approved routes:

```text
GET /api/v1/filesystem/namespaces
GET /api/v1/filesystem/namespaces/:namespaceId/children
GET /api/v1/filesystem/shared-with-me
GET /api/v1/filesystem/nodes/:nodeId/effective-access
```

- [ ] **Step 1: Write query tests**

`ListNamespaces` must return:

- current user's USER Namespace;
- current WORKSPACE Namespace when membership is active;
- Organization Namespace summary when at least one node is visible;
- capabilities per Namespace.

`SharedWithMe` must deduplicate `node_id`, union allowed actions, preserve Deny, and return source Grant IDs.

- [ ] **Step 2: Write HTTP tests**

Test:

- missing Organization context → 400;
- unauthorized Namespace → 404, not 403, to avoid discovery;
- invalid Workspace membership → 403;
- cursor and `q` forwarding;
- old workspace routes still pass.

- [ ] **Step 3: Run failing tests**

```powershell
cd backend
go test ./internal/modules/filesystem/application/query -run "Namespace|Shared|Effective" -count=1
go test ./internal/modules/filesystem/interface/http -run Governance -count=1
```

- [ ] **Step 4: Implement handlers and OpenAPI**

Mount unified routes separately from `/workspaces/:wsId`, but reuse existing entitlement and authentication middleware. Parse Workspace context as optional for `my`, required for workspace and Organization audience resolution.

- [ ] **Step 5: Run tests**

```powershell
go test ./internal/modules/filesystem/application/query -count=1
go test ./internal/modules/filesystem/interface/http -count=1
```

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/modules/filesystem backend/api/openapi/filesystem.yaml
git commit -m "feat(filesystem): expose governed file spaces"
```

---

### Task 6: Grant write API and sharing approval workflow

**Files:**
- Create: `backend/migrations/000085_filesystem_share_requests.up.sql`
- Create: `backend/migrations/000085_filesystem_share_requests.down.sql`
- Create: `backend/migrations/filesystem_share_requests_test.go`
- Create: `backend/internal/modules/filesystem/domain/model/share_request.go`
- Create: `backend/internal/modules/filesystem/application/command/share_request.go`
- Create: `backend/internal/modules/filesystem/application/command/share_request_test.go`
- Modify: `backend/internal/modules/filesystem/application/command/grant_resource.go`
- Modify: `backend/internal/modules/filesystem/interface/http/access.go`
- Modify: `backend/internal/modules/filesystem/interface/http/governance.go`
- Modify: `backend/api/openapi/filesystem.yaml`

**Interfaces:**
- Grant request body:

```json
{
  "principal_type": "WORKSPACE",
  "principal_ids": ["uuid"],
  "actions": ["READ", "DOWNLOAD"],
  "inherit_to_children": true,
  "expires_at": null
}
```

- Approval routes exactly match the design spec.

- [ ] **Step 1: Write policy matrix tests**

Verify:

- selected user read share is immediate;
- personal → whole Workspace creates `PENDING` unless actor is Workspace admin;
- personal/workspace → Organization creates `PENDING`;
- Organization `DELETE`, `SHARE`, `ADMIN` is rejected;
- Restricted → Organization is rejected;
- approval creates Grants and Outbox in one transaction;
- rejection creates no Grant.

- [ ] **Step 2: Run failures**

```powershell
cd backend
go test ./migrations -run ShareRequest -count=1
go test ./internal/modules/filesystem/application/command -run ShareRequest -count=1
```

- [ ] **Step 3: Implement persistence and commands**

Use states:

```text
PENDING, APPROVED, REJECTED, CANCELLED
```

Approving a non-pending request must return a conflict and remain idempotent for the same command key.

- [ ] **Step 4: Upgrade Grant HTTP compatibility**

Continue accepting legacy:

```json
{"grantee_id":"uuid","permission":"READ","effect":"ALLOW"}
```

Normalize it to one USER principal Grant.

- [ ] **Step 5: Run tests**

```powershell
go test ./migrations -run ShareRequest -count=1
go test ./internal/modules/filesystem/application/command -run "Grant|ShareRequest" -count=1
go test ./internal/modules/filesystem/interface/http -run "Grant|ShareRequest" -count=1
```

- [ ] **Step 6: Commit**

```powershell
git add backend/migrations/000085* backend/migrations/filesystem_share_requests_test.go backend/internal/modules/filesystem backend/api/openapi/filesystem.yaml
git commit -m "feat(filesystem): add governed sharing approvals"
```

---

### Task 7: Namespace-aware quota and Placement

**Files:**
- Modify: `backend/internal/modules/filesystem/domain/service/placement.go`
- Modify: `backend/internal/modules/filesystem/domain/service/placement_test.go`
- Modify: `backend/internal/modules/filesystem/application/command/create_transfer.go`
- Modify: `backend/internal/modules/filesystem/application/command/create_transfer_test.go`
- Modify: `backend/internal/modules/filesystem/infrastructure/persistence/filesystem_repo.go`
- Modify: `backend/internal/modules/filesystem/module.go`

**Interfaces:**
- Placement precedence:

```text
USER: USER > ORGANIZATION > SYSTEM
WORKSPACE: WORKSPACE > ORGANIZATION > SYSTEM
ORGANIZATION: ORGANIZATION > SYSTEM
```

- [ ] **Step 1: Write Placement tests**

Cover all three chains, disabled bindings, wrong Organization, and fallback. Assert a shared file read never invokes Placement or creates a Replica.

- [ ] **Step 2: Write transfer test**

Create transfer against each Namespace type and assert selected Provider and Pool are persisted in the session request hash.

- [ ] **Step 3: Run failures**

```powershell
cd backend
go test ./internal/modules/filesystem/domain/service -run Placement -count=1
go test ./internal/modules/filesystem/application/command -run CreateTransfer -count=1
```

- [ ] **Step 4: Wire Placement in `module.go`**

Replace the `nil` placer passed to `NewCreateTransferHandler` with the repository-backed Placement engine. Keep direct-local gateway validation for selected LOCAL Providers; fail explicitly when the selected Provider cannot be served by the configured gateway.

- [ ] **Step 5: Run tests**

```powershell
go test ./internal/modules/filesystem/domain/service -run Placement -count=1
go test ./internal/modules/filesystem/application/command -run CreateTransfer -count=1
go test ./internal/modules/filesystem/... -count=1
```

- [ ] **Step 6: Commit**

```powershell
git add backend/internal/modules/filesystem
git commit -m "feat(filesystem): place uploads by namespace scope"
```

---

### Task 8: Frontend contracts and transport path

**Files:**
- Modify: `fronted/packages/contracts/src/filesystem/index.ts`
- Modify: `fronted/packages/upstream/src/filesystem.ts`
- Modify: `fronted/packages/upstream/src/filesystem.test.ts`
- Modify: `fronted/packages/bff-modules/src/filesystem/index.ts`
- Modify: `fronted/packages/bff-modules/src/filesystem/index.test.ts`
- Modify: `fronted/packages/web-sdk/src/filesystem.ts`
- Modify: `fronted/packages/web-sdk/src/filesystem.test.ts`
- Create: `fronted/apps/web/app/api/bff/v1/filesystem/namespaces/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/filesystem/namespaces/[namespaceId]/children/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/filesystem/shared-with-me/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/filesystem/nodes/[nodeId]/effective-access/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/filesystem/nodes/[nodeId]/grants/route.ts`
- Create: `fronted/apps/web/app/api/bff/v1/filesystem/nodes/[nodeId]/share-requests/route.ts`

**Interfaces:**
- Produces Zod schemas and inferred types for Namespace summary, Grant, effective access, shared item, and share request.

- [ ] **Step 1: Write contract/transport tests**

Use fixtures:

```ts
const namespace = {
  id: "00000000-0000-4000-8000-000000000001",
  namespace_type: "USER",
  organization_id: "00000000-0000-4000-8000-000000000002",
  workspace_id: null,
  owner_user_id: "00000000-0000-4000-8000-000000000003",
  root_node_id: "00000000-0000-4000-8000-000000000004",
  status: "ACTIVE",
  capabilities: ["READ", "WRITE", "SHARE"],
};
```

Assert query forwarding and `Idempotency-Key` on writes.

- [ ] **Step 2: Run failures**

```powershell
cd fronted
pnpm vitest run packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts
```

- [ ] **Step 3: Implement schemas and clients**

Use one top-level BFF path `/api/bff/v1/filesystem`. Do not nest personal or Organization spaces under a Workspace URL.

- [ ] **Step 4: Run tests and typecheck**

```powershell
pnpm vitest run packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts
pnpm typecheck
```

- [ ] **Step 5: Commit**

```powershell
git add fronted/packages fronted/apps/web/app/api/bff/v1/filesystem
git commit -m "feat(filesystem-ui): add governed space clients"
```

---

### Task 9: URL state, loaders, and four-space navigation

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/file-space-view.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/file-space-view.test.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileSpaceNav.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/FileSpaceNav.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/workbench-state.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/workbench-state.test.ts`
- Modify: `fronted/apps/web/app/(platform)/files/page-state.ts`
- Modify: `fronted/apps/web/app/(platform)/files/page-state.test.ts`
- Modify: `fronted/apps/web/src/server/loaders/filesystem.ts`
- Modify: `fronted/apps/web/src/server/loaders/filesystem.test.ts`
- Modify: `fronted/apps/web/app/(platform)/files/page.tsx`

**Interfaces:**
- Produces:

```ts
export type FileSpace = "my" | "workspace" | "organization" | "shared";
```

- URL:

```text
/files?space=my
/files?space=workspace
/files?space=organization
/files?space=shared&source=organization
```

- [ ] **Step 1: Write state tests**

Assert:

- invalid space normalizes to `workspace`;
- changing space removes `parent`, `cursor`, and `node`;
- changing space preserves `view`;
- `shared` does not accept `parent`;
- current Organization/Workspace labels are present.

- [ ] **Step 2: Run failures**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/workbench-state.test.ts apps/web/app/(platform)/files/page-state.test.ts apps/web/src/server/loaders/filesystem.test.ts
```

- [ ] **Step 3: Implement loaders and navigation**

`FilesPage` loads accessible Namespace summaries first, resolves the requested space, then loads children only for real Namespaces. `shared` loads the shared projection instead.

`FileSpaceNav` renders:

```text
我的文件
当前工作区
租户文件
与我共享
```

Below the entries, render the existing `DirectoryTree` for the active real Namespace.

- [ ] **Step 4: Run component and accessibility tests**

```powershell
pnpm vitest run apps/web/src/modules/filesystem/components/FileSpaceNav.test.tsx apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx
pnpm typecheck
```

- [ ] **Step 5: Commit**

```powershell
git add fronted/apps/web/app/(platform)/files fronted/apps/web/src/server/loaders/filesystem* fronted/apps/web/src/modules/filesystem/components
git commit -m "feat(filesystem-ui): navigate personal workspace and tenant spaces"
```

---

### Task 10: Multi-Namespace explorer and shared projection

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/SharedWithMePanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/SharedWithMePanel.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/DirectoryTree.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/DirectoryTree.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/directory-dnd.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/directory-dnd.test.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileExplorerPanel.tsx`
- Modify: `fronted/apps/web/app/globals.css`

**Interfaces:**
- `DirectoryTree` receives explicit `namespaceId` and `rootNode`.
- Drag payload includes `namespace_id`; cross-Namespace drops are rejected.

- [ ] **Step 1: Write explorer tests**

Cover:

- switching space resets tree selection;
- two Namespaces with equal Node IDs do not share cache state;
- cross-Namespace drag returns `跨空间请使用复制、转存或采纳`;
- shared view groups by `ORGANIZATION`, `USER`, `WORKSPACE`, `AGENT`;
- shared item opens existing detail drawer by original Node ID.

- [ ] **Step 2: Run failures**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/FilesystemWorkbench.test.tsx apps/web/src/modules/filesystem/components/DirectoryTree.test.tsx apps/web/src/modules/filesystem/components/directory-dnd.test.ts apps/web/src/modules/filesystem/components/SharedWithMePanel.test.tsx
```

- [ ] **Step 3: Implement multi-Namespace keys**

Use:

```ts
const scopeKey = `${space}:${workspaceId}:${namespace.id}:${namespace.root_node_id ?? ""}`;
```

Transfer queues remain scoped to target Namespace. Shared view does not expose upload, create directory, move, or drag targets.

- [ ] **Step 4: Implement shared projection**

Render source, owner/sharer, permissions, expiry, and original path. Avoid pretending it is a normal directory tree.

- [ ] **Step 5: Run tests**

```powershell
pnpm vitest run apps/web/src/modules/filesystem
pnpm typecheck
```

- [ ] **Step 6: Commit**

```powershell
git add fronted/apps/web/src/modules/filesystem fronted/apps/web/app/globals.css
git commit -m "feat(filesystem-ui): browse governed spaces and shared files"
```

---

### Task 11: Share modal and effective-access drawer

**Files:**
- Create: `fronted/apps/web/src/modules/filesystem/components/ShareFileModal.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/ShareFileModal.test.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/share-file-form.ts`
- Create: `fronted/apps/web/src/modules/filesystem/components/share-file-form.test.ts`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileActionMenu.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileDetailDrawer.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FileDetailDrawer.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`

**Interfaces:**
- Two-step share flow: audience, then permissions/expiry.
- Submit result is either `ACTIVE` or `PENDING_APPROVAL`.

- [ ] **Step 1: Write form tests**

Assert:

- default actions are READ and DOWNLOAD;
- Organization audience cannot select DELETE, SHARE, or ADMIN;
- Restricted item cannot target Organization;
- directory inheritance defaults true;
- exact impact summary uses selected Workspace/member count;
- submit button is `立即共享` or `提交审批`.

- [ ] **Step 2: Write drawer tests**

Add a `共享与权限` tab showing:

- effective actions;
- direct vs inherited source;
- audience;
- expiry;
- pending approval;
- explicit Deny.

- [ ] **Step 3: Run failures**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem/components/share-file-form.test.ts apps/web/src/modules/filesystem/components/ShareFileModal.test.tsx apps/web/src/modules/filesystem/components/FileDetailDrawer.test.tsx
```

- [ ] **Step 4: Implement with existing Modal and Drawer**

Do not use browser confirm dialogs. Hide Share when effective access lacks `SHARE`. In shared view, allow reshare only when the source Grant includes `SHARE`.

- [ ] **Step 5: Run tests**

```powershell
pnpm vitest run apps/web/src/modules/filesystem
pnpm typecheck
```

- [ ] **Step 6: Commit**

```powershell
git add fronted/apps/web/src/modules/filesystem
git commit -m "feat(filesystem-ui): share files with governed audiences"
```

---

### Task 12: Tenant governance permissions, approval UI, audit, and adoption

**Files:**
- Modify: `backend/internal/platform/authz/org_permissions.go`
- Modify: `backend/internal/modules/governance/application/command/role_commands.go`
- Modify: `backend/internal/modules/governance/application/command/bootstrap_test.go`
- Create: `backend/internal/modules/filesystem/application/command/adopt_file.go`
- Create: `backend/internal/modules/filesystem/application/command/adopt_file_test.go`
- Modify: `backend/internal/modules/filesystem/interface/http/governance.go`
- Create: `fronted/apps/web/src/modules/filesystem/components/ShareApprovalPanel.tsx`
- Create: `fronted/apps/web/src/modules/filesystem/components/ShareApprovalPanel.test.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/AuditWorkspace.tsx`
- Modify: `fronted/apps/web/src/modules/filesystem/components/workbench-permissions.ts`

**Interfaces:**
- Adds `org:file:admin`.
- Adoption creates a new Organization Namespace Node and lineage relation; it is not sharing.

- [ ] **Step 1: Write permission/bootstrap tests**

Assert Tenant Owner receives `org:file:admin`; ordinary Workspace Owner does not.

- [ ] **Step 2: Write adoption tests**

Cover:

- source READ and Organization Namespace WRITE required;
- same-Organization policy may reuse Blob;
- quota increments in target Namespace;
- source permissions are not copied;
- lineage and audit events are recorded;
- cross-Organization source rejected.

- [ ] **Step 3: Write approval UI tests**

Tenant file administrators see pending count and approve/reject controls. Ordinary users do not receive the management entry.

- [ ] **Step 4: Run failures**

```powershell
cd backend
go test ./internal/modules/governance/application/command -run Bootstrap -count=1
go test ./internal/modules/filesystem/application/command -run Adopt -count=1
cd ../fronted
pnpm vitest run apps/web/src/modules/filesystem/components/ShareApprovalPanel.test.tsx apps/web/src/modules/filesystem/components/workbench-permissions.test.ts
```

- [ ] **Step 5: Implement and connect audit filters**

Audit events include Namespace, Node, principal before/after, reason, trace ID, and action for share request, approval, rejection, revoke, sensitive read, and adoption.

- [ ] **Step 6: Run tests**

```powershell
cd backend
go test ./internal/modules/governance/application/command -run Bootstrap -count=1
go test ./internal/modules/filesystem/... -count=1
cd ../fronted
pnpm vitest run apps/web/src/modules/filesystem
pnpm typecheck
```

- [ ] **Step 7: Commit**

```powershell
git add backend fronted
git commit -m "feat(filesystem): complete tenant file governance"
```

---

### Task 13: Migration, security, accessibility, and rollout verification

**Files:**
- Modify: `backend/tests/filesystem/security/tenant_isolation_test.go`
- Modify: `docs/verification/unified-file-infrastructure-v1.0.md`
- Create: `docs/verification/enterprise-file-governance-sharing.md`
- Modify: `fronted/apps/web/src/modules/filesystem/components/FilesystemWorkbench.accessibility.test.tsx`

**Interfaces:**
- Produces release evidence and rollback instructions; no new product behavior.

- [ ] **Step 1: Add end-to-end authorization matrix tests**

Minimum matrix:

```text
Tenant Owner → Organization Namespace: allow admin
Tenant Owner → another user's private Namespace: deny without Grant
Workspace member → public workspace top-level directory: allow read
Workspace member → admin-only directory: deny
Workspace A member → Organization directory shared to A: allow
Workspace B member → same directory shared only to A: deny
Personal owner → own private Node: allow
Revoked recipient → source Node: deny immediately
Cross-Organization recipient → source Node: deny
```

- [ ] **Step 2: Run backend full verification**

```powershell
cd backend
go test ./internal/modules/filesystem/... -count=1
go test ./migrations -count=1
$env:REQUIRE_INTEGRATION="1"
go test -tags=integration ./tests/filesystem/security/... -count=1
```

Expected: PASS.

- [ ] **Step 3: Run frontend full verification**

```powershell
cd fronted
pnpm vitest run apps/web/src/modules/filesystem
pnpm vitest run packages/upstream/src/filesystem.test.ts packages/bff-modules/src/filesystem/index.test.ts packages/web-sdk/src/filesystem.test.ts
pnpm typecheck
```

Expected: PASS.

- [ ] **Step 4: Verify migration and compatibility manually**

Using a copy of existing data:

1. migrate 82 → 85;
2. verify every old Namespace is `WORKSPACE`;
3. verify every old Grant is `USER`;
4. open existing `/files` route and confirm old files remain visible;
5. call old workspace Grant API and confirm compatibility;
6. revoke a new typed Grant and confirm immediate denial;
7. roll down 85 → 82 only in disposable verification database.

- [ ] **Step 5: Record release evidence**

Document exact command output, migration row counts, security matrix, known operational requirements, and rollback boundary in `docs/verification/enterprise-file-governance-sharing.md`.

- [ ] **Step 6: Commit**

```powershell
git add backend/tests fronted/apps/web/src/modules/filesystem docs/verification
git commit -m "test(filesystem): verify enterprise file governance"
```

---

## Delivery Sequence

1. Tasks 1–2: typed Namespace foundation, deployable without exposing new UI.
2. Tasks 3–5: principal sharing read path while old Grant writes remain compatible.
3. Tasks 6–7: approval and actual Namespace-aware storage placement.
4. Tasks 8–10: contracts, transport, and four-space file center.
5. Tasks 11–12: sharing UX, Tenant Owner governance, approval, adoption.
6. Task 13: security, migration, accessibility, and release evidence.

Do not expose Organization or User space navigation until Tasks 1–5 and Task 8 are deployed together. Keep the old workspace route available for at least one release after the unified API is enabled.
