# Copy Item Path to Clipboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to highlight any item (note, folder, or root notebook) in Tome's navigation tree and copy its clean, home-relative path (e.g. `~/Notes/folder/file.md`) directly to the clipboard with `[C]`.

**Architecture:** Implement a robust path formatting utility in `internal/notes` that converts any absolute path to a tilde-relative path with normalized `/` slashes. Connect this utility to key handling in `cmd/tome/main.go` on `"c"` and `"C"`, writing to the system clipboard via `github.com/atotto/clipboard`, flashing a 2-second confirmation badge in the footer, and adding `[C]` to the shortcut legend.

**Tech Stack:** Go 1.22+, `github.com/atotto/clipboard`, Bubble Tea (`github.com/charmbracelet/bubbletea`), Lip Gloss (`github.com/charmbracelet/lipgloss`).

**Spec:** `docs/superpowers/specs/2026-11-08-copy-path-to-clipboard-design.md`

## Global Constraints

- Paths inside the user's home directory must start with `~` and use forward slashes `/` (e.g., `~/Notes/folder/file.md`).
- Paths outside the home directory must retain their root and use normalized forward slashes `/`.
- False prefix matches (e.g., `homeDir = /home/jose` matching `/home/joseph/file.md`) must be prevented by respecting path boundaries.
- Copying the root notebook row (`node.ID == 0`) must format and copy `m.workspacePath` (e.g., `~/Notes`).
- Clipboard errors must not panic or crash; failures display a warning badge (e.g., `⚠ Clipboard unavailable`).
- `[C]` shortcut must only trigger when in navigation tree pane (`m.activePane == 0`) and when no modals, search drawer, filter, AI assistant, or editor are active.
- All code changes must pass `go test ./...`.
- Code changes must be committed and pushed to GitHub per user rules.

## Review Focus

1. **Prefix boundary safety:** When a directory shares a substring prefix with the home directory (e.g., `/home/user2` vs `/home/user`), verify it does not falsely collapse to `~2`. Tested in Task 1.
2. **Windows path and case normalization:** Paths with mixed slashes and case variations (e.g. `C:\users\josep\Notes\doc.md` vs `C:\Users\josep`) must format correctly to `~/Notes/doc.md`. Tested in Task 1.
3. **Root notebook row copying:** Selecting root notebook row (`ID: 0`) copies the workspace directory itself (e.g. `~/Notes`). Tested in Task 2.
4. **Transient notification dismissal:** Status confirmation clears automatically after 2 seconds via a message tick without interrupting user interactions. Tested in Task 2.
5. **No shortcut hijacking:** Pressing `c` while filtering (`/`), searching (`Ctrl+F`), typing in modals, or editing notes (`E`) must pass the character to the active text input instead of copying path. Tested in Task 2.

---

### Task 1: Path Formatting Utility in `internal/notes`

**Files:**
- Create: `internal/notes/path.go`
- Create: `internal/notes/path_test.go`

**Interfaces:**
- Produces: `notes.FormatFriendlyPath(targetPath string, homeDir string) string`

- [x] **Step 1: Write the failing tests in `internal/notes/path_test.go`**

```go
package notes

import (
	"path/filepath"
	"testing"
)

func TestFormatFriendlyPath(t *testing.T) {
	tests := []struct {
		name       string
		targetPath string
		homeDir    string
		expected   string
	}{
		{
			name:       "note inside home directory",
			targetPath: filepath.Join("C:", "Users", "josep", "Notes", "folder", "file.md"),
			homeDir:    filepath.Join("C:", "Users", "josep"),
			expected:   "~/Notes/folder/file.md",
		},
		{
			name:       "root notebook folder inside home directory",
			targetPath: filepath.Join("C:", "Users", "josep", "Notes"),
			homeDir:    filepath.Join("C:", "Users", "josep"),
			expected:   "~/Notes",
		},
		{
			name:       "exact home directory",
			targetPath: filepath.Join("C:", "Users", "josep"),
			homeDir:    filepath.Join("C:", "Users", "josep"),
			expected:   "~",
		},
		{
			name:       "path outside home directory",
			targetPath: filepath.Join("D:", "OtherVault", "notes", "file.md"),
			homeDir:    filepath.Join("C:", "Users", "josep"),
			expected:   "D:/OtherVault/notes/file.md",
		},
		{
			name:       "partial prefix boundary safety",
			targetPath: filepath.Join("C:", "Users", "joseph", "notes.md"),
			homeDir:    filepath.Join("C:", "Users", "jose"),
			expected:   "C:/Users/joseph/notes.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := FormatFriendlyPath(tc.targetPath, tc.homeDir)
			if actual != tc.expected {
				t.Errorf("FormatFriendlyPath(%q, %q) = %q, want %q", tc.targetPath, tc.homeDir, actual, tc.expected)
			}
		})
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/notes -run TestFormatFriendlyPath`
Expected: FAIL with undefined `FormatFriendlyPath`.

- [x] **Step 3: Implement `FormatFriendlyPath` in `internal/notes/path.go`**

```go
package notes

import (
	"path/filepath"
	"strings"
)

// FormatFriendlyPath returns a normalized path string using forward slashes,
// replacing homeDir with "~" if targetPath is inside homeDir.
func FormatFriendlyPath(targetPath string, homeDir string) string {
	cleanTarget := filepath.Clean(targetPath)
	cleanHome := filepath.Clean(homeDir)

	if cleanHome != "" && cleanHome != "." {
		// Case-insensitive comparison on Windows, standard on Unix
		lowerTarget := strings.ToLower(cleanTarget)
		lowerHome := strings.ToLower(cleanHome)

		if lowerTarget == lowerHome {
			return "~"
		}

		homeSep := lowerHome + string(filepath.Separator)
		if strings.HasPrefix(lowerTarget, homeSep) {
			rel := cleanTarget[len(homeSep):]
			return "~/" + filepath.ToSlash(rel)
		}
	}

	return filepath.ToSlash(cleanTarget)
}
```

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/notes -run TestFormatFriendlyPath`
Expected: PASS.

- [x] **Step 5: Commit and push**

```bash
git add internal/notes/path.go internal/notes/path_test.go
git commit -m "feat: add FormatFriendlyPath utility with tests"
git push
```

---

### Task 2: Tree Key Handler, Clipboard Copy & Status Confirmation

**Files:**
- Modify: `cmd/tome/main.go`
- Modify: `cmd/tome/main_test.go`

**Interfaces:**
- Consumes: `notes.FormatFriendlyPath`, `github.com/atotto/clipboard`
- Produces: `clearStatusMsg` Bubble Tea message, `"c"` / `"C"` key handler in tree pane, updated footer items.

- [x] **Step 1: Write tests in `cmd/tome/main_test.go` for `clearStatusMsg` and `"c"` key press**

Verify that:
- Sending `clearStatusMsg` resets `m.statusMsg` to `""`.
- In tree navigation, pressing `"c"` sets `m.statusMsg` containing `"✓ Copied:"`.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/tome -run TestCopyPathKey`
Expected: FAIL due to unhandled key / undefined `clearStatusMsg`.

- [x] **Step 3: Implement clipboard copy and status update in `cmd/tome/main.go`**

1. Define `type clearStatusMsg struct{}` and tick helper:
```go
func clearStatusTick(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}
```
2. In `Update`:
   - Handle `clearStatusMsg`:
     ```go
     case clearStatusMsg:
         m.statusMsg = ""
         return m, nil
     ```
   - In `tea.KeyMsg`:
     Inside the main navigation keys `switch msg.String()` (when `m.activePane == 0` or default):
     ```go
     case "c", "C":
         if m.activePane == 0 && m.selectedIdx >= 0 && m.selectedIdx < len(m.visibleIndices) {
             actualID := m.visibleIndices[m.selectedIdx]
             var targetPath string
             for idx := range m.allNodes {
                 if m.allNodes[idx].ID == actualID {
                     targetPath = m.allNodes[idx].Path
                     break
                 }
             }
             if targetPath == "" && actualID == 0 {
                 targetPath = m.workspacePath
             }
             if targetPath != "" {
                 homeDir, _ := os.UserHomeDir()
                 formatted := notes.FormatFriendlyPath(targetPath, homeDir)
                 if err := clipboard.WriteAll(formatted); err != nil {
                     m.statusMsg = "⚠ Clipboard unavailable"
                 } else {
                     m.statusMsg = fmt.Sprintf("✓ Copied: %s", formatted)
                 }
                 return m, clearStatusTick(2 * time.Second)
             }
         }
     ```
3. In `getFooterItems()`:
   - Prepend `ui.ActiveBadgeStyle.Render(m.statusMsg) + "  "` if `m.statusMsg != ""`.
   - Add `ui.FooterKeyStyle.Render("[C]") + ui.FooterDescStyle.Render(" Copy Path  ")` to tree pane footer items.

- [x] **Step 4: Run all tests to verify they pass**

Run: `go test ./...`
Expected: PASS.

- [x] **Step 5: Verify building application binary**

Run: `go build -o tome.exe ./cmd/tome`
Expected: Clean build with exit code 0.

- [x] **Step 6: Commit and push**

```bash
git add cmd/tome/main.go cmd/tome/main_test.go
git commit -m "feat: add [c] shortcut to copy item path to clipboard"
git push
```
