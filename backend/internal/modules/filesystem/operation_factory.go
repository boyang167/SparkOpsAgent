package filesystem

import (
	"fmt"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/operation"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
)

type OperationFactory struct {
	Replicator       *operation.ReplicateBlobHandler
	Promoter         *command.PromoteReplicaHandler
	LifecycleRunner  *operation.RunLifecycleHandler
	GarbageCollector *operation.GarbageCollector
}

func (f OperationFactory) Validate() error {
	switch {
	case f.Replicator == nil:
		return fmt.Errorf("filesystem replicate operation handler is required")
	case f.Promoter == nil:
		return fmt.Errorf("filesystem promote replica handler is required")
	case f.LifecycleRunner == nil:
		return fmt.Errorf("filesystem lifecycle operation handler is required")
	case f.GarbageCollector == nil:
		return fmt.Errorf("filesystem garbage collect operation handler is required")
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
		{Type: operation.OperationTypeRunLifecycle, Handler: f.LifecycleRunner.Handle},
		{Type: operation.OperationTypeGarbageCollect, Handler: f.GarbageCollector.Handle},
	}, nil
}

func (f OperationFactory) Register(registry messaging.OperationRegistry) error {
	descriptors, err := f.Descriptors()
	if err != nil {
		return err
	}
	return messaging.RegisterOperationDescriptors(registry, descriptors)
}
