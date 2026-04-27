package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

func safeDisplayText(s string) string {
	s = ansi.Strip(s)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\n', '\t':
			b.WriteRune(r)
		case '\r':
			continue
		default:
			if !unicode.IsControl(r) {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
