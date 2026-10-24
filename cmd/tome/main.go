package main

import (
	"fmt"
	"strings"
	"time"

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

	// AI Research Assistant Drawer
	aiOpen       bool
	aiQuery      string
	aiResponse   string
	aiStreaming  bool
	aiTokenIndex int

	// Status messages
	statusMsg string
}

type aiTokenMsg string
type aiDoneMsg struct{}

func streamNextToken(token string, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return aiTokenMsg(token)
	})
}

var sampleAIStream = []string{
	"Searching knowledge base across all active notebooks...\n\n",
	"✦ **Found 3 relevant references** in your workspace:\n",
	"  • `Grimoire/The Arcane Architecture.md` (High relevance)\n",
	"  • `Codex/Cross-Platform Sync.md`\n",
	"  • `Journal/2026-10-24 Daily Reflection.md`\n\n",
	"**Synthesis & Strategic Findings**:\n",
	"1. **Pixel-Perfect Viewport:** Strict mathematical column/row budgeting prevents border breakage.\n",
	"2. **Subtle Aesthetic:** Selection cursor uses an elegant left accent bar (`▌`) instead of abrasive blocks.\n",
	"3. **Multi-Platform Zero Setup:** Seamless compilation for Windows and Apple Silicon Mac via GitLab.\n\n",
	"*(Press [Esc] or [A] to close this drawer)*",
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
Polishing the UI: spacious browsing tree, zero line-wrapping, and smooth truncation with ellipsis.`,
		},
	}
}

func initialModel() Model {
	items := sampleData()
	m := Model{
		allItems:    items,
		selectedIdx: 1, // Start on first note (The Arcane Architecture.md)
		activePane:  0,
		statusMsg:   "Welcome to Tome • Press [A] for Cloud AI Assistant",
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

	case aiTokenMsg:
		m.aiResponse += string(msg)
		m.aiTokenIndex++
		if m.aiTokenIndex < len(sampleAIStream) {
			return m, streamNextToken(sampleAIStream[m.aiTokenIndex], 45*time.Millisecond)
		}
		m.aiStreaming = false

	case tea.KeyMsg:
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
			// Toggle folder expand/collapse if folder
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
				actualIdx := m.visibleIndices[m.selectedIdx]
				if m.allItems[actualIdx].IsFolder {
					m.allItems[actualIdx].Expanded = !m.allItems[actualIdx].Expanded
					m.rebuildVisible()
				}
			}

		case "a":
			// Toggle AI Assistant Drawer
			m.aiOpen = !m.aiOpen
			if m.aiOpen {
				m.aiResponse = ""
				m.aiTokenIndex = 0
				m.aiStreaming = true
				m.recalculateViewport()
				return m, streamNextToken(sampleAIStream[0], 60*time.Millisecond)
			} else {
				m.recalculateViewport()
			}

		case "esc":
			if m.aiOpen {
				m.aiOpen = false
				m.recalculateViewport()
			}
		}

	case tea.MouseMsg:
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
			// Row 0: Title bar, Row 1: Divider, Row 2: Top Border of Pane
			// Item 0 begins at Row 3
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
	aiHeight := 0
	if m.aiOpen {
		aiHeight = max(6, min(10, m.height/3))
	}
	// Panes fill the exact remaining vertical space down to the footer
	panesHeight := max(4, m.height-headerHeight-footerHeight-aiHeight)

	usableWidth := max(20, m.width-2)
	leftTotalWidth := max(34, min(52, usableWidth*36/100))
	rightTotalWidth := max(16, usableWidth-leftTotalWidth)

	// Inner content dimensions (Borders take 2 vertical rows: top & bottom)
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

// truncateWithDots truncates string with "..." strictly measuring terminal visual columns
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

func (m Model) View() string {
	if !m.ready || m.width <= 0 || m.height <= 0 {
		return "\n  Summoning Tome..."
	}

	// 1. Exact Screen Height Budgeting
	headerHeight := 2
	footerHeight := 1
	aiHeight := 0
	if m.aiOpen {
		aiHeight = max(6, min(10, m.height/3))
	}
	// PanesHeight exactly spans from header to footer: Header(2) + PanesHeight + AI(aiHeight) + Footer(1) == m.height
	panesHeight := max(4, m.height-headerHeight-footerHeight-aiHeight)
	panesContentHeight := panesHeight - 2 // Top border (1) + bottom border (1) = 2

	usableWidth := max(20, m.width-2) // 2-char margin prevents auto-wrap
	leftTotalWidth := max(34, min(52, usableWidth*36/100))
	rightTotalWidth := max(16, usableWidth-leftTotalWidth)

	// 2. Header Banner (Exactly 2 rows)
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

	// 3. Tree Pane (Left)
	innerLeftWidth := leftTotalWidth - 4 // Content width inside borders & padding
	var treeLines []string

	availTitleWidth := max(4, innerLeftWidth-2)
	maxVisibleRows := panesContentHeight

	for i, visIdx := range m.visibleIndices {
		if len(treeLines) >= maxVisibleRows {
			break // Strictly ensure treeLines never exceeds pane content height
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

		// Truncate cleanly with "..." if title exceeds available columns
		rawTitle := truncateWithDots(treeBranch+icon+item.Title, availTitleWidth)
		// Crucial: Replace spaces with non-breaking spaces (\u00a0) so Lipgloss NEVER word-wraps!
		noBreakTitle := strings.ReplaceAll(rawTitle, " ", "\u00a0")

		if i == m.selectedIdx {
			// Single unbreakable line: Flame orange cursor bar + text
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
	// Content height is panesContentHeight. With 2 border rows, total rendered box is exactly panesHeight!
	leftView := leftStyle.
		Width(leftTotalWidth - 4).
		Height(panesContentHeight).
		Render(strings.Join(treeLines, "\n"))

	// 4. Preview Pane (Right)
	rightStyle := ui.PaneStyle
	if m.activePane == 1 {
		rightStyle = ui.ActivePaneStyle
	}
	rightView := rightStyle.
		Width(rightTotalWidth - 4).
		Height(panesContentHeight).
		Render(m.viewport.View())

	// Horizontal side-by-side join (total height = panesHeight)
	panesView := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)

	// 5. AI Research Drawer (if active)
	var aiView string
	if m.aiOpen {
		statusIndicator := "✦ TOME AI RESEARCHER (Headless Cloud) ─ [Streaming...]"
		if !m.aiStreaming {
			statusIndicator = "✦ TOME AI RESEARCHER (Headless Cloud) ─ [Ready]  (Esc to close)"
		}
		titleBar := lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorAmberGold).
			Render(statusIndicator)

		aiContentHeight := max(2, aiHeight-2)
		aiView = ui.AIDrawerStyle.
			Width(usableWidth - 4).
			Height(aiContentHeight).
			Render(titleBar + "\n" + m.aiResponse)
	}

	// 6. 1-Row Footer (Affixed to the bottom of the screen)
	kNav := ui.FooterKeyStyle.Render("[↑/↓]") + ui.FooterDescStyle.Render(" Navigate  ")
	kExp := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Toggle Folder  ")
	kAi := ui.FooterKeyStyle.Render("[A]") + ui.FooterDescStyle.Render(" Cloud AI  ")
	kTab := ui.FooterKeyStyle.Render("[Tab]") + ui.FooterDescStyle.Render(" Switch Pane  ")
	kQuit := ui.FooterKeyStyle.Render("[Q]") + ui.FooterDescStyle.Render(" Quit")
	footer := " " + kNav + kExp + kAi + kTab + kQuit
	if lipgloss.Width(footer) > usableWidth {
		footer = footer[:usableWidth]
	}

	// 7. Compose View
	var fullScreen string
	if m.aiOpen {
		fullScreen = lipgloss.JoinVertical(lipgloss.Left, headerBlock, panesView, aiView, footer)
	} else {
		fullScreen = lipgloss.JoinVertical(lipgloss.Left, headerBlock, panesView, footer)
	}

	// 8. Absolute Guarantee: Exactly fits m.height without emitting extra newlines
	lines := strings.Split(fullScreen, "\n")
	if len(lines) > m.height && m.height > 2 {
		lines = lines[:m.height]
		fullScreen = strings.Join(lines, "\n")
	}

	return fullScreen
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
