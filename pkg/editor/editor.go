package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Editor manages text editing operations
type Editor struct {
	Executable string
	Args       []string
}

// New creates a new Editor instance
func New() *Editor {
	return &Editor{
		Executable: os.Getenv("EDITOR"),
	}
}

// SetExecutable sets the editor executable
func (e *Editor) SetExecutable(exec string) {
	e.Executable = exec
}

// SetArgs sets additional arguments for the editor
func (e *Editor) SetArgs(args []string) {
	e.Args = args
}

// validateExecutable checks that the editor executable is a simple name (not a
// shell command with arguments or path separators) and is resolvable in PATH.
func (e *Editor) validateExecutable() error {
	if e.Executable == "" {
		return fmt.Errorf("EDITOR environment variable not set")
	}
	if strings.ContainsAny(e.Executable, ";|&$`<>") {
		return fmt.Errorf("EDITOR contains shell metacharacters: %s", e.Executable)
	}
	// Only allow absolute paths or simple names resolvable in PATH.
	if filepath.IsAbs(e.Executable) {
		if _, err := os.Stat(e.Executable); err != nil {
			return fmt.Errorf("editor not found: %s", e.Executable)
		}
		return nil
	}
	_, err := exec.LookPath(e.Executable)
	if err != nil {
		return fmt.Errorf("editor not found in PATH: %s", e.Executable)
	}
	return nil
}

// EditFile opens a file in the editor
func (e *Editor) EditFile(path string) error {
	if err := e.validateExecutable(); err != nil {
		return err
	}

	cmdArgs := append([]string{path}, e.Args...)
	cmd := exec.Command(e.Executable, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// EditString opens content in a temporary file in the editor
func (e *Editor) EditString(content string) (string, error) {
	if err := e.validateExecutable(); err != nil {
		return "", err
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "synesis-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	// Write content
	if err := os.WriteFile(tmpFile.Name(), []byte(content), 0644); err != nil {
		return "", err
	}
	tmpFile.Close()

	// Open in editor
	cmdArgs := append([]string{tmpFile.Name()}, e.Args...)
	cmd := exec.Command(e.Executable, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Read modified content
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// EditPrompt opens the prompt in editor and returns the result
func (e *Editor) EditPrompt(initial string) (string, error) {
	return e.EditString(initial)
}

// RunEditorCommand runs an editor command on a file
func RunEditorCommand(path string) error {
	editor := New()
	return editor.EditFile(path)
}

// EditContentInEditor opens content in editor and returns the result
func EditContentInEditor(content string) (string, error) {
	editor := New()
	return editor.EditString(content)
}

// EditFileInEditor opens a file in the editor
func EditFileInEditor(path string) error {
	editor := New()
	return editor.EditFile(path)
}
