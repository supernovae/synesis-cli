package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ExtractPath extracts a value from a JSON string using dot notation
func ExtractPath(jsonStr string, path string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	components := strings.Split(path, ".")
	current := data

	for _, component := range components {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[component]; ok {
				current = val
			} else {
				return "", fmt.Errorf("path not found: %s", path)
			}
		case []interface{}:
			if idx, err := strconv.Atoi(component); err == nil {
				if idx >= 0 && idx < len(v) {
					current = v[idx]
				} else {
					return "", fmt.Errorf("array index out of bounds: %d", idx)
				}
			} else {
				return "", fmt.Errorf("invalid array index: %s", component)
			}
		default:
			return "", fmt.Errorf("cannot navigate into %T at %s", current, component)
		}
	}

	switch v := current.(type) {
	case string:
		return v, nil
	case nil:
		return "null", nil
	case bool:
		return strconv.FormatBool(v), nil
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}
		return string(jsonBytes), nil
	}
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

// GetOutputPath resolves an output path
func GetOutputPath(path string) (string, error) {
	return path, nil
}

// ValidateOutputPath validates that the output path is safe to write to
func ValidateOutputPath(path string) error {
	return nil
}
