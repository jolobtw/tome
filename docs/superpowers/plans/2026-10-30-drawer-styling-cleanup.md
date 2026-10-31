# Search & Research Drawers Styling and Empty State Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix background highlighting/bleed artifacts across the Search and Research drawers, eliminate redundant placeholder/helper copy, and refine the empty states to be subtle, clean, and polished.

**Architecture:** Introduce `ui.FillLineBg(line string, width int, bg lipgloss.Color) string` in `internal/ui/theme.go` that pads a line to the exact width and re-asserts the background color after any inner ANSI reset (`\x1b[0m`), preventing the terminal's default background from showing through as uncolored spaces. Use this helper across the Search and Research drawer render loops in `cmd/tome/main.go`, and update input placeholders and empty state text to be clean, modern, and non-redundant.

**Tech Stack:** Go 1.22+, Bubble Tea (`github.com/charmbracelet/bubbletea`), Lip Gloss (`github.com/charmbracelet/lipgloss`).

## Global Constraints
- Auto-push rule: whenever changes are committed, push them to GitHub.
- Background color for drawer interiors must be solid `ui.ColorDarkSlate` (`#13141C`) without gaps, banding, or color leaks.
- All tests must pass cleanly.

## Review Focus
1. `FillLineBg` must handle ANSI reset sequences (`\x1b[0m`) by re-asserting the background color on all subsequent spaces.
2. `FillLineBg` on lines with trailing uncolored spaces (such as from `textinput.View()`) must trim and re-pad with the solid background color.
3. Search drawer placeholder must be clean (`"Search note contents..."`) and empty state must be concise without redundant examples.
4. Research drawer placeholder must be clean (`"Ask a question about your notes..."`) and empty state must be subtle without redundant text.
5. All drawer lines must clamp and pad to the drawer width with solid background coverage.

---

### Task 1: Add `FillLineBg` helper in `internal/ui/theme.go` and unit tests

**Files:**
- Modify: `internal/ui/theme.go`
- Test: `cmd/tome/main_test.go`

**Interfaces:**
- Produces: `ui.FillLineBg(line string, width int, bg lipgloss.Color) string`

- [ ] **Step 1: Write the failing unit tests for `FillLineBg` in `cmd/tome/main_test.go`**

```go
func TestFillLineBg(t *testing.T) {
	bg := ui.ColorDarkSlate // #13141C -> \x1b[48;2;19;20;28m
	raw := "\x1b[1;38;2;255;121;0m🔍 \x1b[0mS\x1b[0mearch...                  "
	filled := ui.FillLineBg(raw, 40, bg)

	if !strings.Contains(filled, "\x1b[48;2;19;20;28m") {
		t.Errorf("expected filled line to contain background ANSI sequence, got: %q", filled)
	}

	// Verify that the visual width is at least the requested width
	if lipgloss.Width(filled) != 40 {
		t.Errorf("expected visual width 40, got %d", lipgloss.Width(filled))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestFillLineBg ./cmd/tome`
Expected: FAIL with "undefined: ui.FillLineBg"

- [ ] **Step 3: Implement `FillLineBg` in `internal/ui/theme.go`**

```go
// FillLineBg pads an ANSI line to the target width and ensures that every character
// and whitespace padding retains the given background color, even across ANSI reset codes.
func FillLineBg(line string, width int, bg lipgloss.Color) string {
	trimmed := strings.TrimRight(line, " ")
	w := lipgloss.Width(trimmed)
	padLen := max(0, width-w)
	padded := trimmed + strings.Repeat(" ", padLen)

	bgSeq := lipgloss.NewStyle().Background(bg).Render("X")
	bgPrefix := strings.TrimSuffix(bgSeq, "X\x1b[0m")
	if !strings.HasPrefix(bgPrefix, "\x1b[") {
		return padded
	}

	reasserted := strings.ReplaceAll(padded, "\x1b[0m", "\x1b[0m"+bgPrefix)
	reasserted = strings.ReplaceAll(reasserted, "\x1b[m", "\x1b[m"+bgPrefix)
	return bgPrefix + reasserted + "\x1b[0m"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run TestFillLineBg ./cmd/tome`
Expected: PASS

- [ ] **Step 5: Commit and push**

```bash
git add internal/ui/theme.go cmd/tome/main_test.go
git commit -m "feat: add FillLineBg helper to eliminate background color bleed"
git push
```

---

### Task 2: Refine Search and Research drawers with `FillLineBg`, clean placeholders, and subtle empty states

**Files:**
- Modify: `cmd/tome/main.go`
- Test: `cmd/tome/main_test.go`

- [ ] **Step 1: Update input placeholders and styles in `cmd/tome/main.go`**
  - Update `si.Placeholder` to `"Search note contents..."`.
  - Update `ti.Placeholder` to `"Ask a question about your notes..."`.
  - Ensure `si.TextStyle` and `si.PlaceholderStyle` have `.Background(ui.ColorDarkSlate)`.
  - Ensure `ti.TextStyle` and `ti.PlaceholderStyle` have `.Background(ui.ColorDarkSlate)`.

- [ ] **Step 2: Update Search drawer rendering in `cmd/tome/main.go`**
  - Refine empty query message to a quiet, subtle line:
    `resultsLines = append(resultsLines, ui.SubtitleStyle.Render("\n  Type to search note contents across your vault."))`
  - In drawer clamped lines processing:
    Use `ui.FillLineBg(l, innerContentWidth, ui.ColorDarkSlate)`.
  - For empty drawer fill lines:
    Use `ui.FillLineBg("", innerContentWidth, ui.ColorDarkSlate)`.
  - In `drawerLines`:
    Use `ui.FillLineBg("", usableWidth, ui.ColorDarkSlate)`.

- [ ] **Step 3: Update Research / AI drawer rendering in `cmd/tome/main.go`**
  - Refine idle answer area to a subtle, quiet line:
    `answerArea = ui.SubtitleStyle.Render("\n  Ask any question to synthesize insights across your notes.")`
  - In drawer clamped lines processing:
    Use `ui.FillLineBg(l, innerContentWidth, ui.ColorDarkSlate)`.
  - For empty drawer fill lines:
    Use `ui.FillLineBg("", innerContentWidth, ui.ColorDarkSlate)`.

- [ ] **Step 4: Run full test suite and build binary**

Run: `go test -count=1 ./...`
Expected: PASS
Run: `go build -o tome.exe ./cmd/tome`
Expected: PASS

- [ ] **Step 5: Commit and push**

```bash
git add cmd/tome/main.go cmd/tome/main_test.go
git commit -m "style: polish search and research drawers with solid backgrounds and subtle empty states"
git push
```
