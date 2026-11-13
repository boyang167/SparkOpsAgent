//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironmentRoundTripsAuthorizedContent(t *testing.T) {
	env := NewTestEnv(t)
	file := env.UploadAs(t, env.OrganizationA.User, []byte("integration-content"))

	response := env.DownloadAs(t, env.OrganizationA.User, file.ID)

	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, []byte("integration-content"), response.Body)
}
