package query

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
)

func testSecretConnectivity(ctx context.Context, s *model.Secret, creds model.SecretCredentials) (bool, string) {
	baseURL, err := normalizeServiceURL(s.ServiceURL)
	if err != nil {
		return false, "无效的服务地址，请填写完整 URL（如 https://192.168.5.24）"
	}

	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // probe internal registries
		},
	}

	candidates := connectionTestURLs(s.ServiceType, baseURL)
	if len(candidates) == 0 {
		candidates = []string{baseURL}
	}

	lastMsg := "连接失败"
	for _, testURL := range candidates {
		ok, msg := probeURL(ctx, client, testURL, s.Username, creds.Password)
		if ok {
			return true, "连接成功"
		}
		lastMsg = msg
	}
	return false, lastMsg
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

func connectionTestURLs(serviceType, baseURL string) []string {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "harbor":
		urls := []string{
			joinURL(baseURL, "/api/v2.0/health"),
			joinURL(baseURL, "/api/v2.0/systeminfo"),
			baseURL,
		}
		if strings.HasPrefix(baseURL, "http://") {
			httpsBase := "https://" + strings.TrimPrefix(baseURL, "http://")
			urls = append(urls,
				joinURL(httpsBase, "/api/v2.0/health"),
				joinURL(httpsBase, "/api/v2.0/systeminfo"),
				httpsBase,
			)
		}
		return urls
	case "gitlab":
		return []string{
			joinURL(baseURL, "/api/v4/version"),
			joinURL(baseURL, "/-/readiness"),
			baseURL,
		}
	case "github":
		return []string{
			joinURL(baseURL, "/api/v3"),
			baseURL,
		}
	case "jenkins":
		return []string{
			joinURL(baseURL, "/login"),
			joinURL(baseURL, "/api/json"),
			baseURL,
		}
	case "docker", "docker_registry", "docker registry":
		return []string{
			joinURL(baseURL, "/v2/"),
			baseURL,
		}
	case "sonarqube":
		return []string{
			joinURL(baseURL, "/api/system/status"),
			baseURL,
		}
	case "nexus":
		return []string{
			joinURL(baseURL, "/service/rest/v1/status"),
			baseURL,
		}
	default:
		return []string{baseURL}
	}
}

func joinURL(base, path string) string {
	return strings.TrimSuffix(base, "/") + path
}

func probeURL(ctx context.Context, client *http.Client, testURL, username, password string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, testURL, nil)
	if err != nil {
		return false, "无效的服务地址"
	}
	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return false, "连接被拒绝，请检查地址与端口"
		}
		if strings.Contains(strings.ToLower(err.Error()), "tls") || strings.Contains(strings.ToLower(err.Error()), "certificate") {
			return false, "TLS 握手失败，请尝试 https:// 地址"
		}
		return false, "连接失败"
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		return true, "连接成功"
	case resp.StatusCode == 401:
		return true, "连接成功（服务可达，凭据未校验）"
	case resp.StatusCode == 403:
		return true, "连接成功（服务可达，访问受限）"
	default:
		return false, fmt.Sprintf("服务返回异常状态: %d", resp.StatusCode)
	}
}
