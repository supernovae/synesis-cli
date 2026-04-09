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

// runSummarize implements the summarize command
func runSummarize(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("summarize", flag.ContinueOnError)
	fs.SetOutput(nil)
	model := fs.String("model", "", "model to use")
	temperature := fs.Float64("temperature", 0.5, "temperature")
	timeout := fs.Int("timeout", 120, "timeout in seconds")
	output := fs.String("output", "text", "output format: text, json")
	short := fs.Bool("short", false, "short summary")
	renderModeStr := fs.String("render", "plain", "render mode: plain, markdown, raw")
	dryRun := fs.Bool("dry-run", false, "show request that would be sent without making API call")
	showUsage := fs.Bool("usage", false, "show token usage and latency after response")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printSummarizeUsage()
			return nil
		}
	}

	// Check for stdin
	stat, _ := os.Stdin.Stat()
	hasStdin := (stat.Mode() & os.ModeCharDevice) == 0

	var content string
	if hasStdin {
		data, _ := os.ReadFile("/dev/stdin")
		content = strings.TrimSpace(string(data))
	}

	// Build prompt
	var prompt string
	if *short {
		prompt = "Summarize the following in 1-2 sentences:\n\n" + content
	} else {
		prompt = "Summarize the following concisely:\n\n" + content
	}

	// Add remaining args
	if len(fs.Args()) > 0 {
		prompt += "\n\nUser note: " + strings.Join(fs.Args(), " ")
	}

	if content == "" && len(fs.Args()) == 0 {
		return fmt.Errorf("no input (use -h for help)")
	}

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
		{Role: "system", Content: "You are a helpful assistant that summarizes content concisely."},
		{Role: "user", Content: prompt},
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

	// Execute
	cli := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Track timing for usage reporting
	startTime := time.Now()

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

	// Parse render mode
	renderMode := ui.RenderPlain
	if *renderModeStr != "" {
		m, err := ui.ParseRenderMode(*renderModeStr)
		if err != nil {
			return fmt.Errorf("render mode: %w", err)
		}
		renderMode = m
	}

	switch *output {
	case "json":
		fmt.Fprintf(os.Stdout, `{"summary": %s}`+"\n", jsonMarshal(content))
	default:
		// Apply render mode
		rendered := ui.RenderResponse(content, renderMode, noColor, ui.IsTerminal())
		fmt.Println(rendered)
	}

	return nil
}

func printSummarizeUsage() {
	fmt.Print(`synesis summarize - Summarize input text

Usage: synesis summarize [options]

Options:
  -model string        model to use
  -temperature float   temperature (default 0.5)
  -timeout int         timeout in seconds (default 120)
  -output string       output format: text, json (default "text")
  -short               short summary (1-2 sentences)
  -render string       render mode: plain, markdown, raw (default "plain")
  -dry-run             show request that would be sent without making API call
  -usage               show token usage and latency after response

Examples:
  cat article.txt | synesis summarize
  cat report.pdf | synesis summarize --short
  echo "Some text" | synesis summarize --usage

`)
}