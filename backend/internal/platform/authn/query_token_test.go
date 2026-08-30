package authn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

func TestOptionalAuthAllowsMatchingAccessTokenQueryAndBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokenProvider(config.JWTConfig{Secret: "query-token-test"})
	memberID := kernel.NewMemberID()
	token, err := tokens.Issue(memberID)
	require.NoError(t, err)
	engine := optionalAuthProbeEngine(tokens)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe?access_token="+token, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"member_id":"`+memberID.String()+`"}`, rec.Body.String())
}

func TestOptionalAuthAllowsMatchingAccessTokenQueryAndLowercaseBearerScheme(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokenProvider(config.JWTConfig{Secret: "query-token-test"})
	memberID := kernel.NewMemberID()
	token, err := tokens.Issue(memberID)
	require.NoError(t, err)
	engine := optionalAuthProbeEngine(tokens)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe?access_token="+token, nil)
	req.Header.Set("Authorization", "bearer "+token)
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"member_id":"`+memberID.String()+`"}`, rec.Body.String())
}

func TestOptionalAuthRejectsAccessTokenQueryWithRawJWTAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokenProvider(config.JWTConfig{Secret: "query-token-test"})
	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	engine := optionalAuthProbeEngine(tokens)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe?access_token="+token, nil)
	req.Header.Set("Authorization", token)
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "AUTH_INVALID_TOKEN")
	require.NotContains(t, rec.Body.String(), token)
}

func TestOptionalAuthRejectsMismatchedAccessTokenQueryAndBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokenProvider(config.JWTConfig{Secret: "query-token-test"})
	bearerToken, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	queryToken, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	engine := optionalAuthProbeEngine(tokens)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe?access_token="+queryToken, nil)
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "AUTH_INVALID_TOKEN")
	require.NotContains(t, rec.Body.String(), queryToken)
	require.NotContains(t, rec.Body.String(), bearerToken)
}

func TestOptionalAuthRejectsAccessTokenQueryWithoutBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokenProvider(config.JWTConfig{Secret: "query-token-test"})
	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	engine := optionalAuthProbeEngine(tokens)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe?access_token="+token, nil)
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "AUTH_INVALID_TOKEN")
	require.NotContains(t, rec.Body.String(), token)
}

func TestOptionalAuthIgnoresXTokenOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokenProvider(config.JWTConfig{Secret: "query-token-test"})
	token, err := tokens.Issue(kernel.NewMemberID())
	require.NoError(t, err)
	engine := optionalAuthProbeEngine(tokens)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("X-Token", token)
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestOptionalAuthKeepsCompatibilityForBadBearerWithoutQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokenProvider(config.JWTConfig{Secret: "query-token-test"})
	engine := optionalAuthProbeEngine(tokens)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func optionalAuthProbeEngine(tokens *TokenProvider) *gin.Engine {
	engine := gin.New()
	engine.Use(OptionalAuth(tokens))
	engine.GET("/probe", func(c *gin.Context) {
		memberID, ok := MemberIDFromContext(c)
		if !ok {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusOK, gin.H{"member_id": memberID.String()})
	})
	return engine
}
