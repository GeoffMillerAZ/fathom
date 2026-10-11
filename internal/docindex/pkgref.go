package docindex

// pkgref.go — the coordinates of one package version and the report kept
// beside its doc units (KR-12, docs/design/knowledge-retrieval.md §7). The
// reader that filled the index from the toolchain cache (KR-11:
// IngestDepDocs, depDocUnits and the per-language extractors) was removed
// while its only intended caller, the dependency plane's indexer (SG-37), is
// unbuilt; reverting the removal commit restores it. Nothing here reads a
// cache directory or fetches (I56; TestDepDocsNeverDial).

import (
	"errors"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/syslink"
)

// Unit kinds recorded on doc_units.kind for a dependency's docs (beside
// split.go's KindSection and friends).
const (
	// KindDepDoc is one doc comment or docstring, keyed to the API symbol it
	// documents (heading path = the symbol).
	KindDepDoc = "dep-doc"
	// KindDepReadme is a section of the package's own README.
	KindDepReadme = "dep-readme"
)

// depReportRef is the fingerprint row that marks a package version as
// indexed (even with no doc units) and carries what indexing cut.
const depReportRef = "depdocs:report"

// depIndexReport is the JSON kept in the depReportRef fingerprint.
type depIndexReport struct {
	Elided  int    `json:"elided"`
	Skipped int    `json:"skipped"`
	Partial string `json:"partial,omitempty"`
}

var (
	// ErrBadPkgKey is ParsePkgRef's refusal of a key that is not a
	// `pkg:<ecosystem>/<name>[@<version>]` coordinate.
	ErrBadPkgKey = errors.New("docindex: not a pkg: key")
)

// PkgRef is the coordinates of one package version: a `pkg:` contract key
// (internal/syslink's grammar) without its #symbol part.
type PkgRef struct {
	Ecosystem string // golang, npm, pypi, maven, cargo or nuget
	Name      string
	Version   string // as the lockfile pins it; empty only for an unpinned scope
}

// Key renders the canonical `pkg:` key, "pkg:<ecosystem>/<name>[@<version>]".
// It is also the service id the package's doc units are stored under: a doc
// unit is version-exact because the version is part of its scope.
func (r PkgRef) Key() string {
	k := "pkg:" + r.Ecosystem + "/" + r.Name
	if r.Version != "" {
		k += "@" + r.Version
	}
	return k
}

// ParsePkgRef parses a `pkg:` key (the prefix may be omitted) through
// syslink.ParseContractKey's own grammar, so the ecosystem list and its
// normalization stay one definition. A `#symbol` part is refused: a doc
// scope is a package version, and the symbol belongs in the query. The
// version is optional here so an unpinned scope can be named (never
// guessed) by the caller; a key the grammar rejects reports its reason.
func ParsePkgRef(s string) (PkgRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return PkgRef{}, fmt.Errorf("%w: empty key", ErrBadPkgKey)
	}
	if !strings.HasPrefix(s, "pkg:") {
		s = "pkg:" + s
	}
	ck, err := syslink.ParseContractKey(s)
	if err != nil {
		return PkgRef{}, fmt.Errorf("%w: %v", ErrBadPkgKey, err)
	}
	eco, rest, _ := strings.Cut(ck.Body, "/")
	if strings.Contains(rest, "#") {
		return PkgRef{}, fmt.Errorf("%w: %q carries a #symbol part; scope a docs search to ecosystem/name@version and put the symbol in the query", ErrBadPkgKey, s)
	}
	name, version := rest, ""
	if at := strings.LastIndexByte(rest, '@'); at > 0 {
		name, version = rest[:at], rest[at+1:]
	}
	return PkgRef{Ecosystem: eco, Name: name, Version: version}, nil
}
