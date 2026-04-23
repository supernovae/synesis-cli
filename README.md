# Synesis - Shell-friendly AI CLI

A production-grade Go CLI that connects to OpenAI-compatible APIs and works naturally from Unix shells. Ships with 20+ built-in commands for git workflows, code review, interactive chat, and scripted automation.

## Features

- **Interactive chat** - REPL mode and named chat sessions with history
- **One-shot ask** - Quick prompts for scripts, pipelines, and automation
- **Structured extraction** - Extract fields from text into JSON
- **Summarization** - Summarize content from stdin, files, or prompts
- **Code review** - Review staged, unstaged, or branch diffs with AI feedback
- **Commit message generation** - Generate commit messages from git diffs
- **PR summary** - Summarize commits between two refs as a grouped changelog
- **Release notes** - Generate release notes from git history (tag-aware)
- **Explain commit** - Explain a commit or series of commits (the "why")
- **Configuration profiles** - Multiple named profiles with keychain-backed credentials
- **Prompt templates** - Store, reuse, and parameterize prompts
- **Shell completion** - Bash, Zsh, and Fish completion scripts
- **File watching** - Watch files and run prompts on change
- **Model listing** - List available models from the API
- **Diagnostics** - Validate configuration and API connectivity

## Installation

```bash
# From source
make build
sudo cp bin/synesis /usr/local/bin/

# Or use go install
go install synesis.sh/synesis/cmd/synesis@latest
```

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SYNESIS_BASE_URL` | API endpoint URL |
| `SYNESIS_API_KEY` | Your API key |
| `SYNESIS_MODEL` | Default model |
| `SYNESIS_TIMEOUT` | Request timeout in seconds |

### Config File

Create `~/.config/synesis/config.yaml`:

```yaml
base_url: https://api.kybern.dev/v1
api_key: sk-your-key-here
model: auto
timeout: 120
```

### Configuration Profiles

Multiple named profiles for different API providers or use cases:

```bash
# Create a profile for a different provider
synesis profile create local --base-url http://localhost:11434/v1 --model llama2

# Switch between profiles
synesis --profile local ask "hello"

# List profiles
synesis profile list

# Show profile details
synesis profile show local

# Delete a profile
synesis profile delete local
```

API keys are stored securely via the OS keychain (macOS Keychain, Linux libsecret).

## Usage Examples

### Interactive Chat

```bash
# Interactive REPL (TTY required)
synesis repl

# Named chat session
synesis chat "help me write a function"
synesis chat --session my-session   # continue existing session

# Session management
synesis session --list
synesis session --delete session-id
synesis session --rename old-name new-name
synesis session --export session-id --output chat.md
```

### Basic Prompts

```bash
# Simple question
synesis ask "what is the capital of France"

# With specific model
synesis ask --model horizon "explain quantum computing"

# Read prompt from clipboard (macOS: pbpaste)
synesis ask --from-clipboard "summarize this"

# Copy last response to clipboard (macOS: pbcopy)
synesis ask "generate a regex" --copy-last
```

### Pipelines and Stdin

```bash
# Summarize log file
cat /var/log/system.log | synesis summarize

# Short summary (1-2 sentences)
cat article.txt | synesis summarize --short

# Extract structured data
cat incident.txt | synesis extract --field service --field severity --field impact

# Ask about git diff
git diff | synesis ask "summarize these changes"

# Find anomalies in kubectl output
kubectl get pods -A | synesis ask "find anomalies"
```

### Structured JSON Output

```bash
# Get JSON output for scripts
synesis ask --output json "list 3 colors" | jq '.content'

# Extract fields to JSON
echo "Error in service auth at 3pm" | synesis extract --field error_type --field timestamp --field service

# jq-style field selection on raw response
synesis ask "what is the weather" --jq '.choices[0].message.content'

# Dot-notation JSON extraction
synesis ask --output json "pick a color" --extract-path choices.0.message.content
```

### Bundles (Reusable Prompt Packages)

Bundles are YAML files that package a system prompt, user prompt template, model, and settings together:

```yaml
# mybundle.yaml
system: "You are a security expert"
prompt: "Review this code for vulnerabilities:\n{{ .Input }}"
model: gpt-4o
temperature: 0.3
max_tokens: 2000
```

```bash
synesis ask --bundle mybundle.yaml < vulnerable_code.go
```

### Tool Use (Function Calling)

Define tools in a JSON file and let the model call them:

```bash
synesis ask --tools functions.json --tool-choice required "get the current weather in Tokyo"
```

### Prompt Templates

Store and reuse prompts with variable substitution:

```bash
# Create a template
synesis template create review-pr \
  --system "You are a code reviewer" \
  --description "Review a pull request" \
  --user "Review these changes:\n{{ .Input }}"

# Run it
git diff | synesis template run review-pr

# List templates
synesis template list

# Delete
synesis template delete review-pr
```

### Git Workflows

#### Code Review

```bash
# Review staged changes
git diff --cached | synesis review

# Review since a tag
synesis review --since v1.2.0

# Review branch vs main
synesis review --branch main --conventional

# Dry run (show the request without calling the API)
synesis review --dry-run

# Review with token usage stats
synesis review --usage
```

#### Commit Messages

```bash
# Generate commit message from diff
git diff | synesis commit-message

# Conventional commit format
git diff | synesis commit-message --conventional --notify api

# From file
synesis commit-message < /tmp/diff.txt

# With usage stats
git diff | synesis commit-message --usage
```

#### PR Summary

Summarize commits between two refs as a grouped changelog:

```bash
# Between main and HEAD
synesis pr-summary

# Between tags
synesis pr-summary --base v1.0.0 --head v1.1.0

# Brief single-paragraph format
git log main..HEAD --oneline | synesis pr-summary --format brief

# Dry run
synesis pr-summary --dry-run
```

#### Release Notes

```bash
# For a tag (auto-detects previous tag)
synesis release-notes --tag v1.2.0

# Between two tags
synesis release-notes --from v1.0.0 --to v1.1.0

# Include commit SHAs
synesis release-notes --tag v2.0.0 --include-commits

# Markdown output (default)
synesis release-notes --tag v1.2.0 --output md
```

#### Explain a Commit

Understand what a commit does and why it was made:

```bash
# Explain HEAD
synesis explain-commit HEAD

# Explain by SHA
synesis explain-commit abc1234

# Multiple commits
synesis explain-commit HEAD~3 HEAD~2 HEAD~1

# With git show --stat
synesis explain-commit HEAD --stat

# Exclude the diff (faster, less context)
synesis explain-commit HEAD --no-diff
```

### File Watching

Watch files for changes and run prompts automatically:

```bash
# Watch a file and print events
synesis watch output.log

# With interval (default watches with inotify/fsnotify)
synesis watch --interval 5s output.log
```

### Editor Integration

Open a file in your `$EDITOR`, edit it, and save:

```bash
synesis editor /path/to/file.txt
```

### System Presets

List available system presets:

```bash
synesis presets
```

### Shell Completion

```bash
# Bash
synesis completion bash > /usr/local/etc/bash_completion.d/synesis

# Zsh
synesis completion zsh > /usr/local/share/zsh/site-functions/_synesis

# Fish
synesis completion fish > ~/.config/fish/completions/synesis.fish
```

### Authentication

```bash
# Configure via config file
synesis auth --set-url https://api.kybern.dev/v1
synesis auth --set-token sk-your-key

# Check auth status
synesis auth
```

### Configuration Management

```bash
# Show effective configuration
synesis config

# Validate configuration
synesis config --validate

# Show configuration sources
synesis config --sources
```

### Diagnostics

```bash
# Run full diagnostics
synesis doctor

# Verbose mode
synesis doctor -v

# Auto-fix issues
synesis doctor --fix
```

## Output File Modes

Write or append output to files without piping:

```bash
synesis ask "generate docs" --write-output README.md
synesis ask "log this" --append-output /var/log/synesis.log
```

## Command Reference

### `synesis ask [options] <prompt>`
One-shot prompt/answer mode for scripting. Supports stdin, clipboard, jq filtering, bundles, and file output.

### `synesis chat [options]`
Start or continue an interactive chat session.

### `synesis repl [options]`
Start an interactive REPL session (requires a TTY). Supports `/exit`, `/clear`, `/model`, `/session`.

### `synesis session [options]`
Manage chat sessions (list, continue, delete, rename, export, import).

### `synesis extract --field <name> [options]`
Extract structured fields from input into JSON.

### `synesis summarize [options]`
Summarize stdin, files, or prompts. Use `--short` for 1-2 sentence summaries.

### `synesis commit-message [options]`
Generate commit message from diff.

### `synesis review [options]`
Review code changes. Auto-detects staged, unstaged, or branch diffs.

### `synesis pr-summary [options]`
Summarize commits between two refs as a grouped changelog.

### `synesis release-notes [options]`
Generate release notes from git history. Tag-aware with `--tag`, `--from`, `--to`.

### `synesis explain-commit [options] [ref...]`
Explain a commit or series of commits — what changed and why.

### `synesis models`
List available models from the API.

### `synesis config [options]`
Show or validate configuration.

### `synesis auth [options]`
Configure authentication.

### `synesis profile <subcommand>`
Manage configuration profiles (list, show, create, delete).

### `synesis template <subcommand>`
Manage prompt templates (list, show, create, delete, run).

### `synesis presets`
List available system presets.

### `synesis editor <file>`
Edit a file in `$EDITOR`.

### `synesis watch <paths...>`
Watch files for changes and report events.

### `synesis completion <bash|zsh|fish>`
Generate shell completion scripts.

### `synesis doctor`
Run diagnostics and validation.

## Output Formats

- `--output text` - Plain text (default)
- `--output json` - Valid JSON
- `--output ndjson` - Newline-delimited JSON

## Response Rendering

Control how AI responses are formatted:

- `--render plain` - Strip all markdown, plain text output (default for most commands)
- `--render markdown` - Render markdown formatting with colors
- `--render raw` - Output raw response unchanged

## Global Flags

| Flag | Description |
|------|-------------|
| `--profile string` | Use named configuration profile |
| `--quiet` | Suppress non-essential output |
| `--no-color` | Disable color output |
| `--model string` | Specify model |
| `--temperature float` | Set temperature (0–2) |
| `--max-tokens int` | Set max tokens |
| `--timeout int` | Request timeout in seconds |

## Shared Command Flags

These flags are available across most API-calling commands:

| Flag | Description |
|------|-------------|
| `--dry-run` | Show the request that would be sent without calling the API |
| `--usage` | Show token usage (prompt/completion/total) and latency after response |
| `--no-stream` | Disable streaming, wait for full response |
| `--output text\|json\|ndjson\|md` | Output format |
| `--render plain\|markdown\|raw` | Response rendering mode |
| `--model string` | Override the model |
| `--temperature float` | Override temperature |
| `--timeout int` | Override request timeout |

## Exit Codes

- `0` - Success
- `1` - Usage/general failure
- `2` - Configuration/auth error
- `3` - API error
- `4` - Timeout/interrupted
- `5` - Parse/serialization error

## Development

```bash
# Build
make build

# Test
make test

# Lint
make lint

# Run with Docker
docker build -t synesis .
docker run --rm -it synesis ask "hello"
```

## License

SPDX-License-Identifier: Apache-2.0
Copyright (c) 2025 The synesis Authors
