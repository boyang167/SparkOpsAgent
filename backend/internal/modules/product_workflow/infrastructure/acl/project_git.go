package acl

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
	gitpkg "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/git"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/crypto"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type ProjectGitLookup struct {
	db  *gorm.DB
	box *crypto.SecretBox
}

func NewProjectGitLookup(db *gorm.DB, box *crypto.SecretBox) *ProjectGitLookup {
	return &ProjectGitLookup{db: db, box: box}
}

func (l *ProjectGitLookup) LookupProjectGit(ctx context.Context, projectID kernel.ProjectID) (command.ProjectGit, error) {
	var row struct {
		GitURL      string  `gorm:"column:git_url"`
		ProductID   *string `gorm:"column:product_id"`
		ServiceURL  string  `gorm:"column:service_url"`
		Credentials string  `gorm:"column:credential_blob"`
	}
	err := l.db.WithContext(ctx).Table("workspace_app.projects AS p").
		Select("p.git_url, p.product_id, s.service_url, s.credential_blob").
		Joins("LEFT JOIN workspace_app.secrets AS s ON s.id = p.secret_id").
		Where("p.id = ?", projectID.String()).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return command.ProjectGit{}, err
		}
		return command.ProjectGit{}, err
	}
	out := command.ProjectGit{GitURL: row.GitURL, ServiceURL: row.ServiceURL}
	if row.ProductID != nil && *row.ProductID != "" {
		parsed, err := kernel.ParseProductID(*row.ProductID)
		if err != nil {
			return command.ProjectGit{}, err
		}
		out.ProductID = parsed
	}
	out.Token = l.tokenFromBlob(row.Credentials)
	return out, nil
}

func (l *ProjectGitLookup) tokenFromBlob(blob string) string {
	blob = strings.TrimSpace(blob)
	if blob == "" || l.box == nil {
		return ""
	}
	plain, err := l.box.Decrypt(blob)
	if err != nil || plain == "" {
		return ""
	}
	var creds model.SecretCredentials
	if err := json.Unmarshal([]byte(plain), &creds); err != nil {
		return ""
	}
	return gitpkg.ResolveAuthToken(creds)
}
