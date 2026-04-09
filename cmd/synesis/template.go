package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"synesis.sh/synesis/pkg/template"
	"synesis.sh/synesis/pkg/ui"
)

// runTemplate implements the template command
func runTemplate(args []string, noColor, quiet bool, profileName string) error {
	if len(args) == 0 {
		printTemplateUsage()
		return nil
	}

	subcmd := strings.ToLower(args[0])
	subArgs := args[1:]

	switch subcmd {
	case "list":
		return runTemplateList(noColor, quiet)
	case "show":
		return runTemplateShow(subArgs, noColor, quiet)
	case "create":
		return runTemplateCreate(subArgs, noColor, quiet)
	case "delete":
		return runTemplateDelete(subArgs, noColor, quiet)
	case "run":
		return runTemplateRun(subArgs, noColor, quiet, profileName)
	default:
		return fmt.Errorf("unknown template subcommand: %s", subcmd)
	}
}

func runTemplateList(noColor, quiet bool) error {
	store, err := template.NewStore(template.DefaultDir())
	if err != nil {
		return fmt.Errorf("template store error: %w", err)
	}

	templates, err := store.List()
	if err != nil {
		return fmt.Errorf("list templates: %w", err)
	}

	if len(templates) == 0 {
		if !quiet {
			fmt.Println("No templates configured.")
			fmt.Println("Use 'synesis template create <name>' to add a template.")
		}
		return nil
	}

	fmt.Println("Available templates:")
	for _, t := range templates {
		desc := t.Description
		if desc == "" {
			desc = "(no description)"
		}
		varInfo := ""
		if len(t.Variables) > 0 {
			varInfo = fmt.Sprintf(" [%s]", strings.Join(t.Variables, ", "))
		}
		fmt.Printf("  %s - %s%s\n", t.Name, desc, varInfo)
	}

	return nil
}

func runTemplateShow(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("template show", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("template name required")
	}

	templateName := fs.Arg(0)

	store, err := template.NewStore(template.DefaultDir())
	if err != nil {
		return fmt.Errorf("template store error: %w", err)
	}

	t, err := store.Get(templateName)
	if err != nil {
		return err
	}

	fmt.Printf("Template: %s\n", t.Name)
	fmt.Println(strings.Repeat("=", 40))
	if t.Description != "" {
		fmt.Printf("Description: %s\n", t.Description)
	}
	if len(t.Variables) > 0 {
		fmt.Printf("Variables: %s\n", strings.Join(t.Variables, ", "))
	}
	if t.SystemPrompt != "" {
		fmt.Printf("\nSystem Prompt:\n%s\n", t.SystemPrompt)
	}
	fmt.Printf("\nUser Prompt:\n%s\n", t.UserPrompt)

	return nil
}

func runTemplateCreate(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("template create", flag.ExitOnError)
	description := fs.String("description", "", "template description")
	system := fs.String("system", "", "system prompt")
	user := fs.String("user", "", "user prompt (required)")
	vars := fs.String("vars", "", "comma-separated list of variables")
	templateFile := fs.String("file", "", "load template from file")
	force := fs.Bool("force", false, "overwrite existing template")

	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("template name required")
	}

	templateName := fs.Arg(0)

	// Load from file if specified
	if *templateFile != "" {
		t, err := template.LoadFromFile(*templateFile)
		if err != nil {
			return fmt.Errorf("load template file: %w", err)
		}
		t.Name = templateName

		store, err := template.NewStore(template.DefaultDir())
		if err != nil {
			return fmt.Errorf("template store error: %w", err)
		}

		if store.Exists(templateName) && !*force {
			return fmt.Errorf("template '%s' already exists (use --force to overwrite)", templateName)
		}

		if err := store.Save(t); err != nil {
			return fmt.Errorf("save template: %w", err)
		}

		if !quiet {
			fmt.Printf("Template '%s' created from file.\n", templateName)
		}
		return nil
	}

	if *user == "" {
		return fmt.Errorf("user prompt required (use --user or interactive mode)")
	}

	// Parse variables
	var varList []string
	if *vars != "" {
		for _, v := range strings.Split(*vars, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				varList = append(varList, v)
			}
		}
	}

	t := &template.Template{
		Name:         templateName,
		Description:  *description,
		SystemPrompt: *system,
		UserPrompt:   *user,
		Variables:    varList,
	}

	store, err := template.NewStore(template.DefaultDir())
	if err != nil {
		return fmt.Errorf("template store error: %w", err)
	}

	if store.Exists(templateName) && !*force {
		return fmt.Errorf("template '%s' already exists (use --force to overwrite)", templateName)
	}

	if err := store.Save(t); err != nil {
		return fmt.Errorf("save template: %w", err)
	}

	if !quiet {
		fmt.Printf("Template '%s' created successfully.\n", templateName)
	}

	return nil
}

func runTemplateCreateInteractive(templateName string, quiet bool) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Creating template:", templateName)
	fmt.Println("Enter template details (empty line to skip optional fields)")
	fmt.Println()

	// Description
	fmt.Print("Description (optional): ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	// System prompt
	fmt.Print("System prompt (optional, end with empty line): ")
	var systemPrompt strings.Builder
	for {
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if systemPrompt.Len() > 0 {
			systemPrompt.WriteString("\n")
		}
		systemPrompt.WriteString(line)
	}

	// User prompt (required)
	fmt.Println("User prompt (required, end with two empty lines):")
	var userPrompt strings.Builder
	emptyLines := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.TrimSpace(line) == "" {
			emptyLines++
			if emptyLines >= 2 {
				break
			}
			continue
		}
		emptyLines = 0
		if userPrompt.Len() > 0 {
			userPrompt.WriteString("\n")
		}
		userPrompt.WriteString(strings.TrimSpace(line))
	}

	// Variables
	fmt.Print("Variables (comma-separated, optional): ")
	varsLine, _ := reader.ReadString('\n')
	var varList []string
	for _, v := range strings.Split(strings.TrimSpace(varsLine), ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			varList = append(varList, v)
		}
	}

	t := &template.Template{
		Name:         templateName,
		Description:  description,
		SystemPrompt: systemPrompt.String(),
		UserPrompt:   userPrompt.String(),
		Variables:    varList,
	}

	store, err := template.NewStore(template.DefaultDir())
	if err != nil {
		return fmt.Errorf("template store error: %w", err)
	}

	if store.Exists(templateName) {
		return fmt.Errorf("template '%s' already exists", templateName)
	}

	if err := store.Save(t); err != nil {
		return fmt.Errorf("save template: %w", err)
	}

	if !quiet {
		fmt.Printf("Template '%s' created successfully.\n", templateName)
	}

	return nil
}

func runTemplateDelete(args []string, noColor, quiet bool) error {
	fs := flag.NewFlagSet("template delete", flag.ExitOnError)
	force := fs.Bool("force", false, "skip confirmation")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("template name required")
	}

	templateName := fs.Arg(0)

	store, err := template.NewStore(template.DefaultDir())
	if err != nil {
		return fmt.Errorf("template store error: %w", err)
	}

	if !store.Exists(templateName) {
		return fmt.Errorf("template not found: %s", templateName)
	}

	// Confirm deletion
	if !*force && ui.IsTerminal() {
		fmt.Printf("Are you sure you want to delete template '%s'? [y/N] ", templateName)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}

	if err := store.Delete(templateName); err != nil {
		return fmt.Errorf("delete template: %w", err)
	}

	if !quiet {
		fmt.Printf("Template '%s' deleted.\n", templateName)
	}

	return nil
}

func runTemplateRun(args []string, noColor, quiet bool, profileName string) error {
	fs := flag.NewFlagSet("template run", flag.ExitOnError)
	templateFile := fs.String("template-file", "", "load one-off template from file")
	output := fs.String("output", "text", "output format: text, json")
	var vars []string
	fs.Var(&stringSliceValue{&vars}, "var", "variable assignment (key=value, can repeat)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		return fmt.Errorf("template name required")
	}

	templateName := fs.Arg(0)

	// Load template
	var t *template.Template
	var err error

	if *templateFile != "" {
		t, err = template.LoadFromFile(*templateFile)
		if err != nil {
			return fmt.Errorf("load template file: %w", err)
		}
	} else {
		store, err := template.NewStore(template.DefaultDir())
		if err != nil {
			return fmt.Errorf("template store error: %w", err)
		}
		t, err = store.Get(templateName)
		if err != nil {
			return err
		}
	}

	// Parse variables
	varMap := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid variable format: %s (use key=value)", v)
		}
		varMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	// Render template
	rendered, err := t.Render(varMap)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	// Output
	if *output == "json" {
		fmt.Printf(`{"system_prompt": %q, "user_prompt": %q}`+"\n",
			escapeJSON(rendered.SystemPrompt), escapeJSON(rendered.UserPrompt))
	} else {
		if rendered.SystemPrompt != "" {
			fmt.Printf("System: %s\n\n", rendered.SystemPrompt)
		}
		fmt.Printf("User: %s\n", rendered.UserPrompt)
	}

	return nil
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

func printTemplateUsage() {
	fmt.Print(`synesis template - Manage prompt templates

Usage: synesis template <subcommand> [options]

Subcommands:
  list              List all templates
  show <name>       Show template details
  create <name>     Create a new template
  delete <name>     Delete a template
  run <name>        Run a template with variables

Options for 'create':
  -description string  Template description
  -system string       System prompt
  -user string         User prompt (required)
  -vars string         Comma-separated list of variables
  -file string         Load template from YAML file
  -force               Overwrite existing template

Options for 'run':
  -var key=value       Variable assignment (can repeat)
  -template-file string  Load one-off template from file
  -output text|json    Output format

Examples:
  synesis template create reviewer --description "Code review template" \
    --system "You are a code reviewer" --user "Review {{.file}} for {{.focus}}" \
    --vars "file,focus"

  synesis template run reviewer --var file=main.go --var focus=security

  synesis template run --template-file /path/to/template.yaml --var name=value

`)
}
