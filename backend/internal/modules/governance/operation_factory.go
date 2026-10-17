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

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if registry == nil {
		return fmt.Errorf("governance operation registry is required")
	}
	if err := registry.Register(
		governanceoutbound.OperationTypeMemberRoleSync,
		governanceoutbound.NewMemberRoleSyncHandler(f.RoleSync),
	); err != nil {
		return fmt.Errorf("register governance operation %s: %w", governanceoutbound.OperationTypeMemberRoleSync, err)
	}
	return nil
}
