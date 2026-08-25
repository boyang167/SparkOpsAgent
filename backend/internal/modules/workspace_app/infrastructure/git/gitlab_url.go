package git

import (
	"fmt"
	"net/url"
	"strings"
)

func ParseGitProjectPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("git url required")
	}
	if strings.HasPrefix(raw, "git@") {
		rest := strings.TrimPrefix(raw, "git@")
		colon := strings.Index(rest, ":")
		if colon <= 0 {
			return "", fmt.Errorf("invalid git ssh url")
		}
		path := strings.Trim(strings.TrimSuffix(rest[colon+1:], ".git"), "/")
		if path == "" {
			return "", fmt.Errorf("git project path missing in url")
		}
		return path, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid git url")
	}
	path := strings.Trim(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return "", fmt.Errorf("git project path missing in url")
	}
	return path, nil
}

func ResolveGitLabAPIBase(gitURL, serviceURL string) (string, error) {
	if base := strings.TrimRight(strings.TrimSpace(serviceURL), "/"); base != "" {
		return base, nil
	}
	parsed, err := url.Parse(strings.TrimSpace(gitURL))
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("gitlab service url required for ssh git address")
	}
	switch parsed.Scheme {
	case "http", "https":
		return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), nil
	default:
		return "", fmt.Errorf("gitlab service url required for ssh git address")
	}
}

// ParseGitLabProjectFromURL extracts API base (http only) and project path from an HTTP(S) git URL.
func ParseGitLabProjectFromURL(raw string) (apiBase, projectPath string, err error) {
	projectPath, err = ParseGitProjectPath(raw)
	if err != nil {
		return "", "", err
	}
	apiBase, err = ResolveGitLabAPIBase(raw, "")
	if err != nil {
		return "", "", err
	}
	return apiBase, projectPath, nil
}
