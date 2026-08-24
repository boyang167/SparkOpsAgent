package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/tenant/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authz"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type TenantStore struct {
	db *gorm.DB
}

func NewTenantStore(db *gorm.DB) *TenantStore {
	return &TenantStore{db: db}
}

func AutoMigrate(db *gorm.DB) error {
	if db.Dialector.Name() == "postgres" {
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS tenant").Error; err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(&organizationModel{}, &organizationMemberModel{}, &workspaceModel{}, &workspaceMemberModel{}); err != nil {
		return err
	}
	return ensureOrganizationSlugPartialUniqueIndex(db)
}

func ensureOrganizationSlugPartialUniqueIndex(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	// Slug must be unique among active rows only so soft-deleted orgs can release their slug.
	stmts := []string{
		`ALTER TABLE tenant.organizations DROP CONSTRAINT IF EXISTS organizations_slug_key`,
		`DROP INDEX IF EXISTS tenant.organizations_slug_key`,
		`DROP INDEX IF EXISTS tenant.idx_tenant_organizations_slug`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_organizations_slug_active
		 ON tenant.organizations (slug) WHERE deleted_at IS NULL`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

type organizationMemberModel struct {
	OrgID    string `gorm:"primaryKey"`
	MemberID string `gorm:"primaryKey"`
	Role     string
	JoinedAt time.Time
	postgres.AuditColumns
}

func (organizationMemberModel) TableName() string { return "tenant.organization_members" }

type organizationModel struct {
	ID            string `gorm:"column:id;primaryKey"`
	Name          string
	Slug          string
	Status        string
	MaxWorkspaces int
	MaxAgents     int
	MaxProjects   int
	TokenBudget   int64
	postgres.AuditColumns
}

func (organizationModel) TableName() string { return "tenant.organizations" }

type workspaceModel struct {
	ID              string `gorm:"column:id;primaryKey"`
	OrgID           string
	Name            string
	Slug            string
	Status          string
	MaxAgents       int
	MaxProjects     int
	LegacyNamespace string `gorm:"column:legacy_namespace;size:128"`
	postgres.AuditColumns
}

func (workspaceModel) TableName() string { return "tenant.workspaces" }

type workspaceMemberModel struct {
	WorkspaceID string `gorm:"primaryKey"`
	MemberID    string `gorm:"primaryKey"`
	Role        string
	JoinedAt    time.Time
	postgres.AuditColumns
}

func (workspaceMemberModel) TableName() string { return "tenant.workspace_members" }

func (s *TenantStore) SaveOrganization(ctx context.Context, org *model.Organization) error {
	row := organizationModel{
		ID: org.ID.String(), Name: org.Name, Slug: org.Slug,
		Status: org.Status,
		MaxWorkspaces: org.Quota.MaxWorkspaces, MaxAgents: org.Quota.MaxAgents,
		MaxProjects: org.Quota.MaxProjects, TokenBudget: org.Quota.TokenBudget,
		AuditColumns: postgres.AuditColumns{CreatedAt: org.CreatedAt, UpdatedAt: org.CreatedAt},
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *TenantStore) FindOrganizationByID(ctx context.Context, id kernel.OrgID) (*model.Organization, error) {
	var row organizationModel
	err := s.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrOrganizationNotFound
	}
	if err != nil {
		return nil, err
	}
	orgID, _ := kernel.ParseOrgID(row.ID)
	return &model.Organization{
		ID: orgID, Name: row.Name, Slug: row.Slug,
		Status: row.Status, Quota: rowToOrgQuota(row), CreatedAt: row.CreatedAt,
	}, nil
}

func (s *TenantStore) ListAllOrganizations(ctx context.Context) ([]*model.Organization, error) {
	return s.listOrganizations(ctx, false)
}

func (s *TenantStore) ListAllOrganizationsIncludingDeleted(ctx context.Context) ([]*model.Organization, error) {
	return s.listOrganizations(ctx, true)
}

func (s *TenantStore) listOrganizations(ctx context.Context, includeDeleted bool) ([]*model.Organization, error) {
	var rows []organizationModel
	q := s.db.WithContext(ctx).Order("name ASC")
	if includeDeleted {
		q = q.Unscoped()
	} else {
		q = q.Where("deleted_at IS NULL")
	}
	err := q.Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Organization, 0, len(rows))
	for _, row := range rows {
		orgID, parseErr := kernel.ParseOrgID(row.ID)
		if parseErr != nil {
			continue
		}
		status := row.Status
		if includeDeleted && row.DeletedAt.Valid {
			status = "deleted"
		}
		out = append(out, &model.Organization{
			ID: orgID, Name: row.Name, Slug: row.Slug,
			Status: status, Quota: rowToOrgQuota(row), CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func (s *TenantStore) UpdateOrganizationStatus(ctx context.Context, orgID kernel.OrgID, status string) error {
	res := s.db.WithContext(ctx).Model(&organizationModel{}).
		Where("id = ? AND deleted_at IS NULL", orgID.String()).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrganizationNotFound
	}
	return nil
}

func (s *TenantStore) SoftDeleteOrganization(ctx context.Context, orgID kernel.OrgID) error {
	res := s.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", orgID.String()).
		Delete(&organizationModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrganizationNotFound
	}
	return nil
}

func (s *TenantStore) TransferOrganizationOwner(ctx context.Context, orgID kernel.OrgID, fromID, toID kernel.MemberID) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var fromRow organizationMemberModel
		if err := tx.Where("org_id = ? AND member_id = ?", orgID.String(), fromID.String()).
			First(&fromRow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrOrgMemberNotFound
			}
			return err
		}
		if fromRow.Role != domain.OrgRoleOwner {
			return domain.ErrOnlyOwnerCanTransfer
		}
		var toRow organizationMemberModel
		if err := tx.Where("org_id = ? AND member_id = ?", orgID.String(), toID.String()).
			First(&toRow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrNewOwnerNotMember
			}
			return err
		}
		if fromID == toID {
			return domain.ErrCannotTransferToSelf
		}
		now := time.Now().UTC()
		if err := tx.Model(&organizationMemberModel{}).
			Where("org_id = ? AND member_id = ?", orgID.String(), fromID.String()).
			Updates(map[string]interface{}{"role": domain.OrgRoleAdmin, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&organizationMemberModel{}).
			Where("org_id = ? AND member_id = ?", orgID.String(), toID.String()).
			Updates(map[string]interface{}{"role": domain.OrgRoleOwner, "updated_at": now}).Error
	})
}

func (s *TenantStore) ExistsOrganizationBySlug(ctx context.Context, slug string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&organizationModel{}).
		Where("slug = ? AND deleted_at IS NULL", slug).
		Count(&count).Error
	return count > 0, err
}

func (s *TenantStore) AddOrganizationMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, role string) error {
	var existing int64
	err := s.db.WithContext(ctx).Model(&organizationMemberModel{}).
		Where("org_id = ? AND member_id = ?", orgID.String(), memberID.String()).
		Count(&existing).Error
	if err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	now := time.Now().UTC()
	row := organizationMemberModel{
		OrgID:    orgID.String(),
		MemberID: memberID.String(),
		Role:     role,
		JoinedAt: now,
		AuditColumns: postgres.AuditColumns{CreatedAt: now, UpdatedAt: now},
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *TenantStore) IsWorkspaceMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&workspaceMemberModel{}).
		Where("workspace_id = ? AND member_id = ?", workspaceID.String(), memberID.String()).
		Count(&count).Error
	return count > 0, err
}

func (s *TenantStore) CanCreateWorkspaceInOrg(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (bool, error) {
	role, err := s.GetOrganizationMemberRole(ctx, orgID, memberID)
	if err != nil {
		return false, err
	}
	return authz.OrgRoleHasPermission(role, authz.PermOrgWorkspaceCreate), nil
}

func (s *TenantStore) IsOrganizationMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&organizationMemberModel{}).
		Where("org_id = ? AND member_id = ?", orgID.String(), memberID.String()).
		Count(&count).Error
	return count > 0, err
}

func (s *TenantStore) GetOrganizationMemberRole(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (string, error) {
	var row organizationMemberModel
	err := s.db.WithContext(ctx).
		Where("org_id = ? AND member_id = ?", orgID.String(), memberID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", domain.ErrOrgMemberNotFound
	}
	if err != nil {
		return "", err
	}
	return row.Role, nil
}

func (s *TenantStore) ListOrganizationMembers(ctx context.Context, orgID kernel.OrgID) ([]repository.OrgMember, error) {
	type row struct {
		MemberID    string
		Email       string
		DisplayName string
		Role        string
		JoinedAt    time.Time
	}
	var rows []row
	err := s.db.WithContext(ctx).Raw(`
		SELECT om.member_id, m.email, m.display_name, om.role, om.joined_at
		FROM tenant.organization_members om
		JOIN identity.members m ON m.id = om.member_id
		WHERE om.org_id = ? AND om.deleted_at IS NULL AND m.deleted_at IS NULL
		ORDER BY om.role ASC, m.display_name ASC
	`, orgID.String()).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]repository.OrgMember, 0, len(rows))
	for _, r := range rows {
		memberID, parseErr := kernel.ParseMemberID(r.MemberID)
		if parseErr != nil {
			continue
		}
		out = append(out, repository.OrgMember{
			MemberID: memberID, Email: r.Email, DisplayName: r.DisplayName,
			Role: r.Role, JoinedAt: r.JoinedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *TenantStore) UpdateOrganizationMemberRole(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, role string) error {
	res := s.db.WithContext(ctx).Model(&organizationMemberModel{}).
		Where("org_id = ? AND member_id = ?", orgID.String(), memberID.String()).
		Updates(map[string]interface{}{"role": role, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrgMemberNotFound
	}
	return nil
}

func (s *TenantStore) RemoveOrganizationMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) error {
	res := s.db.WithContext(ctx).
		Where("org_id = ? AND member_id = ?", orgID.String(), memberID.String()).
		Delete(&organizationMemberModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrgMemberNotFound
	}
	return nil
}

func (s *TenantStore) CountOrganizationMembersByRole(ctx context.Context, orgID kernel.OrgID, role string) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&organizationMemberModel{}).
		Where("org_id = ? AND role = ?", orgID.String(), role).
		Count(&count).Error
	return int(count), err
}

func (s *TenantStore) ListOrganizationsByMember(ctx context.Context, memberID kernel.MemberID) ([]*model.Organization, error) {
	type orgRow struct {
		ID   string
		Name string
		Slug string
	}
	var rows []orgRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT o.id, o.name, o.slug
		FROM tenant.organizations o
		WHERE o.id IN (
			SELECT om.org_id FROM tenant.organization_members om WHERE om.member_id = ?
			UNION
			SELECT w.org_id
			FROM tenant.workspaces w
			JOIN tenant.workspace_members wm ON wm.workspace_id = w.id
			WHERE wm.member_id = ?
		)
		AND o.deleted_at IS NULL
		ORDER BY o.name ASC
	`, memberID.String(), memberID.String()).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Organization, 0, len(rows))
	for _, row := range rows {
		orgID, parseErr := kernel.ParseOrgID(row.ID)
		if parseErr != nil {
			continue
		}
		out = append(out, &model.Organization{
			ID: orgID, Name: row.Name, Slug: row.Slug,
		})
	}
	return out, nil
}

func (s *TenantStore) SaveWorkspace(ctx context.Context, ws *model.Workspace) error {
	row := workspaceModel{
		ID: ws.ID.String(), OrgID: ws.OrgID.String(), Name: ws.Name, Slug: ws.Slug,
		Status: ws.Status, MaxAgents: ws.Quota.MaxAgents, MaxProjects: ws.Quota.MaxProjects,
		AuditColumns: postgres.AuditColumns{CreatedAt: ws.CreatedAt, UpdatedAt: ws.CreatedAt},
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func (s *TenantStore) FindWorkspaceByID(ctx context.Context, id kernel.WorkspaceID) (*model.Workspace, error) {
	var row workspaceModel
	err := s.db.WithContext(ctx).Where("id = ?", id.String()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	return rowToWorkspace(row), nil
}

func (s *TenantStore) ListAllWorkspacesWithMembership(ctx context.Context, memberID kernel.MemberID) ([]repository.WorkspaceWithMembership, error) {
	var rows []struct {
		ID          string
		OrgID       string
		Name        string
		Slug        string
		Status      string
		MaxAgents   int
		MaxProjects int
		CreatedAt   time.Time
		IsMember    bool
	}
	err := s.db.WithContext(ctx).Raw(`
		SELECT w.id, w.org_id, w.name, w.slug, w.status, w.max_agents, w.max_projects,
		       w.created_at,
		       (wm.member_id IS NOT NULL) AS is_member
		FROM tenant.workspaces w
		INNER JOIN tenant.organizations o ON o.id = w.org_id AND o.deleted_at IS NULL
		LEFT JOIN tenant.workspace_members wm
		  ON wm.workspace_id = w.id AND wm.member_id = ?
		WHERE w.deleted_at IS NULL
		ORDER BY w.name ASC
	`, memberID.String()).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]repository.WorkspaceWithMembership, 0, len(rows))
	for _, row := range rows {
		wsID, parseErr := kernel.ParseWorkspaceID(row.ID)
		if parseErr != nil {
			continue
		}
		orgID, parseErr := kernel.ParseOrgID(row.OrgID)
		if parseErr != nil {
			continue
		}
		out = append(out, repository.WorkspaceWithMembership{
			Workspace: &model.Workspace{
				ID: wsID, OrgID: orgID,
				Name: row.Name, Slug: row.Slug, Status: row.Status,
				Quota: model.WorkspaceQuota{MaxAgents: row.MaxAgents, MaxProjects: row.MaxProjects},
				CreatedAt: row.CreatedAt,
			},
			IsMember: row.IsMember,
		})
	}
	return out, nil
}

func (s *TenantStore) ListWorkspacesByMember(ctx context.Context, memberID kernel.MemberID) ([]*model.Workspace, error) {
	var rows []workspaceModel
	err := s.db.WithContext(ctx).
		Table("tenant.workspaces").
		Select("tenant.workspaces.*").
		Joins("JOIN tenant.workspace_members wm ON wm.workspace_id = tenant.workspaces.id").
		Joins("JOIN tenant.organizations o ON o.id = tenant.workspaces.org_id AND o.deleted_at IS NULL").
		Where("wm.member_id = ?", memberID.String()).
		Where("tenant.workspaces.deleted_at IS NULL").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*model.Workspace, 0, len(rows))
	for _, row := range rows {
		out = append(out, rowToWorkspace(row))
	}
	return out, nil
}

func (s *TenantStore) ExistsWorkspaceByOrgSlug(ctx context.Context, orgID kernel.OrgID, slug string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&workspaceModel{}).
		Where("org_id = ? AND slug = ?", orgID.String(), slug).Count(&count).Error
	return count > 0, err
}

func (s *TenantStore) AddWorkspaceMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, role string) error {
	now := time.Now().UTC()
	row := workspaceMemberModel{
		WorkspaceID: workspaceID.String(),
		MemberID:    memberID.String(),
		Role:        role,
		JoinedAt:    now,
		AuditColumns: postgres.AuditColumns{CreatedAt: now, UpdatedAt: now},
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

func rowToWorkspace(row workspaceModel) *model.Workspace {
	wsID, _ := kernel.ParseWorkspaceID(row.ID)
	orgID, _ := kernel.ParseOrgID(row.OrgID)
	return &model.Workspace{
		ID: wsID, OrgID: orgID,
		Name: row.Name, Slug: row.Slug, Status: row.Status,
		Quota: model.WorkspaceQuota{MaxAgents: row.MaxAgents, MaxProjects: row.MaxProjects},
		CreatedAt: row.CreatedAt,
	}
}

var (
	_ repository.OrganizationRepository = (*orgRepo)(nil)
	_ repository.WorkspaceRepository    = (*wsRepo)(nil)
)

type orgRepo struct{ store *TenantStore }
type wsRepo struct{ store *TenantStore }

func NewOrganizationRepository(store *TenantStore) repository.OrganizationRepository {
	return &orgRepo{store: store}
}
func NewWorkspaceRepository(store *TenantStore) repository.WorkspaceRepository {
	return &wsRepo{store: store}
}

func (r *orgRepo) Save(ctx context.Context, org *model.Organization) error {
	return r.store.SaveOrganization(ctx, org)
}
func (r *orgRepo) FindByID(ctx context.Context, id kernel.OrgID) (*model.Organization, error) {
	return r.store.FindOrganizationByID(ctx, id)
}
func (r *orgRepo) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	return r.store.ExistsOrganizationBySlug(ctx, slug)
}
func (r *orgRepo) ListByMember(ctx context.Context, memberID kernel.MemberID) ([]*model.Organization, error) {
	return r.store.ListOrganizationsByMember(ctx, memberID)
}
func (r *orgRepo) ListAll(ctx context.Context) ([]*model.Organization, error) {
	return r.store.ListAllOrganizations(ctx)
}
func (r *orgRepo) ListAllIncludingDeleted(ctx context.Context) ([]*model.Organization, error) {
	return r.store.ListAllOrganizationsIncludingDeleted(ctx)
}
func (r *orgRepo) UpdateStatus(ctx context.Context, orgID kernel.OrgID, status string) error {
	return r.store.UpdateOrganizationStatus(ctx, orgID, status)
}
func (r *orgRepo) SoftDelete(ctx context.Context, orgID kernel.OrgID) error {
	return r.store.SoftDeleteOrganization(ctx, orgID)
}
func (r *orgRepo) AddMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, role string) error {
	return r.store.AddOrganizationMember(ctx, orgID, memberID, role)
}
func (r *orgRepo) IsMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (bool, error) {
	return r.store.IsOrganizationMember(ctx, orgID, memberID)
}
func (r *orgRepo) GetMemberRole(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) (string, error) {
	return r.store.GetOrganizationMemberRole(ctx, orgID, memberID)
}
func (r *orgRepo) ListMembers(ctx context.Context, orgID kernel.OrgID) ([]repository.OrgMember, error) {
	return r.store.ListOrganizationMembers(ctx, orgID)
}
func (r *orgRepo) UpdateMemberRole(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID, role string) error {
	return r.store.UpdateOrganizationMemberRole(ctx, orgID, memberID, role)
}
func (r *orgRepo) RemoveMember(ctx context.Context, orgID kernel.OrgID, memberID kernel.MemberID) error {
	return r.store.RemoveOrganizationMember(ctx, orgID, memberID)
}
func (r *orgRepo) CountMembersByRole(ctx context.Context, orgID kernel.OrgID, role string) (int, error) {
	return r.store.CountOrganizationMembersByRole(ctx, orgID, role)
}
func (s *TenantStore) UpdateOrganizationQuota(ctx context.Context, orgID kernel.OrgID, quota model.OrgQuota) error {
	res := s.db.WithContext(ctx).Model(&organizationModel{}).
		Where("id = ?", orgID.String()).
		Updates(map[string]interface{}{
			"max_workspaces": quota.MaxWorkspaces,
			"max_agents":     quota.MaxAgents,
			"max_projects":   quota.MaxProjects,
			"token_budget":   quota.TokenBudget,
			"updated_at":     time.Now().UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrOrganizationNotFound
	}
	return nil
}

func (s *TenantStore) CountWorkspacesByOrg(ctx context.Context, orgID kernel.OrgID) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&workspaceModel{}).
		Where("org_id = ? AND deleted_at IS NULL", orgID.String()).
		Count(&count).Error
	return int(count), err
}

func (s *TenantStore) GetOrganizationUsage(ctx context.Context, orgID kernel.OrgID) (*repository.OrgUsage, error) {
	wsCount, err := s.CountWorkspacesByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	var agentCount int64
	err = s.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM agent.agents a
		JOIN workspace_app.projects p ON p.id = a.project_id
		JOIN tenant.workspaces w ON w.id = p.workspace_id
		WHERE w.org_id = ? AND a.status != 'archived' AND a.deleted_at IS NULL
	`, orgID.String()).Scan(&agentCount).Error
	if err != nil {
		return nil, err
	}
	var projectCount int64
	err = s.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM workspace_app.projects p
		JOIN tenant.workspaces w ON w.id = p.workspace_id
		WHERE w.org_id = ? AND p.status = 'active' AND p.deleted_at IS NULL
	`, orgID.String()).Scan(&projectCount).Error
	if err != nil {
		return nil, err
	}
	return &repository.OrgUsage{
		Workspaces: wsCount,
		Agents:     int(agentCount),
		Projects:   int(projectCount),
	}, nil
}

func rowToOrgQuota(row organizationModel) model.OrgQuota {
	q := model.OrgQuota{
		MaxWorkspaces: row.MaxWorkspaces,
		MaxAgents:     row.MaxAgents,
		MaxProjects:   row.MaxProjects,
		TokenBudget:   row.TokenBudget,
	}
	if q.MaxWorkspaces <= 0 {
		q.MaxWorkspaces = model.DefaultOrgQuota().MaxWorkspaces
	}
	if q.MaxAgents <= 0 {
		q.MaxAgents = model.DefaultOrgQuota().MaxAgents
	}
	if q.MaxProjects <= 0 {
		q.MaxProjects = model.DefaultOrgQuota().MaxProjects
	}
	return q
}

func (r *orgRepo) UpdateQuota(ctx context.Context, orgID kernel.OrgID, quota model.OrgQuota) error {
	return r.store.UpdateOrganizationQuota(ctx, orgID, quota)
}
func (r *orgRepo) GetUsage(ctx context.Context, orgID kernel.OrgID) (*repository.OrgUsage, error) {
	return r.store.GetOrganizationUsage(ctx, orgID)
}
func (r *orgRepo) TransferOwner(ctx context.Context, orgID kernel.OrgID, fromID, toID kernel.MemberID) error {
	return r.store.TransferOrganizationOwner(ctx, orgID, fromID, toID)
}

func (r *wsRepo) Save(ctx context.Context, ws *model.Workspace) error {
	return r.store.SaveWorkspace(ctx, ws)
}
func (r *wsRepo) FindByID(ctx context.Context, id kernel.WorkspaceID) (*model.Workspace, error) {
	return r.store.FindWorkspaceByID(ctx, id)
}
func (r *wsRepo) ListByMember(ctx context.Context, memberID kernel.MemberID) ([]*model.Workspace, error) {
	return r.store.ListWorkspacesByMember(ctx, memberID)
}
func (r *wsRepo) ListAllWithMembership(ctx context.Context, memberID kernel.MemberID) ([]repository.WorkspaceWithMembership, error) {
	return r.store.ListAllWorkspacesWithMembership(ctx, memberID)
}
func (r *wsRepo) ExistsByOrgSlug(ctx context.Context, orgID kernel.OrgID, slug string) (bool, error) {
	return r.store.ExistsWorkspaceByOrgSlug(ctx, orgID, slug)
}
func (r *wsRepo) AddMember(ctx context.Context, workspaceID kernel.WorkspaceID, memberID kernel.MemberID, role string) error {
	return r.store.AddWorkspaceMember(ctx, workspaceID, memberID, role)
}
