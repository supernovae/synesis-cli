package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the Synesis CLI configuration
type Config struct {
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Model    string `yaml:"model,omitempty"`
	Timeout  int    `yaml:"timeout,omitempty"` // seconds
	OrgID    string `yaml:"org_id,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"` // chat/completions or responses
}

// EffectiveConfig returns a copy with secrets redacted
func (c *Config) EffectiveConfig() *Config {
	cp := *c
	if cp.APIKey != "" {
		cp.APIKey = RedactedSecret
	}
	return &cp
}

const RedactedSecret = "[REDACTED]"

// LoadedConfig holds resolved configuration and source info
type LoadedConfig struct {
	Sources []string // which sources were used
	File    string   // config file path if used
	Cfg     Config   // resolved config
	EnvUsed bool     // whether env vars were used
}

// Resolve merges config from file, env vars, and defaults
func Resolve() (*LoadedConfig, error) {
	cfg := Config{
		Timeout:  120,
		Endpoint: "chat/completions",
	}
	sources := []string{"defaults"}
	envUsed := false

	// Load config file
	paths := configPaths()
	var loadedFile string

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var fileCfg Config
			if err := yaml.Unmarshal(data, &fileCfg); err == nil {
				if fileCfg.BaseURL != "" {
					cfg.BaseURL = fileCfg.BaseURL
					loadedFile = p
				}
				if fileCfg.APIKey != "" {
					cfg.APIKey = fileCfg.APIKey
				}
				if fileCfg.Model != "" {
					cfg.Model = fileCfg.Model
				}
				if fileCfg.Timeout > 0 {
					cfg.Timeout = fileCfg.Timeout
				}
				if fileCfg.OrgID != "" {
					cfg.OrgID = fileCfg.OrgID
				}
				if fileCfg.Endpoint != "" {
					cfg.Endpoint = fileCfg.Endpoint
				}
				sources = append(sources, "config:"+filepath.Base(p))
				break // first valid config file wins
			}
		}
	}

	// Environment variables override file
	if v := os.Getenv("SYNESIS_BASE_URL"); v != "" {
		cfg.BaseURL = v
		sources = append(sources, "env:SYNESIS_BASE_URL")
		envUsed = true
	}
	if v := os.Getenv("SYNESIS_API_KEY"); v != "" {
		cfg.APIKey = v
		sources = append(sources, "env:SYNESIS_API_KEY")
		envUsed = true
	}
	if v := os.Getenv("SYNESIS_MODEL"); v != "" {
		cfg.Model = v
		sources = append(sources, "env:SYNESIS_MODEL")
	}
	if v := os.Getenv("SYNESIS_TIMEOUT"); v != "" {
		var to int
		if _, err := fmt.Sscanf(v, "%d", &to); err == nil {
			cfg.Timeout = to
			sources = append(sources, "env:SYNESIS_TIMEOUT")
		}
	}
	if v := os.Getenv("SYNESIS_ENDPOINT"); v != "" {
		cfg.Endpoint = v
		sources = append(sources, "env:SYNESIS_ENDPOINT")
	}
	if v := os.Getenv("SYNESIS_ORG_ID"); v != "" {
		cfg.OrgID = v
		sources = append(sources, "env:SYNESIS_ORG_ID")
	}

	return &LoadedConfig{
		Sources: sources,
		File:    loadedFile,
		Cfg:     cfg,
		EnvUsed: envUsed,
	}, nil
}

func configPaths() []string {
	var paths []string
	if p := os.Getenv("SYNESIS_CONFIG_PATH"); p != "" {
		paths = append(paths, p)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "synesis", "config.yaml"))
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		paths = append(paths, filepath.Join(home, ".config", "synesis", "config.yaml"))
	}
	return paths
}

// Validate checks configuration is sufficient for API calls
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return &ValidationError{Msg: "base_url is required"}
	}
	if c.APIKey == "" {
		return &ValidationError{Msg: "api_key is required"}
	}
	if c.Timeout <= 0 {
		return &ValidationError{Msg: "timeout must be positive"}
	}
	return nil
}

// ValidationError represents a configuration validation failure
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string {
	return e.Msg
}

// TimeoutDuration returns the timeout as a time.Duration
func (c *Config) TimeoutDuration() time.Duration {
	if c.Timeout <= 0 {
		return 120 * time.Second
	}
	return time.Duration(c.Timeout) * time.Second
}