package http_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	pluginhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/httpx"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type installStub struct {
	raw   []byte
	actor kernel.MemberID
}

func (s *installStub) Handle(_ context.Context, raw []byte, actor kernel.MemberID) (*model.PluginRecord, error) {
	s.raw = append([]byte(nil), raw...)
	s.actor = actor
	return &model.PluginRecord{ID: "example", Status: model.PluginStatusPROpen}, nil
}

type listStub struct{}

func (listStub) Handle(context.Context) ([]*model.PluginRecord, error) { return nil, nil }

type getStub struct{}

func (getStub) Handle(context.Context, string) (*model.PluginRecord, error) { return nil, nil }

type statusStub struct{}

func (statusStub) Handle(context.Context, string) (*model.PluginRecord, error) { return nil, nil }

type uninstallStub struct{}

func (uninstallStub) Handle(context.Context, string, kernel.MemberID) (*model.PluginRecord, error) {
	return nil, nil
}

type workspaceListStub struct{}

func (workspaceListStub) Handle(context.Context, kernel.WorkspaceID) ([]query.WorkspacePlugin, error) {
	return []query.WorkspacePlugin{{
		PluginRecord: model.PluginRecord{ID: "example", Status: model.PluginStatusDeployed},
		Enabled:      true,
	}}, nil
}

type workspaceCommandStub struct {
	workspaceID kernel.WorkspaceID
	pluginID    string
	actor       kernel.MemberID
	enabled     bool
}

func (s *workspaceCommandStub) Handle(
	_ context.Context,
	workspaceID kernel.WorkspaceID,
	pluginID string,
	actor kernel.MemberID,
) (*model.WorkspaceBinding, error) {
	s.workspaceID, s.pluginID, s.actor = workspaceID, pluginID, actor
	return &model.WorkspaceBinding{
		WorkspaceID: workspaceID, PluginID: pluginID, Enabled: s.enabled, EnabledBy: actor,
	}, nil
}

func TestInstallAcceptsOnlyMultipartFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	install := &installStub{}
	handler := pluginhttp.NewHandler(install, listStub{}, getStub{}, statusStub{}, statusStub{}, uninstallStub{})
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret"})
	actor := kernel.NewMemberID()
	token, err := tokens.Issue(actor)
	require.NoError(t, err)
	engine := gin.New()
	engine.POST("/install", authn.RequireAuth(tokens), handler.Install)

	body, contentType := multipartBody(t, map[string]string{"file": "zip-content"}, nil)
	request := httptest.NewRequest(http.MethodPost, "/install", body)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"id":"example"`)
	require.NotContains(t, response.Body.String(), `"ID"`)
	require.Equal(t, []byte("zip-content"), install.raw)
	require.Equal(t, actor, install.actor)
}

func TestInstallRejectsMultipartFieldsOtherThanSingleFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := pluginhttp.NewHandler(&installStub{}, listStub{}, getStub{}, statusStub{}, statusStub{}, uninstallStub{})
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret"})
	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	engine := gin.New()
	engine.POST("/install", authn.RequireAuth(tokens), handler.Install)

	body, contentType := multipartBody(t, map[string]string{"file": "zip-content"}, map[string]string{"note": "not allowed"})
	request := httptest.NewRequest(http.MethodPost, "/install", body)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "INVALID_REQUEST")
}

func TestInstallUploadLimitsReserveMultipartOverhead(t *testing.T) {
	require.Equal(t, int64(50<<20), pluginhttp.MaxUploadBytes)
	require.Equal(t, int64(51<<20), pluginhttp.MaxRequestBytes)
}

func TestInstallRejectsArchiveOverFiftyMiBAndCleansTemporaryFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	install := &installStub{}
	handler := pluginhttp.NewHandler(install, listStub{}, getStub{}, statusStub{}, statusStub{}, uninstallStub{})
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret"})
	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	engine := gin.New()
	engine.POST("/install", authn.RequireAuth(tokens), handler.Install)
	tempDir := t.TempDir()
	t.Setenv("TMP", tempDir)
	t.Setenv("TEMP", tempDir)

	body, contentType := multipartBody(t, map[string]string{
		"file": strings.Repeat("x", int(pluginhttp.MaxUploadBytes)+1),
	}, nil)
	request := httptest.NewRequest(http.MethodPost, "/install", body)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.Empty(t, install.raw)
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestInstallRejectsMultipartRequestOverFiftyOneMiB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := pluginhttp.NewHandler(&installStub{}, listStub{}, getStub{}, statusStub{}, statusStub{}, uninstallStub{})
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret"})
	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	engine := gin.New()
	engine.POST("/install", authn.RequireAuth(tokens), handler.Install)

	body, contentType := multipartBody(t, map[string]string{
		"file": strings.Repeat("x", int(pluginhttp.MaxRequestBytes)+1),
	}, nil)
	request := httptest.NewRequest(http.MethodPost, "/install", body)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
}

func TestWorkspacePluginHandlersUseWorkspaceAndAuthenticatedActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	wsID, actor := kernel.NewWorkspaceID(), kernel.NewMemberID()
	enable := &workspaceCommandStub{enabled: true}
	disable := &workspaceCommandStub{}
	handler := pluginhttp.NewWorkspaceHandler(workspaceListStub{}, enable, disable)
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret"})
	token, err := tokens.Issue(actor)
	require.NoError(t, err)
	engine := gin.New()
	engine.GET("/workspaces/:wsId/plugins", authn.RequireAuth(tokens), httpx.RequireWorkspace(), handler.ListWorkspace)
	engine.POST("/workspaces/:wsId/plugins/:id/enable", authn.RequireAuth(tokens), httpx.RequireWorkspace(), handler.Enable)
	engine.POST("/workspaces/:wsId/plugins/:id/disable", authn.RequireAuth(tokens), httpx.RequireWorkspace(), handler.Disable)

	for _, requestCase := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/workspaces/" + wsID.String() + "/plugins"},
		{http.MethodPost, "/workspaces/" + wsID.String() + "/plugins/example/enable"},
		{http.MethodPost, "/workspaces/" + wsID.String() + "/plugins/example/disable"},
	} {
		request := httptest.NewRequest(requestCase.method, requestCase.path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}

	require.Equal(t, wsID, enable.workspaceID)
	require.Equal(t, "example", enable.pluginID)
	require.Equal(t, actor, enable.actor)
	require.Equal(t, wsID, disable.workspaceID)
	require.Equal(t, actor, disable.actor)
}

func multipartBody(t *testing.T, files map[string]string, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, content := range files {
		part, err := writer.CreateFormFile(name, "plugin.zip")
		require.NoError(t, err)
		_, err = part.Write([]byte(content))
		require.NoError(t, err)
	}
	for name, value := range fields {
		require.NoError(t, writer.WriteField(name, value))
	}
	require.NoError(t, writer.Close())
	return &body, writer.FormDataContentType()
}
