package performance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type LoadReport struct {
	GeneratedAt time.Time      `json:"generated_at"`
	BaseURL     string         `json:"base_url"`
	Resume      ResumeResult   `json:"resume"`
	Capacity    CapacityResult `json:"capacity"`
	Metadata    MetadataResult `json:"metadata"`
}

type ResumeResult struct {
	FileBytes                  int64         `json:"file_bytes"`
	SHA256                     string        `json:"sha256"`
	UploadedBeforeRestartParts int           `json:"uploaded_before_restart_parts"`
	TotalParts                 int           `json:"total_parts"`
	SessionP95                 time.Duration `json:"session_p95_ns"`
	CommitP95                  time.Duration `json:"commit_p95_ns"`
	Passed                     bool          `json:"passed"`
}

type CapacityResult struct {
	ConcurrentTransfers int           `json:"concurrent_transfers"`
	SuccessfulTransfers int           `json:"successful_transfers"`
	FailedTransfers     int           `json:"failed_transfers"`
	ErrorRate           float64       `json:"error_rate"`
	SessionP95          time.Duration `json:"session_p95_ns"`
	Elapsed             time.Duration `json:"elapsed_ns"`
	NoDuplicateSessions bool          `json:"no_duplicate_sessions"`
	QuotaOversellCheck  string        `json:"quota_oversell_check"`
	Passed              bool          `json:"passed"`
}

type MetadataResult struct {
	SimulatedBytes int64         `json:"simulated_bytes"`
	RowsCreated    int           `json:"rows_created"`
	PageRequests   int           `json:"page_requests"`
	ListP95        time.Duration `json:"list_p95_ns"`
	Passed         bool          `json:"passed"`
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	filtered := make([]time.Duration, 0, len(values))
	for _, value := range values {
		if value > 0 {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return 0
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i] < filtered[j] })
	rank := int(float64(len(filtered))*quantile + 0.999999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(filtered) {
		rank = len(filtered)
	}
	return filtered[rank-1]
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func writeReportFromEnv(t *testing.T, report LoadReport) {
	t.Helper()
	output := os.Getenv("FS_LOAD_REPORT")
	if output == "" {
		output = filepath.Join("..", "..", "..", "artifacts", "filesystem-load-report")
	}
	if err := writeReport(output, report); err != nil {
		t.Fatalf("write load report: %v", err)
	}
}

func writeReport(outputBase string, report LoadReport) error {
	if err := os.MkdirAll(filepath.Dir(outputBase), 0o755); err != nil {
		return err
	}
	if existingRaw, err := os.ReadFile(outputBase + ".json"); err == nil {
		var existing LoadReport
		if json.Unmarshal(existingRaw, &existing) == nil {
			if report.Resume.TotalParts == 0 {
				report.Resume = existing.Resume
			}
			if report.Capacity.ConcurrentTransfers == 0 {
				report.Capacity = existing.Capacity
			}
			if report.Metadata.PageRequests == 0 {
				report.Metadata = existing.Metadata
			}
		}
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputBase+".json", append(raw, '\n'), 0o600); err != nil {
		return err
	}
	markdown := fmt.Sprintf(`# Filesystem Load Report

- Generated: %s
- Target: %s

## 20 GiB resume

- Passed: %t
- SHA-256: %s
- Parts before restart / total: %d / %d
- Session P95: %s
- Commit P95: %s

## Transfer capacity

- Passed: %t
- Concurrent transfers: %d
- Successful / failed: %d / %d
- Error rate: %.2f%%
- Session P95: %s
- Duplicate sessions: %t
- Quota oversell evidence: %s

## Metadata load

- Passed: %t
- Simulated logical capacity: %d bytes
- Rows created: %d
- List requests: %d
- List P95: %s
`,
		report.GeneratedAt.Format(time.RFC3339), report.BaseURL,
		report.Resume.Passed, report.Resume.SHA256, report.Resume.UploadedBeforeRestartParts,
		report.Resume.TotalParts, report.Resume.SessionP95, report.Resume.CommitP95,
		report.Capacity.Passed, report.Capacity.ConcurrentTransfers, report.Capacity.SuccessfulTransfers,
		report.Capacity.FailedTransfers, report.Capacity.ErrorRate*100, report.Capacity.SessionP95,
		!report.Capacity.NoDuplicateSessions, report.Capacity.QuotaOversellCheck,
		report.Metadata.Passed, report.Metadata.SimulatedBytes, report.Metadata.RowsCreated,
		report.Metadata.PageRequests, report.Metadata.ListP95,
	)
	return os.WriteFile(outputBase+".md", []byte(markdown), 0o600)
}
