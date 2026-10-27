package fusion

// shebangdispatch.go — D4 (O756, K15): a script with no extension (bats-core's
// bin/bats, a git hook, a libexec/ helper) is read by the dialect whose
// manifest declares the interpreter its `#!` line names. It is ONE rule in the
// content-sniff path: sniffedExtractor accepts connector.ClassifiedByShebang
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
	"context"
	"path"
	"path/filepath"
	"sort"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
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

// shebangDialectExt is the first extension of dialect's manifest when the
// manifest declares match.shebangs. A manifest load error leaves no dialect
// (the same fail-open convention as dialectRules).
func shebangDialectExt(dialect string) (string, bool) {
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
func (p *Pipeline) shebangExtractor(dialect string) (extractor.Extractor, bool) {
	x, ok := shebangDialectExt(dialect)
	if !ok {
		return nil, false
	}
	e, ok := p.exts[x]
	return e, ok
}

// ShebangDialect is the dialect ingest's dispatch reads relPath with by its
// shebang alone, "" when the rule does not claim it (an extension, no `#!`
// line, a binary file, an interpreter no manifest declares, or an earlier
// signal that claims the file). indexbench lang-oracle measures exactly these
// files (ownedSourceRefs; LB-F23: the harness measures what ingest dispatches).
func (p *Pipeline) ShebangDialect(relPath string, content []byte) string {
	if !shebangCandidate(relPath, content) || binaryExcluded(relPath, content) {
		return ""
	}
	v := connector.ClassifyDialect(dialectRules(), relPath, content)
	if v.By != connector.ClassifiedByShebang {
		return ""
	}
	se, ok := p.shebangExtractor(v.Dialect)
	if !ok {
		return ""
	}
	if e, _, ok, _ := p.resolveExtractor(relPath, content, nil); !ok || e != se {
		return ""
	}
	return v.Dialect
}

// shebangResolveLang is the file-resolve language (its dialect's language
// tag, LB-F11) an extensionless script the shebang rule dispatches resolves
// under (O793), "" when the rule does not claim relPath or its dialect names
// no import strategy.
func (p *Pipeline) shebangResolveLang(relPath string, content []byte) string {
	d := p.ShebangDialect(relPath, content)
	if d == "" {
		return ""
	}
	x, ok := shebangDialectExt(d)
	if !ok {
		return ""
	}
	return fileResolveLangByExt()[x]
}

var (
	shebangResolveOnce sync.Once
	shebangResolveAny  bool
)

// shebangScriptPath reports whether relPath may be a script the file
// resolver reads by its shebang: a path with no extension that no basename
// claims (a Makefile, a Dockerfile), while some dialect that declares a
// shebang also names an import strategy. It is a path test, so an edit to a
// text file with no extension (LICENSE) also runs the pass, which then reads
// and drops it.
func shebangScriptPath(relPath string) bool {
	if path.Ext(path.Base(relPath)) != "" || connector.LanguageForPath(relPath) != core.LangUnclassified {
		return false
	}
	shebangResolveOnce.Do(func() {
		ds, err := extractor.LoadDialects()
		if err != nil {
			return
		}
		for _, d := range ds {
			if len(d.Match.Shebangs) > 0 && len(d.Match.Extensions) > 0 && fileResolveLangByExt()[d.Match.Extensions[0]] != "" {
				shebangResolveAny = true
			}
		}
	})
	return shebangResolveAny
}

// shebangScripts adds to langFiles each script of scripts (shebangScriptPath
// refs) the shebang rule dispatches to a file-resolve language, read through
// the connector (the size cap and the binary sniff first), keeping each
// language's files in refs' order. A read the governor or ctx halts stops
// the pass; any other read failure leaves the file out.
func (p *Pipeline) shebangScripts(ctx context.Context, refs, scripts []core.SourceRef, langFiles map[string][]core.SourceRef) error {
	if len(scripts) == 0 {
		return nil
	}
	added := map[string]bool{}
	for _, r := range scripts {
		b, err := p.conn.Fetch(ctx, r)
		if err != nil {
			if h := haltError(ctx, err); h != nil {
				return h
			}
			continue
		}
		if len(b) > MaxExtractSize {
			continue
		}
		if lang := p.shebangResolveLang(r.Path, b); lang != "" {
			langFiles[lang] = append(langFiles[lang], r)
			added[lang] = true
		}
	}
	if len(added) == 0 {
		return nil
	}
	order := make(map[string]int, len(refs))
	for i, r := range refs {
		order[r.Path] = i
	}
	for lang := range added {
		fs := langFiles[lang]
		sort.SliceStable(fs, func(i, j int) bool { return order[fs[i].Path] < order[fs[j].Path] })
	}
	return nil
}
