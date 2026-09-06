package command

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/application/query"
)

// SyncBuiltinTemplates upgrades legacy workspace Jenkinsfile templates to the
// current SparkOps built-in pipeline source.
func SyncBuiltinTemplates(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}

	detail := query.BuiltinJenkinsfileTemplateDetail()
	var rows []struct {
		ID string
	}
	err := db.WithContext(ctx).
		Table("workspace_app.templates").
		Select("id").
		Where("deleted_at IS NULL").
		Where("LOWER(name) = ?", "jenkinsfile").
		Where("LOWER(kind) = ?", "jenkinsfile").
		Find(&rows).Error
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, row := range rows {
		var current struct {
			Detail string
		}
		if err := db.WithContext(ctx).
			Table("workspace_app.templates").
			Select("detail").
			Where("id = ?", row.ID).
			Take(&current).Error; err != nil {
			return err
		}
		if !query.IsLegacyJenkinsfileTemplateDetail(current.Detail) {
			continue
		}
		if strings.TrimSpace(current.Detail) == strings.TrimSpace(detail) {
			continue
		}
		if err := db.WithContext(ctx).
			Table("workspace_app.templates").
			Where("id = ?", row.ID).
			Updates(map[string]any{
				"detail":     detail,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}
