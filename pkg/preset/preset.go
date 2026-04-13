package preset

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Preset represents a system prompt preset
type Preset struct {
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	SystemPrompt    string   `yaml:"system_prompt"`
	SuggestedModels []string `yaml:"suggested_models,omitempty"`
}

// Store manages preset files
type Store struct {
	dir string
}

// NewStore creates a new preset store
func NewStore(dir string) (*Store, error) {
	if dir == "" {
		dir = defaultPresetDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// defaultPresetDir returns the default preset directory
func defaultPresetDir() string {
	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			xdgData = filepath.Join(home, ".local", "share")
		}
	}
	if xdgData == "" {
		xdgData = "."
	}
	return filepath.Join(xdgData, "synesis", "presets")
}

// List returns all available presets
func (s *Store) List() ([]Preset, error) {
	files, err := filepath.Glob(filepath.Join(s.dir, "*.yaml"))
	if err != nil {
		return nil, err
	}

	var presets []Preset
	for _, file := range files {
		p, err := Load(file)
		if err != nil {
			continue
		}
		presets = append(presets, p)
	}
	return presets, nil
}

// Get returns a preset by name
func (s *Store) Get(name string) (*Preset, error) {
	file := filepath.Join(s.dir, name+".yaml")
	p, err := Load(file)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Save saves a preset
func (s *Store) Save(p *Preset) error {
	file := filepath.Join(s.dir, p.Name+".yaml")
	return saveYAML(file, p)
}

// Delete removes a preset
func (s *Store) Delete(name string) error {
	file := filepath.Join(s.dir, name+".yaml")
	return os.Remove(file)
}

// Load loads a preset from a file
func Load(file string) (Preset, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return Preset{}, err
	}

	var p Preset
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Preset{}, err
	}
	return p, nil
}

// SaveYAML saves a preset to a YAML file
func (s *Store) SaveYAML(p *Preset) error {
	file := filepath.Join(s.dir, p.Name+".yaml")
	return saveYAML(file, p)
}

// saveYAML writes data to a YAML file
func saveYAML(file string, data interface{}) error {
	b, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(file, b, 0644)
}
