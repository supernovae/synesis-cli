package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/bundle"
	"synesis.sh/synesis/pkg/clipboard"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/session"
	"synesis.sh/synesis/pkg/ui"
)

// sessionToAPIMessages converts session messages to API messages
func sessionToAPIMessages(msgs []session.Message) []api.Message {
	result := make([]api.Message, len(msgs))
	for i, m := range msgs {
		result[i] = api.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result
}

// runChat implements the chat command
func runChat(args []string, noColor, quiet bool, profileName string) error {
	// Parse flags
	fs := flag.NewFlagSet("chat", flag.ExitOnError)
	model := fs.String("model", "", "model to use")
	temperature := fs.Float64("temperature", 0.7, "temperature setting")
	maxTokens := fs.Int("max-tokens", 0, "max tokens (0 = default)")
	stream := fs.Bool("stream", true, "stream response")
	sessionID := fs.String("session", "", "continue existing session")
	system := fs.String("system", "", "system prompt")
	saveSession := fs.Bool("save-session", false, "save session after chat")
	timeout := fs.Int("timeout", 0, "timeout in seconds")
	includeStdin := fs.Bool("include-stdin", false, "include stdin in prompt")
	fromClipboard := fs.Bool("from-clipboard", false, "read prompt from clipboard")
	copyLast := fs.Bool("copy-last", false, "copy last response to clipboard")
	raw := fs.Bool("raw", false, "output raw response without formatting")
	renderModeStr := fs.String("render", "plain", "render mode: plain, markdown, raw")
	toolsFile := fs.String("tools", "", "JSON file with tool definitions")
	toolChoice := fs.String("tool-choice", "auto", "tool choice: auto, none, required")
	output := fs.String("output", "text", "output format: text, json, ndjson")
	dryRun := fs.Bool("dry-run", false, "show request that would be sent without making API call")
	showUsage := fs.Bool("usage", false, "show token usage and latency after response")
	bundlePath := fs.String("bundle", "", "bundle file to load (YAML format)")

	fs.Parse(args)

	// Detect if stdin has content
	stat, _ := os.Stdin.Stat()
	hasStdin := (stat.Mode() & os.ModeCharDevice) == 0

	// Check for TTY mode
	isTTY := ui.IsTerminal()

	// Load config
	cfg, err := config.Resolve(profileName)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Override with flags
	modelName := cfg.Cfg.Model
	if *model != "" {
		modelName = *model
	}
	if modelName == "" {
		modelName = "gpt-4o-mini" // default
	}

	// Get session store
	store, err := getSessionStore()
	if err != nil {
		return fmt.Errorf("session error: %w", err)
	}

	// Setup messages
	var messages []api.Message
	var sess *session.Session

	// Load or create session
	if *sessionID != "" {
		sess, err = store.Get(*sessionID)
		if err != nil {
			sess, err = store.FindByName(*sessionID)
			if err != nil {
				return fmt.Errorf("session not found: %s", *sessionID)
			}
		}
		messages = sessionToAPIMessages(sess.Messages)
	} else {
		// Create new session
		sess, err = store.Create(modelName, *system)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
	}

	// Load bundle if specified
	var bundlePrompt strings.Builder
	var bundleSystem string
	if *bundlePath != "" {
		b, err := bundle.Load(*bundlePath)
		if err != nil {
			return fmt.Errorf("load bundle: %w", err)
		}
		if err := b.Validate(); err != nil {
			return fmt.Errorf("bundle validation: %w", err)
		}
		bundleSystem = b.GetSystem()
		bundlePromptStr, err := b.GetPrompt()
		if err != nil {
			return fmt.Errorf("bundle prompt: %w", err)
		}
		bundlePrompt.WriteString(bundlePromptStr)
	}

	// Add system message from bundle or flag (flag overrides bundle)
	finalSystem := *system
	if finalSystem == "" && bundleSystem != "" {
		finalSystem = bundleSystem
	}
	if finalSystem != "" {
		messages = append(messages, api.Message{Role: "system", Content: finalSystem})
	} else if sess.System != "" {
		messages = append(messages, api.Message{Role: "system", Content: sess.System})
	}

	// Add bundle prompt to clipboard if requested
	if *fromClipboard {
		clipboardText, err := clipboard.Paste()
		if err != nil {
			return fmt.Errorf("failed to read from clipboard: %w", err)
		}
		if clipboardText != "" {
			if bundlePrompt.Len() > 0 {
				bundlePrompt.WriteString("\n\n")
			}
			bundlePrompt.WriteString(clipboardText)
		}
	}

	// Track whether stdin was consumed as prompt
	promptRead := false

	// Read stdin if available and requested
	if hasStdin && *includeStdin && !*fromClipboard {
		stdinData, err := os.ReadFile("/dev/stdin")
		if err == nil {
			prompt := strings.TrimSpace(string(stdinData))
			if prompt != "" {
				messages = append(messages, api.Message{Role: "user", Content: prompt})
				promptRead = true
			}
		}
	}

	// Read prompt interactively if no args provided and no prompt read yet
	if len(fs.Args()) == 0 && !promptRead && !*fromClipboard {
		fmt.Println("Enter your prompt (Ctrl+C to exit):")
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("read error: %w", err)
			}
			line = strings.TrimRight(line, "\r\n")
			if line != "" {
				messages = append(messages, api.Message{Role: "user", Content: line})
				break
			}
		}
	}

	if len(fs.Args()) > 0 {
		messages = append(messages, api.Message{Role: "user", Content: strings.Join(fs.Args(), " ")})
	}

	// If no messages, error
	if len(messages) == 0 {
		return fmt.Errorf("no prompt provided")
	}

	// Load tools if specified
	var tools []api.Tool
	if *toolsFile != "" {
		data, err := os.ReadFile(*toolsFile)
		if err != nil {
			return fmt.Errorf("read tools file: %w", err)
		}
		if err := json.Unmarshal(data, &tools); err != nil {
			return fmt.Errorf("parse tools JSON: %w", err)
		}
	}

	// Build request
	req := &api.ChatRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: *temperature,
		Tools:       tools,
	}
	if *maxTokens > 0 {
		req.MaxTokens = *maxTokens
	}
	if *toolChoice != "" && *toolChoice != "auto" {
		if *toolChoice == "none" {
			req.ToolChoice = "none"
		} else if *toolChoice == "required" {
			req.ToolChoice = "required"
		}
	}

	// Handle dry-run mode
	if *dryRun {
		outputJSON := *output == "json" || *output == "ndjson"
		ui.PrintDryRun(cfg, req, outputJSON)
		return nil
	}

	// Create client
	cli := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	defer cli.Close()

	// Setup context with timeout
	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*timeout)*time.Second)
		defer cancel()
	}

	// Track timing for usage reporting
	startTime := time.Now()

	// Determine output mode
	var outputMode ui.OutputMode
	switch *output {
	case "json":
		outputMode = ui.OutputJSON
	case "ndjson":
		outputMode = ui.OutputNDJSON
	}

	// Parse render mode
	renderMode := ui.RenderPlain
	if *renderModeStr != "" {
		m, err := ui.ParseRenderMode(*renderModeStr)
		if err != nil {
			return fmt.Errorf("render mode: %w", err)
		}
		renderMode = m
	}

	// Execute based on streaming preference
	var finalContent string
	if *stream && isTTY {
		// Streaming mode
		fmt.Print(" Thinking...")
		var content strings.Builder
		err := cli.StreamChat(ctx, req, func(token string, err error) {
			if err != nil {
				ui.Error("%v", err)
				return
			}
			content.WriteString(token)
			fmt.Print(token)
		})
		if err != nil {
			return err
		}
		finalContent = content.String()
		fmt.Println()
	} else {
		// Non-streaming mode
		resp, err := cli.Chat(ctx, req)
		if err != nil {
			return err
		}

		if len(resp.Choices) == 0 {
			return fmt.Errorf("no response from API")
		}

		finalContent = resp.Choices[0].Message.Content

		// Show usage if requested
		if *showUsage {
			latencyMs := time.Since(startTime).Milliseconds()
			ui.PrintUsage(modelName, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, latencyMs)
		}

		// Handle output format for non-streaming
		switch outputMode {
		case ui.OutputJSON:
			fmt.Fprintf(os.Stdout, `{"content": "%s"}`+"\n", escapeJSON(finalContent))
		case ui.OutputNDJSON:
			fmt.Fprintf(os.Stdout, "\"%s\"\n", escapeJSON(finalContent))
		default:
			// Apply render mode
			rendered := ui.RenderResponse(finalContent, renderMode, noColor, isTTY)
			if *raw || renderMode == ui.RenderRaw {
				os.Stdout.WriteString(rendered)
			} else {
				fmt.Println(rendered)
			}
		}
	}

	// Build user prompt string for saving
	promptStr := ""
	if hasStdin && *includeStdin {
		stdinData, _ := os.ReadFile("/dev/stdin")
		promptStr = strings.TrimSpace(string(stdinData))
		if len(fs.Args()) > 0 {
			promptStr = strings.Join(fs.Args(), " ") + "\n\n" + promptStr
		}
	} else {
		promptStr = strings.Join(fs.Args(), " ")
	}

	// Save user and assistant messages to session
	_ = store.AddMessage(sess, "user", promptStr)
	_ = store.AddMessage(sess, "assistant", finalContent)

	// Save session if requested
	if *saveSession {
		_ = store.Update(sess)
		if !quiet {
			fmt.Fprintf(os.Stderr, "Session saved: %s\n", sess.ID)
		}
	}

	// Copy last response to clipboard if requested
	if *copyLast {
		if err := clipboard.Copy(finalContent); err != nil {
			if !quiet {
				ui.Error("Failed to copy to clipboard: %v", err)
			}
		}
	}

	return nil
}
