package domain

import "time"

// CompletionRequest represents a unified LLM request.
type CompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []Message         `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"` // user, assistant, system
	Content string `json:"content"`
}

// CompletionResponse represents a unified LLM response.
type CompletionResponse struct {
	ID         string    `json:"id"`
	Model      string    `json:"model"`
	Provider   string    `json:"provider"`
	Content    string    `json:"content"`
	Usage      Usage     `json:"usage"`
	FinishTime time.Time `json:"finish_time"`
}

// StreamChunk represents a single streaming response chunk.
type StreamChunk struct {
	Delta string `json:"delta"`
	Done  bool   `json:"done"`
	Error error  `json:"error,omitempty"`
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost,omitempty"`
}
