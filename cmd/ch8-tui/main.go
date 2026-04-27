package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ch8-tui/internal/config"
	"ch8-tui/internal/ollama"
	"ch8-tui/internal/storage"
	"ch8-tui/internal/tui"
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
