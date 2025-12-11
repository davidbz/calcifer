package openai

import (
	"context"
	"testing"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
)

func TestNewProvider_Success(t *testing.T) {
	config := Config{
		APIKey:     "test-api-key",
		BaseURL:    "https://api.openai.com/v1",
		Timeout:    60,
		MaxRetries: 3,
	}

	provider, err := NewProvider(config)

	require.NoError(t, err)
	require.NotNil(t, provider)
	require.Equal(t, "openai", provider.name)
}

func TestNewProvider_MissingAPIKey(t *testing.T) {
	config := Config{
		APIKey:     "",
		BaseURL:    "https://api.openai.com/v1",
		Timeout:    60,
		MaxRetries: 3,
	}

	provider, err := NewProvider(config)

	require.Error(t, err)
	require.Nil(t, provider)
	require.Contains(t, err.Error(), "OpenAI API key is required")
}

func TestProvider_Name(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	require.Equal(t, "openai", provider.Name())
}

func TestProvider_IsModelSupported(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	tests := []struct {
		name      string
		model     string
		supported bool
	}{
		{
			name:      "GPT-4 is supported",
			model:     "gpt-4",
			supported: true,
		},
		{
			name:      "GPT-4 Turbo is supported",
			model:     "gpt-4-turbo",
			supported: true,
		},
		{
			name:      "GPT-3.5 Turbo is supported",
			model:     "gpt-3.5-turbo",
			supported: true,
		},
		{
			name:      "Unknown model is not supported",
			model:     "unknown-model",
			supported: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.IsModelSupported(ctx, tt.model)
			require.Equal(t, tt.supported, result)
		})
	}
}

func TestProvider_Complete_NilRequest(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	ctx := context.Background()
	resp, err := provider.Complete(ctx, nil)

	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "request cannot be nil")
}

func TestProvider_Stream_NilRequest(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	ctx := context.Background()
	chunks, err := provider.Stream(ctx, nil)

	require.Error(t, err)
	require.Nil(t, chunks)
	require.Contains(t, err.Error(), "request cannot be nil")
}

func TestToSDKParams(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	req := &domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
			{Role: "assistant", Content: "I'm doing well, thank you!"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
	}

	params := provider.toSDKParams(req)

	require.Equal(t, openai.ChatModel("gpt-4"), params.Model)
	require.Len(t, params.Messages, 3)
	require.NotNil(t, params.Temperature)
	require.NotNil(t, params.MaxTokens)
}

func TestToSDKParams_MessageRoles(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	tests := []struct {
		name     string
		role     string
		content  string
		expected string
	}{
		{
			name:     "User message",
			role:     "user",
			content:  "Hello",
			expected: "user",
		},
		{
			name:     "Assistant message",
			role:     "assistant",
			content:  "Hi there",
			expected: "assistant",
		},
		{
			name:     "System message",
			role:     "system",
			content:  "You are helpful",
			expected: "system",
		},
		{
			name:     "Unknown role defaults to user",
			role:     "unknown",
			content:  "Test",
			expected: "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &domain.CompletionRequest{
				Model: "gpt-4",
				Messages: []domain.Message{
					{Role: tt.role, Content: tt.content},
				},
			}

			params := provider.toSDKParams(req)
			require.Len(t, params.Messages, 1)
		})
	}
}

func TestToDomainResponse(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	sdkResp := &openai.ChatCompletion{
		ID:    "chatcmpl-123",
		Model: "gpt-4",
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					Content: "Hello! How can I help you today?",
				},
			},
		},
		Usage: openai.CompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	domainResp := provider.toDomainResponse(sdkResp)

	require.Equal(t, "chatcmpl-123", domainResp.ID)
	require.Equal(t, "gpt-4", domainResp.Model)
	require.Equal(t, "openai", domainResp.Provider)
	require.Equal(t, "Hello! How can I help you today?", domainResp.Content)
	require.Equal(t, 10, domainResp.Usage.PromptTokens)
	require.Equal(t, 20, domainResp.Usage.CompletionTokens)
	require.Equal(t, 30, domainResp.Usage.TotalTokens)
	require.Greater(t, domainResp.Usage.Cost, 0.0)
}

func TestToDomainResponse_EmptyChoices(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	sdkResp := &openai.ChatCompletion{
		ID:      "chatcmpl-123",
		Model:   "gpt-4",
		Choices: []openai.ChatCompletionChoice{},
		Usage: openai.CompletionUsage{
			PromptTokens:     10,
			CompletionTokens: 0,
			TotalTokens:      10,
		},
	}

	domainResp := provider.toDomainResponse(sdkResp)

	require.Empty(t, domainResp.Content)
	require.Equal(t, "chatcmpl-123", domainResp.ID)
}

func TestGetModelConfig(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	tests := []struct {
		name               string
		model              string
		expectedInputCost  float64
		expectedOutputCost float64
	}{
		{
			name:               "GPT-4",
			model:              "gpt-4",
			expectedInputCost:  0.03,
			expectedOutputCost: 0.06,
		},
		{
			name:               "GPT-4 Turbo",
			model:              "gpt-4-turbo",
			expectedInputCost:  0.01,
			expectedOutputCost: 0.03,
		},
		{
			name:               "GPT-3.5 Turbo",
			model:              "gpt-3.5-turbo",
			expectedInputCost:  0.0005,
			expectedOutputCost: 0.0015,
		},
		{
			name:               "Unknown model",
			model:              "unknown-model",
			expectedInputCost:  0,
			expectedOutputCost: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := provider.getModelConfig(tt.model)
			require.Equal(t, tt.expectedInputCost, config.InputCostPer1K)
			require.Equal(t, tt.expectedOutputCost, config.OutputCostPer1K)
		})
	}
}

func TestToDomainResponse_CostCalculation(t *testing.T) {
	provider := &Provider{
		name: "openai",
	}

	tests := []struct {
		name                string
		model               string
		promptTokens        int64
		completionTokens    int64
		expectedCostMinimum float64
		expectedCostMaximum float64
	}{
		{
			name:                "GPT-4 cost calculation",
			model:               "gpt-4",
			promptTokens:        1000,
			completionTokens:    500,
			expectedCostMinimum: 0.06, // (1000/1000)*0.03 + (500/1000)*0.06
			expectedCostMaximum: 0.06, // exact match
		},
		{
			name:                "GPT-3.5 Turbo cost calculation",
			model:               "gpt-3.5-turbo",
			promptTokens:        1000,
			completionTokens:    1000,
			expectedCostMinimum: 0.002, // (1000/1000)*0.0005 + (1000/1000)*0.0015
			expectedCostMaximum: 0.002, // exact match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdkResp := &openai.ChatCompletion{
				ID:    "chatcmpl-test",
				Model: openai.ChatModel(tt.model),
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "Test response",
						},
					},
				},
				Usage: openai.CompletionUsage{
					PromptTokens:     tt.promptTokens,
					CompletionTokens: tt.completionTokens,
					TotalTokens:      tt.promptTokens + tt.completionTokens,
				},
			}

			domainResp := provider.toDomainResponse(sdkResp)

			require.GreaterOrEqual(t, domainResp.Usage.Cost, tt.expectedCostMinimum)
			require.LessOrEqual(t, domainResp.Usage.Cost, tt.expectedCostMaximum)
		})
	}
}
