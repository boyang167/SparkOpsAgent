package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/delivery/deployrun/domain"
	"github.com/stretchr/testify/require"
)

func TestDeploymentRunningMapsToConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	HandleError(ctx, domain.ErrDeploymentAlreadyRunning)

	require.Equal(t, 409, recorder.Code)
	var response Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Code)
	require.Equal(t, "DELIVERY_DEPLOYMENT_RUNNING", response.Data.(map[string]any)["error_code"])
}
