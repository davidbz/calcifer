package echo_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/provider/echo"
)

func TestNewProvider(t *testing.T) {
	provider := echo.NewProvider()

	require.NotNil(t, provider)
	require.Equal(t, "echo", provider.Name())
}

func TestComplete_Success(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	req := &domain.CompletionRequest{
		Model: "echo4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello world"},
		},
	}

	resp, err := provider.Complete(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "echo4", resp.Model)
	require.Equal(t, "echo", resp.Provider)
	require.Equal(t, "[user]: Hello world\n", resp.Content)
	require.Equal(t, 3, resp.Usage.PromptTokens)     // "[user]:" "Hello" "world" = 3 words
	require.Equal(t, 3, resp.Usage.CompletionTokens) // Same as input
	require.Equal(t, 6, resp.Usage.TotalTokens)
	require.NotEmpty(t, resp.ID)
}

func TestComplete_NilRequest(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	resp, err := provider.Complete(ctx, nil)

	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "request cannot be nil")
}

func TestComplete_UnsupportedModel(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	req := &domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := provider.Complete(ctx, req)

	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "not supported")
}

func TestComplete_EmptyMessages(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	req := &domain.CompletionRequest{
		Model:    "echo4",
		Messages: []domain.Message{},
	}

	resp, err := provider.Complete(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Empty(t, resp.Content)
	require.Equal(t, 0, resp.Usage.PromptTokens)
	require.Equal(t, 0, resp.Usage.CompletionTokens)
	require.Equal(t, 0, resp.Usage.TotalTokens)
}

func TestComplete_MultipleMessages(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	req := &domain.CompletionRequest{
		Model: "echo4",
		Messages: []domain.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello world"},
			{Role: "assistant", Content: "Hi there"},
		},
	}

	resp, err := provider.Complete(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "[system]: You are helpful\n[user]: Hello world\n[assistant]: Hi there\n", resp.Content)
	require.Equal(t, 10, resp.Usage.PromptTokens) // All words including brackets/colons
	require.Equal(t, 10, resp.Usage.CompletionTokens)
	require.Equal(t, 20, resp.Usage.TotalTokens)
}

func TestStream_Success(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	req := &domain.CompletionRequest{
		Model: "echo4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello world"},
		},
	}

	chunks, err := provider.Stream(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, chunks)

	var builder strings.Builder
	var doneReceived bool

	for chunk := range chunks {
		if chunk.Done {
			doneReceived = true
			require.NoError(t, chunk.Error)
		} else {
			builder.WriteString(chunk.Delta)
		}
	}

	receivedContent := builder.String()

	require.True(t, doneReceived)
	// Stream joins words with spaces, so we get the content reconstructed
	require.Contains(t, receivedContent, "[user]:")
	require.Contains(t, receivedContent, "Hello")
	require.Contains(t, receivedContent, "world")
}

func TestStream_NilRequest(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	chunks, err := provider.Stream(ctx, nil)

	require.Error(t, err)
	require.Nil(t, chunks)
	require.Contains(t, err.Error(), "request cannot be nil")
}

func TestStream_UnsupportedModel(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	req := &domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	chunks, err := provider.Stream(ctx, req)

	require.Error(t, err)
	require.Nil(t, chunks)
	require.Contains(t, err.Error(), "not supported")
}

func TestStream_ContextCancellation(t *testing.T) {
	provider := echo.NewProvider()
	ctx, cancel := context.WithCancel(context.Background())

	req := &domain.CompletionRequest{
		Model: "echo4",
		Messages: []domain.Message{
			{Role: "user", Content: "This is a longer message for testing cancellation"},
		},
	}

	chunks, err := provider.Stream(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, chunks)

	// Cancel after receiving first chunk
	cancel()

	var lastChunk domain.StreamChunk
	for chunk := range chunks {
		lastChunk = chunk
	}

	// Should eventually receive done chunk with error
	require.True(t, lastChunk.Done)
}

func TestStream_EmptyMessages(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	req := &domain.CompletionRequest{
		Model:    "echo4",
		Messages: []domain.Message{},
	}

	chunks, err := provider.Stream(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, chunks)

	var doneReceived bool
	for chunk := range chunks {
		if chunk.Done {
			doneReceived = true
			require.Empty(t, chunk.Delta)
		}
	}

	require.True(t, doneReceived)
}

func TestIsModelSupported(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	require.True(t, provider.IsModelSupported(ctx, "echo4"))
	require.False(t, provider.IsModelSupported(ctx, "gpt-4"))
	require.False(t, provider.IsModelSupported(ctx, "echo3"))
	require.False(t, provider.IsModelSupported(ctx, ""))
}

func TestSupportedModels(t *testing.T) {
	provider := echo.NewProvider()
	ctx := context.Background()

	models := provider.SupportedModels(ctx)

	require.Len(t, models, 1)
	require.Contains(t, models, "echo4")
}
