//go:build unix

package lang_test

import (
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// The cache directory holds machine code the process executes: a directory
// another user owns is refused though no one else can write it by mode. /usr is
// owned by root and not group or world writable. A root test run owns it, so
// there is nothing to refuse.
func TestCompilationCacheRefusesADirectoryAnotherUserOwns(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root owns /usr")
	}
	fi, err := os.Stat("/usr")
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0o022 != 0 {
		t.Skipf("/usr is not a root-owned, unwritable directory here: %v", err)
	}
	if err := lang.EnableCompilationCache("/usr"); err == nil || !strings.Contains(err.Error(), "owned by another user") {
		t.Fatalf("a directory another user owns: %v", err)
	}
}
