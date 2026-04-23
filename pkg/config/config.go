package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
	"synesis.sh/synesis/pkg/keychain"
)

// Config represents the Synesis CLI configuration
type Config struct {
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Model    string `yaml:"model,omitempty"`
	Timeout  int    `yaml:"timeout,omitempty"` // seconds
	OrgID    string `yaml:"org_id,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"` // chat/completions or responses

	// Profile support
	DefaultProfile string            `yaml:"default_profile,omitempty"`
	Profiles       map[string]Profile `yaml:"profiles,omitempty"`
}

// Profile represents a named configuration profile
type Profile struct {
	Name     string `yaml:"name,omitempty"`
	BaseURL  string `yaml:"base_url,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Model    string `yaml:"model,omitempty"`
	Timeout  int    `yaml:"timeout,omitempty"`
	OrgID    string `yaml:"org_id,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
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

// DefaultModel is the fallback model used when no model is configured.
const DefaultModel = "gpt-4o-mini"

// LoadedConfig holds resolved configuration and source info
type LoadedConfig struct {
	Sources     []string // which sources were used
	File        string   // config file path if used
	Cfg         Config   // resolved config
	EnvUsed     bool     // whether env vars were used
	ProfileUsed string   // which profile was used (if any)
}

// Resolve merges config from file, env vars, and defaults
// If profileName is provided, it applies that profile's settings
func Resolve(profileName string) (*LoadedConfig, error) {
	cfg := Config{
		Timeout:  120,
		Endpoint: "chat/completions",
	}
	sources := []string{"defaults"}
	envUsed := false
	var profileUsed string

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
				if fileCfg.DefaultProfile != "" {
					cfg.DefaultProfile = fileCfg.DefaultProfile
				}
				if fileCfg.Profiles != nil {
					cfg.Profiles = fileCfg.Profiles
				}
				sources = append(sources, "config:"+filepath.Base(p))
				break // first valid config file wins
			}
		}
	}

	// Apply profile if specified via argument or default
	profileToApply := profileName
	if profileToApply == "" && cfg.DefaultProfile != "" {
		profileToApply = cfg.DefaultProfile
	}

	if profileToApply != "" && cfg.Profiles != nil {
		if profile, ok := cfg.Profiles[profileToApply]; ok {
			profileUsed = profileToApply
			// Profile settings override file config
			if profile.BaseURL != "" {
				cfg.BaseURL = profile.BaseURL
			}
			if profile.APIKey != "" {
				cfg.APIKey = profile.APIKey
			}
			if profile.Model != "" {
				cfg.Model = profile.Model
			}
			if profile.Timeout > 0 {
				cfg.Timeout = profile.Timeout
			}
			if profile.OrgID != "" {
				cfg.OrgID = profile.OrgID
			}
			if profile.Endpoint != "" {
				cfg.Endpoint = profile.Endpoint
			}
			sources = append(sources, "profile:"+profileToApply)
		}
	}

	// Environment variables override file and profile (highest precedence)
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

	// Fallback: try OS keychain if API key still not resolved
	if cfg.APIKey == "" {
		if kcKey, err := keychain.GetAPIKey(); err == nil && kcKey != "" {
			cfg.APIKey = kcKey
			sources = append(sources, "keychain:synesis")
		} else if !errors.Is(err, keychain.ErrNotFound) && !errors.Is(err, keychain.ErrNotSupported) {
			// Log keychain errors that aren't "not found" or "unsupported" (don't fail, just note)
			_ = err // silently ignore; keychain is best-effort
		}
	}

	return &LoadedConfig{
		Sources:     sources,
		File:        loadedFile,
		Cfg:         cfg,
		EnvUsed:     envUsed,
		ProfileUsed: profileUsed,
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

// GetProfile returns a profile by name, or nil if not found
func (c *Config) GetProfile(name string) *Profile {
	if c.Profiles == nil {
		return nil
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return nil
	}
	return &profile
}

// ListProfiles returns all profile names
func (c *Config) ListProfiles() []string {
	if c.Profiles == nil {
		return nil
	}
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	return names
}

// ProfileExists checks if a profile exists
func (c *Config) ProfileExists(name string) bool {
	return c.GetProfile(name) != nil
}

// SaveConfig saves the configuration to the specified path
func SaveConfig(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Create parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// GetConfigPath returns the primary config file path
func GetConfigPath() string {
	paths := configPaths()
	if len(paths) == 0 {
		return ""
	}

	// Use XDG path if available
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "synesis", "config.yaml")
	}

	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, ".config", "synesis", "config.yaml")
	}

	return paths[0]
}