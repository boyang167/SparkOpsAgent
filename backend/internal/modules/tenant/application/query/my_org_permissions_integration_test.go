package query_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/application/query"
	tenantpersistence "github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestPersistedOrganizationOwnerRoleBootstrapsFileAdminWhenPostgresConfigured(t *testing.T) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		t.Skip("DATABASE_DSN is not set")
	}
	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE SCHEMA IF NOT EXISTS tenant").Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE IF NOT EXISTS tenant.organization_members (
			org_id UUID NOT NULL,
			member_id UUID NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (org_id, member_id)
		)
	`).Error)
	orgID := kernel.OrgID{UUID: uuid.New()}
	ownerID := kernel.MemberID{UUID: uuid.New()}
	adminID := kernel.MemberID{UUID: uuid.New()}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM tenant.organization_members WHERE org_id = ?", orgID.UUID).Error
	})
	require.NoError(t, db.Exec(
		"INSERT INTO tenant.organization_members (org_id, member_id, role) VALUES (?, ?, 'owner'), (?, ?, 'admin')",
		orgID.UUID, ownerID.UUID, orgID.UUID, adminID.UUID,
	).Error)
	handler := query.NewMyOrgPermissionsHandler(
		tenantpersistence.NewOrganizationRepository(tenantpersistence.NewTenantStore(db)),
	)

	ownerPermissions, err := handler.Handle(context.Background(), orgID, ownerID)
	require.NoError(t, err)
	require.Contains(t, ownerPermissions, authz.PermOrgFileAdmin)
	adminPermissions, err := handler.Handle(context.Background(), orgID, adminID)
	require.NoError(t, err)
	require.NotContains(t, adminPermissions, authz.PermOrgFileAdmin)

	ownerPermissions, err = handler.Handle(context.Background(), orgID, ownerID)
	require.NoError(t, err)
	require.Equal(t, 1, countPermission(ownerPermissions, authz.PermOrgFileAdmin))
}

func countPermission(permissions []string, target string) int {
	count := 0
	for _, permission := range permissions {
		if permission == target {
			count++
		}
	}
	return count
}
