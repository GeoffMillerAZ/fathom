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
// removed when a pass starts and written only when the pass completes with
// every package processed, so a halted, killed or failing pass leaves no
// state and the next ingest runs it again.

import (
	"context"
	"path/filepath"
	"strings"
)

// Typed-pass state keys and the value a successful pass records.
const (
	typedPassStatePrefix = "typed-pass-state:"
	goPassStateKey       = typedPassStatePrefix + "go"
	cuePassStateKey      = typedPassStatePrefix + "cue"
	passStateOK          = "ok"
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

// passStateOK reports whether key records a successful pass for service.
func (p *Pipeline) passStateOK(ctx context.Context, service, key string) bool {
	v, found, err := p.st.TypedEdgePackageFingerprint(ctx, service, key)
	return err == nil && found && v == passStateOK
}

// beginPassState removes key's success record as a pass starts.
func (p *Pipeline) beginPassState(ctx context.Context, service, key string) {
	_ = p.st.DeleteTypedEdgePackage(ctx, service, key)
}

// commitPassState records a successful pass.
func (p *Pipeline) commitPassState(ctx context.Context, service, key string) {
	_ = p.st.SetTypedEdgePackageFingerprint(ctx, service, key, passStateOK)
}

// isNamespacedTypedKey reports whether a typed-edge package key belongs to
// another pass (CUE, the file resolver) or is a pass-state row — never a Go
// package the Go pass may reconcile away.
func isNamespacedTypedKey(id string) bool {
	return strings.HasPrefix(id, cuePackageKeyPrefix) ||
		strings.HasPrefix(id, fileResolvedKeyPrefix) ||
		strings.HasPrefix(id, typedPassStatePrefix)
}

// SkippedPassReason is the Report's disclosure for a gated-off pass.
const SkippedPassReason = "no input changed since the last successful pass"
