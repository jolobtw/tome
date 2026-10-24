package notes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Node struct {
	ID        int
	Title     string
	FileName  string
	Path      string
	RelPath   string
	IsFolder  bool
	Expanded  bool
	Level     int
	ParentID  int
	ChildIDs  []int
	ModTime   time.Time
	NoteCount int // For folders: total notes inside
	IsLast    bool
	Prefix    string // Connected tree guide lines (e.g. │  ├─ )
}

// ScanTree scans the notebook directory and builds an indexed tree of folders and markdown notes
func ScanTree(root string, sortMode string) ([]Node, bool, error) {
	var allNodes []Node
	nodeID := 0

	// Check if this is an Obsidian vault
	obsidianDir := filepath.Join(root, ".obsidian")
	isObsidian := false
	if fi, err := os.Stat(obsidianDir); err == nil && fi.IsDir() {
		isObsidian = true
	}

	var walkDir func(currentPath string, parentID int, level int) ([]int, error)
	walkDir = func(currentPath string, parentID int, level int) ([]int, error) {
		entries, err := os.ReadDir(currentPath)
		if err != nil {
			return nil, err
		}

		type dirEntryItem struct {
			entry    os.DirEntry
			isFolder bool
			modTime  time.Time
		}

		var dirs []dirEntryItem
		var files []dirEntryItem

		for _, e := range entries {
			name := e.Name()
			// Skip hidden files/directories (e.g. .git, .obsidian, .config.json)
			if strings.HasPrefix(name, ".") {
				continue
			}

			info, err := e.Info()
			if err != nil {
				continue
			}

			if e.IsDir() {
				dirs = append(dirs, dirEntryItem{
					entry:    e,
					isFolder: true,
					modTime:  info.ModTime(),
				})
			} else if strings.HasSuffix(strings.ToLower(name), ".md") {
				files = append(files, dirEntryItem{
					entry:    e,
					isFolder: false,
					modTime:  info.ModTime(),
				})
			}
		}

		// Sort entries
		if sortMode == "date" {
			sort.Slice(dirs, func(i, j int) bool {
				return dirs[i].modTime.After(dirs[j].modTime)
			})
			sort.Slice(files, func(i, j int) bool {
				return files[i].modTime.After(files[j].modTime)
			})
		} else {
			sort.Slice(dirs, func(i, j int) bool {
				return strings.ToLower(dirs[i].entry.Name()) < strings.ToLower(dirs[j].entry.Name())
			})
			sort.Slice(files, func(i, j int) bool {
				return strings.ToLower(files[i].entry.Name()) < strings.ToLower(files[j].entry.Name())
			})
		}

		var myChildIDs []int

		// 1. Process subfolders first
		for _, d := range dirs {
			thisID := nodeID
			nodeID++

			folderPath := filepath.Join(currentPath, d.entry.Name())
			rel, _ := filepath.Rel(root, folderPath)

			folderNode := Node{
				ID:       thisID,
				Title:    d.entry.Name(),
				FileName: d.entry.Name(),
				Path:     folderPath,
				RelPath:  rel,
				IsFolder: true,
				Expanded: false, // Default to collapsed
				Level:    level,
				ParentID: parentID,
				ModTime:  d.modTime,
			}

			allNodes = append(allNodes, folderNode)
			myChildIDs = append(myChildIDs, thisID)

			// Recursively walk subfolder
			subChildIDs, _ := walkDir(folderPath, thisID, level+1)

			// Update node with children & note count
			for i := range allNodes {
				if allNodes[i].ID == thisID {
					allNodes[i].ChildIDs = subChildIDs
					allNodes[i].NoteCount = countNotes(allNodes, thisID)
					break
				}
			}
		}

		// 2. Process Markdown files
		for _, f := range files {
			thisID := nodeID
			nodeID++

			filePath := filepath.Join(currentPath, f.entry.Name())
			rel, _ := filepath.Rel(root, filePath)

			fileNode := Node{
				ID:       thisID,
				Title:    f.entry.Name(),
				FileName: f.entry.Name(),
				Path:     filePath,
				RelPath:  rel,
				IsFolder: false,
				Expanded: false,
				Level:    level,
				ParentID: parentID,
				ModTime:  f.modTime,
			}

			allNodes = append(allNodes, fileNode)
			myChildIDs = append(myChildIDs, thisID)
		}

		return myChildIDs, nil
	}

	_, err := walkDir(root, -1, 0)
	if err != nil {
		return nil, isObsidian, err
	}

	return allNodes, isObsidian, nil
}

// countNotes recursively counts markdown notes within a folder
func countNotes(nodes []Node, folderID int) int {
	count := 0
	for _, n := range nodes {
		if n.ParentID == folderID {
			if !n.IsFolder {
				count++
			} else {
				count += countNotes(nodes, n.ID)
			}
		}
	}
	return count
}

// ReadNoteContent reads a note file from disk
func ReadNoteContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
