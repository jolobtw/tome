package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jolobtw/tome/internal/markdown"
	"github.com/jolobtw/tome/internal/notes"
	"github.com/jolobtw/tome/internal/ui"
)

const AppVersion = "v1.0.4"

type ContextLine struct {
	LineNum int
	Text    string
	IsMatch bool
}

type SearchResult struct {
	NodeID       int
	Path         string
	Title        string
	RelPath      string
	LineNum      int
	Snippet      string
	ContextLines []ContextLine
}

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

	// Quick Filter (Option 1 - [/])
	filterActive bool
	filterInput  textinput.Model

	// Floating Modal System (New Note, New Folder, Quick Log, Rename, Delete, Move)
	modalType       string // "", "new_note", "new_folder", "quick_log", "rename", "delete", "move"
	modalInput      textinput.Model
	modalDestPath   string
	modalTargetItem *notes.Node
	modalError      string
	moveFolders     []string
	moveSelected    int

	// Top Slide-Down Content Search Drawer (Option 2 - [Ctrl+F])
	searchOpen     bool
	searchSliding  bool
	searchSlideDir int // +1 opening, -1 closing
	searchHeight   int
	searchTargetH  int
	searchInput    textinput.Model
	searchResults  []SearchResult
	searchSelected int

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

	// Formatting Cheat Sheet Drawer
	cheatSheetOpen bool
	cheatSliding   bool
	cheatSlideDir  int // +1 opening, -1 closing
	cheatHeight    int
	cheatTargetH   int

	// Version & GitHub Update Indicator
	latestVersion string
	hasUpdate     bool

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

	// Handle CLI arguments: --version or folder path
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-v" || arg == "--version" || arg == "version" {
			fmt.Printf("Tome %s (The Arcane Terminal Notebook)\n", AppVersion)
			os.Exit(0)
		}
		if arg != "" && arg != "-h" && arg != "--help" {
			if strings.HasPrefix(arg, "~") {
				home, _ := os.UserHomeDir()
				arg = filepath.Join(home, strings.TrimPrefix(arg, "~"))
			}
			absPath, err := filepath.Abs(arg)
			if err == nil {
				// Create directory if it does not exist yet
				_ = os.MkdirAll(absPath, 0755)
				if stat, err := os.Stat(absPath); err == nil && stat.IsDir() {
					workspacePath = absPath
					cfg.ActiveNotebook = absPath
					_ = notes.SaveConfig(cfg)
				}
			}
		}
	}

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
	ta.Cursor.SetMode(cursor.CursorStatic)

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

	fi := textinput.New()
	fi.Prompt = ""
	fi.Placeholder = "Type to filter notes... (Esc to clear)"
	fi.TextStyle = lipgloss.NewStyle().Foreground(ui.ColorTextBright)
	fi.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Italic(true)
	fi.CharLimit = 50

	si := textinput.New()
	si.Prompt = "🔍 "
	si.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColorFlameOrange).Bold(true)
	si.Placeholder = "Search note contents (e.g. dragon, quest, spell)..."
	si.TextStyle = lipgloss.NewStyle().Foreground(ui.ColorTextBright)
	si.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Italic(true)
	si.CharLimit = 80

	m := Model{
		config:        cfg,
		workspacePath: workspacePath,
		workspaceName: workspaceName,
		isObsidian:    isObsidian,
		allNodes:      allNodes,
		selectedIdx:   0,
		activePane:    0,
		editor:        ta,
		filterInput:   fi,
		searchInput:   si,
		aiInput:       ti,
		aiState:       "idle",
		modalInput:    mi,
		statusMsg:     "Welcome to Tome • Press [/] to Filter • [Ctrl+F] Search • [Ctrl+K] AI",
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
	if m.filterActive && strings.TrimSpace(m.filterInput.Value()) != "" {
		q := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
		var visible []int
		for idx := range m.allNodes {
			if !m.allNodes[idx].IsFolder {
				titleLower := strings.ToLower(m.allNodes[idx].Title)
				relLower := strings.ToLower(m.allNodes[idx].RelPath)
				if strings.Contains(titleLower, q) || strings.Contains(relLower, q) {
					m.allNodes[idx].Prefix = ""
					visible = append(visible, m.allNodes[idx].ID)
				}
			}
		}
		m.visibleIndices = visible
		if m.selectedIdx >= len(m.visibleIndices) {
			m.selectedIdx = max(0, len(m.visibleIndices)-1)
		}
		return
	}

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
	return checkGitHubUpdateCmd
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case versionCheckMsg:
		if msg.err == nil && msg.hasUpdate {
			m.latestVersion = msg.latestVersion
			m.hasUpdate = true
		}
		return m, nil

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

		if m.cheatSliding {
			if m.cheatSlideDir > 0 {
				m.cheatHeight += 4
				if m.cheatHeight >= m.cheatTargetH {
					m.cheatHeight = m.cheatTargetH
					m.cheatSliding = false
				} else {
					cmds = append(cmds, slideTick())
				}
			} else {
				m.cheatHeight -= 4
				if m.cheatHeight <= 0 {
					m.cheatHeight = 0
					m.cheatSliding = false
					m.cheatSheetOpen = false
				} else {
					cmds = append(cmds, slideTick())
				}
			}
		}

		if m.searchSliding {
			if m.searchSlideDir > 0 {
				m.searchHeight += 4
				if m.searchHeight >= m.searchTargetH {
					m.searchHeight = m.searchTargetH
					m.searchSliding = false
					m.searchInput.Focus()
				} else {
					cmds = append(cmds, slideTick())
				}
			} else {
				m.searchHeight -= 4
				if m.searchHeight <= 0 {
					m.searchHeight = 0
					m.searchSliding = false
					m.searchOpen = false
					m.searchInput.Blur()
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
				m.modalError = ""
				m.modalTargetItem = nil
				m.modalInput.Blur()
				return m, nil

			case "up", "k", "w":
				if m.modalType == "move" {
					if m.moveSelected > 0 {
						m.moveSelected--
					}
					return m, nil
				}

			case "down", "j", "s":
				if m.modalType == "move" {
					if m.moveSelected < len(m.moveFolders)-1 {
						m.moveSelected++
					}
					return m, nil
				}

			case "y", "Y":
				if m.modalType == "delete" {
					m.confirmDelete()
					return m, nil
				}

			case "n", "N":
				if m.modalType == "delete" {
					m.modalType = ""
					m.modalError = ""
					m.modalTargetItem = nil
					return m, nil
				}

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
				case "rename":
					m.confirmRename()
					return m, nil
				case "delete":
					m.confirmDelete()
					return m, nil
				case "move":
					m.confirmMove()
					return m, nil
				}
				m.modalType = ""
				m.modalInput.Blur()
				return m, nil
			}

			if m.modalType == "delete" || m.modalType == "move" {
				// Don't send other keys to modalInput when in delete or move modals
				return m, nil
			}

			var tiCmd tea.Cmd
			m.modalInput, tiCmd = m.modalInput.Update(msg)
			return m, tiCmd
		}

		// When top Search Drawer is open, route keys to search
		if m.searchOpen && !m.searchSliding {
			switch msg.String() {
			case "esc", "ctrl+f":
				m.searchSliding = true
				m.searchSlideDir = -1
				m.searchInput.Blur()
				return m, slideTick()

			case "up":
				if m.searchSelected > 0 {
					m.searchSelected--
				}
				return m, nil

			case "down":
				if m.searchSelected < len(m.searchResults)-1 {
					m.searchSelected++
				}
				return m, nil

			case "enter":
				if len(m.searchResults) > 0 && m.searchSelected >= 0 && m.searchSelected < len(m.searchResults) {
					res := m.searchResults[m.searchSelected]
					targetPath := res.Path
					targetLine := res.LineNum

					m.searchOpen = false
					m.searchSliding = false
					m.searchHeight = 0
					m.searchInput.Blur()

					m.rescanTreePreservingExpansion(targetPath)
					cleanTarget := filepath.Clean(targetPath)
					for vIdx, id := range m.visibleIndices {
						for idx := range m.allNodes {
							if m.allNodes[idx].ID == id && filepath.Clean(m.allNodes[idx].Path) == cleanTarget {
								m.selectedIdx = vIdx
								break
							}
						}
					}
					m.activePane = 1
					if m.viewport.Width > 0 {
						m.viewport.SetContent(m.renderActivePreview(m.viewport.Width))
						m.viewport.SetYOffset(max(0, targetLine-3))
					}
					return m, nil
				}
				return m, nil
			}

			var siCmd tea.Cmd
			m.searchInput, siCmd = m.searchInput.Update(msg)
			m.performContentSearch(m.searchInput.Value())
			return m, siCmd
		}

		// When in in-place note editor mode, route keys directly to editor
		if m.inEditMode {
			// If cheat sheet drawer is open, check keys
			if m.cheatSheetOpen && !m.cheatSliding {
				switch msg.String() {
				case "ctrl+g", "ctrl+/", "ctrl+_", "ctrl+?", "f1", "esc":
					m.cheatSliding = true
					m.cheatSlideDir = -1
					return m, slideTick()
				default:
					return m, nil
				}
			}

			switch msg.String() {
			case "ctrl+g", "ctrl+/", "ctrl+_", "ctrl+?", "f1":
				m.cheatSheetOpen = true
				m.cheatSliding = true
				m.cheatSlideDir = 1
				m.cheatHeight = 4
				m.cheatTargetH = min(18, max(14, int(float64(m.height)*0.60)))
				return m, slideTick()

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
				m.cheatSheetOpen = false
				m.cheatSliding = false
				m.cheatHeight = 0
				m.updateViewportPreview()
				return m, nil

			case "ctrl+q":
				m.inEditMode = false
				m.saveStatus = ""
				m.editor.Blur()
				m.cheatSheetOpen = false
				m.cheatSliding = false
				m.cheatHeight = 0
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

		// When cheat sheet drawer is open in preview mode, route keys to dismiss it
		if m.cheatSheetOpen && !m.cheatSliding {
			switch msg.String() {
			case "ctrl+g", "ctrl+/", "ctrl+_", "ctrl+?", "f1", "esc", "q", "enter":
				m.cheatSliding = true
				m.cheatSlideDir = -1
				return m, slideTick()
			default:
				return m, nil
			}
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

		// Quick Note Filter Mode ([/])
		if m.filterActive {
			switch msg.String() {
			case "esc":
				m.filterActive = false
				m.filterInput.Reset()
				m.filterInput.Blur()
				m.rebuildVisible()
				m.updateViewportPreview()
				return m, nil

			case "enter":
				chosenID := -1
				if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
					chosenID = m.visibleIndices[m.selectedIdx]
				}
				m.filterActive = false
				m.filterInput.Reset()
				m.filterInput.Blur()

				if chosenID != -1 {
					// Expand ancestors of chosen note so it stays visible in full tree
					for pID := m.getNodeParentID(chosenID); pID != -1; {
						found := false
						for idx := range m.allNodes {
							if m.allNodes[idx].ID == pID {
								m.allNodes[idx].Expanded = true
								pID = m.allNodes[idx].ParentID
								found = true
								break
							}
						}
						if !found {
							break
						}
					}
				}

				m.rebuildVisible()

				if chosenID != -1 {
					for vIdx, id := range m.visibleIndices {
						if id == chosenID {
							m.selectedIdx = vIdx
							break
						}
					}
				}
				m.updateViewportPreview()
				return m, nil

			case "up":
				if m.selectedIdx > 0 {
					m.selectedIdx--
					m.updateViewportPreview()
				}
				return m, nil

			case "down":
				if m.selectedIdx < len(m.visibleIndices)-1 {
					m.selectedIdx++
					m.updateViewportPreview()
				}
				return m, nil

			case "tab":
				m.filterActive = false
				m.filterInput.Blur()
				m.activePane = 1
				m.updateViewportPreview()
				return m, nil
			}

			var fiCmd tea.Cmd
			m.filterInput, fiCmd = m.filterInput.Update(msg)
			m.rebuildVisible()
			m.updateViewportPreview()
			return m, fiCmd
		}

		// Main navigation keys
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.activePane = 1 - m.activePane

		case "home", "g":
			m.viewport.GotoTop()
			return m, nil

		case "end", "G":
			m.viewport.GotoBottom()
			return m, nil

		case "pgup", "ctrl+u":
			m.viewport.HalfViewUp()
			return m, nil

		case "pgdown", "ctrl+d":
			m.viewport.HalfViewDown()
			return m, nil

		case "up", "k", "w":
			if m.activePane == 1 {
				m.viewport.LineUp(3)
			} else if m.selectedIdx > 0 {
				m.selectedIdx--
				m.updateViewportPreview()
			}

		case "down", "j", "s":
			if m.activePane == 1 {
				m.viewport.LineDown(3)
			} else if m.selectedIdx < len(m.visibleIndices)-1 {
				m.selectedIdx++
				m.updateViewportPreview()
			}

		case "a", "left":
			if m.activePane == 1 {
				m.activePane = 0
				return m, nil
			}
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
					if !current.IsFolder {
						m.activePane = 1
						return m, nil
					}
					if !current.Expanded {
						current.Expanded = true
						m.rebuildVisible()
						m.updateViewportPreview()
					} else if len(current.ChildIDs) > 0 && m.selectedIdx < len(m.visibleIndices)-1 {
						m.selectedIdx++
						m.updateViewportPreview()
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
						scrollToBottom(&m.editor)
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

		case "r":
			m.openRenameModal()
			if m.modalType == "rename" {
				return m, m.modalInput.Focus()
			}
			return m, nil

		case "x", "delete":
			m.openDeleteModal()
			return m, nil

		case "m":
			m.openMoveModal()
			return m, nil

		case "/":
			m.activePane = 0
			m.filterActive = true
			m.filterInput.Reset()
			m.filterInput.Focus()
			m.rebuildVisible()
			return m, m.filterInput.Focus()

		case "ctrl+f":
			if m.searchOpen {
				m.searchSliding = true
				m.searchSlideDir = -1
				return m, slideTick()
			}
			usableW := max(20, m.width-2)
			m.searchOpen = true
			m.searchSliding = true
			m.searchSlideDir = 1
			m.searchHeight = 4
			m.searchTargetH = min(36, max(20, int(float64(m.height)*0.85)))
			m.searchInput.Reset()
			m.searchResults = nil
			m.searchSelected = 0
			m.searchInput.Width = max(20, usableW-12)
			return m, slideTick()

		case "ctrl+k":
			m.aiOpen = true
			m.sliding = true
			m.slideDir = 1
			m.animHeight = 4
			m.targetHeight = max(12, int(float64(m.height)*0.65))
			m.aiInput.Width = max(20, m.width-12)
			return m, slideTick()

		case "ctrl+g", "ctrl+/", "ctrl+_", "ctrl+?", "f1", "?":
			m.cheatSheetOpen = true
			m.cheatSliding = true
			m.cheatSlideDir = 1
			m.cheatHeight = 4
			m.cheatTargetH = min(18, max(14, int(float64(m.height)*0.60)))
			return m, slideTick()
		}

	case tea.MouseMsg:
		if m.searchOpen && !m.searchSliding {
			switch msg.Type {
			case tea.MouseWheelUp:
				if m.searchSelected > 0 {
					m.searchSelected--
				}
				return m, nil
			case tea.MouseWheelDown:
				if m.searchSelected < len(m.searchResults)-1 {
					m.searchSelected++
				}
				return m, nil
			}
			return m, nil
		}

		if m.modalType != "" || m.aiOpen {
			return m, nil
		}

		// Only a deliberate LEFT CLICK while cheat sheet is open dismisses it (NOT mouse movement!)
		if msg.Type == tea.MouseLeft && m.cheatSheetOpen && !m.cheatSliding {
			m.cheatSliding = true
			m.cheatSlideDir = -1
			return m, slideTick()
		}

		usableWidth := max(20, m.width-2)
		leftTotalWidth := max(34, min(52, usableWidth*36/100))

		if m.inEditMode {
			switch msg.Type {
			case tea.MouseWheelUp:
				upMsg := tea.KeyMsg{Type: tea.KeyUp}
				for i := 0; i < 3; i++ {
					m.editor, _ = m.editor.Update(upMsg)
				}
				return m, nil

			case tea.MouseWheelDown:
				downMsg := tea.KeyMsg{Type: tea.KeyDown}
				for i := 0; i < 3; i++ {
					m.editor, _ = m.editor.Update(downMsg)
				}
				return m, nil

			case tea.MouseLeft:
				if m.cheatSheetOpen && !m.cheatSliding {
					m.cheatSliding = true
					m.cheatSlideDir = -1
					return m, slideTick()
				}

				// Translate screen coordinates to local editor coordinates:
				// X: right pane starts at leftTotalWidth; 1 col border + 1 col padding = +2
				// Y: 2 header banner rows + 1 top border row + 1 editor title row = 4 rows
				editorX := msg.X - (leftTotalWidth + 2)
				editorY := msg.Y - 4

				if editorX >= 0 && editorY >= 0 && editorX < m.editor.Width() && editorY < m.editor.Height() {
					handleEditorClick(&m.editor, editorX, editorY)
					return m, nil
				}
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
				m.viewport.LineUp(5)
			}

		case tea.MouseWheelDown:
			if msg.X < leftTotalWidth {
				if m.selectedIdx < len(m.visibleIndices)-1 {
					m.selectedIdx++
					m.updateViewportPreview()
				}
			} else {
				m.viewport.LineDown(5)
			}

		case tea.MouseLeft:
			if msg.X >= leftTotalWidth {
				m.activePane = 1
				return m, nil
			}
			m.activePane = 0
			offset := 3
			if m.filterActive {
				offset = 5
			}
			clickedRow := msg.Y - offset
			if clickedRow >= 0 && clickedRow < len(m.visibleIndices) {
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
	m.viewport.GotoBottom()
}

func (m *Model) updateViewportPreview() {
	if m.viewport.Width > 0 {
		m.viewport.SetContent(m.renderActivePreview(m.viewport.Width))
		m.viewport.GotoBottom()
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
	starGlyph := lipgloss.NewStyle().Foreground(ui.ColorFlameOrange).Bold(true).Render("✦")
	sepGlyph := lipgloss.NewStyle().Foreground(ui.ColorTextMuted).Render("❖")
	titleGradient := ui.Gradient("❖ TOME", "#FF4500", "#FFD166")
	bannerLeft := fmt.Sprintf("%s %s %s %s", starGlyph, titleGradient, sepGlyph, ui.SubtitleStyle.Render("The Arcane Terminal Notebook"))

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

	var verBadge string
	if m.hasUpdate {
		verBadge = lipgloss.NewStyle().
			Foreground(ui.ColorGoldGlow).
			Background(lipgloss.Color("#4D1A00")).
			Bold(true).
			Padding(0, 1).
			Render(fmt.Sprintf("⇡ %s (New: %s)", AppVersion, m.latestVersion))
	} else {
		verBadge = ui.BadgeStyle.Render(AppVersion)
	}

	bannerRight := fmt.Sprintf("%s %s %s", wsBadge, vaultBadge, verBadge)

	leftLen := lipgloss.Width(bannerLeft)
	rightLen := lipgloss.Width(bannerRight)
	if leftLen+rightLen+2 > usableWidth {
		bannerLeft = fmt.Sprintf("%s %s", starGlyph, titleGradient)
		leftLen = lipgloss.Width(bannerLeft)
	}
	gap := max(1, usableWidth-leftLen-rightLen-2)
	row1 := " " + bannerLeft + strings.Repeat(" ", gap) + bannerRight
	row2 := lipgloss.NewStyle().Foreground(ui.ColorBorderMuted).Render(strings.Repeat("─", usableWidth))
	headerBlock := lipgloss.JoinVertical(lipgloss.Left, row1, row2)

	// 2. Tree Pane (Left)
	innerLeftWidth := leftTotalWidth - 4
	var treeLines []string
	availTitleWidth := max(4, innerLeftWidth-4)

	if m.filterActive {
		m.filterInput.Width = max(10, innerLeftWidth-8)
		filterPrompt := lipgloss.NewStyle().Foreground(ui.ColorFlameOrange).Bold(true).Render("🔍 ")
		treeLines = append(treeLines, filterPrompt+m.filterInput.View())
		filterDivider := lipgloss.NewStyle().Foreground(ui.ColorBorderMuted).Render(strings.Repeat("─", innerLeftWidth))
		treeLines = append(treeLines, filterDivider)
	}

	if m.filterActive && len(m.visibleIndices) == 0 {
		treeLines = append(treeLines, ui.SubtitleStyle.Render("  No notes match filter"))
	}

	for i, visID := range m.visibleIndices {
		if len(treeLines) >= panesContentHeight {
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

		var displayTitle string
		if m.filterActive {
			dir := filepath.Dir(item.RelPath)
			if dir != "." && dir != "" {
				displayTitle = fmt.Sprintf("%s%s (%s)", icon, item.Title, dir)
			} else {
				displayTitle = icon + item.Title
			}
		} else {
			displayTitle = item.Prefix + icon + item.Title
		}

		rawTitle := truncateWithDots(displayTitle, availTitleWidth)
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
		Width(leftTotalWidth - 2).
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
		Width(rightTotalWidth - 2).
		Height(panesContentHeight).
		Render(rightContent)

	// 4. Combine Full Background (Undisturbed!)
	panesView := lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)

	// 5. Footer Bar
	var footer string
	if m.modalType != "" {
		switch m.modalType {
		case "rename":
			kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Save Name  ")
			kClose := ui.FooterKeyStyle.Render("[Esc]") + ui.FooterDescStyle.Render(" Cancel")
			footer = " ✦ Rename Item │ " + kEnter + kClose
		case "delete":
			kEnter := ui.FooterKeyStyle.Render("[Enter/Y]") + ui.FooterDescStyle.Render(" Confirm Delete  ")
			kClose := ui.FooterKeyStyle.Render("[Esc/N]") + ui.FooterDescStyle.Render(" Cancel")
			footer = " ⚠ Delete Item │ " + kEnter + kClose
		case "move":
			kNav := ui.FooterKeyStyle.Render("[W/S/↑/↓]") + ui.FooterDescStyle.Render(" Select  ")
			kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Move Here  ")
			kClose := ui.FooterKeyStyle.Render("[Esc]") + ui.FooterDescStyle.Render(" Cancel")
			footer = " ✦ Move Item │ " + kNav + kEnter + kClose
		default:
			kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Confirm  ")
			kClose := ui.FooterKeyStyle.Render("[Esc]") + ui.FooterDescStyle.Render(" Cancel")
			footer = " ✦ Modal Prompt │ " + kEnter + kClose
		}
	} else if m.searchOpen {
		kNav := ui.FooterKeyStyle.Render("[↑/↓]") + ui.FooterDescStyle.Render(" Select  ")
		kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Jump to Note  ")
		kClose := ui.FooterKeyStyle.Render("[Esc/Ctrl+F]") + ui.FooterDescStyle.Render(" Close Search")
		footer = " ✦ Content Search │ " + kNav + kEnter + kClose
	} else if m.filterActive {
		kNav := ui.FooterKeyStyle.Render("[↑/↓]") + ui.FooterDescStyle.Render(" Select  ")
		kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Open Note  ")
		kClose := ui.FooterKeyStyle.Render("[Esc]") + ui.FooterDescStyle.Render(" Clear Filter")
		footer = " 🔍 Quick Filter │ " + kNav + kEnter + kClose
	} else if m.inEditMode {
		kSave := ui.FooterKeyStyle.Render("[Ctrl+S]") + ui.FooterDescStyle.Render(" Save  ")
		kDone := ui.FooterKeyStyle.Render("[Esc]") + ui.FooterDescStyle.Render(" Save & Finish  ")
		kDiscard := ui.FooterKeyStyle.Render("[Ctrl+Q]") + ui.FooterDescStyle.Render(" Discard  ")
		kCheat := ui.FooterKeyStyle.Render("[F1/Ctrl+G]") + ui.FooterDescStyle.Render(" Guide")
		var saveBadge string
		if m.saveStatus != "" {
			if m.saveStatus == "✓ Saved" {
				saveBadge = ui.ActiveBadgeStyle.Render(m.saveStatus) + "  "
			} else {
				saveBadge = ui.BadgeStyle.Render(m.saveStatus) + "  "
			}
		}
		if m.cheatSheetOpen {
			kCloseCheat := ui.FooterKeyStyle.Render("[Esc/F1]") + ui.FooterDescStyle.Render(" Close Guide  ")
			footer = " ✦ Formatting Guide │ " + kCloseCheat + kSave + kDiscard
		} else {
			footer = " ✎ In-Place Editor │ " + saveBadge + kSave + kDone + kDiscard + kCheat
		}
	} else if m.aiOpen {
		kEnter := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Ask AI  ")
		kClose := ui.FooterKeyStyle.Render("[Esc/Ctrl+K]") + ui.FooterDescStyle.Render(" Close Assistant  ")
		kQuit := ui.FooterKeyStyle.Render("[Ctrl+C]") + ui.FooterDescStyle.Render(" Quit")
		footer = " " + kEnter + kClose + kQuit
	} else if m.cheatSheetOpen {
		kClose := ui.FooterKeyStyle.Render("[Esc/F1]") + ui.FooterDescStyle.Render(" Close Guide  ")
		kQuit := ui.FooterKeyStyle.Render("[Q]") + ui.FooterDescStyle.Render(" Quit")
		footer = " ✦ Formatting Guide │ " + kClose + kQuit
	} else {
		if m.activePane == 1 {
			kNav := ui.FooterKeyStyle.Render("[W/S/Wheel]") + ui.FooterDescStyle.Render(" Scroll  ")
			kSearch := ui.FooterKeyStyle.Render("[Ctrl+F]") + ui.FooterDescStyle.Render(" Search  ")
			kTab := ui.FooterKeyStyle.Render("[Tab/A]") + ui.FooterDescStyle.Render(" Tree Pane  ")
			kEdit := ui.FooterKeyStyle.Render("[E]") + ui.FooterDescStyle.Render(" Edit  ")
			kCheat := ui.FooterKeyStyle.Render("[F1/Ctrl+G]") + ui.FooterDescStyle.Render(" Guide  ")
			kAi := ui.FooterKeyStyle.Render("[Ctrl+K]") + ui.FooterDescStyle.Render(" Cloud AI  ")
			kQuit := ui.FooterKeyStyle.Render("[Q]") + ui.FooterDescStyle.Render(" Quit")
			footer = " 📄 Preview │ " + kNav + kSearch + kTab + kEdit + kCheat + kAi + kQuit
		} else {
			kNav := ui.FooterKeyStyle.Render("[W/S/A/D]") + ui.FooterDescStyle.Render(" Move/Fold  ")
			kFilter := ui.FooterKeyStyle.Render("[/]") + ui.FooterDescStyle.Render(" Filter  ")
			kSearch := ui.FooterKeyStyle.Render("[Ctrl+F]") + ui.FooterDescStyle.Render(" Search  ")
			kExp := ui.FooterKeyStyle.Render("[Enter]") + ui.FooterDescStyle.Render(" Open  ")
			kEdit := ui.FooterKeyStyle.Render("[E]") + ui.FooterDescStyle.Render(" Edit  ")
			kNew := ui.FooterKeyStyle.Render("[N]") + ui.FooterDescStyle.Render(" New  ")
			kFolder := ui.FooterKeyStyle.Render("[F]") + ui.FooterDescStyle.Render(" Folder  ")
			kRename := ui.FooterKeyStyle.Render("[R]") + ui.FooterDescStyle.Render(" Rename  ")
			kMove := ui.FooterKeyStyle.Render("[M]") + ui.FooterDescStyle.Render(" Move  ")
			kDelete := ui.FooterKeyStyle.Render("[X]") + ui.FooterDescStyle.Render(" Del  ")
			kLog := ui.FooterKeyStyle.Render("[L]") + ui.FooterDescStyle.Render(" Quick Log  ")
			kAi := ui.FooterKeyStyle.Render("[Ctrl+K]") + ui.FooterDescStyle.Render(" Cloud AI  ")
			kQuit := ui.FooterKeyStyle.Render("[Q]") + ui.FooterDescStyle.Render(" Quit")
			footer = " " + kNav + kFilter + kSearch + kExp + kEdit + kNew + kFolder + kRename + kMove + kDelete + kLog + kAi + kQuit
		}
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

	// 7b. Layered Sliding Formatting Cheat Sheet Drawer
	if m.cheatSheetOpen && m.cheatHeight > 0 {
		drawerInnerWidth := usableWidth - 4
		drawerContentHeight := max(2, m.cheatHeight - 2)

		titlePrefix := "✦ FORMATTING CHEAT SHEET "
		ruleLen := max(2, drawerInnerWidth-lipgloss.Width(titlePrefix)-2)
		statusText := titlePrefix + strings.Repeat("─", ruleLen)
		drawerTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorFlameOrange).
			Render(statusText)

		content := renderCheatSheetContent(drawerInnerWidth)
		drawerContent := fmt.Sprintf("%s\n\n%s", drawerTitle, content)

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

		cheatBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ui.ColorFlameOrange).
			Background(ui.ColorDarkSlate).
			Padding(0, 1)

		cheatDrawerRendered := cheatBoxStyle.
			Width(drawerInnerWidth).
			Height(drawerContentHeight).
			MaxHeight(m.cheatHeight).
			Render(strings.Join(clampedLines, "\n"))

		drawerLines := strings.Split(cheatDrawerRendered, "\n")
		if len(drawerLines) > m.cheatHeight {
			drawerLines = drawerLines[:m.cheatHeight]
		}
		for len(drawerLines) < m.cheatHeight {
			drawerLines = append(drawerLines, strings.Repeat(" ", usableWidth))
		}

		footerRow := len(bgLines) - 1
		startRow := footerRow - m.cheatHeight
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

	// 7c. Layered Sliding Content Search Top Drawer (from top row 2)
	if m.searchOpen && m.searchHeight > 0 {
		drawerInnerWidth := usableWidth - 4
		drawerContentHeight := max(2, m.searchHeight-2)

		titlePrefix := "✦ TOME CONTENT SEARCH "
		matchCount := ""
		if len(m.searchResults) > 0 {
			matchCount = fmt.Sprintf(" %d matches ", len(m.searchResults))
		}
		ruleLen := max(2, drawerInnerWidth-lipgloss.Width(titlePrefix)-lipgloss.Width(matchCount)-2)
		statusText := titlePrefix + strings.Repeat("─", ruleLen) + matchCount
		drawerTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.ColorFlameOrange).
			Render(statusText)

		m.searchInput.Width = max(20, drawerInnerWidth-6)
		inputRow := m.searchInput.View()

		var resultsLines []string
		query := strings.TrimSpace(m.searchInput.Value())

		if query == "" {
			resultsLines = append(resultsLines, ui.SubtitleStyle.Render("\n  Type keywords (e.g. dragon, quest, spell) to search across all notes in your vault."))
		} else if len(m.searchResults) == 0 {
			resultsLines = append(resultsLines, ui.SubtitleStyle.Render(fmt.Sprintf("\n  No notes found matching %q.", query)))
		} else {
			type EntrySpan struct {
				StartLine int
				EndLine   int
			}

			var allResultLines []string
			var spans []EntrySpan

			for idx, res := range m.searchResults {
				isSelected := (idx == m.searchSelected)
				start := len(allResultLines)

				header := formatBreadcrumb(res.RelPath, res.LineNum, isSelected, drawerInnerWidth-4)
				allResultLines = append(allResultLines, header)

				for _, c := range res.ContextLines {
					cLine := formatContextLine(c, query, isSelected, drawerInnerWidth-6)
					allResultLines = append(allResultLines, cLine)
				}

				end := len(allResultLines) - 1
				spans = append(spans, EntrySpan{StartLine: start, EndLine: end})

				// Clean blank line spacing between entries (no horizontal divider rule)
				if idx < len(m.searchResults)-1 {
					allResultLines = append(allResultLines, "")
				}
			}

			availHeight := max(4, drawerContentHeight-4)

			scrollOffset := 0
			if len(spans) > 0 && m.searchSelected >= 0 && m.searchSelected < len(spans) {
				span := spans[m.searchSelected]
				if span.EndLine >= scrollOffset+availHeight {
					scrollOffset = span.EndLine - availHeight + 1
				}
				if span.StartLine < scrollOffset {
					scrollOffset = span.StartLine
				}
				if scrollOffset < 0 {
					scrollOffset = 0
				}
			}

			visibleEnd := min(len(allResultLines), scrollOffset+availHeight)
			if scrollOffset < len(allResultLines) {
				resultsLines = allResultLines[scrollOffset:visibleEnd]
			}
		}

		drawerContent := fmt.Sprintf("%s\n\n%s\n\n%s", drawerTitle, inputRow, strings.Join(resultsLines, "\n"))

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
			clampedLines = append(clampedLines, padRightWithBg(l, innerContentWidth, ui.ColorDarkSlate))
		}
		for len(clampedLines) < drawerContentHeight {
			clampedLines = append(clampedLines, padRightWithBg("", innerContentWidth, ui.ColorDarkSlate))
		}

		searchBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(ui.ColorFlameOrange).
			BorderBackground(ui.ColorDarkSlate).
			Background(ui.ColorDarkSlate).
			Padding(0, 1)

		searchDrawerRendered := searchBoxStyle.
			Width(drawerInnerWidth).
			Height(drawerContentHeight).
			MaxHeight(m.searchHeight).
			Render(strings.Join(clampedLines, "\n"))

		drawerLines := strings.Split(searchDrawerRendered, "\n")
		if len(drawerLines) > m.searchHeight {
			drawerLines = drawerLines[:m.searchHeight]
		}
		for len(drawerLines) < m.searchHeight {
			drawerLines = append(drawerLines, padRightWithBg("", usableWidth, ui.ColorDarkSlate))
		}

		startRow := 2
		footerRow := len(bgLines) - 1

		for i, dLine := range drawerLines {
			targetRow := startRow + i
			if targetRow >= startRow && targetRow < footerRow {
				bgLines[targetRow] = dLine
			}
		}
	}

	// 8. Layered Floating Modal Overlay (New Note, New Folder, Quick Log, Rename, Delete, Move)
	if m.modalType != "" {
		modalWidth := min(54, usableWidth-4)
		innerModalWidth := max(10, modalWidth-6)
		modalBg := ui.ColorDarkSlate
		borderColor := ui.ColorFlameOrange

		homeDir, _ := os.UserHomeDir()
		m.modalInput.Width = max(20, innerModalWidth-2)

		var title string
		var bodyView string
		var actionHint string

		switch m.modalType {
		case "new_note":
			title = ui.GradientWithBg("✦  CREATE NOTE", "#FF4500", "#FFD166", modalBg)
			bodyView = m.modalInput.View()
			actionHint = "[Enter] Create  •  [Esc] Cancel"

		case "new_folder":
			title = ui.GradientWithBg("✦  CREATE FOLDER", "#FF4500", "#FFD166", modalBg)
			bodyView = m.modalInput.View()
			actionHint = "[Enter] Create  •  [Esc] Cancel"

		case "quick_log":
			title = ui.GradientWithBg("✦  QUICK LOG", "#FF4500", "#FFD166", modalBg)
			bodyView = m.modalInput.View()
			actionHint = "[Enter] Save  •  [Esc] Cancel"

		case "rename":
			itemType := "NOTE"
			if m.modalTargetItem != nil && m.modalTargetItem.IsFolder {
				itemType = "FOLDER"
			}
			title = ui.GradientWithBg(fmt.Sprintf("✦  RENAME %s", itemType), "#FF4500", "#FFD166", modalBg)
			bodyView = m.modalInput.View()
			actionHint = "[Enter] Rename  •  [Esc] Cancel"

		case "delete":
			borderColor = ui.ColorCrimson
			itemType := "NOTE"
			icon := "󰈙 "
			if m.modalTargetItem != nil && m.modalTargetItem.IsFolder {
				itemType = "FOLDER"
				icon = "📁 "
			}
			currName := ""
			if m.modalTargetItem != nil {
				currName = m.modalTargetItem.Title
			}
			title = lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorCrimson).
				Background(modalBg).
				Render(fmt.Sprintf("⚠  DELETE %s", itemType))
			bodyView = lipgloss.NewStyle().
				Bold(true).
				Foreground(ui.ColorTextBright).
				Background(ui.ColorSelectBg).
				Padding(0, 1).
				Render(icon + currName)
			actionHint = "[Enter / Y] Delete  •  [Esc / N] Cancel"

		case "move":
			title = ui.GradientWithBg("✦  MOVE ITEM", "#FF4500", "#FFD166", modalBg)
			actionHint = "[W/S/↑/↓] Select  •  [Enter] Move  •  [Esc] Cancel"

			maxRows := 5
			startIdx := 0
			if m.moveSelected >= maxRows {
				startIdx = m.moveSelected - maxRows + 1
			}
			endIdx := min(len(m.moveFolders), startIdx+maxRows)

			var folderLines []string
			for idx := startIdx; idx < endIdx; idx++ {
				fPath := m.moveFolders[idx]
				displayName := fPath
				if displayName == m.workspacePath {
					displayName = "~/Notes (Root)"
				} else if strings.HasPrefix(displayName, homeDir) {
					displayName = "~" + strings.TrimPrefix(displayName, homeDir)
				}
				if len(displayName) > innerModalWidth-6 {
					displayName = truncateWithDots(displayName, innerModalWidth-6)
				}

				if idx == m.moveSelected {
					cursor := ui.TreeCursor.Background(modalBg).Render("❯ ")
					pill := ui.ActiveBadgeStyle.Render("📁 " + displayName)
					folderLines = append(folderLines, cursor+pill)
				} else {
					folderLines = append(folderLines, "  "+ui.TreeNormalFolder.Background(modalBg).Render("📁 "+displayName))
				}
			}
			bodyView = strings.Join(folderLines, "\n")
		}

		hintStyle := ui.SubtitleStyle.Background(modalBg).Render(actionHint)

		var modalContentLines []string
		modalContentLines = append(modalContentLines, title)
		modalContentLines = append(modalContentLines, "")
		modalContentLines = append(modalContentLines, bodyView)
		if m.modalError != "" {
			errText := lipgloss.NewStyle().Foreground(ui.ColorCrimson).Background(modalBg).Bold(true).Render("⚠ " + m.modalError)
			modalContentLines = append(modalContentLines, "", errText)
		}
		modalContentLines = append(modalContentLines, "")
		modalContentLines = append(modalContentLines, hintStyle)

		var flatLines []string
		for _, item := range modalContentLines {
			for _, sub := range strings.Split(item, "\n") {
				flatLines = append(flatLines, sub)
			}
		}

		var paddedLines []string
		for _, l := range flatLines {
			if lipgloss.Width(l) > innerModalWidth {
				l = ansi.Truncate(l, innerModalWidth, "")
			}
			paddedLines = append(paddedLines, padRightWithBg(l, innerModalWidth, modalBg))
		}
		modalContent := strings.Join(paddedLines, "\n")

		modalBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(borderColor).
			BorderBackground(modalBg).
			Background(modalBg).
			Padding(1, 2)

		modalRendered := modalBoxStyle.Width(modalWidth - 2).Render(modalContent)
		modalLines := strings.Split(modalRendered, "\n")
		modalHeight := len(modalLines)
		startRow := max(2, (m.height-modalHeight)/2)

		mWidth := lipgloss.Width(modalLines[0])
		startCol := max(0, (usableWidth-mWidth)/2)

		for rIdx, mLine := range modalLines {
			targetRow := startRow + rIdx
			if targetRow >= 0 && targetRow < len(bgLines)-1 {
				bg := bgLines[targetRow]
				bgW := ansi.StringWidth(bg)
				if bgW < usableWidth {
					bg = bg + strings.Repeat(" ", usableWidth-bgW)
				}
				leftPart := ansi.Truncate(bg, startCol, "")
				leftPartW := ansi.StringWidth(leftPart)
				if leftPartW < startCol {
					leftPart = leftPart + strings.Repeat(" ", startCol-leftPartW)
				}
				rightPart := ansi.TruncateLeft(bg, startCol+mWidth, "")
				bgLines[targetRow] = leftPart + mLine + rightPart
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

func (m Model) getSelectedNode() *notes.Node {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
		actualID := m.visibleIndices[m.selectedIdx]
		for idx := range m.allNodes {
			if m.allNodes[idx].ID == actualID {
				return &m.allNodes[idx]
			}
		}
	}
	return nil
}

func (m Model) getDestinationDir() string {
	if node := m.getSelectedNode(); node != nil {
		if node.IsFolder {
			return node.Path
		}
		return filepath.Dir(node.Path)
	}
	return m.workspacePath
}

func (m *Model) openRenameModal() {
	item := m.getSelectedNode()
	if item == nil || item.Path == m.workspacePath {
		return
	}
	m.modalType = "rename"
	m.modalTargetItem = item
	m.modalError = ""
	m.modalInput.Reset()
	if item.IsFolder {
		m.modalInput.SetValue(item.Title)
		m.modalInput.Placeholder = "New folder name"
	} else {
		ext := filepath.Ext(item.FileName)
		baseName := strings.TrimSuffix(item.FileName, ext)
		m.modalInput.SetValue(baseName)
		m.modalInput.Placeholder = "New note title"
	}
	m.modalInput.Focus()
}

func (m *Model) confirmRename() {
	if m.modalTargetItem == nil {
		m.modalType = ""
		m.modalInput.Blur()
		return
	}
	newName := strings.TrimSpace(m.modalInput.Value())
	if newName == "" {
		m.modalType = ""
		m.modalTargetItem = nil
		m.modalInput.Blur()
		return
	}

	if strings.ContainsAny(newName, `\/:*?"<>|`) {
		m.modalError = `Name cannot contain \ / : * ? " < > |`
		return
	}

	oldPath := m.modalTargetItem.Path
	dir := filepath.Dir(oldPath)
	var newPath string
	if m.modalTargetItem.IsFolder {
		newPath = filepath.Join(dir, newName)
	} else {
		if !strings.HasSuffix(strings.ToLower(newName), ".md") {
			newName += ".md"
		}
		newPath = filepath.Join(dir, newName)
	}

	if filepath.Clean(newPath) == filepath.Clean(oldPath) {
		m.modalType = ""
		m.modalTargetItem = nil
		m.modalInput.Blur()
		return
	}

	if _, err := os.Stat(newPath); err == nil {
		m.modalError = "An item with that name already exists in this location."
		return
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		m.modalError = fmt.Sprintf("Error renaming: %v", err)
		return
	}

	m.modalType = ""
	m.modalTargetItem = nil
	m.modalError = ""
	m.modalInput.Blur()

	m.rescanTreePreservingExpansion(newPath)

	for vIdx, id := range m.visibleIndices {
		for idx := range m.allNodes {
			if m.allNodes[idx].ID == id && m.allNodes[idx].Path == newPath {
				m.selectedIdx = vIdx
				break
			}
		}
	}
	m.updateViewportPreview()
}

func (m *Model) openDeleteModal() {
	item := m.getSelectedNode()
	if item == nil || item.Path == m.workspacePath {
		return
	}
	m.modalType = "delete"
	m.modalTargetItem = item
	m.modalError = ""
}

func (m *Model) confirmDelete() {
	if m.modalTargetItem == nil {
		m.modalType = ""
		return
	}
	targetPath := m.modalTargetItem.Path
	if err := os.RemoveAll(targetPath); err != nil {
		m.modalError = fmt.Sprintf("Error deleting: %v", err)
		return
	}

	m.modalType = ""
	m.modalTargetItem = nil
	m.modalError = ""

	m.rescanTreePreservingExpansion("")

	if m.selectedIdx >= len(m.visibleIndices) {
		m.selectedIdx = max(0, len(m.visibleIndices)-1)
	}
	m.updateViewportPreview()
}

func (m *Model) openMoveModal() {
	item := m.getSelectedNode()
	if item == nil || item.Path == m.workspacePath {
		return
	}
	m.modalType = "move"
	m.modalTargetItem = item
	m.modalError = ""
	m.moveSelected = 0

	destFolders := []string{m.workspacePath}
	cleanItemPath := filepath.Clean(item.Path)
	for _, n := range m.allNodes {
		if n.IsFolder {
			cleanNodePath := filepath.Clean(n.Path)
			if cleanNodePath == cleanItemPath {
				continue
			}
			if item.IsFolder && strings.HasPrefix(cleanNodePath, cleanItemPath+string(filepath.Separator)) {
				continue
			}
			destFolders = append(destFolders, n.Path)
		}
	}
	m.moveFolders = destFolders

	currentParent := filepath.Clean(filepath.Dir(item.Path))
	for idx, f := range m.moveFolders {
		if filepath.Clean(f) == currentParent {
			m.moveSelected = idx
			break
		}
	}
}

func (m *Model) confirmMove() {
	if m.modalTargetItem == nil || len(m.moveFolders) == 0 || m.moveSelected < 0 || m.moveSelected >= len(m.moveFolders) {
		m.modalType = ""
		return
	}

	destDir := m.moveFolders[m.moveSelected]
	oldPath := m.modalTargetItem.Path
	currentParent := filepath.Dir(oldPath)

	if filepath.Clean(destDir) == filepath.Clean(currentParent) {
		m.modalType = ""
		m.modalTargetItem = nil
		return
	}

	newPath := filepath.Join(destDir, filepath.Base(oldPath))
	if _, err := os.Stat(newPath); err == nil {
		m.modalError = "An item with this name already exists in destination!"
		return
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		m.modalError = fmt.Sprintf("Error moving: %v", err)
		return
	}

	m.modalType = ""
	m.modalTargetItem = nil
	m.modalError = ""

	m.rescanTreePreservingExpansion(newPath)

	for vIdx, id := range m.visibleIndices {
		for idx := range m.allNodes {
			if m.allNodes[idx].ID == id && m.allNodes[idx].Path == newPath {
				m.selectedIdx = vIdx
				break
			}
		}
	}
	m.updateViewportPreview()
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

// rescanTreePreservingExpansion rescans the workspace directory while preserving
// the expanded state of all previously expanded folders, and ensures all ancestors of
// targetPath are expanded so the new item is immediately visible.
func (m *Model) rescanTreePreservingExpansion(targetPath string) {
	// 1. Remember all folders that the user currently has expanded
	expandedFolders := make(map[string]bool)
	for _, node := range m.allNodes {
		if node.IsFolder && node.Expanded {
			expandedFolders[filepath.Clean(node.Path)] = true
		}
	}

	// 2. Rescan the workspace from disk
	nodes, isObs, _ := notes.ScanTree(m.workspacePath, m.config.SortMode)
	m.allNodes = nodes
	m.isObsidian = isObs

	// 3. Restore previous expansion states and expand ancestors of targetPath
	cleanTarget := filepath.Clean(targetPath)
	for idx := range m.allNodes {
		if m.allNodes[idx].IsFolder {
			cleanNodePath := filepath.Clean(m.allNodes[idx].Path)

			// Keep previously expanded folders open
			if expandedFolders[cleanNodePath] {
				m.allNodes[idx].Expanded = true
			}

			// Also expand all ancestors of the target item so it's visible in the tree
			if cleanTarget != "" && cleanNodePath != cleanTarget {
				rel, err := filepath.Rel(cleanNodePath, cleanTarget)
				if err == nil && !strings.HasPrefix(rel, "..") {
					m.allNodes[idx].Expanded = true
				}
			}
		}
	}

	m.rebuildVisible()
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

	// Rescan tree while preserving all expanded folders, and select the new note
	m.rescanTreePreservingExpansion(targetPath)

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
	focusCmd := m.editor.Focus()
	scrollToBottom(&m.editor)
	return focusCmd
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

	// Rescan tree while preserving all expanded folders, and expand parent of new folder
	m.rescanTreePreservingExpansion(targetPath)

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

	// Rescan tree while preserving all expanded folders, and select quick log note
	m.rescanTreePreservingExpansion(targetPath)

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

func (m *Model) getNodeParentID(id int) int {
	for idx := range m.allNodes {
		if m.allNodes[idx].ID == id {
			return m.allNodes[idx].ParentID
		}
	}
	return -1
}

func (m *Model) performContentSearch(query string) {
	q := strings.TrimSpace(query)
	m.searchResults = nil
	m.searchSelected = 0
	if q == "" {
		return
	}

	qLower := strings.ToLower(q)
	for _, node := range m.allNodes {
		if node.IsFolder {
			continue
		}
		data, err := os.ReadFile(node.Path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for lineIdx, line := range lines {
			lineTrimmed := strings.TrimRight(line, "\r")
			if strings.Contains(strings.ToLower(lineTrimmed), qLower) {
				snippet := strings.TrimSpace(lineTrimmed)
				if snippet == "" {
					snippet = "(empty line match)"
				}
				// Build up to 3 context lines: [lineIdx-1, lineIdx, lineIdx+1]
				var contextLines []ContextLine
				startLine := max(0, lineIdx-1)
				endLine := min(len(lines)-1, lineIdx+1)
				for cIdx := startLine; cIdx <= endLine; cIdx++ {
					cText := strings.TrimRight(lines[cIdx], "\r")
					contextLines = append(contextLines, ContextLine{
						LineNum: cIdx + 1,
						Text:    cText,
						IsMatch: cIdx == lineIdx,
					})
				}

				m.searchResults = append(m.searchResults, SearchResult{
					NodeID:       node.ID,
					Path:         node.Path,
					Title:        node.Title,
					RelPath:      node.RelPath,
					LineNum:      lineIdx + 1,
					Snippet:      snippet,
					ContextLines: contextLines,
				})
				if len(m.searchResults) >= 40 {
					return
				}
			}
		}
	}
}

func formatBreadcrumb(relPath string, lineNum int, isSelected bool, maxWidth int) string {
	cleanRel := filepath.ToSlash(relPath)
	parts := strings.Split(cleanRel, "/")

	sep := lipgloss.NewStyle().Foreground(ui.ColorBorderMuted).Background(ui.ColorDarkSlate).Render(" › ")

	var breadParts []string
	if len(parts) > 1 {
		folderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C5A880")).Background(ui.ColorDarkSlate)
		for _, f := range parts[:len(parts)-1] {
			breadParts = append(breadParts, folderStyle.Render("📁 "+f))
		}
	}

	fileTitle := parts[len(parts)-1]
	fileStyle := lipgloss.NewStyle().Foreground(ui.ColorTextBright).Background(ui.ColorDarkSlate).Bold(true)
	if isSelected {
		fileStyle = lipgloss.NewStyle().Foreground(ui.ColorAmberGold).Background(ui.ColorDarkSlate).Bold(true)
	}
	breadParts = append(breadParts, fileStyle.Render("󰈙 "+fileTitle))

	lineBadge := lipgloss.NewStyle().Foreground(ui.ColorBorderMuted).Background(ui.ColorDarkSlate).Render(fmt.Sprintf(" :%d", lineNum))

	fullBread := strings.Join(breadParts, sep) + lineBadge
	if maxWidth > 6 && lipgloss.Width(fullBread) > maxWidth {
		fullBread = truncateWithDots(fullBread, maxWidth)
	}

	if isSelected {
		cursor := ui.TreeCursor.Background(ui.ColorDarkSlate).Render("❯ ")
		return cursor + fullBread
	}
	return "  " + fullBread
}

func formatContextLine(c ContextLine, query string, isSelected bool, maxWidth int) string {
	indent := "  "

	s := strings.TrimRight(c.Text, "\r")
	if strings.TrimSpace(s) == "" {
		s = " "
	}

	lineFg := lipgloss.Color("#8A8F9E")
	baseStyle := lipgloss.NewStyle().Faint(true).Foreground(lineFg).Background(ui.ColorDarkSlate)

	if !c.IsMatch {
		// Context line: faint / muted, soft slate text with background
		if maxWidth > 6 && lipgloss.Width(s) > maxWidth-6 {
			s = truncateWithDots(s, maxWidth-6)
		}
		return indent + baseStyle.Render(s)
	}

	// Match line
	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)
	sLower := strings.ToLower(s)
	idx := strings.Index(sLower, qLower)
	if idx == -1 || q == "" {
		if maxWidth > 6 && lipgloss.Width(s) > maxWidth-6 {
			s = truncateWithDots(s, maxWidth-6)
		}
		return indent + baseStyle.Render(s)
	}

	// Crop long line around match
	cropStart := max(0, idx-24)
	cropEnd := min(len(s), idx+len(q)+50)
	before := s[cropStart:idx]
	if cropStart > 0 {
		before = "…" + before
	}
	match := s[idx : idx+len(q)]
	after := s[idx+len(q) : cropEnd]
	if cropEnd < len(s) {
		after = after + "…"
	}

	// Match word highlight: same text color as the rest of the line, with subtle theme-appropriate amber glow background
	matchStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#482A16")).
		Foreground(lineFg)

	rendered := indent + baseStyle.Render(before) + matchStyle.Render(match) + baseStyle.Render(after)
	if maxWidth > 6 && lipgloss.Width(rendered) > maxWidth {
		rendered = truncateWithDots(rendered, maxWidth)
	}
	return rendered
}

// scrollToBottom primes the editor's viewport buffer and scrolls to the end of the note
func scrollToBottom(ta *textarea.Model) {
	_ = ta.View()
	ta.CursorEnd()
	*ta, _ = ta.Update(tea.KeyMsg{Type: tea.KeyDown})
}

// findCursorVisualRow scans the editor's rendered view to locate which visible row contains the cursor
func findCursorVisualRow(ta *textarea.Model) int {
	view := ta.View()
	vLines := strings.Split(view, "\n")
	for i, l := range vLines {
		if strings.Contains(l, "\x1b[7;") || strings.Contains(l, "\x1b[7m") {
			return i
		}
	}
	return -1
}

// handleEditorClick translates a mouse click to the closest line and column in the editor
func handleEditorClick(ta *textarea.Model, targetX, targetY int) {
	if targetX < 0 {
		targetX = 0
	}
	curVisualRow := findCursorVisualRow(ta)
	if curVisualRow < 0 {
		return
	}

	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	downMsg := tea.KeyMsg{Type: tea.KeyDown}

	if targetY > curVisualRow {
		for i := 0; i < targetY-curVisualRow; i++ {
			*ta, _ = ta.Update(downMsg)
		}
	} else if targetY < curVisualRow {
		for i := 0; i < curVisualRow-targetY; i++ {
			*ta, _ = ta.Update(upMsg)
		}
	}

	ta.SetCursor(targetX)
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func padRightWithBg(s string, width int, bg lipgloss.Color) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", width-w))
}

func renderCheatSheetContent(innerWidth int) string {
	bStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorTextBright)
	iStyle := lipgloss.NewStyle().Italic(true).Foreground(ui.ColorAmberGold)
	biStyle := lipgloss.NewStyle().Bold(true).Italic(true).Foreground(ui.ColorGoldGlow)
	sStyle := lipgloss.NewStyle().Strikethrough(true).Foreground(ui.ColorTextMuted)
	hStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.ColorFlameOrange)
	uStyle := lipgloss.NewStyle().Underline(true).Foreground(ui.ColorTextBright)
	cStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan)
	lStyle := lipgloss.NewStyle().Foreground(ui.ColorCyan).Underline(true)
	gStyle := lipgloss.NewStyle().Foreground(ui.ColorGreen).Bold(true)
	headStyle := lipgloss.NewStyle().Foreground(ui.ColorGoldGlow).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(ui.ColorAmberGold).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ui.ColorTextMuted)

	col1W := 26
	col2W := 22

	rows := [][]string{
		{"**bold** or __bold__", bStyle.Render("bold"), "Bold text"},
		{"*italic* or _italic_", iStyle.Render("italic"), "Italic text"},
		{"***bold italic***", biStyle.Render("bold italic"), "Bold & Italic"},
		{"~~strike~~", sStyle.Render("strike"), "Strikethrough"},
		{"==highlight==", hStyle.Render("highlight"), "Flame highlight"},
		{"<u>underline</u>", uStyle.Render("underline"), "Underline text"},
		{"`code`", cStyle.Render("code"), "Inline code snippet"},
		{"[[Link]] or [[Link|Alias]]", lStyle.Render("󰌹 Alias"), "Obsidian wiki-link"},
		{"- [ ] / - [x]", gStyle.Render("○") + " / " + gStyle.Render("✔"), "Task checklist"},
		{"- Item  (indent:   - )", ui.TreeCursor.Render("•") + " Item", "Nested bullet lists"},
		{"# H1  ## H2  ### H3", headStyle.Render("Heading"), "Headings with rule"},
		{"> [!NOTE] / [!TIP] / [!WARN]", cStyle.Render("▎ NOTE"), "Obsidian callout box"},
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  %s %s %s\n",
		padRight(keyStyle.Render("MARKDOWN SYNTAX"), col1W),
		padRight(keyStyle.Render("PREVIEW"), col2W),
		keyStyle.Render("DESCRIPTION")))

	divLen := max(10, innerWidth-6)
	sb.WriteString("  " + lipgloss.NewStyle().Foreground(ui.ColorBorderMuted).Render(strings.Repeat("─", divLen)) + "\n")

	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("  %s %s %s\n",
			padRight(r[0], col1W),
			padRight(r[1], col2W),
			descStyle.Render(r[2])))
	}

	return sb.String()
}

type versionCheckMsg struct {
	latestVersion string
	hasUpdate     bool
	err           error
}

func checkGitHubUpdateCmd() tea.Msg {
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/jolobtw/tome/releases/latest", nil)
	if err != nil {
		return versionCheckMsg{err: err}
	}
	req.Header.Set("User-Agent", "tome-app")
	resp, err := client.Do(req)
	if err != nil {
		return versionCheckMsg{err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return versionCheckMsg{err: fmt.Errorf("status: %d", resp.StatusCode)}
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return versionCheckMsg{err: err}
	}

	latest := strings.TrimSpace(release.TagName)
	hasUpdate := isNewerVersion(AppVersion, latest)
	return versionCheckMsg{latestVersion: latest, hasUpdate: hasUpdate}
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	var res []int
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			break
		}
		res = append(res, n)
	}
	for len(res) < 3 {
		res = append(res, 0)
	}
	return res
}

func isNewerVersion(current, latest string) bool {
	c := parseVersion(current)
	l := parseVersion(latest)
	for i := 0; i < 3; i++ {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}
