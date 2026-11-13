//go:build integration

package security_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	testenv "github.com/sparkops/sparkops-agent/backend/tests/filesystem/integration"
)

func TestWorkspaceCannotReadAnotherOrganizationsBlob(t *testing.T) {
	env := testenv.NewTestEnv(t)
	file := env.UploadAs(t, env.OrganizationA.User, []byte("secret"))

	response := env.DownloadAs(t, env.OrganizationB.User, file.ID)

	require.Equal(t, http.StatusForbidden, response.StatusCode)
	require.NotContains(t, string(response.Body), "secret")
}
