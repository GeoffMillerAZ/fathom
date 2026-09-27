package fusion

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestProcessToolchainScavengesAndCleans (RT-106): a process's toolchain
// shim lives at <tmp>/fathom-toolchain-<pid>; installing it removes the
// shim directories of processes no longer running (a daemon SIGKILLed or
// exiting through os.Exit never ran its cleanup), keeps a live process's,
// puts the shim first on PATH, and the cleanup restores PATH and removes it.
func TestProcessToolchainScavengesAndCleans(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no shim on windows")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go on PATH")
	}
	t.Setenv("PATH", os.Getenv("PATH"))
	root := t.TempDir()
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	deadDir := filepath.Join(root, "fathom-toolchain-"+strconv.Itoa(dead.Process.Pid))
	liveDir := filepath.Join(root, "fathom-toolchain-"+strconv.Itoa(os.Getppid()))
	for _, d := range []string{deadDir, liveDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	undo, err := InstallProcessToolchain(root)
	if err != nil {
		t.Fatal(err)
	}
	own := filepath.Join(root, "fathom-toolchain-"+strconv.Itoa(os.Getpid()))
	if _, err := os.Stat(deadDir); !os.IsNotExist(err) {
		t.Fatal("a dead process's shim directory was not scavenged")
	}
	if _, err := os.Stat(liveDir); err != nil {
		t.Fatal("a live process's shim directory was removed")
	}
	if _, err := os.Stat(filepath.Join(own, "go")); err != nil {
		t.Fatalf("no shim at %s: %v", own, err)
	}
	if !strings.HasPrefix(os.Getenv("PATH"), own+string(os.PathListSeparator)) {
		t.Fatalf("PATH %q does not start with the shim", os.Getenv("PATH"))
	}
	undo()
	if _, err := os.Stat(own); !os.IsNotExist(err) {
		t.Fatal("cleanup left the shim directory")
	}
	if strings.Contains(os.Getenv("PATH"), own) {
		t.Fatal("cleanup left the shim on PATH")
	}
}
