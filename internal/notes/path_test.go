package notes

import (
	"path/filepath"
	"testing"
)

func TestFormatFriendlyPath(t *testing.T) {
	tests := []struct {
		name       string
		targetPath string
		homeDir    string
		expected   string
	}{
		{
			name:       "note inside home directory",
			targetPath: filepath.Join("C:\\", "Users", "josep", "Notes", "folder", "file.md"),
			homeDir:    filepath.Join("C:\\", "Users", "josep"),
			expected:   "~/Notes/folder/file.md",
		},
		{
			name:       "root notebook folder inside home directory",
			targetPath: filepath.Join("C:\\", "Users", "josep", "Notes"),
			homeDir:    filepath.Join("C:\\", "Users", "josep"),
			expected:   "~/Notes",
		},
		{
			name:       "exact home directory",
			targetPath: filepath.Join("C:\\", "Users", "josep"),
			homeDir:    filepath.Join("C:\\", "Users", "josep"),
			expected:   "~",
		},
		{
			name:       "path outside home directory",
			targetPath: filepath.Join("D:\\", "OtherVault", "notes", "file.md"),
			homeDir:    filepath.Join("C:\\", "Users", "josep"),
			expected:   "D:/OtherVault/notes/file.md",
		},
		{
			name:       "partial prefix boundary safety",
			targetPath: filepath.Join("C:\\", "Users", "joseph", "notes.md"),
			homeDir:    filepath.Join("C:\\", "Users", "jose"),
			expected:   "C:/Users/joseph/notes.md",
		},
		{
			name:       "case-insensitive match on windows",
			targetPath: filepath.Join("c:\\", "users", "josep", "Notes", "daily.md"),
			homeDir:    filepath.Join("C:\\", "Users", "josep"),
			expected:   "~/Notes/daily.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := FormatFriendlyPath(tc.targetPath, tc.homeDir)
			if actual != tc.expected {
				t.Errorf("FormatFriendlyPath(%q, %q) = %q, want %q", tc.targetPath, tc.homeDir, actual, tc.expected)
			}
		})
	}
}
