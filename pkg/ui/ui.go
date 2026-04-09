package ui

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"
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
	fmt.Fprint(os.Stderr, "\r" + strings.Repeat(" ", 80) + "\r")
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