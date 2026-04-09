package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"synesis.sh/synesis/pkg/session"
	"synesis.sh/synesis/pkg/ui"
)

// runSession implements the session command
func runSession(args []string, noColor, quiet bool, profileName string) error {
	if len(args) == 0 {
		// Default to list
		return runSessionList(noColor, quiet)
	}

	subcmd := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subcmd {
	case "list", "ls":
		return runSessionList(noColor, quiet)
	case "show":
		return runSessionShow(subArgs, noColor, quiet)
	case "delete", "rm":
		return runSessionDelete(subArgs, noColor, quiet)
	case "rename":
		return runSessionRename(subArgs, noColor, quiet)
	case "export":
		return runSessionExport(subArgs, noColor, quiet)
	case "import":
		return runSessionImport(subArgs, noColor, quiet)
	default:
		// Try to show session by ID/name for backward compatibility
		return runSessionShow(args, noColor, quiet)
	}
}

func runSessionList(noColor, quiet bool) error {
	fs := flag.NewFlagSet("session list", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output JSON")
	head := fs.Int("n", 10, "number of sessions to show")
	fs.Parse(nil)

	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

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
		if !quiet {
			fmt.Println("No sessions found")
		}
		return nil
	}

	showCount := *head
	if showCount > len(sessions) {
		showCount = len(sessions)
	}

	printSessions(sessions[:showCount], noColor)
	return nil
}

func runSessionShow(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("session show", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("session ID or name required")
	}

	sessionID := fs.Arg(0)

	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

	var sess *session.Session
	sess, err = store.Get(sessionID)
	if err != nil {
		sess, err = store.FindByName(sessionID)
	}
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if *jsonOutput {
		data, _ := json.Marshal(sess)
		fmt.Println(string(data))
		return nil
	}

	printSessionDetail(sess, noColor)
	return nil
}

func runSessionDelete(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("session delete", flag.ExitOnError)
	force := fs.Bool("force", false, "skip confirmation")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("session ID or name required")
	}

	sessionID := fs.Arg(0)

	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

	var sess *session.Session
	sess, err = store.Get(sessionID)
	if err != nil {
		sess, err = store.FindByName(sessionID)
	}
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Confirm deletion
	if !*force && ui.IsTerminal() {
		fmt.Printf("Are you sure you want to delete session '%s'? [y/N] ", sessionID)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	if err := store.Delete(sess.ID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if !quiet {
		fmt.Printf("Deleted session: %s\n", sess.ID)
	}
	return nil
}

func runSessionRename(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("session rename", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() < 2 {
		return fmt.Errorf("session ID and new name required")
	}

	sessionID := fs.Arg(0)
	newName := fs.Arg(1)

	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

	var sess *session.Session
	sess, err = store.Get(sessionID)
	if err != nil {
		sess, err = store.FindByName(sessionID)
	}
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if err := store.Rename(sess.ID, newName); err != nil {
		return fmt.Errorf("rename session: %w", err)
	}
	if !quiet {
		fmt.Printf("Renamed session %s to %s\n", sess.ID, newName)
	}
	return nil
}

func runSessionExport(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("session export", flag.ExitOnError)
	format := fs.String("format", "json", "export format: json, md")
	output := fs.String("output", "", "output file (default: stdout)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("session ID or name required")
	}

	sessionID := fs.Arg(0)

	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

	var sess *session.Session
	sess, err = store.Get(sessionID)
	if err != nil {
		sess, err = store.FindByName(sessionID)
	}
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Export
	data, err := store.Export(sess, *format)
	if err != nil {
		return fmt.Errorf("export session: %w", err)
	}

	// Write output
	if *output != "" {
		if err := os.WriteFile(*output, data, 0o600); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		if !quiet {
			fmt.Printf("Session exported to: %s\n", *output)
		}
	} else {
		os.Stdout.Write(data)
	}

	return nil
}

func runSessionImport(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("session import", flag.ExitOnError)
	format := fs.String("format", "json", "import format: json, md")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("input file required")
	}

	inputFile := fs.Arg(0)

	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

	// Read input file
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("read input file: %w", err)
	}

	// Import and save
	sess, err := store.ImportAndSave(data, *format)
	if err != nil {
		return fmt.Errorf("import session: %w", err)
	}

	if !quiet {
		fmt.Printf("Session imported successfully.\n")
		fmt.Printf("  ID: %s\n", sess.ID)
		fmt.Printf("  Messages: %d\n", len(sess.Messages))
	}

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
