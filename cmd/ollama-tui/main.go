package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ollama-tui/internal/config"
	"ollama-tui/internal/ollama"
	"ollama-tui/internal/storage"
	"ollama-tui/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	store, err := storage.New(cfg.StoragePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "storage error: %v\n", err)
		os.Exit(1)
	}

	app := tui.New(cfg, store, ollama.NewClient(cfg.OllamaBaseURL))
	if _, err := tea.NewProgram(app, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}
