# Ch8 TUI

A terminal-only chat client for local Ollama models. It is built with Go, Bubble Tea, Bubbles, and Lip Gloss. There is no browser UI, Electron shell, cloud API, or auth layer.

## Requirements

- Go 1.22+
- Ollama running locally
- At least one installed model

```sh
ollama serve
ollama pull qwen2.5:7b
```

## Install

Install the `ch8-tui` command into your Go binary directory:

```sh
go install github.com/N0n3Xx/ch8-tui/cmd/ch8-tui@latest
```

Make sure your Go binary directory is on `PATH`:

```sh
export PATH="$HOME/go/bin:$PATH"
```

Then run:

```sh
ch8-tui
```

To use a shorter command such as `ch8`, build or symlink it under that name:

```sh
go build -o "$HOME/go/bin/ch8" ./cmd/ch8-tui
ch8
```

## Development

```sh
go mod tidy
go run ./cmd/ch8-tui
```

Build a binary:

```sh
go build -o ch8-tui ./cmd/ch8-tui
./ch8-tui
```

Run the test suite:

```sh
go test ./...
go test -race ./...
go vet ./...
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
- Terminal control sequence stripping for displayed model and chat content

## Shortcuts

| Shortcut | Action |
| --- | --- |
| `Enter` | Send message, or open model selector when input is empty |
| `Alt+Enter` / `Shift+Enter` | Insert newline when supported by terminal |
| `F2` / `Alt+M` / `Ctrl+M` | Model selector |
| `F3` / `Alt+O` / `Ctrl+O` | Chat browser |
| `F4` / `Alt+T` / `Ctrl+T` | Toggle telemetry |
| `Ctrl+N` | New chat |
| `Ctrl+S` | Save chat |
| `Ctrl+R` | Regenerate last response |
| `Ctrl+E` | Edit last user message |
| `End` / `Ctrl+J` | Jump to latest chat message |
| `Ctrl+C` | Cancel generation, or quit when idle |
| `Esc` | Close modal |

Model selector and chat browser support `up`/`down` or `j`/`k` navigation.

Some terminals reserve or encode `Ctrl+M` as `Enter`. Use `F2` or `Alt+M` for the model selector if `Ctrl+M` is intercepted by your terminal.

## Configuration

Config is created at:

```text
~/.config/ch8-tui/config.json
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
  "storage_path": "~/.config/ch8-tui/chats",
  "theme": "default"
}
```

`ollama_base_url` should point to the Ollama HTTP API. The default is local-only:

```text
http://localhost:11434
```

`default_model` is updated when you select a model in the model selector. `storage_path` defaults to the `chats` directory inside the app config directory.

## Storage

Chats are stored as local JSON files under `storage_path`. Chat filenames use app-generated safe IDs, and loaded chat IDs are normalized to their filenames before load, save, or delete operations.

Existing chats from an earlier `ollama-tui` build are not migrated automatically. To move them manually:

```sh
mkdir -p ~/.config/ch8-tui
cp -R ~/.config/ollama-tui/chats ~/.config/ch8-tui/
```

## Privacy And Security

- Ch8 TUI talks only to the configured Ollama API endpoint.
- Prompts, responses, selected model names, and response telemetry are stored locally in JSON chat files.
- There is no built-in cloud API, browser UI, authentication layer, or remote sync.
- Treat `ollama_base_url` as trusted configuration. Pointing it at a remote server sends your prompts and chat context to that server.
- Terminal escape/control sequences from model output, saved chats, model names, and error text are stripped before display.
- Chat load/delete/save paths reject unsafe chat IDs to prevent path traversal through crafted chat files.
