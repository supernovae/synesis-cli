package streaming

import (
	"strings"
	"testing"
)

func TestParseChatDelta(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantContent string
		wantDone    bool
		wantErr     bool
	}{
		{
			name:        "empty data",
			data:        "",
			wantContent: "",
			wantDone:    false,
			wantErr:     false,
		},
		{
			name:        "whitespace only",
			data:        "   ",
			wantContent: "",
			wantDone:    false,
			wantErr:     false,
		},
		{
			name:        "done signal",
			data:        "[DONE]",
			wantContent: "",
			wantDone:    true,
			wantErr:     false,
		},
		{
			name:        "content delta",
			data:        `{"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`,
			wantContent: "Hello",
			wantDone:    false,
			wantErr:     false,
		},
		{
			name:        "invalid JSON",
			data:        `{invalid`,
			wantErr:     true,
		},
		{
			name:        "error response",
			data:        `{"error":{"message":"rate limited","type":"invalid_request_error"}}`,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, done, err := ParseChatDelta(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseChatDelta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if done != tt.wantDone {
				t.Errorf("ParseChatDelta() done = %v, wantDone %v", done, tt.wantDone)
			}
			if content != tt.wantContent {
				t.Errorf("ParseChatDelta() content = %v, wantContent %v", content, tt.wantContent)
			}
		})
	}
}

func TestParser_ProcessLine(t *testing.T) {
	var receivedContent strings.Builder
	parser := NewParser(func(content string) {
		receivedContent.WriteString(content)
	})

	// Test comment line
	parser.processLine([]byte(": This is a comment"))
	if receivedContent.Len() != 0 {
		t.Error("comment should be ignored")
	}

	// Test data field
	parser.processLine([]byte(`data: {"choices":[{"delta":{"content":"test"}}]}`))
	if receivedContent.String() != `{"choices":[{"delta":{"content":"test"}}]}` {
		t.Errorf("expected content, got %s", receivedContent.String())
	}
}

func TestParser_CommentLine(t *testing.T) {
	var calls int
	parser := NewParser(func(content string) {
		calls++
	})

	// Multiple comment lines should not trigger callback
	parser.processLine([]byte(":comment"))
	parser.processLine([]byte(": another comment"))
	parser.processLine([]byte(":"))

	if calls != 0 {
		t.Errorf("expected no calls for comments, got %d", calls)
	}
}

func TestIsUTF8Valid(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{"valid ASCII", "hello", true},
		{"valid UTF-8", "hello world", true},
		{"valid UTF-8 with emoji", "hello 🌍", true},
		{"invalid UTF-8", "hello\x80world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, valid := IsUTF8Valid(tt.input)
			if valid != tt.wantValid {
				t.Errorf("IsUTF8Valid() valid = %v, wantValid %v", valid, tt.wantValid)
			}
		})
	}
}

// Fuzz test for ParseChatDelta
func FuzzParseChatDelta(f *testing.F) {
	// Valid SSE data patterns
	f.Add(`{"choices":[{"delta":{"content":"test"}}]}`)
	f.Add(`[DONE]`)
	f.Add("")
	f.Add(`{"error":{"message":"test"}}`)

	f.Fuzz(func(t *testing.T, data string) {
		// Should never panic
		ParseChatDelta(data)
	})
}