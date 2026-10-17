package jenkins

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	gjenkins "github.com/bndr/gojenkins"
	"github.com/stretchr/testify/require"
)

func TestExecutorDisableJobPostsDisable(t *testing.T) {
	var disabled atomic.Bool
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/crumbIssuer"):
			http.NotFound(w, r)
		case r.Method == http.MethodGet && path == "/job/team-a/job/orders/api/json":
			_, _ = io.WriteString(w, `{"name":"orders","url":"`+server.URL+`/job/team-a/job/orders/"}`)
		case r.Method == http.MethodPost && path == "/job/team-a/job/orders/disable":
			disabled.Store(true)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	executor := &Executor{client: gjenkins.CreateJenkins(server.Client(), server.URL)}

	require.NoError(t, executor.DisableJob(context.Background(), "team-a", "orders"))
	require.True(t, disabled.Load())
}

func TestExecutorDisableJobTreatsMissingJobAsDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			t.Errorf("unexpected POST %s", r.URL.Path)
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	executor := &Executor{client: gjenkins.CreateJenkins(server.Client(), server.URL)}

	require.NoError(t, executor.DisableJob(context.Background(), "team-a", "orders"))
}

func TestExecutorReportsDisabledJobState(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/crumbIssuer"):
			http.NotFound(w, r)
		case r.Method == http.MethodGet && path == "/job/team-a/job/orders/api/json":
			_, _ = io.WriteString(w, `{"name":"orders","url":"`+server.URL+`/job/team-a/job/orders/","buildable":false}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	executor := &Executor{client: gjenkins.CreateJenkins(server.Client(), server.URL)}

	disabled, err := executor.IsJobDisabled(context.Background(), "team-a", "orders")

	require.NoError(t, err)
	require.True(t, disabled)
}
