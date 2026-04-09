package streaming

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Event represents a parsed SSE event
type Event struct {
	Event string `json:"event,omitempty"`
	Data  string `json:"data,omitempty"`
	ID    string `json:"id,omitempty"`
}

// Parser handles Server-Sent Events streaming
type Parser struct {
	// OnContent is called with each content chunk
	OnContent func(content string)
	// OnContentError is called on content parse errors (non-blocking)
	OnContentError func(err error)
	// Buffer for incomplete lines
	buf     []byte
	lastRune rune
}

// NewParser creates a streaming parser
func NewParser(onContent func(string)) *Parser {
	return &Parser{
		OnContent: onContent,
	}
}

// Parse reads and parses SSE events from the stream
func (p *Parser) Parse(ctx context.Context, r io.Reader) error {
	reader := bufio.NewReader(r)
	var lineBuf bytes.Buffer

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Flush any remaining buffer
				if lineBuf.Len() > 0 {
					p.processLine(lineBuf.Bytes())
				}
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		lineBuf.Write(line)

		if isPrefix {
			continue // line continues
		}

		p.processLine(lineBuf.Bytes())
		lineBuf.Reset()
	}
}

func (p *Parser) processLine(line []byte) {
	// Skip comments
	if len(line) > 0 && line[0] == ':' {
		return
	}

	// Parse "field: value" format
	parts := splitn[string](string(line), ":", 2)
	if len(parts) < 2 {
		return
	}

	field := strings.TrimSpace(parts[0])
	value := strings.TrimPrefix(parts[1], " ")

	switch field {
	case "data":
		if p.OnContent != nil && value != "" {
			onError := p.OnContentError
			p.OnContent(value)
			// Try to detect errors in content but don't fail
			if onError != nil {
				if strings.Contains(value, "[DONE]") {
					return
				}
				var check struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				if json.Unmarshal([]byte(value), &check); check.Error.Message != "" {
					onError(errors.New(check.Error.Message))
				}
			}
		}
	case "event":
		// Could handle custom events
	case "id":
		// Could handle reconnection
	}
}

func splitn[T string](s string, sep string, n int) []T {
	parts := strings.SplitN(s, sep, n)
	var result []T
	for _, p := range parts {
		result = append(result, T(p))
	}
	return result
}

// ChatResponse represents a parsed streaming response
type ChatResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// ParseChatDelta parses a streaming delta from JSON data
func ParseChatDelta(data string) (string, bool, error) {
	// Handle empty or whitespace-only data
	if strings.TrimSpace(data) == "" {
		return "", false, nil
	}

	// Handle [DONE] signal
	if data == "[DONE]" {
		return "", true, nil
	}

	var resp ChatResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return "", false, fmt.Errorf("parse error: %w", err)
	}

	if resp.Error != nil {
		return "", false, fmt.Errorf("API error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return "", false, nil
	}

	return resp.Choices[0].Delta.Content, false, nil
}

// IsUTF8Valid checks if content is valid UTF-8, fixing common issues
func IsUTF8Valid(s string) (string, bool) {
	if utf8.ValidString(s) {
		return s, true
	}
	// Try to fix common issues
	fixed := strings.ToValidUTF8(s, "")
	return fixed, fixed == s
}