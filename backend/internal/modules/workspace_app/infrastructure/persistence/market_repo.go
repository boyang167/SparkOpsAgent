package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/repository"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/postgres"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type dockerfileModel struct {
	ID             string `gorm:"column:id;primaryKey"`
	WorkspaceID    string
	Name           string
	Language       string
	Description    string `gorm:"type:text"`
	Detail         string `gorm:"type:text"`
	Dockerfile     string `gorm:"type:text"`
	Version        string
	FullPath       string
	Sha256         string
	DownloadCount  int
	Score          int
	Status         string
	CreatedBy      string
	postgres.AuditColumns
}

func (dockerfileModel) TableName() string { return "workspace_app.dockerfiles" }

type MarketRepository struct{ db *gorm.DB }

func NewMarketRepository(db *gorm.DB) repository.MarketRepository {
	return &MarketRepository{db: db}
}

func (r *MarketRepository) SaveDockerfile(ctx context.Context, df *model.Dockerfile) error {
	row := dockerfileModel{
		ID: df.ID.String(), WorkspaceID: df.WorkspaceID.String(),
		Name: df.Name.String(), Language: df.Language, Description: df.Description,
		Detail: df.Detail, Dockerfile: df.Dockerfile, Version: df.Version,
		FullPath: df.FullPath, Sha256: df.Sha256, DownloadCount: df.DownloadCount,
		Score: df.Score, Status: string(df.Status), CreatedBy: df.CreatedBy.String(),
		AuditColumns: postgres.AuditColumns{CreatedAt: df.CreatedAt, UpdatedAt: time.Now().UTC()},
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *MarketRepository) ListDockerfilesByWorkspace(ctx context.Context, wsID kernel.WorkspaceID) ([]*model.Dockerfile, error) {
	var rows []dockerfileModel
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND status = ?", wsID.String(), string(model.DockerfileStatusActive)).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.Dockerfile, 0, len(rows))
	for _, row := range rows {
		df, err := rowToDockerfile(row)
		if err != nil {
			return nil, err
		}
		out = append(out, df)
	}
	return out, nil
}

func rowToDockerfile(row dockerfileModel) (*model.Dockerfile, error) {
	name, err := model.NewDockerfileName(row.Name)
	if err != nil {
		return nil, err
	}
	id, _ := kernel.ParseDockerfileID(row.ID)
	wsID, _ := kernel.ParseWorkspaceID(row.WorkspaceID)
	createdBy, _ := kernel.ParseMemberID(row.CreatedBy)
	return &model.Dockerfile{
		ID: id, WorkspaceID: wsID, Name: name, Language: row.Language,
		Description: row.Description, Detail: row.Detail, Dockerfile: row.Dockerfile,
		Version: row.Version, FullPath: row.FullPath, Sha256: row.Sha256,
		DownloadCount: row.DownloadCount, Score: row.Score,
		Status: model.DockerfileStatus(row.Status), CreatedBy: createdBy, CreatedAt: row.CreatedAt,
	}, nil
}

var _ repository.MarketRepository = (*MarketRepository)(nil)
