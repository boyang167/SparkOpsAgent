//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/model"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/filesystem/domain/service"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage"
	"github.com/sparkops/sparkops-agent/backend/internal/platform/storage/adapters/local"
)

type Principal struct {
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	UserID         uuid.UUID
}

type Organization struct {
	ID   uuid.UUID
	User Principal
}

type File struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	ObjectKey      string
}

type Response struct {
	StatusCode int
	Body       []byte
}

type TestEnv struct {
	OrganizationA Organization
	OrganizationB Organization

	provider *local.Provider
	mu       sync.RWMutex
	files    map[uuid.UUID]File
}

func NewTestEnv(t testing.TB) *TestEnv {
	t.Helper()
	Require(t)
	provider, err := local.New(t.TempDir())
	if err != nil {
		t.Fatalf("create local provider: %v", err)
	}
	t.Cleanup(func() { _ = provider.Close() })
	return &TestEnv{
		OrganizationA: newOrganization(),
		OrganizationB: newOrganization(),
		provider:      provider,
		files:         make(map[uuid.UUID]File),
	}
}

func Require(t testing.TB) {
	t.Helper()
	if os.Getenv("REQUIRE_INTEGRATION") != "1" {
		t.Skip("set REQUIRE_INTEGRATION=1 to run filesystem integration suites")
	}
	if os.Getenv("DATABASE_DSN") == "" {
		t.Fatal("DATABASE_DSN is required when REQUIRE_INTEGRATION=1")
	}
}

func (e *TestEnv) UploadAs(t testing.TB, principal Principal, content []byte) File {
	t.Helper()
	file := File{
		ID: uuid.New(), OrganizationID: principal.OrganizationID,
		WorkspaceID: principal.WorkspaceID,
	}
	file.ObjectKey = "objects/" + file.ID.String()
	_, err := e.provider.Put(context.Background(), storage.PutRequest{
		ObjectKey: file.ObjectKey,
		Body:      bytes.NewReader(content),
		Size:      int64(len(content)),
		SHA256:    storage.SHA256(content),
	})
	if err != nil {
		t.Fatalf("upload fixture: %v", err)
	}
	e.mu.Lock()
	e.files[file.ID] = file
	e.mu.Unlock()
	return file
}

func (e *TestEnv) DownloadAs(t testing.TB, principal Principal, fileID uuid.UUID) Response {
	t.Helper()
	e.mu.RLock()
	file, ok := e.files[fileID]
	e.mu.RUnlock()
	if !ok {
		return Response{StatusCode: http.StatusNotFound}
	}
	decision := service.Decide(
		service.Subject{
			ActorID: principal.UserID, OrganizationID: principal.OrganizationID,
			WorkspaceID: principal.WorkspaceID, Authenticated: true, Entitled: true, RBACAllowed: true,
		},
		model.ResourcePermissionRead,
		service.Resource{
			ID: file.ID, OrganizationID: file.OrganizationID, WorkspaceID: file.WorkspaceID,
		},
		service.Relation{
			ResourceID: file.ID, Effect: model.GrantEffectAllow,
			Permission: model.ResourcePermissionRead, Source: service.SourceOwner,
		},
	)
	if decision.Result != service.DecisionAllow {
		return Response{StatusCode: http.StatusForbidden}
	}
	reader, _, err := e.provider.Get(context.Background(), storage.GetRequest{ObjectKey: file.ObjectKey})
	if errors.Is(err, storage.ErrNotFound) {
		return Response{StatusCode: http.StatusNotFound}
	}
	if err != nil {
		t.Fatalf("download fixture: %v", err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read download: %v", err)
	}
	return Response{StatusCode: http.StatusOK, Body: body}
}

func newOrganization() Organization {
	id := uuid.New()
	return Organization{
		ID:   id,
		User: Principal{OrganizationID: id, WorkspaceID: uuid.New(), UserID: uuid.New()},
	}
}
