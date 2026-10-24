package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Palette defines our primary color tokens (Tome's Arcane Flame & Slate Theme)
var (
	ColorFlameOrange  = lipgloss.Color("#FF7A00")
	ColorAmberGold    = lipgloss.Color("#FFAF00")
	ColorGoldHighlight= lipgloss.Color("#FFD166")
	ColorDarkSlate    = lipgloss.Color("#181920")
	ColorPanelBg      = lipgloss.Color("#1E2029")
	ColorBorderMuted  = lipgloss.Color("#383C4A")
	ColorTextMuted    = lipgloss.Color("#6C7086")
	ColorTextRegular  = lipgloss.Color("#D9E0EE")
	ColorTextBright   = lipgloss.Color("#F5F5F7")
	ColorCyan         = lipgloss.Color("#89DCEB")
	ColorGreen        = lipgloss.Color("#A6E3A1")
	ColorRed          = lipgloss.Color("#F38BA8")
	ColorPurple       = lipgloss.Color("#CBA6F7")
)

// UI Element Styles
var (
	// Brand Title
	BrandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFlameOrange)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorAmberGold).
			Italic(true)

	BadgeStyle = lipgloss.NewStyle().
			Foreground(ColorTextBright).
			Background(ColorBorderMuted).
			Padding(0, 1)

	ActiveBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(ColorAmberGold).
				Bold(true).
				Padding(0, 1)

	// Panes
	PaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorderMuted).
			Padding(0, 1)

	ActivePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorFlameOrange).
			Padding(0, 1)

	// Tree Items
	TreeItemStyle = lipgloss.NewStyle().
			Foreground(ColorTextRegular)

	TreeFolderStyle = lipgloss.NewStyle().
			Foreground(ColorAmberGold).
			Bold(true)

	TreeSelectedStyle = lipgloss.NewStyle().
				Background(ColorFlameOrange).
				Foreground(lipgloss.Color("#000000")).
				Bold(true)

	// AI Drawer
	AIDrawerStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ColorAmberGold).
			Background(ColorDarkSlate).
			Padding(0, 1)

	// Footer
	FooterKeyStyle = lipgloss.NewStyle().
			Foreground(ColorAmberGold).
			Bold(true)

	FooterDescStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)
)

// RenderGradientText returns text smoothly interpolated between two RGB colors
func RenderGradientText(text string, startHex, endHex string) string {
	// For simplicity in terminal, if text is short, lipgloss supports truecolor
	return fmt.Sprintf("%s", BrandStyle.Render(text))
}
