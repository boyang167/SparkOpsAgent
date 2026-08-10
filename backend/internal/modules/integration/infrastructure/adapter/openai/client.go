package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sparkops/sparkops-agent/backend/internal/modules/integration/domain/port"
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewClient(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Client{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type chatPayload struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (c *Client) ChatStream(ctx context.Context, req port.ChatRequest, onDelta func(string) error) error {
	messages := make([]chatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, chatMessage{Role: m.Role, Content: m.Content})
	}
	payload := chatPayload{
		Model: req.Model, Messages: messages, Temperature: req.Temperature,
		MaxTokens: req.MaxTokens, Stream: true,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("openai chat failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		if err := onDelta(delta); err != nil {
			return err
		}
	}
}

func (c *Client) ListModels(_ context.Context) ([]port.ModelInfo, error) {
	return []port.ModelInfo{
		{ID: "gpt-4o", Provider: "openai"},
		{ID: "gpt-4o-mini", Provider: "openai"},
	}, nil
}

var _ port.LLMClient = (*Client)(nil)
