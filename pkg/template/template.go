package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Template represents a reusable prompt template
type Template struct {
	Name         string   `yaml:"name" json:"name"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	SystemPrompt string   `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`
	UserPrompt   string   `yaml:"user_prompt" json:"user_prompt"`
	Variables    []string `yaml:"variables,omitempty" json:"variables,omitempty"`
}

// RenderedTemplate contains the rendered template output
type RenderedTemplate struct {
	SystemPrompt string
	UserPrompt   string
}

// Validate checks if the template is valid
func (t *Template) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if t.UserPrompt == "" {
		return fmt.Errorf("user_prompt is required")
	}
	return nil
}

// Render executes the template with the given variables
func (t *Template) Render(vars map[string]string) (*RenderedTemplate, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}

	// Check for missing required variables
	varMap := make(map[string]string)
	for _, v := range t.Variables {
		if val, ok := vars[v]; ok {
			varMap[v] = val
		} else {
			return nil, fmt.Errorf("missing required variable: %s", v)
		}
	}
	// Add any extra vars that were provided
	for k, v := range vars {
		varMap[k] = v
	}

	// Render system prompt if present
	var systemPrompt string
	if t.SystemPrompt != "" {
		tmpl, err := template.New("system").Parse(t.SystemPrompt)
		if err != nil {
			return nil, fmt.Errorf("parse system prompt: %w", err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, varMap); err != nil {
			return nil, fmt.Errorf("render system prompt: %w", err)
		}
		systemPrompt = strings.TrimSpace(buf.String())
	}

	// Render user prompt
	var userPrompt string
	tmpl, err := template.New("user").Parse(t.UserPrompt)
	if err != nil {
		return nil, fmt.Errorf("parse user prompt: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, varMap); err != nil {
		return nil, fmt.Errorf("render user prompt: %w", err)
	}
	userPrompt = strings.TrimSpace(buf.String())

	return &RenderedTemplate{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}, nil
}

// GetRequiredVariables returns the list of required variable names
func (t *Template) GetRequiredVariables() []string {
	return t.Variables
}
