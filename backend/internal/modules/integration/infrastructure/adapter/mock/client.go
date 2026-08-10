package mock

import (
	"context"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/domain/port"
)

type Client struct{}

func NewClient() *Client { return &Client{} }

func (c *Client) ChatStream(_ context.Context, req port.ChatRequest, onDelta func(string) error) error {
	input := lastUserMessage(req.Messages)
	reply := "Mock reply: " + input
	for _, part := range strings.Fields(reply) {
		if err := onDelta(part + " "); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func (c *Client) ListModels(_ context.Context) ([]port.ModelInfo, error) {
	return []port.ModelInfo{
		{ID: "mock-gpt", Provider: "mock"},
	}, nil
}

func lastUserMessage(messages []port.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

var _ port.LLMClient = (*Client)(nil)
