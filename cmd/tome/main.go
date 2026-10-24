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
	ID        int
	Title     string
	Path      string
	IsFolder  bool
	Expanded  bool
	Level     int
	Content   string
	ChildIDs  []int
	ParentID  int
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
	"Analyzing your Tome workspace across all folders...\n\n",
	"✦ **Found 3 relevant references** in your knowledge base:\n",
	"  • `01 - Grimoire/The Arcane Architecture.md` (High relevance)\n",
	"  • `02 - Codex/Cross-Platform Sync.md`\n",
	"  • `03 - Journal/2026-10-24 Daily Reflection.md`\n\n",
	"**Synthesis & Strategic Guidance**:\n",
	"1. **Layout Integrity:** By adhering to a strict height budget, Tome eliminates UI overflow.\n",
	"2. **Dual-Environment Velocity:** Pushing to GitLab triggers parallel compilation for Windows (.exe) and macOS (ARM64).\n",
	"3. **Fluidity:** The Go event loop guarantees zero-lag mouse hover scrolling and instant switching.\n\n",
	"*(Press [Esc] or [A] to close this drawer)*",
}

func sampleData() []Item {
	return []Item{
		{
			ID:       0,
			Title:    "Grimoire (Core Lore)",
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
> Tome is engineered to combine raw terminal velocity with obsidian-grade aesthetics.

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
			Title:    "Codex (Multi-Machine Sync)",
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
			Title:    "Journal (Daily Thoughts)",
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
Prototyping the strict height budget engine to ensure the AI assistant drawer never pushes elements off-screen.`,
		},
	}
}

func initialModel() Model {
	items := sampleData()
	m := Model{
		allItems:    items,
		selectedIdx: 1, // Start on first note
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

		// Strict layout budgeting
		headerHeight := 2
		footerHeight := 1
		aiHeight := 0
		if m.aiOpen {
			aiHeight = max(6, min(10, m.height/3))
		}
		panesHeight := max(5, m.height-headerHeight-footerHeight-aiHeight)

		leftWidth := max(24, min(36, m.width/3))
		rightWidth := max(20, m.width-leftWidth-1)

		viewportWidth := max(10, rightWidth-4)
		viewportHeight := max(3, panesHeight-2)

		if !m.ready {
			m.viewport = viewport.New(viewportWidth, viewportHeight)
			m.viewport.SetContent(m.renderActivePreview(viewportWidth))
			m.ready = true
		} else {
			m.viewport.Width = viewportWidth
			m.viewport.Height = viewportHeight
			m.viewport.SetContent(m.renderActivePreview(viewportWidth))
		}

	case aiTokenMsg:
		m.aiResponse += string(msg)
		m.aiTokenIndex++
		if m.aiTokenIndex < len(sampleAIStream) {
			return m, streamNextToken(sampleAIStream[m.aiTokenIndex], 60*time.Millisecond)
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
				// Trigger recalculation of dimensions
				m.recalculateViewport()
				return m, streamNextToken(sampleAIStream[0], 80*time.Millisecond)
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
		leftWidth := max(24, min(36, m.width/3))

		switch msg.Type {
		case tea.MouseWheelUp:
			if msg.X < leftWidth {
				// Scrolling over left tree navigates items
				if m.selectedIdx > 0 {
					m.selectedIdx--
					m.updateViewportPreview()
				}
			} else {
				// Scrolling over right preview scrolls document
				m.viewport.LineUp(3)
			}

		case tea.MouseWheelDown:
			if msg.X < leftWidth {
				if m.selectedIdx < len(m.visibleIndices)-1 {
					m.selectedIdx++
					m.updateViewportPreview()
				}
			} else {
				m.viewport.LineDown(3)
			}

		case tea.MouseLeft:
			// Click selection in tree
			headerHeight := 2
			clickedRow := msg.Y - headerHeight
			if msg.X < leftWidth && clickedRow >= 0 && clickedRow < len(m.visibleIndices) {
				if m.selectedIdx == clickedRow {
					// Second click toggles folder
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
	headerHeight := 2
	footerHeight := 1
	aiHeight := 0
	if m.aiOpen {
		aiHeight = max(6, min(10, m.height/3))
	}
	panesHeight := max(5, m.height-headerHeight-footerHeight-aiHeight)
	leftWidth := max(24, min(36, m.width/3))
	rightWidth := max(20, m.width-leftWidth-1)

	m.viewport.Width = max(10, rightWidth-4)
	m.viewport.Height = max(3, panesHeight-2)
	m.viewport.SetContent(m.renderActivePreview(m.viewport.Width))
}

func (m *Model) updateViewportPreview() {
	m.viewport.SetContent(m.renderActivePreview(m.viewport.Width))
	m.viewport.GotoTop()
}

func (m Model) renderActivePreview(width int) string {
	if m.selectedIdx < 0 || m.selectedIdx >= len(m.visibleIndices) {
		return "Select a note to preview."
	}
	actualIdx := m.visibleIndices[m.selectedIdx]
	item := m.allItems[actualIdx]

	if item.IsFolder {
		return fmt.Sprintf("\n  📁 **Folder: %s**\n\n  Contains %d notes. Press [Enter] or click to toggle.",
			item.Title, len(item.ChildIDs))
	}

	return markdown.RenderDocument(item.Content, width)
}

func (m Model) View() string {
	if !m.ready {
		return "\n  Summoning Tome..."
	}

	// 1. Strict Height & Width Budgeting
	headerHeight := 2
	footerHeight := 1
	aiHeight := 0
	if m.aiOpen {
		aiHeight = max(6, min(10, m.height/3))
	}
	panesHeight := max(5, m.height-headerHeight-footerHeight-aiHeight)

	leftWidth := max(24, min(36, m.width/3))
	rightWidth := max(20, m.width-leftWidth-1)

	// 2. Render Header (Exactly 2 rows)
	title := ui.BrandStyle.Render("📖 T O M E")
	subtitle := ui.SubtitleStyle.Render(" The Arcane Terminal Notebook")
	wsPill := ui.BadgeStyle.Render("Workspace: ~/Notes")
	aiPill := ui.ActiveBadgeStyle.Render("✦ Cloud AI: Ready")

	headerLeft := title + subtitle
	headerRight := wsPill + " " + aiPill
	spacing := max(1, m.width-lipgloss.Width(headerLeft)-lipgloss.Width(headerRight)-2)
	headerRow1 := " " + headerLeft + strings.Repeat(" ", spacing) + headerRight
	headerDivider := lipgloss.NewStyle().Foreground(ui.ColorBorderMuted).Render(strings.Repeat("━", m.width))
	headerBlock := lipgloss.JoinVertical(lipgloss.Left, headerRow1, headerDivider)

	// 3. Render Tree Pane (Left)
	var treeLines []string
	for i, visIdx := range m.visibleIndices {
		item := m.allItems[visIdx]

		prefix := "  "
		if item.Level > 0 {
			prefix = "   ├─ "
		}

		icon := "󰈙 "
		if item.IsFolder {
			if item.Expanded {
				icon = " "
			} else {
				icon = " "
			}
		}

		lineText := prefix + icon + item.Title
		availWidth := leftWidth - 4
		if len(lineText) > availWidth && availWidth > 5 {
			lineText = lineText[:availWidth-3] + "..."
		}

		if i == m.selectedIdx {
			treeLines = append(treeLines, ui.TreeSelectedStyle.Render(fmt.Sprintf("%-*s", availWidth+2, lineText)))
		} else {
			if item.IsFolder {
				treeLines = append(treeLines, ui.TreeFolderStyle.Render(lineText))
			} else {
				treeLines = append(treeLines, ui.TreeItemStyle.Render(lineText))
			}
		}
	}

	leftStyle := ui.PaneStyle
	if m.activePane == 0 {
		leftStyle = ui.ActivePaneStyle
	}
	leftView := leftStyle.
		Width(leftWidth - 2).
		Height(panesHeight - 2).
		Render(strings.Join(treeLines, "\n"))

	// 4. Render Preview Pane (Right)
	rightStyle := ui.PaneStyle
	if m.activePane == 1 {
		rightStyle = ui.ActivePaneStyle
	}
	rightView := rightStyle.
		Width(rightWidth - 2).
		Height(panesHeight - 2).
		Render(m.viewport.View())

	// Join Panes horizontally
	panesView := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)

	// 5. Render AI Drawer (if open)
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

		bodyText := m.aiResponse
		aiView = ui.AIDrawerStyle.
			Width(m.width - 4).
			Height(aiHeight - 2).
			Render(titleBar + "\n" + bodyText)
	}

	// 6. Render Footer (Exactly 1 row)
	kNav := ui.FooterKeyStyle.Render("[↑/↓/W/S]") + ui.FooterDescStyle.Render(" Move  ")
	kExp := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Toggle Folder  ")
	kAi  := ui.FooterKeyStyle.Render("[A]") + ui.FooterDescStyle.Render(" Cloud AI  ")
	kTab := ui.FooterKeyStyle.Render("[Tab]") + ui.FooterDescStyle.Render(" Switch Pane  ")
	kQuit:= ui.FooterKeyStyle.Render("[Q]") + ui.FooterDescStyle.Render(" Quit")
	footer := " " + kNav + kExp + kAi + kTab + kQuit

	// 7. Strict Composition: Stack strictly to terminal height
	var fullScreen string
	if m.aiOpen {
		fullScreen = lipgloss.JoinVertical(lipgloss.Left, headerBlock, panesView, aiView, footer)
	} else {
		fullScreen = lipgloss.JoinVertical(lipgloss.Left, headerBlock, panesView, footer)
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
