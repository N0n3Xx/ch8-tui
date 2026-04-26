package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestFooterFitsCommonWidths(t *testing.T) {
	app := &App{}
	for _, width := range []int{20, 32, 48, 80, 120} {
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
