package template

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Store handles template persistence
type Store struct {
	dir string
}

// NewStore creates a template store at the given directory
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create template dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// DefaultDir returns the default template directory
func DefaultDir() string {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		return filepath.Join(xdg, "synesis", "templates")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, ".config", "synesis", "templates")
	}
	return ".synesis/templates"
}

// Save saves a template to the store
func (s *Store) Save(t *Template) error {
	if err := t.Validate(); err != nil {
		return err
	}

	path := s.templatePath(t.Name)
	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
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

// Get retrieves a template by name
func (s *Store) Get(name string) (*Template, error) {
	path := s.templatePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("template not found: %s", name)
		}
		return nil, fmt.Errorf("read template: %w", err)
	}

	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	return &t, nil
}

// List returns all templates sorted by name
func (s *Store) List() ([]Template, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	var templates []Template
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".yaml")
		t, err := s.Get(name)
		if err != nil {
			continue // skip invalid templates
		}
		templates = append(templates, *t)
	}

	// Sort by name
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	return templates, nil
}

// Delete removes a template
func (s *Store) Delete(name string) error {
	path := s.templatePath(name)
	return os.Remove(path)
}

// Exists checks if a template exists
func (s *Store) Exists(name string) bool {
	_, err := s.Get(name)
	return err == nil
}

func (s *Store) templatePath(name string) string {
	return filepath.Join(s.dir, name+".yaml")
}

// LoadFromFile loads a template from an arbitrary file path
func LoadFromFile(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template file: %w", err)
	}

	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse template file: %w", err)
	}

	if err := t.Validate(); err != nil {
		return nil, err
	}

	return &t, nil
}
