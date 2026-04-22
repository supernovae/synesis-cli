package jq

import (
	"testing"
)

func TestApply_FieldAccess(t *testing.T) {
	input := `{"choices": [{"message": {"content": "hello"}}]}`
	result, err := Apply(input, ".choices[0].message.content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}
}

func TestApply_NestedField(t *testing.T) {
	input := `{"data": {"user": {"name": "Alice", "age": 30}}}`
	result, err := Apply(input, ".data.user.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Alice" {
		t.Errorf("expected 'Alice', got '%s'", result)
	}
}

func TestApply_ArrayIndex(t *testing.T) {
	input := `{"items": [10, 20, 30]}`
	result, err := Apply(input, ".items[1]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "20" {
		t.Errorf("expected '20', got '%s'", result)
	}
}

func TestApply_ArraySlice(t *testing.T) {
	input := `{"items": [1, 2, 3, 4, 5]}`
	result, err := Apply(input, ".items[1:3]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "[2,3]" {
		t.Errorf("expected '[2,3]', got '%s'", result)
	}
}

func TestApply_Wildcard(t *testing.T) {
	input := `{"items": [1, 2, 3]}`
	result, err := Apply(input, ".items[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "[1,2,3]" {
		t.Errorf("expected '[1,2,3]', got '%s'", result)
	}
}

func TestApply_RecursiveDescent(t *testing.T) {
	input := `{"a": {"b": {"c": 42}}}`
	result, err := Apply(input, "..c")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "42" {
		t.Errorf("expected '42', got '%s'", result)
	}
}

func TestApply_ObjectConstruction(t *testing.T) {
	input := `{"name": "Alice", "age": 30, "city": "NYC"}`
	result, err := Apply(input, "[.name, .age]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `["Alice",30]` {
		t.Errorf("expected '[\"Alice\",30]', got '%s'", result)
	}
}

func TestApply_StringLiteral(t *testing.T) {
	input := `{}`
	result, err := Apply(input, `"hello"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}
}

func TestApply_BooleanLiterals(t *testing.T) {
	input := `{}`
	result, err := Apply(input, "true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "true" {
		t.Errorf("expected 'true', got '%s'", result)
	}

	result, err = Apply(input, "false")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "false" {
		t.Errorf("expected 'false', got '%s'", result)
	}
}

func TestApply_NullLiteral(t *testing.T) {
	input := `{}`
	result, err := Apply(input, "null")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "null" {
		t.Errorf("expected 'null', got '%s'", result)
	}
}

func TestApply_NumberLiteral(t *testing.T) {
	input := `{}`
	result, err := Apply(input, "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "42" {
		t.Errorf("expected '42', got '%s'", result)
	}
}

func TestApply_NonExistentField(t *testing.T) {
	input := `{"name": "Alice"}`
	result, err := Apply(input, ".missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for missing field, got '%s'", result)
	}
}

func TestApply_CompoundFilter(t *testing.T) {
	input := `{
		"choices": [
			{"message": {"content": "first"}},
			{"message": {"content": "second"}}
		]
	}`
	result, err := Apply(input, ".choices[0].message.content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "first" {
		t.Errorf("expected 'first', got '%s'", result)
	}
}

func TestApply_EmptyObject(t *testing.T) {
	input := `{}`
	result, err := Apply(input, "{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "{}" {
		t.Errorf("expected '{}', got '%s'", result)
	}
}

func TestApply_EmptyArray(t *testing.T) {
	input := `{}`
	result, err := Apply(input, "[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "[]" {
		t.Errorf("expected '[]', got '%s'", result)
	}
}
