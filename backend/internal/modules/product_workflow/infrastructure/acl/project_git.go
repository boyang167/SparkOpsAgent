package acl

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectGitLookup struct{ db *gorm.DB }

func NewProjectGitLookup(db *gorm.DB) *ProjectGitLookup {
	return &ProjectGitLookup{db: db}
}

func (l *ProjectGitLookup) LookupProjectGit(ctx context.Context, projectID kernel.ProjectID) (command.ProjectGit, error) {
	var row struct {
		GitURL    string  `gorm:"column:git_url"`
		ProductID *string `gorm:"column:product_id"`
	}
	err := l.db.WithContext(ctx).Table("workspace_app.projects").
		Select("git_url", "product_id").
		Where("id = ?", projectID.String()).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return command.ProjectGit{}, err
		}
		return command.ProjectGit{}, err
	}
	out := command.ProjectGit{GitURL: row.GitURL}
	if row.ProductID != nil && *row.ProductID != "" {
		parsed, err := kernel.ParseProductID(*row.ProductID)
		if err != nil {
			return command.ProjectGit{}, err
		}
		out.ProductID = parsed
	}
	return out, nil
}
