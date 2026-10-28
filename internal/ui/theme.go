package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	ColorCrimson      = lipgloss.Color("#FF4D4D")
	ColorPurple       = lipgloss.Color("#CBA6F7")
)

// UI Element Styles
var (
	BrandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFlameOrange)

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

// GradientWithBg creates a gradient with an explicit solid background color on every rune
func GradientWithBg(text string, fromHex, toHex string, bg lipgloss.Color) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}
	if n == 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(fromHex)).Background(bg).Render(text)
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
		result += lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor)).Background(bg).Render(string(r))
	}
	return result
}

const RemoteThemeURL = "https://raw.githubusercontent.com/jolobtw/arcane-theme/main/theme.json"

type ThemeFile struct {
	Palette struct {
		CanvasBg        string `json:"canvas_bg"`
		PanelBg         string `json:"panel_bg"`
		SelectionBg     string `json:"selection_bg"`
		BorderActive    string `json:"border_active"`
		BorderMuted     string `json:"border_muted"`
		AccentFlame     string `json:"accent_flame"`
		AccentFlameDeep string `json:"accent_flame_deep"`
		AccentAmber     string `json:"accent_amber"`
		AccentGold      string `json:"accent_gold"`
		GradientStart   string `json:"gradient_start"`
		GradientEnd     string `json:"gradient_end"`
		AlertBg         string `json:"alert_bg"`
		TextBright      string `json:"text_bright"`
		TextRegular     string `json:"text_regular"`
		TextMuted       string `json:"text_muted"`
		StatusError     string `json:"status_error"`
		StatusSuccess   string `json:"status_success"`
		StatusInfo      string `json:"status_info"`
		FolderWarm      string `json:"folder_warm"`
	} `json:"palette"`
	Header struct {
		StarGlyph      string   `json:"star_glyph"`
		StarColor      string   `json:"star_color"`
		TitleGlyph     string   `json:"title_glyph"`
		TitleGradient  []string `json:"title_gradient"`
		SeparatorGlyph string   `json:"separator_glyph"`
		SeparatorColor string   `json:"separator_color"`
		TaglineColor   string   `json:"tagline_color"`
		TaglineItalic  bool     `json:"tagline_italic"`
		DividerGlyph   string   `json:"divider_glyph"`
		DividerColor   string   `json:"divider_color"`
	} `json:"header"`
}

func RefreshStyles() {
	BrandStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorFlameOrange)
	SubtitleStyle = lipgloss.NewStyle().Foreground(ColorTextMuted).Italic(true)
	BadgeStyle = lipgloss.NewStyle().Foreground(ColorTextMuted).Background(ColorSelectBg).Padding(0, 1)
	ActiveBadgeStyle = lipgloss.NewStyle().Foreground(ColorGoldGlow).Background(ColorSelectBg).Bold(true).Padding(0, 1)
	PaneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorderMuted).Padding(0, 1)
	ActivePaneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorFlameOrange).Padding(0, 1)
	TreeNormalItem = lipgloss.NewStyle().Foreground(ColorTextRegular)
	TreeCursor = lipgloss.NewStyle().Foreground(ColorFlameOrange).Bold(true)
	TreeSelectedText = lipgloss.NewStyle().Foreground(ColorTextBright).Background(ColorSelectBg).Bold(true)
	AIDrawerStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAmberGold).Background(ColorDarkSlate).Padding(0, 1)
	FooterKeyStyle = lipgloss.NewStyle().Foreground(ColorAmberGold).Bold(true)
	FooterDescStyle = lipgloss.NewStyle().Foreground(ColorTextMuted)
}

func ApplyTheme(th ThemeFile) {
	if th.Palette.CanvasBg != "" {
		ColorDarkSlate = lipgloss.Color(th.Palette.CanvasBg)
	}
	if th.Palette.PanelBg != "" {
		ColorPanelBg = lipgloss.Color(th.Palette.PanelBg)
	}
	if th.Palette.SelectionBg != "" {
		ColorSelectBg = lipgloss.Color(th.Palette.SelectionBg)
	}
	if th.Palette.BorderActive != "" {
		ColorBorderActive = lipgloss.Color(th.Palette.BorderActive)
	}
	if th.Palette.BorderMuted != "" {
		ColorBorderMuted = lipgloss.Color(th.Palette.BorderMuted)
	}
	if th.Palette.AccentFlame != "" {
		ColorFlameOrange = lipgloss.Color(th.Palette.AccentFlame)
	}
	if th.Palette.AccentFlameDeep != "" {
		ColorFlameDeep = lipgloss.Color(th.Palette.AccentFlameDeep)
	}
	if th.Palette.AccentAmber != "" {
		ColorAmberGold = lipgloss.Color(th.Palette.AccentAmber)
	}
	if th.Palette.AccentGold != "" {
		ColorGoldGlow = lipgloss.Color(th.Palette.AccentGold)
	}
	if th.Palette.TextBright != "" {
		ColorTextBright = lipgloss.Color(th.Palette.TextBright)
	}
	if th.Palette.TextRegular != "" {
		ColorTextRegular = lipgloss.Color(th.Palette.TextRegular)
	}
	if th.Palette.TextMuted != "" {
		ColorTextMuted = lipgloss.Color(th.Palette.TextMuted)
	}
	if th.Palette.StatusError != "" {
		ColorCrimson = lipgloss.Color(th.Palette.StatusError)
	}
	if th.Palette.StatusSuccess != "" {
		ColorGreen = lipgloss.Color(th.Palette.StatusSuccess)
	}
	if th.Palette.StatusInfo != "" {
		ColorCyan = lipgloss.Color(th.Palette.StatusInfo)
	}
	if th.Palette.FolderWarm != "" {
		TreeNormalFolder = lipgloss.NewStyle().Foreground(lipgloss.Color(th.Palette.FolderWarm))
	}
	RefreshStyles()
}

// InitAutoSyncTheme loads ~/.config/arcane/theme.json immediately, and spawns a background fetch from GitHub
func InitAutoSyncTheme() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	configDir := filepath.Join(home, ".config", "arcane")
	themePath := filepath.Join(configDir, "theme.json")

	// 1. Read existing local cache immediately (instant < 1ms)
	if data, err := os.ReadFile(themePath); err == nil {
		var th ThemeFile
		if err := json.Unmarshal(data, &th); err == nil {
			ApplyTheme(th)
		}
	}

	// 2. Spawn non-blocking background fetch from GitHub
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		req, err := http.NewRequest("GET", RemoteThemeURL, nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "Tome-AutoSync/1.0")
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}
			var th ThemeFile
			if err := json.Unmarshal(body, &th); err == nil {
				_ = os.MkdirAll(configDir, 0755)
				_ = os.WriteFile(themePath, body, 0644)
				ApplyTheme(th)
			}
		}
	}()
}

// FormatHeader renders the exact 2-row Arcane header matching the design system
func FormatHeader(appTitle, tagline string, badges []string, width int) (row1, row2 string) {
	star := lipgloss.NewStyle().Foreground(ColorFlameOrange).Bold(true).Render("✦")
	sep := lipgloss.NewStyle().Foreground(ColorTextMuted).Render("❖")
	title := Gradient(fmt.Sprintf("❖ %s", appTitle), "#FF4500", "#FFD166")
	tag := SubtitleStyle.Render(tagline)

	leftPart := fmt.Sprintf("%s %s %s %s", star, title, sep, tag)
	rightPart := strings.Join(badges, " ")

	leftLen := lipgloss.Width(leftPart)
	rightLen := lipgloss.Width(rightPart)
	if leftLen+rightLen+2 > width {
		leftPart = fmt.Sprintf("%s %s", star, title)
		leftLen = lipgloss.Width(leftPart)
	}

	gap := 1
	if width-leftLen-rightLen-2 > 1 {
		gap = width - leftLen - rightLen - 2
	}
	row1 = " " + leftPart + strings.Repeat(" ", gap) + rightPart
	row2 = lipgloss.NewStyle().Foreground(ColorBorderMuted).Render(strings.Repeat("─", width))
	return row1, row2
}
