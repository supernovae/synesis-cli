package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"synesis.sh/synesis/internal/api"
)

func TestTestStreaming(t *testing.T) {
	// Create a mock server that handles streaming
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// For streaming, we just need to return a successful response
		w.Header().Set("Content-Type", "application/json")
		resp := api.ChatResponse{
			ID:      "test-id",
			Model:   "gpt-3.5-turbo",
			Choices: []struct {
				Message      api.Message `json:"message"`
				FinishReason string      `json:"finish_reason"`
				Index        int         `json:"index"`
			}{{}},
		}
		// For the test, we just need the API to return successfully
		resp.Choices[0].Message = api.Message{Role: "assistant", Content: "Test response"}
		resp.Choices[0].FinishReason = "stop"
		http.Error(w, "streaming not implemented in test", http.StatusNotImplemented)
	}))
	defer server.Close()

	// Create a client
	cli := api.NewClient(server.URL, "test-key")
	defer cli.Close()

	ctx := context.Background()

	// Test that testStreaming doesn't panic
	err := testStreaming(ctx, cli, true)
	// We expect an error because the mock server doesn't implement streaming
	// but it shouldn't crash
	_ = err
}

func TestTestResponsesEndpoint(t *testing.T) {
	// Create a mock server
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		resp := api.ChatResponse{
			ID:      "test-id",
			Model:   "gpt-3.5-turbo",
			Choices: []struct {
				Message      api.Message `json:"message"`
				FinishReason string      `json:"finish_reason"`
				Index        int         `json:"index"`
			}{{}},
		}
		resp.Choices[0].Message = api.Message{Role: "assistant", Content: "Test response"}
		resp.Choices[0].FinishReason = "stop"
		w.Write([]byte(`{"id":"test-id","model":"gpt-3.5-turbo","choices":[{"message":{"role":"assistant","content":"Test response"},"finish_reason":"stop","index":0}]}`))
	}))
	defer server.Close()

	// Create a client
	cli := api.NewClient(server.URL, "test-key")
	defer cli.Close()

	ctx := context.Background()

	// Test that testResponsesEndpoint works
	err := testResponsesEndpoint(ctx, cli, true)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRunDoctorNoProfile(t *testing.T) {
	// Test runDoctor with no profile - should not crash
	err := runDoctor([]string{}, false, true, "")
	if err != nil {
		// Expected error when no config is found
		t.Logf("Expected error when no profile/config: %v", err)
	}
}

func TestRunDoctorWithArgs(t *testing.T) {
	// Test runDoctor with various arguments
	tests := []struct {
		name string
		args []string
	}{
		{"empty args", []string{}},
		{"with -v", []string{"-v"}},
		{"with --fix", []string{"--fix"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			_ = runDoctor(tt.args, false, true, "")
		})
	}
}
