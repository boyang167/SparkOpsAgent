package plugin

import (
	"context"
	"encoding/json"
)

type PluginAuditRecord struct {
	Action      string
	WorkspaceID string
	MemberID    string
	DetailJSON  string
}

type PluginAuditSink interface {
	CreatePluginAudit(ctx context.Context, record PluginAuditRecord) error
}

type operationAuditLogger struct{ sink PluginAuditSink }

func NewOperationAuditLogger(sink PluginAuditSink) AuditLogger {
	return &operationAuditLogger{sink: sink}
}

var safeAuditDetailKeys = map[string]struct{}{
	"cluster_id": {}, "namespace": {}, "pod": {}, "container": {},
	"status": {}, "model": {}, "record_id": {}, "diagnosis_id": {},
}

func (l *operationAuditLogger) Record(ctx context.Context, event AuditEvent) error {
	safe := make(map[string]any)
	for key, value := range event.Detail {
		if _, ok := safeAuditDetailKeys[key]; ok {
			safe[key] = value
		}
	}
	detail, err := json.Marshal(safe)
	if err != nil {
		return err
	}
	return l.sink.CreatePluginAudit(ctx, PluginAuditRecord{
		Action: event.Action, WorkspaceID: event.WorkspaceID,
		MemberID: event.MemberID, DetailJSON: string(detail),
	})
}
