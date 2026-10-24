package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tome/internal/markdown"
	"tome/internal/ui"
)

// Item represents a node in the tree (folder or note)
type Item struct {
	ID       int
	Title    string
	Path     string
	IsFolder bool
	Expanded bool
	Level    int
	Content  string
	ChildIDs []int
	ParentID int
}

type Model struct {
	width          int
	height         int
	ready          bool
	activePane     int // 0: Tree, 1: Preview
	allItems       []Item
	visibleIndices []int
	selectedIdx    int
	viewport       viewport.Model

	// Interactive AI Knowledge Assistant
	aiOpen       bool
	aiInput      textinput.Model
	aiState      string // "idle", "thinking", "streaming"
	aiQuery      string
	aiResponse   string
	aiAnimStep   int
	aiStreamList []string
	aiStreamIdx  int

	// Smooth slide animation
	animHeight   int
	targetHeight int
	sliding      bool
	slideDir     int // +1 opening, -1 closing

	statusMsg string
}

// Custom animation and streaming messages
type animTickMsg time.Time
type slideTickMsg struct{}
type aiStreamMsg string
type aiStartStreamMsg struct{}

func animTick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg {
		return animTickMsg(t)
	})
}

func slideTick() tea.Cmd {
	return tea.Tick(18*time.Millisecond, func(t time.Time) tea.Msg {
		return slideTickMsg{}
	})
}

func streamToken(token string, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return aiStreamMsg(token)
	})
}

var arcaneSpinners = []string{
	"✦ ✧ ✦ ✧",
	"✧ ✦ ✧ ✦",
	"᚛ ᚜ ᚛ ᚜",
	"᚜ ᚛ ᚜ ᚛",
	"❖ ✦ ❖ ✦",
}

func sampleData() []Item {
	return []Item{
		{
			ID:       0,
			Title:    "Grimoire",
			IsFolder: true,
			Expanded: true,
			Level:    0,
			ParentID: -1,
			ChildIDs: []int{1, 2},
		},
		{
			ID:       1,
			Title:    "The Arcane Architecture.md",
			IsFolder: false,
			Level:    1,
			ParentID: 0,
			Content: `# The Arcane Architecture 📖

> [!NOTE]
> Tome combines raw compiled terminal speed with Obsidian-grade aesthetics.

### Guiding Principles
- [x] Zero external dependencies at runtime.
- [x] Strict viewport bounding: no screen jumps or off-screen overflows.
- [x] Yazi-inspired mouse hover responsiveness.
- [ ] Direct cloud AI research stream.

### Visual Foundations
We use 24-bit TrueColor ANSI styling with crisp 90° box borders and subtle slate backgrounds.

~~~go
func Launch() error {
    // Zero-overhead compiled startup in <10ms
    return tea.NewProgram(tomeModel).Run()
}
~~~

Refer to [[Cross-Platform Sync]] for multi-machine workflows.`,
		},
		{
			ID:       2,
			Title:    "Color Palette & Glyphs.md",
			IsFolder: false,
			Level:    1,
			ParentID: 0,
			Content: `# Color Palette & Glyphs 🎨

### The Arcane Flame Theme
- **Flame Orange** (#FF7A00): Primary accent, titles, and active pane borders.
- **Amber Gold** (#FFAF00): Subtitles, folder glyphs, and warnings.
- **Dark Slate** (#181920): Card background and terminal panels.

> [!TIP]
> Ensure a Nerd Font (such as FiraCode NF or MesloLGS NF) is active in your terminal for crisp folder icons.`,
		},
		{
			ID:       3,
			Title:    "Codex",
			IsFolder: true,
			Expanded: true,
			Level:    0,
			ParentID: -1,
			ChildIDs: []int{4},
		},
		{
			ID:       4,
			Title:    "Cross-Platform Sync.md",
			IsFolder: false,
			Level:    1,
			ParentID: 3,
			Content: `# Cross-Platform Sync: PC & MacBook 🚀

> [!IMPORTANT]
> Keep your personal Windows PC and work MacBook synchronized effortlessly via GitLab CI/CD.

### The Automated Workflow
1. **Push from Home PC:** Run 'git push' after adding features or tweaking your layout.
2. **GitLab Cloud Runner:** Automatically cross-compiles both Windows ('tome.exe') and macOS ('tome-darwin-arm64').
3. **One-Line Pull on Mac:** Grab the binary directly without needing developer tools installed on your work laptop.`,
		},
		{
			ID:       5,
			Title:    "Journal",
			IsFolder: true,
			Expanded: false,
			Level:    0,
			ParentID: -1,
			ChildIDs: []int{6},
		},
		{
			ID:       6,
			Title:    "2026-10-24 Daily Reflection.md",
			IsFolder: false,
			Level:    1,
			ParentID: 5,
			Content: `# 2026-10-24 Daily Reflection ☕

Decided on the name **Tome**. 
Designing the sliding AI research assistant drawer with smooth overlay mechanics.`,
		},
	}
}

func initialModel() Model {
	items := sampleData()

	ti := textinput.New()
	ti.Placeholder = "Ask about your notes... (e.g. 'What did we decide on sync?')"
	ti.CharLimit = 160
	ti.Prompt = "✦ > "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColorFlameOrange).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ui.ColorTextBright)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Italic(true)

	m := Model{
		allItems:    items,
		selectedIdx: 1,
		activePane:  0,
		aiInput:     ti,
		aiState:     "idle",
		statusMsg:   "Welcome to Tome • Press [Ctrl+K] for Cloud AI Assistant",
	}
	m.rebuildVisible()
	return m
}

func (m *Model) rebuildVisible() {
	var visible []int
	for i, it := range m.allItems {
		if it.Level == 0 {
			visible = append(visible, i)
			if it.IsFolder && it.Expanded {
				for _, childID := range it.ChildIDs {
					for ci, candidate := range m.allItems {
						if candidate.ID == childID {
							visible = append(visible, ci)
						}
					}
				}
			}
		}
	}
	m.visibleIndices = visible
	if m.selectedIdx >= len(m.visibleIndices) {
		m.selectedIdx = max(0, len(m.visibleIndices)-1)
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		m.recalculateViewport()
		m.ready = true

	// Slide-up animation tick
	case slideTickMsg:
		if m.sliding {
			if m.slideDir > 0 {
				// Sliding UP
				m.animHeight += 4
				if m.animHeight >= m.targetHeight {
					m.animHeight = m.targetHeight
					m.sliding = false
					m.aiInput.Focus()
				} else {
					cmds = append(cmds, slideTick())
				}
			} else {
				// Sliding DOWN
				m.animHeight -= 4
				if m.animHeight <= 0 {
					m.animHeight = 0
					m.sliding = false
					m.aiOpen = false
					m.aiInput.Blur()
				} else {
					cmds = append(cmds, slideTick())
				}
			}
		}

	// Arcane Rune/Ember Spinner Tick
	case animTickMsg:
		if m.aiState == "thinking" {
			m.aiAnimStep++
			cmds = append(cmds, animTick())
		}

	case aiStartStreamMsg:
		m.aiState = "streaming"
		m.aiStreamIdx = 0
		if len(m.aiStreamList) > 0 {
			cmds = append(cmds, streamToken(m.aiStreamList[0], 35*time.Millisecond))
		}

	case aiStreamMsg:
		m.aiResponse += string(msg)
		m.aiStreamIdx++
		if m.aiStreamIdx < len(m.aiStreamList) {
			cmds = append(cmds, streamToken(m.aiStreamList[m.aiStreamIdx], 30*time.Millisecond))
		} else {
			m.aiState = "idle"
		}

	case tea.KeyMsg:
		// If AI drawer is open, key events are captured by the AI drawer
		if m.aiOpen && !m.sliding {
			switch msg.String() {
			case "esc", "ctrl+k":
				// Start slide-down close
				m.sliding = true
				m.slideDir = -1
				return m, slideTick()

			case "enter":
				query := strings.TrimSpace(m.aiInput.Value())
				if query != "" && m.aiState == "idle" {
					m.aiQuery = query
					m.aiResponse = ""
					m.aiState = "thinking"
					m.aiAnimStep = 0
					m.aiInput.SetValue("")

					// Build smart contextual response tokens based on query
					m.aiStreamList = buildAIResponse(query)

					// Trigger thinking spinner and delay streaming start by 500ms
					cmds = append(cmds, animTick(), tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
						return aiStartStreamMsg{}
					}))
					return m, tea.Batch(cmds...)
				}
			}

			// Forward typing to text input
			var tiCmd tea.Cmd
			m.aiInput, tiCmd = m.aiInput.Update(msg)
			cmds = append(cmds, tiCmd)
			return m, tea.Batch(cmds...)
		}

		// Main navigation keys (when AI drawer is closed)
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.activePane = 1 - m.activePane

		case "up", "k", "w":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				m.updateViewportPreview()
			}

		case "down", "j", "s":
			if m.selectedIdx < len(m.visibleIndices)-1 {
				m.selectedIdx++
				m.updateViewportPreview()
			}

		case "enter", "space":
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
				actualIdx := m.visibleIndices[m.selectedIdx]
				if m.allItems[actualIdx].IsFolder {
					m.allItems[actualIdx].Expanded = !m.allItems[actualIdx].Expanded
					m.rebuildVisible()
				}
			}

		case "ctrl+k":
			// Open AI drawer with slide-up animation
			m.aiOpen = true
			m.sliding = true
			m.slideDir = 1
			m.animHeight = 4
			m.targetHeight = max(12, int(float64(m.height)*0.65))
			m.aiInput.Width = max(20, m.width-12)
			return m, slideTick()
		}

	case tea.MouseMsg:
		if m.aiOpen {
			// Don't scroll background when drawer is open
			return m, nil
		}

		usableWidth := max(20, m.width-2)
		leftTotalWidth := max(34, min(52, usableWidth*36/100))

		switch msg.Type {
		case tea.MouseWheelUp:
			if msg.X < leftTotalWidth {
				if m.selectedIdx > 0 {
					m.selectedIdx--
					m.updateViewportPreview()
				}
			} else {
				m.viewport.LineUp(3)
			}

		case tea.MouseWheelDown:
			if msg.X < leftTotalWidth {
				if m.selectedIdx < len(m.visibleIndices)-1 {
					m.selectedIdx++
					m.updateViewportPreview()
				}
			} else {
				m.viewport.LineDown(3)
			}

		case tea.MouseLeft:
			clickedRow := msg.Y - 3
			if msg.X < leftTotalWidth && clickedRow >= 0 && clickedRow < len(m.visibleIndices) {
				if m.selectedIdx == clickedRow {
					actualIdx := m.visibleIndices[clickedRow]
					if m.allItems[actualIdx].IsFolder {
						m.allItems[actualIdx].Expanded = !m.allItems[actualIdx].Expanded
						m.rebuildVisible()
					}
				} else {
					m.selectedIdx = clickedRow
					m.updateViewportPreview()
				}
			}
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) recalculateViewport() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	headerHeight := 2
	footerHeight := 1
	// The background always renders at FULL height (no squishing!)
	panesHeight := max(4, m.height-headerHeight-footerHeight)

	usableWidth := max(20, m.width-2)
	leftTotalWidth := max(34, min(52, usableWidth*36/100))
	rightTotalWidth := max(16, usableWidth-leftTotalWidth)

	viewportWidth := max(10, rightTotalWidth-4)
	viewportHeight := max(2, panesHeight-2)

	if !m.ready {
		m.viewport = viewport.New(viewportWidth, viewportHeight)
	} else {
		m.viewport.Width = viewportWidth
		m.viewport.Height = viewportHeight
	}
	m.viewport.SetContent(m.renderActivePreview(viewportWidth))
}

func (m *Model) updateViewportPreview() {
	if m.viewport.Width > 0 {
		m.viewport.SetContent(m.renderActivePreview(m.viewport.Width))
		m.viewport.GotoTop()
	}
}

func (m Model) renderActivePreview(width int) string {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.visibleIndices) {
		return "Select a note to preview."
	}
	actualIdx := m.visibleIndices[m.selectedIdx]
	item := m.allItems[actualIdx]

	if item.IsFolder {
		return fmt.Sprintf("\n  📁 **Folder: %s**\n\n  Contains %d notes. Press [Enter] or click to toggle open/close.",
			item.Title, len(item.ChildIDs))
	}

	return markdown.RenderDocument(item.Content, width)
}

func truncateWithDots(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return strings.Repeat(".", maxWidth)
	}
	targetWidth := maxWidth - 3
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > targetWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}

// buildAIResponse generates contextual responses simulating cloud AI research
func buildAIResponse(query string) []string {
	q := strings.ToLower(query)

	if strings.Contains(q, "sync") || strings.Contains(q, "mac") || strings.Contains(q, "laptop") || strings.Contains(q, "git") {
		return []string{
			"✦ **Found 1 primary reference** in your knowledge base:\n",
			"  • `Codex/Cross-Platform Sync.md`\n\n",
			"**Synthesized Guidance**:\n",
			"• **Single Pipeline:** Push code from your home Windows PC to your private GitLab repo.\n",
			"• **Automated Runner:** GitLab CI/CD builds both `tome.exe` and `tome-darwin-arm64` in parallel.\n",
			"• **Zero Dependencies:** On your work MacBook, you download the compiled binary without installing Go or build tools.\n",
		}
	}

	if strings.Contains(q, "arch") || strings.Contains(q, "speed") || strings.Contains(q, "perf") || strings.Contains(q, "go") {
		return []string{
			"✦ **Found 1 primary reference** in your knowledge base:\n",
			"  • `Grimoire/The Arcane Architecture.md`\n\n",
			"**Architecture Review**:\n",
			"• **Sub-10ms Startup:** Compiled Go binary loads instantly compared to script interpretation.\n",
			"• **60 FPS Fluidity:** Bubbletea event loop delivers rock-solid Yazi-style hover mouse wheel navigation.\n",
			"• **Headless Cloud AI:** Goroutines stream Gemini/Claude tokens in the background without UI stutters.\n",
		}
	}

	return []string{
		fmt.Sprintf("✦ **Researching across all notes for:** \"%s\"\n\n", query),
		"• Scanned `Grimoire/`, `Codex/`, and `Journal/`.\n",
		"• Identified key principles: strict viewport containment, 24-bit TrueColor flame palette, and zero runtime dependencies.\n",
		"• Ready to connect to your live Gemini / Claude API key for deep cross-note semantic synthesis.\n",
	}
}

func (m Model) View() string {
	if !m.ready || m.width <= 0 || m.height <= 0 {
		return "\n  Summoning Tome..."
	}

	usableWidth := max(20, m.width-2)
	headerHeight := 2
	footerHeight := 1
	panesHeight := max(4, m.height-headerHeight-footerHeight)
	panesContentHeight := panesHeight - 2

	leftTotalWidth := max(34, min(52, usableWidth*36/100))
	rightTotalWidth := max(16, usableWidth-leftTotalWidth)

	// 1. Header Banner
	titleGradient := ui.Gradient("❖  T O M E", "#FF4500", "#FFD166")
	bannerLeft := fmt.Sprintf("✦ %s ❖ %s", titleGradient, ui.SubtitleStyle.Render("The Arcane Terminal Notebook"))

	wsBadge := ui.BadgeStyle.Render("󰉋 ~/Notes")
	aiBadge := ui.ActiveBadgeStyle.Render("✦ Cloud AI: Online")
	bannerRight := fmt.Sprintf("%s %s", wsBadge, aiBadge)

	leftLen := lipgloss.Width(bannerLeft)
	rightLen := lipgloss.Width(bannerRight)
	gap := max(1, usableWidth-leftLen-rightLen-2)
	row1 := " " + bannerLeft + strings.Repeat(" ", gap) + bannerRight
	row2 := lipgloss.NewStyle().Foreground(ui.ColorBorderMuted).Render(strings.Repeat("─", usableWidth))
	headerBlock := lipgloss.JoinVertical(lipgloss.Left, row1, row2)

	// 2. Tree Pane (Left)
	innerLeftWidth := leftTotalWidth - 4
	var treeLines []string
	availTitleWidth := max(4, innerLeftWidth-2)
	maxVisibleRows := panesContentHeight

	for i, visIdx := range m.visibleIndices {
		if len(treeLines) >= maxVisibleRows {
			break
		}
		item := m.allItems[visIdx]

		treeBranch := ""
		if item.Level > 0 {
			treeBranch = "└─ "
		}

		icon := "󰈙 "
		if item.IsFolder {
			if item.Expanded {
				icon = " "
			} else {
				icon = " "
			}
		}

		rawTitle := truncateWithDots(treeBranch+icon+item.Title, availTitleWidth)
		noBreakTitle := strings.ReplaceAll(rawTitle, " ", "\u00a0")

		if i == m.selectedIdx {
			cursor := ui.TreeCursor.Render("▌") + "\u00a0"
			titlePill := ui.TreeSelectedText.Render(noBreakTitle)
			treeLines = append(treeLines, cursor+titlePill)
		} else {
			indent := "\u00a0\u00a0"
			var titleText string
			if item.IsFolder {
				titleText = ui.TreeNormalFolder.Render(noBreakTitle)
			} else {
				titleText = ui.TreeNormalItem.Render(noBreakTitle)
			}
			treeLines = append(treeLines, indent+titleText)
		}
	}

	leftStyle := ui.PaneStyle
	if m.activePane == 0 {
		leftStyle = ui.ActivePaneStyle
	}
	leftView := leftStyle.
		Width(leftTotalWidth - 4).
		Height(panesContentHeight).
		Render(strings.Join(treeLines, "\n"))

	// 3. Preview Pane (Right)
	rightStyle := ui.PaneStyle
	if m.activePane == 1 {
		rightStyle = ui.ActivePaneStyle
	}
	rightView := rightStyle.
		Width(rightTotalWidth - 4).
		Height(panesContentHeight).
		Render(m.viewport.View())

	// 4. Combine Full Background (Undisturbed!)
	panesView := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)

	// 5. Footer Bar
	var footer string
	if m.aiOpen {
		kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Ask AI  ")
		kClose := ui.FooterKeyStyle.Render("[Esc/Ctrl+K]") + ui.FooterDescStyle.Render(" Close Assistant  ")
		kQuit := ui.FooterKeyStyle.Render("[Ctrl+C]") + ui.FooterDescStyle.Render(" Quit")
		footer = " " + kEnter + kClose + kQuit
	} else {
		kNav := ui.FooterKeyStyle.Render("[↑/↓]") + ui.FooterDescStyle.Render(" Navigate  ")
		kExp := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Toggle Folder  ")
		kAi := ui.FooterKeyStyle.Render("[Ctrl+K]") + ui.FooterDescStyle.Render(" Cloud AI  ")
		kTab := ui.FooterKeyStyle.Render("[Tab]") + ui.FooterDescStyle.Render(" Switch Pane  ")
		kQuit := ui.FooterKeyStyle.Render("[Q]") + ui.FooterDescStyle.Render(" Quit")
		footer = " " + kNav + kExp + kAi + kTab + kQuit
	}
	if lipgloss.Width(footer) > usableWidth {
		footer = footer[:usableWidth]
	}

	// 6. Base Full-Screen Background
	fullScreen := lipgloss.JoinVertical(lipgloss.Left, headerBlock, panesView, footer)
	bgLines := strings.Split(fullScreen, "\n")

	// 7. Layered Sliding AI Drawer (Overlays directly on top of background!)
	if m.aiOpen && m.animHeight > 0 {
		drawerInnerWidth := usableWidth - 4
		drawerContentHeight := max(2, m.animHeight-2)

		// Header of the drawer (strictly fixed and static - no spinner in title bar)
		statusText := "✦ TOME KNOWLEDGE RESEARCHER ────────────────────────────── [Esc: Close]"
		drawerTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorAmberGold).
			Render(statusText)

		// Question input prompt
		m.aiInput.Width = max(20, drawerInnerWidth-8)
		inputRow := m.aiInput.View()

		// Answer area
		var answerArea string
		if m.aiResponse != "" {
			answerArea = m.aiResponse
		} else if m.aiQuery != "" && m.aiState == "thinking" {
			answerArea = "\n  Communing with the Grimoire and researching your notes..."
		} else {
			answerArea = ui.SubtitleStyle.Render("\n  Type any question about your notes above and press [Enter] to research.")
		}

		// Stack drawer content
		drawerContent := fmt.Sprintf("%s\n\n%s\n%s", drawerTitle, inputRow, answerArea)

		// Strictly clamp lines so content height is perfectly locked
		rawLines := strings.Split(drawerContent, "\n")
		var clampedLines []string
		innerContentWidth := max(10, drawerInnerWidth-2)

		for idx, l := range rawLines {
			if idx >= drawerContentHeight {
				break
			}
			if lipgloss.Width(l) > innerContentWidth {
				l = truncateWithDots(l, innerContentWidth)
			}
			// Pad line with spaces so drawer is opaque over background
			pad := max(0, innerContentWidth-lipgloss.Width(l))
			clampedLines = append(clampedLines, l+strings.Repeat(" ", pad))
		}
		for len(clampedLines) < drawerContentHeight {
			clampedLines = append(clampedLines, strings.Repeat(" ", innerContentWidth))
		}

		aiBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ui.ColorFlameOrange).
			Background(ui.ColorDarkSlate).
			Padding(0, 1)

		aiDrawerRendered := aiBoxStyle.
			Width(drawerInnerWidth).
			Height(drawerContentHeight).
			MaxHeight(m.animHeight).
			Render(strings.Join(clampedLines, "\n"))

		drawerLines := strings.Split(aiDrawerRendered, "\n")
		// Strictly enforce exact height: len(drawerLines) is locked to m.animHeight
		if len(drawerLines) > m.animHeight {
			drawerLines = drawerLines[:m.animHeight]
		}
		for len(drawerLines) < m.animHeight {
			drawerLines = append(drawerLines, strings.Repeat(" ", usableWidth))
		}

		// Overlap calculation: overlay drawer directly over the bottom rows of the background (above footer)
		footerRow := len(bgLines) - 1
		startRow := footerRow - m.animHeight
		if startRow < 2 {
			startRow = 2
		}

		for i, dLine := range drawerLines {
			targetRow := startRow + i
			if targetRow >= 0 && targetRow < footerRow {
				bgLines[targetRow] = dLine
			}
		}
	}

	// 8. Output lines strictly clamped to terminal height
	if len(bgLines) > m.height && m.height > 2 {
		bgLines = bgLines[:m.height]
	}

	return strings.Join(bgLines, "\n")
}

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),       // Dedicated alternate screen buffer
		tea.WithMouseCellMotion(), // Rich hover & wheel mouse capture
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error starting Tome: %v\n", err)
	}
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
