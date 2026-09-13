package scenariobench

import (
	"os"
	"testing"
)

// writeTestFile is a small shared helper: write content to path, failing
// the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
