package main

import (
	"strings"
	"testing"

	"github.com/jolobtw/tome/internal/ui"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v1.0.0", "v0.9.9", false},
		{"0.1.0", "v0.1.1", true},
		{"v1.0.0", "v1.0.1", true},
		{"v1.2.0", "v1.1.9", false},
	}

	for _, tt := range tests {
		result := isNewerVersion(tt.current, tt.latest)
		if result != tt.expected {
			t.Errorf("isNewerVersion(%q, %q) = %v; want %v", tt.current, tt.latest, result, tt.expected)
		}
	}
}

func TestParseVersion(t *testing.T) {
	parsed := parseVersion("v1.2.3")
	if len(parsed) != 3 || parsed[0] != 1 || parsed[1] != 2 || parsed[2] != 3 {
		t.Fatalf("unexpected parseVersion output: %v", parsed)
	}
}

func TestWrapFooterItems(t *testing.T) {
	items := []string{
		"[Ctrl+S] Save  ",
		"[Esc] Finish  ",
		"[Ctrl+Q] Discard  ",
		"[F1] Guide  ",
	}

	// Wide width should keep it in 1 line
	wide := wrapFooterItems(items, 200)
	if strings.Contains(wide, "\n") {
		t.Errorf("expected 1 line for wide width, got: %q", wide)
	}

	// Narrow width should wrap into multiple lines
	narrow := wrapFooterItems(items, 25)
	if !strings.Contains(narrow, "\n") {
		t.Errorf("expected multiple lines for narrow width, got: %q", narrow)
	}
}

func TestFooterHeightCalculation(t *testing.T) {
	m := Model{
		activePane: 0,
		ready:      true,
	}

	hWide := m.getFooterHeight(200)
	hNarrow := m.getFooterHeight(40)

	if hWide < 1 {
		t.Errorf("expected hWide >= 1, got %d", hWide)
	}
	if hNarrow <= hWide {
		t.Errorf("expected hNarrow (%d) > hWide (%d)", hNarrow, hWide)
	}
}

func TestTreeGuideStyle(t *testing.T) {
	renderedGuide := ui.TreeGuide.Render("│  ├─ ")
	if renderedGuide == "" {
		t.Errorf("expected rendered guide lines, got empty string")
	}
}

func TestDimANSILine(t *testing.T) {
	// Plain text should be wrapped in dimmed foreground
	plain := "Hello world note preview"
	dimmedPlain := ui.DimANSILine(plain, 0.35)
	if !strings.Contains(dimmedPlain, "\x1b[38;2;") || !strings.Contains(dimmedPlain, "Hello world") {
		t.Errorf("expected plain text to be wrapped in dimmed foreground, got: %q", dimmedPlain)
	}

	// Bold compound text should convert 1 to 22 and dim RGB colors
	compound := "\x1b[1;38;2;255;255;255;48;2;38;40;56m1. Terra Campaign\x1b[0m"
	dimmedCompound := ui.DimANSILine(compound, 0.35)
	if strings.Contains(dimmedCompound, "38;2;255;255;255") {
		t.Errorf("expected bright white to be dimmed, got: %q", dimmedCompound)
	}
	if strings.Contains(dimmedCompound, "[1;") {
		t.Errorf("expected bold code 1 to be removed or converted to 22, got: %q", dimmedCompound)
	}
}

func TestGetTreeNodeArrow(t *testing.T) {
	tests := []struct {
		name        string
		isFolder    bool
		hasChildren bool
		isExpanded  bool
		expected    string
	}{
		{"Folder with children collapsed", true, true, false, "▸ "},
		{"Folder with children expanded", true, true, true, "▾ "},
		{"Empty folder collapsed", true, false, false, "  "},
		{"Empty folder expanded", true, false, true, "  "},
		{"Note file", false, false, false, "  "},
		{"Note file with children flag false", false, true, false, "  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTreeNodeArrow(tt.isFolder, tt.hasChildren, tt.isExpanded)
			if got != tt.expected {
				t.Errorf("getTreeNodeArrow(%v, %v, %v) = %q; want %q",
					tt.isFolder, tt.hasChildren, tt.isExpanded, got, tt.expected)
			}
		})
	}
}
