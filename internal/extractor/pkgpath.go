package extractor

// pkgpath.go — pkgFromPath, the file-stem package label the hand-written
// extractors share. Moved from the retired CGO pyextract.go (LB-F393).

import "strings"

// pkgFromPath returns a simple package label derived from the file path
// (the base name without extension).
func pkgFromPath(relPath string) string {
	base := relPath
	// Find last slash.
	for i := len(relPath) - 1; i >= 0; i-- {
		if relPath[i] == '/' || relPath[i] == '\\' {
			base = relPath[i+1:]
			break
		}
	}
	// Strip extension.
	if dot := strings.LastIndex(base, "."); dot >= 0 {
		base = base[:dot]
	}
	return base
}
