package tui

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var orderedListPattern = regexp.MustCompile(`^(\d+)([.)])\s+(.*)$`)

type markdownListItem struct {
	indent int
	marker string
	text   string
}

func renderAssistantMarkdown(content string, width int) string {
	width = max(8, width)
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	var out []string
	inCode := false

	flushBlank := func() {
		if len(out) == 0 || out[len(out)-1] != "" {
			out = append(out, "")
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				inCode = false
				flushBlank()
			} else {
				flushBlank()
				inCode = true
			}
			continue
		}
		if inCode {
			innerWidth := max(1, width-2)
			code := truncateVisible(line, innerWidth)
			out = append(out, markdownCodeBlockStyle.Render(" "+padRightVisible(code, innerWidth)+" "))
			continue
		}
		if trimmed == "" {
			flushBlank()
			continue
		}
		if headingLevel, heading := parseHeading(trimmed); headingLevel > 0 {
			prefix := strings.Repeat(" ", headingLevel-1)
			rendered := markdownHeadingStyle.Render(strings.ToUpper(heading))
			out = append(out, wrapStyled(prefix+rendered, width))
			continue
		}
		if item, ok := parseListItem(line); ok {
			out = append(out, renderListItem(item, width)...)
			continue
		}
		out = append(out, wrapStyled(renderInlineMarkdown(line), width))
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func parseHeading(line string) (int, string) {
	level := 0
	for level < len(line) && level < 3 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level+1:])
}

func parseListItem(line string) (markdownListItem, bool) {
	indent := visualIndent(line)
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return markdownListItem{}, false
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return markdownListItem{
			indent: indent,
			marker: "-",
			text:   strings.TrimSpace(trimmed[2:]),
		}, true
	}
	if match := orderedListPattern.FindStringSubmatch(trimmed); match != nil {
		return markdownListItem{
			indent: indent,
			marker: match[1] + ".",
			text:   strings.TrimSpace(match[3]),
		}, true
	}
	return markdownListItem{}, false
}

func visualIndent(line string) int {
	indent := 0
	for _, r := range line {
		switch r {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			return indent
		}
	}
	return indent
}

func renderListItem(item markdownListItem, width int) []string {
	indent := min(item.indent, max(0, width-4))
	prefix := strings.Repeat(" ", indent) + item.marker + " "
	continuation := strings.Repeat(" ", lipgloss.Width(prefix))
	firstWidth := max(1, width-lipgloss.Width(prefix))
	nextWidth := max(1, width-lipgloss.Width(continuation))
	wrapped := wrapMarkdownTokens(item.text, firstWidth, nextWidth)
	if len(wrapped) == 0 {
		return []string{strings.TrimRight(prefix, " ")}
	}
	lines := make([]string, 0, len(wrapped))
	lines = append(lines, prefix+renderInlineMarkdown(wrapped[0]))
	for _, line := range wrapped[1:] {
		lines = append(lines, continuation+renderInlineMarkdown(line))
	}
	return lines
}

func wrapMarkdownTokens(text string, firstWidth, nextWidth int) []string {
	tokens := markdownTokens(text)
	if len(tokens) == 0 {
		return nil
	}
	var lines []string
	var current []string
	currentWidth := 0
	limit := firstWidth
	flush := func() {
		if len(current) == 0 {
			return
		}
		lines = append(lines, strings.Join(current, " "))
		current = nil
		currentWidth = 0
		limit = nextWidth
	}
	for _, token := range tokens {
		tokenWidth := markdownTokenWidth(token)
		added := tokenWidth
		if len(current) > 0 {
			added++
		}
		if len(current) > 0 && currentWidth+added > limit {
			flush()
			added = tokenWidth
		}
		current = append(current, token)
		currentWidth += added
	}
	flush()
	return lines
}

func markdownTokens(text string) []string {
	var tokens []string
	for i := 0; i < len(text); {
		if text[i] == ' ' || text[i] == '\t' {
			i++
			continue
		}
		if text[i] == '`' {
			if end := strings.IndexByte(text[i+1:], '`'); end >= 0 {
				tokens = append(tokens, text[i:i+end+2])
				i += end + 2
				continue
			}
		}
		if strings.HasPrefix(text[i:], "**") {
			if end := strings.Index(text[i+2:], "**"); end >= 0 {
				tokens = append(tokens, text[i:i+end+4])
				i += end + 4
				continue
			}
		}
		if text[i] == '*' {
			if end := strings.IndexByte(text[i+1:], '*'); end >= 0 {
				tokens = append(tokens, text[i:i+end+2])
				i += end + 2
				continue
			}
		}
		start := i
		for i < len(text) && text[i] != ' ' && text[i] != '\t' {
			i++
		}
		tokens = append(tokens, text[start:i])
	}
	return tokens
}

func markdownTokenWidth(token string) int {
	return lipgloss.Width(stripInlineMarkdown(token))
}

func stripInlineMarkdown(token string) string {
	if strings.HasPrefix(token, "`") && strings.HasSuffix(token, "`") && len(token) >= 2 {
		return token[1 : len(token)-1]
	}
	if strings.HasPrefix(token, "**") && strings.HasSuffix(token, "**") && len(token) >= 4 {
		return token[2 : len(token)-2]
	}
	if strings.HasPrefix(token, "*") && strings.HasSuffix(token, "*") && len(token) >= 2 {
		if _, err := strconv.Atoi(strings.Trim(token, "*")); err == nil {
			return token
		}
		return token[1 : len(token)-1]
	}
	return token
}

func renderInlineMarkdown(line string) string {
	var b strings.Builder
	for i := 0; i < len(line); {
		switch {
		case line[i] == '`':
			if end := strings.IndexByte(line[i+1:], '`'); end >= 0 {
				text := line[i+1 : i+1+end]
				b.WriteString(markdownCodeStyle.Render(text))
				i += end + 2
				continue
			}
		case strings.HasPrefix(line[i:], "**"):
			if end := strings.Index(line[i+2:], "**"); end >= 0 {
				text := line[i+2 : i+2+end]
				b.WriteString(markdownBoldStyle.Render(text))
				i += end + 4
				continue
			}
		case line[i] == '*':
			if end := strings.IndexByte(line[i+1:], '*'); end >= 0 {
				text := line[i+1 : i+1+end]
				b.WriteString(markdownItalicStyle.Render(text))
				i += end + 2
				continue
			}
		}
		b.WriteByte(line[i])
		i++
	}
	return b.String()
}

func wrapStyled(s string, width int) string {
	return wrapStyle.Width(width).Render(s)
}

func truncateVisible(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	for i := range runes {
		if lipgloss.Width(string(runes[:i+1])) > width {
			return string(runes[:i])
		}
	}
	return s
}

func padRightVisible(s string, width int) string {
	if pad := width - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
