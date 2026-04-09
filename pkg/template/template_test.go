package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplate_Validate(t *testing.T) {
	// Valid template
	valid := &Template{
		Name:       "test",
		UserPrompt: "Hello {{.name}}",
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid template should pass validation: %v", err)
	}

	// Missing name
	noName := &Template{UserPrompt: "Hello"}
	if err := noName.Validate(); err == nil {
		t.Error("template without name should fail validation")
	}

	// Missing user prompt
	noPrompt := &Template{Name: "test"}
	if err := noPrompt.Validate(); err == nil {
		t.Error("template without user_prompt should fail validation")
	}
}

func TestTemplate_Render_Basic(t *testing.T) {
	tmpl := &Template{
		Name:       "test",
		UserPrompt: "Hello {{.name}}",
		Variables:  []string{"name"},
	}

	rendered, err := tmpl.Render(map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rendered.UserPrompt != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", rendered.UserPrompt)
	}
}

func TestTemplate_Render_SystemPrompt(t *testing.T) {
	tmpl := &Template{
		Name:         "test",
		SystemPrompt: "You are a {{.role}}",
		UserPrompt:   "Process {{.task}}",
		Variables:    []string{"role", "task"},
	}

	rendered, err := tmpl.Render(map[string]string{"role": "reviewer", "task": "the code"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rendered.SystemPrompt != "You are a reviewer" {
		t.Errorf("expected 'You are a reviewer', got %q", rendered.SystemPrompt)
	}
	if rendered.UserPrompt != "Process the code" {
		t.Errorf("expected 'Process the code', got %q", rendered.UserPrompt)
	}
}

func TestTemplate_Render_MissingVariable(t *testing.T) {
	tmpl := &Template{
		Name:       "test",
		UserPrompt: "Hello {{.name}}",
		Variables:  []string{"name"},
	}

	_, err := tmpl.Render(map[string]string{})
	if err == nil {
		t.Error("expected error for missing variable")
	}
	if !strings.Contains(err.Error(), "missing required variable") {
		t.Errorf("expected missing variable error, got: %v", err)
	}
}

func TestTemplate_Render_ExtraVariables(t *testing.T) {
	tmpl := &Template{
		Name:       "test",
		UserPrompt: "Hello {{.name}}",
		Variables:  []string{"name"},
	}

	// Extra variables should be allowed but not required
	rendered, err := tmpl.Render(map[string]string{"name": "World", "extra": "ignored"})
	if err != nil {
		t.Fatalf("unexpected error with extra vars: %v", err)
	}
	if rendered.UserPrompt != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", rendered.UserPrompt)
	}
}

func TestTemplate_Render_InvalidTemplateSyntax(t *testing.T) {
	tmpl := &Template{
		Name:       "test",
		UserPrompt: "Hello {{.name", // Missing closing brace
		Variables:  []string{"name"},
	}

	_, err := tmpl.Render(map[string]string{"name": "World"})
	if err == nil {
		t.Error("expected error for invalid template syntax")
	}
}

func TestTemplate_Render_MultipleVariables(t *testing.T) {
	tmpl := &Template{
		Name:       "test",
		UserPrompt: "{{.greeting}} {{.name}}, you are a {{.role}}",
		Variables:  []string{"greeting", "name", "role"},
	}

	rendered, err := tmpl.Render(map[string]string{
		"greeting": "Hello",
		"name":     "Alice",
		"role":     "engineer",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rendered.UserPrompt != "Hello Alice, you are a engineer" {
		t.Errorf("expected 'Hello Alice, you are a engineer', got %q", rendered.UserPrompt)
	}
}

func TestTemplate_GetRequiredVariables(t *testing.T) {
	tmpl := &Template{
		Name:       "test",
		UserPrompt: "Hello",
		Variables:  []string{"a", "b", "c"},
	}

	vars := tmpl.GetRequiredVariables()
	if len(vars) != 3 {
		t.Errorf("expected 3 variables, got %d", len(vars))
	}
}

func TestStore_SaveAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	tmpl := &Template{
		Name:         "test",
		Description:  "Test template",
		SystemPrompt: "You are helpful",
		UserPrompt:   "Hello {{.name}}",
		Variables:    []string{"name"},
	}

	// Save
	if err := store.Save(tmpl); err != nil {
		t.Fatalf("failed to save template: %v", err)
	}

	// Get
	retrieved, err := store.Get("test")
	if err != nil {
		t.Fatalf("failed to get template: %v", err)
	}

	if retrieved.Name != "test" {
		t.Errorf("expected name 'test', got %q", retrieved.Name)
	}
	if retrieved.Description != "Test template" {
		t.Errorf("expected description 'Test template', got %q", retrieved.Description)
	}
	if retrieved.UserPrompt != "Hello {{.name}}" {
		t.Errorf("expected user prompt 'Hello {{.name}}', got %q", retrieved.UserPrompt)
	}
}

func TestStore_GetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Save multiple templates
	templates := []*Template{
		{Name: "alpha", UserPrompt: "A"},
		{Name: "beta", UserPrompt: "B"},
		{Name: "gamma", UserPrompt: "C"},
	}
	for _, tmpl := range templates {
		if err := store.Save(tmpl); err != nil {
			t.Fatalf("failed to save template: %v", err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("expected 3 templates, got %d", len(list))
	}

	// Check sorted order
	expected := []string{"alpha", "beta", "gamma"}
	for i, tmpl := range list {
		if tmpl.Name != expected[i] {
			t.Errorf("expected name %q at position %d, got %q", expected[i], i, tmpl.Name)
		}
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Save then delete
	tmpl := &Template{Name: "test", UserPrompt: "Hello"}
	if err := store.Save(tmpl); err != nil {
		t.Fatalf("failed to save template: %v", err)
	}

	if err := store.Delete("test"); err != nil {
		t.Fatalf("failed to delete template: %v", err)
	}

	// Verify deletion
	_, err = store.Get("test")
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestStore_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if store.Exists("test") {
		t.Error("expected template to not exist")
	}

	tmpl := &Template{Name: "test", UserPrompt: "Hello"}
	if err := store.Save(tmpl); err != nil {
		t.Fatalf("failed to save template: %v", err)
	}

	if !store.Exists("test") {
		t.Error("expected template to exist after save")
	}
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "template.yaml")
	content := `
name: test
description: Test template
system_prompt: "You are helpful"
user_prompt: "Hello {{.name}}"
variables:
  - name
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write template file: %v", err)
	}

	tmpl, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("failed to load template: %v", err)
	}

	if tmpl.Name != "test" {
		t.Errorf("expected name 'test', got %q", tmpl.Name)
	}
	if tmpl.Description != "Test template" {
		t.Errorf("expected description 'Test template', got %q", tmpl.Description)
	}
}

func TestLoadFromFile_InvalidPath(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestStore_DefaultDir(t *testing.T) {
	dir := DefaultDir()
	if dir == "" {
		t.Error("expected non-empty default directory")
	}
}
