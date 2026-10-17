package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type projectMemberModel struct {
	ProjectID string `gorm:"primaryKey;type:uuid"`
	MemberID  string `gorm:"primaryKey;type:uuid"`
	Role      string
	JoinedAt  time.Time
	postgres.AuditColumns
}

func (projectMemberModel) TableName() string { return "workspace_app.project_members" }

type ProjectMemberRepository struct{ db *gorm.DB }

func NewProjectMemberRepository(db *gorm.DB) repository.ProjectMemberRepository {
	return &ProjectMemberRepository{db: db}
}

func (r *ProjectMemberRepository) Save(ctx context.Context, member *model.ProjectMember) error {
	now := time.Now().UTC()
	row := projectMemberModel{
		ProjectID:    member.ProjectID.String(),
		MemberID:     member.MemberID.String(),
		Role:         member.Role,
		JoinedAt:     member.JoinedAt,
		AuditColumns: postgres.AuditColumns{CreatedAt: now, UpdatedAt: now},
	}
	return postgres.DB(ctx, r.db).Save(&row).Error
}

func (r *ProjectMemberRepository) Find(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID) (*model.ProjectMember, error) {
	var row projectMemberModel
	err := r.db.WithContext(ctx).
		Where(postgres.UUIDEq("project_id")+" AND "+postgres.UUIDEq("member_id"), projectID.String(), memberID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrProjectMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	pid, _ := kernel.ParseProjectID(row.ProjectID)
	mid, _ := kernel.ParseMemberID(row.MemberID)
	return &model.ProjectMember{ProjectID: pid, MemberID: mid, Role: row.Role, JoinedAt: row.JoinedAt}, nil
}

func (r *ProjectMemberRepository) ListByProject(ctx context.Context, projectID kernel.ProjectID) ([]repository.ProjectMemberRecord, error) {
	type row struct {
		MemberID    string
		Email       string
		DisplayName string
		Role        string
		JoinedAt    time.Time
	}
	var rows []row
	err := r.db.WithContext(ctx).Raw(`
		SELECT pm.member_id, m.email, m.display_name, pm.role, pm.joined_at
		FROM workspace_app.project_members pm
		JOIN identity.members m ON m.id = pm.member_id
		WHERE pm.project_id::uuid = ?::uuid AND pm.deleted_at IS NULL AND m.deleted_at IS NULL
		ORDER BY pm.role ASC, m.display_name ASC
	`, projectID.String()).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]repository.ProjectMemberRecord, 0, len(rows))
	for _, item := range rows {
		mid, parseErr := kernel.ParseMemberID(item.MemberID)
		if parseErr != nil {
			continue
		}
		out = append(out, repository.ProjectMemberRecord{
			MemberID: mid, Email: item.Email, DisplayName: item.DisplayName,
			Role: item.Role, JoinedAt: item.JoinedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (r *ProjectMemberRepository) ListProjectIDsForMember(ctx context.Context, wsID kernel.WorkspaceID, memberID kernel.MemberID) ([]kernel.ProjectID, error) {
	type row struct{ ID string }
	var rows []row
	err := r.db.WithContext(ctx).Raw(`
		SELECT p.id
		FROM workspace_app.project_members pm
		JOIN workspace_app.projects p ON p.id = pm.project_id
		WHERE pm.member_id::uuid = ?::uuid AND p.workspace_id::uuid = ?::uuid AND p.status = 'active'
		  AND pm.deleted_at IS NULL AND p.deleted_at IS NULL
	`, memberID.String(), wsID.String()).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]kernel.ProjectID, 0, len(rows))
	for _, item := range rows {
		id, parseErr := kernel.ParseProjectID(item.ID)
		if parseErr != nil {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

func (r *ProjectMemberRepository) UpdateRole(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID, role string) error {
	res := r.db.WithContext(ctx).Model(&projectMemberModel{}).
		Where(postgres.UUIDEq("project_id")+" AND "+postgres.UUIDEq("member_id"), projectID.String(), memberID.String()).
		Updates(map[string]interface{}{"role": role, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrProjectMemberNotFound
	}
	return nil
}

func (r *ProjectMemberRepository) Remove(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID) error {
	res := r.db.WithContext(ctx).
		Where(postgres.UUIDEq("project_id")+" AND "+postgres.UUIDEq("member_id"), projectID.String(), memberID.String()).
		Delete(&projectMemberModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrProjectMemberNotFound
	}
	return nil
}

func (r *ProjectMemberRepository) HasAccess(ctx context.Context, projectID kernel.ProjectID, memberID kernel.MemberID) (bool, string, error) {
	var row projectMemberModel
	err := r.db.WithContext(ctx).
		Where(postgres.UUIDEq("project_id")+" AND "+postgres.UUIDEq("member_id"), projectID.String(), memberID.String()).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, row.Role, nil
}

var _ repository.ProjectMemberRepository = (*ProjectMemberRepository)(nil)
