package filesystem

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/operation"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/stretchr/testify/require"
)

func TestOperationFactoryRegistersFilesystemOperations(t *testing.T) {
	factory := OperationFactory{
		Replicator:       operation.NewReplicateBlobHandler(nil, nil, nil, operation.DefaultReplicationConfig()),
		Promoter:         command.NewPromoteReplicaHandler(nil, nil),
		LifecycleRunner:  operation.NewRunLifecycleHandler(nil, service.NewLifecycleEngine()),
		GarbageCollector: operation.NewGarbageCollector(nil, nil, operation.DefaultGarbageCollectConfig()),
	}
	require.NoError(t, factory.Validate())

	registry := messaging.NewOperationRegistry()
	require.NoError(t, factory.Register(registry))

	for _, opType := range []string{
		operation.OperationTypeReplicateBlob,
		operation.OperationTypeRunLifecycle,
		operation.OperationTypeGarbageCollect,
	} {
		handler, ok := registry.Handler(opType)
		require.True(t, ok, opType)
		require.NotNil(t, handler)
	}
}

func TestOperationFactoryRejectsMissingDependency(t *testing.T) {
	require.Error(t, (OperationFactory{}).Validate())
}
