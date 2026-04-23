package session

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestStore_Create(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "You are helpful")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", sess.Model)
	}
	if sess.System != "You are helpful" {
		t.Errorf("expected system prompt")
	}
	if sess.ID == "" {
		t.Error("expected session ID")
	}
	if sess.CreatedAt.IsZero() {
		t.Error("expected created_at")
	}
}

func TestStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create session
	created, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Get session
	sess, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, sess.ID)
	}
}

func TestStore_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = store.Get("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestStore_Update(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	originalTime := sess.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	sess.Name = "My Session"
	err = store.Update(sess)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify update
	updated, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Name != "My Session" {
		t.Errorf("expected name My Session, got %s", updated.Name)
	}
	if !updated.UpdatedAt.After(originalTime) {
		t.Error("expected updated_at to be newer")
	}
}

func TestStore_AddMessage(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.AddMessage(sess, "user", "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.AddMessage(sess, "assistant", "Hi there!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Reload and check
	updated, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updated.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(updated.Messages))
	}
	if updated.Messages[0].Role != "user" {
		t.Errorf("expected first role user, got %s", updated.Messages[0].Role)
	}
	if updated.Messages[0].Content != "Hello" {
		t.Errorf("expected content Hello, got %s", updated.Messages[0].Content)
	}
}

func TestStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		_, err := store.Create("gpt-4", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}

	// Should be sorted by updated, newest first
	for i := 0; i < len(sessions)-1; i++ {
		if sessions[i].UpdatedAt.Before(sessions[i+1].UpdatedAt) {
			t.Error("expected sessions sorted by updated_at descending")
		}
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Delete(sess.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not exist
	_, err = store.Get(sess.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestStore_FindByName(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sess.Name = "my-important-session"
	err = store.Update(sess)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find by exact name
	found, err := store.FindByName("my-important-session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != sess.ID {
		t.Errorf("expected session %s, got %s", sess.ID, found.ID)
	}

	// Case insensitive
	found, err = store.FindByName("MY-IMPORTANT-SESSION")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != sess.ID {
		t.Errorf("expected case-insensitive match")
	}
}

func TestStore_Rename(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = store.Rename(sess.ID, "New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name New Name, got %s", updated.Name)
	}
}

func TestStore_Prune(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create 5 sessions
	for i := 0; i < 5; i++ {
		_, err := store.Create("gpt-4", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Prune to keep 2
	err = store.Prune(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions after prune, got %d", len(sessions))
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	path := store.sessionPath(sess.ID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected session file to exist")
	}

	// Verify no temp file left behind
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("temp file should not exist after write")
	}
}

func TestStore_ExportJSON(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "You are helpful")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add a message
	_ = store.AddMessage(sess, "user", "Hello")
	_ = store.AddMessage(sess, "assistant", "Hi there!")

	// Export to JSON
	data, err := store.ExportJSON(sess)
	if err != nil {
		t.Fatalf("export JSON failed: %v", err)
	}

	// Verify it's valid JSON
	if len(data) == 0 {
		t.Error("expected non-empty JSON output")
	}
	if string(data[:1]) != "{" {
		t.Error("expected JSON object")
	}
}

func TestStore_ExportMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "You are helpful")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = store.AddMessage(sess, "user", "Hello")
	_ = store.AddMessage(sess, "assistant", "Hi there!")

	// Export to Markdown
	data, err := store.ExportMarkdown(sess)
	if err != nil {
		t.Fatalf("export markdown failed: %v", err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Error("expected non-empty markdown output")
	}
	if !contains(content, "# Session:") {
		t.Error("expected session header")
	}
	if !contains(content, "## Conversation") {
		t.Error("expected conversation section")
	}
	if !contains(content, "Hello") {
		t.Error("expected user message content")
	}
}

func TestStore_ImportSession(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a session to export
	original, err := store.Create("gpt-4", "You are helpful")
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	_ = store.AddMessage(original, "user", "Hello")
	_ = store.AddMessage(original, "assistant", "Hi!")

	// Export and re-import
	data, err := store.ExportJSON(original)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	imported, err := store.ImportSession(data)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	// Verify imported session has correct structure
	if imported.ID == "" {
		t.Error("expected imported session to have ID")
	}
	if imported.ID == original.ID {
		t.Error("expected imported session to have new ID")
	}
	if len(imported.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(imported.Messages))
	}
}

func TestStore_ImportAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create test session data
	sess := &Session{
		ID:     "original-id",
		Model:  "gpt-4",
		System: "You are helpful",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi!"},
		},
	}

	data, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Import and save
	imported, err := store.ImportAndSave(data, "json")
	if err != nil {
		t.Fatalf("import and save failed: %v", err)
	}

	// Verify session was saved
	saved, err := store.Get(imported.ID)
	if err != nil {
		t.Fatalf("get saved session failed: %v", err)
	}

	if len(saved.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(saved.Messages))
	}
}

func TestStore_ImportMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markdown := `# Session: test

**Model:** gpt-4
**Created:** 2024-01-01T00:00:00Z
**Updated:** 2024-01-01T00:00:00Z

## Conversation

### User

Hello, how are you?

### Assistant

I'm doing well, thank you!

`

	sess, err := store.ImportMarkdown([]byte(markdown))
	if err != nil {
		t.Fatalf("import markdown failed: %v", err)
	}

	if len(sess.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(sess.Messages))
	}
	if sess.Messages[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %s", sess.Messages[0].Role)
	}
	if sess.Messages[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %s", sess.Messages[1].Role)
	}
}

func TestValidateSession(t *testing.T) {
	// Valid session
	valid := &Session{
		ID:    "test-id",
		Model: "gpt-4",
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
	}
	if err := validateSession(valid); err != nil {
		t.Errorf("valid session should pass: %v", err)
	}

	// Missing ID
	noID := &Session{Model: "gpt-4", Messages: []Message{{Role: "user", Content: "Hi"}}}
	if err := validateSession(noID); err == nil {
		t.Error("expected error for missing ID")
	}

	// Missing model
	noModel := &Session{ID: "test", Messages: []Message{{Role: "user", Content: "Hi"}}}
	if err := validateSession(noModel); err == nil {
		t.Error("expected error for missing model")
	}

	// No messages
	noMsgs := &Session{ID: "test", Model: "gpt-4"}
	if err := validateSession(noMsgs); err == nil {
		t.Error("expected error for no messages")
	}

	// Empty message role
	badRole := &Session{
		ID:    "test",
		Model: "gpt-4",
		Messages: []Message{
			{Role: "", Content: "Hi"},
		},
	}
	if err := validateSession(badRole); err == nil {
		t.Error("expected error for empty role")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}