package certify

// lostdeclarations.go — LostDeclarations (CV-21, ADR-044 §2.4): a
// `// certifies:` marker declared in a changed Go file at base whose
// (kind, id) target no changed file declares at head, and is not retired,
// is a lost declaration — a governing test's own certification silently
// disappearing is exactly the kind of loss vision 34 exists to catch (a
// characterization/golden test that quietly stopped certifying anything).
//
// Pure over the given base/head reads; this package never imports
// internal/conserve (which imports this package, to call LostDeclarations
// from its certificates plane), so LostDeclarations returns its own
// LostDeclaration shape rather than conserve.Finding, and computes ONLY
// the base-vs-head fact — applying a plan's retires:/surface_change:
// excuse (I45) is internal/conserve's own concern, exactly like
// ObligationRule/CertificationRecord mirror internal/analysis/review's
// convention of never importing the packages whose data they carry.

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// LostDeclaration is one (kind, id) `// certifies:` target LostDeclarations
// found declared at base and undeclared at head. File/Line name where the
// BASE declaration lived (the marker's own comment position,
// extractor.CertifyDeclaration.File/Line, already parsed by
// certifiesTargetsAt and cheap to carry through) — empty only when that
// marker itself carried no location, which ScanCertifiesMarkers never
// actually produces today, but conserve.Finding's own contract still
// treats File="" as "no anchor" rather than assuming one.
type LostDeclaration struct {
	Kind string
	ID   string
	File string
	Line int
}

// LostDeclarations returns every (kind, id) `// certifies:` target
// declared in a changed Go file at base (baseFile) that no changed Go
// file declares at head (headFile) — sorted by (Kind, ID). changedFiles
// scopes the scan (ADR-044 §0: "base facts come from the changed files
// only" — a certifies marker in an untouched file cannot have gone
// missing). A file that fails to parse, or is absent at that point,
// contributes no declarations there — the same "skip unparseable, skip
// absent" discipline internal/analysis/routes and internal/conserve.Input
// already document; baseFile/headFile nil-safe (returns nil).
func LostDeclarations(service string, changedFiles []string, baseFile, headFile func(path string) ([]byte, bool, error)) []LostDeclaration {
	base := certifiesTargetsAt(service, changedFiles, baseFile)
	head := certifiesTargetsAt(service, changedFiles, headFile)

	var out []LostDeclaration
	for target, loc := range base {
		if _, ok := head[target]; ok {
			continue
		}
		kind, id, _ := strings.Cut(target, ":")
		out = append(out, LostDeclaration{Kind: kind, ID: id, File: loc.file, Line: loc.line})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// declSite is a `// certifies:` marker's own file:line, kept alongside the
// "<kind>:<id>" target set certifiesTargetsAt builds — the marker's
// location (extractor.CertifyDeclaration.File/Line) is already parsed by
// ScanCertifiesMarkers; carrying it through costs nothing LostDeclarations
// wasn't already doing.
type declSite struct {
	file string
	line int
}

// certifiesTargetsAt returns every "<kind>:<id>" target declared by any
// `// certifies:` marker in any of changedFiles, as read by read, mapped
// to that marker's own file:line (the first one found, when a target is
// declared more than once across changedFiles — LostDeclarations only
// needs A location, not every one).
func certifiesTargetsAt(service string, changedFiles []string, read func(path string) ([]byte, bool, error)) map[string]declSite {
	out := map[string]declSite{}
	if read == nil {
		return out
	}
	for _, path := range changedFiles {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		content, ok, err := read(path)
		if err != nil || !ok {
			continue
		}
		decls, _, derr := extractor.ScanCertifiesMarkers(service, path, content)
		if derr != nil {
			continue
		}
		for _, d := range decls {
			key := string(d.Kind) + ":" + d.TargetID
			if _, exists := out[key]; exists {
				continue
			}
			out[key] = declSite{file: d.File, line: d.Line}
		}
	}
	return out
}
