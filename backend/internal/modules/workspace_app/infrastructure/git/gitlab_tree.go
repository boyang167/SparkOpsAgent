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

type TreeItem struct {
	Name string
	Path string
	Type string
}

func ListGitLabTree(
	ctx context.Context,
	gitURL string,
	serviceURL string,
	creds model.SecretCredentials,
	ref string,
	path string,
) ([]TreeItem, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("git ref is required")
	}
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
	endpoint := fmt.Sprintf(
		"%s/api/v4/projects/%s/repository/tree?ref=%s&per_page=100",
		apiBase,
		encoded,
		url.QueryEscape(ref),
	)
	path = strings.Trim(path, "/")
	if path != "" {
		endpoint += "&path=" + url.QueryEscape(path)
	}

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
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("gitlab decode: %w", err)
	}
	out := make([]TreeItem, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSpace(row.Name)
		itemPath := strings.TrimSpace(row.Path)
		if name == "" || itemPath == "" {
			continue
		}
		out = append(out, TreeItem{
			Name: name,
			Path: itemPath,
			Type: strings.TrimSpace(row.Type),
		})
	}
	return out, nil
}
