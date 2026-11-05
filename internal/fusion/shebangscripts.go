package fusion

// shebangscripts.go — the file resolver's half of the shebang rule (O793):
// the pass reads each extensionless script itself (shebangScripts) and resolves
// the ones the dispatch (langroute's Router.ShebangDialect) claims under their
// dialect's language tag, as importers and as targets.

import (
	"context"
	"path"
	"sort"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// shebangResolveLang is the file-resolve language (its dialect's language
// tag, LB-F11) an extensionless script the shebang rule dispatches resolves
// under (O793), "" when the rule does not claim relPath or its dialect names
// no import strategy.
func (p *Pipeline) shebangResolveLang(relPath string, content []byte) string {
	d := p.ShebangDialect(relPath, content)
	if d == "" {
		return ""
	}
	x, ok := langroute.ShebangDialectExt(d)
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
