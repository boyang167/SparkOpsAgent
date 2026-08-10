package openai_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/domain/port"
	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/infrastructure/adapter/openai"
	"github.com/stretchr/testify/require"
)

func TestClient_ChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	client := openai.NewClient("test-key", srv.URL)
	var chunks []string
	err := client.ChatStream(context.Background(), port.ChatRequest{
		Model: "gpt-4o",
		Messages: []port.Message{
			{Role: "user", Content: "hello"},
		},
	}, func(delta string) error {
		chunks = append(chunks, delta)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"Hi"}, chunks)
}

func TestClient_ChatStream_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	client := openai.NewClient("test-key", srv.URL)
	err := client.ChatStream(context.Background(), port.ChatRequest{Model: "gpt-4o"}, func(string) error { return nil })
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "openai chat failed"))
}
