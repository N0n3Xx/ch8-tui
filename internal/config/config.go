package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const defaultSystemPrompt = "You are a helpful local terminal assistant. Answer clearly, naturally, and practically. Keep responses concise unless the user asks for detail."

type Config struct {
	OllamaBaseURL       string `json:"ollama_base_url"`
	DefaultModel        string `json:"default_model"`
	DefaultSystemPrompt string `json:"default_system_prompt"`
	MaxContextMessages  int    `json:"max_context_messages"`
	MaxContextChars     int    `json:"max_context_characters"`
	StallThresholdSecs  int    `json:"stall_threshold_seconds"`
	TelemetryEnabled    bool   `json:"telemetry_enabled"`
	StoragePath         string `json:"storage_path"`
	Theme               string `json:"theme"`
}

func Load() (*Config, error) {
	root, err := configRoot()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(root, "ch8-tui")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, err
	}

	path := filepath.Join(appDir, "config.json")
	cfg := defaults(appDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, Save(cfg)
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.applyDefaults(appDir)
	return cfg, Save(cfg)
}

func Save(cfg *Config) error {
	root, err := configRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "ch8-tui", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func defaults(appDir string) *Config {
	return &Config{
		OllamaBaseURL:       "http://localhost:11434",
		DefaultSystemPrompt: defaultSystemPrompt,
		MaxContextMessages:  40,
		MaxContextChars:     24000,
		StallThresholdSecs:  10,
		TelemetryEnabled:    true,
		StoragePath:         filepath.Join(appDir, "chats"),
		Theme:               "default",
	}
}

func (c *Config) applyDefaults(appDir string) {
	if c.OllamaBaseURL == "" {
		c.OllamaBaseURL = "http://localhost:11434"
	}
	if c.DefaultSystemPrompt == "" {
		c.DefaultSystemPrompt = defaultSystemPrompt
	}
	if c.MaxContextMessages <= 0 {
		c.MaxContextMessages = 40
	}
	if c.MaxContextChars <= 0 {
		c.MaxContextChars = 24000
	}
	if c.StallThresholdSecs <= 0 {
		c.StallThresholdSecs = 10
	}
	if c.StoragePath == "" {
		c.StoragePath = filepath.Join(appDir, "chats")
	}
	if c.Theme == "" {
		c.Theme = "default"
	}
}

func configRoot() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}
