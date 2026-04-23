package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunCompletion_Bash(t *testing.T) {
	err := runCompletion([]string{"bash"}, false, true)
	if err != nil {
		t.Fatalf("runCompletion bash failed: %v", err)
	}
}

func TestRunCompletion_Zsh(t *testing.T) {
	err := runCompletion([]string{"zsh"}, false, true)
	if err != nil {
		t.Fatalf("runCompletion zsh failed: %v", err)
	}
}

func TestRunCompletion_InvalidShell(t *testing.T) {
	err := runCompletion([]string{"powershell"}, false, true)
	if err == nil {
		t.Fatal("expected error for invalid shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("expected 'unsupported shell' error, got: %v", err)
	}
}

func TestRunCompletion_NoArgs(t *testing.T) {
	err := runCompletion([]string{}, false, true)
	if err != nil {
		t.Fatalf("runCompletion with no args failed: %v", err)
	}
}

func TestGenerateBashCompletion(t *testing.T) {
	err := generateBashCompletion()
	if err != nil {
		t.Fatalf("generateBashCompletion failed: %v", err)
	}
}

func TestGenerateZshCompletion(t *testing.T) {
	err := generateZshCompletion()
	if err != nil {
		t.Fatalf("generateZshCompletion failed: %v", err)
	}
}

func TestGenerateFishCompletion(t *testing.T) {
	err := generateFishCompletion()
	if err != nil {
		t.Fatalf("generateFishCompletion failed: %v", err)
	}
}

func TestGenerateFishCompletion_ContainsNewCommands(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := generateFishCompletion()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)

	if err != nil {
		t.Fatalf("generateFishCompletion failed: %v", err)
	}
	out := buf.String()

	for _, cmd := range []string{"review", "pr-summary", "release-notes", "explain-commit"} {
		if !strings.Contains(out, `-a "`+cmd+`"`) {
			t.Errorf("Fish completion missing command %q", cmd)
		}
	}
}
