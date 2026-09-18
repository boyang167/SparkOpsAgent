package acl

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/engine"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type RegistryResolver struct{ db *gorm.DB }

func NewRegistryResolver(db *gorm.DB) *RegistryResolver {
	return &RegistryResolver{db: db}
}

func (r *RegistryResolver) Resolve(ctx context.Context, projectID kernel.ProjectID, registrySecretID string) (string, error) {
	secretID := strings.TrimSpace(registrySecretID)
	if secretID == "" {
		return "", nil
	}
	var project struct {
		WorkspaceID string
		Name        string
		Namespace   string
	}
	err := r.db.WithContext(ctx).Table("workspace_app.projects").
		Select("workspace_id, name, namespace").
		Where("id = ?", projectID.String()).
		Take(&project).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || strings.TrimSpace(project.Name) == "" {
		return "", domain.ErrInvalidWorkflowInput
	}
	if err != nil {
		return "", err
	}

	var secret struct {
		ServiceType string
		ServiceURL  string
	}
	err = r.db.WithContext(ctx).Table("workspace_app.secrets").
		Select("service_type, service_url").
		Where("id = ? AND workspace_id = ? AND status = ?", secretID, project.WorkspaceID, "active").
		Take(&secret).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", domain.ErrRegistryNotFound
	}
	if err != nil {
		return "", err
	}
	kind := strings.ToLower(strings.TrimSpace(secret.ServiceType))
	if !strings.Contains(kind, "harbor") && !strings.Contains(kind, "registry") && !strings.Contains(kind, "docker") {
		return "", domain.ErrRegistryNotFound
	}
	namespace := strings.TrimSpace(project.Namespace)
	if namespace == "" {
		namespace = project.Name
	}
	path := engine.ImageRepositoryPath(secret.ServiceURL, namespace, project.Name)
	if path == "" {
		return "", domain.ErrRegistryNotFound
	}
	return path, nil
}
