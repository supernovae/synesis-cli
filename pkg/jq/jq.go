package jq

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

)

// Apply applies a jq-style filter to JSON content and returns the result.
func Apply(content string, filter string) (string, error) {
	if filter == "" {
		return content, nil
	}

	var data interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", fmt.Errorf("parse json: %w", err)
	}

	result, err := parseAndApply(data, filter)
	if err != nil {
		return "", fmt.Errorf("apply filter: %w", err)
	}

	output, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}

	// Convert to string, stripping quotes from string results
	out := strings.TrimSpace(string(output))
	if len(out) >= 2 && out[0] == '"' && out[len(out)-1] == '"' {
		return out[1 : len(out)-1], nil
	}
	return out, nil
}

func parseAndApply(data interface{}, filter string) (interface{}, error) {
	trimmed := strings.TrimSpace(filter)

	// Handle string literal
	if strings.HasPrefix(trimmed, `"`) {
		end := findMatchingQuote(trimmed)
		if end < 0 {
			return nil, fmt.Errorf("unterminated string")
		}
		var s string
		if err := json.Unmarshal([]byte(trimmed[:end+1]), &s); err != nil {
			return nil, fmt.Errorf("invalid string: %w", err)
		}
		return s, nil
	}

	// Handle literal: true, false, null
	if trimmed == "true" || trimmed == "false" || trimmed == "null" {
		var val interface{}
		if err := json.Unmarshal([]byte(trimmed), &val); err != nil {
			return nil, fmt.Errorf("invalid literal: %w", err)
		}
		return val, nil
	}

	// Handle number literal
	if len(trimmed) > 0 && (trimmed[0] >= '0' && trimmed[0] <= '9' || trimmed[0] == '-') {
		var num interface{}
		if err := json.Unmarshal([]byte(trimmed), &num); err == nil {
			return num, nil
		}
	}

	// Handle object literal {}
	if trimmed == "{}" {
		return map[string]interface{}{}, nil
	}

	// Handle array literal []
	if trimmed == "[]" {
		return []interface{}{}, nil
	}

	// Handle array construction [.field1, .field2]
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		inner := trimmed[1 : len(trimmed)-1]
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return []interface{}{}, nil
		}
		parts := splitTopLevel(inner)
		result := make([]interface{}, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			val, err := parseAndApply(data, part)
			if err != nil {
				return nil, fmt.Errorf("array element %q: %w", part, err)
			}
			result = append(result, val)
		}
		return result, nil
	}

	// Handle recursive descent ..field
	if strings.HasPrefix(trimmed, "..") {
		field := trimmed[2:]
		return recursiveField(data, field), nil
	}

	// Handle field access: .field or .field.subfield or .field[0] etc.
	if !strings.HasPrefix(trimmed, ".") {
		return nil, fmt.Errorf("filter must start with '.'")
	}

	rest := trimmed[1:] // skip leading dot
	return walkFilter(data, rest)
}

func walkFilter(data interface{}, rest string) (interface{}, error) {
	rest = strings.TrimLeft(rest, ".")
	if rest == "" {
		return data, nil
	}

	// Check for array iteration: .[]
	if strings.HasPrefix(rest, "[]") {
		rest = rest[2:]
		result := traverseAll(data)
		if rest == "" {
			return result, nil
		}
		return walkFilter(result, rest)
	}

	// Extract field name
	fieldName, remaining := extractFieldNameAndRemaining(rest)
	if fieldName == "" {
		return nil, fmt.Errorf("expected field name")
	}

	// Access the field
	val := accessField(data, fieldName)
	if val == nil && data != nil {
		// Check if field exists
		if m, ok := data.(map[string]interface{}); ok {
			if _, exists := m[fieldName]; !exists {
				return "", nil // return empty string for missing fields
			}
		}
	}

	// Process remaining: [0], .field, etc.
	remaining = strings.TrimLeft(remaining, ".")
	if remaining == "" {
		return val, nil
	}

	// Check for array index/slice
	if len(remaining) > 0 && remaining[0] == '[' {
		bracketEnd := strings.Index(remaining, "]")
		if bracketEnd < 0 {
			return nil, fmt.Errorf("unmatched '['")
		}
		inner := remaining[1:bracketEnd]
		afterBracket := remaining[bracketEnd+1:]

		if inner == "*" || inner == "" {
			val = traverseAll(val)
		} else if strings.Contains(inner, ":") {
			parts := strings.SplitN(inner, ":", 2)
			start, end := 0, 0
			var err error
			if parts[0] != "" {
				start, err = strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("invalid slice start: %s", parts[0])
				}
			}
			if parts[1] != "" {
				end, err = strconv.Atoi(parts[1])
				if err != nil {
					return nil, fmt.Errorf("invalid slice end: %s", parts[1])
				}
			}
			val = sliceArray(val, start, end)
		} else {
			idx, err := strconv.Atoi(inner)
			if err != nil {
				return nil, fmt.Errorf("invalid index: %s", inner)
			}
			val = accessIndex(val, idx)
		}

		if afterBracket == "" {
			return val, nil
		}
		return walkFilter(val, afterBracket)
	}

	// Continue with more field access
	if remaining != "" {
		return walkFilter(val, remaining)
	}

	return val, nil
}

func accessField(data interface{}, field string) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		if val, ok := v[field]; ok {
			return val
		}
		return nil
	case []interface{}:
		idx, err := strconv.Atoi(field)
		if err == nil && idx >= 0 && idx < len(v) {
			return v[idx]
		}
		return nil
	}
	return nil
}

func accessIndex(data interface{}, idx int) interface{} {
	switch v := data.(type) {
	case []interface{}:
		if idx < 0 {
			idx = len(v) + idx
		}
		if idx >= 0 && idx < len(v) {
			return v[idx]
		}
		return nil
	case map[string]interface{}:
		return accessField(v, strconv.Itoa(idx))
	}
	return nil
}

func sliceArray(data interface{}, start, end int) interface{} {
	switch v := data.(type) {
	case []interface{}:
		if start < 0 {
			start = 0
		}
		if end < 0 || end > len(v) {
			end = len(v)
		}
		if start >= len(v) {
			return []interface{}{}
		}
		if end < start {
			end = start
		}
		return v[start:end]
	}
	return nil
}

func traverseAll(data interface{}) interface{} {
	switch v := data.(type) {
	case []interface{}:
		return v
	case map[string]interface{}:
		vals := make([]interface{}, 0, len(v))
		for _, val := range v {
			vals = append(vals, val)
		}
		return vals
	}
	return []interface{}{data}
}

func recursiveField(root interface{}, field string) interface{} {
	var result []interface{}
	collectField(root, field, &result)
	if len(result) == 0 {
		return ""
	}
	if len(result) == 1 {
		return result[0]
	}
	return result
}

func collectField(v interface{}, field string, result *[]interface{}) {
	switch val := v.(type) {
	case map[string]interface{}:
		if val, ok := val[field]; ok {
			*result = append(*result, val)
		}
		for _, item := range val {
			collectField(item, field, result)
		}
	case []interface{}:
		for _, item := range val {
			if fieldVal, ok := item.(map[string]interface{}); ok {
				if val, exists := fieldVal[field]; exists {
					*result = append(*result, val)
				}
			}
			collectField(item, field, result)
		}
	}
}

func findMatchingQuote(s string) int {
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return -1
}

func extractFieldNameAndRemaining(s string) (string, string) {
	i := 0
	for i < len(s) && s[i] != '[' && s[i] != '.' {
		if !isIdentChar(s[i]) {
			break
		}
		i++
	}
	return s[:i], s[i:]
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func splitTopLevel(s string) []string {
	var parts []string
	var current strings.Builder
	depth := 0
	for _, c := range s {
		switch c {
		case '[', '{', '(':
			depth++
			current.WriteRune(c)
		case ']', '}', ')':
			depth--
			current.WriteRune(c)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(c)
			}
		default:
			current.WriteRune(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
