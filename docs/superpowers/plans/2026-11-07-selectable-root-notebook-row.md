# Selectable Root Notebook Row Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the root notebook as the topmost selectable row in Tome's navigation sidebar so users can highlight it to create notes and folders directly at the top level of the notebook.

**Architecture:** Update `ScanTree` to produce a root `Node` (Level 0, ParentID -1) representing the notebook directory itself, with all vault entries nested underneath (Level 1+). Update tree rendering, folder preview cards, and workspace initialization to support the root node seamlessly while keeping safety guards active.

**Tech Stack:** Go 1.22+, Bubble Tea (`github.com/charmbracelet/bubbletea`), Lip Gloss (`github.com/charmbracelet/lipgloss`).

**Spec:** `docs/superpowers/specs/2026-11-07-selectable-root-notebook-row-design.md`

## Global Constraints

- Root node path must match `workspacePath` exactly.
- Root node must have `ParentID == -1`, `Level == 0`, and `IsFolder == true`.
- Actions `openRenameModal`, `openDeleteModal`, and `openMoveModal` must never operate on the root node.
- New note (`N`) and new folder (`F`) creation with root node selected must target `workspacePath`.
- All code changes must pass `go test ./...`.
- Code changes must be committed and pushed to GitHub per user rules.

## Review Focus

1. **Workspace with only subfolders and no root notes:** Selecting root node allows `getDestinationDir()` to return `workspacePath`. Tested in Task 3.
2. **Completely empty workspace:** Root node is created with 0 children, and `initialModel` seeds the welcome note without crashing. Tested in Task 2.
3. **Collapsing/expanding root node:** Toggling root node collapse hides all children from `visibleIndices`. Tested in Task 3.
4. **Safety guards:** Rename, delete, and move modals remain strictly blocked on root node. Tested in Task 3.
5. **Continuous tree lines:** Direct children under root (`Level: 1`) show branch lines (`├─ `, `└─ `) flush to margin without indentation. Tested in Task 3.

---

### Task 1: Root Node Generation in Scanner

**Files:**
- Modify: `internal/notes/scanner.go:28-173`
- Create: `internal/notes/scanner_test.go`

**Interfaces:**
- Consumes: `notes.ScanTree(root string, sortMode string) ([]Node, bool, error)`
- Produces: `allNodes` containing root node as the first element (`ID: 0`, `Level: 0`, `ParentID: -1`, `IsFolder: true`, `Expanded: true`, `Path: root`) and all vault items having `ParentID: 0` or their respective parent folder ID.

- [ ] **Step 1: Write the failing tests in `internal/notes/scanner_test.go`**

```go
package notes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanTreeIncludesRootNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-scanner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subFolder := filepath.Join(tempDir, "SubFolder")
	if err := os.Mkdir(subFolder, 0755); err != nil {
		t.Fatalf("failed to create subfolder: %v", err)
	}
	notePath := filepath.Join(subFolder, "TestNote.md")
	if err := os.WriteFile(notePath, []byte("# Test Note\n"), 0644); err != nil {
		t.Fatalf("failed to create test note: %v", err)
	}

	nodes, isObs, err := ScanTree(tempDir, "alpha")
	if err != nil {
		t.Fatalf("unexpected ScanTree error: %v", err)
	}
	if isObs {
		t.Errorf("expected isObs to be false")
	}

	if len(nodes) < 1 {
		t.Fatalf("expected at least 1 node (root), got %d", len(nodes))
	}

	rootNode := nodes[0]
	if rootNode.ID != 0 {
		t.Errorf("expected rootNode.ID == 0, got %d", rootNode.ID)
	}
	if rootNode.ParentID != -1 {
		t.Errorf("expected rootNode.ParentID == -1, got %d", rootNode.ParentID)
	}
	if rootNode.Level != 0 {
		t.Errorf("expected rootNode.Level == 0, got %d", rootNode.Level)
	}
	if !rootNode.IsFolder {
		t.Errorf("expected rootNode.IsFolder == true")
	}
	if !rootNode.Expanded {
		t.Errorf("expected rootNode.Expanded == true")
	}
	if rootNode.Path != tempDir {
		t.Errorf("expected rootNode.Path == %q, got %q", tempDir, rootNode.Path)
	}
	if rootNode.NoteCount != 1 {
		t.Errorf("expected rootNode.NoteCount == 1, got %d", rootNode.NoteCount)
	}
	if len(rootNode.ChildIDs) != 1 {
		t.Errorf("expected rootNode.ChildIDs len == 1, got %d", len(rootNode.ChildIDs))
	}
}

func TestScanTreeEmptyDirectoryHasRootNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-scanner-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	nodes, _, err := ScanTree(tempDir, "alpha")
	if err != nil {
		t.Fatalf("unexpected ScanTree error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected exactly 1 node (root), got %d", len(nodes))
	}
	if nodes[0].NoteCount != 0 {
		t.Errorf("expected root note count == 0, got %d", nodes[0].NoteCount)
	}
	if len(nodes[0].ChildIDs) != 0 {
		t.Errorf("expected root child IDs len == 0, got %d", len(nodes[0].ChildIDs))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/notes/...`
Expected: FAIL because root node is not currently generated (root has no ID 0, ParentID -1 node).

- [ ] **Step 3: Implement root node generation in `internal/notes/scanner.go`**

Update `ScanTree` in `internal/notes/scanner.go`:
```go
func ScanTree(root string, sortMode string) ([]Node, bool, error) {
	var allNodes []Node
	nodeID := 1 // 0 is reserved for root node

	// Check if this is an Obsidian vault
	obsidianDir := filepath.Join(root, ".obsidian")
	isObsidian := false
	if fi, err := os.Stat(obsidianDir); err == nil && fi.IsDir() {
		isObsidian = true
	}

	var walkDir func(currentPath string, parentID int, level int) ([]int, error)
	// walkDir recursively traverses directories...
```
Call `childIDs, err := walkDir(root, 0, 1)` to scan vault contents as children of root (ParentID: 0, Level: 1).
Create `rootNode`:
```go
	rootNode := Node{
		ID:        0,
		Title:     filepath.Base(root),
		FileName:  filepath.Base(root),
		Path:      root,
		RelPath:   "",
		IsFolder:  true,
		Expanded:  true,
		Level:     0,
		ParentID:  -1,
		ChildIDs:  childIDs,
	}
	allNodes = append([]Node{rootNode}, allNodes...)
	allNodes[0].NoteCount = countNotes(allNodes, 0)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/notes/...`
Expected: PASS

- [ ] **Step 5: Commit and push**

```bash
git add internal/notes/scanner.go internal/notes/scanner_test.go
git commit -m "feat: add root notebook node to scanner tree"
git push
```

---

### Task 2: Empty Workspace Welcome Note Seeding

**Files:**
- Modify: `cmd/tome/main.go:184-205`
- Test: `cmd/tome/main_test.go`

**Interfaces:**
- Consumes: `notes.ScanTree` with root node
- Produces: Correct welcome note seeding when workspace has 0 notes and 0 subfolders.

- [ ] **Step 1: Write test for empty workspace initialization in `cmd/tome/main_test.go`**

```go
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
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test -run TestWelcomeNoteSeedingWithRootNode ./cmd/tome`
Expected: PASS

- [ ] **Step 3: Update `cmd/tome/main.go` empty workspace check**

In `initialModel()` in `cmd/tome/main.go`:
Update:
```go
	allNodes, isObsidian, _ := notes.ScanTree(workspacePath, cfg.SortMode)

	// If workspace is completely empty (no child notes or subfolders), provide an initial welcome note
	if len(allNodes) == 0 || (len(allNodes) == 1 && len(allNodes[0].ChildIDs) == 0) {
		welcomePath := filepath.Join(workspacePath, "Welcome to Tome.md")
		welcomeContent := `# Welcome to Tome 📖

Your new arcane terminal notebook is now connected to **` + workspacePath + `**!

### 🧙 Quick Arcane Runes
- **[N]**: Conjure a new Markdown note in the active folder
- **[F]**: Inscribe a new subfolder
- **[E]**: Edit the current note in-place
- **[Tab]**: Shift focus between the Notebook Tree and Preview
- **[/]**: Quick-filter notes by title
- **[Ctrl+F]**: Search entire notebook content
- **[Ctrl+K]**: Summon Headless Cloud AI Research
- **[F1]**: Markdown Cheat Sheet & formatting grimoire

*Begin your chronicles here or delete this parchment to start fresh.*
`
		_ = os.WriteFile(welcomePath, []byte(welcomeContent), 0644)
		allNodes, isObsidian, _ = notes.ScanTree(workspacePath, cfg.SortMode)
	}
```

- [ ] **Step 4: Run all tests to verify**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit and push**

```bash
git add cmd/tome/main.go cmd/tome/main_test.go
git commit -m "fix: adjust empty workspace detection for root tree node"
git push
```

---

### Task 3: Root Node Display, Safety Guards & Tree Navigation

**Files:**
- Modify: `cmd/tome/main.go` (formatTreeNodeDisplay, renderActivePreview, rebuildVisible)
- Test: `cmd/tome/main_test.go`

**Interfaces:**
- Consumes: Root node with `ParentID == -1`
- Produces:
  - `formatTreeNodeDisplay`: uses notebook icon `󰉋 ` for root node
  - `renderActivePreview`: displays notebook folder card for root node
  - `getDestinationDir()`: returns `m.workspacePath` when root node is selected
  - Tree guide lines: direct children under root show branch lines flush to margin

- [ ] **Step 1: Write unit tests in `cmd/tome/main_test.go`**

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test -run "TestRootNodeFormattingAndIcon|TestRootNodeDestinationDirAndGuards|TestRootNodeCollapseInVisibleIndices" ./cmd/tome`
Expected: FAIL on `TestRootNodeFormattingAndIcon` because icon is currently ` ` instead of `󰉋 `.

- [ ] **Step 3: Update `formatTreeNodeDisplay` and `renderActivePreview` in `cmd/tome/main.go`**

In `cmd/tome/main.go`:
```go
func formatTreeNodeDisplay(item *notes.Node, isFilterActive bool) string {
	hasChildren := len(item.ChildIDs) > 0
	arrow := getTreeNodeArrow(item.IsFolder, hasChildren, item.Expanded)

	icon := "󰈙 "
	if item.IsFolder {
		if item.ParentID == -1 {
			icon = "󰉋 "
		} else if item.Expanded {
			icon = " "
		} else {
			icon = " "
		}
	}
...
```
In `renderActivePreview`:
```go
	if item.IsFolder {
		status := "Collapsed (Press [Enter] to expand)"
		folderIcon := " "
		if item.Expanded {
			status = "Expanded (Press [Enter] to collapse)"
			folderIcon = " "
		}
		if item.ParentID == -1 {
			folderIcon = "󰉋 "
		}
		title := ui.BrandStyle.Render(folderIcon + item.Title)
		displayPath := item.RelPath
		if displayPath == "" {
			displayPath = item.Path
		}
		info := fmt.Sprintf("\n  %s\n\n  • **Path:** `%s`\n  • **Notes Contained:** %d\n  • **State:** %s\n\n  Press **[Enter]**, **[Space]**, or click to toggle folder contents.",
			title, displayPath, item.NoteCount, status)
		return markdown.RenderDocument(info, width)
	}
```

- [ ] **Step 4: Run all tests to verify they pass**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Verify build & commit and push**

Run: `go build -o tome.exe ./cmd/tome`
```bash
git add cmd/tome/main.go cmd/tome/main_test.go
git commit -m "feat: render root notebook row in tree and support root folder previews"
git push
```
