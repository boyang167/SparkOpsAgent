package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/workspace_app/domain/model"
)

func TestListGitLabRefs_mergeRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/api/v4/projects/")
		require.Contains(t, r.URL.Path, "/merge_requests")
		require.Equal(t, "opened", r.URL.Query().Get("state"))
		_, _ = w.Write([]byte(`[
			{"iid":10,"title":"fix bug","source_branch":"fix-bug"},
			{"iid":8,"title":"Revert merge","source_branch":"revert-main"}
		]`))
	}))
	defer server.Close()

	items, err := ListGitLabRefs(
		context.Background(),
		"https://gitlab.example.com/group/demo.git",
		server.URL,
		model.SecretCredentials{Token: "token"},
		"pr",
	)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "10", items[0].Name)
	require.Equal(t, "fix bug", items[0].Title)
	require.Equal(t, "8", items[1].Name)
}
