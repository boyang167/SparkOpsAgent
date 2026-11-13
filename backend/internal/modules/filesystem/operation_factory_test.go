package filesystem

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/application/operation"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/messaging"
	"github.com/stretchr/testify/require"
)

func TestOperationFactoryRegistersReplicateBlobOperation(t *testing.T) {
	factory := OperationFactory{
		Replicator: operation.NewReplicateBlobHandler(nil, nil, nil, operation.DefaultReplicationConfig()),
		Promoter:   command.NewPromoteReplicaHandler(nil, nil),
	}
	require.NoError(t, factory.Validate())

	registry := messaging.NewOperationRegistry()
	require.NoError(t, factory.Register(registry))

	handler, ok := registry.Handler(operation.OperationTypeReplicateBlob)
	require.True(t, ok)
	require.NotNil(t, handler)
}

func TestOperationFactoryRejectsMissingDependency(t *testing.T) {
	require.Error(t, (OperationFactory{}).Validate())
}
