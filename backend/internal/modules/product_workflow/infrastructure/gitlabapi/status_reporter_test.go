package gitlabapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/application/command"
	workflowoutbound "github.com/sparkops/sparkops-agent/backend/internal/modules/product_workflow/infrastructure/outbound"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

type statusLookup struct {
	project command.ProjectGit
	err     error
}

func (l statusLookup) LookupProjectGit(context.Context, kernel.ProjectID) (command.ProjectGit, error) {
	return l.project, l.err
}

func TestReporterQueriesCurrentContextStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "secret", r.Header.Get("PRIVATE-TOKEN"))
		require.Equal(t, "/api/v4/projects/group%2Frepo/repository/commits/abc/statuses", r.URL.EscapedPath())
		_, _ = w.Write([]byte(`[{"status":"success","name":"sparkops/workflow"}]`))
	}))
	defer server.Close()

	reporter := Reporter{Lookup: statusLookup{project: command.ProjectGit{
		GitURL: "https://git.example/group/repo.git", ServiceURL: server.URL, Token: "secret",
	}}}
	status, err := reporter.CurrentStatus(context.Background(), workflowoutbound.GitLabStatusPayload{
		ProjectID: kernel.NewProjectID().String(), Commit: "abc", Context: "sparkops/workflow",
	})

	require.NoError(t, err)
	require.Equal(t, "success", status)
}

func TestReporterPropagatesGitLabStatusFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`upstream unavailable`))
	}))
	defer server.Close()

	reporter := Reporter{Lookup: statusLookup{project: command.ProjectGit{
		GitURL: "https://git.example/group/repo.git", ServiceURL: server.URL, Token: "secret",
	}}}
	err := reporter.Report(context.Background(), workflowoutbound.GitLabStatusPayload{
		ProjectID: kernel.NewProjectID().String(), Commit: "abc", Context: "sparkops/workflow",
		DesiredStatus: "success", Description: "done",
	})

	require.Error(t, err)
}

func TestReporterBoundsCurrentAndSetStatusIO(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)
	reporter := Reporter{
		Lookup: statusLookup{project: command.ProjectGit{
			GitURL: "https://git.example/group/repo.git", ServiceURL: server.URL, Token: "secret",
		}},
		HTTP: &http.Client{Timeout: 25 * time.Millisecond},
	}
	payload := workflowoutbound.GitLabStatusPayload{
		ProjectID: kernel.NewProjectID().String(), Commit: "abc", Context: "sparkops/workflow",
		DesiredStatus: "success",
	}

	started := time.Now()
	_, err := reporter.CurrentStatus(context.Background(), payload)
	require.Error(t, err)
	require.Less(t, time.Since(started), time.Second)

	started = time.Now()
	err = reporter.Report(context.Background(), payload)
	require.Error(t, err)
	require.Less(t, time.Since(started), time.Second)
}
