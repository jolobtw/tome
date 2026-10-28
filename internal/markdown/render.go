package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jolobtw/tome/internal/ui"
)

var (
	wikiLinkRe   = regexp.MustCompile(`\[\[([^\]\|]+)(?:\|([^\]]+))?\]\]`)
	codeRe       = regexp.MustCompile("`([^`]+)`")
	boldItalicRe = regexp.MustCompile(`\*\*\*([^*]+)\*\*\*`)
	boldAstRe    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	boldUndRe    = regexp.MustCompile(`__([^_]+)__`)
	italicAstRe  = regexp.MustCompile(`(^|[\s\(\[\{<"'])\*([^*\s](?:[^*]*[^*\s])?)\*([\s\)\|\}\]>"'.,!?;:]|$)`)
	italicUndRe  = regexp.MustCompile(`(^|[\s\(\[\{<"'])_([^\s_](?:[^_]*[^\s_])?)_([\s\)\|\}\]>"'.,!?;:]|$)`)
	strikeRe     = regexp.MustCompile(`~~([^~]+)~~`)
	highlightRe  = regexp.MustCompile(`==([^=]+)==`)
	underlineRe  = regexp.MustCompile(`<(?:u|ins)>([^<]+)</(?:u|ins)>`)
)

// wrapText breaks a string into lines that do not exceed maxWidth visual columns
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	var currentLine string

	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if lipgloss.Width(currentLine+" "+word) <= maxWidth {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return lines
}

// RenderDocument converts raw markdown text into styled terminal ANSI lines with given width
func RenderDocument(raw string, width int) string {
	if width < 24 {
		width = 24
	}

	lines := strings.Split(raw, "\n")
	var rendered []string
	inCodeBlock := false
	currentCalloutColor := ui.ColorCyan

	// Box border style for codeblocks
	codeBorderStyle := lipgloss.NewStyle().Foreground(ui.ColorBorderMuted)
	boxWidth := max(20, width-6)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 1. Code Blocks: Fully enclosed box with matching corners and equal width
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				lang := strings.TrimPrefix(strings.TrimPrefix(trimmed, "```"), "~~~")
				if lang == "" {
					lang = "code"
				}
				ruleLen := max(2, boxWidth-len(lang)-6)
				headerLine := fmt.Sprintf("  ╭─ %s %s╮", lang, strings.Repeat("─", ruleLen))
				rendered = append(rendered, codeBorderStyle.Render(headerLine))
			} else {
				footerLine := fmt.Sprintf("  ╰%s╯", strings.Repeat("─", boxWidth-2))
				rendered = append(rendered, codeBorderStyle.Render(footerLine))
			}
			continue
		}

		if inCodeBlock {
			leftBar := codeBorderStyle.Render("  │  ")
			rightBar := codeBorderStyle.Render("│")

			availCodeWidth := max(4, boxWidth-6)
			displayCode := line
			if lipgloss.Width(displayCode) > availCodeWidth {
				displayCode = displayCode[:availCodeWidth-3] + "..."
			}
			padSpaces := max(0, availCodeWidth-lipgloss.Width(displayCode))
			paddedCode := lipgloss.NewStyle().Foreground(ui.ColorGreen).Render(displayCode) + strings.Repeat(" ", padSpaces)

			rendered = append(rendered, leftBar+paddedCode+" "+rightBar)
			continue
		}

		// 2. Obsidian Callouts: Left accent bar down every wrapped line
		if strings.HasPrefix(trimmed, "> [!NOTE]") || strings.HasPrefix(trimmed, "> [!INFO]") {
			currentCalloutColor = ui.ColorCyan
			bar := lipgloss.NewStyle().Foreground(currentCalloutColor).Bold(true).Render("  ▎ ")
			title := lipgloss.NewStyle().Foreground(currentCalloutColor).Bold(true).Render("ℹ NOTE")
			rendered = append(rendered, "", bar+title)
			continue
		}
		if strings.HasPrefix(trimmed, "> [!TIP]") {
			currentCalloutColor = ui.ColorGreen
			bar := lipgloss.NewStyle().Foreground(currentCalloutColor).Bold(true).Render("  ▎ ")
			title := lipgloss.NewStyle().Foreground(currentCalloutColor).Bold(true).Render("💡 TIP")
			rendered = append(rendered, "", bar+title)
			continue
		}
		if strings.HasPrefix(trimmed, "> [!WARNING]") || strings.HasPrefix(trimmed, "> [!IMPORTANT]") {
			currentCalloutColor = ui.ColorAmberGold
			bar := lipgloss.NewStyle().Foreground(currentCalloutColor).Bold(true).Render("  ▎ ")
			title := lipgloss.NewStyle().Foreground(currentCalloutColor).Bold(true).Render("⚠️  WARNING")
			rendered = append(rendered, "", bar+title)
			continue
		}
		if strings.HasPrefix(trimmed, "> [!CAUTION]") {
			currentCalloutColor = ui.ColorRed
			bar := lipgloss.NewStyle().Foreground(currentCalloutColor).Bold(true).Render("  ▎ ")
			title := lipgloss.NewStyle().Foreground(currentCalloutColor).Bold(true).Render("🛑 CAUTION")
			rendered = append(rendered, "", bar+title)
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			content := strings.TrimPrefix(line, ">")
			content = strings.TrimPrefix(content, " ")
			bar := lipgloss.NewStyle().Foreground(currentCalloutColor).Render("  ▎ ")

			availWidth := max(10, width-8)
			wrapped := wrapText(formatInlineText(content), availWidth)
			for _, wLine := range wrapped {
				rendered = append(rendered, bar+wLine)
			}
			continue
		}

		// 3. Headings
		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimPrefix(trimmed, "# ")
			h1 := lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorFlameOrange).
				Render("  " + title)
			underlineLen := min(width-6, lipgloss.Width(title)+4)
			rule := lipgloss.NewStyle().
				Foreground(ui.ColorAmberGold).
				Render("  " + strings.Repeat("━", underlineLen))
			rendered = append(rendered, "", h1, rule)
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			title := strings.TrimPrefix(trimmed, "## ")
			h2 := lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorAmberGold).
				Render("  " + title)
			rendered = append(rendered, "", h2)
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			title := strings.TrimPrefix(trimmed, "### ")
			h3 := lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorGoldGlow).
				Render("  " + title)
			rendered = append(rendered, "", h3)
			continue
		}

		// 4. Horizontal Dividers
		if trimmed == "---" || trimmed == "***" {
			rule := lipgloss.NewStyle().
				Foreground(ui.ColorBorderMuted).
				Render("  " + strings.Repeat("─", max(10, width-8)))
			rendered = append(rendered, "", rule, "")
			continue
		}

		// 5. Checklists (with proper indented wrapping and multi-dash support)
		cDashCount := 0
		for cDashCount < len(trimmed) && (trimmed[cDashCount] == '-' || trimmed[cDashCount] == '*' || trimmed[cDashCount] == '+') {
			cDashCount++
		}
		var isChecklist bool
		var isChecked bool
		var checkBody string
		if cDashCount > 0 {
			after := trimmed[cDashCount:]
			if strings.HasPrefix(after, " [x] ") || strings.HasPrefix(after, " [X] ") {
				isChecklist = true
				isChecked = true
				checkBody = strings.TrimPrefix(after[5:], " ")
			} else if strings.HasPrefix(after, " [ ] ") {
				isChecklist = true
				isChecked = false
				checkBody = strings.TrimPrefix(after[5:], " ")
			}
		}

		if isChecklist {
			rawLeading := len(line) - len(strings.TrimLeft(line, " \t"))
			depth := min(4, rawLeading/2+(cDashCount-1))
			baseMargin := strings.Repeat("  ", depth+1)
			var prefixStr string
			if isChecked {
				prefixStr = lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true).Render(baseMargin + "\u2714 ")
			} else {
				prefixStr = lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Render(baseMargin + "\u25cb ")
			}
			indent := baseMargin + "  "

			availWidth := max(10, width-len(indent)-4)
			wrapped := wrapText(formatInlineText(checkBody), availWidth)
			for idx, wLine := range wrapped {
				if idx == 0 {
					rendered = append(rendered, prefixStr+wLine)
				} else {
					rendered = append(rendered, indent+wLine)
				}
			}
			continue
		}

		// 6. Bullet lists (with proper indented nesting, wrapping, and multi-dash outline shorthand)
		if cDashCount > 0 && cDashCount < len(trimmed) && trimmed[cDashCount] == ' ' {
			body := strings.TrimLeft(trimmed[cDashCount:], " ")

			rawLeading := len(line) - len(strings.TrimLeft(line, " \t"))
			depth := min(4, rawLeading/2+(cDashCount-1))

			// Choose bullet glyph and color according to nesting level
			var bulletGlyph string
			var bulletColor lipgloss.Color
			switch depth {
			case 0:
				bulletGlyph = "\u2022 "
				bulletColor = ui.ColorAmberGold
			case 1:
				bulletGlyph = "\u25e6 "
				bulletColor = ui.ColorGoldGlow
			default:
				bulletGlyph = "\u25aa "
				bulletColor = ui.ColorTextMuted
			}

			// Base indentation: 2 spaces margin + 2 spaces per depth level
			baseMargin := strings.Repeat("  ", depth+1)
			bullet := lipgloss.NewStyle().Foreground(bulletColor).Render(baseMargin + bulletGlyph)
			indent := baseMargin + "  "

			availWidth := max(10, width-len(indent)-4)
			wrapped := wrapText(formatInlineText(body), availWidth)
			for idx, wLine := range wrapped {
				if idx == 0 {
					rendered = append(rendered, bullet+wLine)
				} else {
					rendered = append(rendered, indent+wLine)
				}
			}
			continue
		}

		// 7. Regular paragraph text (word-wrapped cleanly inside viewport)
		if trimmed == "" {
			rendered = append(rendered, "")
		} else {
			availWidth := max(10, width-6)
			wrapped := wrapText(formatInlineText(line), availWidth)
			for _, wLine := range wrapped {
				rendered = append(rendered, "  "+wLine)
			}
		}
	}

	return strings.Join(rendered, "\n")
}

// styleSpan styles each word individually within a span so ANSI codes stay tightly bound when line-wrapped
func styleSpan(text string, style lipgloss.Style) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	styled := make([]string, len(words))
	for i, w := range words {
		styled[i] = style.Render(w)
	}
	return strings.Join(styled, " ")
}

// formatInlineText handles inline markdown formatting: links, code, bold, italic, strikethrough, highlights, and underlines
func formatInlineText(text string) string {
	boldStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorTextBright)
	italicStyle := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorAmberGold)
	boldItalicStyle := lipgloss.NewStyle().Bold(true).Italic(true).Foreground(ui.ColorGoldGlow)
	strikeStyle := lipgloss.NewStyle().Strikethrough(true).Foreground(ui.ColorTextMuted)
	hlStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorFlameOrange)
	uStyle := lipgloss.NewStyle().Underline(true).Foreground(ui.ColorTextBright)
	codeStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
	linkStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Underline(true)

	// 1. Wiki-links: [[Target]] or [[Target|Alias]]
	text = wikiLinkRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := wikiLinkRe.FindStringSubmatch(m)
		target := sub[1]
		display := target
		if len(sub) > 2 && sub[2] != "" {
			display = sub[2]
		}
		return styleSpan("󰌹 "+display, linkStyle)
	})

	// 2. Inline code: `code`
	text = codeRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := codeRe.FindStringSubmatch(m)
		return styleSpan(sub[1], codeStyle)
	})

	// 3. Bold & Italic: ***text***
	text = boldItalicRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := boldItalicRe.FindStringSubmatch(m)
		return styleSpan(sub[1], boldItalicStyle)
	})

	// 4. Bold: **text** or __text__
	text = boldAstRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := boldAstRe.FindStringSubmatch(m)
		return styleSpan(sub[1], boldStyle)
	})
	text = boldUndRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := boldUndRe.FindStringSubmatch(m)
		return styleSpan(sub[1], boldStyle)
	})

	// 5. Italic: *text* or _text_
	for italicAstRe.MatchString(text) {
		text = italicAstRe.ReplaceAllStringFunc(text, func(m string) string {
			sub := italicAstRe.FindStringSubmatch(m)
			return sub[1] + styleSpan(sub[2], italicStyle) + sub[3]
		})
	}
	for italicUndRe.MatchString(text) {
		text = italicUndRe.ReplaceAllStringFunc(text, func(m string) string {
			sub := italicUndRe.FindStringSubmatch(m)
			return sub[1] + styleSpan(sub[2], italicStyle) + sub[3]
		})
	}

	// 6. Strikethrough: ~~text~~
	text = strikeRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := strikeRe.FindStringSubmatch(m)
		return styleSpan(sub[1], strikeStyle)
	})

	// 7. Highlight: ==text==
	text = highlightRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := highlightRe.FindStringSubmatch(m)
		return styleSpan(sub[1], hlStyle)
	})

	// 8. Underline: <u>text</u> or <ins>text</ins>
	text = underlineRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := underlineRe.FindStringSubmatch(m)
		return styleSpan(sub[1], uStyle)
	})

	return text
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
