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

// runAsk implements the ask command (one-shot mode)
func runAsk(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(nil) // Disable default error output
	model := fs.String("model", "", "model to use")
	temperature := fs.Float64("temperature", 0.7, "temperature")
	maxTokens := fs.Int("max-tokens", 0, "max tokens")
	system := fs.String("system", "", "system prompt")
	timeout := fs.Int("timeout", 120, "timeout in seconds")
	output := fs.String("output", "text", "output format: text, json, ndjson")
	raw := fs.Bool("raw", false, "raw output")
	noStream := fs.Bool("no-stream", false, "disable streaming")
	includeStdin := fs.Bool("include-stdin", true, "include stdin in prompt")

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
	cfg, err := config.Resolve()
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

	// Add system prompt if provided
	if *system != "" {
		messages = append(messages, api.Message{Role: "system", Content: *system})
	}

	// Build user prompt
	var prompt strings.Builder

	// First add positional args
	if len(fs.Args()) > 0 {
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

	// Build request
	req := &api.ChatRequest{
		Model:         modelName,
		Messages:      messages,
		Temperature:   *temperature,
		Stream:        !*noStream && ui.IsTerminal(), // Stream in terminal
	}
	if *maxTokens > 0 {
		req.MaxTokens = *maxTokens
	}

	// Create client
	cli := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	defer cli.Close()

	// Setup context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Output mode
	var outputMode ui.OutputMode
	switch *output {
	case "json":
		outputMode = ui.OutputJSON
	case "ndjson":
		outputMode = ui.OutputNDJSON
	}

	isFrontend := ui.IsTerminal() && !*noStream

	if isFrontend {
		// Streaming mode for terminal
		var content strings.Builder
		err := cli.StreamChat(ctx, req, func(token string, err error) {
			if err != nil {
				ui.Error("%v", err)
				return
			}
			content.WriteString(token)
			os.Stdout.WriteString(token)
			os.Stdout.Sync()
		})
		if err != nil {
			return err
		}
		os.Stdout.WriteString("\n")
	} else {
		// Non-streaming for scripts
		resp, err := cli.Chat(ctx, req)
		if err != nil {
			return err
		}

		if len(resp.Choices) == 0 {
			return fmt.Errorf("no response")
		}

		content := resp.Choices[0].Message.Content

		switch outputMode {
		case ui.OutputJSON:
			fmt.Fprintf(os.Stdout, `{"content": %s}`+"\n", jsonMarshal(content))
		case ui.OutputNDJSON:
			fmt.Fprintf(os.Stdout, `%s`+"\n", jsonMarshal(content))
		default:
			if *raw {
				os.Stdout.WriteString(content)
			} else {
				fmt.Println(content)
			}
		}
	}

	return nil
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
  -no-stream           disable streaming
  -include-stdin bool  include stdin in prompt (default true)

Examples:
  synesis ask "what time is it"
  echo "hello world" | synesis ask "translate to french"
  synesis ask --output json "list 3 colors"
  cat log.txt | synesis ask "find errors"

`)
}