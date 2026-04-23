package main

import (
	"encoding/json"
	"flag"
	"os"
	"testing"

	"synesis.sh/synesis/pkg/session"
)

func TestSessionCmd_List(t *testing.T) {
	// Create temp session dir
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a session with a name
	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sess.Name = "Test Session"
	if err := store.Update(sess); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test that getSessionStore uses correct dir
	sessions, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "Test Session" {
		t.Errorf("expected name 'Test Session', got %s", sessions[0].Name)
	}
}

func TestSessionCmd_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Delete
	err = store.Delete(sess.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify not found
	_, err = store.Get(sess.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestSessionCmd_Rename(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Rename
	err = store.Rename(sess.ID, "My New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify rename worked
	updated, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "My New Name" {
		t.Errorf("expected name 'My New Name', got %s", updated.Name)
	}
}

func TestSessionCmd_FindByName(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
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

func TestPrintSessions(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sess.Name = "TestPrint"

	err = store.AddMessage(sess, "user", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Capture output - just verify it doesn't panic
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open("/dev/null")
	defer func() { os.Stdout = oldStdout }()

	printSessions(sessions, false)
}

func TestPrintSessionDetail(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sess.Name = "DetailTest"
	sess.Summary = "A test summary"
	err = store.AddMessage(sess, "user", "test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout, _ = os.Open("/dev/null")
	defer func() { os.Stdout = oldStdout }()

	printSessionDetail(sess, false)
}

func TestFlagParsing(t *testing.T) {
	// Test -list flag
	fs := flag.NewFlagSet("session", flag.ExitOnError)
	_ = fs.Bool("list", false, "list all sessions")
	args := []string{"-list"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test -delete flag
	fs = flag.NewFlagSet("session", flag.ExitOnError)
	_ = fs.Bool("delete", false, "delete session")
	args = []string{"-delete", "somesessionid"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test -rename flag
	fs = flag.NewFlagSet("session", flag.ExitOnError)
	rename := fs.String("rename", "", "rename session")
	args = []string{"-rename", "NewName", "sessionid"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *rename != "NewName" {
		t.Errorf("expected rename 'NewName', got %s", *rename)
	}

	// Test -json flag
	fs = flag.NewFlagSet("session", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output JSON")
	args = []string{"-json"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !*jsonOutput {
		t.Error("expected -json to be true")
	}

	// Test -n flag
	fs = flag.NewFlagSet("session", flag.ExitOnError)
	head := fs.Int("n", 10, "number of sessions to show")
	args = []string{"-n", "5"}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *head != 5 {
		t.Errorf("expected head 5, got %d", *head)
	}
}

func TestSessionJSONRoundtrip(t *testing.T) {
	sess := &session.Session{
		ID:       "test-id",
		Name:     "Test",
		Model:    "gpt-4",
		Messages: []session.Message{},
		Summary:  "summary",
		System:   "you are helpful",
	}

	// Marshal
	data, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unmarshal
	var loaded session.Session
	err = json.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.ID != sess.ID {
		t.Errorf("expected ID %s, got %s", sess.ID, loaded.ID)
	}
	if loaded.Name != sess.Name {
		t.Errorf("expected Name %s, got %s", sess.Name, loaded.Name)
	}
	if loaded.Model != sess.Model {
		t.Errorf("expected Model %s, got %s", sess.Model, loaded.Model)
	}
	if loaded.Summary != sess.Summary {
		t.Errorf("expected Summary %s, got %s", sess.Summary, loaded.Summary)
	}
	if loaded.System != sess.System {
		t.Errorf("expected System %s, got %s", sess.System, loaded.System)
	}
}

func TestSessionAddMessage(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess, err := store.Create("gpt-4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add user message
	err = store.AddMessage(sess, "user", "Hello, world!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add assistant message
	err = store.AddMessage(sess, "assistant", "Hi there!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify messages are persisted
	updated, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updated.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(updated.Messages))
	}
	if updated.Messages[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %s", updated.Messages[0].Role)
	}
	if updated.Messages[0].Content != "Hello, world!" {
		t.Errorf("expected first message content 'Hello, world!', got %s", updated.Messages[0].Content)
	}
	if updated.Messages[1].Role != "assistant" {
		t.Errorf("expected second message role 'assistant', got %s", updated.Messages[1].Role)
	}
}