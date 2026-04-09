package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/ui"
)

// runModels implements the models command
func runModels(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("models", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output JSON")

	fs.Parse(args)

	// Load config
	cfg, err := config.Resolve(profileName)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if err := cfg.Cfg.Validate(); err != nil {
		return err
	}

	// Create client
	cli := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	defer cli.Close()

	// Fetch models
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := cli.ListModels(ctx)
	if err != nil {
		if !quiet {
			ui.Warning("Model listing not available: %v", err)
		}
		return nil
	}

	if len(models) == 0 {
		if !quiet {
			fmt.Println("No models available")
		}
		return nil
	}

	// Output
	if *jsonOutput {
		data, _ := json.Marshal(models)
		fmt.Println(string(data))
	} else {
		fmt.Println("Available models:")
		for _, m := range models {
			fmt.Printf("  %s\n", m.ID)
		}
	}

	return nil
}