package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/platform/audit"
)

type auditListStub struct {
	filter audit.ListFilter
	rows   []audit.LogView
}

func (s *auditListStub) List(_ context.Context, filter audit.ListFilter) ([]audit.LogView, error) {
	s.filter = filter
	return s.rows, nil
}

func TestListFileAuditEventsScopesWorkspaceAndDecisionBeforePagination(t *testing.T) {
	workspaceID := uuid.New()
	store := &auditListStub{rows: []audit.LogView{{
		ID: "event-1", ActorID: uuid.NewString(), Action: "filesystem.node.rename",
		ResourceType: "filesystem", ResourceID: uuid.NewString(),
		Detail:    map[string]any{"workspace_id": workspaceID.String(), "decision": "DENY"},
		CreatedAt: "2026-11-14T04:00:00Z",
	}}}

	result, err := NewListFileAuditEventsHandler(store).Handle(context.Background(), ListFileAuditEvents{
		OrganizationID: uuid.New(),
		WorkspaceID:    workspaceID,
		Result:         "deny",
		Limit:          20,
	})

	require.NoError(t, err)
	require.Equal(t, workspaceID.String(), store.filter.WorkspaceID)
	require.Equal(t, "DENY", store.filter.Decision)
	require.Len(t, result.Items, 1)
	require.Equal(t, "DENY", result.Items[0].Result)
}

func TestListFileAuditEventsRecursivelyRedactsSensitiveDetail(t *testing.T) {
	workspaceID := uuid.New()
	store := &auditListStub{rows: []audit.LogView{{
		ID: "event-1", ActorID: uuid.NewString(), Action: "filesystem.capability.issued",
		ResourceType: "filesystem", ResourceID: uuid.NewString(),
		Detail: map[string]any{
			"workspace_id": workspaceID.String(),
			"token_hash":   "hash-value",
			"nested": map[string]any{
				"token":         "token-value",
				"client_secret": "secret-value",
				"credential":    "credential-value",
				"capability":    "capability-value",
				"safe":          "keep-me",
			},
			"items": []any{map[string]any{"secret": "nested-secret", "name": "visible"}},
		},
		CreatedAt: "2026-11-14T04:00:00Z",
	}}}

	result, err := NewListFileAuditEventsHandler(store).Handle(context.Background(), ListFileAuditEvents{
		OrganizationID: uuid.New(), WorkspaceID: workspaceID, Limit: 20,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	detail := result.Items[0].Detail
	require.Equal(t, workspaceID.String(), detail["workspace_id"])
	require.Equal(t, "[REDACTED]", detail["token_hash"])
	nested := detail["nested"].(map[string]any)
	require.Equal(t, "keep-me", nested["safe"])
	for _, key := range []string{"token", "client_secret", "credential", "capability"} {
		require.Equal(t, "[REDACTED]", nested[key])
	}
	item := detail["items"].([]any)[0].(map[string]any)
	require.Equal(t, "[REDACTED]", item["secret"])
	require.Equal(t, "visible", item["name"])
}

func TestListFileAuditEventsUsesNormalizedExactSensitiveKeysWithoutHidingMetadata(t *testing.T) {
	workspaceID := uuid.New()
	sensitive := map[string]any{
		"Token": "x", "TOKEN-HASH": "x", "password": "x", "clientSecret": "x",
		"apiKey": "x", "X-Access-Key": "x", "privateKey": "x",
		"Authorization": "x", "session-cookie": "x", "databaseCredential": "x",
	}
	safe := map[string]any{
		"token_count": 7, "tokenizer": "bpe", "credential_id": "credential-1",
		"access_key_id": "public-id", "private_key_algorithm": "ed25519",
	}
	detail := map[string]any{
		"workspace_id": workspaceID.String(),
		"sensitive":    sensitive,
		"safe":         safe,
		"nested": []any{
			map[string]any{"API_KEY": "x", "token_count": 2},
		},
	}
	store := &auditListStub{rows: []audit.LogView{{
		ID: "event-2", ActorID: uuid.NewString(), Action: "filesystem.capability.issued",
		ResourceType: "filesystem", ResourceID: uuid.NewString(), Detail: detail,
		CreatedAt: "2026-11-14T04:00:00Z",
	}}}

	result, err := NewListFileAuditEventsHandler(store).Handle(context.Background(), ListFileAuditEvents{
		OrganizationID: uuid.New(), WorkspaceID: workspaceID, Limit: 20,
	})

	require.NoError(t, err)
	actualSensitive := result.Items[0].Detail["sensitive"].(map[string]any)
	for key := range sensitive {
		require.Equal(t, "[REDACTED]", actualSensitive[key], key)
	}
	require.Equal(t, safe, result.Items[0].Detail["safe"])
	nested := result.Items[0].Detail["nested"].([]any)[0].(map[string]any)
	require.Equal(t, "[REDACTED]", nested["API_KEY"])
	require.Equal(t, 2, nested["token_count"])
}
