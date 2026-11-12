package entitlements

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFilesystemMiddlewareHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name       string
		config     string
		wantStatus int
		wantCode   string
	}{
		{name: "disabled", config: `{"features":{"filesystem":false}}`, wantStatus: http.StatusForbidden, wantCode: "ENTITLEMENT_MODULE_DISABLED"},
		{name: "default enabled", config: `{}`, wantStatus: http.StatusNoContent},
		{name: "explicit enabled", config: `{"features":{"filesystem":true}}`, wantStatus: http.StatusNoContent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := entitlementTestDB(t, tt.config)
			InvalidateCache()
			SetCacheTTL(0)

			router := gin.New()
			router.Use(Middleware(db))
			router.GET("/api/v1/workspaces/:wsId/files", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/files", nil)
			req.Header.Set("X-Workspace-Id", "00000000-0000-0000-0000-000000000001")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantCode != "" {
				var response struct {
					Data struct {
						ErrorCode string `json:"error_code"`
					} `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
				require.Equal(t, tt.wantCode, response.Data.ErrorCode)
			}
		})
	}
}

func entitlementTestDB(t *testing.T, config string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS platform_admin").Error)
	require.NoError(t, db.Exec("ATTACH DATABASE ':memory:' AS plugin").Error)
	require.NoError(t, db.Exec(`CREATE TABLE platform_admin.sys_cfg (
		config TEXT, deleted_at DATETIME, created_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE platform_admin.cluster_info (
		id TEXT, deleted_at DATETIME
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE plugin.workspace_bindings (
		workspace_id TEXT, plugin_id TEXT, enabled BOOLEAN
	)`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO platform_admin.sys_cfg (config, created_at) VALUES (?, CURRENT_TIMESTAMP)`,
		config,
	).Error)
	return db
}
