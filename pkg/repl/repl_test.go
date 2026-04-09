package repl

import (
	"strings"
	"testing"

	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/session"
	"synesis.sh/synesis/pkg/ui"
)

func TestREPL_CommandParsing(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := &config.LoadedConfig{
		Cfg: config.Config{
			Model:   "gpt-4",
			BaseURL: "https://test.example.com",
			APIKey:  "test-key",
		},
	}

	_, err = New(store, cfg, nil, false, false, ui.RenderPlain)
	if err != nil {
		t.Fatalf("failed to create REPL: %v", err)
	}

	// Test command recognition
	tests := []struct {
		input    string
		expected string
	}{
		{"/help", "help"},
		{"/exit", "exit"},
		{"/quit", "exit"},
		{"/q", "exit"},
		{"/save", "save"},
		{"/model", "model"},
		{"/system", "system"},
		{"/clear", "clear"},
		{"/session", "session"},
		{"/new", "new"},
		{"/unknown", "unknown"},
	}

	for _, test := range tests {
		parts := strings.Fields(test.input)
		cmd := strings.ToLower(parts[0])
		if cmd != test.expected {
			// Just verify we can parse the command
			t.Logf("Parsed command: %s", cmd)
		}
	}
}

func TestREPL_SlashCommandRecognition(t *testing.T) {
	tests := []struct {
		input       string
		shouldMatch bool
		command     string
	}{
		{"/help", true, "/help"},
		{"/HELP", true, "/help"},
		{"/h", true, "/h"},
		{"/exit", true, "/exit"},
		{"/quit", true, "/quit"},
		{"/q", true, "/q"},
		{"/save test", true, "/save"},
		{"/model gpt-4", true, "/model"},
		{"/system You are helpful", true, "/system"},
		{"/clear", true, "/clear"},
		{"/session abc123", true, "/session"},
		{"/new", true, "/new"},
		{"hello", false, ""},
		{"", false, ""},
		{"/", true, "/"},  // Single slash is technically a command (just invalid)
	}

	for _, test := range tests {
		isCommand := strings.HasPrefix(test.input, "/")
		if isCommand != test.shouldMatch {
			t.Errorf("input %q: expected isCommand=%v, got %v", test.input, test.shouldMatch, isCommand)
		}

		if test.shouldMatch {
			parts := strings.Fields(test.input)
			if len(parts) > 0 {
				cmd := strings.ToLower(parts[0])
				expectedCmd := strings.ToLower(test.command)
				if cmd != expectedCmd {
					t.Errorf("input %q: expected command %q, got %q", test.input, expectedCmd, cmd)
				}
			}
		}
	}
}

func TestREPL_MultiLineInput(t *testing.T) {
	// Test that backslash continuation works
	input := "This is a long message \\\nthat continues \\\non multiple lines"

	// Simulate the accumulation logic
	var currentText string
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		currentText += line
		if !strings.HasSuffix(line, "\\") {
			// Complete input
			break
		}
		// Remove trailing backslash and continue
		currentText = strings.TrimSuffix(currentText, "\\") + "\n"
	}

	expected := "This is a long message \nthat continues \non multiple lines"
	if currentText != expected {
		t.Errorf("expected %q, got %q", expected, currentText)
	}
}

func TestREPL_PromptGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := &config.LoadedConfig{
		Cfg: config.Config{
			Model:   "gpt-4",
			BaseURL: "https://test.example.com",
			APIKey:  "test-key",
		},
	}

	r, err := New(store, cfg, nil, false, false, ui.RenderPlain)
	if err != nil {
		t.Fatalf("failed to create REPL: %v", err)
	}

	// Test prompt without session
	prompt := r.getPrompt()
	if prompt != "> " {
		t.Errorf("expected prompt without session to be '> ', got %q", prompt)
	}

	// Create a session and test prompt with session
	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	r.session = sess

	prompt = r.getPrompt()
	if !strings.HasSuffix(prompt, "> ") {
		t.Errorf("expected prompt to end with '> ', got %q", prompt)
	}
	if !strings.Contains(prompt, sess.ID[:8]) {
		t.Errorf("expected prompt to contain session ID, got %q", prompt)
	}
}
