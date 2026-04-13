package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/keychain"
	"synesis.sh/synesis/pkg/ui"
)

// runAuth implements the auth command for configuring authentication
func runAuth(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("auth", flag.ExitOnError)
	baseURL := fs.String("url", "", "API base URL")
	token := fs.String("token", "", "API token")
	setToken := fs.String("set-token", "", "set token in config file")
	setURL := fs.String("set-url", "", "set URL in config file")
	printToken := fs.Bool("show-token", false, "show current token")
	useKeychain := fs.Bool("use-keychain", false, "store/retrieve token from system keychain")
	clearKeychain := fs.Bool("clear-keychain", false, "remove token from system keychain")

	fs.Parse(args)

	// Handle clear-keychain
	if *clearKeychain {
		if err := keychain.DeleteAPIKey(); err != nil {
			return fmt.Errorf("clear keychain: %w", err)
		}
		if !quiet {
			fmt.Println("Token removed from system keychain")
		}
		return nil
	}

	cfg, err := config.Resolve(profileName)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Handle set-token
	if *setToken != "" {
		if *useKeychain {
			if err := keychain.SetAPIKey(*setToken); err != nil {
				return fmt.Errorf("keychain: %w", err)
			}
			if !quiet {
				fmt.Println("Token stored in system keychain")
			}
		} else {
			return writeConfigValue("api_key", *setToken, quiet)
		}
		return nil
	}

	// Handle set-url
	if *setURL != "" {
		return writeConfigValue("base_url", *setURL, quiet)
	}

	// Show URL if set
	if *baseURL != "" || *token != "" {
		// Simple auth set just prints confirmation
		if !quiet {
			if *baseURL != "" {
				cfg.Cfg.BaseURL = *baseURL
			}
			if *token != "" {
				if *useKeychain {
					if err := keychain.SetAPIKey(*token); err != nil {
						return fmt.Errorf("keychain: %w", err)
					}
					fmt.Println("URL set; token stored in system keychain")
					return nil
				}
				cfg.Cfg.APIKey = *token
			}
			if err := cfg.Cfg.Validate(); err != nil {
				return err
			}
			fmt.Println("Authentication configured")
		}
		return nil
	}

	// Default: show current auth status
	if !quiet {
		fmt.Println("Current authentication status:")

		if cfg.Cfg.BaseURL != "" {
			fmt.Printf("  %s  %s\n",
				ui.Color("URL:", ui.ColorCyan, false),
				cfg.Cfg.BaseURL)
		} else {
			fmt.Printf("  %s  %s\n",
				ui.Color("URL:", ui.ColorCyan, false),
				ui.Color("not set", ui.ColorGray, false))
		}

		// Check keychain first if --use-keychain or keychain has a key
		hasKC, kcErr := keychain.HasAPIKey()
		tokenFromKeychain := hasKC && kcErr == nil

		if *printToken {
			if tokenFromKeychain {
				if key, err := keychain.GetAPIKey(); err == nil {
					fmt.Printf("  %s  %s\n",
						ui.Color("Token:", ui.ColorCyan, false),
						key)
				}
			} else if cfg.Cfg.APIKey != "" {
				fmt.Printf("  %s  %s\n",
					ui.Color("Token:", ui.ColorCyan, false),
					cfg.Cfg.APIKey)
			} else {
				fmt.Printf("  %s  %s\n",
					ui.Color("Token:", ui.ColorCyan, false),
					ui.Color("not set", ui.ColorGray, false))
			}
		} else if tokenFromKeychain {
			fmt.Printf("  %s  %s\n",
				ui.Color("Token:", ui.ColorCyan, false),
				ui.Color("[SET] (keychain)", ui.ColorGreen, false))
		} else if cfg.Cfg.APIKey != "" {
			fmt.Printf("  %s  %s\n",
				ui.Color("Token:", ui.ColorCyan, false),
				ui.Color("[SET] (config)", ui.ColorGreen, false))
		} else {
			fmt.Printf("  %s  %s\n",
				ui.Color("Token:", ui.ColorCyan, false),
				ui.Color("not set", ui.ColorGray, false))
		}
	}

	return nil
}

func writeConfigValue(key, value string, quiet bool) error {
	// Determine config file path
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	var configPath string
	if xdgConfig != "" {
		configPath = filepath.Join(xdgConfig, "synesis", "config.yaml")
	} else {
		home, _ := os.UserHomeDir()
		if home != "" {
			configPath = filepath.Join(home, ".config", "synesis", "config.yaml")
		} else {
			return fmt.Errorf("cannot determine config path")
		}
	}

	// Create directory if needed
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Load existing or create new
	var cfg config.Config
	if data, err := os.ReadFile(configPath); err == nil {
		yaml.Unmarshal(data, &cfg)
	}

	// Set value
	switch key {
	case "api_key":
		cfg.APIKey = value
	case "base_url":
		cfg.BaseURL = value
	case "model":
		cfg.Model = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	// Write config
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	if !quiet {
		fmt.Printf("Set %s in %s\n", key, configPath)
	}

	return nil
}
