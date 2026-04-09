package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/ui"
)

// runCommitMessage generates commit messages from git diff
func runCommitMessage(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("commit-message", flag.ContinueOnError)
	fs.SetOutput(nil)
	model := fs.String("model", "", "model to use")
	temperature := fs.Float64("temperature", 0.5, "temperature")
	timeout := fs.Int("timeout", 60, "timeout in seconds")
	output := fs.String("output", "text", "output format: text, json")
	conventional := fs.Bool("conventional", false, "conventional commits format")
	notify := fs.String("notify", "", "notify scope (e.g., api, ui)")
	renderModeStr := fs.String("render", "plain", "render mode: plain, markdown, raw")
	dryRun := fs.Bool("dry-run", false, "show request that would be sent without making API call")
	showUsage := fs.Bool("usage", false, "show token usage and latency after response")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printCommitMessageUsage()
			return nil
		}
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

	// Read stdin or use git diff
	var content string
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Piped
		data, _ := os.ReadFile("/dev/stdin")
		content = strings.TrimSpace(string(data))
	} else {
		// Try git diff
		data, _ := os.ReadFile("/dev/stdin") // This will be empty, try running git diff
		// Actually we need to check if there's a git repo and get diff
		_ = data
		// For now, require stdin if no args
	}

	if content == "" && len(fs.Args()) > 0 {
		// Try reading from file args
		for _, f := range fs.Args() {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			content += string(data) + "\n"
		}
	}

	if content == "" {
		return fmt.Errorf("no input: provide via stdin or file arguments (use -h for help)")
	}

	// Build commit message prompt
	var prompt strings.Builder
	prompt.WriteString("Generate a commit message for the following changes.\n\n")
	prompt.WriteString("Changes:\n")
	prompt.WriteString(content)

	if *conventional {
		prompt.WriteString("\n\nUse conventional commit format: <type>(<scope>): <description>")
		if *notify != "" {
			prompt.WriteString("\nScope: " + *notify)
		}
		prompt.WriteString("\nTypes: feat, fix, docs, style, refactor, test, chore")
	}

	prompt.WriteString("\n\nRespond ONLY with the commit message, no explanation.")

	// Load config
	cfg, err := config.Resolve(profileName)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if err := cfg.Cfg.Validate(); err != nil {
		return err
	}

	// Model
	modelName := cfg.Cfg.Model
	if *model != "" {
		modelName = *model
	}
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	// Request
	messages := []api.Message{
		{Role: "system", Content: "You are an expert at writing concise, descriptive commit messages. Format them clearly."},
		{Role: "user", Content: prompt.String()},
	}

	req := &api.ChatRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: *temperature,
	}

	// Handle dry-run mode
	if *dryRun {
		outputJSON := *output == "json" || *output == "ndjson"
		ui.PrintDryRun(cfg, req, outputJSON)
		return nil
	}

	// Track timing for usage reporting
	startTime := time.Now()

	// Execute
	cli := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	resp, err := cli.Chat(ctx, req)
	if err != nil {
		return err
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response")
	}

	msg := resp.Choices[0].Message.Content
	msg = strings.TrimSpace(msg)

	// Show usage if requested
	if *showUsage {
		latencyMs := time.Since(startTime).Milliseconds()
		ui.PrintUsage(modelName, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, latencyMs)
	}

	switch *output {
	case "json":
		jsonOutput := fmt.Sprintf(`{"commit_message": %s}`, jsonMarshal(msg))
		rendered := ui.RenderResponse(jsonOutput, renderMode, noColor, ui.IsTerminal())
		fmt.Fprintln(os.Stdout, rendered)
	default:
		rendered := ui.RenderResponse(msg, renderMode, noColor, ui.IsTerminal())
		fmt.Println(rendered)
	}

	return nil
}

func printCommitMessageUsage() {
	fmt.Print(`synesis commit-message - Generate commit messages from git diff

Usage: synesis commit-message [options]

Options:
  -model string        model to use
  -temperature float   temperature (default 0.5)
  -timeout int         timeout in seconds (default 60)
  -output string       output format: text, json (default "text")
  -conventional        use conventional commits format
  -notify string       notify scope (e.g., api, ui)
  -render string       render mode: plain, markdown, raw (default "plain")
  -dry-run             show request that would be sent without making API call
  -usage               show token usage and latency after response

Examples:
  git diff | synesis commit-message
  git diff | synesis commit-message --conventional
  git diff | synesis commit-message --usage

`)
}