package gitlabapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	gitpkg "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/git"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httperror"
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
	HTTP   *http.Client
}

func (r Reporter) client() *http.Client {
	if r.HTTP != nil {
		return r.HTTP
	}
	return &http.Client{Timeout: 12 * time.Second}
}

func (r Reporter) CurrentStatus(ctx context.Context, payload workflowoutbound.GitLabStatusPayload) (string, error) {
	apiBase, token, projectPath, err := r.target(ctx, payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(apiBase, "/") + "/api/v4/projects/" +
		url.PathEscape(projectPath) + "/repository/commits/" + url.PathEscape(payload.Commit) + "/statuses"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", httperror.SanitizeNetwork(err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	resp, err := r.client().Do(req)
	if err != nil {
		return "", httperror.SanitizeNetwork(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= http.StatusMultipleChoices {
		return "", classifyGitLabHTTP(resp.StatusCode, raw)
	}
	var statuses []struct {
		Status string `json:"status"`
		Name   string `json:"name"`
	}
	if err := json.Unmarshal(raw, &statuses); err != nil {
		return "", err
	}
	for _, status := range statuses {
		if status.Name == payload.Context {
			return status.Status, nil
		}
	}
	return "", nil
}

func (r Reporter) Report(ctx context.Context, payload workflowoutbound.GitLabStatusPayload) error {
	apiBase, token, projectPath, err := r.target(ctx, payload)
	if err != nil {
		return err
	}
	return ReportStatusWithClient(
		ctx, apiBase, token, projectPath, payload.Commit,
		payload.DesiredStatus, payload.Context, payload.Description, r.client(),
	)
}

func (r Reporter) target(
	ctx context.Context,
	payload workflowoutbound.GitLabStatusPayload,
) (apiBase, token, projectPath string, err error) {
	if r.Lookup == nil {
		return "", "", "", fmt.Errorf("project git lookup is required")
	}
	projectID, err := kernel.ParseProjectID(payload.ProjectID)
	if err != nil {
		return "", "", "", err
	}
	project, err := r.Lookup.LookupProjectGit(ctx, projectID)
	if err != nil {
		return "", "", "", err
	}
	if strings.TrimSpace(project.Token) == "" || strings.TrimSpace(project.GitURL) == "" {
		return "", "", "", fmt.Errorf("project GitLab configuration is incomplete")
	}
	apiBase, err = gitpkg.ResolveGitLabAPIBase(project.GitURL, project.ServiceURL)
	if err != nil {
		return "", "", "", err
	}
	projectPath, err = gitpkg.ParseGitProjectPath(project.GitURL)
	if err != nil {
		return "", "", "", err
	}
	return apiBase, project.Token, projectPath, nil
}
