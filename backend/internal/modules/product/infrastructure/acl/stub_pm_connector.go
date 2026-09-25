package acl

import (
	"context"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type stubPmConnector struct {
	provider string
}

func NewStubPmConnector(provider string) *stubPmConnector {
	return &stubPmConnector{provider: provider}
}

func (s *stubPmConnector) Provider() string { return s.provider }

func (s *stubPmConnector) GetIterationIssues(context.Context, kernel.WorkspaceID, string, string, []string) ([]port.PmIssue, error) {
	return nil, domain.ErrPmNotConfigured
}

func (s *stubPmConnector) GetIssue(context.Context, kernel.WorkspaceID, string, string) (*port.PmIssue, error) {
	return nil, domain.ErrPmNotConfigured
}

func (s *stubPmConnector) TransitionIssue(context.Context, kernel.WorkspaceID, string, string, string) error {
	return domain.ErrPmNotConfigured
}
