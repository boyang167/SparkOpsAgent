package performance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

const simulatedCapacityBytes int64 = 100 * 1024 * 1024 * 1024 * 1024

func TestFilesystemMetadataCapacity(t *testing.T) {
	config := requireLoadConfig(t)
	rowCount := envInt("FS_LOAD_METADATA_ROWS", 10_000)
	pageRequests := envInt("FS_LOAD_METADATA_PAGES", 100)
	client := newLoadClient(config)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	runID := time.Now().UTC().Format("20060102T150405.000000000")
	for i := 0; i < rowCount; i++ {
		body := map[string]any{"name": fmt.Sprintf("load-%s-%08d", runID, i)}
		if err := client.controlJSON(ctx, http.MethodPost, "/api/v1/workspaces/"+config.WorkspaceID+"/directories", fmt.Sprintf("metadata-%s-%d", runID, i), body, nil); err != nil {
			t.Fatalf("create metadata row %d: %v", i, err)
		}
	}

	durations := make([]time.Duration, pageRequests)
	cursor := ""
	for i := 0; i < pageRequests; i++ {
		query := url.Values{"limit": {"100"}}
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		started := time.Now()
		var page struct {
			NextCursor string `json:"next_cursor"`
		}
		if err := client.controlJSON(ctx, http.MethodGet, "/api/v1/workspaces/"+config.WorkspaceID+"/files?"+query.Encode(), "", nil, &page); err != nil {
			t.Fatalf("list metadata page %d: %v", i, err)
		}
		durations[i] = time.Since(started)
		cursor = page.NextCursor
	}

	p95 := percentile(durations, 0.95)
	if p95 > 500*time.Millisecond {
		t.Fatalf("metadata list p95 = %s, want <=500ms", p95)
	}
	report := LoadReport{
		GeneratedAt: time.Now().UTC(),
		BaseURL:     config.BaseURL,
		Metadata: MetadataResult{
			SimulatedBytes: simulatedCapacityBytes,
			RowsCreated:    rowCount, PageRequests: pageRequests, ListP95: p95, Passed: true,
		},
	}
	writeReportFromEnv(t, report)
}

func (c *loadClient) controlJSON(ctx context.Context, method, path, idempotencyKey string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.config.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s: %s", resp.Status, string(data))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	var wrapped struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		return err
	}
	return json.Unmarshal(wrapped.Data, out)
}
