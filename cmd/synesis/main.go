package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/session"
	"synesis.sh/synesis/pkg/ui"
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
		err = runChat(args, *noColor, *quiet)
	case "ask":
		err = runAsk(args, *noColor, *quiet)
	case "session":
		err = runSession(args, *noColor, *quiet)
	case "models":
		err = runModels(args, *noColor, *quiet)
	case "config":
		err = runConfig(args, *noColor, *quiet)
	case "auth":
		err = runAuth(args, *noColor, *quiet)
	case "extract":
		err = runExtract(args, *noColor, *quiet)
	case "summarize":
		err = runSummarize(args, *noColor, *quiet)
	case "commit-message":
		err = runCommitMessage(args, *noColor, *quiet)
	case "doctor":
		err = runDoctor(args, *noColor, *quiet)
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
  doctor           Run diagnostics

Options:
  -version         Show version information
  -quiet           Suppress non-essential output
  -no-color        Disable color output

Run 'synesis <command> --help' for more details.

`)
}

// Global client for reuse
var globalClient api.Client

func getClient() (api.Client, error) {
	if globalClient != nil {
		return globalClient, nil
	}

	cfg, err := config.Resolve()
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