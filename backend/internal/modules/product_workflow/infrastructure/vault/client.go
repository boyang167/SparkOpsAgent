package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/httperror"
)

type Client struct {
	Address string
	Token   string
	HTTP    *http.Client
}

func (c Client) Read(ctx context.Context, secretPath string) (string, error) {
	address := strings.TrimRight(strings.TrimSpace(c.Address), "/")
	secretPath = strings.Trim(strings.TrimSpace(secretPath), "/")
	if address == "" || c.Token == "" || secretPath == "" {
		return "", fmt.Errorf("vault is not configured")
	}
	endpoint := address + "/v1/" + secretPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", httperror.SanitizeNetwork(err)
	}
	req.Header.Set("X-Vault-Token", c.Token)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", httperror.SanitizeNetwork(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("vault status %d", resp.StatusCode)
	}
	var payload struct {
		Data struct {
			Data  map[string]any `json:"data"`
			Value string         `json:"value"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Data.Data != nil {
		if value, ok := payload.Data.Data["value"].(string); ok {
			return value, nil
		}
		for _, value := range payload.Data.Data {
			if text, ok := value.(string); ok {
				return text, nil
			}
		}
	}
	if payload.Data.Value != "" {
		return payload.Data.Value, nil
	}
	return "", fmt.Errorf("vault secret %s has no string value", secretPath)
}
