package filesystem

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/operation"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

type OperationFactory struct {
	Replicator *operation.ReplicateBlobHandler
	Promoter   *command.PromoteReplicaHandler
}

func (f OperationFactory) Validate() error {
	switch {
	case f.Replicator == nil:
		return fmt.Errorf("filesystem replicate operation handler is required")
	case f.Promoter == nil:
		return fmt.Errorf("filesystem promote replica handler is required")
	default:
		return nil
	}
}

func (f OperationFactory) Descriptors() ([]messaging.OperationDescriptor, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return []messaging.OperationDescriptor{
		{Type: operation.OperationTypeReplicateBlob, Handler: f.Replicator.Handle},
	}, nil
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	descriptors, err := f.Descriptors()
	if err != nil {
		return err
	}
	return messaging.RegisterOperationDescriptors(registry, descriptors)
}
