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
  - Press `Ctrl+K` to open the AI researcher drawer.
  - Non-blocking background streaming powered by Go Goroutines.
- **Single Zero-Dependency Binary:**
  - Instant startup (< 10ms).
  - Cross-compiles for Windows, macOS (Intel & Apple Silicon), and Linux.

---

## ⌨️ Navigation & Controls

| Shortcut | Action |
|:---|:---|
| `W/S` or `↑/↓` or `J/K` | Move selection up / down |
| `A` or `Left` | Collapse folder (or jump to parent folder) |
| `D` or `Right` | Expand folder (or step into first child) |
| `Enter` or `Space` | Toggle folder expand / collapse |
| `/` | **Quick Filter:** Real-time tree filter by note title & folder path |
| `Ctrl+F` | **Content Search:** Full-text keyword search across all notes |
| `E` | **In-Place Editor:** Edit active note directly in Tome |
| `N` | **New Note:** Create a markdown note in current folder |
| `F` | **New Folder:** Create a new subfolder |
| `R` | **Rename:** Rename selected note or folder |
| `M` | **Move:** Move item to any directory in the workspace |
| `X` | **Delete:** Safely remove selected note or folder |
| `L` | **Quick Log:** Append timestamped bullet to today's daily log |
| `F1` or `Ctrl+G` | Open Markdown formatting guide & cheat sheet |
| `Tab` | Switch active focus between Tree and Preview |
| `Mouse Hover + Scroll` | Context-aware scrolling (Tree vs Preview) |
| `Left Click` | Select item or toggle folder |
| `Ctrl+K` | Toggle Cloud AI Research Assistant |
| `Esc` | Dismiss modals, search, or close AI Drawer |
| `Q` or `Ctrl+C` | Quit Tome |

---

## 🛠️ Installation & Running

### 🍎 macOS (Ghostty / Terminal)

**Option 1: Quick Install (Pre-compiled Binary — No Go needed)**
```bash
# Apple Silicon (M1/M2/M3/M4)
curl -L -o tome https://github.com/jolobtw/tome/releases/latest/download/tome-darwin-arm64
chmod +x tome
sudo mv tome /usr/local/bin/

# Launch Tome
tome
```

**Option 2: Install with Go**
```bash
go install github.com/jolobtw/tome/cmd/tome@latest
```

### 📂 Choosing Your Notes Folder

Tome can open your default vault or any custom folder you choose:

```bash
# Open your default vault (~/Notes)
tome

# Open the current directory
tome .

# Open any specific notes directory
tome ~/Documents/WorkNotes
```
*(Tome will remember your chosen folder as your active notebook for future sessions).*

---

### 🪟 Windows (Windows Terminal / PowerShell)

```powershell
# Build binary
go build -o tome.exe ./cmd/tome

# Run
.\tome.exe
```
