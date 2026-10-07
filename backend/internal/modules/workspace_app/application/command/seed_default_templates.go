package command

import (
	"context"
	"embed"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
)

//go:embed default_templates/*.yaml
var defaultTemplateFS embed.FS

const defaultTemplateCreator = "00000000-0000-4000-8000-000000000099"

type defaultTemplateSpec struct {
	Name string
	Kind string
	// File is read from default_templates; empty means the built-in Jenkinsfile source.
	File string
	// Aliases are legacy names that already count as this default.
	Aliases []string
}

var defaultTemplateSpecs = []defaultTemplateSpec{
	{Name: "default-deployment", Kind: "Deployment", File: "deployment.yaml"},
	{Name: "default-service", Kind: "Service", File: "service.yaml"},
	{Name: "default-configmap", Kind: "ConfigMap", File: "configmap.yaml"},
	{Name: "default-ingress", Kind: "Ingress", File: "ingress.yaml"},
	{Name: "default-secret", Kind: "Secret", File: "secret.yaml"},
	{Name: "default-pvc", Kind: "PersistentVolumeClaim", File: "persistentvolumeclaim.yaml"},
	{Name: "default-pv", Kind: "PersistentVolume", File: "persistentvolume.yaml"},
	{Name: "default-storageclass", Kind: "StorageClass", File: "storageclass.yaml"},
	{Name: "default-statefulset", Kind: "StatefulSet", File: "statefulset.yaml"},
	{Name: "default-job", Kind: "Job", File: "job.yaml"},
	{Name: "default-virtualservice", Kind: "VirtualService", File: "virtualservice.yaml"},
	{Name: "default-destinationrule", Kind: "DestinationRule", File: "destinationrule.yaml"},
	{Name: "default-gateway", Kind: "Gateway", File: "gateway.yaml"},
	{Name: "default-jenkinsfile", Kind: "jenkinsFile", Aliases: []string{"jenkinsfile"}},
}

func defaultTemplateDetail(spec defaultTemplateSpec) (string, error) {
	if spec.File == "" {
		return query.BuiltinJenkinsfileTemplateDetail(), nil
	}
	raw, err := defaultTemplateFS.ReadFile("default_templates/" + spec.File)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// SeedDefaultTemplates gives every live workspace the built-in default templates.
func SeedDefaultTemplates(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var workspaceIDs []string
	if err := db.WithContext(ctx).
		Table("tenant.workspaces").
		Where("deleted_at IS NULL").
		Pluck("id", &workspaceIDs).Error; err != nil {
		return err
	}
	for _, workspaceID := range workspaceIDs {
		if err := SeedWorkspaceDefaultTemplates(ctx, db, workspaceID, ""); err != nil {
			return err
		}
	}
	return nil
}

// SeedWorkspaceDefaultTemplates inserts missing defaults for one workspace.
// Soft-deleted rows still count as present so a removed default is not revived.
func SeedWorkspaceDefaultTemplates(ctx context.Context, db *gorm.DB, workspaceID, createdBy string) error {
	if db == nil || strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	if strings.TrimSpace(createdBy) == "" {
		createdBy = defaultTemplateCreator
	}
	var existing []struct {
		Name string
		Kind string
	}
	if err := db.WithContext(ctx).
		Table("workspace_app.templates").
		Select("name, kind").
		Where("workspace_id = ?", workspaceID).
		Find(&existing).Error; err != nil {
		return err
	}
	present := make(map[string]bool, len(existing))
	for _, row := range existing {
		present[templateKey(row.Name, row.Kind)] = true
	}

	now := time.Now().UTC()
	for _, spec := range defaultTemplateSpecs {
		if present[templateKey(spec.Name, spec.Kind)] || aliasPresent(present, spec) {
			continue
		}
		detail, err := defaultTemplateDetail(spec)
		if err != nil {
			return err
		}
		if err := db.WithContext(ctx).Table("workspace_app.templates").Create(map[string]any{
			"id":           uuid.NewString(),
			"workspace_id": workspaceID,
			"name":         spec.Name,
			"kind":         spec.Kind,
			"language":     "other",
			"detail":       detail,
			"rules":        "",
			"status":       "active",
			"is_public":    false,
			"created_by":   createdBy,
			"created_at":   now,
			"updated_at":   now,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func aliasPresent(present map[string]bool, spec defaultTemplateSpec) bool {
	for _, alias := range spec.Aliases {
		if present[templateKey(alias, spec.Kind)] {
			return true
		}
	}
	return false
}

func templateKey(name, kind string) string {
	return strings.ToLower(strings.TrimSpace(name)) + "|" + strings.ToLower(strings.TrimSpace(kind))
}
