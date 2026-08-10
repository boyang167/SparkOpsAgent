package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/command"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/application/query"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/identity/domain/repository"
	identityhttp "github.com/sparkops/sparkops-agent/backend/internal/modules/identity/interface/http"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/authn"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/config"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/eventbus"
	"github.com/sparkops/sparkops-agent/backend/internal/shared/kernel"
	"github.com/stretchr/testify/require"
)

type memoryMemberRepo struct {
	mu      sync.Mutex
	byEmail map[string]*model.Member
	byID    map[string]*model.Member
}

func newMemoryMemberRepo() *memoryMemberRepo {
	return &memoryMemberRepo{
		byEmail: make(map[string]*model.Member),
		byID:    make(map[string]*model.Member),
	}
}

func (r *memoryMemberRepo) Save(_ context.Context, member *model.Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copyMember := *member
	r.byEmail[member.Email] = &copyMember
	r.byID[member.ID.String()] = &copyMember
	return nil
}

func (r *memoryMemberRepo) FindByEmail(_ context.Context, email string) (*model.Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byEmail[email]
	if !ok {
		return nil, kernel.NewDomainError("IDENTITY_MEMBER_NOT_FOUND", "member not found")
	}
	copyMember := *m
	return &copyMember, nil
}

func (r *memoryMemberRepo) FindByID(_ context.Context, id kernel.MemberID) (*model.Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byID[id.String()]
	if !ok {
		return nil, kernel.NewDomainError("IDENTITY_MEMBER_NOT_FOUND", "member not found")
	}
	copyMember := *m
	return &copyMember, nil
}

func (r *memoryMemberRepo) ExistsByEmail(_ context.Context, email string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.byEmail[email]
	return ok, nil
}

var _ repository.MemberRepository = (*memoryMemberRepo)(nil)

func setupIdentityRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newMemoryMemberRepo()
	bus := eventbus.NewInMemory()
	tokens := authn.NewTokenProvider(config.JWTConfig{Secret: "test-secret", ExpireHour: 1})

	handler := identityhttp.NewHandler(
		command.NewRegisterMemberHandler(repo, bus),
		command.NewLoginHandler(repo, tokens),
		query.NewGetCurrentMemberHandler(repo, nil),
		command.NewUpdateProfileHandler(repo),
		command.NewChangePasswordHandler(repo, model.BcryptHasher{}),
	)

	r := gin.New()
	auth := r.Group("/api/v1/auth")
	auth.POST("/register", handler.Register)
	auth.POST("/login", handler.Login)
	auth.GET("/me", authn.RequireAuth(tokens), handler.Me)
	return r
}

func TestRegisterLoginMe(t *testing.T) {
	r := setupIdentityRouter(t)

	body, _ := json.Marshal(map[string]string{
		"email":        "user@sparkops.local",
		"password":     "Password1!",
		"display_name": "User",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "user@sparkops.local",
		"password": "Password1!",
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var loginEnv struct {
		Data identityhttp.LoginResponse `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&loginEnv))
	require.NotEmpty(t, loginEnv.Data.Token)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginEnv.Data.Token)
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}
