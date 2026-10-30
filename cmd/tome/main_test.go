package main

import (
	"strings"
	"testing"
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
