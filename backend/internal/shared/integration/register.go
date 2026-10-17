package integration

import "github.com/sparkops/sparkops-agent/backend/internal/shared/events"

func Register(registry *events.Registry) error {
	if err := events.RegisterJSON[WorkspaceCreated](
		registry,
		EventWorkspaceCreated,
		1,
		func(ev WorkspaceCreated) events.AggregateRef {
			return events.AggregateRef{Type: "workspace", ID: ev.WorkspaceID.String()}
		},
	); err != nil {
		return err
	}
	return events.RegisterJSON[SecretUpdated](
		registry,
		EventSecretUpdated,
		1,
		func(ev SecretUpdated) events.AggregateRef {
			return events.AggregateRef{Type: "secret", ID: ev.SecretID.String()}
		},
	)
}
