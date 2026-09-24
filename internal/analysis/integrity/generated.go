package integrity

// generated.go — CV-11 (ADR-044 §1.5): GeneratedHandEdits, a hand edit to
// a generated file whose generator inputs did not change in the range.

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// generatorInputExts is the same-directory generator-input class this
// function checks (design §1.5's "same-directory .proto, .cue, .sql").
var generatorInputExts = []string{".proto", ".cue", ".sql"}

// GeneratedInput bundles what GeneratedHandEdits needs.
type GeneratedInput struct {
	ChangedFiles []string
	BaseFile     func(path string) ([]byte, bool, error)
	HeadFile     func(path string) ([]byte, bool, error)
	// SiblingFiles maps a directory (repo-relative, "." for the root) to
	// every OTHER repo-relative file that exists in it at HEAD —
	// conserve.Input carries no directory-listing primitive of its own,
	// so the caller supplies this (e.g. from the store's indexed source
	// list, or a filepath.Glob against the worktree). A directory absent
	// from this map is treated as "could not be listed": GeneratedHandEdits
	// then names it in the returned Missing rather than either fabricating
	// a hand-edit finding it cannot actually prove, or a silent clean pass.
	SiblingFiles map[string][]string
}

// GeneratedHandEdits reports a changed file connector.IsGenerated tags at
// HEAD whose same-directory .proto/.cue/.sql generator inputs (design
// §1.5) did not change between base and head. missing names every changed
// generated file whose directory in.SiblingFiles could not list (I44: an
// unlistable directory means this function cannot PROVE the inputs were
// unchanged, so it reports the gap rather than staying silent).
//
// Scope (disclosed, narrower than design §1.5): a `//go:generate` source
// named in the package is NOT checked — only the same-directory .proto/
// .cue/.sql class is. This function only ever ADDS evidence it can prove;
// a generated file whose only generator input is a go:generate source is
// simply never flagged here (a false negative, never a false positive).
// CV-M3's 30-day kill window (docs/plans/refactor-conservation.md) is what
// will show whether this narrower version already catches real cases
// before the go:generate class is worth the extra parsing it would need.
func GeneratedHandEdits(in GeneratedInput) (findings []Finding, missing []string) {
	missingSet := map[string]bool{}
	for _, p := range in.ChangedFiles {
		head, ok, err := in.HeadFile(p)
		if err != nil || !ok || !connector.IsGenerated(p, head) {
			continue
		}
		base, baseOK, _ := in.BaseFile(p)
		if !baseOK {
			continue // newly added generated file: nothing to compare against, never a hand edit
		}
		if bytes.Equal(base, head) {
			continue // unchanged content: nothing to flag regardless of inputs
		}

		dir := path.Dir(p)
		siblings, listed := in.SiblingFiles[dir]
		if !listed {
			missingSet[fmt.Sprintf("directory %s could not be listed for %s's generator inputs", dir, p)] = true
			continue
		}
		inputs := generatorInputCandidates(dir, siblings)
		if len(inputs) == 0 {
			continue // no same-directory .proto/.cue/.sql input found — nothing this function can check
		}

		anyChanged := false
		for _, ip := range inputs {
			b, _, _ := in.BaseFile(ip)
			h, _, _ := in.HeadFile(ip)
			if !bytes.Equal(b, h) {
				anyChanged = true
				break
			}
		}
		if anyChanged {
			continue // a real generator input changed too — regenerating explains the diff
		}

		findings = append(findings, Finding{
			Kind:     KindHandEdit,
			Key:      p,
			Detail:   fmt.Sprintf("generated file changed but its generator input(s) (%s) did not", strings.Join(inputs, ", ")),
			BaseFile: p,
			HeadFile: p,
		})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Key < findings[j].Key })
	for m := range missingSet {
		missing = append(missing, m)
	}
	sort.Strings(missing)
	return findings, missing
}

// generatorInputCandidates returns every path in siblings (repo-relative,
// living in dir) whose extension is one of generatorInputExts, sorted.
func generatorInputCandidates(dir string, siblings []string) []string {
	var out []string
	for _, s := range siblings {
		if path.Dir(s) != dir {
			continue
		}
		for _, ext := range generatorInputExts {
			if strings.HasSuffix(s, ext) {
				out = append(out, s)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
