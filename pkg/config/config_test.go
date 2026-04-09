package config

import (
	"os"
	"path/filepath"
	"testing"
)

// All env vars that Resolve() reads
var allEnvKeys = []string{
	"SYNESIS_CONFIG_PATH",
	"SYNESIS_BASE_URL",
	"SYNESIS_API_KEY",
	"SYNESIS_MODEL",
	"SYNESIS_TIMEOUT",
	"SYNESIS_ENDPOINT",
	"SYNESIS_ORG_ID",
	"XDG_CONFIG_HOME",
	"XDG_DATA_HOME",
}

func unsetAllEnv() {
	for _, k := range allEnvKeys {
		os.Unsetenv(k)
	}
}

// clearEnv removes all Synesis-related env vars so tests are isolated.
func clearEnv(t *testing.T) {
	t.Helper()
	// Save current values so they can be restored by t.Cleanup
	for _, k := range allEnvKeys {
		old, ok := os.LookupEnv(k)
		os.Unsetenv(k)
		if ok {
			t.Cleanup(func() { os.Setenv(k, old) })
		} else {
			t.Cleanup(func() { os.Unsetenv(k) })
		}
	}
}

func TestResolve_Defaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cfg.Timeout != 120 {
		t.Errorf("expected timeout 120, got %d", cfg.Cfg.Timeout)
	}
	if cfg.Cfg.Endpoint != "chat/completions" {
		t.Errorf("expected endpoint chat/completions, got %s", cfg.Cfg.Endpoint)
	}
}

func TestResolve_EnvVars(t *testing.T) {
	clearEnv(t)

	os.Setenv("SYNESIS_BASE_URL", "https://test.example.com")
	os.Setenv("SYNESIS_API_KEY", "test-key")
	os.Setenv("SYNESIS_TIMEOUT", "60")
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cfg.BaseURL != "https://test.example.com" {
		t.Errorf("expected base_url from env, got %s", cfg.Cfg.BaseURL)
	}
	if cfg.Cfg.APIKey != "test-key" {
		t.Errorf("expected api_key from env, got %s", cfg.Cfg.APIKey)
	}
	if cfg.Cfg.Timeout != 60 {
		t.Errorf("expected timeout from env, got %d", cfg.Cfg.Timeout)
	}
}

func TestResolve_FileOverrides(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
base_url: https://file.example.com
api_key: file-key
timeout: 90
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	os.Setenv("SYNESIS_BASE_URL", "https://env.example.com")
	os.Setenv("SYNESIS_API_KEY", "env-key")
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cfg.BaseURL != "https://env.example.com" {
		t.Errorf("expected base_url from env, got %s", cfg.Cfg.BaseURL)
	}
	if cfg.Cfg.APIKey != "env-key" {
		t.Errorf("expected api_key from env, got %s", cfg.Cfg.APIKey)
	}
	if cfg.Cfg.Timeout != 90 {
		t.Errorf("expected timeout 90 from file, got %d", cfg.Cfg.Timeout)
	}
}

func TestResolve_InvalidYAML(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	invalidContent := `
base_url: [unclosed
  broken: yaml
`
	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("invalid YAML should not cause fatal error, got: %v", err)
	}
	if cfg.Cfg.Timeout != 120 {
		t.Errorf("expected default timeout 120, got %d", cfg.Cfg.Timeout)
	}
}

func TestResolve_PermissionDenied(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `base_url: https://test.example.com
`
	if err := os.WriteFile(configPath, []byte(configContent), 0000); err != nil {
		t.Skip("cannot create unreadable file on this system")
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		return
	}
	_ = cfg
}

func TestResolve_InvalidTimeoutValue(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `timeout: not-a-number
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("invalid timeout should not cause fatal error, got: %v", err)
	}
	if cfg.Cfg.Timeout != 120 {
		t.Errorf("expected default timeout 120 on parse failure, got %d", cfg.Cfg.Timeout)
	}
}

func TestResolve_NegativeTimeout(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `timeout: -5
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("negative timeout should not cause fatal error, got: %v", err)
	}
	// Negative timeout is ignored by the file loader (> 0 guard),
	// so the resolved config falls back to default Timeout=120.
	if cfg.Cfg.Timeout != 120 {
		t.Errorf("expected default timeout 120 on negative value, got %d", cfg.Cfg.Timeout)
	}
}

func TestResolve_EmptyConfigFile(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("empty config file should not cause error, got: %v", err)
	}
	if cfg.Cfg.Timeout != 120 || cfg.Cfg.BaseURL != "" || cfg.Cfg.Endpoint != "chat/completions" {
		t.Errorf("expected all defaults, got timeout=%d base_url=%s endpoint=%s",
			cfg.Cfg.Timeout, cfg.Cfg.BaseURL, cfg.Cfg.Endpoint)
	}
}

func TestResolve_ConfigPathPrecedence(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	xdgConfig := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(xdgConfig, 0755); err != nil {
		t.Fatalf("failed to create xdg config dir: %v", err)
	}
	configContent := `base_url: https://file.example.com`

	os.Setenv("XDG_CONFIG_HOME", xdgConfig)
	configPath := filepath.Join(xdgConfig, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cfg.BaseURL != "https://file.example.com" {
		t.Errorf("expected base_url from config file, got %s", cfg.Cfg.BaseURL)
	}
	if len(cfg.Sources) == 0 {
		t.Error("expected at least one config source")
	}
}

func TestConfig_Validate_MissingAPIKey(t *testing.T) {
	cfg := &Config{
		BaseURL: "https://api.example.com",
		APIKey:  "",
		Timeout: 120,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected Validate() to reject missing API key")
	}
}

func TestConfig_Validate_MissingBaseURL(t *testing.T) {
	cfg := &Config{
		BaseURL: "",
		APIKey:  "test-key",
		Timeout: 120,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected Validate() to reject missing base URL")
	}
}

func TestConfig_Validate_ZeroTimeout(t *testing.T) {
	cfg := &Config{
		BaseURL: "https://api.example.com",
		APIKey:  "test-key",
		Timeout: 0,
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected Validate() to reject zero timeout")
	}
}

func TestConfig_TimeoutDuration(t *testing.T) {
	cfg := &Config{Timeout: 120}
	if d := cfg.TimeoutDuration(); d != 120*1e9 {
		t.Errorf("expected 120s, got %v", d)
	}
}

func TestConfigPaths(t *testing.T) {
	clearEnv(t)

	paths := configPaths()
	if len(paths) == 0 {
		t.Error("expected at least one config path")
	}
	found := false
	for _, p := range paths {
		if p != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one non-empty config path")
	}
}

func TestResolve_EnvTimeoutTakesPrecedence(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `timeout: 90
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	os.Setenv("SYNESIS_TIMEOUT", "45")
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cfg.Timeout != 45 {
		t.Errorf("expected timeout 45 from env, got %d", cfg.Cfg.Timeout)
	}
}

func TestResolve_AllEnvVarsSet(t *testing.T) {
	clearEnv(t)

	os.Setenv("SYNESIS_BASE_URL", "https://all-env.example.com")
	os.Setenv("SYNESIS_API_KEY", "super-secret-key")
	os.Setenv("SYNESIS_ENDPOINT", "v1/chat/completions")
	os.Setenv("SYNESIS_TIMEOUT", "300")
	os.Setenv("SYNESIS_MODEL", "test-model")
	os.Setenv("SYNESIS_ORG_ID", "test-org")
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cfg.BaseURL != "https://all-env.example.com" {
		t.Errorf("expected base_url from env, got %s", cfg.Cfg.BaseURL)
	}
	if cfg.Cfg.APIKey != "super-secret-key" {
		t.Errorf("expected api_key from env, got %s", cfg.Cfg.APIKey)
	}
	if cfg.Cfg.Endpoint != "v1/chat/completions" {
		t.Errorf("expected endpoint from env, got %s", cfg.Cfg.Endpoint)
	}
	if cfg.Cfg.Timeout != 300 {
		t.Errorf("expected timeout from env, got %d", cfg.Cfg.Timeout)
	}
	if cfg.Cfg.Model != "test-model" {
		t.Errorf("expected model from env, got %s", cfg.Cfg.Model)
	}
	if cfg.Cfg.OrgID != "test-org" {
		t.Errorf("expected org_id from env, got %s", cfg.Cfg.OrgID)
	}
}

func TestResolve_MalformedYAML_LeadingWhitespace(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `  base_url: https://test.example.com
timeout: notint
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		return
	}
	_ = cfg
}

func TestResolve_NonExistentConfigFile(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("non-existent config should not error, got: %v", err)
	}
	if cfg.Cfg.Timeout != 120 {
		t.Errorf("expected default timeout 120, got %d", cfg.Cfg.Timeout)
	}
}

func TestResolve_BothFileAndEnvEndpoint(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `endpoint: v1/custom/chat
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	os.Setenv("SYNESIS_ENDPOINT", "v1/env/endpoint")
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cfg.Endpoint != "v1/env/endpoint" {
		t.Errorf("expected endpoint from env, got %s", cfg.Cfg.Endpoint)
	}
}

func TestLoadedConfig_ReportsSources(t *testing.T) {
	clearEnv(t)

	os.Setenv("SYNESIS_API_KEY", "source-test-key")
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Sources) == 0 {
		t.Error("expected at least one source to be reported")
	}
	found := false
	for _, s := range cfg.Sources {
		if s == "env:SYNESIS_API_KEY" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected env source in sources list, got %v", cfg.Sources)
	}
}

func TestResolve_EnvOverridesFileForAllFields(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
base_url: https://file.example.com
api_key: file-key
endpoint: v1/file/completions
timeout: 10
model: file-model
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	os.Setenv("SYNESIS_BASE_URL", "https://env.example.com")
	os.Setenv("SYNESIS_ENDPOINT", "v1/env/completions")
	os.Setenv("SYNESIS_TIMEOUT", "999")
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cfg.BaseURL != "https://env.example.com" {
		t.Errorf("expected base_url from env, got %s", cfg.Cfg.BaseURL)
	}
	if cfg.Cfg.Endpoint != "v1/env/completions" {
		t.Errorf("expected endpoint from env, got %s", cfg.Cfg.Endpoint)
	}
	if cfg.Cfg.Timeout != 999 {
		t.Errorf("expected timeout from env, got %d", cfg.Cfg.Timeout)
	}
	if cfg.Cfg.APIKey != "file-key" {
		t.Errorf("expected api_key from file, got %s", cfg.Cfg.APIKey)
	}
	if cfg.Cfg.Model != "file-model" {
		t.Errorf("expected model from file, got %s", cfg.Cfg.Model)
	}
}

func TestResolve_ConcurrentCalls(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `base_url: https://concurrent.example.com`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	errCh := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func() {
			cfg, err := Resolve()
			if err != nil {
				errCh <- err
				return
			}
			if cfg.Cfg.BaseURL != "https://concurrent.example.com" {
				errCh <- err
				return
			}
			errCh <- nil
		}()
	}
	for i := 0; i < 10; i++ {
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("concurrent call %d failed: %v", i, err)
			}
		}
	}
}

func TestResolve_LargeTimeoutValue(t *testing.T) {
	clearEnv(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `timeout: 8640000
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	os.Setenv("SYNESIS_CONFIG_PATH", configPath)
	defer clearEnv(t)

	cfg, err := Resolve()
	if err != nil {
		t.Fatalf("large timeout should not cause fatal error, got: %v", err)
	}
	if cfg.Cfg.Timeout != 8640000 {
		t.Errorf("expected timeout 8640000, got %d", cfg.Cfg.Timeout)
	}
}
