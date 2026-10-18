package harbor

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httperror"
)

const defaultTimeout = 15 * time.Second

// EnsureProject verifies a Harbor project exists and creates it when missing.
// Harbor may return 403 (not 404) for unknown projects when the caller is not a system admin.
func EnsureProject(ctx context.Context, serviceURL, username, password, projectName string) error {
	projectName = strings.TrimSpace(strings.ToLower(projectName))
	if projectName == "" {
		return fmt.Errorf("harbor project name is empty")
	}
	baseURL, err := normalizeServiceURL(serviceURL)
	if err != nil {
		return fmt.Errorf("invalid harbor service url: %w", err)
	}

	client := &http.Client{
		Timeout: defaultTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // internal registries
		},
	}

	checkURL := joinURL(baseURL, "/api/v2.0/projects/"+url.PathEscape(projectName))
	status, body, err := doRequest(ctx, client, http.MethodGet, checkURL, username, password, nil)
	if err != nil {
		return fmt.Errorf("check harbor project %q: %w", projectName, err)
	}
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusNotFound, http.StatusForbidden:
		// continue to create
	default:
		return fmt.Errorf("check harbor project %q: HTTP %d: %s", projectName, status, truncateBody(body))
	}

	payload, err := json.Marshal(map[string]any{
		"project_name": projectName,
		"public":       false,
	})
	if err != nil {
		return fmt.Errorf("encode harbor project payload: %w", err)
	}
	createURL := joinURL(baseURL, "/api/v2.0/projects")
	status, body, err = doRequest(ctx, client, http.MethodPost, createURL, username, password, payload)
	if err != nil {
		return fmt.Errorf("create harbor project %q: %w", projectName, err)
	}
	switch status {
	case http.StatusCreated, http.StatusConflict:
		return nil
	default:
		return fmt.Errorf(
			"create harbor project %q: HTTP %d: %s (use a Harbor system admin account in workspace secrets)",
			projectName,
			status,
			truncateBody(body),
		)
	}
}

func doRequest(
	ctx context.Context,
	client *http.Client,
	method, requestURL, username, password string,
	body []byte,
) (int, []byte, error) {
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return 0, nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, httperror.SanitizeNetwork(err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, respBody, nil
}

func normalizeServiceURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("empty url")
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	return strings.TrimSuffix(u.String(), "/"), nil
}

func joinURL(base, path string) string {
	return strings.TrimSuffix(base, "/") + path
}

func truncateBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}
