package main

import (
	"encoding/json"
	"flag"
	"testing"

	"synesis.sh/synesis/internal/api"
)

func TestJqFlag_Parsing(t *testing.T) {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(nil)
	jqExpr := fs.String("jq", "", "jq-style field selection")

	err := fs.Parse([]string{"--jq", ".choices[0].message.content"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *jqExpr != ".choices[0].message.content" {
		t.Errorf("expected .choices[0].message.content, got %s", *jqExpr)
	}
}

func TestPrintRequestFlag_Parsing(t *testing.T) {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(nil)
	printRequest := fs.Bool("print-request", false, "print full request payload")

	err := fs.Parse([]string{"--print-request"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*printRequest {
		t.Error("expected --print-request to be true")
	}
}

func TestBothJqAndPrintRequest_Parsing(t *testing.T) {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(nil)
	jqExpr := fs.String("jq", "", "jq-style field selection")
	printRequest := fs.Bool("print-request", false, "print full request payload")

	err := fs.Parse([]string{"--jq", ".content", "--print-request"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *jqExpr != ".content" {
		t.Errorf("expected jq to be .content, got %s", *jqExpr)
	}
	if !*printRequest {
		t.Error("expected --print-request to be true")
	}
}

func TestJqAndExtractPath_Parsing(t *testing.T) {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(nil)
	jqExpr := fs.String("jq", "", "jq-style field selection")
	extractPath := fs.String("extract-path", "", "extract-path style")

	err := fs.Parse([]string{"--jq", ".content", "--extract-path", "choices.0.message.content"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *jqExpr != ".content" {
		t.Errorf("expected jq to be .content, got %s", *jqExpr)
	}
	if *extractPath != "choices.0.message.content" {
		t.Errorf("expected extract-path to be set, got %s", *extractPath)
	}
}

func TestRedactRequest_HidesSystemMessages(t *testing.T) {
	req := &api.ChatRequest{
		Model:   "gpt-4",
		Stream:  false,
		Messages: []api.Message{
			{Role: "system", Content: "secret system prompt"},
			{Role: "user", Content: "hello world"},
		},
	}

	redacted := redactRequest(req)

	if len(redacted.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(redacted.Messages))
	}
	if redacted.Messages[0].Content != "[REDACTED]" {
		t.Errorf("expected system message to be [REDACTED], got %s", redacted.Messages[0].Content)
	}
	if redacted.Messages[1].Content != "hello world" {
		t.Errorf("expected user message to be preserved, got %s", redacted.Messages[1].Content)
	}
}

func TestRedactRequest_PreservesNonSystemMessages(t *testing.T) {
	req := &api.ChatRequest{
		Model:  "gpt-4",
		Messages: []api.Message{
			{Role: "user", Content: "user message"},
			{Role: "assistant", Content: "assistant reply"},
		},
	}

	redacted := redactRequest(req)

	if redacted.Messages[0].Content != "user message" {
		t.Errorf("expected user message preserved, got %s", redacted.Messages[0].Content)
	}
	if redacted.Messages[1].Content != "assistant reply" {
		t.Errorf("expected assistant message preserved, got %s", redacted.Messages[1].Content)
	}
}

func TestRedactRequest_PreservesModelAndSettings(t *testing.T) {
	req := &api.ChatRequest{
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
		Stream:      true,
		Messages: []api.Message{
			{Role: "system", Content: "secret"},
		},
	}

	redacted := redactRequest(req)

	if redacted.Model != "gpt-4" {
		t.Errorf("expected model to be preserved, got %s", redacted.Model)
	}
	if redacted.Temperature != 0.7 {
		t.Errorf("expected temperature to be preserved, got %f", redacted.Temperature)
	}
	if redacted.MaxTokens != 1000 {
		t.Errorf("expected max tokens to be preserved, got %d", redacted.MaxTokens)
	}
	if redacted.Stream != true {
		t.Errorf("expected stream to be preserved, got %v", redacted.Stream)
	}
}

func TestRedactRequest_HandlesEmptyMessages(t *testing.T) {
	req := &api.ChatRequest{
		Model:    "gpt-4",
		Messages: []api.Message{},
	}

	redacted := redactRequest(req)

	if len(redacted.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(redacted.Messages))
	}
}

func TestRedactRequest_HandlesEmptyTools(t *testing.T) {
	req := &api.ChatRequest{
		Model:    "gpt-4",
		Messages: []api.Message{{Role: "user", Content: "hello"}},
		Tools:    []api.Tool{},
	}

	redacted := redactRequest(req)

	if len(redacted.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(redacted.Tools))
	}
}

func TestPrintRequest_OutputFormat(t *testing.T) {
	req := &api.ChatRequest{
		Model:  "gpt-4",
		Messages: []api.Message{
			{Role: "system", Content: "secret"},
			{Role: "user", Content: "hello"},
		},
	}

	redacted := redactRequest(req)

	output, err := json.MarshalIndent(redacted, "", "  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	err = json.Unmarshal(output, &parsed)
	if err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	messages, ok := parsed["messages"].([]interface{})
	if !ok {
		t.Fatal("expected messages in output")
	}
	sysMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected first message to be an object")
	}
	if sysMsg["content"] != "[REDACTED]" {
		t.Errorf("expected system message to be [REDACTED], got %v", sysMsg["content"])
	}
}

func TestJqExpression_ExtractsContent(t *testing.T) {
	content := `{"choices": [{"message": {"content": "hello world"}}]}`

	var result map[string]interface{}
	err := json.Unmarshal([]byte(content), &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok {
		t.Fatal("expected choices to be an array")
	}
	if len(choices) == 0 {
		t.Fatal("expected at least one choice")
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected choice to be an object")
	}
	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		t.Fatal("expected message to be an object")
	}
	msgContent, ok := message["content"].(string)
	if !ok {
		t.Fatal("expected content to be a string")
	}
	if msgContent != "hello world" {
		t.Errorf("expected 'hello world', got %s", msgContent)
	}
}

func TestRedactRequest_PreservesToolCalls(t *testing.T) {
	req := &api.ChatRequest{
		Model:  "gpt-4",
		Messages: []api.Message{
			{Role: "assistant", Content: "", ToolCalls: []api.ToolCall{
				{ID: "call_123", Type: "function", Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "get_weather", Arguments: `{"location": "NYC"}`}},
			}},
		},
	}

	redacted := redactRequest(req)

	if len(redacted.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(redacted.Messages))
	}
	if len(redacted.Messages[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(redacted.Messages[0].ToolCalls))
	}
	if redacted.Messages[0].ToolCalls[0].ID != "call_123" {
		t.Errorf("expected tool call ID to be preserved, got %s", redacted.Messages[0].ToolCalls[0].ID)
	}
}

func TestRedactRequest_PreservesToolChoice(t *testing.T) {
	req := &api.ChatRequest{
		Model:      "gpt-4",
		ToolChoice: "required",
		Messages: []api.Message{
			{Role: "user", Content: "hello"},
		},
	}

	redacted := redactRequest(req)

	if redacted.ToolChoice != "required" {
		t.Errorf("expected tool choice to be preserved, got %v", redacted.ToolChoice)
	}
}
