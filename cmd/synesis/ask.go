package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/bundle"
	"synesis.sh/synesis/pkg/clipboard"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/jq"
	outputpkg "synesis.sh/synesis/pkg/output"
	"synesis.sh/synesis/pkg/ui"
)

// runAsk implements the ask command (one-shot mode)
func runAsk(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(nil) // Disable default error output
	model := fs.String("model", "", "model to use")
	temperature := fs.Float64("temperature", 0.7, "temperature")
	maxTokens := fs.Int("max-tokens", 0, "max tokens")
	system := fs.String("system", "", "system prompt")
	timeout := fs.Int("timeout", 120, "timeout in seconds")
	output := fs.String("output", "text", "output format: text, json, ndjson")
	raw := fs.Bool("raw", false, "raw output")
	renderModeStr := fs.String("render", "plain", "render mode: plain, markdown, raw")
	toolsFile := fs.String("tools", "", "JSON file with tool definitions")
	toolChoice := fs.String("tool-choice", "auto", "tool choice: auto, none, required")
	noStream := fs.Bool("no-stream", false, "disable streaming")
	includeStdin := fs.Bool("include-stdin", true, "include stdin in prompt")
	fromClipboard := fs.Bool("from-clipboard", false, "read prompt from clipboard")
	copyLast := fs.Bool("copy-last", false, "copy last response to clipboard")
	dryRun := fs.Bool("dry-run", false, "show request that would be sent without making API call")
	printRequest := fs.Bool("print-request", false, "print full request payload (redacted)")
	jqExpr := fs.String("jq", "", "jq-style field selection (e.g., .choices[0].message.content)")
	showUsage := fs.Bool("usage", false, "show token usage and latency after response")
	bundlePath := fs.String("bundle", "", "bundle file to load (YAML format)")
	extractPath := fs.String("extract-path", "", "extract JSON field using dot notation (e.g., choices.0.message.content)")
	writeOutput := fs.String("write-output", "", "write output to file (overwrites)")
	appendOutput := fs.String("append-output", "", "append output to file")

	// Parse, capturing error but not printing
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printAskUsage()
			return nil
		}
		return err
	}

	// Check if stdin has content
	stat, _ := os.Stdin.Stat()
	hasStdin := (stat.Mode() & os.ModeCharDevice) == 0

	// Load config
	cfg, err := config.Resolve(profileName)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Validate config
	if err := cfg.Cfg.Validate(); err != nil {
		return err
	}

	// Model override
	modelName := cfg.Cfg.Model
	if *model != "" {
		modelName = *model
	}
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	// Build messages
	var messages []api.Message

	// Load bundle if specified
	var bundlePrompt strings.Builder
	var bundleSystem string
	// bundleFiles reserved for future use (file references from bundle)
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

	// Add system prompt from bundle or flag (flag overrides bundle)
	finalSystem := *system
	if finalSystem == "" && bundleSystem != "" {
		finalSystem = bundleSystem
	}
	if finalSystem != "" {
		messages = append(messages, api.Message{Role: "system", Content: finalSystem})
	}

	// Build user prompt
	var prompt strings.Builder

	// Add bundle prompt first
	if bundlePrompt.Len() > 0 {
		prompt.WriteString(bundlePrompt.String())
	}

	// Add clipboard content if requested
	if *fromClipboard {
		clipboardText, err := clipboard.Paste()
		if err != nil {
			return fmt.Errorf("failed to read from clipboard: %w", err)
		}
		if clipboardText != "" {
			if prompt.Len() > 0 {
				prompt.WriteString("\n\n")
			}
			prompt.WriteString(clipboardText)
		}
	}

	// First add positional args (if not from clipboard)
	if len(fs.Args()) > 0 && !*fromClipboard {
		if prompt.Len() > 0 {
			prompt.WriteString("\n\n")
		}
		prompt.WriteString(strings.Join(fs.Args(), " "))
	}

	// Add stdin if present
	if hasStdin && *includeStdin {
		stdinData, err := os.ReadFile("/dev/stdin")
		if err == nil {
			stdinContent := strings.TrimSpace(string(stdinData))
			if stdinContent != "" {
				if prompt.Len() > 0 {
					prompt.WriteString("\n\n")
				}
				prompt.WriteString(stdinContent)
			}
		}
	}

	if prompt.Len() == 0 {
		return fmt.Errorf("no prompt provided (use -h for help)")
	}

	messages = append(messages, api.Message{Role: "user", Content: prompt.String()})

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
		Model:         modelName,
		Messages:      messages,
		Temperature:   *temperature,
		Stream:        !*noStream && ui.IsTerminal(),
		Tools:         tools,
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

	// Print full request if requested
	if *printRequest {
		redactedReq := redactRequest(req)
		outputJSON, _ := json.MarshalIndent(redactedReq, "", "  ")
		fmt.Fprintf(os.Stderr, "Request payload:\n%s\n\n", string(outputJSON))
	}

	// Create client
	cli := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	defer cli.Close()

	// Setup context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Track timing for usage reporting
	startTime := time.Now()

	// Output mode
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

	isFrontend := ui.IsTerminal() && !*noStream
	var content string

	if isFrontend {
		// Streaming mode for terminal
		var contentBuilder strings.Builder
		err := cli.StreamChat(ctx, req, func(token string, err error) {
			if err != nil {
				ui.Error("%v", err)
				return
			}
			contentBuilder.WriteString(token)
			os.Stdout.WriteString(token)
			os.Stdout.Sync()
		})
		if err != nil {
			return err
		}
		os.Stdout.WriteString("\n")
		content = contentBuilder.String()
	} else {
		// Non-streaming for scripts
		resp, err := cli.Chat(ctx, req)
		if err != nil {
			return err
		}

		if len(resp.Choices) == 0 {
			return fmt.Errorf("no response")
		}

		content = resp.Choices[0].Message.Content

		// Show usage if requested
		if *showUsage {
			latencyMs := time.Since(startTime).Milliseconds()
			ui.PrintUsage(modelName, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, latencyMs)
		}

		// Handle output format for non-streaming
		switch outputMode {
		case ui.OutputJSON:
			fmt.Fprintf(os.Stdout, `{"content": %s}`+"\n", jsonMarshal(content))
		case ui.OutputNDJSON:
			fmt.Fprintf(os.Stdout, `%s`+"\n", jsonMarshal(content))
		default:
			// Apply render mode
			rendered := ui.RenderResponse(content, renderMode, noColor, ui.IsTerminal())
			if *raw || renderMode == ui.RenderRaw {
				os.Stdout.WriteString(rendered)
			} else {
				fmt.Println(rendered)
			}
		}
	}

	// Apply jq-style field selection if specified
	if *jqExpr != "" {
		result, err := jq.Apply(content, *jqExpr)
		if err != nil {
			return fmt.Errorf("jq filter: %w", err)
		}
		fmt.Println(result)
		content = result
	}

	// Apply extract-path if specified (and jq not specified)
	if *extractPath != "" && *jqExpr == "" {
		result, err := outputpkg.ExtractPath([]byte(content), *extractPath)
		if err != nil {
			return fmt.Errorf("extract-path: %w", err)
		}
		fmt.Println(result)
		content = string(result)
	}

	// Write to file if specified
	if *writeOutput != "" {
		if err := outputpkg.WriteOutput([]byte(content), *writeOutput); err != nil {
			return err
		}
	}

	// Append to file if specified
	if *appendOutput != "" {
		if err := outputpkg.AppendOutput([]byte(content), *appendOutput); err != nil {
			return err
		}
	}

	if *copyLast {
		if err := clipboard.Copy(content); err != nil {
			if !quiet {
				ui.Error("Failed to copy to clipboard: %v", err)
			}
		}
	}

	return nil
}

// redactRequest creates a copy of the request with sensitive fields redacted
func redactRequest(req *api.ChatRequest) *api.ChatRequest {
	redacted := &api.ChatRequest{
		Model:         req.Model,
		Messages:      make([]api.Message, len(req.Messages)),
		Temperature:   req.Temperature,
		Stream:        req.Stream,
		MaxTokens:     req.MaxTokens,
		ToolChoice:    req.ToolChoice,
		Tools:         make([]api.Tool, len(req.Tools)),
	}

	for i, msg := range req.Messages {
		redacted.Messages[i] = api.Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  msg.ToolCalls,
			ToolCallID: msg.ToolCallID,
		}
		if msg.Role == "system" {
			redacted.Messages[i].Content = "[REDACTED]"
		}
	}

	for i, tool := range req.Tools {
		redacted.Tools[i] = tool
	}

	return redacted
}

func printAskUsage() {
	fmt.Print(`synesis ask - One-shot prompt/answer mode

Usage: synesis ask [options] <prompt>

Options:
  -model string        model to use
  -temperature float   temperature (default 0.7)
  -max-tokens int      max tokens
  -system string       system prompt
  -timeout int         timeout in seconds (default 120)
  -output text|json|ndjson   output format (default text)
  -raw                 raw output (no newline)
  -render string       render mode: plain, markdown, raw (default "plain")
  -tools file          JSON file with tool definitions
  -tool-choice string  tool choice: auto, none, required (default "auto")
  -no-stream           disable streaming
  -include-stdin bool  include stdin in prompt (default true)
  -from-clipboard      read prompt from clipboard
  -copy-last           copy last response to clipboard
  -bundle file         bundle file to load (YAML format)
  -jq string           jq-style field selection (e.g., .choices[0].message.content)
  -extract-path string extract JSON field using dot notation (e.g., choices.0.message.content)
  -print-request       print full request payload (redacted)
  -write-output file   write output to file (overwrites)
  -append-output file  append output to file

Examples:
  synesis ask "what time is it"
  echo "hello world" | synesis ask "translate to french"
  synesis ask --output json "list 3 colors"
  synesis ask --render markdown "explain closures"
  synesis ask --tools functions.json --tool-choice required "extract data"
  cat log.txt | synesis ask "find errors"
  synesis ask --bundle mybundle.yaml
  synesis ask --jq '.choices[0].message.content' "what is 2+2"
  synesis ask --print-request "hello"

Bundle Format (YAML):
  system: "You are a helpful assistant"
  prompt: "Analyze this data"
  files:
    - path: data.csv
      role: context
  model: gpt-4o
  temperature: 0.7
  max_tokens: 2000

`)
}