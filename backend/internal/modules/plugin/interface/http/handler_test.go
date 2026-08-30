package http_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/domain/model"
	pluginhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/plugin/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
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

func TestInstallUploadLimitIsFiftyMiB(t *testing.T) {
	require.Equal(t, int64(50<<20), pluginhttp.MaxUploadBytes)
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
