package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/ui"
)

// runDoctor runs diagnostics
func runDoctor(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	_ = fs.Bool("v", false, "verbose output")
	fix := fs.Bool("fix", false, "attempt to fix issues")

	fs.Parse(args)

	var issues, warnings int

	printCheck := func(name string, ok bool, msg string) {
		status := ui.Color("PASS", ui.ColorGreen, false)
		if !ok {
			status = ui.Color("FAIL", ui.ColorRed, false)
			issues++
		}
		fmt.Printf("  [%s] %s", status, name)
		if msg != "" {
			fmt.Printf(": %s", msg)
		}
		fmt.Println()
	}

	fmt.Println("Running diagnostics...")

	// 1. Config check
	fmt.Println("1. Configuration:")
	cfg, err := config.Resolve(profileName)
	if err != nil {
		printCheck("config loading", false, err.Error())
	} else {
		printCheck("config loading", true, "")
		if cfg.ProfileUsed != "" && !quiet {
			fmt.Printf("   Using profile: %s\n", cfg.ProfileUsed)
		}
	}

	// 2. Validate config
	validConfig := cfg.Cfg.Validate() == nil
	if validConfig {
		printCheck("config validation", true, "")
	} else {
		printCheck("config validation", false, "missing required fields")
	}

	// 3. Check config file permissions
	if cfg.File != "" {
		info, err := os.Stat(cfg.File)
		if err == nil {
			perm := info.Mode().Perm()
			if perm&0o077 != 0 {
				warnMsg := "config file has overly broad permissions"
				printCheck("config permissions", false, warnMsg)
				if *fix {
					os.Chmod(cfg.File, 0o600)
					printCheck("config permissions", true, "fixed")
				}
			} else {
				printCheck("config permissions", true, "")
			}
		}
	}

	// 4. Environment variables
	fmt.Println("\n2. Environment:")
	envChecks := []string{
		"SYNESIS_BASE_URL",
		"SYNESIS_API_KEY",
		"SYNESIS_MODEL",
		"SYNESIS_TIMEOUT",
	}
	for _, e := range envChecks {
		if os.Getenv(e) != "" {
			printCheck(e, true, "[SET]")
		} else {
			printCheck(e, true, "[not set]")
		}
	}

	// 5. Network connectivity
	fmt.Println("\n3. Network:")
	if cfg.Cfg.BaseURL != "" {
		// Make HEAD request
		client := &http.Client{Timeout: 10 * time.Second}
		req, _ := http.NewRequest("GET", cfg.Cfg.BaseURL+"/v1/models", nil)
		resp, err := client.Do(req)
		if err == nil {
			printCheck("API endpoint reachable", true, fmt.Sprintf("HTTP %d", resp.StatusCode))
			resp.Body.Close()
		} else {
			printCheck("API endpoint reachable", false, err.Error())
		}
	}

	// 6. API authentication
	fmt.Println("\n4. Authentication:")
	if cfg.Cfg.BaseURL != "" && cfg.Cfg.APIKey != "" {
		cli := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Try to list models
		models, err := cli.ListModels(ctx)
		if err == nil {
			printCheck("API authentication", true, fmt.Sprintf("%d models available", len(models)))
		} else {
			errMsg := err.Error()
			if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "authentication") {
				printCheck("API authentication", false, "invalid token")
			} else {
				printCheck("API authentication", false, errMsg)
			}
		}
	} else if cfg.Cfg.APIKey == "" {
		printCheck("API authentication", false, "no API key configured")
	}

	// 7. Session storage
	fmt.Println("\n5. Storage:")
	store, err := getSessionStore()
	if err == nil {
		printCheck("session directory", true, "accessible")
		sessions, _ := store.List()
		printCheck("session count", true, fmt.Sprintf("%d sessions", len(sessions)))
	} else {
		printCheck("session directory", false, err.Error())
	}

	// Summary
	fmt.Println()
	if issues > 0 {
		printSummary := ui.Color("FAILED", ui.ColorRed, false)
		fmt.Printf("Result: %s (%d issues, %d warnings)\n", printSummary, issues, warnings)
		return fmt.Errorf("diagnostics failed")
	} else if warnings > 0 {
		printSummary := ui.Color("WARNINGS", ui.ColorYellow, false)
		fmt.Printf("Result: %s (%d warnings)\n", printSummary, warnings)
	} else {
		printSummary := ui.Color("OK", ui.ColorGreen, false)
		fmt.Printf("Result: %s\n", printSummary)
	}

	return nil
}