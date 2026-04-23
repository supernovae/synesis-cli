package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/repl"
	"synesis.sh/synesis/pkg/ui"
)

// runREPL implements the repl command
func runREPL(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("repl", flag.ExitOnError)
	renderModeStr := fs.String("render", "plain", "render mode: plain, markdown, raw")
	fs.Parse(args)

	// Check TTY
	if !ui.IsTerminal() {
		return fmt.Errorf("REPL requires an interactive terminal")
	}

	// Load config
	cfg, err := config.Resolve(profileName)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if err := cfg.Cfg.Validate(); err != nil {
		return err
	}

	// Create client
	client := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	defer client.Close()

	// Get session store
	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

	// Parse render mode
	renderMode := ui.RenderPlain
	if *renderModeStr != "" {
		m, err := ui.ParseRenderMode(*renderModeStr)
		if err != nil {
			return fmt.Errorf("render mode: %w", err)
		}
		renderMode = m
	}

	// Create REPL
	r, err := repl.New(store, cfg, client, noColor, quiet, renderMode)
	if err != nil {
		return fmt.Errorf("create REPL: %w", err)
	}

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stdout, "\nInterrupted. Type /exit to quit.")
	}()

	// Run REPL
	err = r.Run()
	if err != nil && err.Error() != "EOF" {
		return err
	}

	return nil
}
