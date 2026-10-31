# Design Doc: Navigation Tree Expand/Collapse Arrows

## Intent & The Why
Currently, folders in Tome's navigation sidebar look visually identical whether they contain notes/subfolders or are completely empty. Users cannot tell if pressing `Enter` / `Right Arrow` or clicking a folder will reveal any contents. Adding clear expander arrows for folders with contents—and omitting them for empty folders—provides immediate visual clarity.

## Goals
- Display an arrow indicator (`▸` for collapsed, `▾` for expanded) before folders that have contents (at least 1 child note or subfolder).
- Display no arrow (substituting spaces for alignment) for folders with no contents.
- Keep all folder icons, note icons, and titles strictly aligned across rows.

## Non-Goals
- Adding item count badges or text tags (the user explicitly opted for the clean arrow approach).
- Changing folder creation, deletion, or keyboard navigation mechanics.

## Approaches Considered
1. **Leading Arrow with Space Alignment (Chosen)**:
   - Expandable folders show `▸ ` (collapsed) or `▾ ` (expanded) followed by the folder icon.
   - Empty folders and notes show two spaces `  ` instead of an arrow, preserving identical indentation and alignment across the sidebar.
   - Trade-off: Uses 2 character spaces horizontally, but provides standard, consistent alignment familiar from modern file explorers.
2. **Arrow Replacing the Folder Icon**:
   - Arrows replace the folder icon (`▸` / `▾`). Empty folders show a static folder icon.
   - Trade-off: Inconsistent iconography and harder to distinguish folders from notes at a glance.

## Implementation Details
1. **Folder Contents Check**:
   - Check if a node has children (`len(item.ChildIDs) > 0`).
2. **Icon & Prefix Construction in `cmd/tome/main.go`**:
   - For folders with `len(item.ChildIDs) > 0`:
     - If `item.Expanded`: arrow prefix is `▾ `
     - If collapsed: arrow prefix is `▸ `
   - For folders with `len(item.ChildIDs) == 0`:
     - Arrow prefix is `  ` (two spaces)
   - For notes (`!item.IsFolder`):
     - Arrow prefix is `  ` (two spaces)
   - Followed by the existing item icon (` `, ` `, `󰈙 `) and title.
3. **Width & Truncation**:
   - The arrow prefix is factored into width calculation and line rendering so titles truncate cleanly with dots when pane width is restricted.
4. **Automated Tests**:
   - Add unit test verifying that expandable folders receive arrow prefixes while empty folders and files receive space padding.
