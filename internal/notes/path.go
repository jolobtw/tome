package notes

import (
	"path/filepath"
	"strings"
)

// FormatFriendlyPath returns a normalized path string using forward slashes,
// replacing homeDir with "~" if targetPath is inside homeDir.
func FormatFriendlyPath(targetPath string, homeDir string) string {
	cleanTarget := filepath.Clean(targetPath)
	cleanHome := filepath.Clean(homeDir)

	if cleanHome != "" && cleanHome != "." {
		lowerTarget := strings.ToLower(cleanTarget)
		lowerHome := strings.ToLower(cleanHome)

		if lowerTarget == lowerHome {
			return "~"
		}

		homeSep := lowerHome + string(filepath.Separator)
		if strings.HasPrefix(lowerTarget, homeSep) {
			rel := cleanTarget[len(homeSep):]
			return "~/" + filepath.ToSlash(rel)
		}
	}

	return filepath.ToSlash(cleanTarget)
}
