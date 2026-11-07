package notes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanTreeIncludesRootNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-scanner-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subFolder := filepath.Join(tempDir, "SubFolder")
	if err := os.Mkdir(subFolder, 0755); err != nil {
		t.Fatalf("failed to create subfolder: %v", err)
	}
	notePath := filepath.Join(subFolder, "TestNote.md")
	if err := os.WriteFile(notePath, []byte("# Test Note\n"), 0644); err != nil {
		t.Fatalf("failed to create test note: %v", err)
	}

	nodes, isObs, err := ScanTree(tempDir, "alpha")
	if err != nil {
		t.Fatalf("unexpected ScanTree error: %v", err)
	}
	if isObs {
		t.Errorf("expected isObs to be false")
	}

	if len(nodes) < 1 {
		t.Fatalf("expected at least 1 node (root), got %d", len(nodes))
	}

	rootNode := nodes[0]
	if rootNode.ID != 0 {
		t.Errorf("expected rootNode.ID == 0, got %d", rootNode.ID)
	}
	if rootNode.ParentID != -1 {
		t.Errorf("expected rootNode.ParentID == -1, got %d", rootNode.ParentID)
	}
	if rootNode.Level != 0 {
		t.Errorf("expected rootNode.Level == 0, got %d", rootNode.Level)
	}
	if !rootNode.IsFolder {
		t.Errorf("expected rootNode.IsFolder == true")
	}
	if !rootNode.Expanded {
		t.Errorf("expected rootNode.Expanded == true")
	}
	if rootNode.Path != tempDir {
		t.Errorf("expected rootNode.Path == %q, got %q", tempDir, rootNode.Path)
	}
	if rootNode.NoteCount != 1 {
		t.Errorf("expected rootNode.NoteCount == 1, got %d", rootNode.NoteCount)
	}
	if len(rootNode.ChildIDs) != 1 {
		t.Errorf("expected rootNode.ChildIDs len == 1, got %d", len(rootNode.ChildIDs))
	}
}

func TestScanTreeEmptyDirectoryHasRootNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tome-scanner-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	nodes, _, err := ScanTree(tempDir, "alpha")
	if err != nil {
		t.Fatalf("unexpected ScanTree error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected exactly 1 node (root), got %d", len(nodes))
	}
	if nodes[0].NoteCount != 0 {
		t.Errorf("expected root note count == 0, got %d", nodes[0].NoteCount)
	}
	if len(nodes[0].ChildIDs) != 0 {
		t.Errorf("expected root child IDs len == 0, got %d", len(nodes[0].ChildIDs))
	}
}
