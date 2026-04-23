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
	"synesis.sh/synesis/pkg/schema"
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
	var fieldList []string
	fs.Var(&stringSliceValue{&fieldList}, "field", "field to extract (can repeat)")
	schemaName := fs.String("schema", "", "named schema or JSON schema file path")
	schemaFile := fs.String("schema-file", "", "JSON schema file path (explicit)")
	listSchemas := fs.Bool("list-schemas", false, "list available named schemas")
	maxRepair := fs.Int("max-repair", 2, "max repair retries when schema validation fails")
	metadata := fs.Bool("metadata", false, "include uncertainty metadata")
	renderModeStr := fs.String("render", "plain", "render mode: plain, markdown, raw")
	output := fs.String("output", "json", "output format: json")
	dryRun := fs.Bool("dry-run", false, "show request that would be sent without making API call")
	showUsage := fs.Bool("usage", false, "show token usage and latency after response")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			printExtractUsage()
			return nil
		}
	}

	// Handle --list-schemas before anything else
	if *listSchemas {
		return runListSchemas()
	}

	renderMode := ui.RenderPlain
	if *renderModeStr != "" {
		m, err := ui.ParseRenderMode(*renderModeStr)
		if err != nil {
			return fmt.Errorf("render mode: %w", err)
		}
		renderMode = m
	}

	// Resolve schema: --schema-file takes precedence over --schema
	var sch *schema.Schema
	var schemaErr error
	switch {
	case *schemaFile != "":
		sch, schemaErr = schema.Load(*schemaFile)
	case *schemaName != "":
		store := schema.NewStore()
		sch, schemaErr = store.Find(*schemaName)
	}
	if schemaErr != nil {
		return fmt.Errorf("schema: %w", schemaErr)
	}

	// When a schema is used, derive fieldList from its "required" or top-level "properties" keys.
	if sch != nil && len(fieldList) == 0 {
		fieldList = extractFieldNamesFromSchema(sch)
	}

	if len(fieldList) == 0 {
		return fmt.Errorf("at least one --field required, or provide a --schema/--schema-file (use -h for help)")
	}

	// Check stdin
	stat, _ := os.Stdin.Stat()
	hasStdin := (stat.Mode() & os.ModeCharDevice) == 0

	cfg, err := config.Resolve(profileName)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}
	if err := cfg.Cfg.Validate(); err != nil {
		return err
	}

	modelName := cfg.Cfg.Model
	if *model != "" {
		modelName = *model
	}
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	var inputContent string
	if hasStdin {
		data, _ := os.ReadFile("/dev/stdin")
		inputContent = strings.TrimSpace(string(data))
	}

	// Build extraction prompt
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
	for i, f := range fieldList {
		if i > 0 {
			prompt.WriteString(", ")
		}
		prompt.WriteString(fmt.Sprintf(`"%s": <value for %s>`, f, f))
	}
	prompt.WriteString("}")

	messages := []api.Message{
		{Role: "system", Content: "You are a data extraction tool. Given text, extract the requested fields into JSON. This is NOT a conversation - do NOT ask clarifying questions. Just parse the input and return JSON. Use null for missing fields. NEVER respond with prose, never ask questions, always respond with valid JSON only."},
		{Role: "user", Content: prompt.String()},
	}

	req := &api.ChatRequest{
		Model:          modelName,
		Messages:       messages,
		Temperature:    *temperature,
		ResponseFormat: api.ResponseFormat{Type: "json_object"},
	}

	// If a schema is provided, inject it into response_format for providers that support it.
	if sch != nil {
		var schemaObj map[string]any
		if err := json.Unmarshal(sch.Raw, &schemaObj); err == nil {
			req.ResponseFormat = api.ResponseFormat{
				Type:       "json_schema",
				JsonSchema: schemaObj,
			}
		}
	}

	if *dryRun {
		outputJSON := *output == "json" || *output == "ndjson"
		ui.PrintDryRun(cfg, req, outputJSON)
		return nil
	}

	cli := api.NewClient(cfg.Cfg.BaseURL, cfg.Cfg.APIKey)
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	startTime := time.Now()

	resp, err := cli.Chat(ctx, req)
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("no response")
	}

	content := resp.Choices[0].Message.Content
	content = cleanJSONFromMarkdown(content)

	var extracted map[string]any
	parseErr := json.Unmarshal([]byte(content), &extracted)
	if parseErr != nil {
		extracted = nil
	}

	// Schema validation with bounded repair retries
	repairCount := 0
	for {
		if sch == nil {
			break
		}
		if extracted == nil {
			break
		}
		res, err := sch.Validate(extracted)
		if err != nil {
			break
		}
		if res.Valid() {
			break
		}
		if repairCount >= *maxRepair {
			// Return validation errors as structured output
			var msgs []string
			for _, verr := range res.Errors() {
				msgs = append(msgs, verr.String())
			}
			result := make(map[string]any)
			for _, f := range fieldList {
				if v, ok := extracted[f]; ok {
					result[f] = v
				} else {
					result[f] = nil
				}
			}
			result["_validation_errors"] = msgs
			if *metadata {
				result["_metadata"] = map[string]any{
					"model":         modelName,
					"repair_tries":  repairCount,
					"valid":         false,
				}
			}
			printResult(result, *output, renderMode, noColor)
			return nil
		}

		// Attempt repair: ask model to fix the JSON
		repairCount++
		repairPrompt := buildRepairPrompt(content, res.Errors(), fieldList)
		repairReq := &api.ChatRequest{
			Model:       modelName,
			Messages:    []api.Message{{Role: "user", Content: repairPrompt}},
			Temperature: 0.0,
			ResponseFormat: api.ResponseFormat{Type: "json_object"},
		}
		if sch != nil {
			var schemaObj map[string]any
			if err := json.Unmarshal(sch.Raw, &schemaObj); err == nil {
				repairReq.ResponseFormat = api.ResponseFormat{Type: "json_schema", JsonSchema: schemaObj}
			}
		}
		repairCtx, repairCancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
		repairResp, repairErr := cli.Chat(repairCtx, repairReq)
		repairCancel()
		if repairErr != nil {
			break
		}
		if len(repairResp.Choices) == 0 {
			break
		}
		content = cleanJSONFromMarkdown(repairResp.Choices[0].Message.Content)
		extracted = nil
		if err := json.Unmarshal([]byte(content), &extracted); err != nil {
			break
		}
	}

	// Build final result
	result := make(map[string]any)
	for _, f := range fieldList {
		if extracted != nil {
			if v, ok := extracted[f]; ok {
				result[f] = v
				continue
			}
		}
		result[f] = nil
	}

	if *metadata {
		meta := map[string]any{
			"extracted_from": "input",
			"model":          modelName,
		}
		if sch != nil {
			meta["schema"] = sch.Name
		}
		if repairCount > 0 {
			meta["repair_tries"] = repairCount
		}
		result["_metadata"] = meta
	}

	if *showUsage {
		latencyMs := time.Since(startTime).Milliseconds()
		ui.PrintUsage(modelName, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, latencyMs)
	}

	printResult(result, *output, renderMode, noColor)
	return nil
}

func printResult(result map[string]any, output string, renderMode ui.RenderMode, noColor bool) {
	var data []byte
	switch output {
	case "json":
		data, _ = json.MarshalIndent(result, "", "  ")
	default:
		data, _ = json.Marshal(result)
	}
	rendered := ui.RenderResponse(string(data), renderMode, noColor, ui.IsTerminal())
	fmt.Println(rendered)
}

func cleanJSONFromMarkdown(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		if idx := strings.Index(content, "\n"); idx > 0 {
			content = content[idx+1:]
		}
		if strings.HasSuffix(content, "```") {
			content = strings.TrimSuffix(content, "```")
		}
	}
	return strings.TrimSpace(content)
}

func extractFieldNamesFromSchema(sch *schema.Schema) []string {
	var root map[string]any
	if err := json.Unmarshal(sch.Raw, &root); err != nil {
		return nil
	}
	// Prefer "required" array
	if req, ok := root["required"].([]any); ok {
		var out []string
		for _, v := range req {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	// Fallback to top-level "properties" keys
	if props, ok := root["properties"].(map[string]any); ok {
		var out []string
		for k := range props {
			out = append(out, k)
		}
		return out
	}
	return nil
}

func buildRepairPrompt(badJSON string, errs []gojsonschema.ResultError, fields []string) string {
	var b strings.Builder
	b.WriteString("The following JSON does not conform to the required schema. Please fix it and respond ONLY with corrected valid JSON.\n\nErrors:\n")
	for _, e := range errs {
		b.WriteString("- " + e.String() + "\n")
	}
	b.WriteString("\nInvalid JSON:\n")
	b.WriteString(badJSON)
	b.WriteString("\n\nFields expected: ")
	b.WriteString(strings.Join(fields, ", "))
	b.WriteString("\n\nRespond with valid JSON only.")
	return b.String()
}

func runListSchemas() error {
	store := schema.NewStore()
	names, err := store.List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Println("No named schemas found.")
		fmt.Println("Schema search paths:")
		for _, p := range store.Paths() {
			fmt.Printf("  %s\n", p)
		}
		return nil
	}
	fmt.Println("Available schemas:")
	for _, n := range names {
		fmt.Printf("  - %s\n", n)
	}
	return nil
}

func printExtractUsage() {
	fmt.Print(`synesis extract - Extract structured fields from input

Usage: synesis extract [--field <name>...] [--schema <name>|--schema-file <path>] [options]

Options:
  -model string        model to use
  -temperature float   temperature (default 0.3, lower = more deterministic)
  -timeout int         timeout in seconds (default 120)
  -schema name         named schema or JSON schema file path
  -schema-file path    JSON schema file path (explicit)
  -list-schemas        list available named schemas
  -max-repair int      max repair retries on validation failure (default 2)
  -metadata            include uncertainty metadata
  -output json         output format (default json)

Examples:
  cat incident.txt | synesis extract --field service --field severity --field impact
  echo "Error in service x at 3pm" | synesis extract --field error_type --field timestamp
  synesis extract --schema-file person.json < bio.txt
  synesis extract --schema person < bio.txt
  synesis extract --list-schemas

`)
}
