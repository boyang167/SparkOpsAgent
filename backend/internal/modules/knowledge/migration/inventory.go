package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/knowledge/domain/repository"
)

type InventoryReport struct {
	TotalDocuments       int64               `json:"total_documents"`
	EmptyContent         int64               `json:"empty_content"`
	TotalBytes           int64               `json:"total_bytes"`
	MaxDocumentBytes     int64               `json:"max_document_bytes"`
	EncodingAnomalies    int64               `json:"encoding_anomalies"`
	DuplicateSHA256      int64               `json:"duplicate_sha256"`
	MissingWorkspace     int64               `json:"missing_workspace"`
	EstimatedBlobCount   int64               `json:"estimated_blob_count"`
	DuplicateSHA256Items []string            `json:"duplicate_sha256_items,omitempty"`
}

type InventoryRunner struct {
	repo repository.KnowledgeBaseRepository
}

func NewInventoryRunner(repo repository.KnowledgeBaseRepository) *InventoryRunner {
	return &InventoryRunner{repo: repo}
}

func (r *InventoryRunner) Run(ctx context.Context) (InventoryReport, error) {
	docs, err := r.repo.ListDocumentsForMigration(ctx, nil, 1_000_000)
	if err != nil {
		return InventoryReport{}, err
	}
	report := InventoryReport{}
	shaCounts := map[string]int64{}
	for _, doc := range docs {
		report.TotalDocuments++
		content := doc.Content
		n := int64(len(content))
		report.TotalBytes += n
		if n == 0 {
			report.EmptyContent++
		}
		if n > report.MaxDocumentBytes {
			report.MaxDocumentBytes = n
		}
		sha := SHA256Hex([]byte(content))
		shaCounts[sha]++
		if _, err := r.repo.FindWorkspaceIDForDocument(ctx, doc.ID); err != nil {
			report.MissingWorkspace++
		}
	}
	unique := int64(len(shaCounts))
	report.EstimatedBlobCount = unique
	for sha, count := range shaCounts {
		if count > 1 {
			report.DuplicateSHA256 += count - 1
			report.DuplicateSHA256Items = append(report.DuplicateSHA256Items, sha)
		}
	}
	sort.Strings(report.DuplicateSHA256Items)
	return report, nil
}

func (r *InventoryRunner) PrintJSON(ctx context.Context) error {
	report, err := r.Run(ctx)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(payload))
	return nil
}
