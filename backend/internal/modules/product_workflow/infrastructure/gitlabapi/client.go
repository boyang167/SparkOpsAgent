package gitlabapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	gitpkg "github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/infrastructure/git"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

type HookClient struct {
	HTTP *http.Client
}

func (c HookClient) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

type HookSpec struct {
	APIBase string
	Token   string
	Project string
	HookURL string
	Secret  string
	Events  []string
}

func (c HookClient) Ensure(ctx context.Context, spec HookSpec) error {
	projectPath, err := gitpkg.ParseGitProjectPath(spec.Project)
	if err != nil {
		projectPath = strings.Trim(spec.Project, "/")
	}
	if projectPath == "" {
		return fmt.Errorf("gitlab project path required")
	}
	apiBase := strings.TrimRight(strings.TrimSpace(spec.APIBase), "/")
	if apiBase == "" {
		return fmt.Errorf("gitlab api base required")
	}
	encoded := url.PathEscape(projectPath)
	body := hookBody(spec.HookURL, spec.Secret, spec.Events)
	existing, err := c.find(ctx, apiBase, encoded, spec.Token, spec.HookURL)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(body)
	method := http.MethodPost
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/hooks", apiBase, encoded)
	if existing > 0 {
		method = http.MethodPut
		endpoint = fmt.Sprintf("%s/%d", endpoint, existing)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", spec.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("gitlab hook %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	return nil
}

func (c HookClient) find(ctx context.Context, apiBase, project, token, hookURL string) (int, error) {
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/hooks", apiBase, project)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	resp, err := c.client().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("gitlab list hooks %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	var rows []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(payload, &rows); err != nil {
		return 0, err
	}
	for _, row := range rows {
		if strings.TrimRight(row.URL, "/") == strings.TrimRight(hookURL, "/") {
			return row.ID, nil
		}
	}
	return 0, nil
}

func hookBody(hookURL, secret string, events []string) map[string]any {
	body := map[string]any{
		"url": hookURL, "token": secret, "enable_ssl_verification": false,
		"push_events": false, "tag_push_events": false, "merge_requests_events": false,
	}
	if len(events) == 0 {
		body["push_events"] = true
		return body
	}
	for _, event := range events {
		switch event {
		case "push":
			body["push_events"] = true
		case "tag":
			body["tag_push_events"] = true
		case "merge_request":
			body["merge_requests_events"] = true
		}
	}
	return body
}

func ReportStatus(ctx context.Context, apiBase, token, projectPath, sha, state, description string) error {
	return ReportStatusWithContext(ctx, apiBase, token, projectPath, sha, state, "sparkops/workflow", description)
}

func ReportStatusWithContext(ctx context.Context, apiBase, token, projectPath, sha, state, statusContext, description string) error {
	if strings.TrimSpace(sha) == "" {
		return nil
	}
	encodedProject := url.PathEscape(projectPath)
	encodedSHA := url.PathEscape(sha)
	raw, _ := json.Marshal(map[string]string{
		"state": state, "context": statusContext, "description": description,
	})
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/statuses/%s", strings.TrimRight(apiBase, "/"), encodedProject, encodedSHA)
	return gitlabJSON(ctx, http.MethodPost, endpoint, token, raw)
}

func ReportMergeRequestNote(ctx context.Context, apiBase, token, projectPath string, iid int, body string) error {
	if iid <= 0 {
		return nil
	}
	encodedProject := url.PathEscape(projectPath)
	raw, _ := json.Marshal(map[string]string{"body": body})
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/notes", strings.TrimRight(apiBase, "/"), encodedProject, iid)
	return gitlabJSON(ctx, http.MethodPost, endpoint, token, raw)
}

func gitlabJSON(ctx context.Context, method, endpoint, token string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return classifyGitLabHTTP(resp.StatusCode, payload)
	}
	return nil
}

func classifyGitLabHTTP(status int, payload []byte) error {
	err := fmt.Errorf("gitlab status %d: %s", status, strings.TrimSpace(string(payload)))
	if status >= http.StatusBadRequest &&
		status < http.StatusInternalServerError &&
		status != http.StatusRequestTimeout &&
		status != http.StatusTooManyRequests {
		return messaging.Permanent(err)
	}
	return err
}
