package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Session represents a stored conversation
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
	System    string    `json:"system,omitempty"`
	Summary   string    `json:"summary,omitempty"`
}

// Message represents a chat message in storage
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// Store handles session persistence
type Store struct {
	dir string
}

// NewStore creates a session store at the given directory
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Create creates a new session
func (s *Store) Create(model, system string) (*Session, error) {
	id := uuid.New().String()
	now := time.Now()
	sess := &Session{
		ID:        id,
		Model:     model,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if system != "" {
		sess.System = system
	}

	if err := s.save(sess); err != nil {
		return nil, err
	}

	return sess, nil
}

// Get retrieves a session by ID
func (s *Store) Get(id string) (*Session, error) {
	path := s.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}

	return &sess, nil
}

// FindByName finds a session by name (case-insensitive prefix match)
func (s *Store) FindByName(name string) (*Session, error) {
	sessions, err := s.List()
	if err != nil {
		return nil, err
	}

	name = strings.ToLower(name)
	for _, sess := range sessions {
		if strings.EqualFold(sess.Name, name) || strings.HasPrefix(strings.ToLower(sess.Name), name) {
			return &sess, nil
		}
	}
	return nil, fmt.Errorf("session not found: %s", name)
}

// Update saves changes to a session
func (s *Store) Update(sess *Session) error {
	sess.UpdatedAt = time.Now()
	return s.save(sess)
}

// AddMessage appends a message to a session
func (s *Store) AddMessage(sess *Session, role, content string) error {
	sess.Messages = append(sess.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	sess.UpdatedAt = time.Now()
	return s.save(sess)
}

// List returns all sessions sorted by last updated
func (s *Store) List() ([]Session, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		sess, err := s.Get(id)
		if err != nil {
			continue // skip invalid sessions
		}
		sessions = append(sessions, *sess)
	}

	// Sort by updated time, newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Delete removes a session
func (s *Store) Delete(id string) error {
	path := s.sessionPath(id)
	return os.Remove(path)
}

// Prune removes old sessions, keeping count newest
func (s *Store) Prune(keep int) error {
	sessions, err := s.List()
	if err != nil {
		return err
	}

	if len(sessions) <= keep {
		return nil
	}

	toDelete := sessions[keep:]
	for _, sess := range toDelete {
		if err := s.Delete(sess.ID); err != nil {
			// Log but continue
			fmt.Fprintf(os.Stderr, "warning: failed to delete session %s: %v\n", sess.ID, err)
		}
	}

	return nil
}

// Rename updates session name
func (s *Store) Rename(id, newName string) error {
	sess, err := s.Get(id)
	if err != nil {
		return err
	}
	sess.Name = newName
	return s.Update(sess)
}

func (s *Store) sessionPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// save writes a session atomically via temp file + rename
func (s *Store) save(sess *Session) error {
	path := s.sessionPath(sess.ID)
	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	// Write to temp file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}