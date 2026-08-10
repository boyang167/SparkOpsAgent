package kernel_test

import (
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestWorkflowID_ParseAndString(t *testing.T) {
	id := kernel.NewWorkflowID()
	parsed, err := kernel.ParseWorkflowID(id.String())
	require.NoError(t, err)
	require.Equal(t, id, parsed)
}

func TestKnowledgeBaseID_ParseAndString(t *testing.T) {
	id := kernel.NewKnowledgeBaseID()
	parsed, err := kernel.ParseKnowledgeBaseID(id.String())
	require.NoError(t, err)
	require.Equal(t, id, parsed)
}

func TestDocumentID_ParseAndString(t *testing.T) {
	id := kernel.NewDocumentID()
	parsed, err := kernel.ParseDocumentID(id.String())
	require.NoError(t, err)
	require.Equal(t, id, parsed)
}
