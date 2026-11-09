package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jolobtw/tome/internal/notes"
	"github.com/jolobtw/tome/internal/ui"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v1.0.0", "v0.9.9", false},
		{"0.1.0", "v0.1.1", true},
		{"v1.0.0", "v1.0.1", true},
		{"v1.2.0", "v1.1.9", false},
	}

	for _, tt := range tests {
		result := isNewerVersion(tt.current, tt.latest)
		if result != tt.expected {
			t.Errorf("isNewerVersion(%q, %q) = %v; want %v", tt.current, tt.latest, result, tt.expected)
		}
	}
}

func TestParseVersion(t *testing.T) {
	parsed := parseVersion("v1.2.3")
	if len(parsed) != 3 || parsed[0] != 1 || parsed[1] != 2 || parsed[2] != 3 {
		t.Fatalf("unexpected parseVersion output: %v", parsed)
	}
}

func TestWrapFooterItems(t *testing.T) {
	items := []string{
		"[Ctrl+S] Save  ",
		"[Esc] Finish  ",
		"[Ctrl+Q] Discard  ",
		"[F1] Guide  ",
	}

	// Wide width should keep it in 1 line
	wide := wrapFooterItems(items, 200)
	if strings.Contains(wide, "\n") {
		t.Errorf("expected 1 line for wide width, got: %q", wide)
	}

	// Narrow width should wrap into multiple lines
	narrow := wrapFooterItems(items, 25)
	if !strings.Contains(narrow, "\n") {
		t.Errorf("expected multiple lines for narrow width, got: %q", narrow)
	}
}

func TestFooterHeightCalculation(t *testing.T) {
	m := Model{
		activePane: 0,
		ready:      true,
	}

	hWide := m.getFooterHeight(200)
	hNarrow := m.getFooterHeight(40)

	if hWide < 1 {
		t.Errorf("expected hWide >= 1, got %d", hWide)
	}
	if hNarrow <= hWide {
		t.Errorf("expected hNarrow (%d) > hWide (%d)", hNarrow, hWide)
	}
}

func TestTreeGuideStyle(t *testing.T) {
	renderedGuide := ui.TreeGuide.Render("│  ├─ ")
	if renderedGuide == "" {
		t.Errorf("expected rendered guide lines, got empty string")
	}
}

func TestDimANSILine(t *testing.T) {
	// Plain text should be wrapped in dimmed foreground
	plain := "Hello world note preview"
	dimmedPlain := ui.DimANSILine(plain, 0.35)
	if !strings.Contains(dimmedPlain, "\x1b[38;2;") || !strings.Contains(dimmedPlain, "Hello world") {
		t.Errorf("expected plain text to be wrapped in dimmed foreground, got: %q", dimmedPlain)
	}

	// Bold compound text should convert 1 to 22 and dim RGB colors
	compound := "\x1b[1;38;2;255;255;255;48;2;38;40;56m1. Terra Campaign\x1b[0m"
	dimmedCompound := ui.DimANSILine(compound, 0.35)
	if strings.Contains(dimmedCompound, "38;2;255;255;255") {
		t.Errorf("expected bright white to be dimmed, got: %q", dimmedCompound)
	}
	if strings.Contains(dimmedCompound, "[1;") {
		t.Errorf("expected bold code 1 to be removed or converted to 22, got: %q", dimmedCompound)
	}
}

func TestGetTreeNodeArrow(t *testing.T) {
	tests := []struct {
		name        string
		isFolder    bool
		hasChildren bool
		isExpanded  bool
		expected    string
	}{
		{"Folder with children collapsed", true, true, false, "▸ "},
		{"Folder with children expanded", true, true, true, "▾ "},
		{"Empty folder collapsed", true, false, false, "  "},
		{"Empty folder expanded", true, false, true, "  "},
		{"Note file", false, false, false, "  "},
		{"Note file with children flag false", false, true, false, "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTreeNodeArrow(tt.isFolder, tt.hasChildren, tt.isExpanded)
			if got != tt.expected {
				t.Errorf("getTreeNodeArrow(%v, %v, %v) = %q; want %q",
					tt.isFolder, tt.hasChildren, tt.isExpanded, got, tt.expected)
			}
		})
	}
}

func TestFormatTreeNodeDisplay(t *testing.T) {
	folderWithChildren := &notes.Node{
		Title:    "Lore",
		IsFolder: true,
		ChildIDs: []int{1, 2},
		Expanded: false,
	}
	resCollapsed := formatTreeNodeDisplay(folderWithChildren, false)
	if !strings.HasPrefix(resCollapsed, "▸  Lore") {
		t.Errorf("expected collapsed folder to start with '▸  Lore', got %q", resCollapsed)
	}

	folderExpanded := &notes.Node{
		Title:    "Lore",
		IsFolder: true,
		ChildIDs: []int{1, 2},
		Expanded: true,
	}
	resExpanded := formatTreeNodeDisplay(folderExpanded, false)
	if !strings.HasPrefix(resExpanded, "▾  Lore") {
		t.Errorf("expected expanded folder to start with '▾  Lore', got %q", resExpanded)
	}

	emptyFolder := &notes.Node{
		Title:    "Empty",
		IsFolder: true,
		ChildIDs: nil,
		Expanded: false,
	}
	resEmpty := formatTreeNodeDisplay(emptyFolder, false)
	if !strings.HasPrefix(resEmpty, "   Empty") {
		t.Errorf("expected empty folder to start with '   Empty', got %q", resEmpty)
	}

	noteNode := &notes.Node{
		Title:    "QuickLog",
		IsFolder: false,
	}
	resNote := formatTreeNodeDisplay(noteNode, false)
	if !strings.HasPrefix(resNote, "  󰈙 QuickLog") {
		t.Errorf("expected note to start with '  󰈙 QuickLog', got %q", resNote)
	}

	// Filter active mode
	noteInSubdir := &notes.Node{
		Title:    "World",
		RelPath:  "Lore/World.md",
		IsFolder: false,
	}
	resFiltered := formatTreeNodeDisplay(noteInSubdir, true)
	if !strings.Contains(resFiltered, "  󰈙 World (Lore)") {
		t.Errorf("expected filtered note to show directory suffix, got %q", resFiltered)
	}
}

func TestFillLineBg(t *testing.T) {
	bg := ui.ColorDarkSlate // #13141C -> \x1b[48;2;19;20;28m
	raw := "\x1b[1;38;2;255;121;0m🔍 \x1b[0mS\x1b[0mearch...                  "
	filled := ui.FillLineBg(raw, 40, bg)

	// Specifically verify that inner \x1b[0m resets re-assert the background sequence
	if !strings.Contains(filled, "\x1b[0m\x1b[48;2;19;20;28m") {
		t.Errorf("expected inner ANSI reset to re-assert background sequence, got: %q", filled)
	}

	if lipgloss.Width(filled) != 40 {
		t.Errorf("expected visual width 40, got %d", lipgloss.Width(filled))
	}
}

func TestDrawerPlaceholdersAndEmptyStates(t *testing.T) {
	si := newSearchInput()
	if si.Placeholder != "Search note contents..." {
		t.Errorf("expected clean search placeholder, got %q", si.Placeholder)
	}
	if strings.Contains(si.Placeholder, "dragon") || strings.Contains(si.Placeholder, "spell") {
		t.Errorf("search placeholder contains redundant keyword examples: %q", si.Placeholder)
	}

	ai := newAIInput()
	if ai.Placeholder != "Ask a question about your notes..." {
		t.Errorf("expected clean AI placeholder, got %q", ai.Placeholder)
	}
	if strings.Contains(ai.Placeholder, "sync") {
		t.Errorf("AI placeholder contains redundant sync examples: %q", ai.Placeholder)
	}
}

func TestBuildNewNoteDetails(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantFileName    string
		wantTitle       string
		wantContent     string
		wantErr         bool
		errContains     string
	}{
		{
			name:         "Standard note name with spaces and casing",
			input:        "Meeting Notes with Team",
			wantFileName: "Meeting Notes with Team.md",
			wantTitle:    "Meeting Notes with Team",
			wantContent:  "# Meeting Notes with Team\n\n",
			wantErr:      false,
		},
		{
			name:         "Note name with .md extension already typed",
			input:        "Project Roadmap.md",
			wantFileName: "Project Roadmap.md",
			wantTitle:    "Project Roadmap",
			wantContent:  "# Project Roadmap\n\n",
			wantErr:      false,
		},
		{
			name:         "Note name with leading and trailing whitespace",
			input:        "   Arcane Spells   ",
			wantFileName: "Arcane Spells.md",
			wantTitle:    "Arcane Spells",
			wantContent:  "# Arcane Spells\n\n",
			wantErr:      false,
		},
		{
			name:        "Empty input",
			input:       "   ",
			wantErr:     true,
			errContains: "empty note name",
		},
		{
			name:        "Only .md typed",
			input:       ".md",
			wantErr:     true,
			errContains: "empty note name",
		},
		{
			name:        "Invalid filename characters",
			input:       "Plan: Part 1",
			wantErr:     true,
			errContains: "cannot contain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName, title, content, err := buildNewNoteDetails(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if fileName != tt.wantFileName {
				t.Errorf("fileName = %q; want %q", fileName, tt.wantFileName)
			}
			if title != tt.wantTitle {
				t.Errorf("title = %q; want %q", title, tt.wantTitle)
			}
			if content != tt.wantContent {
				t.Errorf("content = %q; want %q", content, tt.wantContent)
			}

			// Ensure Obsidian tags and frontmatter are NOT present
			if strings.Contains(content, "tags:") {
				t.Errorf("initial content should not contain obsidian tags: %q", content)
			}
			if strings.Contains(content, "---") {
				t.Errorf("initial content should not contain frontmatter dashes: %q", content)
			}
		})
	}
}

func TestWelcomeNoteSeedingWithRootNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-welcome-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	nodes, _, err := notes.ScanTree(tempDir, "alpha")
	if err != nil {
		t.Fatalf("unexpected ScanTree error: %v", err)
	}

	// Workspace is empty if len(nodes) == 0 or (len(nodes) == 1 && len(nodes[0].ChildIDs) == 0)
	isEmpty := len(nodes) == 0 || (len(nodes) == 1 && len(nodes[0].ChildIDs) == 0)
	if !isEmpty {
		t.Errorf("expected newly created directory to be detected as empty workspace")
	}

	welcomePath := filepath.Join(tempDir, "Welcome to Tome.md")
	if err := os.WriteFile(welcomePath, []byte("# Welcome to Tome\n"), 0644); err != nil {
		t.Fatalf("failed to write welcome note: %v", err)
	}

	nodesAfter, _, err := notes.ScanTree(tempDir, "alpha")
	if err != nil {
		t.Fatalf("unexpected ScanTree error: %v", err)
	}
	isEmptyAfter := len(nodesAfter) == 0 || (len(nodesAfter) == 1 && len(nodesAfter[0].ChildIDs) == 0)
	if isEmptyAfter {
		t.Errorf("expected workspace with welcome note to NOT be empty")
	}
	if len(nodesAfter) != 2 {
		t.Errorf("expected 2 nodes (root + welcome note), got %d", len(nodesAfter))
	}
}

func TestRootNodeFormattingAndIcon(t *testing.T) {
	rootNodeWithChildren := &notes.Node{
		Title:    "MyNotebook",
		IsFolder: true,
		ParentID: -1,
		Level:    0,
		ChildIDs: []int{1, 2},
		Expanded: true,
	}

	display := formatTreeNodeDisplay(rootNodeWithChildren, false)
	if !strings.HasPrefix(display, "▾ 󰉋 MyNotebook") {
		t.Errorf("expected expanded root node to start with '▾ 󰉋 MyNotebook', got %q", display)
	}

	rootNodeCollapsed := &notes.Node{
		Title:    "MyNotebook",
		IsFolder: true,
		ParentID: -1,
		Level:    0,
		ChildIDs: []int{1, 2},
		Expanded: false,
	}
	displayCollapsed := formatTreeNodeDisplay(rootNodeCollapsed, false)
	if !strings.HasPrefix(displayCollapsed, "▸ 󰉋 MyNotebook") {
		t.Errorf("expected collapsed root node to start with '▸ 󰉋 MyNotebook', got %q", displayCollapsed)
	}
}

func TestRootNodeDestinationDirAndGuards(t *testing.T) {
	wsPath := "C:\\Users\\test\\vault"
	m := Model{
		workspacePath:  wsPath,
		workspaceName:  "vault",
		visibleIndices: []int{0},
		selectedIdx:    0,
		allNodes: []notes.Node{
			{
				ID:       0,
				Title:    "vault",
				Path:     wsPath,
				IsFolder: true,
				ParentID: -1,
				Level:    0,
			},
		},
	}

	destDir := m.getDestinationDir()
	if destDir != wsPath {
		t.Errorf("getDestinationDir() = %q; want %q", destDir, wsPath)
	}

	// Verify rename, delete, move modals do not activate for root node
	m.openRenameModal()
	if m.modalType != "" {
		t.Errorf("expected rename modal to remain closed for root node, got %q", m.modalType)
	}

	m.openDeleteModal()
	if m.modalType != "" {
		t.Errorf("expected delete modal to remain closed for root node, got %q", m.modalType)
	}

	m.openMoveModal()
	if m.modalType != "" {
		t.Errorf("expected move modal to remain closed for root node, got %q", m.modalType)
	}
}

func TestRootNodeCollapseInVisibleIndices(t *testing.T) {
	wsPath := "C:\\Users\\test\\vault"
	m := Model{
		workspacePath: wsPath,
		workspaceName: "vault",
		allNodes: []notes.Node{
			{
				ID:       0,
				Title:    "vault",
				Path:     wsPath,
				IsFolder: true,
				ParentID: -1,
				Level:    0,
				Expanded: true,
				ChildIDs: []int{1},
			},
			{
				ID:       1,
				Title:    "Note1.md",
				Path:     wsPath + "\\Note1.md",
				IsFolder: false,
				ParentID: 0,
				Level:    1,
			},
		},
	}

	m.rebuildVisible()
	if len(m.visibleIndices) != 2 {
		t.Fatalf("expected 2 visible items when root is expanded, got %d", len(m.visibleIndices))
	}

	// Collapse root
	m.allNodes[0].Expanded = false
	m.rebuildVisible()
	if len(m.visibleIndices) != 1 {
		t.Fatalf("expected 1 visible item when root is collapsed, got %d", len(m.visibleIndices))
	}
	if m.visibleIndices[0] != 0 {
		t.Errorf("expected visible item to be root node (ID 0), got %d", m.visibleIndices[0])
	}
}

func TestOpenMoveModalNoDuplicateRootFolder(t *testing.T) {
	wsPath := "C:\\Users\\test\\vault"
	subPath := filepath.Join(wsPath, "SubFolder")
	m := Model{
		workspacePath:  wsPath,
		workspaceName:  "vault",
		visibleIndices: []int{0, 1},
		selectedIdx:    1,
		allNodes: []notes.Node{
			{
				ID:       0,
				Title:    "vault",
				Path:     wsPath,
				IsFolder: true,
				ParentID: -1,
				Level:    0,
			},
			{
				ID:       1,
				Title:    "SubFolder",
				Path:     subPath,
				IsFolder: true,
				ParentID: 0,
				Level:    1,
			},
		},
	}

	m.openMoveModal()
	countRoot := 0
	cleanRoot := filepath.Clean(wsPath)
	for _, f := range m.moveFolders {
		if filepath.Clean(f) == cleanRoot {
			countRoot++
		}
	}
	if countRoot != 1 {
		t.Errorf("expected root folder to appear in moveFolders exactly once, got %d times in %v", countRoot, m.moveFolders)
	}
}

func TestRescanTreePreservesCollapsedRoot(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-rescan-collapsed-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subFolder := filepath.Join(tempDir, "SubFolder")
	if err := os.Mkdir(subFolder, 0755); err != nil {
		t.Fatalf("failed to create subfolder: %v", err)
	}

	m := Model{
		workspacePath: tempDir,
		workspaceName: filepath.Base(tempDir),
	}
	nodes, _, err := notes.ScanTree(tempDir, "alpha")
	if err != nil {
		t.Fatalf("unexpected ScanTree error: %v", err)
	}
	m.allNodes = nodes

	// User collapses root notebook
	m.allNodes[0].Expanded = false

	// Trigger rescan
	m.rescanTreePreservingExpansion("")

	if m.allNodes[0].Expanded {
		t.Errorf("expected root node to remain collapsed after rescanTreePreservingExpansion")
	}
}

func TestCopyPathKey(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	wsPath := filepath.Join(homeDir, "Notes")
	notePath := filepath.Join(wsPath, "daily.md")

	m := Model{
		activePane:     0,
		workspacePath:  wsPath,
		workspaceName:  "Notes",
		visibleIndices: []int{0, 1},
		selectedIdx:    1,
		allNodes: []notes.Node{
			{
				ID:       0,
				Title:    "Notes",
				Path:     wsPath,
				IsFolder: true,
				ParentID: -1,
				Level:    0,
			},
			{
				ID:       1,
				Title:    "daily",
				Path:     notePath,
				IsFolder: false,
				ParentID: 0,
				Level:    1,
			},
		},
	}

	// Pressing "c"
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model := updated.(Model)
	if !strings.Contains(model.statusMsg, "Copied:") {
		t.Fatalf("expected statusMsg to contain 'Copied:', got: %q", model.statusMsg)
	}
	if !strings.Contains(model.statusMsg, "~/Notes/daily.md") {
		t.Fatalf("expected statusMsg to contain '~/Notes/daily.md', got: %q", model.statusMsg)
	}
	if cmd == nil {
		t.Fatalf("expected timer cmd to clear statusMsg, got nil")
	}

	// Delivering clearStatusMsg resets statusMsg
	cleared, _ := model.Update(clearStatusMsg{})
	if cleared.(Model).statusMsg != "" {
		t.Errorf("expected statusMsg to be cleared, got: %q", cleared.(Model).statusMsg)
	}
}

func TestCopyPathRootNotebook(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	wsPath := filepath.Join(homeDir, "Notes")

	m := Model{
		activePane:     0,
		workspacePath:  wsPath,
		workspaceName:  "Notes",
		visibleIndices: []int{0},
		selectedIdx:    0,
		allNodes: []notes.Node{
			{
				ID:       0,
				Title:    "Notes",
				Path:     wsPath,
				IsFolder: true,
				ParentID: -1,
				Level:    0,
			},
		},
	}

	// Pressing "c" on root notebook
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model := updated.(Model)
	if !strings.Contains(model.statusMsg, "~/Notes") {
		t.Fatalf("expected statusMsg to contain '~/Notes', got: %q", model.statusMsg)
	}
}

func TestFooterIncludesCopyPath(t *testing.T) {
	m := Model{
		activePane: 0,
	}
	items := m.getFooterItems()
	found := false
	for _, it := range items {
		if strings.Contains(it, "[C]") && strings.Contains(it, "Copy Path") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected footer to contain [C] Copy Path in tree pane")
	}
}

func TestPreviewStartsAtTopOnSelection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-preview-top-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "note.md")
	var lines []string
	for i := 1; i <= 60; i++ {
		lines = append(lines, fmt.Sprintf("Line %d: This is sample note content.", i))
	}
	if err := os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	m := Model{
		width:          80,
		height:         24,
		ready:          true,
		activePane:     0,
		editor:         textarea.New(),
		workspacePath:  tempDir,
		workspaceName:  "Test",
		visibleIndices: []int{0},
		selectedIdx:    0,
		allNodes: []notes.Node{
			{
				ID:       0,
				Title:    "note",
				Path:     filePath,
				IsFolder: false,
			},
		},
	}

	m.recalculateViewport()
	if m.viewport.YOffset != 0 {
		t.Errorf("expected preview to start at top (YOffset == 0), got: %d", m.viewport.YOffset)
	}

	// Scrolling down and re-selecting should also reset preview to the top
	m.viewport.GotoBottom()
	if m.viewport.YOffset == 0 {
		t.Fatalf("expected GotoBottom to have non-zero YOffset")
	}

	m.updateViewportPreview()
	if m.viewport.YOffset != 0 {
		t.Errorf("expected updateViewportPreview to reset YOffset to 0, got: %d", m.viewport.YOffset)
	}
}

func TestPreviewMouseWheelScroll1Line(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-preview-wheel-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "note.md")
	var lines []string
	for i := 1; i <= 60; i++ {
		lines = append(lines, fmt.Sprintf("Line %d content", i))
	}
	if err := os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	m := Model{
		width:          80,
		height:         24,
		ready:          true,
		activePane:     0,
		editor:         textarea.New(),
		workspacePath:  tempDir,
		workspaceName:  "Test",
		visibleIndices: []int{0},
		selectedIdx:    0,
		allNodes: []notes.Node{
			{
				ID:       0,
				Title:    "note",
				Path:     filePath,
				IsFolder: false,
			},
		},
	}
	m.recalculateViewport()

	// Preview pane X coordinate (leftTotalWidth is max(34, min(52, 78*36/100)) = 34)
	previewX := 45
	mouseWheelDown := tea.MouseMsg{
		Type:   tea.MouseWheelDown,
		X:      previewX,
		Y:      10,
		Button: tea.MouseButtonWheelDown,
	}

	updated, _ := m.Update(mouseWheelDown)
	m1 := updated.(Model)
	if m1.viewport.YOffset != 1 {
		t.Errorf("expected YOffset to be 1 after 1 wheel down scroll, got: %d", m1.viewport.YOffset)
	}

	updated, _ = m1.Update(mouseWheelDown)
	m2 := updated.(Model)
	if m2.viewport.YOffset != 2 {
		t.Errorf("expected YOffset to be 2 after second wheel down scroll, got: %d", m2.viewport.YOffset)
	}

	mouseWheelUp := tea.MouseMsg{
		Type:   tea.MouseWheelUp,
		X:      previewX,
		Y:      10,
		Button: tea.MouseButtonWheelUp,
	}
	updated, _ = m2.Update(mouseWheelUp)
	m3 := updated.(Model)
	if m3.viewport.YOffset != 1 {
		t.Errorf("expected YOffset to be 1 after wheel up scroll, got: %d", m3.viewport.YOffset)
	}
}

func TestTopAndBottomKeybinds(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-preview-nav-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "note.md")
	var lines []string
	for i := 1; i <= 60; i++ {
		lines = append(lines, fmt.Sprintf("Line %d content", i))
	}
	if err := os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write test note: %v", err)
	}

	m := Model{
		width:          80,
		height:         24,
		ready:          true,
		activePane:     0,
		editor:         textarea.New(),
		workspacePath:  tempDir,
		workspaceName:  "Test",
		visibleIndices: []int{0},
		selectedIdx:    0,
		allNodes: []notes.Node{
			{
				ID:       0,
				Title:    "note",
				Path:     filePath,
				IsFolder: false,
			},
		},
	}
	m.recalculateViewport()

	// 1. From Tree Pane (activePane == 0), press 'b' to jump note preview to bottom
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	mTreeBottom := updated.(Model)
	if mTreeBottom.viewport.YOffset == 0 || !mTreeBottom.viewport.AtBottom() {
		t.Errorf("expected 'b' in tree pane to scroll preview to bottom, got YOffset: %d", mTreeBottom.viewport.YOffset)
	}

	// Press 't' in tree pane to jump back to top
	updated, _ = mTreeBottom.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	mTreeTop := updated.(Model)
	if mTreeTop.viewport.YOffset != 0 {
		t.Errorf("expected 't' in tree pane to scroll preview to top, got YOffset: %d", mTreeTop.viewport.YOffset)
	}

	// Test ctrl+end and ctrl+home in tree pane
	updated, _ = mTreeTop.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	mTreeCtrlEnd := updated.(Model)
	if mTreeCtrlEnd.viewport.YOffset == 0 {
		t.Errorf("expected ctrl+end in tree pane to scroll preview to bottom")
	}

	updated, _ = mTreeCtrlEnd.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})
	mTreeCtrlHome := updated.(Model)
	if mTreeCtrlHome.viewport.YOffset != 0 {
		t.Errorf("expected ctrl+home in tree pane to scroll preview to top")
	}

	// 2. Switch to Preview Pane (activePane == 1)
	mPreview := mTreeTop
	mPreview.activePane = 1

	// Test 'b' and 't'
	updated, _ = mPreview.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if updated.(Model).viewport.YOffset == 0 {
		t.Errorf("expected 'b' in preview pane to scroll to bottom")
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if updated.(Model).viewport.YOffset != 0 {
		t.Errorf("expected 't' in preview pane to scroll to top")
	}

	// Test 'G' and 'g'
	updated, _ = mPreview.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if updated.(Model).viewport.YOffset == 0 {
		t.Errorf("expected 'G' in preview pane to scroll to bottom")
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if updated.(Model).viewport.YOffset != 0 {
		t.Errorf("expected 'g' in preview pane to scroll to top")
	}

	// Test 'End' and 'Home'
	updated, _ = mPreview.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if updated.(Model).viewport.YOffset == 0 {
		t.Errorf("expected 'End' in preview pane to scroll to bottom")
	}
	updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyHome})
	if updated.(Model).viewport.YOffset != 0 {
		t.Errorf("expected 'Home' in preview pane to scroll to top")
	}
}

func TestFooterIncludesTopEndKeybinds(t *testing.T) {
	// Tree pane footer
	mTree := Model{activePane: 0}
	itemsTree := mTree.getFooterItems()
	foundTree := false
	for _, it := range itemsTree {
		if strings.Contains(it, "[T/B]") && strings.Contains(it, "Note Top/End") {
			foundTree = true
			break
		}
	}
	if !foundTree {
		t.Errorf("expected tree pane footer to include [T/B] Note Top/End")
	}

	// Preview pane footer
	mPreview := Model{activePane: 1}
	itemsPreview := mPreview.getFooterItems()
	foundPreview := false
	for _, it := range itemsPreview {
		if strings.Contains(it, "[Home/End/T/B]") && strings.Contains(it, "Top/End") {
			foundPreview = true
			break
		}
	}
	if !foundPreview {
		t.Errorf("expected preview pane footer to include [Home/End/T/B] Top/End")
	}
}


