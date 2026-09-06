package harbor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEnsureProjectExists(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/team-a") {
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	err := EnsureProject(context.Background(), server.URL, "admin", "secret", "team-a")
	if err != nil {
		t.Fatalf("EnsureProject() error = %v", err)
	}
}

func TestEnsureProjectCreatesWhenMissing(t *testing.T) {
	t.Parallel()
	var created bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/team-a"):
			w.WriteHeader(http.StatusForbidden)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/projects":
			created = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	err := EnsureProject(context.Background(), server.URL, "admin", "secret", "team-a")
	if err != nil {
		t.Fatalf("EnsureProject() error = %v", err)
	}
	if !created {
		t.Fatal("expected harbor project create request")
	}
}

func TestEnsureProjectAcceptsConflict(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/team-a"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/projects":
			w.WriteHeader(http.StatusConflict)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	if err := EnsureProject(context.Background(), server.URL, "admin", "secret", "team-a"); err != nil {
		t.Fatalf("EnsureProject() error = %v", err)
	}
}
