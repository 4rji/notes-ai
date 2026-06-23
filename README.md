# gov-notes

A CLI tool that reads your notes (`.md`, `.txt`, `.json`) from the current directory — recursively — and lets you ask questions about them using AI (Anthropic or OpenAI). Run it from any folder and it will use that folder as its context.

---

## Build

Requirements: [Go 1.22+](https://go.dev/dl/)

```bash
git clone https://github.com/4rji/gov-notes
cd gov-notes
go build -o gov-notes .
```

Move the binary to your PATH:

```bash
mv gov-notes /usr/local/bin/
```

Cross-compile for other platforms:

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o gov-notes-linux .

# Windows
GOOS=windows GOARCH=amd64 go build -o gov-notes.exe .
```

---

## Configuration

Add your API key to `~/.zshrc` (or `~/.bashrc`):

```bash
# Anthropic (default)
export ANTHROPIC_API_KEY="sk-ant-..."

# Or OpenAI
export OPENAI_API_KEY="sk-..."
export GOV_NOTES_PROVIDER="openai"
```

Then reload your shell:

```bash
source ~/.zshrc
```

If no key is found, the program prints setup instructions and exits.

### All environment variables

| Variable | Description | Default |
|---|---|---|
| `ANTHROPIC_API_KEY` | Anthropic API key | — |
| `OPENAI_API_KEY` | OpenAI API key | — |
| `GOV_NOTES_PROVIDER` | `anthropic` or `openai` | `anthropic` |
| `GOV_NOTES_MODEL` | Model name | `claude-sonnet-4-6` / `gpt-4o` |
| `GOV_NOTES_SYSTEM` | Custom system prompt | built-in default |

---

## Usage

```bash
# Interactive chat mode
cd ~/my-notes
gov-notes

# Ask a single question and exit
gov-notes ask "what projects do I have pending?"

# Override provider or model at runtime
gov-notes --provider openai
gov-notes --model claude-opus-4-8 ask "summarize my tasks"
```

### Interactive mode commands

| Command | Description |
|---|---|
| `/info` | List loaded files |
| `/reload` | Reload files from disk |
| `/limpiar` | Clear conversation history |
| `/salir` | Exit |
| `Ctrl+C` | Exit |

---

## How it works

1. On startup, it walks the current directory recursively and loads all `.md`, `.txt`, and `.json` files (up to 500 KB each).
2. The file contents are passed as context to the AI model.
3. You ask questions; it answers based on your notes.
4. In interactive mode, conversation history is maintained across turns (last 20 exchanges).

Ignored directories: `.git`, `node_modules`, `vendor`, `.venv`, `__pycache__`, `.cache`.

---

## Project structure

```
gov-notes/
├── main.go          # CLI entry point, flags, subcommands
├── config/
│   └── config.go    # Env var loading, setup instructions
├── reader/
│   └── reader.go    # Recursive file walker
├── ai/
│   └── client.go    # HTTP clients for Anthropic and OpenAI
├── chat/
│   └── chat.go      # Interactive loop and single-question mode
└── go.mod
```

No external SDKs — only one dependency (`readline`) for the interactive prompt. API calls are made directly over HTTP.
