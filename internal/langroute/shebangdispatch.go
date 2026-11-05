package langroute

// shebangdispatch.go — D4 (O756, K15): a script with no extension (bats-core's
// bin/bats, a git hook, a libexec/ helper) is read by the dialect whose
// manifest declares the interpreter its `#!` line names. It is ONE rule in the
// content-sniff path: sniffedExtractor accepts langclass.ClassifiedByShebang
// only for a file with no extension, and the classifier reads only the first
// line (connector's shebangInterpreter). The file goes to the extractor ingest
// already runs for the dialect's first extension (bash's .sh, python's .py,
// ruby's .rb, javascript's .js), so no new extractor instance is built; each
// of those extractors parses a path with no extension with its dialect's
// default grammar (TSExtractor.variantFor). A file with an extension is never
// claimed by its shebang, and an interpreter no manifest declares (perl, zsh)
// stays unextracted. O793: the file resolver keys files by extension, so the
// pass reads each extensionless script itself (shebangScripts) and resolves
// the ones the rule dispatches under their dialect's language tag, as
// importers and as targets.

import (
	"bytes"
	"path/filepath"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langclass"
)

var (
	shebangExtOnce sync.Once
	shebangExtOf   map[string]string // dialect id -> its manifest's first extension, for each dialect that declares match.shebangs
)

// shebangCandidate reports whether the shebang rule may claim relPath: a file
// with no extension whose first line is a `#!` line.
func shebangCandidate(relPath string, content []byte) bool {
	return filepath.Ext(relPath) == "" && bytes.HasPrefix(content, []byte("#!"))
}

// ShebangDialectExt is the first extension of dialect's manifest when the
// manifest declares match.shebangs. A manifest load error leaves no dialect
// (the same fail-open convention as DialectRules).
func ShebangDialectExt(dialect string) (string, bool) {
	shebangExtOnce.Do(func() {
		shebangExtOf = map[string]string{}
		ds, err := extractor.LoadDialects()
		if err != nil {
			return
		}
		for _, d := range ds {
			if len(d.Match.Shebangs) > 0 && len(d.Match.Extensions) > 0 {
				shebangExtOf[d.ID] = d.Match.Extensions[0]
			}
		}
	})
	x, ok := shebangExtOf[dialect]
	return x, ok
}

// shebangExtractor is the extractor ingest runs for dialect's first
// extension; ok is false when the dialect declares no shebang or nothing is
// registered for that extension (a refused dialect).
func (r *Router) shebangExtractor(dialect string) (extractor.Extractor, bool) {
	x, ok := ShebangDialectExt(dialect)
	if !ok {
		return nil, false
	}
	e, ok := r.exts[x]
	return e, ok
}

// ShebangDialect is the dialect ingest's dispatch reads relPath with by its
// shebang alone, "" when the rule does not claim it (an extension, no `#!`
// line, a binary file, an interpreter no manifest declares, or an earlier
// signal that claims the file). indexbench lang-oracle measures exactly these
// files (ownedSourceRefs; LB-F23: the harness measures what ingest dispatches).
func (r *Router) ShebangDialect(relPath string, content []byte) string {
	if !shebangCandidate(relPath, content) || BinaryExcluded(relPath, content) {
		return ""
	}
	v := langclass.ClassifyDialect(DialectRules(), relPath, content)
	if v.By != langclass.ClassifiedByShebang {
		return ""
	}
	se, ok := r.shebangExtractor(v.Dialect)
	if !ok {
		return ""
	}
	if e, _, ok, _ := r.ResolveExtractor(relPath, content, nil); !ok || e != se {
		return ""
	}
	return v.Dialect
}
