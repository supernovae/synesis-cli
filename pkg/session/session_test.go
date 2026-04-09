package session

import (
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

	// Find by name
	found, err := store.FindByName("my-important")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != sess.ID {
		t.Errorf("expected session %s, got %s", sess.ID, found.ID)
	}

	// Case insensitive
	found, err = store.FindByName("MY-IMPORTANT")
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