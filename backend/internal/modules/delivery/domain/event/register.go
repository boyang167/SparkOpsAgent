package event

import "github.com/sparkops/sparkops-agent/backend/internal/shared/events"

func Register(registry *events.Registry) error {
	if err := events.RegisterJSON[BuildCompleted](
		registry,
		BuildCompletedName,
		1,
		func(ev BuildCompleted) events.AggregateRef {
			return events.AggregateRef{Type: "build", ID: ev.BuildID.String()}
		},
	); err != nil {
		return err
	}
	return events.RegisterJSON[DeployCompleted](
		registry,
		DeployCompletedName,
		1,
		func(ev DeployCompleted) events.AggregateRef {
			return events.AggregateRef{Type: "deployment_run", ID: ev.RunID.String()}
		},
	)
}
