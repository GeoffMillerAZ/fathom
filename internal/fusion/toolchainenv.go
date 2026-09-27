package fusion

// toolchainenv.go — RT-106 (docs/design/responsible-tenant.md §7.2): the
// typed pass's toolchain children run capped and at background priority.
// go/packages runs `go list`, which compiles export data for every
// dependency with as many parallel compilers as the machine has cores; on
// a shared laptop that competed with the operator's own builds at full
// priority. Children now get GOFLAGS=-p=2 and GOMAXPROCS=2
// (backgroundToolchainEnv, carried by TypedEdgeEnv), and run niced: Go's
// SysProcAttr has no niceness field and go/packages builds its own exec.Cmd,
// so the daemon installs a `go` shim first on its PATH
// (InstallBackgroundToolchain) that runs the real toolchain under
// `nice -n 10` — and `taskpolicy -b` on darwin — and every process it forks
// inherits the priority.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// toolchainProcs is the parallelism cap for the typed pass's children.
const toolchainProcs = "2"

// backgroundToolchainEnv returns env with the typed pass's caps applied:
// GOMAXPROCS=2, and GOFLAGS carrying -p=2 (other inherited flags kept; an
// inherited -p replaced). Later entries win in exec's environment, so the
// caps are appended.
func backgroundToolchainEnv(env []string) []string {
	flags := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GOFLAGS="); ok {
			flags = v
		}
	}
	var kept []string
	for _, f := range strings.Fields(flags) {
		if strings.HasPrefix(f, "-p=") || f == "-p" {
			continue
		}
		kept = append(kept, f)
	}
	kept = append(kept, "-p="+toolchainProcs)
	out := make([]string, 0, len(env)+2)
	out = append(out, env...)
	return append(out, "GOFLAGS="+strings.Join(kept, " "), "GOMAXPROCS="+toolchainProcs)
}

// shellQuote single-quotes s for sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// writeToolchainShim writes dir/go: a script that runs realGo at background
// priority with the caps, whatever environment it is started with. It
// returns the shim's path.
func writeToolchainShim(dir, realGo string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	prio := []string{}
	if nice, err := exec.LookPath("nice"); err == nil {
		prio = append(prio, shellQuote(nice), "-n", "10")
	}
	if runtime.GOOS == "darwin" {
		if _, err := os.Stat("/usr/sbin/taskpolicy"); err == nil {
			prio = append(prio, "/usr/sbin/taskpolicy", "-b")
		}
	}
	script := "#!/bin/sh\n" +
		"# fathom background toolchain shim (RT-106): the daemon's go, capped and niced.\n" +
		"GOMAXPROCS=" + toolchainProcs + "; export GOMAXPROCS\n" +
		"GOFLAGS=\"${GOFLAGS:+$GOFLAGS }-p=" + toolchainProcs + "\"; export GOFLAGS\n" +
		"exec " + strings.Join(append(prio, shellQuote(realGo)), " ") + " \"$@\"\n"
	p := filepath.Join(dir, "go")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(script), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		return "", err
	}
	return p, nil
}

// InstallBackgroundToolchain writes the shim for the `go` currently on PATH
// into dir and puts dir first on this process's PATH, so every `go` the
// process execs (go/packages' go list, go env) runs capped and niced. The
// cleanup restores PATH and removes dir. For a long-lived daemon only — it
// changes the process environment. No `go` on PATH: nothing to shim.
func InstallBackgroundToolchain(dir string) (func(), error) {
	realGo, err := exec.LookPath("go")
	if err != nil {
		return func() {}, nil
	}
	if abs, aerr := filepath.Abs(realGo); aerr == nil {
		realGo = abs
	}
	if filepath.Dir(realGo) == dir {
		return func() {}, nil // already installed
	}
	if _, err := writeToolchainShim(dir, realGo); err != nil {
		return func() {}, fmt.Errorf("toolchain shim: %w", err)
	}
	prev := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+prev); err != nil {
		return func() {}, err
	}
	return func() {
		_ = os.Setenv("PATH", prev)
		_ = os.RemoveAll(dir)
	}, nil
}

// toolchainDirPrefix names a process's shim directory:
// <tmp>/fathom-toolchain-<pid>.
const toolchainDirPrefix = "fathom-toolchain-"

// InstallProcessToolchain is InstallBackgroundToolchain at this process's
// own shim directory, <tmpRoot>/fathom-toolchain-<pid> (tmpRoot "" =
// os.TempDir()), for every process that runs the typed pass — the daemon
// and a CLI ingest alike (RT-106). It first removes the shim directories of
// processes no longer running: one killed (SIGKILL, the exit-mark ladder's
// os.Exit) never ran its cleanup.
func InstallProcessToolchain(tmpRoot string) (func(), error) {
	if tmpRoot == "" {
		tmpRoot = os.TempDir()
	}
	if entries, err := os.ReadDir(tmpRoot); err == nil {
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() || !strings.HasPrefix(name, toolchainDirPrefix) {
				continue
			}
			pid, perr := strconv.Atoi(strings.TrimPrefix(name, toolchainDirPrefix))
			if perr != nil || pid == os.Getpid() || processAlive(pid) {
				continue
			}
			_ = os.RemoveAll(filepath.Join(tmpRoot, name))
		}
	}
	dir := filepath.Join(tmpRoot, toolchainDirPrefix+strconv.Itoa(os.Getpid()))
	undo, err := InstallBackgroundToolchain(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
	}
	return undo, err
}
