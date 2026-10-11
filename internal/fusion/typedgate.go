package fusion

// typedgate.go — RT-104, RT-105 (docs/design/responsible-tenant.md §7.2):
// the typed Go pass and the CUE pass run only when one of their inputs
// changed since their last successful pass. Before, every ingest — a
// one-line README edit included — ran go/packages over the whole module
// (the audit's typed-pass-without-go-change hazard: ~1 GiB of heap and a
// fleet of `go list` compiles per auto-refresh).
//
// "Changed" is what this ingest's hash gate found new or changed plus what
// it found deleted. "Since the last successful pass" is a state row kept in
// the typed-edge fingerprint table under a namespaced key
// (typed-pass-state:go, typed-pass-state:cue — the same table the CUE pass
// ("cue:") and the file resolver ("file-resolved:") already share): it is
// removed when a pass starts and written when the pass completes — "ok"
// with every package processed, "partial" when packages failed typed
// loading or scope excluded them all (the same inputs fail the same way, so
// it gates too, and the skip says so) — so a halted, killed or unloadable
// pass leaves no state and the next ingest runs it again.

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// Typed-pass state keys and the value a successful pass records.
const (
	typedPassStatePrefix = "typed-pass-state:"
	goPassStateKey       = typedPassStatePrefix + "go"
	cuePassStateKey      = typedPassStatePrefix + "cue"
	passStateOK          = "ok"
	// passStatePartial records a pass that ran to completion with packages
	// that failed to load (or every package excluded by scope): deterministic
	// outcomes of the same inputs, so it gates like a clean pass — the next
	// run waits for an input change — and the skip says it was partial. A
	// pass halted, cancelled or failing to load at all records nothing and
	// runs again.
	passStatePartial = "partial"
)

// isGoInput reports whether path is an input of the typed Go pass: Go
// source or a module/workspace file.
func isGoInput(path string) bool {
	switch filepath.Base(path) {
	case "go.mod", "go.sum", "go.work", "go.work.sum":
		return true
	}
	return strings.HasSuffix(path, ".go")
}

// isCUEInput reports whether path is an input of the CUE pass (every .cue
// file, cue.mod/module.cue included).
func isCUEInput(path string) bool { return strings.HasSuffix(path, ".cue") }

// passNeeded is the gate both passes share: run when the last pass did not
// complete successfully, or when an input changed.
func passNeeded(changed []string, lastOK bool, isInput func(string) bool) bool {
	if !lastOK {
		return true
	}
	for _, p := range changed {
		if isInput(p) {
			return true
		}
	}
	return false
}

// typedPassNeeded reports whether the typed Go pass must run: a Go input
// (.go, go.mod, go.sum, go.work) is in changed, or no successful pass is
// recorded (lastOK false).
func typedPassNeeded(changed []string, lastOK bool) bool {
	return passNeeded(changed, lastOK, isGoInput)
}

// cuePassNeeded is typedPassNeeded for the CUE pass and .cue inputs.
func cuePassNeeded(changed []string, lastOK bool) bool {
	return passNeeded(changed, lastOK, isCUEInput)
}

// passStateOK reports whether key records a completed pass for service
// (clean or partial).
func (p *Pipeline) passStateOK(ctx context.Context, service, key string) bool {
	return p.passState(ctx, service, key) != ""
}

// passStateEpoch is the epoch a pass's state row carries: TypedEdgeEpoch
// for the typed Go pass, "" for a pass whose output has no epoch of its own.
//
// EW-63 (K26 condition d): the per-package fingerprints carry the
// TypedEdgeEpoch salt, but the gate above them skipped the whole pass when
// no Go input changed and a completed pass was recorded, so a binary with
// a new epoch never reached the salted fingerprints: every Go service kept
// the old epoch's typed edges until one of its .go files changed. The state
// row now records the epoch it was written under, and a row from another
// epoch (or from before rows carried one) reads as no completed pass.
func passStateEpoch(key string) string {
	if key == goPassStateKey {
		return TypedEdgeEpoch
	}
	return ""
}

// passStateValue is the row a completed pass with outcome writes under key:
// the outcome, and for an epoch-carrying pass "@epoch-<n>".
func passStateValue(key, outcome string) string {
	if e := passStateEpoch(key); e != "" {
		return outcome + "@epoch-" + e
	}
	return outcome
}

// passState is key's recorded outcome: passStateOK, passStatePartial or ""
// (no completed pass recorded at this epoch).
func (p *Pipeline) passState(ctx context.Context, service, key string) string {
	v, found, err := p.st.TypedEdgePackageFingerprint(ctx, service, key)
	if err != nil || !found {
		return ""
	}
	switch v {
	case passStateValue(key, passStateOK):
		return passStateOK
	case passStateValue(key, passStatePartial):
		return passStatePartial
	}
	return ""
}

// skippedReason is the Report's disclosure for a gated-off pass whose last
// run recorded state.
func (p *Pipeline) skippedReason(ctx context.Context, service, key string) string {
	if p.passState(ctx, service, key) == passStatePartial {
		return SkippedPartialPassReason
	}
	return SkippedPassReason
}

// beginPassState removes key's success record as a pass starts.
func (p *Pipeline) beginPassState(ctx context.Context, service, key string) {
	_ = p.st.DeleteTypedEdgePackage(ctx, service, key)
}

// commitPassState records a successful pass.
func (p *Pipeline) commitPassState(ctx context.Context, service, key string) {
	_ = p.st.SetTypedEdgePackageFingerprint(ctx, service, key, passStateValue(key, passStateOK))
}

// commitPartialPassState records a pass that completed with failed or
// excluded packages (passStatePartial).
func (p *Pipeline) commitPartialPassState(ctx context.Context, service, key string) {
	_ = p.st.SetTypedEdgePackageFingerprint(ctx, service, key, passStateValue(key, passStatePartial))
}

// isNamespacedTypedKey reports whether a typed-edge package key belongs to
// another pass (CUE, the file resolver, the doc-xref and route-ref passes)
// or is a pass-state row — never a Go package the Go pass may reconcile
// away. O335: the doc-xref and route-ref keys carry no prefix, so before
// they were named here a Go-only change (which skips both gated passes)
// purged their edges until a doc or route candidate next changed.
func isNamespacedTypedKey(id string) bool {
	return strings.HasPrefix(id, cuePackageKeyPrefix) ||
		strings.HasPrefix(id, fileResolvedKeyPrefix) ||
		strings.HasPrefix(id, typedPassStatePrefix) ||
		id == docXrefPackageKey || id == routeRefPackageKey || id == policyRefPackageKey
}

// SkippedPassReason is the Report's disclosure for a gated-off pass.
const SkippedPassReason = "no input changed since the last successful pass"

// SkippedPartialPassReason is SkippedPassReason when the last pass completed
// with packages that failed to load: its partial typed edges stand.
const SkippedPartialPassReason = "no input changed since the last pass, which completed with package(s) that failed typed loading — its partial typed edges stand"

// passKeysFor names the pass-state rows path is an input of: the typed Go
// pass, the CUE pass, the file resolver, and the doc-xref and route-ref
// passes.
func passKeysFor(path string) []string {
	var keys []string
	if isGoInput(path) {
		keys = append(keys, goPassStateKey)
	}
	if isCUEInput(path) {
		keys = append(keys, cuePassStateKey)
	}
	if fileResolveInput(path) {
		keys = append(keys, fileResolveStateKey)
	}
	if isDocXrefInput(path) {
		keys = append(keys, docXrefStateKey)
	}
	if isRouteCandidate(path) {
		keys = append(keys, routeRefStateKey)
	}
	if isPolicyCandidate(path) {
		keys = append(keys, policyRefStateKey)
	}
	return keys
}

// passInvalidator removes a pass's success record before the first new
// fingerprint of one of its inputs is written (design §7.2: "changed since
// the last successful pass"). The gate compares this ingest's changes only;
// a fingerprint written by the one-file refresh, or by an ingest that then
// failed before its passes ran, would otherwise make the input look
// unchanged to every later ingest while the success record still stood.
// Each key is removed at most once per invalidator.
type passInvalidator struct {
	st      store.Store
	service string
	done    map[string]bool
	// lambda is whether the service declares a Lambda function (EW-57):
	// 0 unread, 1 no, 2 yes — read at most once per invalidator.
	lambda int8
}

func newPassInvalidator(st store.Store, service string) *passInvalidator {
	return &passInvalidator{st: st, service: service, done: map[string]bool{}}
}

// note invalidates the passes path is an input of. An error means the
// record may still stand, so the caller must not write the fingerprint.
func (v *passInvalidator) note(ctx context.Context, path string) error {
	for _, key := range v.keysFor(ctx, path) {
		if v.done[key] {
			continue
		}
		if err := v.st.DeleteTypedEdgePackage(ctx, v.service, key); err != nil {
			return fmt.Errorf("fusion: invalidate %s for %q: %w", key, path, err)
		}
		v.done[key] = true
	}
	return nil
}

// keysFor is passKeysFor(path), and the doc-xref pass's record for a code
// file a Lambda handler binding may name when the service declares a Lambda
// function (EW-57: elsewhere such a file is no input of that pass). The
// store is read only while that record still stands to be removed.
func (v *passInvalidator) keysFor(ctx context.Context, path string) []string {
	keys := passKeysFor(path)
	if isLambdaHandlerSource(path) && !v.done[docXrefStateKey] && !slices.Contains(keys, docXrefStateKey) && v.declaresLambda(ctx) {
		keys = append(keys, docXrefStateKey)
	}
	return keys
}

// declaresLambda is serviceDeclaresLambda, read once per invalidator (one
// ingest, or one one-file refresh). A nil invalidator answers true: the
// pass runs, as before the gate.
func (v *passInvalidator) declaresLambda(ctx context.Context) bool {
	if v == nil {
		return true
	}
	if v.lambda == 0 {
		v.lambda = 1
		if serviceDeclaresLambda(ctx, v.st, v.service) {
			v.lambda = 2
		}
	}
	return v.lambda == 2
}
