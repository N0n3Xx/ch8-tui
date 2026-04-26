package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderAssistantMarkdownRemovesCommonSyntax(t *testing.T) {
	input := strings.Join([]string{
		"## Heading",
		"",
		"This is **bold**, *italic*, and `code`.",
		"- first",
		"* second",
		"```",
		"fmt.Println(\"hi\")",
		"```",
	}, "\n")
	got := renderAssistantMarkdown(input, 60)
	for _, raw := range []string{"## Heading", "**bold**", "*italic*", "`code`", "```"} {
		if strings.Contains(got, raw) {
			t.Fatalf("rendered markdown still contains raw syntax %q: %q", raw, got)
		}
	}
	for _, want := range []string{"HEADING", "bold", "italic", "code", "first", "second", "fmt.Println"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered markdown missing %q: %q", want, got)
		}
	}
}

func TestRenderAssistantMarkdownKeepsCodeBlocksBounded(t *testing.T) {
	got := renderAssistantMarkdown("```\n12345678901234567890\n```", 10)
	for _, line := range strings.Split(got, "\n") {
		if lipgloss.Width(line) > 10 {
			t.Fatalf("line exceeds width: %d %q", lipgloss.Width(line), line)
		}
	}
}

func TestRenderAssistantMarkdownNestedUnorderedLists(t *testing.T) {
	got := renderAssistantMarkdown(strings.Join([]string{
		"- Item one",
		"- Item two",
		"  - Sub-item A",
		"  - Sub-item B",
		"- Item three",
	}, "\n"), 80)
	lines := strings.Split(got, "\n")
	want := []string{
		"- Item one",
		"- Item two",
		"  - Sub-item A",
		"  - Sub-item B",
		"- Item three",
	}
	if len(lines) != len(want) {
		t.Fatalf("line count = %d, want %d:\n%q", len(lines), len(want), got)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d = %q, want %q\nfull:\n%q", i, lines[i], want[i], got)
		}
	}
}

func TestRenderAssistantMarkdownMixedOrderedNestedLists(t *testing.T) {
	got := renderAssistantMarkdown(strings.Join([]string{
		"Steps:",
		"1. Install **Ollama**",
		"   - Pull `qwen`",
		"   1. Start server",
		"2. Run app",
	}, "\n"), 80)
	for _, want := range []string{
		"Steps:",
		"1. Install ",
		"   - Pull ",
		"   1. Start server",
		"2. Run app",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%q", want, got)
		}
	}
	for _, raw := range []string{"**Ollama**", "`qwen`"} {
		if strings.Contains(got, raw) {
			t.Fatalf("rendered list still contains raw syntax %q:\n%q", raw, got)
		}
	}
}

func TestRenderAssistantMarkdownWrappedListContinuationAligns(t *testing.T) {
	got := renderAssistantMarkdown("- This item has many words that should wrap under the text instead of under the bullet", 32)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapped list item, got:\n%q", got)
	}
	if !strings.HasPrefix(lines[0], "- ") {
		t.Fatalf("first line missing bullet: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  ") || strings.HasPrefix(lines[1], "  -") {
		t.Fatalf("wrapped line not aligned under text: %q\nfull:\n%q", lines[1], got)
	}
}
