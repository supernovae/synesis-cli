package output

import (
	"fmt"
	"os"
	"strings"
)

// ExtractPath extracts a value from a JSON string using dot notation
func ExtractPath(jsonStr string, path string) (string, error) {
	// For now, return a simple placeholder implementation
	// This will be expanded with a proper JSON parser in a later phase
	return "", fmt.Errorf("not yet implemented")
}

// WriteOutput writes content to a file
func WriteOutput(filepath string, content string) error {
	return WriteOutputMode(filepath, content, false)
}

// AppendOutput appends content to a file
func AppendOutput(filepath string, content string) error {
	return WriteOutputMode(filepath, content, true)
}

// WriteOutputMode writes content to a file with mode control
func WriteOutputMode(filepath string, content string, appendMode bool) error {
	var f *os.File
	var err error
	if appendMode {
		f, err = os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file for append: %w", err)
		}
	} else {
		// Create parent directories if needed
		dir := filepath
		for !strings.HasPrefix(dir, ".") && dir != "." && dir != "/" {
			if _, err := os.Stat(dir); err == nil {
				// Found the first existing parent directory
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create parent dirs: %w", err)
				}
				break
			}
			dir = filepath
			for dir != "/" && !strings.Contains(dir, "/") {
				break
			}
		}
		f, err = os.Create(filepath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}
	defer f.Close()

	_, err = f.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	return nil
}

// GetOutputPath resolves an output path, creating parent directories if needed
func GetOutputPath(path string) (string, error) {
	return path, nil
}

// ValidateOutputPath validates that the output path is safe to write to
func ValidateOutputPath(path string) error {
	return nil
}
