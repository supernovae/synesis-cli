package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"synesis.sh/synesis/pkg/streaming"
)

// Client is the interface for making API calls
type Client interface {
	// Chat sends a non-streaming chat request
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// StreamChat sends a streaming chat request
	StreamChat(ctx context.Context, req *ChatRequest, onToken func(string, error)) error
	// ListModels lists available models
	ListModels(ctx context.Context) ([]Model, error)
	// Close closes the client
	Close() error
}

// ChatRequest represents a chat API request
type ChatRequest struct {
	Model         string        `json:"model"`
	Messages      []Message     `json:"messages"`
	Stream        bool          `json:"stream,omitempty"`
	Temperature   float64       `json:"temperature,omitempty"`
	MaxTokens     int           `json:"max_tokens,omitempty"`
	Tools         []Tool        `json:"tools,omitempty"`
	ToolChoice    interface{}   `json:"tool_choice,omitempty"`
	System        []Message     `json:"system,omitempty"`
	ResponseFormat ResponseFormat `json:"response_format,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	// For multi-modal
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
}

// Tool defines a function tool
type Tool struct {
	Type     string `json:"type"`
	Function *FunctionDef `json:"function,omitempty"`
}

// FunctionDef defines a function signature
type FunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ToolCall represents a function call
type ToolCall struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ResponseFormat for structured output
type ResponseFormat struct {
	Type       string `json:"type"`
	JsonSchema any    `json:"json_schema,omitempty"`
}

// ChatResponse represents a chat API response
type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message Message `json:"message"`
		FinishReason string `json:"finish_reason"`
		Index int `json:"index"`
	} `json:"choices"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *APIError `json:"error,omitempty"`
}

// Model represents an available model
type Model struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created int64  `json:"created,omitempty"`
	 Owned     string `json:"owned_by,omitempty"`
}

// APIError represents an API error response
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// APIError represents a structured API error
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%s): %s", e.Type, e.Message)
}

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
}

// DefaultRetryConfig provides safe retry defaults
var DefaultRetryConfig = RetryConfig{
	MaxRetries: 3,
	BaseDelay:  500 * time.Millisecond,
}

// httpClient implements Client using HTTP
type httpClient struct {
	baseURL    string
	apiKey     string
	orgID      string
	httpClient *http.Client
	retry      RetryConfig
	endpoint   string // "chat/completions" or "responses"
}

// Option configures the HTTP client
type Option func(*httpClient)

// WithOrgID sets the organization ID
func WithOrgID(org string) Option {
	return func(c *httpClient) {
		c.orgID = org
	}
}

// WithEndpoint sets the API endpoint path
func WithEndpoint(ep string) Option {
	return func(c *httpClient) {
		c.endpoint = ep
	}
}

// WithRetry configures retry behavior
func WithRetry(cfg RetryConfig) Option {
	return func(c *httpClient) {
		c.retry = cfg
	}
}

// If baseURL ends with /v1, normalize it by removing the /v1
// so the endpoint logic can add it consistently
func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	// Remove /v1 suffix if present to avoid duplication
	if strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}
	return baseURL
}

// NewClient creates a new API client
func NewClient(baseURL, apiKey string, opts ...Option) Client {
	baseURL = normalizeBaseURL(baseURL)
	c := &httpClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		retry:      DefaultRetryConfig,
		endpoint:   "chat/completions",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

var ErrNonRetryable = errors.New("non-retryable error")

// isRetryable checks if an error warrants retry
func (c *httpClient) isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		// Retry on rate limit or transient HTTP errors
		switch he.StatusCode {
		case 429, 500, 502, 503, 504:
			return true
		}
		return false
	}
	// Retry on transient network errors (timeouts, connection refused, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}
	return false
}

// buildRequest creates an HTTP request
func (c *httpClient) buildRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal error: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	path = strings.TrimSuffix(c.baseURL, "/") + "/" + strings.TrimPrefix(path, "/")
	req, err := http.NewRequestWithContext(ctx, method, path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.orgID != "" {
		req.Header.Set("OpenAI-Organization", c.orgID)
	}

	return req, nil
}

// doRequest performs an HTTP request with retry logic
func (c *httpClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retry.MaxRetries; attempt++ {
		req, err := c.buildRequest(ctx, method, path, body)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if we should retry
		if !c.isRetryable(err) {
			return nil, ErrNonRetryable
		}

		// Wait before retry
		delay := c.retry.BaseDelay * time.Duration(attempt+1)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, fmt.Errorf("max retries exceeded, last error: %w", lastErr)
}

func (c *httpClient) doRequestAndClose(ctx context.Context, method, path string, body any) (*http.Response, error) {
	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// baseClient is a common helper
var _ Client = (*httpClient)(nil)

func (c *httpClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req == nil {
		return nil, errors.New("nil request")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	endpoint := c.endpoint
	if c.endpoint == "responses" {
		endpoint = "v1/responses"
	} else if c.endpoint == "chat/completions" {
		endpoint = "v1/chat/completions"
	} else {
		// For any other endpoint
		endpoint = "v1/" + endpoint
	}

	resp, err := c.doRequestAndClose(ctx, "POST", endpoint, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	// Read all response
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(data, &chatResp); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return &chatResp, nil
}

// StreamChat sends a streaming chat request
func (c *httpClient) StreamChat(ctx context.Context, req *ChatRequest, onToken func(string, error)) error {
	if req == nil {
		return errors.New("nil request")
	}
	if req.Model == "" {
		return errors.New("model is required")
	}

	req.Stream = true

	endpoint := c.endpoint
	if c.endpoint == "responses" {
		endpoint = "v1/responses"
	} else if c.endpoint == "chat/completions" {
		endpoint = "v1/chat/completions"
	} else {
		// For any other endpoint
		endpoint = "v1/" + endpoint
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	path := strings.TrimSuffix(c.baseURL, "/") + "/" + strings.TrimPrefix(endpoint, "/")
	httpReq, err := http.NewRequestWithContext(ctx, "POST", path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.orgID != "" {
		httpReq.Header.Set("OpenAI-Organization", c.orgID)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	parser := streaming.NewParser(func(content string) {
		delta, done, err := streaming.ParseChatDelta(content)
		if err != nil {
			onToken("", err)
			return
		}
		if done {
			return
		}
		if delta != "" {
			onToken(delta, nil)
		}
	})

	return parser.Parse(ctx, resp.Body)
}

// ChatCompletionClient handles /v1/chat/completions specifically
type ChatCompletionClient struct {
	*httpClient
}

// NewChatCompletionClient creates a client for chat/completions
func NewChatCompletionClient(baseURL, apiKey string, opts ...Option) *ChatCompletionClient {
	o := append(opts, WithEndpoint("chat/completions"))
	return &ChatCompletionClient{
		httpClient: NewClient(baseURL, apiKey, o...).(*httpClient),
	}
}

// ResponsesClient handles /v1/responses API
type ResponsesClient struct {
	*httpClient
}

// NewResponsesClient creates a client for responses API
func NewResponsesClient(baseURL, apiKey string, opts ...Option) *ResponsesClient {
	o := append(opts, WithEndpoint("responses"))
	return &ResponsesClient{
		httpClient: NewClient(baseURL, apiKey, o...).(*httpClient),
	}
}

// ListModels gets available models
func (c *httpClient) ListModels(ctx context.Context) ([]Model, error) {
	resp, err := c.doRequestAndClose(ctx, "GET", "/v1/models", nil)
	if err != nil {
		// Some endpoints don't support model listing
		return nil, fmt.Errorf("models endpoint not available: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil // Not supported
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var respObj struct {
		Data []Model `json:"data"`
	}
	if err := json.Unmarshal(data, &respObj); err != nil {
		return nil, err
	}

	return respObj.Data, nil
}

func (c *httpClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}