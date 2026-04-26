# Ollama TUI

A terminal-only chat client for local Ollama models. It is built with Go, Bubble Tea, Bubbles, and Lip Gloss. There is no browser UI, Electron shell, cloud API, or auth layer.

## Requirements

- Go 1.22+
- Ollama running locally
- At least one installed model

```sh
ollama serve
ollama pull qwen2.5:7b
```

## Run

```sh
go mod tidy
go run ./cmd/ollama-tui
```

Build a binary:

```sh
go build -o ollama-tui ./cmd/ollama-tui
./ollama-tui
```

## Features

- Terminal-native TUI with bordered panels
- Dynamic Ollama model selector
- Streaming chat responses
- Cancel generation with `Ctrl+C`
- Regenerate last assistant response
- Edit last user message
- Persistent JSON chat history
- Chat browser with filtering and delete confirmation
- Toggleable telemetry panel
- Stalled response detection

## Shortcuts

| Shortcut | Action |
| --- | --- |
| `Enter` | Send message |
| `Alt+Enter` / `Shift+Enter` | Insert newline when supported by terminal |
| `Ctrl+M` | Model selector |
| `Ctrl+O` | Chat browser |
| `Ctrl+T` | Toggle telemetry |
| `Ctrl+N` | New chat |
| `Ctrl+S` | Save chat |
| `Ctrl+R` | Regenerate last response |
| `Ctrl+E` | Edit last user message |
| `Ctrl+C` | Cancel generation, or quit when idle |
| `Esc` | Close modal |

Model selector and chat browser support `up`/`down` or `j`/`k` navigation.

## Config And Storage

Config is created at:

```text
~/.config/ollama-tui/config.json
```

Important fields:

```json
{
  "ollama_base_url": "http://localhost:11434",
  "default_model": "",
  "default_system_prompt": "You are a helpful local terminal assistant. Answer clearly, naturally, and practically. Keep responses concise unless the user asks for detail.",
  "max_context_messages": 40,
  "max_context_characters": 24000,
  "stall_threshold_seconds": 10,
  "telemetry_enabled": true,
  "storage_path": "~/.config/ollama-tui/chats",
  "theme": "default"
}
```

Chats are stored as JSON files under `storage_path`.

