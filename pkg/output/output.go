package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itchyny/gojq"
)

// OutputConfig holds configuration for output processing
type OutputConfig struct {
	Format       string // output format: text, json, markdown, csv
	Field        string // jq-style field selection for JSON
	PrintRequest bool   // print the full request payload
}

// ProcessOutput processes the raw output based on configuration
func ProcessOutput(raw []byte, config OutputConfig) ([]byte, error) {
	switch config.Format {
	case "json":
		return formatJSON(raw, config.Field)
	case "markdown":
		return formatMarkdown(raw)
	case "csv":
		return formatCSV(raw)
	default:
		return raw, nil
	}
}

// formatJSON formats output as JSON with optional field extraction
func formatJSON(raw []byte, field string) ([]byte, error) {
	// First, validate the JSON
	var jsonData interface{}
	if err := json.Unmarshal(raw, &jsonData); err != nil {
		// If not valid JSON, return as-is (could be text output)
		return raw, nil
	}

	// If a field is specified, extract it using gojq
	if field != "" {
		query, err := gojq.Parse(field)
		if err != nil {
			return nil, fmt.Errorf("invalid jq query: %w", err)
		}

		iter := query.Run(jsonData)
		var result interface{}
		for {
			v, ok := iter.Next()
			if !ok {
				if err, ok := v.(error); ok {
					return nil, fmt.Errorf("jq query error: %w", err)
				}
				result = v
				break
			}
			result = v
		}

		// Marshal the result back to JSON
		return json.MarshalIndent(result, "", "  ")
	}

	// Return pretty-printed JSON
	return json.MarshalIndent(jsonData, "", "  ")
}

// formatMarkdown formats output as markdown
func formatMarkdown(raw []byte) ([]byte, error) {
	// Try to parse as JSON and format as a markdown code block
	var jsonData interface{}
	if err := json.Unmarshal(raw, &jsonData); err == nil {
		jsonBytes, _ := json.MarshalIndent(jsonData, "", "  ")
		return []byte(fmt.Sprintf("```json\n%s\n```", string(jsonBytes))), nil
	}

	// Return as text code block
	return []byte(fmt.Sprintf("```\n%s\n```", strings.TrimSpace(string(raw)))), nil
}

// formatCSV formats JSON output as CSV
func formatCSV(raw []byte) ([]byte, error) {
	var jsonData []interface{}
	if err := json.Unmarshal(raw, &jsonData); err != nil {
		// Try to parse as a single JSON object
		var singleObj map[string]interface{}
		if err := json.Unmarshal(raw, &singleObj); err == nil {
			jsonData = []interface{}{singleObj}
		} else {
			return nil, fmt.Errorf("output is not valid JSON or JSON array: %w", err)
		}
	}

	if len(jsonData) == 0 {
		return []byte(""), nil
	}

	// Get headers from the first object
	var headers []string
	if obj, ok := jsonData[0].(map[string]interface{}); ok {
		for k := range obj {
			headers = append(headers, k)
		}
	}

	var buf bytes.Buffer
	// Write header
	buf.WriteString(strings.Join(headers, ",") + "\n")

	// Write rows
	for _, item := range jsonData {
		if obj, ok := item.(map[string]interface{}); ok {
			var row []string
			for _, h := range headers {
				val := obj[h]
				valStr := fmt.Sprintf("%v", val)
				// Escape commas and quotes in values
				if strings.Contains(valStr, ",") || strings.Contains(valStr, "\"") || strings.Contains(valStr, "\n") {
					valStr = "\"" + strings.ReplaceAll(valStr, "\"", "\"\"") + "\""
				}
				row = append(row, valStr)
			}
			buf.WriteString(strings.Join(row, ",") + "\n")
		}
	}

	return buf.Bytes(), nil
}

// WriteOutput writes processed output to a file
func WriteOutput(content []byte, filePath string) error {
	return os.WriteFile(filePath, content, 0o600)
}

// AppendOutput appends processed output to a file
func AppendOutput(content []byte, filePath string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(content)
	return err
}

// ExtractPath extracts a value from JSON using a path expression
func ExtractPath(raw []byte, path string) ([]byte, error) {
	var jsonData interface{}
	if err := json.Unmarshal(raw, &jsonData); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Split path by dots and navigate the structure
	parts := strings.Split(path, ".")
	result := jsonData

	for _, part := range parts {
		switch v := result.(type) {
		case map[string]interface{}:
			result = v[part]
		case []interface{}:
			// Handle array indexing like items[0]
			if strings.HasSuffix(part, "]") {
				if idx, err := parseArrayIndex(part); err == nil {
					if idx < len(v) {
						result = v[idx]
					} else {
						return nil, fmt.Errorf("array index out of range: %d", idx)
					}
				} else {
					return nil, fmt.Errorf("invalid array syntax: %s", part)
				}
			} else {
				return nil, fmt.Errorf("cannot access key %q on array", part)
			}
		default:
			return nil, fmt.Errorf("cannot navigate through %T", result)
		}
	}

	// Marshal result to JSON
	return json.MarshalIndent(result, "", "  ")
}

// parseArrayIndex parses array index like "items[0]" and returns the index
func parseArrayIndex(part string) (int, error) {
	start := strings.Index(part, "[")
	end := strings.Index(part, "]")
	if start == -1 || end == -1 || end <= start {
		return -1, fmt.Errorf("invalid array syntax: %s", part)
	}
	idxStr := part[start+1 : end]
	var idx int
	if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil {
		return -1, err
	}
	return idx, nil
}

// RunCommand runs a command and returns its output
func RunCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// GetTempDir returns a temporary directory
func GetTempDir() string {
	return os.TempDir()
}

// GetHomeDir returns the user's home directory
func GetHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// ExpandPath expands ~ and environment variables in a path
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return os.ExpandEnv(path)
}
