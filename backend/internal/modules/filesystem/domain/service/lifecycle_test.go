package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/stretchr/testify/require"
)

func TestLifecycleSelectorMatchesScopeClassificationMimeAndSize(t *testing.T) {
	workspaceID := uuid.New()
	policy, err := model.NewLifecyclePolicy(model.LifecyclePolicyInput{
		ID: uuid.New(), OrganizationID: uuid.New(), Name: "default", Version: 1,
		Rules: map[string]any{
			"rules": []any{
				map[string]any{
					"id": "archive-images",
					"when": map[string]any{
						"scope":          "WORKSPACE",
						"classification": "INTERNAL",
						"mime_prefix":    "image/",
						"min_size":       float64(1024),
					},
					"actions": []any{"archive", "rescan"},
				},
			},
		},
	})
	require.NoError(t, err)

	engine := NewLifecycleEngine()
	decisions, err := engine.Evaluate(policy, LifecycleCandidate{
		ScopeType:      model.ScopeWorkspace,
		ScopeID:        workspaceID,
		Classification: model.TransferClassificationInternal,
		MIMEType:       "image/png",
		Size:           2048,
		CreatedAt:      time.Now().UTC().Add(-48 * time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	require.Equal(t, "archive-images", decisions[0].RuleID)
	require.Equal(t, []LifecycleAction{LifecycleActionArchive, LifecycleActionRescan}, decisions[0].Actions)
}

func TestLifecycleSelectorSkipsPurgeUnderLegalHold(t *testing.T) {
	policy, err := model.NewLifecyclePolicy(model.LifecyclePolicyInput{
		ID: uuid.New(), OrganizationID: uuid.New(), Name: "purge", Version: 1,
		Rules: map[string]any{
			"rules": []any{
				map[string]any{
					"id": "purge-old",
					"when": map[string]any{
						"max_age_hours": float64(1),
					},
					"actions": []any{"purge"},
				},
			},
		},
	})
	require.NoError(t, err)

	engine := NewLifecycleEngine()
	decisions, err := engine.Evaluate(policy, LifecycleCandidate{
		CreatedAt: time.Now().UTC().Add(-48 * time.Hour),
		LegalHold: true,
	})
	require.NoError(t, err)
	require.Empty(t, decisions)
}
