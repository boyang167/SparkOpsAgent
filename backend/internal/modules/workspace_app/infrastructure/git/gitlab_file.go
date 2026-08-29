package git

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
)

func NormalizeRepoFilePath(raw string) string {
	path := strings.TrimSpace(raw)
	path = strings.TrimPrefix(path, "./")
	return strings.Trim(path, "/")
}

func encodeGitLabFilePath(path string) string {
	path = NormalizeRepoFilePath(path)
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "%2F")
}

func GetGitLabFileRaw(
	ctx context.Context,
	gitURL string,
	serviceURL string,
	creds model.SecretCredentials,
	ref string,
	filePath string,
) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("git ref is required")
	}
	filePath = NormalizeRepoFilePath(filePath)
	if filePath == "" {
		return "", fmt.Errorf("file path is required")
	}

	projectPath, err := ParseGitProjectPath(gitURL)
	if err != nil {
		return "", err
	}
	apiBase, err := ResolveGitLabAPIBase(gitURL, serviceURL)
	if err != nil {
		return "", err
	}
	token := ResolveAuthToken(creds)
	if token == "" {
		return "", fmt.Errorf("git secret token is required")
	}

	encodedProject := url.PathEscape(projectPath)
	encodedFile := encodeGitLabFilePath(filePath)
	endpoint := fmt.Sprintf(
		"%s/api/v4/projects/%s/repository/files/%s/raw?ref=%s",
		apiBase,
		encodedProject,
		encodedFile,
		url.QueryEscape(ref),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gitlab request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("gitlab status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}
