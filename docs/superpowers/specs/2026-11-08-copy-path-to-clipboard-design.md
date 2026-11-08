# Design: Copy Item Path to Clipboard

## Intent & Why
Users navigating notes and folders in Tome often need to reference file and folder locations in terminal commands, documentation, tickets, or chat messages. Manually typing or finding the path on disk is slow and error-prone. Providing a single hotkey to copy the formatted path directly to the system clipboard makes referencing fast and effortless.

## Goals, Non-Goals & Anti-Goals
- **Goals:**
  - Pressing `c` in navigation tree mode copies the path of the highlighted item (file, folder, or root notebook) to the clipboard.
  - Formats paths starting from the user's home directory with `~` and normalized forward slashes (e.g., `~/Notes/folder/file.md`).
  - Displays a transient confirmation message (e.g., `✓ Copied path to clipboard`) in the status area for 2 seconds.
  - Updates the bottom footer shortcuts list to include `[C] Copy Path`.
- **Non-Goals:**
  - Multiple clipboard formats menu or modal picker.
  - Operating when modals, search drawers, or editor panes are focused.
- **Anti-Goals:**
  - Overwriting the clipboard silently without user feedback.
  - Crashing or freezing if the clipboard service is unavailable; errors must be handled gracefully.

## Constraints
- Cross-platform support using Go's `github.com/atotto/clipboard`.
- Seamless Windows path conversion (replacing `C:\Users\<user>` with `~` and backslashes `\` with `/`).
- Must adhere to the root notebook node representation (`ID: 0`, matching `workspacePath`).

## Approaches Considered
1. **Approach 1: Dedicated `[C]` Hotkey with Friendly Tilde Path (Selected)**
   - Pressing `c` on any selected item computes the home-relative path (e.g., `~/Notes/folder/file.md`) and writes it to the clipboard, displaying a temporary confirmation message.
   - *Pros:* Simple, fast, one intuitive key, clean universal format.
   - *Selection Reason:* Chosen by user for simplicity and exact match with user reference needs.
2. **Approach 2: Dual Hotkeys (`c` for Home Path, `y` for Vault-Only Path)**
   - Pressing `c` copies home-relative, while `y` copies workspace-relative (`folder/file.md`).
   - *Trade-off:* Extra keybinding complexity with little added benefit.

## Technical Details
- **Key Binding:** In tree mode (when `m.activePane == 0`, no modal open, not in search/AI/filter/editor), handle `"c"`, `"C"`.
- **Selected Node Path Resolution:**
  - Retrieve the currently selected `notes.Node` using `m.visibleIndices[m.selectedIdx]`.
  - Obtain its path: `node.Path` (for files/folders) or `m.workspacePath` (for root).
- **Path Formatting:**
  - Detect `os.UserHomeDir()`. If the absolute path starts with home dir (case-insensitive on Windows), replace it with `~`.
  - Normalize all path separators to `/`.
- **Clipboard Action:**
  - Call `clipboard.WriteAll(formattedPath)`.
  - Set a status message in `Model`: `m.statusMsg = fmt.Sprintf("✓ Copied: %s", formattedPath)`.
  - Dispatch a timer command `clearStatusMsgCmd(2 * time.Second)` that sends a message resetting `m.statusMsg = ""`.
- **UI Display:**
  - Footer bar displays `m.statusMsg` when non-empty, or displays `[C] Copy Path` alongside existing shortcuts.
