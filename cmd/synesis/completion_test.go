package main

import (
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
