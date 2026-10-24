# 📖 T O M E
### The Arcane Terminal Notebook & Knowledge Engine

A high-velocity, aesthetic terminal notebook for browsing, writing, and organizing Markdown knowledge bases with headless cloud AI research.

---

## 🌟 Key Foundations

- **Strict Viewport Budgeting:** The entire interface is mathematically bounded to your terminal window. Engaging the AI assistant dynamically adjusts pane heights without pushing anything off-screen.
- **Yazi-Style Fluid Mouse Control:**
  - Hover over the left tree to select notes with your scroll wheel.
  - Hover over the right preview pane to scroll through your markdown document independently.
  - Click to select or toggle folder hierarchies.
- **Obsidian-Grade Aesthetics:**
  - 24-bit TrueColor ANSI palette (Flame Orange `#FF7A00`, Amber Gold `#FFAF00`, Dark Slate `#181920`).
  - Native callout cards (`> [!NOTE]`, `> [!TIP]`, `> [!WARNING]`, `> [!CAUTION]`).
  - Checklists (`- [x]`, `- [ ]`), inline code blocks, and wiki-links (`[[Note]]`).
- **Headless Cloud AI Research:**
  - Press `A` to open the AI researcher drawer.
  - Non-blocking background streaming powered by Go Goroutines.
- **Single Zero-Dependency Binary:**
  - Instant startup (< 10ms).
  - Cross-compiles for Windows, macOS (Intel & Apple Silicon), and Linux.

---

## ⌨️ Navigation & Controls

| Shortcut | Action |
|:---|:---|
| `W/S` or `↑/↓` or `J/K` | Navigate tree items |
| `Enter` or `Space` | Toggle folder expand / collapse |
| `Mouse Hover + Scroll` | Context-aware scrolling (Tree vs Preview) |
| `Left Click` | Select item or toggle folder |
| `Tab` | Switch active focus between Tree and Preview |
| `A` | Toggle Cloud AI Research Assistant |
| `Esc` | Close AI Drawer |
| `Q` or `Ctrl+C` | Quit Tome |

---

## 🛠️ Building & Running

```bash
# Build binary
go build -o tome.exe ./cmd/tome

# Run
./tome.exe
```
