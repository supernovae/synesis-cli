package repl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/session"
	"synesis.sh/synesis/pkg/ui"
)

// REPL represents an interactive read-eval-print loop
type REPL struct {
	reader      *bufio.Reader
	writer      io.Writer
	store       *session.Store
	cfg         *config.LoadedConfig
	client      api.Client
	session     *session.Session
	model       string
	system      string
	noColor     bool
	quiet       bool
	isTTY       bool
	renderMode  ui.RenderMode
	currentText string
}

// New creates a new REPL instance
func New(store *session.Store, cfg *config.LoadedConfig, client api.Client, noColor, quiet bool, renderMode ui.RenderMode) (*REPL, error) {
	return &REPL{
		reader:     bufio.NewReader(os.Stdin),
		writer:     os.Stdout,
		store:      store,
		cfg:        cfg,
		client:     client,
		model:      cfg.Cfg.Model,
		noColor:    noColor,
		quiet:      quiet,
		isTTY:      ui.IsTerminal(),
		renderMode: renderMode,
	}, nil
}

// Run starts the REPL loop
func (r *REPL) Run() error {
	if !r.isTTY {
		return fmt.Errorf("REPL requires an interactive terminal")
	}

	r.printWelcome()

	for {
		// Show prompt
		fmt.Fprint(r.writer, "\n"+r.getPrompt())

		// Read input
		line, err := r.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Ctrl+D
				fmt.Fprintln(r.writer)
				r.handleExit()
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		line = strings.TrimSpace(line)

		// Handle Ctrl+C (empty line after interrupt)
		if line == "" && r.currentText == "" {
			continue
		}

		// Accumulate multi-line input
		r.currentText += line
		if !strings.HasSuffix(line, "\\") {
			// Process complete input
			if err := r.process(r.currentText); err != nil {
				if err == io.EOF {
					// /exit or /quit command
					return nil
				}
				ui.Error("%v", err)
			}
			r.currentText = ""
		} else {
			// Remove trailing backslash and continue
			r.currentText = strings.TrimSuffix(r.currentText, "\\") + "\n"
		}
	}
}

func (r *REPL) process(input string) error {
	// Check for slash commands
	if strings.HasPrefix(input, "/") {
		return r.handleCommand(input)
	}

	// Regular chat input
	return r.sendChat(input)
}

func (r *REPL) handleCommand(input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "/help", "/h", "/?":
		r.printHelp()
	case "/exit", "/quit", "/q":
		r.handleExit()
		return io.EOF
	case "/save":
		return r.handleSave(args)
	case "/model":
		return r.handleModel(args)
	case "/system":
		return r.handleSystem(args)
	case "/clear":
		r.handleClear()
	case "/session":
		return r.handleSession(args)
	case "/new":
		return r.handleNew(args)
	default:
		fmt.Fprintf(r.writer, "Unknown command: %s. Type /help for available commands.\n", cmd)
	}

	return nil
}

func (r *REPL) sendChat(message string) error {
	if message == "" {
		return nil
	}

	// Create session if needed
	if r.session == nil {
		if err := r.createSession(); err != nil {
			return err
		}
	}

	// Add user message to session
	if err := r.store.AddMessage(r.session, "user", message); err != nil {
		return fmt.Errorf("save message: %w", err)
	}

	// Build API request
	messages := r.buildAPIMessages()

	req := &api.ChatRequest{
		Model:      r.model,
		Messages:   messages,
		Temperature: 0.7,
	}

	// Send request
	fmt.Fprint(r.writer, "\nThinking")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var response strings.Builder
	err := r.client.StreamChat(ctx, req, func(token string, err error) {
		if err != nil {
			return
		}
		response.WriteString(token)
		fmt.Fprint(r.writer, token)
	})

	fmt.Fprintln(r.writer)

	if err != nil {
		return fmt.Errorf("chat error: %w", err)
	}

	// Add assistant response to session
	if response.Len() > 0 {
		if err := r.store.AddMessage(r.session, "assistant", response.String()); err != nil {
			return fmt.Errorf("save response: %w", err)
		}
	}

	return nil
}

func (r *REPL) buildAPIMessages() []api.Message {
	var messages []api.Message

	// Add system prompt if set
	if r.system != "" {
		messages = append(messages, api.Message{Role: "system", Content: r.system})
	}

	// Add session messages
	for _, msg := range r.session.Messages {
		messages = append(messages, api.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return messages
}

func (r *REPL) createSession() error {
	var err error
	r.session, err = r.store.Create(r.model, r.system)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *REPL) handleSave(args []string) error {
	if r.session == nil {
		fmt.Fprintln(r.writer, "No active session to save")
		return nil
	}

	name := ""
	if len(args) > 0 {
		name = strings.Join(args, " ")
	}

	if name != "" {
		r.session.Name = name
	}

	if err := r.store.Update(r.session); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	if name != "" {
		fmt.Fprintf(r.writer, "Session saved as: %s\n", name)
	} else {
		fmt.Fprintf(r.writer, "Session saved (ID: %s)\n", r.session.ID)
	}

	return nil
}

func (r *REPL) handleModel(args []string) error {
	if len(args) == 0 {
		if r.model != "" {
			fmt.Fprintf(r.writer, "Current model: %s\n", r.model)
		} else {
			fmt.Fprintln(r.writer, "No model set")
		}
		return nil
	}

	r.model = strings.Join(args, " ")
	fmt.Fprintf(r.writer, "Model set to: %s\n", r.model)

	// Update session if active
	if r.session != nil {
		r.session.Model = r.model
		if err := r.store.Update(r.session); err != nil {
			return fmt.Errorf("update session: %w", err)
		}
	}

	return nil
}

func (r *REPL) handleSystem(args []string) error {
	if len(args) == 0 {
		if r.system != "" {
			fmt.Fprintf(r.writer, "Current system prompt:\n%s\n", r.system)
		} else {
			fmt.Fprintln(r.writer, "No system prompt set")
		}
		return nil
	}

	r.system = strings.Join(args, " ")
	fmt.Fprintln(r.writer, "System prompt updated")

	// Update session if active
	if r.session != nil {
		r.session.System = r.system
		if err := r.store.Update(r.session); err != nil {
			return fmt.Errorf("update session: %w", err)
		}
	}

	return nil
}

func (r *REPL) handleClear() {
	// Clear screen (ANSI escape)
	fmt.Fprint(r.writer, "\033[2J\033[H")
}

func (r *REPL) handleSession(args []string) error {
	if len(args) == 0 {
		if r.session != nil {
			fmt.Fprintf(r.writer, "Current session: %s", r.session.ID)
			if r.session.Name != "" {
				fmt.Fprintf(r.writer, " (%s)", r.session.Name)
			}
			fmt.Fprintf(r.writer, "\nMessages: %d\n", len(r.session.Messages))
		} else {
			fmt.Fprintln(r.writer, "No active session")
		}
		return nil
	}

	// Load session by ID or name
	sessionID := strings.Join(args, " ")
	sess, err := r.store.Get(sessionID)
	if err != nil {
		sess, err = r.store.FindByName(sessionID)
	}
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	r.session = sess
	r.model = sess.Model
	r.system = sess.System
	fmt.Fprintf(r.writer, "Loaded session: %s\n", sess.ID)

	return nil
}

func (r *REPL) handleNew(args []string) error {
	r.session = nil
	r.system = ""
	fmt.Fprintln(r.writer, "Started new conversation")
	return nil
}

func (r *REPL) handleExit() {
	if r.session != nil && !r.quiet {
		fmt.Fprintf(r.writer, "Session: %s\n", r.session.ID)
	}
	fmt.Fprintln(r.writer, "Goodbye!")
}

func (r *REPL) printWelcome() {
	fmt.Fprintln(r.writer, "Synesis REPL - Interactive Chat")
	fmt.Fprintln(r.writer, "Type /help for available commands")
	fmt.Fprintln(r.writer, "Use \\ at end of line for multi-line input")
	fmt.Fprintln(r.writer, "Ctrl+D or /exit to quit")
}

func (r *REPL) printHelp() {
	help := `
Available commands:
  /help, /h          Show this help message
  /exit, /quit, /q   Exit the REPL
  /save [name]       Save current session (optionally with a name)
  /model [name]      Show or set the current model
  /system [prompt]   Show or set the system prompt
  /clear             Clear the screen
  /session [id]      Show or load a session
  /new               Start a new conversation

Tips:
  - End a line with \ to continue on the next line
  - Messages are automatically saved to the current session
`
	fmt.Fprint(r.writer, help)
}

func (r *REPL) getPrompt() string {
	if r.session != nil {
		return fmt.Sprintf("[%s] > ", r.session.ID[:8])
	}
	return "> "
}
