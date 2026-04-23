package main

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/keychain"
	"synesis.sh/synesis/pkg/ui"
)

// runProfile implements the profile command
func runProfile(args []string, noColor, quiet bool) error {
	if len(args) == 0 {
		printProfileUsage()
		return nil
	}

	subcmd := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subcmd {
	case "list":
		return runProfileList(noColor, quiet)
	case "show":
		return runProfileShow(subArgs, noColor, quiet)
	case "create":
		return runProfileCreate(subArgs, noColor, quiet)
	case "delete":
		return runProfileDelete(subArgs, noColor, quiet)
	default:
		return fmt.Errorf("unknown profile subcommand: %s", subcmd)
	}
}

func runProfileList(noColor, quiet bool) error {
	cfg, err := config.Resolve("")
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	profiles := cfg.Cfg.ListProfiles()
	if len(profiles) == 0 {
		if !quiet {
			fmt.Println("No profiles configured.")
			fmt.Println("Use 'synesis profile create <name>' to add a profile.")
		}
		return nil
	}

	sort.Strings(profiles)

	if cfg.Cfg.DefaultProfile != "" {
		fmt.Printf("Default profile: %s\n\n", cfg.Cfg.DefaultProfile)
	}

	fmt.Println("Available profiles:")
	for _, name := range profiles {
		profile := cfg.Cfg.GetProfile(name)
		if profile == nil {
			continue
		}

		marker := "  "
		if name == cfg.Cfg.DefaultProfile {
			marker = "* "
		}

		desc := profile.Model
		if desc == "" {
			desc = "(no model specified)"
		}
		if profile.BaseURL != "" {
			desc += fmt.Sprintf(" @ %s", profile.BaseURL)
		}

		fmt.Printf("%s%s - %s\n", marker, name, desc)
	}

	return nil
}

func runProfileShow(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("profile show", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("profile name required")
	}

	profileName := fs.Arg(0)

	cfg, err := config.Resolve("")
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	profile := cfg.Cfg.GetProfile(profileName)
	if profile == nil {
		return fmt.Errorf("profile not found: %s", profileName)
	}

	fmt.Printf("Profile: %s\n", profileName)
	fmt.Println(strings.Repeat("=", 40))

	if profile.BaseURL != "" {
		fmt.Printf("Base URL: %s\n", profile.BaseURL)
	}
	if profile.APIKey != "" {
		fmt.Printf("API Key: %s\n", config.RedactedSecret)
	}
	if profile.Model != "" {
		fmt.Printf("Model: %s\n", profile.Model)
	}
	if profile.Timeout > 0 {
		fmt.Printf("Timeout: %ds\n", profile.Timeout)
	}
	if profile.OrgID != "" {
		fmt.Printf("Org ID: %s\n", profile.OrgID)
	}
	if profile.Endpoint != "" {
		fmt.Printf("Endpoint: %s\n", profile.Endpoint)
	}

	return nil
}

func runProfileCreate(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("profile create", flag.ExitOnError)
	baseURL := fs.String("base-url", "", "API base URL")
	apiKey := fs.String("api-key", "", "API key")
	model := fs.String("model", "", "default model")
	timeout := fs.Int("timeout", 0, "timeout in seconds")
	orgID := fs.String("org-id", "", "organization ID")
	endpoint := fs.String("endpoint", "", "API endpoint")
	setDefault := fs.Bool("default", false, "set as default profile")
	force := fs.Bool("force", false, "overwrite existing profile")

	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("profile name required")
	}

	profileName := fs.Arg(0)

	if profileName == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Load existing config
	cfg, err := config.Resolve("")
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Check if profile already exists
	if cfg.Cfg.ProfileExists(profileName) && !*force {
		return fmt.Errorf("profile '%s' already exists (use --force to overwrite)", profileName)
	}

	// Initialize profiles map if needed
	if cfg.Cfg.Profiles == nil {
		cfg.Cfg.Profiles = make(map[string]config.Profile)
	}

	// Create or update profile
	profile := config.Profile{
		Name:     profileName,
		BaseURL:  *baseURL,
		Model:    *model,
		Timeout:  *timeout,
		OrgID:    *orgID,
		Endpoint: *endpoint,
	}

	// Store API key in keychain instead of plaintext config
	if *apiKey != "" {
		if err := keychain.SetProfileAPIKey(profileName, *apiKey); err != nil {
			return fmt.Errorf("store API key in keychain: %w", err)
		}
	}

	cfg.Cfg.Profiles[profileName] = profile

	if *setDefault {
		cfg.Cfg.DefaultProfile = profileName
	}

	// Save config
	configPath := config.GetConfigPath()
	if err := config.SaveConfig(&cfg.Cfg, configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if !quiet {
		fmt.Printf("Profile '%s' created successfully.\n", profileName)
		if *setDefault {
			fmt.Println("Set as default profile.")
		}
		fmt.Printf("Config saved to: %s\n", configPath)
	}

	return nil
}

func runProfileDelete(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("profile delete", flag.ExitOnError)
	force := fs.Bool("force", false, "skip confirmation")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("profile name required")
	}

	profileName := fs.Arg(0)

	cfg, err := config.Resolve("")
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if !cfg.Cfg.ProfileExists(profileName) {
		return fmt.Errorf("profile not found: %s", profileName)
	}

	// Confirm deletion
	if !*force && ui.IsTerminal() {
		fmt.Printf("Are you sure you want to delete profile '%s'? [y/N] ", profileName)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	// Delete profile
	delete(cfg.Cfg.Profiles, profileName)

	// Delete keychain entry for this profile (best-effort)
	_ = keychain.DeleteProfileAPIKey(profileName)

	// Clear default if this was the default
	if cfg.Cfg.DefaultProfile == profileName {
		cfg.Cfg.DefaultProfile = ""
	}

	// Save config
	configPath := config.GetConfigPath()
	if err := config.SaveConfig(&cfg.Cfg, configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	if !quiet {
		fmt.Printf("Profile '%s' deleted.\n", profileName)
	}

	return nil
}

func printProfileUsage() {
	fmt.Print(`synesis profile - Manage configuration profiles

Usage: synesis profile <subcommand> [options]

Subcommands:
  list              List all profiles
  show <name>       Show profile details
  create <name>     Create a new profile
  delete <name>     Delete a profile

Options for 'create':
  -base-url string   API base URL
  -api-key string    API key (will be stored in plaintext)
  -model string      Default model for this profile
  -timeout int       Timeout in seconds
  -org-id string     Organization ID
  -endpoint string   API endpoint (chat/completions or responses)
  -default           Set as default profile
  -force             Overwrite existing profile

Examples:
  synesis profile list
  synesis profile show fast
  synesis profile create fast --model gpt-4o-mini --timeout 30
  synesis profile create local --base-url http://localhost:11434/v1 --model llama2
  synesis profile delete fast

`)
}
