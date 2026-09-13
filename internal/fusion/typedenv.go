package fusion

// typedenv.go — BP-35 (docs/plans/bench-pyramid.md): the environment the
// typed-edge loader hands the `go` subprocess, and the per-service record
// of whether that pass actually ran.
//
// The defect (2026-09-12, /tmp/fathom-http.err): `fathom daemon install`
// writes a launchd plist whose EnvironmentVariables dict declares PATH and
// nothing else. launchd does not add the rest of a login shell's
// environment, so the daemon runs with GOPATH — and HOME — UNSET. With
// GOPATH unset, go1.26 resolves its default to the literal, unexpanded
// "$HOME/go" and refuses every command:
//
//	go: GOPATH entry is relative; must be absolute path: "$HOME/go"
//
// packages.Load surfaces that as a whole-load error, so EVERY daemon-side
// ingest of a Go service fell back to syntax-only edges while the same
// ingest run from a user shell type-checked fine. The fleet index was
// serving a weaker graph than the tests measured, and nothing in doctor's
// 35 checks or staleness_report said so — an instrument degrading toward
// healthy, which ADR-027 forbids.
//
// Two halves, both needed: repair the environment so the pass can run
// (TypedEdgeEnv), and record the failure ON THE SERVICE so a surface can
// report it (TypedEdgeStatus) rather than only on a stderr line nobody
// reads. `fathom daemon install` writing the Go variables into the plist
// (cmd/fathom/daemon.go, BP-37) closes the same hole at the source; this
// file is the belt to that pair of braces, and the only one that helps a
// daemon installed by an older binary.
//
// Deliberately NOT set: GOFLAGS. The invocation this replaces never set it
// and no repo fathom indexes needs -mod=mod; inventing a build flag here
// would change build semantics for every adopter to fix a problem none of
// them reported.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// TypedEdgeEnv returns the environment for the go/packages subprocess:
// this process's own environment, GOWORK=off (typed edges are per-service,
// never workspace-wide), and absolute values for GOPATH, GOCACHE and
// GOMODCACHE whenever the inherited ones are unset, empty, relative, or
// carry an unexpanded variable reference.
//
// Repair order per variable: the inherited value if it is already usable,
// else whatever `go env` answers (asked with an already-repaired GOPATH,
// since `go env` itself refuses to run under a broken one), else a
// home-relative default. A value that survives every fallback empty is
// left unset rather than guessed — the go command's own default is a
// better answer than a fabricated path.
//
// Later entries win in exec's environment, so appending is enough; the
// inherited entries are kept so nothing else (PATH, HOME, GOTOOLCHAIN,
// proxy settings) is silently dropped.
func TypedEdgeEnv() []string {
	env := make([]string, 0, len(os.Environ())+4)
	env = append(env, os.Environ()...)
	env = append(env, "GOWORK=off")

	home, _ := os.UserHomeDir()

	gopath := usablePath(os.Getenv("GOPATH"))
	if gopath == "" {
		// `go env GOPATH` under a broken GOPATH prints its refusal to
		// stderr and nothing to stdout, so an empty answer here is the
		// expected shape of the failure, not a surprise.
		gopath = usablePath(goEnvValue("GOPATH"))
	}
	if gopath == "" && home != "" {
		gopath = filepath.Join(home, "go")
	}
	if gopath != "" {
		env = append(env, "GOPATH="+gopath)
	}

	gocache := usablePath(os.Getenv("GOCACHE"))
	if gocache == "" {
		gocache = usablePath(goEnvValue("GOCACHE", "GOPATH="+gopath))
	}
	if gocache == "" {
		if dir, err := os.UserCacheDir(); err == nil {
			gocache = filepath.Join(dir, "go-build")
		}
	}
	if gocache != "" {
		env = append(env, "GOCACHE="+gocache)
	}

	gomodcache := usablePath(os.Getenv("GOMODCACHE"))
	if gomodcache == "" {
		gomodcache = usablePath(goEnvValue("GOMODCACHE", "GOPATH="+gopath))
	}
	if gomodcache == "" && gopath != "" {
		gomodcache = filepath.Join(gopath, "pkg", "mod")
	}
	if gomodcache != "" {
		env = append(env, "GOMODCACHE="+gomodcache)
	}

	return env
}

// usablePath returns v when every colon-separated entry in it is absolute
// and free of unexpanded variable references, and "" otherwise. The "$"
// test is not paranoia: "$HOME/go" is the EXACT string go1.26 handed the
// daemon, and a path that merely looks absolute after a failed expansion
// ("/go", say) would be worse than no answer at all.
func usablePath(v string) string {
	if v == "" {
		return ""
	}
	for _, part := range filepath.SplitList(v) {
		if part == "" || strings.Contains(part, "$") || !filepath.IsAbs(part) {
			return ""
		}
	}
	return v
}

// goEnvValue asks the go command for one variable, with extra applied on
// top of this process's environment (used to hand it a repaired GOPATH so
// it can answer at all). Returns "" for any failure — a missing or broken
// toolchain is exactly the case the caller's next fallback covers.
func goEnvValue(key string, extra ...string) string {
	if !goToolchainPresent() {
		return ""
	}
	cmd := exec.Command("go", "env", key)
	cmd.Env = append(os.Environ(), extra...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var goLookupOnce struct {
	sync.Once
	found bool
}

// goToolchainPresent caches the PATH lookup only — never the resolved
// variables, which tests and long-lived daemons must be able to see change.
func goToolchainPresent() bool {
	goLookupOnce.Do(func() {
		_, err := exec.LookPath("go")
		goLookupOnce.found = err == nil
	})
	return goLookupOnce.found
}

// ─── per-service typed-edge status ───────────────────────────────────────────

// TypedEdgeStatus is one service's most recent typed-edge pass outcome, as
// the ingest that ran it observed. Process-lifetime and in-memory, exactly
// like Pipeline.Stats' counters: it describes what THIS daemon has done
// since it started, which is the question "is the graph this daemon is
// serving typed?" actually asks. Known=false (the second return of
// Pipeline.TypedEdgeStatus) is the honest "no ingest of this service has
// run in this process" state — never reported as a clean pass.
type TypedEdgeStatus struct {
	// Packages is loaded + cache-hit packages on the last pass.
	Packages int
	// Failed is true when the pass could not run at all, or when at least
	// one package failed to type-check — both mean some edges the index
	// serves are syntax-only while claiming nothing.
	Failed bool
	// Reason names the failure (I24: a named reason, never a bare count).
	// Empty when Failed is false.
	Reason string
}

// TypedEdgeStatus returns the last typed-edge pass outcome recorded for
// service in this process. known is false when no ingest of that service
// has run here yet.
func (p *Pipeline) TypedEdgeStatus(service string) (status TypedEdgeStatus, known bool) {
	p.teMu.Lock()
	defer p.teMu.Unlock()
	st, ok := p.typedEdgeStatus[service]
	return st, ok
}

// noteTypedEdgeStatus records one pass's outcome for service.
func (p *Pipeline) noteTypedEdgeStatus(service string, st TypedEdgeStatus) {
	p.teMu.Lock()
	defer p.teMu.Unlock()
	if p.typedEdgeStatus == nil {
		p.typedEdgeStatus = make(map[string]TypedEdgeStatus)
	}
	p.typedEdgeStatus[service] = st
}
