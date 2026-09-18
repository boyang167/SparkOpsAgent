package gitlabapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	gitpkg "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/git"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type HookSync struct {
	Lookup     command.ProjectGitLookup
	PublicBase string
	Logger     *zap.Logger
}

func (s HookSync) Sync(ctx context.Context, wf *model.ProductWorkflow) error {
	if s.Lookup == nil || wf == nil {
		return nil
	}
	base := strings.TrimRight(strings.TrimSpace(s.PublicBase), "/")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	hookURL := base + "/api/v1/public/workflows/gitlab"
	var failed []string
	for _, trigger := range wf.Triggers {
		if trigger.Type != model.TriggerGit || !trigger.Active() {
			continue
		}
		projectID, err := kernel.ParseProjectID(trigger.ProjectID)
		if err != nil {
			failed = append(failed, "无效项目")
			continue
		}
		project, err := s.Lookup.LookupProjectGit(ctx, projectID)
		if err != nil {
			failed = append(failed, err.Error())
			continue
		}
		if project.ProductID != wf.ProductID {
			failed = append(failed, "项目不属于当前产品")
			continue
		}
		if strings.TrimSpace(project.Token) == "" || strings.TrimSpace(project.GitURL) == "" {
			failed = append(failed, "项目未绑定可用的 GitLab 密钥")
			continue
		}
		apiBase, err := gitpkg.ResolveGitLabAPIBase(project.GitURL, project.ServiceURL)
		if err != nil {
			failed = append(failed, err.Error())
			continue
		}
		err = (HookClient{}).Ensure(ctx, HookSpec{
			APIBase: apiBase, Token: project.Token, Project: project.GitURL,
			HookURL: hookURL, Secret: trigger.Token, Events: trigger.Events,
		})
		if err != nil {
			failed = append(failed, err.Error())
		}
	}
	if len(failed) == 0 {
		return nil
	}
	if s.Logger != nil {
		s.Logger.Warn("gitlab hook sync failed", zap.Strings("errors", failed))
	}
	return kernel.NewDomainError("PRODUCT_WORKFLOW_INVALID_INPUT", "GitLab Hook 注册失败: "+strings.Join(failed, "; "))
}

type Reporter struct {
	Lookup command.ProjectGitLookup
	Logger *zap.Logger
}

func (r Reporter) Report(ctx context.Context, run *model.ProductWorkflowRun) {
	if r.Lookup == nil || run == nil || len(run.Trigger) == 0 {
		return
	}
	var trigger struct {
		Source    string `json:"source"`
		ProjectID string `json:"project_id"`
		Commit    string `json:"commit"`
		Event     string `json:"event"`
		MRIID     int    `json:"mr_iid"`
		Ref       string `json:"ref"`
	}
	if err := json.Unmarshal(run.Trigger, &trigger); err != nil || trigger.Source != "git" {
		return
	}
	projectID, err := kernel.ParseProjectID(trigger.ProjectID)
	if err != nil {
		return
	}
	project, err := r.Lookup.LookupProjectGit(ctx, projectID)
	if err != nil || project.Token == "" || project.GitURL == "" {
		return
	}
	apiBase, err := gitpkg.ResolveGitLabAPIBase(project.GitURL, project.ServiceURL)
	if err != nil {
		return
	}
	projectPath, err := gitpkg.ParseGitProjectPath(project.GitURL)
	if err != nil {
		return
	}
	state := "running"
	switch run.Status {
	case model.RunStatusSuccess:
		state = "success"
	case model.RunStatusFailed:
		state = "failed"
	}
	description := "SparkOps workflow " + string(run.Status)
	if run.ErrorMessage != "" {
		description = run.ErrorMessage
	}
	if err := ReportStatus(ctx, apiBase, project.Token, projectPath, trigger.Commit, state, description); err != nil && r.Logger != nil {
		r.Logger.Warn("gitlab status report failed", zap.Error(err))
	}
	if trigger.MRIID > 0 {
		note := fmt.Sprintf("SparkOps 工作流 %s：%s", run.ID.String(), description)
		if err := ReportMergeRequestNote(ctx, apiBase, project.Token, projectPath, trigger.MRIID, note); err != nil && r.Logger != nil {
			r.Logger.Warn("gitlab merge request note failed", zap.Error(err))
		}
	}
}
