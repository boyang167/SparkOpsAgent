package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type auditSink struct{ record PluginAuditRecord }

func (s *auditSink) CreatePluginAudit(_ context.Context, record PluginAuditRecord) error {
	s.record = record
	return nil
}

func TestOperationAuditLoggerPersistsOnlySafeMetadata(t *testing.T) {
	sink := &auditSink{}
	logger := NewOperationAuditLogger(sink)
	err := logger.Record(context.Background(), AuditEvent{
		Action: "kubernetes.exec.started", WorkspaceID: "ws", MemberID: "member",
		Detail: map[string]any{
			"cluster_id": "cluster", "namespace": "default", "status": "started",
			"prompt": "secret prompt", "command": "cat token", "output": "secret output",
			"api_key": "secret",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "kubernetes.exec.started", sink.record.Action)
	require.Equal(t, "ws", sink.record.WorkspaceID)
	require.Equal(t, "member", sink.record.MemberID)
	require.JSONEq(t, `{"cluster_id":"cluster","namespace":"default","status":"started"}`, sink.record.DetailJSON)
}
