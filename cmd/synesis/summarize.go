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
)

// runSummarize implements the summarize command
func runSummarize(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("summarize", flag.ContinueOnError)
	fs.SetOutput(nil)
	model := fs.String("model", "", "model to use")
	temperature := fs.Float64("temperature", 0.5, "temperature")
	timeout := fs.Int("timeout", 120, "timeout in seconds")
	output := fs.String("output", "text", "output format: text, json")
	short := fs.Bool("short", false, "short summary")

	fs.Parse(args)

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
	cfg, err := config.Resolve()
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

	content = resp.Choices[0].Message.Content

	switch *output {
	case "json":
		fmt.Fprintf(os.Stdout, `{"summary": %s}`+"\n", jsonMarshal(content))
	default:
		fmt.Println(content)
	}

	return nil
}