package governance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOperationFactoryRejectsMissingDependency(t *testing.T) {
	err := (OperationFactory{}).Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "role sync")
}
