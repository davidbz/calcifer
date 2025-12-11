package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the HTTP client for OpenAI API calls.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new OpenAI HTTP client.
func NewClient(config Config) *Client {
	return &Client{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// OpenAI API request/response structures.
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response represents the response from OpenAI API.
type Response struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamChunk struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// Complete sends a non-streaming completion request.
func (c *Client) Complete(ctx context.Context, req openAIRequest) (*Response, error) {
	if c.apiKey == "" {
		return nil, errors.New("API key is not configured")
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp Response
	if decodeErr := json.NewDecoder(resp.Body).Decode(&openAIResp); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", decodeErr)
	}

	return &openAIResp, nil
}

// Stream sends a streaming completion request.
func (c *Client) Stream(ctx context.Context, req openAIRequest) (<-chan StreamResult, error) {
	if c.apiKey == "" {
		return nil, errors.New("API key is not configured")
	}

	req.Stream = true

	//nolint:bodyclose // Response body is closed in processStreamResponse goroutine
	resp, err := c.executeStreamRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	chunks := make(chan StreamResult)
	go c.processStreamResponse(resp, chunks)

	return chunks, nil
}

// executeStreamRequest creates and executes the HTTP request for streaming.
func (c *Client) executeStreamRequest(ctx context.Context, req openAIRequest) (*http.Response, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/chat/completions",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// processStreamResponse reads and processes the streaming response.
func (c *Client) processStreamResponse(resp *http.Response, chunks chan<- StreamResult) {
	defer close(chunks)
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	for {
		chunk, done, err := c.decodeStreamChunk(decoder)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				chunks <- StreamResult{Delta: "", Done: false, Error: err}
			}
			return
		}

		chunks <- chunk

		if done {
			return
		}
	}
}

// decodeStreamChunk decodes a single chunk from the stream.
func (c *Client) decodeStreamChunk(decoder *json.Decoder) (StreamResult, bool, error) {
	var chunk openAIStreamChunk
	if err := decoder.Decode(&chunk); err != nil {
		return StreamResult{}, false, fmt.Errorf("failed to decode stream chunk: %w", err)
	}

	if len(chunk.Choices) == 0 {
		return StreamResult{}, false, nil
	}

	delta := chunk.Choices[0].Delta.Content
	done := chunk.Choices[0].FinishReason != nil

	return StreamResult{
		Delta: delta,
		Done:  done,
		Error: nil,
	}, done, nil
}

// StreamResult represents a single result from the streaming API.
type StreamResult struct {
	Delta string
	Done  bool
	Error error
}
