package filesystemgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
)

// HTTPControlPort calls the API server internal filesystem gateway endpoints.
type HTTPControlPort struct {
	baseURL       string
	token         string
	httpClient    *http.Client
	localProvider storage.Provider
}

func NewHTTPControlPort(baseURL, token string, localProvider storage.Provider) (*HTTPControlPort, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("%w: empty control base url", ErrInvalidGatewayConfig)
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("%w: empty internal token", ErrInvalidGatewayConfig)
	}
	if localProvider == nil {
		return nil, fmt.Errorf("%w: local provider required", ErrInvalidGatewayConfig)
	}
	return &HTTPControlPort{
		baseURL:       baseURL,
		token:         token,
		localProvider: localProvider,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (c *HTTPControlPort) ResolveUpload(ctx context.Context, sessionID string) (UploadGrant, error) {
	var payload struct {
		TemporaryObjectKey string `json:"temporary_object_key"`
		ExpectedSize       int64  `json:"expected_size"`
		Classification     string `json:"classification"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("%s/internal/filesystem/v1/transfers/%s/upload-grant", c.baseURL, sessionID), &payload); err != nil {
		return UploadGrant{}, err
	}
	return UploadGrant{
		TemporaryObjectKey: payload.TemporaryObjectKey,
		ExpectedSize:       payload.ExpectedSize,
		Provider:           c.localProvider,
		Classification:     model.TransferClassification(payload.Classification),
	}, nil
}

func (c *HTTPControlPort) ResolveDownload(_ context.Context, _ string) (DownloadGrant, error) {
	return DownloadGrant{}, fmt.Errorf("%w", ErrControlUnavailable)
}

func (c *HTTPControlPort) RecordUploaded(ctx context.Context, sessionID string, size int64, sha256 string) error {
	body, err := json.Marshal(map[string]any{"size": size, "sha256": sha256})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/internal/filesystem/v1/transfers/%s/uploaded", c.baseURL, sessionID),
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-SparkOps-Internal-Token", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("record uploaded: status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

func (c *HTTPControlPort) getJSON(ctx context.Context, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-SparkOps-Internal-Token", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("control get %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
