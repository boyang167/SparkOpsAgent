package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentSyncHandlerCreatePreviewRejectsInvalidProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewEnvironmentSyncHandler(nil, nil)
	router := gin.New()
	router.POST("/api/v1/products/:productId/environment-sync-previews", handler.CreatePreview)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/not-a-uuid/environment-sync-previews", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestEnvironmentSyncHandlerCreatePreviewRequiresAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewEnvironmentSyncHandler(query.NewCreateEnvironmentSyncPreviewHandler(nil, nil, nil, nil), nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("workspace_id", kernel.NewWorkspaceID())
		c.Next()
	})
	router.POST("/api/v1/products/:productId/environment-sync-previews", handler.CreatePreview)

	body := bytes.NewReader([]byte(`{"workflow_id":"` + kernel.NewProductWorkflowID().String() + `","job_name":"promote","source_environment_id":"` + kernel.NewEnvironmentID().String() + `","target_environment_id":"` + kernel.NewEnvironmentID().String() + `","projects":[{"project_id":"` + kernel.NewProjectID().String() + `","sync_image":true,"sync_yaml":false}]}`))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products/"+kernel.NewProductID().String()+"/environment-sync-previews", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestToEnvironmentSyncPreviewViewIncludesPreviewID(t *testing.T) {
	snapshotID := kernel.NewEnvironmentSyncSnapshotID()
	view := query.ToEnvironmentSyncPreviewView(&model.EnvironmentSyncSnapshot{
		ID:                  snapshotID,
		WorkflowID:          kernel.NewProductWorkflowID(),
		JobName:             "promote",
		SourceEnvironmentID: kernel.NewEnvironmentID(),
		TargetEnvironmentID: kernel.NewEnvironmentID(),
		Checksum:            "abc",
			Projects: []model.EnvironmentSyncProjectSnapshot{{
				ProjectID: kernel.NewProjectID(),
				Option: model.EnvironmentSyncProjectOption{
					SyncImage: true, SyncYAML: true, FirstSync: true,
				},
				Plan: model.EnvironmentSyncPlan{ImageRepository: "registry/demo", ImageTag: "v2"},
			}},
	})
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.Contains(t, string(raw), snapshotID.String())
	require.Contains(t, string(raw), `"preview_id"`)
}
