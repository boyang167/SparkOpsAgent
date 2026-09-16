package keyval_test

import (
	"encoding/json"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/keyval"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/domain/model"
	"github.com/stretchr/testify/require"
)

func TestResolveSubstitutesKeyVals(t *testing.T) {
	def := json.RawMessage(`{"branch":"{{build_branch}}","env":"{{uat_env_id}}"}`)
	resolved, err := keyval.Resolve(def, []model.KeyVal{
		{Key: "build_branch", Value: "release/v2.3"},
		{Key: "uat_env_id", Value: "env-uuid-1"},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"branch":"release/v2.3","env":"env-uuid-1"}`, string(resolved))
}
