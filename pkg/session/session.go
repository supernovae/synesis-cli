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

// ExportFormat represents the export format type
type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportMD   ExportFormat = "md"
)

// Export exports a session to the specified format
func (s *Store) Export(sess *Session, format string) ([]byte, error) {
	switch format {
	case "json":
		return s.ExportJSON(sess)
	case "md", "markdown":
		return s.ExportMarkdown(sess)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// ExportJSON exports a session as JSON
func (s *Store) ExportJSON(sess *Session) ([]byte, error) {
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal session: %w", err)
	}
	return data, nil
}

// ExportMarkdown exports a session as Markdown
func (s *Store) ExportMarkdown(sess *Session) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Session: %s\n\n", sess.ID))
	sb.WriteString(fmt.Sprintf("**Model:** %s\n", sess.Model))
	sb.WriteString(fmt.Sprintf("**Created:** %s\n", sess.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Updated:** %s\n", sess.UpdatedAt.Format(time.RFC3339)))

	if sess.Name != "" {
		sb.WriteString(fmt.Sprintf("**Name:** %s\n\n", sess.Name))
	} else {
		sb.WriteString("\n")
	}

	if sess.System != "" {
		sb.WriteString("## System Prompt\n\n")
		sb.WriteString(sess.System)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Conversation\n\n")
	for _, msg := range sess.Messages {
		role := msg.Role
		if role == "assistant" {
			role = "Assistant"
		} else if role == "user" {
			role = "User"
		} else if role == "system" {
			role = "System"
		}

		sb.WriteString(fmt.Sprintf("### %s\n\n", role))
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
	}

	if sess.Summary != "" {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(sess.Summary)
		sb.WriteString("\n")
	}

	return []byte(sb.String()), nil
}

// ImportSession imports a session from JSON data
func (s *Store) ImportSession(data []byte) (*Session, error) {
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parse session JSON: %w", err)
	}

	// Validate required fields
	if err := validateSession(&sess); err != nil {
		return nil, err
	}

	// Generate new ID to avoid conflicts
	sess.ID = uuid.New().String()
	sess.CreatedAt = time.Now()
	sess.UpdatedAt = time.Now()

	return &sess, nil
}

// ImportFromFile imports a session from a file
func (s *Store) ImportFromFile(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	return s.ImportSession(data)
}

// ImportMarkdown imports a session from Markdown format
// Note: This is a best-effort parser and may not preserve all formatting
func (s *Store) ImportMarkdown(data []byte) (*Session, error) {
	content := string(data)
	sess := &Session{
		ID:        uuid.New().String(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Model:     "imported",
	}

	lines := strings.Split(content, "\n")
	var currentRole string
	var currentContent strings.Builder
	inMessages := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Parse header
		if strings.HasPrefix(trimmed, "**Model:**") {
			sess.Model = strings.TrimSpace(strings.TrimPrefix(trimmed, "**Model:**"))
			continue
		}
		if strings.HasPrefix(trimmed, "**Name:**") {
			sess.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "**Name:**"))
			continue
		}

		// Detect section headers
		if strings.HasPrefix(trimmed, "## Conversation") {
			inMessages = true
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			inMessages = false
			continue
		}

		// Parse messages
		if inMessages && strings.HasPrefix(trimmed, "### ") {
			// Save previous message if any
			if currentRole != "" && currentContent.Len() > 0 {
				sess.Messages = append(sess.Messages, Message{
					Role:    currentRole,
					Content: strings.TrimSpace(currentContent.String()),
				})
				currentContent.Reset()
			}

			role := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			switch strings.ToLower(role) {
			case "user":
				currentRole = "user"
			case "assistant":
				currentRole = "assistant"
			case "system":
				currentRole = "system"
			default:
				currentRole = "unknown"
			}
			continue
		}

		// Accumulate message content
		if currentRole != "" && trimmed != "" {
			if currentContent.Len() > 0 {
				currentContent.WriteString("\n")
			}
			currentContent.WriteString(trimmed)
		}
	}

	// Don't forget the last message
	if currentRole != "" && currentContent.Len() > 0 {
		sess.Messages = append(sess.Messages, Message{
			Role:    currentRole,
			Content: strings.TrimSpace(currentContent.String()),
		})
	}

	if len(sess.Messages) == 0 {
		return nil, fmt.Errorf("no messages found in markdown")
	}

	return sess, nil
}

// validateSession checks if a session has required fields
func validateSession(sess *Session) error {
	if sess.ID == "" {
		return fmt.Errorf("session ID is required")
	}
	if sess.Model == "" {
		return fmt.Errorf("session model is required")
	}
	if len(sess.Messages) == 0 {
		return fmt.Errorf("session must have at least one message")
	}
	for i, msg := range sess.Messages {
		if msg.Role == "" {
			return fmt.Errorf("message %d has empty role", i)
		}
		if msg.Content == "" {
			return fmt.Errorf("message %d has empty content", i)
		}
	}
	return nil
}

// ImportAndSave imports a session and saves it to the store
func (s *Store) ImportAndSave(data []byte, format string) (*Session, error) {
	var sess *Session
	var err error

	switch format {
	case "json":
		sess, err = s.ImportSession(data)
	case "md", "markdown":
		sess, err = s.ImportMarkdown(data)
	default:
		return nil, fmt.Errorf("unsupported import format: %s", format)
	}

	if err != nil {
		return nil, err
	}

	// Save to store
	if err := s.save(sess); err != nil {
		return nil, fmt.Errorf("save imported session: %w", err)
	}

	return sess, nil
}