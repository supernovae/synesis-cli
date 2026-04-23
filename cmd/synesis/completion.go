package main

import (
	"fmt"
	"os"
	"strings"
)

func runCompletion(args []string, noColor, quiet bool) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: synesis completion <bash|zsh|fish>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Generate shell completion scripts for synesis.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  synesis completion bash > /usr/local/etc/bash_completion.d/synesis")
		fmt.Fprintln(os.Stderr, "  synesis completion zsh > /usr/local/share/zsh/site-functions/_synesis")
		fmt.Fprintln(os.Stderr, "  synesis completion fish > ~/.config/fish/completions/synesis.fish")
		return nil
	}

	shell := strings.ToLower(args[0])
	switch shell {
	case "bash":
		return generateBashCompletion()
	case "zsh":
		return generateZshCompletion()
	case "fish":
		return generateFishCompletion()
	default:
		return fmt.Errorf("unsupported shell: %s (use bash, zsh, or fish)", shell)
	}
}

func generateBashCompletion() error {
	// Generate bash completion script
	script := `#!/bin/bash
_synesis_completion() {
    local cur prev words cword
    _get_comp_words_by_ref -n : cur prev words cword

    local commands="chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help"

    case $cword in
        1)
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            ;;
        2)
            case "$prev" in
                chat|ask|summarize)
                    COMPREPLY=($(compgen -W "--model --temperature --max-tokens --system --timeout --output --render --dry-run --usage --bundle --from-clipboard --copy-last" -- "$cur"))
                    ;;
                session)
                    COMPREPLY=($(compgen -W "list show delete rename export import" -- "$cur"))
                    ;;
                template)
                    COMPREPLY=($(compgen -W "list show create delete run" -- "$cur"))
                    ;;
                profile)
                    COMPREPLY=($(compgen -W "list show create delete" -- "$cur"))
                    ;;
                presets)
                    COMPREPLY=($(compgen -W "list" -- "$cur"))
                    ;;
                doctor)
                    COMPREPLY=($(compgen -W "--v --fix" -- "$cur"))
                    ;;
                watch)
                    COMPREPLY=($(compgen -W "--interval --help" -- "$cur"))
                    ;;
            esac
            ;;
    esac
}

complete -F _synesis_completion synesis
`
	fmt.Print(script)
	return nil
}

func generateZshCompletion() error {
	// Generate zsh completion script
	script := `#compdef synesis

_synesis_completion() {
    local -a commands
    commands=(
        'chat:Start an interactive chat session'
        'ask:One-shot prompt/answer mode'
        'session:Manage chat sessions'
        'models:List available models'
        'config:Show configuration'
        'auth:Configure authentication'
        'extract:Extract structured fields from input'
        'summarize:Summarize stdin, files, or prompt'
        'commit-message:Generate commit message from diff'
        'doctor:Run diagnostics'
        'profile:Manage configuration profiles'
        'template:Manage prompt templates'
        'repl:Interactive REPL mode'
        'presets:List available system presets'
        'editor:Edit content in $EDITOR'
        'watch:Watch files for changes'
        'help:Show help'
    )

    _arguments -C \
        '1:command:->command' \
        '*::options:->options'

    case $state in
        command)
            _describe "command" commands
            ;;
        options)
            case $words[1] in
                chat|ask|summarize)
                    _arguments \
                        '--model[model]:model:' \
                        '--temperature[temperature]:float:' \
                        '--max-tokens[max tokens]:int:' \
                        '--system[system prompt]:string:' \
                        '--timeout[timeout]:int:' \
                        '--output[output format]:format:(text json ndjson)' \
                        '--render[render mode]:mode:(plain markdown raw)' \
                        '--dry-run[dry run]' \
                        '--usage[show usage]' \
                        '--bundle[bundle file]:file:' \
                        '--from-clipboard[from clipboard]' \
                        '--copy-last[copy last response]'
                    ;;
                session)
                    _arguments \
                        '1:subcommand:(list show delete rename export import)' \
                        '*::options:--json --format --output'
                    ;;
                template)
                    _arguments \
                        '1:subcommand:(list show create delete run)' \
                        '*::options:--description --system --user --vars --file --force'
                    ;;
                profile)
                    _arguments \
                        '1:subcommand:(list show create delete)' \
                        '*::options:--base-url --api-key --model --timeout --org-id --endpoint --default --force'
                    ;;
            esac
            ;;
    esac
}

_synesis_completion "$@"
`
	fmt.Print(script)
	return nil
}

func generateFishCompletion() error {
	// Generate fish completion script
	script := `complete -c synesis -f

# Commands
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "chat" -d "Start an interactive chat session"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "ask" -d "One-shot prompt/answer mode"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "session" -d "Manage chat sessions"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "models" -d "List available models"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "config" -d "Show configuration"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "auth" -d "Configure authentication"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "extract" -d "Extract structured fields from input"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "summarize" -d "Summarize stdin, files, or prompt"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "commit-message" -d "Generate commit message from diff"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "doctor" -d "Run diagnostics"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "profile" -d "Manage configuration profiles"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "template" -d "Manage prompt templates"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "repl" -d "Interactive REPL mode"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "presets" -d "List available system presets"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "editor" -d "Edit content in $EDITOR"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "watch" -d "Watch files for changes"
complete -c synesis -n "not __fish_seen_subcommand_from chat ask session models config auth extract summarize commit-message doctor profile template repl presets editor watch help" -a "help" -d "Show help"

# Flags for chat/ask/summarize
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l model -d "model"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l temperature -d "temperature"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l max-tokens -d "max tokens"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l system -d "system prompt"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l timeout -d "timeout"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l output -d "output format" -a "text json ndjson"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l render -d "render mode" -a "plain markdown raw"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l dry-run -d "dry run"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l usage -d "show usage"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l bundle -d "bundle file"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l from-clipboard -d "from clipboard"
complete -c synesis -n "__fish_seen_subcommand_from chat ask summarize" -l copy-last -d "copy last response"

# Session subcommands
complete -c synesis -n "__fish_seen_subcommand_from session; and not __fish_seen_subcommand_from list show delete rename export import" -a "list" -d "List sessions"
complete -c synesis -n "__fish_seen_subcommand_from session; and not __fish_seen_subcommand_from list show delete rename export import" -a "show" -d "Show session"
complete -c synesis -n "__fish_seen_subcommand_from session; and not __fish_seen_subcommand_from list show delete rename export import" -a "delete" -d "Delete session"
complete -c synesis -n "__fish_seen_subcommand_from session; and not __fish_seen_subcommand_from list show delete rename export import" -a "rename" -d "Rename session"
complete -c synesis -n "__fish_seen_subcommand_from session; and not __fish_seen_subcommand_from list show delete rename export import" -a "export" -d "Export session"
complete -c synesis -n "__fish_seen_subcommand_from session; and not __fish_seen_subcommand_from list show delete rename export import" -a "import" -d "Import session"

# Template subcommands
complete -c synesis -n "__fish_seen_subcommand_from template; and not __fish_seen_subcommand_from list show create delete run" -a "list" -d "List templates"
complete -c synesis -n "__fish_seen_subcommand_from template; and not __fish_seen_subcommand_from list show create delete run" -a "show" -d "Show template"
complete -c synesis -n "__fish_seen_subcommand_from template; and not __fish_seen_subcommand_from list show create delete run" -a "create" -d "Create template"
complete -c synesis -n "__fish_seen_subcommand_from template; and not __fish_seen_subcommand_from list show create delete run" -a "delete" -d "Delete template"
complete -c synesis -n "__fish_seen_subcommand_from template; and not __fish_seen_subcommand_from list show create delete run" -a "run" -d "Run template"

# Profile subcommands
complete -c synesis -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list show create delete" -a "list" -d "List profiles"
complete -c synesis -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list show create delete" -a "show" -d "Show profile"
complete -c synesis -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list show create delete" -a "create" -d "Create profile"
complete -c synesis -n "__fish_seen_subcommand_from profile; and not __fish_seen_subcommand_from list show create delete" -a "delete" -d "Delete profile"

# Doctor flags
complete -c synesis -n "__fish_seen_subcommand_from doctor" -l v -d "verbose"
complete -c synesis -n "__fish_seen_subcommand_from doctor" -l fix -d "fix issues"

# Watch flags
complete -c synesis -n "__fish_seen_subcommand_from watch" -l interval -d "interval"
complete -c synesis -n "__fish_seen_subcommand_from watch" -l help -d "help"
`
	fmt.Print(script)
	return nil
}
