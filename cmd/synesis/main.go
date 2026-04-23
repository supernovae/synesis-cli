package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/editor"
	"synesis.sh/synesis/pkg/preset"
	"synesis.sh/synesis/pkg/session"
	"synesis.sh/synesis/pkg/ui"
	"synesis.sh/synesis/pkg/watch"

	"github.com/fsnotify/fsnotify"
)

// Version is set by build
var Version = "dev"

// build info
var (
	BuildTime string
	BuildGit  string
)

func main() {
	// Parse global flags before subcommand
	showVersion := flag.Bool("version", false, "show version")
	noColor := flag.Bool("no-color", false, "disable color output")
	quiet := flag.Bool("quiet", false, "suppress non-essential output")
	profile := flag.String("profile", "", "use named profile")
	flag.Parse()

	// Always check version first
	if *showVersion {
		fmt.Printf("synesis version %s", Version)
		if BuildTime != "" {
			fmt.Printf(" (built %s)", BuildTime)
		}
		if BuildGit != "" {
			fmt.Printf(" (%s)", BuildGit)
		}
		fmt.Println()
		os.Exit(0)
	}

	// Configure UI
	ui.DisableSpinner = *quiet || *noColor

	// Get subcommand
	if flag.NArg() < 1 {
		printUsage()
		os.Exit(1)
	}

	cmd := strings.ToLower(flag.Arg(0))

	// Route to subcommand, passing remaining args
	args := flag.Args()[1:]
	var err error

	switch cmd {
	case "chat":
		err = runChat(args, *noColor, *quiet, *profile)
	case "ask":
		err = runAsk(args, *noColor, *quiet, *profile)
	case "session":
		err = runSession(args, *noColor, *quiet, *profile)
	case "models":
		err = runModels(args, *noColor, *quiet, *profile)
	case "config":
		err = runConfig(args, *noColor, *quiet, *profile)
	case "auth":
		err = runAuth(args, *noColor, *quiet, *profile)
	case "extract":
		err = runExtract(args, *noColor, *quiet, *profile)
	case "summarize":
		err = runSummarize(args, *noColor, *quiet, *profile)
	case "commit-message":
		err = runCommitMessage(args, *noColor, *quiet, *profile)
	case "review":
		err = runReview(args, *noColor, *quiet, *profile)
	case "pr-summary":
		err = runPRSummary(args, *noColor, *quiet, *profile)
	case "release-notes":
		err = runReleaseNotes(args, *noColor, *quiet, *profile)
	case "explain-commit":
		err = runExplainCommit(args, *noColor, *quiet, *profile)
	case "doctor":
		err = runDoctor(args, *noColor, *quiet, *profile)
	case "profile":
		err = runProfile(args, *noColor, *quiet)
	case "template":
		err = runTemplate(args, *noColor, *quiet, *profile)
	case "repl":
		err = runREPL(args, *noColor, *quiet, *profile)
	case "presets":
		err = runPresets(args, *noColor, *quiet, *profile)
	case "editor":
		err = runEditor(args, *noColor, *quiet, *profile)
	case "watch":
		err = runWatch(args, *noColor, *quiet, *profile)
	case "completion":
		err = runCompletion(flag.Args()[1:], *noColor, *quiet)
	case "help", "--help", "-h":
		printUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		handleError(err)
	}
}

func handleError(err error) {
	if err == nil {
		return
	}

	// Don't print error in quiet mode for some cases
	exitCode := 1

	switch err.(type) {
	case *config.ValidationError:
		exitCode = 2
	case *api.HTTPError:
		exitCode = 3
	case *api.APIError:
		exitCode = 3
	default:
		// Check for timeout/cancellation
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "canceled") {
			exitCode = 4
		} else if strings.Contains(err.Error(), "parse") || strings.Contains(err.Error(), "json") {
			exitCode = 5
		}
	}

	ui.Error("%v", err)
	os.Exit(exitCode)
}

func printUsage() {
	fmt.Printf(`synesis - Shell-friendly AI CLI

Usage: synesis <command> [options]

Commands:
  chat             Start an interactive chat session
  ask              One-shot prompt/answer mode
  session          Manage chat sessions
  models           List available models
  config           Show configuration
  auth             Configure authentication
  extract          Extract structured fields from input
  summarize        Summarize stdin, files, or prompt
  commit-message   Generate commit message from diff
  review           Review code changes
  pr-summary       Summarize commits between two refs
  release-notes    Generate release notes from git history
  explain-commit   Explain a commit or series of commits
  doctor           Run diagnostics
  profile          Manage configuration profiles
  template         Manage prompt templates
  repl             Interactive REPL mode
  presets          List available system presets
  editor           Edit content in $EDITOR
  watch            Watch files for changes
  completion       Generate shell completion scripts

Options:
  -version         Show version information
  -quiet           Suppress non-essential output
  -no-color        Disable color output
  -profile string  Use named profile

Run 'synesis <command> --help' for more details.

`)
}

// Global client for reuse
var globalClient api.Client

func getClient(profileName string) (api.Client, error) {
	if globalClient != nil {
		return globalClient, nil
	}

	cfg, err := config.Resolve(profileName)
	if err != nil {
		return nil, err
	}

	if err := cfg.Cfg.Validate(); err != nil {
		return nil, err
	}

	globalClient = api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	return globalClient, nil
}

// Global session store
var globalSessionStore *session.Store

func getSessionStore() (*session.Store, error) {
	if globalSessionStore != nil {
		return globalSessionStore, nil
	}

	dir := sessionDir()
	store, err := session.NewStore(dir)
	if err != nil {
		return nil, err
	}

	globalSessionStore = store
	return store, nil
}

func sessionDir() string {
	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			xdgData = filepath.Join(home, ".local", "share")
		}
	}
	if xdgData == "" {
		// Fallback to current directory
		xdgData = "."
	}
	return filepath.Join(xdgData, "synesis", "sessions")
}

// Presets command
func runPresets(args []string, noColor, quiet bool, profile string) error {
	store, err := preset.NewStore("")
	if err != nil {
		return err
	}

	presets, err := store.List()
	if err != nil {
		return err
	}

	if len(presets) == 0 {
		fmt.Println("No presets found")
		return nil
	}

	fmt.Println("Available presets:")
	for _, p := range presets {
		fmt.Printf("  - %s", p.Name)
		if p.Description != "" {
			fmt.Printf(": %s", p.Description)
		}
		fmt.Println()
	}

	return nil
}

// Editor command
func runEditor(args []string, noColor, quiet bool, profile string) error {
	if len(args) < 1 {
		return fmt.Errorf("editor command requires a file path")
	}

	file := args[0]
	return editor.EditFileInEditor(file)
}

// Watch command
func runWatch(args []string, noColor, quiet bool, profile string) error {
	if len(args) < 1 {
		return fmt.Errorf("watch command requires at least one file path")
	}

	w, err := watch.NewWatcher(args)
	if err != nil {
		return err
	}

	fmt.Printf("Watching files: %v\n", args)
	fmt.Println("Press Ctrl+C to stop...")

	w.SetCallback(func(event fsnotify.Event) {
		fmt.Printf("%s: %s\n", event.Op, event.Name)
	})

	return w.Start()
}
