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
// GOFLAGS gains exactly one flag: -p=2 (RT-106, toolchainenv.go), the
// parallelism cap — never a flag that changes build semantics (no -mod,
// no tags); an inherited GOFLAGS is otherwise kept as it was.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/store"
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
// proxy settings) is silently dropped. RT-106: the toolchain caps
// (GOFLAGS -p=2, GOMAXPROCS=2) are applied last.
func TypedEdgeEnv() []string {
	return backgroundToolchainEnv(repairedGoEnv("GOWORK=off"))
}

// repairedGoEnv is TypedEdgeEnv's own repair logic (GOPATH/GOCACHE/
// GOMODCACHE, absolute and expanded), factored out so
// detectWorkspaceOverride can ask `go env GOWORK` under a WORKING
// environment without also forcing GOWORK=off — the same broken-GOPATH
// shape BP-35 exists to repair makes `go env <anything>` fail outright
// (`go: GOPATH entry is relative; must be absolute path: "$HOME/go"`),
// which would otherwise make workspace detection silently report "no
// override" on exactly the daemon environments most likely to need one.
// extra is appended last (e.g. "GOWORK=off" for TypedEdgeEnv, nothing for
// detection) — later entries win in exec's environment.
func repairedGoEnv(extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+4)
	env = append(env, os.Environ()...)

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

	env = append(env, extra...)
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

// detectWorkspaceOverride reports the Go workspace file GOWORK=off
// suppresses for repoPath (wave P3 / O82, .fathom/observations.md) — empty
// when none would apply. `go env GOWORK` run FROM repoPath's own directory,
// WITHOUT the GOWORK=off override, reflects exactly what an un-forced `go
// build` would auto-discover there: upward search for a go.work file, or an
// ambient GOWORK pinned globally via `go env -w` — the 2026-09-16 incident's
// exact shape, where a daemon process's own pinned workspace named sibling
// checkouts that existed only beside the operator's own checkout, not
// beside a push clone elsewhere. Best-effort: "" on any failure (no
// toolchain, a broken environment) or when the answer is "off"/empty —
// never blocking, never guessed.
func detectWorkspaceOverride(ctx context.Context, repoPath string) string {
	if !goToolchainPresent() {
		return ""
	}
	cmd := commandTree(ctx, "go", "env", "GOWORK") // RT-178: a halted pass kills it
	cmd.Dir = repoPath
	cmd.Env = repairedGoEnv() // repaired GOPATH/GOCACHE/GOMODCACHE, deliberately WITHOUT GOWORK=off
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	if v == "" || v == "off" {
		return ""
	}
	return v
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
	// PackagesSeen is every package go/packages returned on the last pass
	// (TypedEdgeReport.PackagesTotal), regardless of outcome — the
	// disclosure ingest's typed_edges.packages_seen carries (wave P3 /
	// O82). 0 when the load itself never got that far (e.g. a whole-load
	// error).
	PackagesSeen int
	// Packages is loaded + cache-hit packages on the last pass —
	// typed_edges.packages_processed.
	Packages int
	// EdgesTotal is every typed edge WRITTEN this pass (direct +
	// iface-dispatch + unresolved + func-value) — typed_edges.edges.
	EdgesTotal int
	// Failed is true when the pass could not run at all, or when at least
	// one package failed to type-check, or when packages were seen but
	// NONE were processed (every one excluded by the connector scope
	// filter — a path-mapping bug's own shape, never a legitimate
	// non-Go-tree zero) — all three mean some edges the index serves are
	// syntax-only while claiming nothing (ADR-027: an instrument may not
	// degrade toward healthy).
	Failed bool
	// Reason names the failure (I24: a named reason, never a bare count).
	// Empty when Failed is false.
	Reason string
	// Fallback names the override this pass applied (currently always
	// "GOWORK=off" when non-empty) — see TypedEdgeReport.Fallback.
	Fallback string
	// DiscoveryGaps/DiscoveryGapFiles/EdgesDroppedOutOfScope carry
	// TypedEdgeReport's own fields of the same name (typedcalls.go) onto
	// the per-service record (DG-9) — before this, list_services,
	// staleness_report and doctor's typed_edges row could never see a
	// discovery gap even though it was computed, deduped and printed to
	// stderr every run (docs/intent/invariants.md I24's documented gap).
	DiscoveryGaps          int
	DiscoveryGapFiles      []string
	EdgesDroppedOutOfScope int
	// Persisted is true when this status was NOT recorded by this process
	// (TypedEdgeStatus fell back to the store, DG-10) — the daemon has not
	// ingested this service since it last started, but a previous process
	// had. RecordedAt then names when that pass ran; a caller (doctor, in
	// particular) uses the pair to report "known as of <RecordedAt>,
	// possibly pre-restart" instead of the honest-but-unhelpful "never
	// known" a bare restart used to produce regardless of history.
	Persisted  bool
	RecordedAt time.Time
}

// TypedEdgeStatus returns the last typed-edge pass outcome recorded for
// service. known is false only when NEITHER this process NOR the store has
// ever recorded one for service — a fresh restart with a populated store
// (DG-10) still answers known=true, with Persisted=true and RecordedAt
// naming the pre-restart pass, rather than the "never known" a purely
// in-memory map produced regardless of what the store already knew.
func (p *Pipeline) TypedEdgeStatus(service string) (status TypedEdgeStatus, known bool) {
	p.teMu.Lock()
	st, ok := p.typedEdgeStatus[service]
	p.teMu.Unlock()
	if ok {
		return st, true
	}
	if p.st == nil {
		return TypedEdgeStatus{}, false
	}
	rec, found, err := p.st.TypedEdgeStatus(context.Background(), service)
	if err != nil || !found {
		return TypedEdgeStatus{}, false
	}
	return TypedEdgeStatus{
		PackagesSeen:           rec.PackagesSeen,
		Packages:               rec.Packages,
		EdgesTotal:             rec.EdgesTotal,
		Failed:                 rec.Failed,
		Reason:                 rec.Reason,
		Fallback:               rec.Fallback,
		DiscoveryGaps:          rec.DiscoveryGaps,
		DiscoveryGapFiles:      rec.DiscoveryGapFiles,
		EdgesDroppedOutOfScope: rec.EdgesDroppedOutOfScope,
		Persisted:              true,
		RecordedAt:             rec.RecordedAt,
	}, true
}

// noteTypedEdgeStatus records one pass's outcome for service, in-memory for
// this process AND persisted to the store (DG-10, best-effort: a store
// write failure never blocks or fails the surrounding ingest — it only
// means a restart before the NEXT successful pass falls back to "never
// known" instead of "known as of the last one," exactly the pre-DG-10
// shape).
func (p *Pipeline) noteTypedEdgeStatus(ctx context.Context, service string, st TypedEdgeStatus) {
	p.teMu.Lock()
	if p.typedEdgeStatus == nil {
		p.typedEdgeStatus = make(map[string]TypedEdgeStatus)
	}
	p.typedEdgeStatus[service] = st
	p.teMu.Unlock()

	if p.st == nil {
		return
	}
	now := time.Now().UTC()
	if err := p.st.SetTypedEdgeStatus(ctx, service, store.TypedEdgeStatusRecord{
		PackagesSeen:           st.PackagesSeen,
		Packages:               st.Packages,
		EdgesTotal:             st.EdgesTotal,
		Failed:                 st.Failed,
		Reason:                 st.Reason,
		Fallback:               st.Fallback,
		DiscoveryGaps:          st.DiscoveryGaps,
		DiscoveryGapFiles:      st.DiscoveryGapFiles,
		EdgesDroppedOutOfScope: st.EdgesDroppedOutOfScope,
		RecordedAt:             now,
	}); err != nil {
		// Finding [6]: this write failing (DB locked, disk full, the TCC
		// wedge on /Volumes this repo's own CLAUDE.md documents as
		// recurring) must never be silent — the "never silent" doctrine
		// (docs/vision/11-dial-to-eleven.md law 3). A swallowed error here
		// means DG-10's whole point (a restart falling back to "known as
		// of the last successful pass" instead of "never known") silently
		// stops working until the next successful write, with zero
		// operator signal — diagWarn is the same seam this package's other
		// failure branches already use (typedcalls.go).
		p.diagWarn("fusion: persisting typed-edge status for %q: %v — DG-10's restart fallback will not see this pass until the next successful write\n", service, err)
	}
}
