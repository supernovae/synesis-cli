package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"synesis.sh/synesis/pkg/config"
)

// runConfig implements the config command
func runConfig(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("config", flag.ExitOnError)
	showSources := fs.Bool("sources", false, "show configuration sources")
	jsonOutput := fs.Bool("json", false, "output JSON")
	validate := fs.Bool("validate", false, "validate configuration")

	fs.Parse(args)

	cfg, err := config.Resolve()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Validate if requested
	if *validate {
		if err := cfg.Cfg.Validate(); err != nil {
			return &config.ValidationError{Msg: err.Error()}
		}
		if !quiet {
			fmt.Println("Configuration is valid")
		}
		return nil
	}

	// Show sources
	if *showSources {
		if *jsonOutput {
			data, _ := json.Marshal(cfg.Sources)
			fmt.Println(string(data))
		} else {
			fmt.Println("Configuration sources:")
			for _, src := range cfg.Sources {
				fmt.Printf("  - %s\n", src)
			}
		}
		return nil
	}

	// Show effective config
	eff := cfg.Cfg.EffectiveConfig()
	if *jsonOutput {
		data, _ := json.Marshal(eff)
		fmt.Println(string(data))
	} else {
		fmt.Println("Effective configuration:")
		fmt.Printf("  Base URL:  %s\n", eff.BaseURL)
		fmt.Printf("  API Key:   %s\n", eff.APIKey)
		fmt.Printf("  Model:     %s\n", eff.Model)
		fmt.Printf("  Timeout:   %d seconds\n", eff.Timeout)
		fmt.Printf("  Endpoint:  %s\n", eff.Endpoint)
		if cfg.File != "" {
			fmt.Printf("  Config File: %s\n", cfg.File)
		}
		if cfg.EnvUsed {
			fmt.Println("  Note: Environment variables override config file")
		}
	}

	return nil
}