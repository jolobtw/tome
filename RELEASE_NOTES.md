# Tome Release Notes

## Version 1.0.4
**Date:** 2026-10-28

### Header Aesthetics & Spacing Polish
- **Tightened Title Typography:** Removed loose letter-spacing and redundant gaps, rendering the title cleanly as `❖ TOME`.
- **Dedicated Glyph Coloring:** The leading arcane star `✦` is now explicitly highlighted in Flame Orange (`#FF7A00`), and the separator diamond `❖` is rendered in Muted Slate (`#6C7086`), ensuring harmonious visual balance with other styled terminal apps.

---

## Version 1.0.3
**Date:** 2026-10-27

### In-App Version Badge & Non-Blocking GitHub Update Alerts
- **Subtle Version Indicator:** Added a subtle version badge (`v1.0.3`) to the top header banner next to the workspace and vault indicators.
- **Asynchronous GitHub Update Detection:** Tome now queries the GitHub Releases API on startup in a non-blocking background goroutine with a 3-second timeout.
- **Dynamic Update Alert Pill:** When a newer release tag is detected on GitHub, the version badge automatically transforms into a vibrant amber glow pill (e.g. `⇡ v1.0.2 (New: v1.0.3)`), informing the user an update is ready without interrupting workflow.
- **CLI Version Flags:** Added support for `tome --version`, `tome -v`, and `tome version` to inspect the version directly from your terminal.

---

## Version 1.0.2
**Date:** 2026-10-27

### Floating Modal Border Preservation & Layered Compositing
- **ANSI-Aware Cell Compositing:** Rewrote modal overlay system with `ansi.Truncate` and `ansi.TruncateLeft`. Floating modals (New Note, New Folder, Rename, Delete, Move) now composite cleanly over the active background without wiping out surrounding columns.
- **Pane Border Integrity:** Preserved all vertical borders, tree hierarchy icons, and preview text surrounding floating dialogs.
- **Exact Border Math:** Corrected 2-column Lipgloss pane width calculation so borders align down to the single character.

---

## Version 1.0.1
**Date:** 2026-10-27

### Custom Vault CLI Arguments & Cross-Platform Releases
- **Folder Arguments:** Supported launching custom vaults via command line (e.g. `tome .` or `tome ~/Documents/Notes`).
- **Automated Multi-Platform Binaries:** Added GitHub Actions release workflow cross-compiling for macOS (Apple Silicon arm64, Intel amd64), Windows (amd64), and Linux (amd64).

---

## Version 1.0.0
**Date:** 2026-10-26

### Initial Go Architecture Release
- **Core Engine:** Complete rewrite of the terminal notebook engine in Go using Bubble Tea, Lipgloss, and Bubbles.
- **Full-Text Search:** Top-drawer keyword search across all notes with highlighted match breadcrumbs.
- **Interactive Modals:** Built-in interactive modals for note creation, deletion, renaming, and organization.
- **In-Place Editor:** Integrated markdown editing directly within the right preview pane.
