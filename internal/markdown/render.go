package markdown

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"tome/internal/ui"
)

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
			// Code content with matching left and right borders
			leftBar := codeBorderStyle.Render("  │  ")
			rightBar := codeBorderStyle.Render("│")

			// Truncate or pad to box width
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

		// 2. Obsidian Callouts: Sleek accent-bar style matching Obsidian Desktop
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
			text := lipgloss.NewStyle().Foreground(ui.ColorTextRegular).Render(formatInlineText(content))
			rendered = append(rendered, bar+text)
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
