package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xeipuuv/gojsonschema"

	"synesis.sh/synesis/internal/api"
	"synesis.sh/synesis/pkg/config"
	"synesis.sh/synesis/pkg/ui"
)

// stringSliceValue implements flag.Value for repeatable string flags
type stringSliceValue struct {
	values *[]string
}

func (v *stringSliceValue) Set(s string) error {
	*(v.values) = append(*(v.values), s)
	return nil
}

func (v *stringSliceValue) String() string {
	return ""
}

// runExtract implements structured field extraction
func runExtract(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(nil)
	model := fs.String("model", "", "model to use")
	temperature := fs.Float64("temperature", 0.3, "temperature (lower = more deterministic)")
	timeout := fs.Int("timeout", 120, "timeout in seconds")
	// Use a custom var to properly collect multiple --field flags
	var fieldList []string
	fs.Var(&stringSliceValue{&fieldList}, "field", "field to extract (can repeat)")
	schemaFile := fs.String("schema", "", "JSON schema file")
	metadata := fs.Bool("metadata", false, "include uncertainty metadata")
	renderModeStr := fs.String("render", "plain", "render mode: plain, markdown, raw")
	output := fs.String("output", "json", "output format: json")
	dryRun := fs.Bool("dry-run", false, "show request that would be sent without making API call")
	showUsage := fs.Bool("usage", false, "show token usage and latency after response")

	// Parse with custom handling
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printExtractUsage()
			return nil
		}
		// Continue on error to allow manual handling
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

	// Get the parsed fields from the var

	if len(fieldList) == 0 {
		return fmt.Errorf("at least one --field required (use -h for help)")
	}

	// Check if stdin has content
	stat, _ := os.Stdin.Stat()
	hasStdin := (stat.Mode() & os.ModeCharDevice) == 0

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

	// Build extraction prompt
	var inputContent string
	if hasStdin {
		data, _ := os.ReadFile("/dev/stdin")
		inputContent = strings.TrimSpace(string(data))
	}

	// Build prompt
	var prompt strings.Builder
	prompt.WriteString("Extract the following fields FROM the text below. This is a data extraction task - you are NOT being asked a question about these topics. Simply parse the text and extract the requested values into JSON.")
	prompt.WriteString("\n\nFields to extract: ")
	for _, f := range fieldList {
		prompt.WriteString(f + ", ")
	}
	prompt.WriteString("\n\nText to extract from:\n")
	if inputContent != "" {
		prompt.WriteString(inputContent)
	} else {
		prompt.WriteString(strings.Join(fs.Args(), " "))
	}
	prompt.WriteString("\n\nIMPORTANT: Extract values FROM this text. Do NOT answer questions about these fields. Respond ONLY with valid JSON (no prose or clarification questions). Use null for fields you cannot extract confidently.")
	prompt.WriteString("\n\nOutput format: {")

	// Add field definitions
	for i, f := range fieldList {
		if i > 0 {
			prompt.WriteString(", ")
		}
		prompt.WriteString(fmt.Sprintf(`"%s": <value for %s>`, f, f))
	}
	prompt.WriteString("}")

	// Create request
	messages := []api.Message{
		{Role: "system", Content: "You are a data extraction tool. Given text, extract the requested fields into JSON. This is NOT a conversation - do NOT ask clarifying questions. Just parse the input and return JSON. Use null for missing fields. NEVER respond with prose, never ask questions, always respond with valid JSON only."},
		{Role: "user", Content: prompt.String()},
	}

	req := &api.ChatRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: *temperature,
		ResponseFormat: api.ResponseFormat{Type: "json_object"},
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

	// Setup context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Track timing for usage reporting
	startTime := time.Now()

	// Execute
	resp, err := cli.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response")
	}

	// Parse JSON from response
	content := resp.Choices[0].Message.Content
	content = strings.TrimSpace(content)

	// Try to extract JSON from markdown if present
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		if idx := strings.Index(content, "\n"); idx > 0 {
			content = content[idx+1:]
		}
		if strings.HasSuffix(content, "```") {
			content = strings.TrimSuffix(content, "```")
		}
	}
	content = strings.TrimSpace(content)

	// Parse the JSON
	var extracted map[string]any
	if err := json.Unmarshal([]byte(content), &extracted); err != nil {
		// Return raw content with null fields
		result := make(map[string]any)
		for _, f := range fieldList {
			result[f] = nil
		}
		if *metadata {
			result["_metadata"] = map[string]string{
				"error":    "parse error",
				"raw":      content,
				"original": err.Error(),
			}
		}
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
		return nil
	}

	// Validate against JSON schema if provided
	if *schemaFile != "" {
		schemaData, err := os.ReadFile(*schemaFile)
		if err != nil {
			return fmt.Errorf("read schema file: %w", err)
		}
		schemaLoader := gojsonschema.NewBytesLoader(schemaData)
		documentLoader := gojsonschema.NewGoLoader(extracted)
		res, err := gojsonschema.Validate(schemaLoader, documentLoader)
		if err != nil {
			return fmt.Errorf("schema validation error: %w", err)
		}
		if !res.Valid() {
			var msgs []string
			for _, verr := range res.Errors() {
				msgs = append(msgs, verr.String())
			}
			return fmt.Errorf("schema validation failed:\n  %s", strings.Join(msgs, "\n  "))
		}
	}

	// Ensure all requested fields exist with null for missing
	result := make(map[string]any)
	for _, f := range fieldList {
		if v, ok := extracted[f]; ok {
			result[f] = v
		} else {
			result[f] = nil
		}
	}

	// Add metadata if requested
	if *metadata {
		result["_metadata"] = map[string]string{
			"extracted_from": "input",
			"model":          modelName,
		}
	}

	// Show usage if requested
	if *showUsage {
		latencyMs := time.Since(startTime).Milliseconds()
		ui.PrintUsage(modelName, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, latencyMs)
	}

	// Output
	switch *output {
	case "json":
		data, _ := json.MarshalIndent(result, "", "  ")
		rendered := ui.RenderResponse(string(data), renderMode, noColor, ui.IsTerminal())
		fmt.Println(rendered)
	default:
		data, _ := json.Marshal(result)
		rendered := ui.RenderResponse(string(data), renderMode, noColor, ui.IsTerminal())
		fmt.Println(rendered)
	}

	return nil
}

func printExtractUsage() {
	fmt.Print(`synesis extract - Extract structured fields from input

Usage: synesis extract --field <name> [--field <name>...] [options]

Options:
  -model string        model to use
  -temperature float   temperature (default 0.3, lower = more deterministic)
  -timeout int         timeout in seconds (default 120)
  -schema file         JSON schema file (optional)
  -metadata            include uncertainty metadata
  -output json         output format (default json)

Examples:
  cat incident.txt | synesis extract --field service --field severity --field impact
  echo "Error in service x at 3pm" | synesis extract --field error_type --field timestamp
  synesis extract --field title --field author < story.txt

`)
}