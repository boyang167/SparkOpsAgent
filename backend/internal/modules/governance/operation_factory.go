package governance

import (
	"fmt"

	governanceoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/governance/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

type OperationFactory struct {
	RoleSync governanceoutbound.MemberRoleSyncer
}

func (f OperationFactory) Validate() error {
	if f.RoleSync == nil {
		return fmt.Errorf("governance operation role sync is required")
	}
	return nil
}

func (f OperationFactory) Descriptors() ([]messaging.OperationDescriptor, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return []messaging.OperationDescriptor{
		{
			Type:    governanceoutbound.OperationTypeMemberRoleSync,
			Handler: governanceoutbound.NewMemberRoleSyncHandler(f.RoleSync),
		},
	}, nil
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	descriptors, err := f.Descriptors()
	if err != nil {
		return err
	}
	return messaging.RegisterOperationDescriptors(registry, descriptors)
}
