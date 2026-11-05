package lang_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// The compilation cache is process state, so the cases that succeed run in a
// child process: the test binary re-run on TestCacheHelper.
func TestEnableCompilationCache(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	os.Chmod(other, 0o700)
	loose := t.TempDir()
	if err := os.Chmod(loose, 0o770); err != nil {
		t.Fatal(err)
	}
	grp := filepath.Join(loose, "cache")

	// Refusals leave the process unchanged, so they run here.
	for name, d := range map[string]string{"relative": "cache", "empty": "", "group writable": loose} {
		if err := lang.EnableCompilationCache(d); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if err := os.Mkdir(grp, 0o770); err != nil {
		t.Fatal(err)
	}
	os.Chmod(grp, 0o770)
	if err := lang.EnableCompilationCache(grp); err == nil || !strings.Contains(err.Error(), "writable") {
		t.Errorf("group-writable directory: %v", err)
	}
	world := t.TempDir()
	os.Chmod(world, 0o777)
	if err := lang.EnableCompilationCache(world); err == nil || !strings.Contains(err.Error(), "writable") {
		t.Errorf("world-writable directory: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestCacheHelper$", "-test.v")
	cmd.Env = append(os.Environ(), "LANG_CACHE_HELPER="+dir, "LANG_CACHE_OTHER="+other)
	if out, err := cmd.CombinedOutput(); err != nil || !strings.Contains(string(out), "--- PASS: TestCacheHelper") {
		t.Fatalf("helper: %v\n%s", err, out)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}

func TestCacheHelper(t *testing.T) {
	dir, other := os.Getenv("LANG_CACHE_HELPER"), os.Getenv("LANG_CACHE_OTHER")
	if dir == "" {
		t.Skip("runs in the child process of TestEnableCompilationCache")
	}
	if err := lang.EnableCompilationCache(dir); err != nil {
		t.Fatal(err)
	}
	if err := lang.EnableCompilationCache(dir); err != nil {
		t.Errorf("the same directory twice: %v", err)
	}
	if err := lang.EnableCompilationCache(other); err == nil {
		t.Error("a second directory must error")
	}
	r, err := lang.New()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := r.Extract(t.Context(), lang.File{Path: "a.py", Src: []byte("def f():\n    pass\n")}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		t.Errorf("the cache directory holds the wazero version directory: %v %v", entries, err)
	}
}
