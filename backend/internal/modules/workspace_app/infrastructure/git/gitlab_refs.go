package git

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
)

type RefItem struct {
	Name      string
	Title     string
	Default   bool
	Protected bool
}

func ResolveAuthToken(creds model.SecretCredentials) string {
	if v := strings.TrimSpace(creds.Token); v != "" {
		return v
	}
	if v := strings.TrimSpace(creds.APIKey); v != "" {
		return v
	}
	return strings.TrimSpace(creds.Password)
}

func ListGitLabRefs(
	ctx context.Context,
	gitURL string,
	serviceURL string,
	creds model.SecretCredentials,
	refType string,
) ([]RefItem, error) {
	projectPath, err := ParseGitProjectPath(gitURL)
	if err != nil {
		return nil, err
	}
	apiBase, err := ResolveGitLabAPIBase(gitURL, serviceURL)
	if err != nil {
		return nil, err
	}
	token := ResolveAuthToken(creds)
	if token == "" {
		return nil, fmt.Errorf("git secret token is required")
	}

	encoded := url.PathEscape(projectPath)
	if refType == "pr" {
		return listGitLabMergeRequests(ctx, apiBase, encoded, token)
	}

	resource := "branches"
	switch refType {
	case "tag":
		resource = "tags"
	case "commit":
		resource = "commits"
	}
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/repository/%s?per_page=100", apiBase, encoded, resource)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitlab status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if refType == "commit" {
		var rows []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(body, &rows); err != nil {
			return nil, fmt.Errorf("gitlab decode: %w", err)
		}
		out := make([]RefItem, 0, len(rows))
		for _, row := range rows {
			name := strings.TrimSpace(row.ID)
			if name == "" {
				continue
			}
			out = append(out, RefItem{Name: name, Title: strings.TrimSpace(row.Title)})
		}
		return out, nil
	}

	var rows []struct {
		Name      string `json:"name"`
		Default   bool   `json:"default"`
		Protected bool   `json:"protected"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("gitlab decode: %w", err)
	}
	out := make([]RefItem, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			continue
		}
		out = append(out, RefItem{
			Name:      name,
			Default:   row.Default,
			Protected: row.Protected,
		})
	}
	return out, nil
}

func listGitLabMergeRequests(
	ctx context.Context,
	apiBase string,
	encodedProject string,
	token string,
) ([]RefItem, error) {
	endpoint := fmt.Sprintf(
		"%s/api/v4/projects/%s/merge_requests?state=opened&order_by=updated_at&sort=desc&per_page=100",
		apiBase,
		encodedProject,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitlab status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rows []struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		SourceBranch string `json:"source_branch"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("gitlab decode: %w", err)
	}

	out := make([]RefItem, 0, len(rows))
	for _, row := range rows {
		if row.IID <= 0 {
			continue
		}
		title := strings.TrimSpace(row.Title)
		if title == "" {
			title = strings.TrimSpace(row.SourceBranch)
		}
		out = append(out, RefItem{
			Name:  fmt.Sprintf("%d", row.IID),
			Title: title,
		})
	}
	return out, nil
}
