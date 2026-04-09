package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"synesis.sh/synesis/pkg/session"
	"synesis.sh/synesis/pkg/ui"
)

// runSession implements the session command
func runSession(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("session", flag.ExitOnError)
	list := fs.Bool("list", false, "list all sessions")
	delete := fs.Bool("delete", false, "delete session")
	rename := fs.String("rename", "", "rename session")
	_ = fs.Int("prune", 0, "prune sessions, keep N newest")
	jsonOutput := fs.Bool("json", false, "output JSON")
	head := fs.Int("n", 10, "number of sessions to show")

	fs.Parse(args)

	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

	// List sessions
	if *list || fs.NArg() == 0 {
		sessions, err := store.List()
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}

		if *jsonOutput {
			data, _ := json.Marshal(sessions)
			fmt.Println(string(data))
			return nil
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found")
			return nil
		}

		showCount := *head
		if showCount > len(sessions) {
			showCount = len(sessions)
		}

		printSessions(sessions[:showCount], noColor)
		return nil
	}

	// Get or operate on specific session
	sessionID := fs.Arg(0)
	var sess *session.Session

	// Try to find by ID or name
	sess, err = store.Get(sessionID)
	if err != nil {
		sess, err = store.FindByName(sessionID)
	}
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Delete
	if *delete {
		if err := store.Delete(sess.ID); err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
		if !quiet {
			fmt.Printf("Deleted session: %s\n", sess.ID)
		}
		return nil
	}

	// Rename
	if *rename != "" {
		if err := store.Rename(sess.ID, *rename); err != nil {
			return fmt.Errorf("rename session: %w", err)
		}
		if !quiet {
			fmt.Printf("Renamed session %s to %s\n", sess.ID, *rename)
		}
		return nil
	}

	// Show session details
	if *jsonOutput {
		data, _ := json.Marshal(sess)
		fmt.Println(string(data))
		return nil
	}

	printSessionDetail(sess, noColor)
	return nil
}

func printSessions(sessions []session.Session, noColor bool) {
	fmt.Printf("Found %d sessions:\n\n", len(sessions))
	for _, s := range sessions {
		ts := s.UpdatedAt.Format("2006-01-02 15:04")
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("  %s  %s  %s\n",
			ui.Color(s.ID[:8], ui.ColorCyan, false),
			ui.Color(name, ui.ColorBold, false),
			ui.Color(ts, ui.ColorGray, false))
	}
}

func printSessionDetail(s *session.Session, noColor bool) {
	fmt.Println("Session Details:")
	fmt.Println("  ID:", s.ID)
	if s.Name != "" {
		fmt.Println("  Name:", s.Name)
	}
	fmt.Println("  Model:", s.Model)
	fmt.Println("  Created:", s.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("  Updated:", s.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("  Messages:", len(s.Messages))
	if s.Summary != "" {
		fmt.Println("  Summary:", s.Summary)
	}

	if len(s.Messages) > 0 {
		fmt.Println("\nRecent messages:")
		show := 5
		if show > len(s.Messages) {
			show = len(s.Messages)
		}
		for _, m := range s.Messages[len(s.Messages)-show:] {
			role := ui.Color(m.Role+":", ui.ColorCyan, false)
			content := m.Content
			if len(content) > 60 {
				content = content[:57] + "..."
			}
			fmt.Printf("    %s %s\n", role, content)
		}
	}
}