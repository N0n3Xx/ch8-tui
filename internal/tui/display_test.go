package tui

import (
	"strings"
	"testing"

	"github.com/N0n3Xx/ch8-tui/internal/config"
	"github.com/N0n3Xx/ch8-tui/internal/storage"
)

func TestSafeDisplayTextStripsTerminalControlSequences(t *testing.T) {
	input := "ok\x1b[31m red\x1b[0m \x1b]52;c;dGVzdA==\x07clip\r\nnext\tcell"
	got := safeDisplayText(input)
	if strings.ContainsAny(got, "\x1b\x07\r") {
		t.Fatalf("control sequence leaked into display text: %q", got)
	}
	for _, want := range []string{"ok", "red", "clip", "\nnext\tcell"} {
		if !strings.Contains(got, want) {
			t.Fatalf("safeDisplayText missing %q: %q", want, got)
		}
	}
}

func TestRenderMessagesStripsTerminalControlSequences(t *testing.T) {
	app := New(&config.Config{}, nil, nil)
	app.width = 80
	app.chat.Messages = append(app.chat.Messages,
		storage.Message{Role: "user", Content: "hello\x1b]52;c;dGVzdA==\x07"},
		storage.Message{Role: "assistant", Content: "reply\x1b[2J"},
	)
	got := app.renderMessages()
	if strings.ContainsAny(got, "\x07") || strings.Contains(got, "\x1b]52") || strings.Contains(got, "\x1b[2J") {
		t.Fatalf("terminal control sequence leaked into rendered messages: %q", got)
	}
}
