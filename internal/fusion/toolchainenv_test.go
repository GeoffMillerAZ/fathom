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

func envValue(env []string, key string) (string, bool) {
	v, ok := "", false
	for _, kv := range env {
		if k, val, found := strings.Cut(kv, "="); found && k == key {
			v, ok = val, true // later entries win, as in exec
		}
	}
	return v, ok
}

// TestTypedPassChildrenNicedAndCapped is RT-106 (design §7.2): the typed
// pass's toolchain children run with GOFLAGS carrying -p=2 (an inherited
// GOFLAGS is kept, an inherited -p replaced), GOMAXPROCS=2, and at
// background priority — the `go` the daemon execs is a shim that runs the
// real toolchain under `nice -n 10` (and `taskpolicy -b` on darwin when it
// exists), so `go list` and every compile it forks inherit it.
func TestTypedPassChildrenNicedAndCapped(t *testing.T) {
	base := []string{"PATH=/usr/bin", "GOFLAGS=-mod=mod -p=8", "GOMAXPROCS=16", "HOME=/h"}
	env := backgroundToolchainEnv(base)
	flags, _ := envValue(env, "GOFLAGS")
	if !strings.Contains(flags, "-mod=mod") || !strings.Contains(flags, "-p=2") || strings.Contains(flags, "-p=8") {
		t.Fatalf("GOFLAGS = %q; want -mod=mod kept and -p=2 in place of -p=8", flags)
	}
	if v, _ := envValue(env, "GOMAXPROCS"); v != "2" {
		t.Fatalf("GOMAXPROCS = %q, want 2", v)
	}
	if v, _ := envValue(env, "HOME"); v != "/h" {
		t.Fatal("inherited variables must survive")
	}
	// TypedEdgeEnv (what packages.Load gets) carries the caps.
	te := TypedEdgeEnv()
	if v, _ := envValue(te, "GOMAXPROCS"); v != "2" {
		t.Fatalf("TypedEdgeEnv GOMAXPROCS = %q, want 2", v)
	}
	if v, _ := envValue(te, "GOFLAGS"); !strings.Contains(v, "-p=2") {
		t.Fatalf("TypedEdgeEnv GOFLAGS = %q, want -p=2", v)
	}

	if runtime.GOOS == "windows" {
		t.Skip("no nice on windows")
	}
	// The shim: a stand-in "real go" reports the niceness and caps it ran
	// with, and the arguments it was handed.
	dir := t.TempDir()
	fake := filepath.Join(dir, "realgo")
	script := "#!/bin/sh\nprintf '%s|%s|%s|%s\\n' \"$(ps -o nice= -p $$ | tr -d ' ')\" \"$GOMAXPROCS\" \"$GOFLAGS\" \"$*\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	shimDir := filepath.Join(dir, "shim")
	shim, err := writeToolchainShim(shimDir, fake)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(shim) != "go" {
		t.Fatalf("shim is %s; it must be named go so PATH lookups of go find it", shim)
	}
	cmd := exec.Command(shim, "list", "-json", "./...")
	cmd.Env = append(os.Environ(), "GOMAXPROCS=16", "GOFLAGS=-mod=mod")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shim: %v", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) != 4 {
		t.Fatalf("shim output %q", out)
	}
	nice, err := strconv.Atoi(parts[0])
	if err != nil || nice < 10 {
		t.Fatalf("toolchain child niceness %q; want ≥ 10", parts[0])
	}
	if parts[1] != "2" || !strings.Contains(parts[2], "-p=2") || !strings.Contains(parts[2], "-mod=mod") {
		t.Fatalf("toolchain child GOMAXPROCS=%q GOFLAGS=%q; want 2 and -p=2 (inherited flags kept)", parts[1], parts[2])
	}
	if parts[3] != "list -json ./..." {
		t.Fatalf("arguments reached the real toolchain as %q", parts[3])
	}
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/usr/sbin/taskpolicy"); err == nil {
			raw, _ := os.ReadFile(shim)
			if !strings.Contains(string(raw), "taskpolicy -b") {
				t.Fatalf("darwin shim does not run the toolchain under taskpolicy -b:\n%s", raw)
			}
		}
	}

	// Installing the shim puts it first on this process's PATH (so every
	// `go` the daemon execs — go/packages' go list, go env — is the shim)
	// and the cleanup restores PATH.
	realPath := os.Getenv("PATH")
	t.Setenv("PATH", realPath)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	cleanup, err := InstallBackgroundToolchain(filepath.Join(dir, "install"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := exec.LookPath("go")
	if filepath.Dir(got) != filepath.Join(dir, "install") {
		cleanup()
		t.Fatalf("go resolves to %s after install; want the shim", got)
	}
	cleanup()
	if os.Getenv("PATH") != realPath {
		t.Fatal("cleanup did not restore PATH")
	}
}
