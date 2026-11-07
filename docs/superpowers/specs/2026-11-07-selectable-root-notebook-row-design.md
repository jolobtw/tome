# Design Doc: Selectable Root Notebook Row in Navigation Tree

## Intent & The Why
Currently in Tome, pressing `N` (New Note) or `F` (New Folder) relies on `getDestinationDir()` to decide where to create an item. If a note is selected, the destination is that note's parent folder; if a folder is selected, the destination is inside that folder. 
When a notebook contains subfolders but no loose markdown notes directly in the root directory, every selectable item in the tree is either a folder or an item inside a folder. Consequently, users have no way to create a top-level note or top-level folder. Adding the root notebook as the top selectable row in the sidebar tree establishes a natural, visual anchor for the vault root, enabling users to highlight the notebook and create top-level notes and folders effortlessly.

## Goals
- Add the root notebook folder as the topmost row in the navigation tree.
- When the root row is selected, pressing `N` creates a note directly in the root directory (`workspacePath`), and pressing `F` creates a folder directly in the root directory.
- Allow expanding/collapsing the entire notebook contents under the root row (defaulting to expanded).
- Ensure safe guards prevent modifying root (rename, delete, move remain disallowed for root).
- Ensure continuous tree guide lines cleanly connect the root folder to all top-level items below it.

## Non-Goals
- Changing shortcut bindings (`N`, `F`, `R`, `X`, `M` retain existing key bindings).
- Changing how notes and subfolders are scanned from disk.

## Approaches Considered
1. **Interactive Modal Destination Toggle (`Tab` in create popup)**:
   - When `N` or `F` is pressed, modal shows destination path with a `Tab` toggle between selected folder and notebook root.
   - Trade-off: Keeps tree unchanged, but requires an extra keypress in the modal and leaves the root invisible in the sidebar.
2. **Dedicated Shift Shortcuts (`Shift+N` / `Shift+F`)**:
   - Separate shortcuts for root creation vs current folder creation.
   - Trade-off: Quick for power users, but requires learning additional key chords and doesn't visually represent root in the tree.
3. **Selectable Root Row in Navigation Tree (Chosen by User)**:
   - The root notebook itself is rendered at the top of the tree (e.g. `▾ 󰉋 NotebookName`).
   - Selecting this row targets the notebook root directory.
   - Trade-off: Familiar desktop file-manager visual hierarchy; provides an intuitive, discoverable target for root-level actions.

## Implementation Details
1. **Tree Representation (`ScanTree` / `rebuildVisible`)**:
   - The tree includes a root node (`Path: workspacePath`, `Title: workspaceName`, `IsFolder: true`, `Expanded: true`, `Level: 0`).
   - Children of the root are placed at `Level: 1` under the root node.
   - Total notes count on the root node matches all notes contained within the workspace.
2. **Action Dispatching**:
   - When the root row is highlighted, `getSelectedNode()` returns the root node.
   - `getDestinationDir()` returns `node.Path` (which is `workspacePath`).
   - `openRenameModal()`, `openDeleteModal()`, and `openMoveModal()` already guard against `item.Path == m.workspacePath` and remain blocked.
3. **Folder Preview Card**:
   - Selecting the root row displays the notebook overview card in the preview pane (title, path, total notes contained, expanded/collapsed status).
4. **Visual Layout & Tree Lines**:
   - The root row displays with `▾ ` (expanded) or `▸ ` (collapsed) and the notebook icon (`󰉋 `).
   - Branch lines (`├─ `, `└─ `, `│  `) cleanly descend from the root row to top-level items.
5. **Automated Tests**:
   - Unit tests verifying:
     - Root node exists at top of scanned/built tree with correct metadata.
     - `getDestinationDir()` returns workspace root when root node is selected.
     - New note / new folder creation correctly targets root path.
