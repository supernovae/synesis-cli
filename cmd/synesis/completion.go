package main

import (
	"fmt"
	"os"
	"strings"
)

func runCompletion(args []string, noColor, quiet bool) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: synesis completion <bash|zsh>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Generate shell completion scripts for synesis.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  synesis completion bash > /usr/local/etc/bash_completion.d/synesis")
		fmt.Fprintln(os.Stderr, "  synesis completion zsh > /usr/local/share/zsh/site-functions/_synesis")
		return nil
	}

	shell := strings.ToLower(args[0])
	switch shell {
	case "bash":
		return generateBashCompletion()
	case "zsh":
		return generateZshCompletion()
	default:
		return fmt.Errorf("unsupported shell: %s (use bash or zsh)", shell)
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
