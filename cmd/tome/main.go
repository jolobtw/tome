package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
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

	// In-place Note Editor
	inEditMode     bool
	editor         textarea.Model
	activeNotePath string
	saveStatus     string
	lastClickTime  time.Time

	// Floating Modal System (New Note, New Folder, Quick Log)
	modalType     string // "", "new_note", "new_folder", "quick_log"
	modalInput    textinput.Model
	modalDestPath string

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

	ta := textarea.New()
	ta.Placeholder = "Write your note here using Markdown..."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.MaxHeight = 0 // Unlimited lines: allows Enter to insert newlines regardless of note length
	ta.EndOfBufferCharacter = ' '

	// Theme-matched styling with NO background color (prevents adaptive light-mode white highlighting!)
	editorStyle := textarea.Style{
		Base:             lipgloss.NewStyle(),
		CursorLine:       lipgloss.NewStyle(), // Clean transparent line, no white block!
		CursorLineNumber: lipgloss.NewStyle().Foreground(ui.ColorFlameOrange).Bold(true),
		EndOfBuffer:      lipgloss.NewStyle().Foreground(ui.ColorBorderMuted),
		LineNumber:       lipgloss.NewStyle().Foreground(ui.ColorBorderMuted),
		Placeholder:      lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Italic(true),
		Prompt:           lipgloss.NewStyle().Foreground(ui.ColorFlameOrange),
		Text:             lipgloss.NewStyle().Foreground(ui.ColorTextRegular),
	}
	ta.FocusedStyle = editorStyle
	ta.BlurredStyle = editorStyle
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(ui.ColorFlameOrange)

	mi := textinput.New()
	mi.Prompt = "✦ > "
	mi.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColorFlameOrange).Bold(true)
	mi.TextStyle = lipgloss.NewStyle().Foreground(ui.ColorTextBright)
	mi.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Italic(true)
	mi.CharLimit = 80

	m := Model{
		config:        cfg,
		workspacePath: workspacePath,
		workspaceName: workspaceName,
		isObsidian:    isObsidian,
		allNodes:      allNodes,
		selectedIdx:   0,
		activePane:    0,
		editor:        ta,
		aiInput:       ti,
		aiState:       "idle",
		modalInput:    mi,
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
			for lvl, hasNext := range ancestorsHasNext {
				if lvl == 0 {
					// Root ancestor: do not indent so branch lines start directly below root folder icon
					continue
				}
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
		// When a floating modal is open, route keys to the modal dialog
		if m.modalType != "" {
			switch msg.String() {
			case "esc":
				m.modalType = ""
				m.modalInput.Blur()
				return m, nil

			case "enter":
				switch m.modalType {
				case "new_note":
					cmd := m.confirmNewNote()
					return m, cmd
				case "new_folder":
					m.confirmNewFolder()
					return m, nil
				case "quick_log":
					m.confirmQuickLog()
					return m, nil
				}
				m.modalType = ""
				m.modalInput.Blur()
				return m, nil
			}

			var tiCmd tea.Cmd
			m.modalInput, tiCmd = m.modalInput.Update(msg)
			return m, tiCmd
		}

		// When in in-place note editor mode, route keys directly to editor
		if m.inEditMode {
			switch msg.String() {
			case "ctrl+s":
				_ = os.WriteFile(m.activeNotePath, []byte(m.editor.Value()), 0644)
				m.saveStatus = "✓ Saved"
				m.updateViewportPreview()
				return m, nil

			case "esc":
				_ = os.WriteFile(m.activeNotePath, []byte(m.editor.Value()), 0644)
				m.inEditMode = false
				m.saveStatus = ""
				m.editor.Blur()
				m.updateViewportPreview()
				return m, nil

			case "ctrl+q":
				m.inEditMode = false
				m.saveStatus = ""
				m.editor.Blur()
				m.updateViewportPreview()
				return m, nil
			}

			var cmd tea.Cmd
			m.editor, cmd = m.editor.Update(msg)
			if m.saveStatus != "" {
				m.saveStatus = "● Modified"
			}
			return m, cmd
		}

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

		case "e":
			// Enter in-place editor mode if selected item is a note
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
				actualID := m.visibleIndices[m.selectedIdx]
				var current *notes.Node
				for idx := range m.allNodes {
					if m.allNodes[idx].ID == actualID {
						current = &m.allNodes[idx]
						break
					}
				}
				if current != nil && !current.IsFolder {
					rawContent, err := notes.ReadNoteContent(current.Path)
					if err == nil {
						// Normalize Windows \r\n to \n to prevent carriage returns from corrupting layout
						normalized := strings.ReplaceAll(rawContent, "\r\n", "\n")
						normalized = strings.ReplaceAll(normalized, "\r", "\n")

						m.inEditMode = true
						m.activePane = 1
						m.activeNotePath = current.Path
						m.saveStatus = ""
						m.editor.SetValue(normalized)
						cmd := m.editor.Focus()
						return m, cmd
					}
				}
			}

		case "n":
			m.openNewNoteModal()
			return m, m.modalInput.Focus()

		case "f":
			m.openNewFolderModal()
			return m, m.modalInput.Focus()

		case "l":
			m.openQuickLogModal()
			return m, m.modalInput.Focus()

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
		if m.modalType != "" || m.aiOpen {
			return m, nil
		}

		usableWidth := max(20, m.width-2)
		leftTotalWidth := max(34, min(52, usableWidth*36/100))

		if m.inEditMode {
			// Translate screen coordinates to local editor coordinates:
			// X: right pane starts at leftTotalWidth; 1 col border + 1 col padding = +2
			// Y: 2 header banner rows + 1 top border row + 1 editor title row = 4 rows
			editorX := msg.X - (leftTotalWidth + 2)
			editorY := msg.Y - 4

			if editorX >= 0 && editorY >= 0 && editorX < m.editor.Width() && editorY < m.editor.Height() {
				localMsg := msg
				localMsg.X = editorX
				localMsg.Y = editorY
				var cmd tea.Cmd
				m.editor, cmd = m.editor.Update(localMsg)
				return m, cmd
			}
			return m, nil
		}

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
				now := time.Now()
				if now.Sub(m.lastClickTime) < 200*time.Millisecond {
					return m, nil
				}
				m.lastClickTime = now

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

	if m.inEditMode {
		var editorCmd tea.Cmd
		m.editor, editorCmd = m.editor.Update(msg)
		cmds = append(cmds, editorCmd)
	} else {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

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
	m.editor.SetWidth(max(10, rightTotalWidth-6))
	m.editor.SetHeight(max(2, panesHeight-3))
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
	availTitleWidth := max(4, innerLeftWidth-4)
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
		noBreakTitle = strings.ReplaceAll(noBreakTitle, "-", "\u2011")

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

	// 3. Preview/Editor Pane (Right)
	rightStyle := ui.PaneStyle
	if m.inEditMode || m.activePane == 1 {
		rightStyle = ui.ActivePaneStyle
	}

	var rightContent string
	if m.inEditMode {
		innerRightWidth := max(10, rightTotalWidth-6)
		noteTitle := filepath.Base(m.activeNotePath)
		bannerPrefix := "✎ EDITING: "
		availTitle := max(4, innerRightWidth-lipgloss.Width(bannerPrefix)-4)
		if lipgloss.Width(noteTitle) > availTitle {
			noteTitle = truncateWithDots(noteTitle, availTitle)
		}

		editorBanner := lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorFlameOrange).
			Render(bannerPrefix + noteTitle)

		ruleWidth := max(2, innerRightWidth-lipgloss.Width(editorBanner)-2)
		divider := lipgloss.NewStyle().
			Foreground(ui.ColorBorderMuted).
			Render(" " + strings.Repeat("─", ruleWidth))

		headerBar := editorBanner + divider
		rawContent := headerBar + "\n" + m.editor.View()

		// Enforce strict line clamping so rightContent never exceeds panesContentHeight
		rLines := strings.Split(rawContent, "\n")
		if len(rLines) > panesContentHeight {
			rLines = rLines[:panesContentHeight]
		}
		rightContent = strings.Join(rLines, "\n")
	} else {
		rightContent = m.viewport.View()
	}

	rightView := rightStyle.
		Width(rightTotalWidth - 4).
		Height(panesContentHeight).
		Render(rightContent)

	// 4. Combine Full Background (Undisturbed!)
	panesView := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)

	// 5. Footer Bar
	var footer string
	if m.modalType != "" {
		kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Confirm  ")
		kClose := ui.FooterKeyStyle.Render("[Esc]") + ui.FooterDescStyle.Render(" Cancel")
		footer = " ✦ Modal Prompt │ " + kEnter + kClose
	} else if m.inEditMode {
		kSave := ui.FooterKeyStyle.Render("[Ctrl+S]") + ui.FooterDescStyle.Render(" Save  ")
		kDone := ui.FooterKeyStyle.Render("[Esc]") + ui.FooterDescStyle.Render(" Save & Finish  ")
		kDiscard := ui.FooterKeyStyle.Render("[Ctrl+Q]") + ui.FooterDescStyle.Render(" Discard")
		var saveBadge string
		if m.saveStatus != "" {
			if m.saveStatus == "✓ Saved" {
				saveBadge = ui.ActiveBadgeStyle.Render(m.saveStatus) + "  "
			} else {
				saveBadge = ui.BadgeStyle.Render(m.saveStatus) + "  "
			}
		}
		footer = " ✎ In-Place Editor │ " + saveBadge + kSave + kDone + kDiscard
	} else if m.aiOpen {
		kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Ask AI  ")
		kClose := ui.FooterKeyStyle.Render("[Esc/Ctrl+K]") + ui.FooterDescStyle.Render(" Close Assistant  ")
		kQuit := ui.FooterKeyStyle.Render("[Ctrl+C]") + ui.FooterDescStyle.Render(" Quit")
		footer = " " + kEnter + kClose + kQuit
	} else {
		kNav := ui.FooterKeyStyle.Render("[W/S/A/D]") + ui.FooterDescStyle.Render(" Move/Fold  ")
		kExp := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Open  ")
		kEdit := ui.FooterKeyStyle.Render("[E]") + ui.FooterDescStyle.Render(" Edit  ")
		kNew := ui.FooterKeyStyle.Render("[N]") + ui.FooterDescStyle.Render(" New Note  ")
		kFolder := ui.FooterKeyStyle.Render("[F]") + ui.FooterDescStyle.Render(" Folder  ")
		kLog := ui.FooterKeyStyle.Render("[L]") + ui.FooterDescStyle.Render(" Quick Log  ")
		kAi := ui.FooterKeyStyle.Render("[Ctrl+K]") + ui.FooterDescStyle.Render(" Cloud AI  ")
		kQuit := ui.FooterKeyStyle.Render("[Q]") + ui.FooterDescStyle.Render(" Quit")
		footer = " " + kNav + kExp + kEdit + kNew + kFolder + kLog + kAi + kQuit
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

	// 8. Layered Floating Modal Overlay (New Note, New Folder, Quick Log)
	if m.modalType != "" {
		modalWidth := min(58, usableWidth-4)
		innerModalWidth := max(10, modalWidth-10)
		var title string
		var subLabel string
		var actionHint string

		homeDir, _ := os.UserHomeDir()
		friendlyDest := m.modalDestPath
		if strings.HasPrefix(friendlyDest, homeDir) {
			friendlyDest = "~" + strings.TrimPrefix(friendlyDest, homeDir)
		}

		switch m.modalType {
		case "new_note":
			title = ui.Gradient("✦  C R E A T E   N E W   N O T E", "#FF4500", "#FFD166")
			subLabel = fmt.Sprintf("󰉋 Saving in: %s", friendlyDest)
			actionHint = "[Enter] Create & Open Editor  •  [Esc] Cancel"
		case "new_folder":
			title = ui.Gradient("✦  C R E A T E   N E W   F O L D E R", "#FF4500", "#FFD166")
			subLabel = fmt.Sprintf("󰉋 Location: %s", friendlyDest)
			actionHint = "[Enter] Create Folder  •  [Esc] Cancel"
		case "quick_log":
			title = ui.Gradient("✦  Q U I C K   D A I L Y   L O G", "#FF4500", "#FFD166")
			today := time.Now().Format("2006-01-02")
			subLabel = fmt.Sprintf("󰈙 Appending to: %s-quick-log.md", today)
			actionHint = "[Enter] Save Log Entry  •  [Esc] Cancel"
		}

		m.modalInput.Width = max(20, innerModalWidth-4)
		inputBox := m.modalInput.View()

		subStyle := ui.SubtitleStyle.Render(subLabel)
		hintStyle := ui.SubtitleStyle.Render(actionHint)

		rule := lipgloss.NewStyle().Foreground(ui.ColorBorderMuted).Render(strings.Repeat("─", innerModalWidth))

		modalContent := lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			subStyle,
			rule,
			"",
			inputBox,
			"",
			rule,
			hintStyle,
		)

		modalBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ui.ColorFlameOrange).
			Padding(1, 2)

		modalRendered := modalBoxStyle.Width(modalWidth - 6).Render(modalContent)
		modalLines := strings.Split(modalRendered, "\n")
		modalHeight := len(modalLines)
		startRow := max(2, (m.height-modalHeight)/2)

		for rIdx, mLine := range modalLines {
			targetRow := startRow + rIdx
			if targetRow >= 0 && targetRow < len(bgLines)-1 {
				bgLines[targetRow] = lipgloss.PlaceHorizontal(usableWidth, lipgloss.Center, mLine)
			}
		}
	}

	// 9. Output lines strictly clamped to terminal height
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

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			sb.WriteRune(r)
		} else if r == ' ' {
			sb.WriteRune('-')
		}
	}
	res := sb.String()
	for strings.Contains(res, "--") {
		res = strings.ReplaceAll(res, "--", "-")
	}
	return strings.Trim(res, "-_")
}

func (m Model) getDestinationDir() string {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
		actualID := m.visibleIndices[m.selectedIdx]
		for idx := range m.allNodes {
			if m.allNodes[idx].ID == actualID {
				if m.allNodes[idx].IsFolder {
					return m.allNodes[idx].Path
				}
				return filepath.Dir(m.allNodes[idx].Path)
			}
		}
	}
	return m.workspacePath
}

func (m *Model) openNewNoteModal() {
	m.modalType = "new_note"
	m.modalDestPath = m.getDestinationDir()
	m.modalInput.Reset()
	m.modalInput.Placeholder = "Note title (e.g. Arcane Secrets)"
	m.modalInput.Focus()
}

func (m *Model) openNewFolderModal() {
	m.modalType = "new_folder"
	m.modalDestPath = m.getDestinationDir()
	m.modalInput.Reset()
	m.modalInput.Placeholder = "Folder name (e.g. Campaign Lore)"
	m.modalInput.Focus()
}

func (m *Model) openQuickLogModal() {
	m.modalType = "quick_log"
	m.modalDestPath = m.workspacePath
	m.modalInput.Reset()
	m.modalInput.Placeholder = "What's on your mind? (Enter to save)"
	m.modalInput.Focus()
}

func (m *Model) confirmNewNote() tea.Cmd {
	title := strings.TrimSpace(m.modalInput.Value())
	if title == "" {
		m.modalType = ""
		m.modalInput.Blur()
		return nil
	}

	slug := slugify(title)
	if slug == "" {
		slug = "untitled"
	}

	now := time.Now()
	datePrefix := now.Format("2006-01-02")
	fileName := fmt.Sprintf("%s-%s.md", datePrefix, slug)
	targetPath := filepath.Join(m.modalDestPath, fileName)

	// Avoid overwriting existing notes with same name
	counter := 1
	for {
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			break
		}
		fileName = fmt.Sprintf("%s-%s-%d.md", datePrefix, slug, counter)
		targetPath = filepath.Join(m.modalDestPath, fileName)
		counter++
	}

	timestamp := now.Format("2006-01-02 15:04")
	initialContent := fmt.Sprintf("---\ntitle: %q\ndate: %s\ntags:\n  - note\n---\n\n# %s\n\n", title, timestamp, title)

	if err := os.WriteFile(targetPath, []byte(initialContent), 0644); err != nil {
		m.modalType = ""
		m.modalInput.Blur()
		return nil
	}

	// Rescan tree and select the new note
	nodes, isObs, _ := notes.ScanTree(m.workspacePath, m.config.SortMode)
	m.allNodes = nodes
	m.isObsidian = isObs

	// Automatically expand parent folder if needed
	for idx := range m.allNodes {
		if m.allNodes[idx].IsFolder && strings.HasPrefix(targetPath, m.allNodes[idx].Path) {
			m.allNodes[idx].Expanded = true
		}
	}

	m.rebuildVisible()

	// Find the new note in visible indices and select it
	for vIdx, id := range m.visibleIndices {
		for idx := range m.allNodes {
			if m.allNodes[idx].ID == id && m.allNodes[idx].Path == targetPath {
				m.selectedIdx = vIdx
				break
			}
		}
	}

	// Close modal and immediately open in-place editor!
	m.modalType = ""
	m.modalInput.Blur()
	m.inEditMode = true
	m.activePane = 1
	m.activeNotePath = targetPath
	m.saveStatus = ""
	m.editor.SetValue(initialContent)
	return m.editor.Focus()
}

func (m *Model) confirmNewFolder() {
	name := strings.TrimSpace(m.modalInput.Value())
	if name == "" {
		m.modalType = ""
		m.modalInput.Blur()
		return
	}

	targetPath := filepath.Join(m.modalDestPath, name)
	_ = os.MkdirAll(targetPath, 0755)

	nodes, isObs, _ := notes.ScanTree(m.workspacePath, m.config.SortMode)
	m.allNodes = nodes
	m.isObsidian = isObs

	// Automatically expand parent folder if needed
	for idx := range m.allNodes {
		if m.allNodes[idx].IsFolder && strings.HasPrefix(targetPath, m.allNodes[idx].Path) {
			m.allNodes[idx].Expanded = true
		}
	}

	m.rebuildVisible()

	// Select the newly created folder
	for vIdx, id := range m.visibleIndices {
		for idx := range m.allNodes {
			if m.allNodes[idx].ID == id && m.allNodes[idx].Path == targetPath {
				m.selectedIdx = vIdx
				break
			}
		}
	}

	m.modalType = ""
	m.modalInput.Blur()
	m.updateViewportPreview()
}

func (m *Model) confirmQuickLog() {
	text := strings.TrimSpace(m.modalInput.Value())
	if text == "" {
		m.modalType = ""
		m.modalInput.Blur()
		return
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("15:04")
	fileName := fmt.Sprintf("%s-quick-log.md", dateStr)
	targetPath := filepath.Join(m.workspacePath, fileName)

	// Ensure file exists with header, and verify it ends with a newline before appending
	prefix := ""
	if existingBytes, err := os.ReadFile(targetPath); err == nil {
		if len(existingBytes) > 0 && !strings.HasSuffix(string(existingBytes), "\n") {
			prefix = "\n"
		}
	} else if os.IsNotExist(err) {
		header := fmt.Sprintf("# Daily Quick Log (%s)\n\n", dateStr)
		_ = os.WriteFile(targetPath, []byte(header), 0644)
	}

	// Append log line
	f, err := os.OpenFile(targetPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		_, _ = f.WriteString(fmt.Sprintf("%s- **[%s]** %s\n", prefix, timeStr, text))
		_ = f.Close()
	}

	nodes, isObs, _ := notes.ScanTree(m.workspacePath, m.config.SortMode)
	m.allNodes = nodes
	m.isObsidian = isObs
	m.rebuildVisible()

	// Select the quick log note so user sees it previewed immediately
	for vIdx, id := range m.visibleIndices {
		for idx := range m.allNodes {
			if m.allNodes[idx].ID == id && m.allNodes[idx].Path == targetPath {
				m.selectedIdx = vIdx
				break
			}
		}
	}

	m.modalType = ""
	m.modalInput.Blur()
	m.updateViewportPreview()
}
