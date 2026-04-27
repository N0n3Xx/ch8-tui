package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"ch8-tui/internal/storage"
)

func TestFooterFitsCommonWidths(t *testing.T) {
	app := &App{}
	for _, width := range []int{12, 20, 32, 48, 80, 120} {
		items := app.footerItems(width)
		rendered := renderFooterItems(items, width)
		if got := lipgloss.Width(rendered); got != width {
			t.Fatalf("footer width %d rendered as %d: %q", width, got, rendered)
		}
		if footerItemsWidth(items) > width && width >= 32 {
			t.Fatalf("selected footer variant exceeds width %d: %d", width, footerItemsWidth(items))
		}
	}
}

func TestFooterStyleDoesNotChangeRenderedWidth(t *testing.T) {
	app := &App{width: 64}
	rendered := app.footer()
	if got := lipgloss.Width(rendered); got != 64 {
		t.Fatalf("styled footer width = %d, want 64: %q", got, rendered)
	}
	if got := lipgloss.Height(rendered); got != 1 {
		t.Fatalf("styled footer height = %d, want 1: %q", got, rendered)
	}
}

func TestViewportContentPadsAboveShortChats(t *testing.T) {
	app := &App{
		width:         80,
		viewport:      viewport.New(74, 8),
		stickToBottom: true,
		chat: &storage.Chat{Messages: []storage.Message{
			{Role: "user", Content: "Test"},
		}},
	}
	app.refreshViewport(false)
	view := app.viewport.View()
	if lipgloss.Height(view) != 8 {
		t.Fatalf("viewport height = %d, want 8: %q", lipgloss.Height(view), view)
	}
	if !strings.Contains(view, "Test") {
		t.Fatalf("viewport does not contain message: %q", view)
	}
	if strings.HasPrefix(view, userStyle.Render("USER")) {
		t.Fatalf("short chat was top-aligned, want top padding: %q", view)
	}
}
