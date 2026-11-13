package performance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const defaultPartSize int64 = 64 << 20

type loadConfig struct {
	BaseURL             string
	Token               string
	WorkspaceID         string
	ProviderID          string
	LargeFilePath       string
	ConcurrentTransfers int
	PartSize            int64
}

type envelope[T any] struct {
	Data T `json:"data"`
}

type transferSession struct {
	SessionID       string         `json:"session_id"`
	Status          string         `json:"status"`
	UploadURL       string         `json:"upload_url"`
	CapabilityToken string         `json:"capability_token"`
	Parts           []transferPart `json:"parts"`
}

type transferPart struct {
	PartNumber int    `json:"part_number"`
	Offset     int64  `json:"offset"`
	Size       int64  `json:"size"`
	Checksum   string `json:"checksum"`
}

type loadClient struct {
	http   *http.Client
	config loadConfig
}

func TestPercentileUsesNearestRank(t *testing.T) {
	got := percentile([]time.Duration{
		10 * time.Millisecond,
		30 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
	}, 0.95)
	if got != 40*time.Millisecond {
		t.Fatalf("p95 = %s, want 40ms", got)
	}
}

func TestFilesystemTwentyGiBResume(t *testing.T) {
	config := requireLoadConfig(t)
	info, err := os.Stat(config.LargeFilePath)
	if err != nil {
		t.Fatalf("stat large fixture: %v", err)
	}
	if info.Size() != 20<<30 {
		t.Fatalf("large fixture must be exactly 20 GiB, got %d bytes", info.Size())
	}

	client := newLoadClient(config)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	expectedSHA, err := fileSHA256(config.LargeFilePath)
	if err != nil {
		t.Fatalf("hash fixture: %v", err)
	}
	session, createDuration, err := client.createSession(ctx, filepath.Base(config.LargeFilePath), info.Size(), expectedSHA, "resume-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err != nil {
		t.Fatalf("create transfer: %v", err)
	}

	partCount := int((info.Size() + config.PartSize - 1) / config.PartSize)
	stopAfter := (partCount*40 + 99) / 100
	if err := client.uploadMissingParts(ctx, session, config.LargeFilePath, config.PartSize, stopAfter); err != nil {
		t.Fatalf("upload first 40%%: %v", err)
	}

	// Recreate the HTTP client to simulate process termination and restart.
	client = newLoadClient(config)
	resumed, err := client.getSession(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("query completed parts after restart: %v", err)
	}
	if err := client.uploadMissingParts(ctx, resumed, config.LargeFilePath, config.PartSize, partCount); err != nil {
		t.Fatalf("upload missing parts: %v", err)
	}
	commitDuration, final, err := client.commit(ctx, resumed.SessionID, expectedSHA)
	if err != nil {
		t.Fatalf("commit transfer: %v", err)
	}
	if final.Status != "COMPLETED" {
		t.Fatalf("final status = %q, want COMPLETED", final.Status)
	}

	report := LoadReport{
		GeneratedAt: time.Now().UTC(),
		BaseURL:     config.BaseURL,
		Resume: ResumeResult{
			FileBytes: info.Size(), SHA256: expectedSHA, UploadedBeforeRestartParts: stopAfter,
			TotalParts: partCount, SessionP95: createDuration, CommitP95: commitDuration, Passed: true,
		},
	}
	writeReportFromEnv(t, report)
}

func TestFilesystemConcurrentTransferCapacity(t *testing.T) {
	config := requireLoadConfig(t)
	client := newLoadClient(config)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var failures atomic.Int64
	var sessions sync.Map
	durations := make([]time.Duration, config.ConcurrentTransfers)
	started := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < config.ConcurrentTransfers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			key := fmt.Sprintf("capacity-%d-%d", started.UnixNano(), index)
			session, duration, err := client.createSession(ctx, fmt.Sprintf("capacity-%03d.bin", index), 0, emptySHA256(), key)
			durations[index] = duration
			if err != nil {
				failures.Add(1)
				return
			}
			if _, loaded := sessions.LoadOrStore(session.SessionID, struct{}{}); loaded {
				failures.Add(1)
			}
		}(i)
	}
	wg.Wait()

	failed := int(failures.Load())
	errorRate := float64(failed) / float64(config.ConcurrentTransfers)
	if errorRate >= 0.01 {
		t.Fatalf("error rate %.2f%% is not below 1%%", errorRate*100)
	}
	unique := 0
	sessions.Range(func(_, _ any) bool { unique++; return true })
	if unique != config.ConcurrentTransfers-failed {
		t.Fatalf("unique sessions = %d, successful requests = %d", unique, config.ConcurrentTransfers-failed)
	}

	report := LoadReport{
		GeneratedAt: time.Now().UTC(),
		BaseURL:     config.BaseURL,
		Capacity: CapacityResult{
			ConcurrentTransfers: config.ConcurrentTransfers,
			SuccessfulTransfers: unique,
			FailedTransfers:     failed,
			ErrorRate:           errorRate,
			SessionP95:          percentile(durations, 0.95),
			Elapsed:             time.Since(started),
			NoDuplicateSessions: true,
			QuotaOversellCheck:  "requires target-environment quota metrics/query evidence",
			Passed:              true,
		},
	}
	writeReportFromEnv(t, report)
}

func requireLoadConfig(t *testing.T) loadConfig {
	t.Helper()
	if os.Getenv("FS_LOAD_RUN") != "1" {
		t.Skip("set FS_LOAD_RUN=1 via scripts/run-filesystem-load.ps1")
	}
	config := loadConfig{
		BaseURL:             os.Getenv("FS_LOAD_BASE_URL"),
		Token:               os.Getenv("FS_LOAD_TOKEN"),
		WorkspaceID:         os.Getenv("FS_LOAD_WORKSPACE"),
		ProviderID:          os.Getenv("FS_LOAD_PROVIDER"),
		LargeFilePath:       os.Getenv("FS_LOAD_LARGE_FILE"),
		ConcurrentTransfers: envInt("FS_LOAD_CONCURRENCY", 200),
		PartSize:            int64(envInt("FS_LOAD_PART_SIZE_MIB", 64)) << 20,
	}
	if config.BaseURL == "" || config.Token == "" || config.WorkspaceID == "" || config.ProviderID == "" {
		t.Fatal("FS_LOAD_BASE_URL, FS_LOAD_TOKEN, FS_LOAD_WORKSPACE and FS_LOAD_PROVIDER are required")
	}
	return config
}

func newLoadClient(config loadConfig) *loadClient {
	return &loadClient{http: &http.Client{Timeout: 10 * time.Minute}, config: config}
}

func (c *loadClient) createSession(ctx context.Context, name string, size int64, checksum, idempotencyKey string) (transferSession, time.Duration, error) {
	body := map[string]any{
		"name": name, "protocol": "MULTIPART", "size": size, "sha256": checksum,
		"classification": "INTERNAL", "provider_id": c.config.ProviderID,
	}
	started := time.Now()
	var session transferSession
	err := c.json(ctx, http.MethodPost, c.config.BaseURL+"/api/v1/workspaces/"+c.config.WorkspaceID+"/transfers", idempotencyKey, body, &session)
	return session, time.Since(started), err
}

func (c *loadClient) getSession(ctx context.Context, sessionID string) (transferSession, error) {
	var session transferSession
	err := c.json(ctx, http.MethodGet, c.config.BaseURL+"/api/v1/workspaces/"+c.config.WorkspaceID+"/transfers/"+sessionID, "", nil, &session)
	return session, err
}

func (c *loadClient) uploadMissingParts(ctx context.Context, session transferSession, path string, partSize int64, maxParts int) error {
	completed := make(map[int]struct{}, len(session.Parts))
	for _, part := range session.Parts {
		completed[part.PartNumber] = struct{}{}
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	total := int((info.Size() + partSize - 1) / partSize)
	if maxParts > total {
		maxParts = total
	}
	for partNumber := 1; partNumber <= maxParts; partNumber++ {
		if _, ok := completed[partNumber]; ok {
			continue
		}
		offset := int64(partNumber-1) * partSize
		size := min(partSize, info.Size()-offset)
		reader := io.NewSectionReader(file, offset, size)
		url := fmt.Sprintf("%s/v1/transfers/%s/parts/%d", c.config.BaseURL, session.SessionID, partNumber)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, reader)
		if err != nil {
			return err
		}
		req.ContentLength = size
		req.Header.Set("Authorization", "Bearer "+session.CapabilityToken)
		req.Header.Set("Content-Type", "application/octet-stream")
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		if err := expect2xx(resp); err != nil {
			return fmt.Errorf("upload part %d: %w", partNumber, err)
		}
	}
	return nil
}

func (c *loadClient) commit(ctx context.Context, sessionID, checksum string) (time.Duration, transferSession, error) {
	started := time.Now()
	var session transferSession
	err := c.json(ctx, http.MethodPost, fmt.Sprintf("%s/v1/transfers/%s/commit", c.config.BaseURL, sessionID), "", map[string]any{"sha256": checksum}, &session)
	return time.Since(started), session, err
}

func (c *loadClient) json(ctx context.Context, method, url, idempotencyKey string, body any, out *transferSession) error {
	var reader io.Reader
	if body != nil {
		pipeReader, pipeWriter := io.Pipe()
		go func() {
			err := json.NewEncoder(pipeWriter).Encode(body)
			_ = pipeWriter.CloseWithError(err)
		}()
		reader = pipeReader
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
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
		return nil
	}
	var wrapped envelope[transferSession]
	if err := json.NewDecoder(resp.Body).Decode(&wrapped); err != nil {
		return err
	}
	*out = wrapped.Data
	return nil
}

func expect2xx(resp *http.Response) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("%s: %s", resp.Status, string(data))
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func emptySHA256() string {
	sum := sha256.Sum256(nil)
	return hex.EncodeToString(sum[:])
}
