package markdown

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"tome/internal/ui"
)

// RenderDocument converts raw markdown text into styled terminal ANSI lines with given width
func RenderDocument(raw string, width int) string {
	if width < 20 {
		width = 20
	}

	lines := strings.Split(raw, "\n")
	var rendered []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 1. Code Blocks
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inCodeBlock = !inCodeBlock
			border := strings.Repeat("─", width-4)
			if inCodeBlock {
				lang := strings.TrimPrefix(strings.TrimPrefix(trimmed, "```"), "~~~")
				if lang == "" {
					lang = "code"
				}
				rendered = append(rendered, lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Render(fmt.Sprintf("  ╭─ %s %s", lang, border[:max(0, len(border)-len(lang)-4)])))
			} else {
				rendered = append(rendered, lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Render(fmt.Sprintf("  ╰%s", border)))
			}
			continue
		}

		if inCodeBlock {
			codeLine := lipgloss.NewStyle().
				Foreground(ui.ColorGreen).
				Render("  │ " + line)
			rendered = append(rendered, codeLine)
			continue
		}

		// 2. Obsidian Callouts
		if strings.HasPrefix(trimmed, "> [!NOTE]") || strings.HasPrefix(trimmed, "> [!INFO]") {
			rendered = append(rendered, lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorCyan).
				Render("  ╭─ ℹ  NOTE ─────────────────────────────────────"))
			continue
		}
		if strings.HasPrefix(trimmed, "> [!TIP]") {
			rendered = append(rendered, lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorGreen).
				Render("  ╭─ 💡 TIP ──────────────────────────────────────"))
			continue
		}
		if strings.HasPrefix(trimmed, "> [!WARNING]") || strings.HasPrefix(trimmed, "> [!IMPORTANT]") {
			rendered = append(rendered, lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorAmberGold).
				Render("  ╭─ ⚠️  WARNING ──────────────────────────────────"))
			continue
		}
		if strings.HasPrefix(trimmed, "> [!CAUTION]") {
			rendered = append(rendered, lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorRed).
				Render("  ╭─ 🛑 CAUTION ──────────────────────────────────"))
			continue
		}
		if strings.HasPrefix(trimmed, ">") {
			content := strings.TrimPrefix(line, ">")
			content = strings.TrimPrefix(content, " ")
			rendered = append(rendered, lipgloss.NewStyle().
				Foreground(ui.ColorTextRegular).
				Render("  │  "+content))
			continue
		}

		// 3. Headings
		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimPrefix(trimmed, "# ")
			h1 := lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorFlameOrange).
				Render("  " + title)
			rule := lipgloss.NewStyle().
				Foreground(ui.ColorAmberGold).
				Render("  " + strings.Repeat("━", min(width-6, len(title)+4)))
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
				Foreground(ui.ColorGoldHighlight).
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

		// 5. Checklists
		if strings.HasPrefix(trimmed, "- [x]") || strings.HasPrefix(trimmed, "* [x]") {
			body := strings.TrimPrefix(trimmed[5:], " ")
			chk := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true).Render("  ✔ ")
			rendered = append(rendered, chk+lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Render(body))
			continue
		}
		if strings.HasPrefix(trimmed, "- [ ]") || strings.HasPrefix(trimmed, "* [ ]") {
			body := strings.TrimPrefix(trimmed[5:], " ")
			unchk := lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Render("  ○ ")
			rendered = append(rendered, unchk+lipgloss.NewStyle().Foreground(ui.ColorTextRegular).Render(body))
			continue
		}

		// 6. Bullet lists
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			body := trimmed[2:]
			bullet := lipgloss.NewStyle().Foreground(ui.ColorAmberGold).Render("  • ")
			rendered = append(rendered, bullet+formatInlineText(body))
			continue
		}

		// 7. Regular text (with inline formatting)
		if trimmed == "" {
			rendered = append(rendered, "")
		} else {
			rendered = append(rendered, "  "+formatInlineText(line))
		}
	}

	return strings.Join(rendered, "\n")
}

// formatInlineText handles inline wiki-links [[Link]], `code`, and **bold**
func formatInlineText(text string) string {
	// Simple inline styling replacements
	// 1. Wiki-links: [[Target]]
	for strings.Contains(text, "[[") && strings.Contains(text, "]]") {
		start := strings.Index(text, "[[")
		end := strings.Index(text, "]]")
		if start < end {
			link := text[start+2 : end]
			styledLink := lipgloss.NewStyle().
				Foreground(ui.ColorCyan).
				Underline(true).
				Render("󰌹 " + link)
			text = text[:start] + styledLink + text[end+2:]
		} else {
			break
		}
	}

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
