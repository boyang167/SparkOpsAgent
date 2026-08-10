package model

import "github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"

type PodPhase string

const (
	PodPhaseRunning PodPhase = "Running"
	PodPhasePending PodPhase = "Pending"
)

type Pod struct {
	ID          kernel.PodID
	ProjectID   kernel.ProjectID
	Name        string
	Namespace   string
	Phase       PodPhase
	Node        string
	Restarts    int
	Container   string
	Image       string
	StartedAt   string
}
