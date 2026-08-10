package port

import "context"

type Message struct {
	Role    string
	Content string
}

type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

type ModelInfo struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
}

type LLMClient interface {
	ChatStream(ctx context.Context, req ChatRequest, onDelta func(string) error) error
	ListModels(ctx context.Context) ([]ModelInfo, error)
}
