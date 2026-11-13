package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
)

func (r *Repository) ListStorageProviders(ctx context.Context, organizationID uuid.UUID) ([]*model.StorageProvider, error) {
	var rows []storageProviderRow
	err := postgres.DB(ctx, r.db).
		Where("organization_id = ?", organizationID).
		Order("name ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, wrapPersistence(err)
	}
	out := make([]*model.StorageProvider, len(rows))
	for i := range rows {
		out[i] = storageProviderFromRow(rows[i])
	}
	return out, nil
}

func (r *Repository) SaveStoragePool(ctx context.Context, pool *model.StoragePool, organizationID uuid.UUID) error {
	if pool == nil || organizationID == uuid.Nil {
		return domain.ErrPersistence
	}
	now := time.Now().UTC()
	snapshot := pool.Snapshot()
	return wrapPersistence(postgres.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		row := storagePoolRow{
			ID: snapshot.ID, OrganizationID: organizationID, Name: snapshot.Name,
			Status: snapshot.Status, PlacementPolicyVersion: snapshot.PlacementPolicyVersion,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "status", "placement_policy_version", "updated_at"}),
		}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Where("pool_id = ?", snapshot.ID).Delete(&storagePoolMemberRow{}).Error; err != nil {
			return err
		}
		for _, member := range snapshot.Members {
			memberRow := storagePoolMemberRow{
				ID: uuid.New(), PoolID: snapshot.ID, ProviderID: member.ProviderID,
				Priority: member.Priority, Mode: member.Mode, CreatedAt: now,
			}
			if err := tx.Create(&memberRow).Error; err != nil {
				return err
			}
		}
		return nil
	}))
}

func (r *Repository) GetStoragePool(ctx context.Context, id uuid.UUID) (*model.StoragePool, error) {
	var row storagePoolRow
	if err := postgres.DB(ctx, r.db).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrStorageProviderNotFound
		}
		return nil, wrapPersistence(err)
	}
	var memberRows []storagePoolMemberRow
	if err := postgres.DB(ctx, r.db).Where("pool_id = ?", id).Order("priority ASC, provider_id ASC").Find(&memberRows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	snapshot := model.StoragePoolSnapshot{
		ID: row.ID, Name: row.Name, PlacementPolicyVersion: row.PlacementPolicyVersion, Status: row.Status,
		Members: make([]model.StoragePoolMember, len(memberRows)),
	}
	for i, member := range memberRows {
		snapshot.Members[i] = model.StoragePoolMember{
			ProviderID: member.ProviderID, Mode: member.Mode, Priority: member.Priority,
		}
	}
	return model.RestoreStoragePool(snapshot), nil
}

func (r *Repository) ListStoragePools(ctx context.Context, organizationID uuid.UUID) ([]model.StoragePoolSnapshot, error) {
	var rows []storagePoolRow
	if err := postgres.DB(ctx, r.db).Where("organization_id = ?", organizationID).Order("name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	if len(rows) == 0 {
		return []model.StoragePoolSnapshot{}, nil
	}
	poolIDs := make([]uuid.UUID, len(rows))
	for i := range rows {
		poolIDs[i] = rows[i].ID
	}
	var memberRows []storagePoolMemberRow
	if err := postgres.DB(ctx, r.db).Where("pool_id IN ?", poolIDs).Order("pool_id ASC, priority ASC, provider_id ASC").Find(&memberRows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	membersByPool := map[uuid.UUID][]model.StoragePoolMember{}
	for _, member := range memberRows {
		membersByPool[member.PoolID] = append(membersByPool[member.PoolID], model.StoragePoolMember{
			ProviderID: member.ProviderID, Mode: member.Mode, Priority: member.Priority,
		})
	}
	out := make([]model.StoragePoolSnapshot, len(rows))
	for i, row := range rows {
		out[i] = model.StoragePoolSnapshot{
			ID: row.ID, Name: row.Name, PlacementPolicyVersion: row.PlacementPolicyVersion,
			Status: row.Status, Members: membersByPool[row.ID],
		}
		if out[i].Members == nil {
			out[i].Members = []model.StoragePoolMember{}
		}
	}
	return out, nil
}

func (r *Repository) SaveStorageBinding(ctx context.Context, binding *model.StorageBinding) error {
	if binding == nil {
		return domain.ErrPersistence
	}
	now := time.Now().UTC()
	row := storageBindingRow{
		ID: binding.ID, OrganizationID: binding.OrganizationID, ScopeType: binding.ScopeType,
		ScopeID: binding.ScopeID, PoolID: binding.PoolID, Priority: binding.Priority, CreatedAt: now,
	}
	return wrapPersistence(postgres.DB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"scope_type", "scope_id", "pool_id", "priority"}),
	}).Create(&row).Error)
}

func (r *Repository) ListStorageBindings(ctx context.Context, organizationID uuid.UUID) ([]model.StorageBinding, error) {
	var rows []storageBindingRow
	if err := postgres.DB(ctx, r.db).Where("organization_id = ?", organizationID).Order("priority ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	out := make([]model.StorageBinding, len(rows))
	for i, row := range rows {
		out[i] = model.StorageBinding{
			ID: row.ID, OrganizationID: row.OrganizationID, ScopeType: row.ScopeType,
			ScopeID: row.ScopeID, PoolID: row.PoolID, Priority: row.Priority,
		}
	}
	return out, nil
}

func (r *Repository) SaveLifecyclePolicy(ctx context.Context, policy *model.LifecyclePolicy) error {
	if policy == nil {
		return domain.ErrPersistence
	}
	now := time.Now().UTC()
	rules := jsonObject(policy.Rules)
	if rules == nil {
		rules = jsonObject{}
	}
	row := lifecyclePolicyRow{
		ID: policy.ID, OrganizationID: policy.OrganizationID, Name: policy.Name, Version: policy.Version,
		Rules: rules, Status: policy.Status, CreatedAt: now, UpdatedAt: now,
	}
	return wrapPersistence(postgres.DB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "version", "rules", "status", "updated_at"}),
	}).Create(&row).Error)
}

func (r *Repository) ListLifecyclePolicies(ctx context.Context, organizationID uuid.UUID) ([]model.LifecyclePolicy, error) {
	var rows []lifecyclePolicyRow
	if err := postgres.DB(ctx, r.db).Where("organization_id = ?", organizationID).Order("name ASC, version DESC, id ASC").Find(&rows).Error; err != nil {
		return nil, wrapPersistence(err)
	}
	out := make([]model.LifecyclePolicy, len(rows))
	for i, row := range rows {
		rules := map[string]any(row.Rules)
		if rules == nil {
			rules = map[string]any{}
		}
		out[i] = model.LifecyclePolicy{
			ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
			Version: row.Version, Rules: rules, Status: row.Status,
		}
	}
	return out, nil
}

func (r *Repository) GetLifecyclePolicy(ctx context.Context, policyID uuid.UUID, version int64) (*model.LifecyclePolicy, error) {
	var row lifecyclePolicyRow
	query := postgres.DB(ctx, r.db).Where("id = ?", policyID)
	if version > 0 {
		query = query.Where("version = ?", version)
	}
	if err := query.Order("version DESC").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrInvalidLifecyclePolicy
		}
		return nil, wrapPersistence(err)
	}
	rules := map[string]any(row.Rules)
	if rules == nil {
		rules = map[string]any{}
	}
	return &model.LifecyclePolicy{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
		Version: row.Version, Rules: rules, Status: row.Status,
	}, nil
}
