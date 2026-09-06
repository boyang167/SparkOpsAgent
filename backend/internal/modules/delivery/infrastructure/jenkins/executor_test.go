package jenkins

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gjenkins "github.com/bndr/gojenkins"
	"github.com/stretchr/testify/require"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
)

func TestJobXMLContainsPipelineAndParameters(t *testing.T) {
	script := `pipeline { stages { stage("build < push & publish") {} } }`

	config := renderJobXML(port.JenkinsJobSpec{
		Folder:         "team-a",
		Job:            "orders",
		PipelineScript: script,
	})

	require.Contains(t, config, `<definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition"`)
	for _, name := range []string{"BRANCH", "CodeVersion", "IMAGE_TAG", "IMAGE_REPOSITORY"} {
		require.Contains(t, config, "<name>"+name+"</name>")
	}
	require.NotContains(t, config, "build < push & publish")
	require.Contains(t, config, "build &lt; push &amp; publish")

	var parsed struct {
		Definition struct {
			Script string `xml:"script"`
		} `xml:"definition"`
	}
	require.NoError(t, xml.Unmarshal([]byte(config), &parsed))
	require.Equal(t, script, parsed.Definition.Script)
}

func TestBuildParamsCarryImageCoordinates(t *testing.T) {
	params := buildParams(port.BuildRequest{
		Namespace:       "team-a",
		ProjectName:     "orders",
		GitRefValue:     "main",
		ImageTag:        "550e8400e29b",
		ImageRepository: "registry.example/team/orders",
		RollbackOf:      42,
	})

	require.Equal(t, "main", params["BRANCH"])
	require.Equal(t, "main", params["CodeVersion"])
	require.Equal(t, "550e8400e29b", params["IMAGE_TAG"])
	require.Equal(t, "registry.example/team/orders", params["IMAGE_REPOSITORY"])
	require.Equal(t, "42", params["ROLLBACK_OF"])
}

func TestBuildParamsDeploymentAlwaysCarriesFourRequiredParameters(t *testing.T) {
	params := buildParams(port.BuildRequest{
		Namespace: "team-a", ProjectName: "orders", GitRefValue: "main",
	})

	require.Contains(t, params, "BRANCH")
	require.Contains(t, params, "CodeVersion")
	require.Contains(t, params, "IMAGE_TAG")
	require.Contains(t, params, "IMAGE_REPOSITORY")
}

func TestBuildParamsOrdinaryBuildOnlyCarriesLegacyBranch(t *testing.T) {
	params := buildParams(port.BuildRequest{Job: "orders-build", GitRefValue: "main"})

	require.Equal(t, map[string]string{"BRANCH": "main"}, params)
}

func TestCreateOrUpdateCreatesFolderThenCreatesAndUpdatesJobAndCredential(t *testing.T) {
	const credentialID = "sparkops-workspace-secret"
	var folderExists, jobExists, credentialExists bool
	var createdJobXML, updatedJobXML, createdCredentialXML, updatedCredentialXML string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/crumbIssuer/api/json"):
			http.NotFound(w, r)
		case r.Method == http.MethodGet && strings.TrimSuffix(r.URL.Path, "/") == "/job/team-a/api/json":
			if !folderExists {
				http.NotFound(w, r)
				return
			}
			_, _ = io.WriteString(w, `{"name":"team-a"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/createItem":
			folderExists = true
		case r.Method == http.MethodGet && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/api/json/":
			if credentialExists {
				_, _ = io.WriteString(w, `{"credentials":[{"id":"`+credentialID+`"}]}`)
			} else {
				_, _ = io.WriteString(w, `{"credentials":[]}`)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/createCredentials":
			createdCredentialXML = readRequestBody(t, r)
			credentialExists = true
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/credential/"+credentialID+"/config.xml":
			updatedCredentialXML = readRequestBody(t, r)
		case r.Method == http.MethodGet && strings.TrimSuffix(r.URL.Path, "/") == "/job/team-a/job/orders/api/json":
			if !jobExists {
				http.NotFound(w, r)
				return
			}
			_, _ = io.WriteString(w, `{"name":"orders"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/createItem":
			createdJobXML = readRequestBody(t, r)
			jobExists = true
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/job/orders/config.xml":
			updatedJobXML = readRequestBody(t, r)
		default:
			t.Errorf("unexpected Jenkins request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	executor := &Executor{client: gjenkins.CreateJenkins(server.Client(), server.URL)}
	spec := port.JenkinsJobSpec{
		Folder: "team-a", Job: "orders", PipelineScript: "pipeline { echo 'first' }",
		Credentials: []port.JenkinsCredential{{
			ID: credentialID, Username: "robot", Secret: "first-secret", Kind: "username-password",
		}},
	}

	require.NoError(t, executor.CreateOrUpdate(context.Background(), spec))
	spec.PipelineScript = "pipeline { echo 'updated' }"
	spec.Credentials[0].Secret = "updated-secret"
	require.NoError(t, executor.CreateOrUpdate(context.Background(), spec))

	require.Contains(t, createdJobXML, "echo &#39;first&#39;")
	require.Contains(t, updatedJobXML, "echo &#39;updated&#39;")
	require.Contains(t, createdCredentialXML, "<id>"+credentialID+"</id>")
	require.Contains(t, createdCredentialXML, "<password>first-secret</password>")
	require.Contains(t, updatedCredentialXML, "<password>updated-secret</password>")
}

func TestSyncCredentialsCreatesAndUpdatesCredential(t *testing.T) {
	const credentialID = "sparkops-workspace-secret"
	var folderExists, credentialExists bool
	var createdCredentialXML, updatedCredentialXML string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/job/team-a/api/json/":
			if folderExists {
				_, _ = io.WriteString(w, `{"name":"team-a"}`)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/createItem":
			folderExists = true
		case r.Method == http.MethodGet && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/api/json/":
			if credentialExists {
				_, _ = io.WriteString(w, `{"credentials":[{"id":"`+credentialID+`"}]}`)
			} else {
				_, _ = io.WriteString(w, `{"credentials":[]}`)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/createCredentials":
			createdCredentialXML = readRequestBody(t, r)
			credentialExists = true
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/credential/"+credentialID+"/config.xml":
			updatedCredentialXML = readRequestBody(t, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	executor := &Executor{client: gjenkins.CreateJenkins(server.Client(), server.URL)}
	credentials := []port.JenkinsCredential{{
		ID: credentialID, Username: "robot", Secret: "first-secret", Kind: "username-password",
	}}
	require.NoError(t, executor.SyncCredentials(context.Background(), "team-a", credentials))
	credentials[0].Secret = "updated-secret"
	require.NoError(t, executor.SyncCredentials(context.Background(), "team-a", credentials))
	require.Contains(t, createdCredentialXML, "<password>first-secret</password>")
	require.Contains(t, updatedCredentialXML, "<password>updated-secret</password>")
}

func TestCreateOrUpdateRecoversFromConcurrentFolderJobAndCredentialCreates(t *testing.T) {
	const credentialID = "sparkops-workspace-secret"
	var folderExists, jobExists, credentialExists bool
	var updatedJobXML, updatedCredentialXML string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/crumbIssuer/api/json"):
			http.NotFound(w, r)
		case r.Method == http.MethodGet && strings.TrimSuffix(r.URL.Path, "/") == "/job/team-a/api/json":
			if !folderExists {
				http.NotFound(w, r)
				return
			}
			_, _ = io.WriteString(w, `{"name":"team-a"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/createItem":
			folderExists = true
			w.WriteHeader(http.StatusConflict)
		case r.Method == http.MethodGet && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/api/json/":
			if credentialExists {
				_, _ = io.WriteString(w, `{"credentials":[{"id":"`+credentialID+`"}]}`)
			} else {
				_, _ = io.WriteString(w, `{"credentials":[]}`)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/createCredentials":
			credentialExists = true
			w.WriteHeader(http.StatusConflict)
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/credentials/store/folder/domain/_/credential/"+credentialID+"/config.xml":
			updatedCredentialXML = readRequestBody(t, r)
		case r.Method == http.MethodGet && strings.TrimSuffix(r.URL.Path, "/") == "/job/team-a/job/orders/api/json":
			if !jobExists {
				http.NotFound(w, r)
				return
			}
			_, _ = io.WriteString(w, `{"name":"orders"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/createItem":
			jobExists = true
			w.WriteHeader(http.StatusConflict)
		case r.Method == http.MethodPost && r.URL.Path == "/job/team-a/job/orders/config.xml":
			updatedJobXML = readRequestBody(t, r)
		default:
			t.Errorf("unexpected Jenkins request: %s %s", r.Method, r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	executor := &Executor{client: gjenkins.CreateJenkins(server.Client(), server.URL)}

	err := executor.CreateOrUpdate(context.Background(), port.JenkinsJobSpec{
		Folder: "team-a", Job: "orders", PipelineScript: "pipeline { echo 'winner config' }",
		Credentials: []port.JenkinsCredential{{
			ID: credentialID, Username: "robot", Secret: "winner-secret", Kind: "username-password",
		}},
	})

	require.NoError(t, err)
	require.Contains(t, updatedJobXML, "winner config")
	require.Contains(t, updatedCredentialXML, "<password>winner-secret</password>")
}

func TestCreateOrUpdateDoesNotSwallowNonConflictCreateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/crumbIssuer/api/json"):
			http.NotFound(w, r)
		case r.Method == http.MethodGet:
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/createItem":
			http.Error(w, "failed", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	executor := &Executor{client: gjenkins.CreateJenkins(server.Client(), server.URL)}

	err := executor.CreateOrUpdate(context.Background(), port.JenkinsJobSpec{
		Folder: "team-a", Job: "orders", PipelineScript: "pipeline {}",
	})

	require.ErrorContains(t, err, "create jenkins folder")
}

func TestCredentialErrorsDoNotExposeSecret(t *testing.T) {
	_, err := jenkinsCredentialValue(port.JenkinsCredential{
		ID: "stable-id", Kind: "unsupported", Secret: "must-not-leak",
	})

	require.Error(t, err)
	require.NotContains(t, err.Error(), "must-not-leak")
}

func TestMockExecutorRecordsSpecsAndKeepsPredictableBuildOutput(t *testing.T) {
	executor := NewMockExecutor()
	spec := port.JenkinsJobSpec{Folder: "team-a", Job: "orders", PipelineScript: "pipeline {}"}
	request := port.BuildRequest{Folder: "team-a", Job: "orders", GitRefValue: "main", ImageTag: "tag-1"}

	require.NoError(t, executor.CreateOrUpdate(context.Background(), spec))
	output, err := executor.RunStreaming(context.Background(), kernel.NewBuildID(), request, nil)

	require.NoError(t, err)
	require.Equal(t, spec, executor.LastJobSpec)
	require.Equal(t, request, executor.LastBuildRequest)
	require.Contains(t, output, "[mock-jenkins] job=orders branch=main")
	require.Contains(t, output, "BUILD SUCCESS")
}

func TestRunStreamingEnforcesConfiguredTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	executor := &Executor{
		client: gjenkins.CreateJenkins(server.Client(), server.URL),
	}

	_, err := executor.RunStreaming(context.Background(), kernel.NewBuildID(), port.BuildRequest{
		Job: "orders", GitRefValue: "main", Timeout: 20 * time.Millisecond,
	}, nil)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRunStreamingReturnsBuildRefreshError(t *testing.T) {
	buildReads := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/job/orders/api/json":
			_, _ = io.WriteString(w, `{"name":"orders","url":"`+server.URL+`/job/orders"}`)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/job/orders/build"):
			w.Header().Set("Location", server.URL+"/queue/item/1/")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/queue/item/1/api/json":
			_, _ = io.WriteString(w, `{"id":1,"executable":{"number":1,"url":"`+server.URL+`/job/orders/1/"}}`)
		case r.Method == http.MethodGet && r.URL.Path == "/job/orders/1/api/json":
			buildReads++
			if buildReads > 4 {
				http.Error(w, "refresh failed", http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, `{"number":1,"building":true,"result":""}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/logText/progressiveText"):
			w.Header().Set("X-Text-Size", "0")
			_, _ = io.WriteString(w, "")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	executor := &Executor{client: gjenkins.CreateJenkins(server.Client(), server.URL)}

	_, err := executor.RunStreaming(context.Background(), kernel.NewBuildID(), port.BuildRequest{
		Job: "orders", GitRefValue: "main",
	}, nil)

	require.ErrorContains(t, err, "refresh jenkins build")
}

func readRequestBody(t *testing.T, r *http.Request) string {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	return strings.TrimSpace(string(body))
}
