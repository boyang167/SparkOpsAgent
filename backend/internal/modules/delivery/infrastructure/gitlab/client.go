package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httperror"
)

type Client struct {
	baseURL    string
	token      string
	pathPrefix string
	httpClient *http.Client
}

func NewFromConfig(cfg config.GitLabConfig) *Client {
	if cfg.BaseURL == "" || cfg.Token == "" {
		return nil
	}
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		token:      cfg.Token,
		pathPrefix: strings.Trim(cfg.ProjectPath, "/"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) ListBranches(ctx context.Context, projectName string) ([]port.GitBranch, error) {
	if c == nil {
		return nil, fmt.Errorf("gitlab not configured")
	}
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return nil, fmt.Errorf("project name required")
	}
	repoPath := projectName
	if c.pathPrefix != "" {
		repoPath = c.pathPrefix + "/" + projectName
	}
	encoded := url.PathEscape(repoPath)
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/repository/branches", c.baseURL, encoded)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab request: %w", httperror.SanitizeNetwork(err))
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gitlab status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rows []struct {
		Name      string `json:"name"`
		Default   bool   `json:"default"`
		Protected bool   `json:"protected"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("gitlab decode: %w", err)
	}
	out := make([]port.GitBranch, 0, len(rows))
	for _, row := range rows {
		out = append(out, port.GitBranch{
			Name: row.Name, Default: row.Default, Protected: row.Protected,
		})
	}
	return out, nil
}

var _ port.GitBranchLister = (*Client)(nil)
