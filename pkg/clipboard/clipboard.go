package clipboard

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
)

// Copy copies text to the clipboard
func Copy(text string) error {
	switch runtime.GOOS {
	case "darwin":
		return copyOSX(text)
	case "linux":
		return copyLinux(text)
	case "windows":
		return copyWindows(text)
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
}

// Paste gets text from the clipboard
func Paste() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return pasteOSX()
	case "linux":
		return pasteLinux()
	case "windows":
		return pasteWindows()
	default:
		return "", fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
}

func copyOSX(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

func pasteOSX() (string, error) {
	cmd := exec.Command("pbpaste")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func copyLinux(text string) error {
	// Try xclip first, then xsel
	cmd := exec.Command("xclip", "-selection", "clipboard")
	cmd.Stdin = bytes.NewBufferString(text)
	err := cmd.Run()
	if err == nil {
		return nil
	}

	// Fall back to xsel
	cmd = exec.Command("xsel", "--clipboard", "--input")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}

func pasteLinux() (string, error) {
	// Try xclip first, then xsel
	cmd := exec.Command("xclip", "-selection", "clipboard", "-o")
	output, err := cmd.Output()
	if err == nil {
		return string(output), nil
	}

	// Fall back to xsel
	cmd = exec.Command("xsel", "--clipboard", "--output")
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func copyWindows(text string) error {
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("[Clipboard]::SetText(%q)", text))
	return cmd.Run()
}

func pasteWindows() (string, error) {
	cmd := exec.Command("powershell", "-Command",
		"[Clipboard]::GetText()")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
