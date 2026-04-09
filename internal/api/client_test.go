package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "base URL without v1",
			input:    "https://api.openai.com",
			expected: "https://api.openai.com",
		},
		{
			name:     "base URL with v1 suffix",
			input:    "https://api.openai.com/v1",
			expected: "https://api.openai.com",
		},
		{
			name:     "base URL with v1 and trailing slash",
			input:    "https://api.openai.com/v1/",
			expected: "https://api.openai.com",
		},
		{
			name:     "custom endpoint without v1",
			input:    "http://localhost:8080",
			expected: "http://localhost:8080",
		},
		{
			name:     "custom endpoint with v1",
			input:    "http://localhost:8080/v1",
			expected: "http://localhost:8080",
		},
		{
			name:     "empty URL",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeBaseURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChatEndpointWithV1Config(t *testing.T) {
	// Test that config with /v1 doesn't cause duplication
	// This is the key test for the bug: Route POST:/v1/chat/chat/completions not found

	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		t.Logf("Received request to path: %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		resp := ChatResponse{
			ID:      "test-id",
			Model:   "gpt-4o-mini",
			Choices: []struct {
				Message      Message `json:"message"`
				FinishReason string  `json:"finish_reason"`
				Index        int     `json:"index"`
			}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Simulate config with /v1 suffix (common user error)
	client := NewClient(server.URL+"/v1", "test-key")
	ctx := context.Background()

	_, err := client.Chat(ctx, &ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	require.NoError(t, err)
	// The path should be /v1/chat/completions, NOT /v1/v1/chat/chat/completions
	t.Logf("EXPECTED: /v1/chat/completions, GOT: %s", receivedPath)
	assert.Equal(t, "/v1/chat/completions", receivedPath,
		"endpoint should not duplicate /v1 when base URL already includes it")
}

func TestChatEndpointWithoutV1Config(t *testing.T) {
	// Test that config without /v1 also works correctly
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		t.Logf("Received request to path: %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		resp := ChatResponse{
			ID:      "test-id",
			Model:   "gpt-4o-mini",
			Choices: []struct {
				Message      Message `json:"message"`
				FinishReason string  `json:"finish_reason"`
				Index        int     `json:"index"`
			}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// No /v1 suffix
	client := NewClient(server.URL, "test-key")
	ctx := context.Background()

	_, err := client.Chat(ctx, &ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	require.NoError(t, err)
	t.Logf("EXPECTED: /v1/chat/completions, GOT: %s", receivedPath)
	assert.Equal(t, "/v1/chat/completions", receivedPath)
}

func TestListModels(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		t.Logf("Received request to path: %s", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		resp := struct {
			Data []Model `json:"data"`
		}{
			Data: []Model{
				{ID: "gpt-4", Object: "model"},
				{ID: "gpt-4o-mini", Object: "model"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "test-key")
	ctx := context.Background()

	models, err := client.ListModels(ctx)

	require.NoError(t, err)
	assert.Equal(t, "/v1/models", receivedPath)
	assert.Len(t, models, 2)
	assert.Equal(t, "gpt-4", models[0].ID)
}

func TestChatWithInvalidModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "test-key")
	ctx := context.Background()

	// Should error when model is empty
	_, err := client.Chat(ctx, &ChatRequest{
		Model: "",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model is required")
}

func TestChatWithNilRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "test-key")
	ctx := context.Background()

	_, err := client.Chat(ctx, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil request")
}

func TestChatWithAuthHeader(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		resp := ChatResponse{
			ID:      "test-id",
			Model:   "gpt-4o-mini",
			Choices: []struct {
				Message      Message `json:"message"`
				FinishReason string  `json:"finish_reason"`
				Index        int     `json:"index"`
			}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "test-api-key-123")
	ctx := context.Background()

	_, err := client.Chat(ctx, &ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-api-key-123", authHeader)
}

func TestChatErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message": "Invalid API key", "type": "invalid_request_error"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "invalid-key")
	ctx := context.Background()

	_, err := client.Chat(ctx, &ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	})

	require.Error(t, err)
	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, 400, httpErr.StatusCode)
}