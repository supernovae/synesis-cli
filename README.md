# Synesis - Shell-friendly AI CLI

A production-grade Go CLI that connects to OpenAI-compatible APIs and works naturally from Unix shells.

## Features

- **Chat sessions** - Create and continue conversations
- **One-shot ask** - Quick prompts for scripts and automation
- **Structured extraction** - Extract fields from text into JSON
- **Summarization** - Summarize content from stdin or files
- **Commit message generation** - Generate commit messages from git diffs
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

## Usage Examples

### Basic Prompts

```bash
# Simple question
synesis ask "what is the capital of France"

# With specific model
synesis ask --model horizon "explain quantum computing"
```

### Pipelines and Stdin

```bash
# Summarize log file
cat /var/log/system.log | synesis summarize

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
```

### Session Management

```bash
# Create new chat session
synesis chat "help me write a function"

# Continue existing session
synesis chat --session my-session

# List sessions
synesis session --list

# Delete session
synesis session --delete session-id
```

### Authentication

```bash
# Configure via config file
synesis auth --set-url https://api.kybern.dev/v1
synesis auth --set-token sk-your-key

# Check auth status
synesis auth
```

### Commit Messages

```bash
# Generate commit message from diff
git diff | synesis commit-message

# Conventional commit format
git diff | synesis commit-message --conventional --notify api

# From file
synesis commit-message < /tmp/diff.txt
```

### Configuration

```bash
# Show effective configuration
synesis config

# Validate configuration
synesis config --validate

# Show sources
synesis config --sources
```

### Diagnostics

```bash
# Run full diagnostics
synesis doctor

# Verbose mode
synesis doctor -v
```

## Command Reference

### `synesis ask [options] <prompt>`
One-shot prompt/answer mode for scripting.

### `synesis chat [options]`
Start or continue an interactive chat session.

### `synesis session [options]`
Manage chat sessions (list, continue, delete, rename).

### `synesis extract --field <name> [options]`
Extract structured fields from input.

### `synesis summarize [options]`
Summarize stdin, files, or prompts.

### `synesis commit-message [options]`
Generate commit message from diff.

### `synesis models`
List available models from the API.

### `synesis config [options]`
Show or validate configuration.

### `synesis auth [options]`
Configure authentication.

### `synesis doctor`
Run diagnostics and validation.

## Output Formats

- `--output text` - Plain text (default)
- `--output json` - Valid JSON
- `--output ndjson` - Newline-delimited JSON

## Response Rendering

Control how AI responses are formatted:

- `--render plain` - Strip all markdown, plain text output (default)
- `--render markdown` - Render markdown formatting (bold, italic, code) with colors
- `--render raw` - Output raw response unchanged

```bash
# Get formatted output with markdown highlighting
synesis ask --render markdown "explain Go interfaces"

# Get plain text for scripts
synesis ask --render plain "what is 2+2"

# Get raw response for processing
synesis ask --render raw "generate JSON"
```

## Flags

| Flag | Description |
|------|-------------|
| `--quiet` | Suppress non-essential output |
| `--no-color` | Disable color output |
| `--model` | Specify model |
| `--temperature` | Set temperature (0-2) |
| `--max-tokens` | Set max tokens |
| `--timeout` | Request timeout in seconds |

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
