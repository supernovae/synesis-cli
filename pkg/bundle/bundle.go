package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Bundle represents a synesis bundle file
type Bundle struct {
	System      string            `yaml:"system,omitempty" json:"system,omitempty"`
	Prompt      string            `yaml:"prompt,omitempty" json:"prompt,omitempty"`
	Files       []File            `yaml:"files,omitempty" json:"files,omitempty"`
	Model       string            `yaml:"model,omitempty" json:"model,omitempty"`
	Temperature float64           `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	MaxTokens   int               `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`
	Endpoint    string            `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	OrgID       string            `yaml:"org_id,omitempty" json:"org_id,omitempty"`
	Timeout     int               `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Tools       []string          `yaml:"tools,omitempty" json:"tools,omitempty"`
	ToolChoice  string            `yaml:"tool_choice,omitempty" json:"tool_choice,omitempty"`
	Outputs     map[string]string `yaml:"outputs,omitempty" json:"outputs,omitempty"`
}

// File represents a file reference in a bundle
type File struct {
	Path   string `yaml:"path" json:"path"`
	Role   string `yaml:"role,omitempty" json:"role,omitempty"`
	Format string `yaml:"format,omitempty" json:"format,omitempty"`
}

// Load loads a bundle from a YAML file
func Load(path string) (*Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bundle file: %w", err)
	}

	return Parse(data, filepath.Dir(path))
}

// Parse parses bundle data from YAML bytes
// dir is the directory to resolve relative file paths against
func Parse(data []byte, dir string) (*Bundle, error) {
	var b Bundle
	if err := yaml.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parse bundle YAML: %w", err)
	}

	// Resolve file paths relative to bundle directory
	for i := range b.Files {
		if !filepath.IsAbs(b.Files[i].Path) {
			b.Files[i].Path = filepath.Join(dir, b.Files[i].Path)
		}
	}

	return &b, nil
}

// Validate checks the bundle is valid
func (b *Bundle) Validate() error {
	if b.Prompt == "" && b.System == "" {
		return fmt.Errorf("bundle must have at least prompt or system content")
	}

	// Validate files exist
	for _, f := range b.Files {
		if _, err := os.Stat(f.Path); err != nil {
			return fmt.Errorf("file not found: %s: %w", f.Path, err)
		}
	}

	return nil
}

// GetPrompt returns the combined prompt with file contents
func (b *Bundle) GetPrompt() (string, error) {
	var prompt strings.Builder

	if b.Prompt != "" {
		prompt.WriteString(b.Prompt)
	}

	// Add file contents
	for _, f := range b.Files {
		if f.Role == "context" || f.Role == "" {
			content, err := os.ReadFile(f.Path)
			if err != nil {
				return "", fmt.Errorf("read file %s: %w", f.Path, err)
			}
			if prompt.Len() > 0 {
				prompt.WriteString("\n\n")
			}
			prompt.WriteString(fmt.Sprintf("--- File: %s ---\n", f.Path))
			prompt.Write(content)
		}
	}

	return prompt.String(), nil
}

// GetSystem returns the system prompt
func (b *Bundle) GetSystem() string {
	return b.System
}

// GetModel returns the model to use
func (b *Bundle) GetModel() string {
	return b.Model
}

// GetTemperature returns the temperature
func (b *Bundle) GetTemperature() float64 {
	if b.Temperature == 0 {
		return 0.7
	}
	return b.Temperature
}

// GetTimeout returns the timeout
func (b *Bundle) GetTimeout() int {
	if b.Timeout == 0 {
		return 120
	}
	return b.Timeout
}

// GetEndpoint returns the API endpoint
func (b *Bundle) GetEndpoint() string {
	if b.Endpoint == "" {
		return "chat/completions"
	}
	return b.Endpoint
}

// GetOrgID returns the org ID
func (b *Bundle) GetOrgID() string {
	return b.OrgID
}

// DefaultDir returns the default bundle directory
func DefaultDir() string {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		return filepath.Join(xdg, "synesis", "bundles")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, ".config", "synesis", "bundles")
	}
	return ".synesis/bundles"
}

// Store handles bundle persistence
type Store struct {
	dir string
}

// NewStore creates a bundle store at the given directory
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create bundle dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// DefaultStore returns the default bundle store
func DefaultStore() (*Store, error) {
	return NewStore(DefaultDir())
}

// Save saves a bundle to the store as YAML
func (s *Store) Save(b *Bundle, name string) error {
	if name == "" {
		return fmt.Errorf("bundle name is required")
	}

	path := s.bundlePath(name)
	data, err := yaml.Marshal(b)
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
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

// Get retrieves a bundle by name
func (s *Store) Get(name string) (*Bundle, error) {
	path := s.bundlePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("bundle not found: %s", name)
		}
		return nil, fmt.Errorf("read bundle: %w", err)
	}

	return Parse(data, filepath.Dir(path))
}

// List returns all bundle names
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
	}

	return names, nil
}

// Delete removes a bundle
func (s *Store) Delete(name string) error {
	path := s.bundlePath(name)
	return os.Remove(path)
}

// Exists checks if a bundle exists
func (s *Store) Exists(name string) bool {
	_, err := s.Get(name)
	return err == nil
}

func (s *Store) bundlePath(name string) string {
	return filepath.Join(s.dir, name+".yaml")
}
