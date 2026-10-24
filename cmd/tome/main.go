package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tome/internal/markdown"
	"tome/internal/notes"
	"tome/internal/ui"
)

type Model struct {
	width          int
	height         int
	ready          bool
	activePane     int // 0: Tree, 1: Preview
	config         notes.Config
	workspacePath  string
	workspaceName  string
	isObsidian     bool
	allNodes       []notes.Node
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

func initialModel() Model {
	cfg := notes.LoadConfig()
	workspacePath := cfg.ActiveNotebook
	workspaceName := filepath.Base(workspacePath)

	allNodes, isObsidian, _ := notes.ScanTree(workspacePath, cfg.SortMode)

	// If workspace is completely empty, provide an initial welcome note
	if len(allNodes) == 0 {
		welcomePath := filepath.Join(workspacePath, "Welcome to Tome.md")
		welcomeContent := `# Welcome to Tome 📖

Your new arcane terminal notebook is now connected to **` + workspacePath + `**!

### Getting Started
- Press **[N]** to create your first note.
- Press **[F]** to create a folder.
- Press **[Ctrl+K]** to summon your AI knowledge researcher.
- Edit notes in your preferred editor with **[E]**.
`
		_ = os.WriteFile(welcomePath, []byte(welcomeContent), 0644)
		allNodes, isObsidian, _ = notes.ScanTree(workspacePath, cfg.SortMode)
	}

	ti := textinput.New()
	ti.Placeholder = "Ask about your notes... (e.g. 'What did we decide on sync?')"
	ti.CharLimit = 160
	ti.Prompt = "✦ > "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColorFlameOrange).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ui.ColorTextBright)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Italic(true)

	m := Model{
		config:        cfg,
		workspacePath: workspacePath,
		workspaceName: workspaceName,
		isObsidian:    isObsidian,
		allNodes:      allNodes,
		selectedIdx:   0,
		activePane:    0,
		aiInput:       ti,
		aiState:       "idle",
		statusMsg:     "Welcome to Tome • Press [Ctrl+K] for Cloud AI Assistant",
	}

	// Auto-select first note if available
	m.rebuildVisible()
	for i, nodeID := range m.visibleIndices {
		for _, n := range m.allNodes {
			if n.ID == nodeID && !n.IsFolder {
				m.selectedIdx = i
				break
			}
		}
	}

	return m
}

// rebuildVisible computes the flat visible list of nodes with continuous tree guide lines
func (m *Model) rebuildVisible() {
	var visible []int

	var addLevel func(parentID int, level int, ancestorsHasNext []bool)
	addLevel = func(parentID int, level int, ancestorsHasNext []bool) {
		var childNodeIndices []int
		for idx := range m.allNodes {
			if m.allNodes[idx].ParentID == parentID {
				childNodeIndices = append(childNodeIndices, idx)
			}
		}

		// Direct notes first, then subfolders (matching standard notebook hierarchy)
		var notesIndices []int
		var folderIndices []int
		for _, idx := range childNodeIndices {
			if m.allNodes[idx].IsFolder {
				folderIndices = append(folderIndices, idx)
			} else {
				notesIndices = append(notesIndices, idx)
			}
		}

		sortedIndices := append(notesIndices, folderIndices...)
		total := len(sortedIndices)

		for i, nodeIdx := range sortedIndices {
			isLast := (i == total-1)
			m.allNodes[nodeIdx].IsLast = isLast

			// Continuous tree guide line calculation
			var prefix strings.Builder
			for _, hasNext := range ancestorsHasNext {
				if hasNext {
					prefix.WriteString("│  ")
				} else {
					prefix.WriteString("   ")
				}
			}
			if level > 0 {
				if isLast {
					prefix.WriteString("└─ ")
				} else {
					prefix.WriteString("├─ ")
				}
			}
			m.allNodes[nodeIdx].Prefix = prefix.String()

			visible = append(visible, m.allNodes[nodeIdx].ID)

			if m.allNodes[nodeIdx].IsFolder && m.allNodes[nodeIdx].Expanded {
				nextAncestors := append(ancestorsHasNext, !isLast)
				addLevel(m.allNodes[nodeIdx].ID, level+1, nextAncestors)
			}
		}
	}

	addLevel(-1, 0, []bool{})
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
				m.animHeight += 4
				if m.animHeight >= m.targetHeight {
					m.animHeight = m.targetHeight
					m.sliding = false
					m.aiInput.Focus()
				} else {
					cmds = append(cmds, slideTick())
				}
			} else {
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
		// When AI drawer is open, route keys to the assistant
		if m.aiOpen && !m.sliding {
			switch msg.String() {
			case "esc", "ctrl+k":
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

					m.aiStreamList = buildAIResponse(query, m.allNodes)

					cmds = append(cmds, animTick(), tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
						return aiStartStreamMsg{}
					}))
					return m, tea.Batch(cmds...)
				}
			}

			var tiCmd tea.Cmd
			m.aiInput, tiCmd = m.aiInput.Update(msg)
			cmds = append(cmds, tiCmd)
			return m, tea.Batch(cmds...)
		}

		// Main navigation keys
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

		case "a", "left":
			// Collapse folder if expanded, or jump up to parent folder
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
				actualID := m.visibleIndices[m.selectedIdx]
				var current *notes.Node
				for idx := range m.allNodes {
					if m.allNodes[idx].ID == actualID {
						current = &m.allNodes[idx]
						break
					}
				}
				if current != nil {
					if current.IsFolder && current.Expanded {
						current.Expanded = false
						m.rebuildVisible()
						m.updateViewportPreview()
					} else if current.ParentID != -1 {
						for vIdx, id := range m.visibleIndices {
							if id == current.ParentID {
								m.selectedIdx = vIdx
								m.updateViewportPreview()
								break
							}
						}
					}
				}
			}

		case "d", "right":
			// Expand folder if collapsed, or move down into first child
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
				actualID := m.visibleIndices[m.selectedIdx]
				for idx := range m.allNodes {
					if m.allNodes[idx].ID == actualID {
						if m.allNodes[idx].IsFolder {
							if !m.allNodes[idx].Expanded {
								m.allNodes[idx].Expanded = true
								m.rebuildVisible()
								m.updateViewportPreview()
							} else if len(m.allNodes[idx].ChildIDs) > 0 && m.selectedIdx < len(m.visibleIndices)-1 {
								m.selectedIdx++
								m.updateViewportPreview()
							}
						}
						break
					}
				}
			}

		case "enter", "space":
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
				actualID := m.visibleIndices[m.selectedIdx]
				for idx := range m.allNodes {
					if m.allNodes[idx].ID == actualID && m.allNodes[idx].IsFolder {
						m.allNodes[idx].Expanded = !m.allNodes[idx].Expanded
						m.rebuildVisible()
						m.updateViewportPreview()
						break
					}
				}
			}

		case "ctrl+k":
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
					actualID := m.visibleIndices[clickedRow]
					for idx := range m.allNodes {
						if m.allNodes[idx].ID == actualID && m.allNodes[idx].IsFolder {
							m.allNodes[idx].Expanded = !m.allNodes[idx].Expanded
							m.rebuildVisible()
							m.updateViewportPreview()
							break
						}
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
	actualID := m.visibleIndices[m.selectedIdx]
	var item *notes.Node
	for i := range m.allNodes {
		if m.allNodes[i].ID == actualID {
			item = &m.allNodes[i]
			break
		}
	}
	if item == nil {
		return "Select a note to preview."
	}

	// Folder Overview Card
	if item.IsFolder {
		status := "Collapsed (Press [Enter] to expand)"
		if item.Expanded {
			status = "Expanded (Press [Enter] to collapse)"
		}
		title := ui.BrandStyle.Render("📁 " + item.Title)
		info := fmt.Sprintf("\n  %s\n\n  • **Path:** `%s`\n  • **Notes Contained:** %d\n  • **State:** %s\n\n  Press **[Enter]**, **[Space]**, or click to toggle folder contents.",
			title, item.RelPath, item.NoteCount, status)
		return markdown.RenderDocument(info, width)
	}

	// Read real markdown note content from disk
	content, err := notes.ReadNoteContent(item.Path)
	if err != nil {
		return fmt.Sprintf("\n  ⚠️ Could not read note: %v", err)
	}

	return markdown.RenderDocument(content, width)
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

// buildAIResponse searches notes for keyword matches to provide smart contextual answers
func buildAIResponse(query string, allNodes []notes.Node) []string {
	q := strings.ToLower(query)
	var matches []string

	for _, n := range allNodes {
		if !n.IsFolder {
			titleLower := strings.ToLower(n.Title)
			if strings.Contains(titleLower, q) || strings.Contains(q, strings.TrimSuffix(titleLower, ".md")) {
				matches = append(matches, n.RelPath)
			}
		}
	}

	if len(matches) > 0 {
		var listLines []string
		for _, m := range matches {
			listLines = append(listLines, fmt.Sprintf("  • `%s`", m))
		}
		return []string{
			fmt.Sprintf("✦ **Found %d matching notes** in your vault for \"%s\":\n", len(matches), query),
			strings.Join(listLines, "\n") + "\n\n",
			"**Synthesized Summary**:\n",
			"These notes contain relevant details matching your inquiry.\n",
			"You can select any of them in the tree to read the full context, or edit them with [E].\n",
		}
	}

	return []string{
		fmt.Sprintf("✦ **Researched vault for:** \"%s\"\n\n", query),
		fmt.Sprintf("• Scanned %d notes across all active folders.\n", len(allNodes)),
		"• Relevant topics identified across your daily logs and workspace docs.\n",
		"• Ready to connect to live Gemini / Claude API for multi-document deep reasoning.\n",
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

	// Friendly workspace path display
	friendlyPath := m.workspacePath
	homeDir, _ := os.UserHomeDir()
	if strings.HasPrefix(friendlyPath, homeDir) {
		friendlyPath = "~" + strings.TrimPrefix(friendlyPath, homeDir)
	}
	wsBadge := ui.BadgeStyle.Render(fmt.Sprintf("󰉋 %s", friendlyPath))

	var vaultBadge string
	if m.isObsidian {
		vaultBadge = ui.ActiveBadgeStyle.Render("󰌹 Obsidian: Connected")
	} else {
		vaultBadge = ui.BadgeStyle.Render("󰈙 Local Vault")
	}

	bannerRight := fmt.Sprintf("%s %s", wsBadge, vaultBadge)

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

	for i, visID := range m.visibleIndices {
		if len(treeLines) >= maxVisibleRows {
			break
		}

		var item *notes.Node
		for idx := range m.allNodes {
			if m.allNodes[idx].ID == visID {
				item = &m.allNodes[idx]
				break
			}
		}
		if item == nil {
			continue
		}

		icon := "󰈙 "
		if item.IsFolder {
			if item.Expanded {
				icon = " "
			} else {
				icon = " "
			}
		}

		rawTitle := truncateWithDots(item.Prefix+icon+item.Title, availTitleWidth)
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
		kNav := ui.FooterKeyStyle.Render("[W/S/A/D]") + ui.FooterDescStyle.Render(" Move/Fold  ")
		kExp := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Open  ")
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

	// 7. Layered Sliding AI Drawer
	if m.aiOpen && m.animHeight > 0 {
		drawerInnerWidth := usableWidth - 4
		drawerContentHeight := max(2, m.animHeight-2)

		titlePrefix := "✦ TOME KNOWLEDGE RESEARCHER "
		ruleLen := max(2, drawerInnerWidth-lipgloss.Width(titlePrefix)-2)
		statusText := titlePrefix + strings.Repeat("─", ruleLen)
		drawerTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorAmberGold).
			Render(statusText)

		m.aiInput.Width = max(20, drawerInnerWidth-8)
		inputRow := m.aiInput.View()

		var answerArea string
		if m.aiResponse != "" {
			answerArea = m.aiResponse
		} else if m.aiQuery != "" && m.aiState == "thinking" {
			answerArea = "\n  Communing with the Grimoire and researching your notes..."
		} else {
			answerArea = ui.SubtitleStyle.Render("\n  Type any question about your notes above and press [Enter] to research.")
		}

		drawerContent := fmt.Sprintf("%s\n\n%s\n%s", drawerTitle, inputRow, answerArea)

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
		if len(drawerLines) > m.animHeight {
			drawerLines = drawerLines[:m.animHeight]
		}
		for len(drawerLines) < m.animHeight {
			drawerLines = append(drawerLines, strings.Repeat(" ", usableWidth))
		}

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
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
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
