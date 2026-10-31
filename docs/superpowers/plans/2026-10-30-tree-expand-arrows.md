# Navigation Tree Expand/Collapse Arrows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Display `▸` / `▾` expansion arrows for folders containing items in the sidebar navigation tree while showing no arrow (and preserving space alignment) for empty folders and notes.

**Architecture:** Introduce a pure helper function `getTreeNodeArrow(isFolder bool, hasChildren bool, isExpanded bool) string` that returns `"▾ "` for expanded folders with children, `"▸ "` for collapsed folders with children, and `"  "` (two spaces) for empty folders and notes. Integrate this into the tree rendering loop in `cmd/tome/main.go` where `nodeDisplay` is constructed, ensuring strict vertical alignment of all icons and titles across rows.

**Tech Stack:** Go 1.22+, Bubble Tea (`github.com/charmbracelet/bubbletea`), Lip Gloss (`github.com/charmbracelet/lipgloss`).

**Spec:** `docs/superpowers/specs/2026-10-30-tree-expand-arrows-design.md`

## Global Constraints
- Arrow prefixes for expandable folders must be `"▸ "` when collapsed and `"▾ "` when expanded.
- Non-expandable items (empty folders and notes) must be prefixed with two spaces `"  "` to maintain uniform column alignment.
- Auto-push rule: whenever changes are committed, push them to GitHub.

## Review Focus
1. Collapsed folder with children: must return `"▸ "` to signal expandable contents.
2. Expanded folder with children: must return `"▾ "` to signal open contents.
3. Empty folder (collapsed): must return `"  "` (two spaces, no arrow).
4. Empty folder (expanded flag set): must return `"  "` (no arrow regardless of expanded state).
5. Note (file node): must return `"  "` (two spaces, no arrow).

---

### Task 1: Add tree node arrow helper and unit tests (TDD)

**Files:**
- Modify: `cmd/tome/main.go`
- Test: `cmd/tome/main_test.go`

**Interfaces:**
- Produces: `getTreeNodeArrow(isFolder bool, hasChildren bool, isExpanded bool) string`

- [ ] **Step 1: Write the failing unit tests for `getTreeNodeArrow` in `cmd/tome/main_test.go`**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestGetTreeNodeArrow ./cmd/tome`
Expected: FAIL with "undefined: getTreeNodeArrow"

- [ ] **Step 3: Implement `getTreeNodeArrow` in `cmd/tome/main.go`**

```go
// getTreeNodeArrow returns an expand/collapse arrow for folders with contents,
// or two spaces for empty folders and notes to preserve vertical alignment.
func getTreeNodeArrow(isFolder bool, hasChildren bool, isExpanded bool) string {
	if !isFolder || !hasChildren {
		return "  "
	}
	if isExpanded {
		return "▾ "
	}
	return "▸ "
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestGetTreeNodeArrow ./cmd/tome`
Expected: PASS

- [ ] **Step 5: Commit and push**

```bash
git add cmd/tome/main.go cmd/tome/main_test.go
git commit -m "feat: add getTreeNodeArrow helper for navigation tree"
git push
```

---

### Task 2: Integrate arrow prefix into tree pane rendering

**Files:**
- Modify: `cmd/tome/main.go:1555-1577`
- Test: `cmd/tome/main_test.go`

**Interfaces:**
- Consumes: `getTreeNodeArrow(isFolder bool, hasChildren bool, isExpanded bool) string`

- [ ] **Step 1: Write integration/regression test for node display formatting in `cmd/tome/main_test.go`**

```go
func TestTreeNodeDisplayFormatting(t *testing.T) {
	folderWithChildren := notes.Node{
		Title:    "Lore",
		IsFolder: true,
		ChildIDs: []int{1, 2},
		Expanded: false,
	}
	arrow := getTreeNodeArrow(folderWithChildren.IsFolder, len(folderWithChildren.ChildIDs) > 0, folderWithChildren.Expanded)
	if arrow != "▸ " {
		t.Errorf("expected '▸ ', got %q", arrow)
	}

	emptyFolder := notes.Node{
		Title:    "Empty",
		IsFolder: true,
		ChildIDs: nil,
		Expanded: false,
	}
	arrowEmpty := getTreeNodeArrow(emptyFolder.IsFolder, len(emptyFolder.ChildIDs) > 0, emptyFolder.Expanded)
	if arrowEmpty != "  " {
		t.Errorf("expected '  ', got %q", arrowEmpty)
	}
}
```

- [ ] **Step 2: Update tree rendering in `cmd/tome/main.go`**

Update `cmd/tome/main.go` in the tree rendering loop:
```go
		hasChildren := len(item.ChildIDs) > 0
		arrow := getTreeNodeArrow(item.IsFolder, hasChildren, item.Expanded)

		icon := "󰈙 "
		if item.IsFolder {
			if item.Expanded {
				icon = " "
			} else {
				icon = " "
			}
		}

		var nodeDisplay string
		var prefix string
		if m.filterActive {
			dir := filepath.Dir(item.RelPath)
			if dir != "." && dir != "" {
				nodeDisplay = fmt.Sprintf("%s%s%s (%s)", arrow, icon, item.Title, dir)
			} else {
				nodeDisplay = arrow + icon + item.Title
			}
		} else {
			prefix = item.Prefix
			nodeDisplay = arrow + icon + item.Title
		}
```

- [ ] **Step 3: Run full test suite and verify build**

Run: `go test ./...`
Expected: PASS
Run: `go build -o tome.exe ./cmd/tome`
Expected: Build succeeds with exit code 0.

- [ ] **Step 4: Commit and push**

```bash
git add cmd/tome/main.go cmd/tome/main_test.go
git commit -m "feat: render expand/collapse arrows in navigation tree"
git push
```
