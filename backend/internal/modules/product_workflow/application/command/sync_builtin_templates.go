package command

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/persistence"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

var (
	stdBuildDeployUATID   = kernel.WorkflowTemplateID{UUID: uuid.MustParse("00000000-0000-4000-8000-000000000001")}
	stdReleaseProdID      = kernel.WorkflowTemplateID{UUID: uuid.MustParse("00000000-0000-4000-8000-000000000002")}
	stdVersionReleaseID   = kernel.WorkflowTemplateID{UUID: uuid.MustParse("00000000-0000-4000-8000-000000000003")}
	systemMemberID        = kernel.MemberID{UUID: uuid.MustParse("00000000-0000-4000-8000-000000000099")}
)

func SyncBuiltinTemplates(ctx context.Context, db *gorm.DB) error {
	repo := persistence.NewWorkflowTemplateRepository(db)
	now := time.Now().UTC()
	templates := []model.WorkflowTemplate{
		{
			ID: stdBuildDeployUATID, Scope: model.TemplateScopeSystem, Name: "标准构建部署 UAT",
			Key: "std-build-deploy-uat", Description: "并行构建产品内项目并部署到 UAT 环境",
			Category: "release", Status: model.TemplateStatusActive, Version: "1.0.0",
			KeyVals: []model.KeyVal{
				{Key: "build_branch", Description: "构建分支", Type: "string", Required: true, Value: "main"},
				{Key: "uat_env_id", Description: "UAT 环境 ID", Type: "string", Required: true},
			},
			Definition: json.RawMessage(`{
				"stages": [
					{"name": "构建", "parallel": true, "jobs": [{"name": "build-services", "type": "sparkops-build", "spec": {"branch": "{{build_branch}}"}}]},
					{"name": "部署 UAT", "jobs": [{"name": "deploy-uat", "type": "sparkops-deploy", "spec": {"environment_id": "{{uat_env_id}}", "source": "fromjob", "job_name": "build-services"}}]}
				]
			}`),
			SlotSchema: json.RawMessage(`{"build_branch": {"label": "构建分支"}, "uat_env_id": {"label": "UAT 环境"}}`),
			Runtime:    json.RawMessage(`{}`), CreatedBy: systemMemberID, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: stdReleaseProdID, Scope: model.TemplateScopeSystem, Name: "标准生产发布",
			Key: "std-release-prod", Description: "构建 → UAT → 审批 → 生产发布",
			Category: "release", Status: model.TemplateStatusActive, Version: "1.0.0",
			KeyVals: []model.KeyVal{
				{Key: "build_branch", Description: "构建分支", Type: "string", Required: true, Value: "main"},
				{Key: "uat_env_id", Description: "UAT 环境 ID", Type: "string", Required: true},
				{Key: "prod_env_id", Description: "生产环境 ID", Type: "string", Required: true},
			},
			Definition: json.RawMessage(`{
				"stages": [
					{"name": "构建", "parallel": true, "jobs": [{"name": "build-services", "type": "sparkops-build", "spec": {"branch": "{{build_branch}}"}}]},
					{"name": "部署 UAT", "jobs": [{"name": "deploy-uat", "type": "sparkops-deploy", "spec": {"environment_id": "{{uat_env_id}}", "source": "fromjob", "job_name": "build-services"}}]},
					{"name": "生产审批", "jobs": [{"name": "prod-approval", "type": "sparkops-approval", "spec": {"provider": "sparkops", "approver_ids": [], "required_approvals": 1, "timeout_minutes": 120, "strategy": {"timeout_minutes": 120, "on_timeout": "fail"}, "notify": []}}]},
					{"name": "部署生产", "jobs": [{"name": "deploy-prod", "type": "sparkops-deploy", "spec": {"environment_id": "{{prod_env_id}}", "source": "fromjob", "job_name": "build-services"}}]}
				]
			}`),
			SlotSchema: json.RawMessage(`{"build_branch": {"label": "构建分支"}, "uat_env_id": {"label": "UAT 环境"}, "prod_env_id": {"label": "生产环境"}}`),
			Runtime:    json.RawMessage(`{}`), CreatedBy: systemMemberID, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: stdVersionReleaseID, Scope: model.TemplateScopeSystem, Name: "标准发布（版本化）",
			Key: "std-version-release", Description: "构建 → 创建版本 → 部署版本",
			Category: "release", Status: model.TemplateStatusActive, Version: "1.0.0",
			KeyVals: []model.KeyVal{
				{Key: "build_branch", Description: "构建分支", Type: "string", Required: true, Value: "main"},
				{Key: "version", Description: "交付版本号", Type: "string", Required: true},
				{Key: "prod_env_id", Description: "生产环境 ID", Type: "string", Required: true},
			},
			Definition: json.RawMessage(`{
				"stages": [
					{"name": "构建", "parallel": true, "jobs": [{"name": "build-all", "type": "sparkops-build", "spec": {"branch": "{{build_branch}}"}}]},
					{"name": "发布", "jobs": [{"name": "deploy-prod", "type": "sparkops-deploy", "spec": {"artifact_source": "version", "source_kind": "build", "environment_id": "{{prod_env_id}}", "source": "fromjob", "job_name": "build-all"}}]}
				]
			}`),
			SlotSchema: json.RawMessage(`{"build_branch": {"label": "构建分支"}, "version": {"label": "版本号"}, "prod_env_id": {"label": "生产环境"}}`),
			Runtime:    json.RawMessage(`{}`), CreatedBy: systemMemberID, CreatedAt: now, UpdatedAt: now,
		},
	}
	for _, tpl := range templates {
		existing, err := repo.FindByKey(ctx, tpl.Key)
		if err == nil {
			tpl.ID = existing.ID
			tpl.CreatedAt = existing.CreatedAt
			if err := repo.Update(ctx, &tpl); err != nil {
				return err
			}
			continue
		}
		if err := repo.Save(ctx, &tpl); err != nil {
			return err
		}
	}
	return nil
}
