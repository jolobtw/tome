package ui

import (
	"fmt"
	"math"
	"strconv"

	"github.com/charmbracelet/lipgloss"
)

// Palette tokens (Tome's Arcane Flame & Slate Theme)
var (
	ColorFlameDeep    = lipgloss.Color("#E64A19")
	ColorFlameOrange  = lipgloss.Color("#FF7A00")
	ColorAmberGold    = lipgloss.Color("#FFAF00")
	ColorGoldGlow     = lipgloss.Color("#FFD166")
	ColorDarkSlate    = lipgloss.Color("#13141C")
	ColorPanelBg      = lipgloss.Color("#181A24")
	ColorSelectBg     = lipgloss.Color("#262938") // Subtle, calm slate for active item
	ColorBorderMuted  = lipgloss.Color("#2E3244")
	ColorBorderActive = lipgloss.Color("#FF7A00")
	ColorTextMuted    = lipgloss.Color("#6C7086")
	ColorTextRegular  = lipgloss.Color("#A6ADC8")
	ColorTextBright   = lipgloss.Color("#FFFFFF")
	ColorCyan         = lipgloss.Color("#89DCEB")
	ColorGreen        = lipgloss.Color("#A6E3A1")
	ColorRed          = lipgloss.Color("#F38BA8")
	ColorPurple       = lipgloss.Color("#CBA6F7")
)

// UI Element Styles
var (
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true)

	BadgeStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Background(ColorSelectBg).
			Padding(0, 1)

	ActiveBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorGoldGlow).
				Background(ColorSelectBg).
				Bold(true).
				Padding(0, 1)

	// Panes: 1 border + 1 padding each side = 4 total horizontal overhead
	PaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorderMuted).
			Padding(0, 1)

	ActivePaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorFlameOrange).
			Padding(0, 1)

	// Tree Items:
	// Unselected items are muted and calm
	TreeNormalItem = lipgloss.NewStyle().
			Foreground(ColorTextRegular)

	// Unselected folder: soft muted slate-gold, NOT bold so it doesn't look active
	TreeNormalFolder = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#C5A880"))

	// Cursor for whatever is ACTUALLY selected
	TreeCursor = lipgloss.NewStyle().
			Foreground(ColorFlameOrange).
			Bold(true)

	// Selected text: bright white on soft subtle dark slate
	TreeSelectedText = lipgloss.NewStyle().
				Foreground(ColorTextBright).
				Background(ColorSelectBg).
				Bold(true)

	// AI Drawer
	AIDrawerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
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

// HexToRGB parses a "#RRGGBB" hex string into r, g, b components
func HexToRGB(hex string) (uint8, uint8, uint8) {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return 255, 255, 255
	}
	val, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 255, 255, 255
	}
	return uint8((val >> 16) & 0xFF), uint8((val >> 8) & 0xFF), uint8(val & 0xFF)
}

// Gradient creates a smooth character-by-character color gradient
func Gradient(text string, fromHex, toHex string) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}
	if n == 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(fromHex)).Render(text)
	}

	r1, g1, b1 := HexToRGB(fromHex)
	r2, g2, b2 := HexToRGB(toHex)

	var result string
	for i, r := range runes {
		ratio := float64(i) / float64(n-1)
		curR := uint8(math.Round(float64(r1) + ratio*(float64(r2)-float64(r1))))
		curG := uint8(math.Round(float64(g1) + ratio*(float64(g2)-float64(g1))))
		curB := uint8(math.Round(float64(b1) + ratio*(float64(b2)-float64(b1))))
		hexColor := fmt.Sprintf("#%02X%02X%02X", curR, curG, curB)
		result += lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor)).Render(string(r))
	}
	return result
}
