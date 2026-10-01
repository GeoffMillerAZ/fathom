package fusion

// toolchainenv.go — RT-106 (docs/design/responsible-tenant.md §7.2): the
// typed pass's toolchain children run capped and at background priority.
// go/packages runs `go list`, which compiles export data for every
// dependency with as many parallel compilers as the machine has cores; on
// a shared laptop that competed with the operator's own builds at full
// priority. Children now get GOFLAGS -p and GOMAXPROCS at most 2
// (FATHOM_TOOLCHAIN_PROCS moves or lifts the ceiling)
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

// defaultToolchainProcs is the parallelism ceiling for the typed pass's
// children.
const defaultToolchainProcs = 2

// toolchainProcsEnv moves the ceiling: a positive integer N caps at N,
// "off" lifts the caps entirely (the environment passes through; the shim
// still nices). Unset, empty or unparseable: defaultToolchainProcs. For an
// operator with cores to spare, and for measuring the typed pass's own cost
// (internal/fusion/perfgate's TestTypedEdges_PerformanceGate) without the
// deliberate throttle.
const toolchainProcsEnv = "FATHOM_TOOLCHAIN_PROCS"

// toolchainProcCap is the ceiling in force; 0 means no cap.
func toolchainProcCap() int {
	v := strings.TrimSpace(os.Getenv(toolchainProcsEnv))
	if strings.EqualFold(v, "off") {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	return defaultToolchainProcs
}

// capped returns min(inherited, ceiling) when inherited is a positive
// integer, else ceiling: the cap never raises what the environment already
// asked for (an operator's, or a gate's, -p=1 stays 1).
func capped(inherited string, ceiling int) int {
	if n, err := strconv.Atoi(inherited); err == nil && n > 0 && n < ceiling {
		return n
	}
	return ceiling
}

// backgroundToolchainEnv returns env with the typed pass's caps applied:
// GOMAXPROCS and GOFLAGS' -p each at most the ceiling (default 2) and never
// above what env already carried; every other inherited GOFLAGS flag is
// kept as it was — -p is parallelism, never build semantics. Later entries
// win in exec's environment, so the caps are appended. With the caps
// lifted (FATHOM_TOOLCHAIN_PROCS=off) env is returned unchanged.
func backgroundToolchainEnv(env []string) []string {
	ceiling := toolchainProcCap()
	if ceiling == 0 {
		return env
	}
	flags, procs := "", ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "GOFLAGS="); ok {
			flags = v
		}
		if v, ok := strings.CutPrefix(kv, "GOMAXPROCS="); ok {
			procs = v
		}
	}
	var kept []string
	inheritedP := ""
	for _, f := range strings.Fields(flags) {
		if v, ok := strings.CutPrefix(f, "-p="); ok {
			inheritedP = v
			continue
		}
		if f == "-p" {
			continue
		}
		kept = append(kept, f)
	}
	kept = append(kept, "-p="+strconv.Itoa(capped(inheritedP, ceiling)))
	out := make([]string, 0, len(env)+2)
	out = append(out, env...)
	return append(out, "GOFLAGS="+strings.Join(kept, " "), "GOMAXPROCS="+strconv.Itoa(capped(procs, ceiling)))
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
	caps := ""
	if ceiling := toolchainProcCap(); ceiling > 0 {
		// The same ceiling backgroundToolchainEnv applies, in sh: never
		// above the cap, never above what the caller already asked for.
		c := strconv.Itoa(ceiling)
		caps = "case \"${GOMAXPROCS:-}\" in ''|0|*[!0-9]*) GOMAXPROCS=" + c + " ;; *) [ \"$GOMAXPROCS\" -gt " + c + " ] && GOMAXPROCS=" + c + " ;; esac; export GOMAXPROCS\n" +
			"_fathom_p=; for _fathom_f in ${GOFLAGS:-}; do case $_fathom_f in -p=*) _fathom_p=${_fathom_f#-p=} ;; esac; done\n" +
			"case \"$_fathom_p\" in ''|0|*[!0-9]*) GOFLAGS=\"${GOFLAGS:+$GOFLAGS }-p=" + c + "\" ;; *) [ \"$_fathom_p\" -gt " + c + " ] && GOFLAGS=\"$GOFLAGS -p=" + c + "\" ;; esac; export GOFLAGS\n"
	}
	script := "#!/bin/sh\n" +
		"# fathom background toolchain shim (RT-106): the daemon's go, capped and niced.\n" +
		caps +
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
