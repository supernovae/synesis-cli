package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
)

// IsTerminal returns true if stdout is a terminal
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// IsStderrTerminal returns true if stderr is a terminal
func IsStderrTerminal() bool {
	return isatty.IsTerminal(os.Stderr.Fd()) || isatty.IsCygwinTerminal(os.Stderr.Fd())
}

// IsStdinTerminal returns true if stdin is a terminal
func IsStdinTerminal() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
}

// OutputMode represents the output format
type OutputMode int

const (
	OutputText OutputMode = iota
	OutputJSON
	OutputNDJSON
)

// RenderMode represents the response rendering mode
type RenderMode int

const (
	RenderPlain RenderMode = iota
	RenderMarkdown
	RenderRaw
)

// ParseRenderMode parses a render mode string
func ParseRenderMode(s string) (RenderMode, error) {
	switch strings.ToLower(s) {
	case "plain", "text":
		return RenderPlain, nil
	case "markdown", "md":
		return RenderMarkdown, nil
	case "raw":
		return RenderRaw, nil
	default:
		return RenderPlain, fmt.Errorf("unknown render mode: %s", s)
	}
}

// Config holds UI configuration
type Config struct {
	NoColor   bool
	Quiet     bool
	Output    OutputMode
	Raw       bool
	IsTTY     bool
}

// DetectConfig detects terminal capabilities
func DetectConfig(noColor, quiet bool) *Config {
	return &Config{
		NoColor: noColor,
		Quiet:   quiet,
		IsTTY:   IsTerminal(),
	}
}

// spinner frames for terminal animation
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner provides terminal spinner
type Spinner struct {
	frame   int
	enabled bool
}

// NewSpinner creates a new spinner
func NewSpinner() *Spinner {
	return &Spinner{
		enabled: IsTerminal() && !DisableSpinner,
	}
}

// Tick advances the spinner
func (s *Spinner) Tick() string {
	if !s.enabled {
		return ""
	}
	frame := spinnerFrames[s.frame%len(spinnerFrames)]
	s.frame++
	return frame + " "
}

// Stop clears spinner state
func (s *Spinner) Stop() {
	if !s.enabled {
		return
	}
	fmt.Fprint(os.Stderr, "\r"+strings.Repeat(" ", 80)+"\r")
}

var DisableSpinner = false

// ProgressWriter writes progress to stderr
type ProgressWriter struct {
	enabled bool
	quiet   bool
}

// NewProgressWriter creates a progress writer
func NewProgressWriter(quiet bool) *ProgressWriter {
	return &ProgressWriter{
		enabled: IsTerminal() && !quiet,
		quiet:   quiet,
	}
}

// Write writes a progress message
func (p *ProgressWriter) Write(b []byte) (int, error) {
	if p.quiet || !p.enabled {
		return len(b), nil
	}
	os.Stderr.Write(b)
	return len(b), nil
}

// PrintOrQuiet prints to stderr unless quiet mode
func PrintOrQuiet(msg string, args ...interface{}) {
	if DisableSpinner {
		return
	}
	if !IsTerminal() {
		return
	}
	fmt.Fprintf(os.Stderr, msg, args...)
}

// Color codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[90m"
	ColorBold   = "\033[1m"
)

// Color wraps text with color codes based on settings
func Color(s string, color string, noColor bool) string {
	if noColor || !IsTerminal() {
		return s
	}
	return color + s + ColorReset
}

// Bold returns bold text
func Bold(s string, noColor bool) string {
	return Color(s, ColorBold, noColor)
}

// Error prints an error message to stderr
func Error(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, Color("Error: ", ColorRed, false)+msg+"\n", args...)
}

// Warning prints a warning to stderr
func Warning(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, Color("Warning: ", ColorYellow, false)+msg+"\n", args...)
}

// Platform detection
var (
	IsWindows = runtime.GOOS == "windows"
	IsMac     = runtime.GOOS == "darwin"
	IsLinux   = runtime.GOOS == "linux"
)

// RenderResponse renders a response according to the render mode
func RenderResponse(content string, mode RenderMode, noColor bool, isTTY bool) string {
	switch mode {
	case RenderRaw:
		return content
	case RenderMarkdown:
		if isTTY {
			return renderMarkdownText(content, noColor)
		}
		return content
	case RenderPlain:
		fallthrough
	default:
		return StripMarkdown(content)
	}
}

// renderMarkdownText applies basic markdown formatting for terminal display
func renderMarkdownText(content string, noColor bool) string {
	if noColor || !IsTerminal() {
		return content
	}

	result := content

	// Bold headers (# Header)
	headerRegex := regexp.MustCompile(`(?m)^(#+)\s+(.+)$`)
	result = headerRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := headerRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		return ColorBold + parts[2] + ColorReset
	})

	// Bold **text**
	boldRegex := regexp.MustCompile(`\*\*(.+?)\*\*`)
	result = boldRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := boldRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		return ColorBold + parts[1] + ColorReset
	})

	// Italic *text*
	italicRegex := regexp.MustCompile(`\*(.+?)\*`)
	result = italicRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := italicRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		return ColorCyan + parts[1] + ColorReset
	})

	// Inline code `code`
	codeRegex := regexp.MustCompile("`([^`]+)`")
	result = codeRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := codeRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		return ColorGray + parts[1] + ColorReset
	})

	return result
}

// StripMarkdown removes markdown formatting from text
func StripMarkdown(content string) string {
	result := content

	// Remove headers
	headerRegex := regexp.MustCompile(`(?m)^(#+)\s+(.+)$`)
	result = headerRegex.ReplaceAllString(result, "$2")

	// Remove bold
	result = strings.ReplaceAll(result, "**", "")

	// Remove italic
	italicRegex := regexp.MustCompile(`\*(.+?)\*`)
	result = italicRegex.ReplaceAllString(result, "$1")

	// Remove inline code
	result = strings.ReplaceAll(result, "`", "")

	// Remove links [text](url) -> text
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	result = linkRegex.ReplaceAllString(result, "$1")

	return result
}

// DryRunOutput holds dry-run display data
type DryRunOutput struct {
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	Endpoint   string `json:"endpoint"`
	Timeout    int    `json:"timeout"`
	APIKey     string `json:"api_key"`
	OrgID      string `json:"org_id,omitempty"`
	Messages   []api.Message `json:"messages"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens  int    `json:"max_tokens,omitempty"`
	Tools      []api.Tool `json:"tools,omitempty"`
	ToolChoice interface{} `json:"tool_choice,omitempty"`
}

// PrintDryRun displays the request that would be sent without making the API call
// If outputJSON is true, outputs to stdout as JSON; otherwise outputs human-readable to stderr
func PrintDryRun(cfg *config.LoadedConfig, req *api.ChatRequest, outputJSON bool) {
	// Prepare output with redacted secrets
	apiKey := cfg.Cfg.APIKey
	if apiKey != "" {
		apiKey = config.RedactedSecret
	}
	orgID := cfg.Cfg.OrgID
	if orgID != "" {
		orgID = config.RedactedSecret
	}

	dryRun := DryRunOutput{
		BaseURL:     cfg.Cfg.BaseURL,
		Model:       req.Model,
		Endpoint:    cfg.Cfg.Endpoint,
		Timeout:     cfg.Cfg.Timeout,
		APIKey:      apiKey,
		OrgID:       orgID,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
	}

	if outputJSON {
		data, err := json.MarshalIndent(dryRun, "", "  ")
		if err != nil {
			Error("dry-run JSON marshal error: %v", err)
			return
		}
		fmt.Println(string(data))
		return
	}

	// Human-readable output to stderr
	fmt.Fprintln(os.Stderr, "=== DRY RUN: Request that would be sent ===")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Resolved Configuration:\n")
	fmt.Fprintf(os.Stderr, "  Base URL: %s\n", dryRun.BaseURL)
	fmt.Fprintf(os.Stderr, "  Model: %s\n", dryRun.Model)
	fmt.Fprintf(os.Stderr, "  Endpoint: %s\n", dryRun.Endpoint)
	fmt.Fprintf(os.Stderr, "  Timeout: %ds\n", dryRun.Timeout)
	if dryRun.OrgID != "" {
		fmt.Fprintf(os.Stderr, "  Org ID: %s\n", dryRun.OrgID)
	}
	fmt.Fprintf(os.Stderr, "  API Key: %s\n", dryRun.APIKey)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Request Payload:\n")

	// Marshal just the request for display
	reqData, _ := json.MarshalIndent(map[string]interface{}{
		"model":         req.Model,
		"messages":      req.Messages,
		"temperature":   req.Temperature,
		"max_tokens":    req.MaxTokens,
		"tools":         req.Tools,
		"tool_choice":   req.ToolChoice,
	}, "", "  ")
	fmt.Fprintln(os.Stderr, string(reqData))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "=== END DRY RUN ===")
}

// PrintUsage displays token usage and latency information to stderr
func PrintUsage(model string, promptTokens, completionTokens, totalTokens int, latencyMs int64) {
	if totalTokens == 0 {
		return // No usage data
	}

	fmt.Fprintf(os.Stderr, "\n[%s] Tokens: %d in / %d out / %d total | Latency: %dms\n",
		model,
		promptTokens,
		completionTokens,
		totalTokens,
		latencyMs)
}
