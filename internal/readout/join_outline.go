package readout

// join_outline.go: the outline join (RO-8, design §5 and §7): the top-level
// symbols of one source file with their line ranges, from fathom's own
// extractors run in process over the text a whole-file read printed (never
// the daemon, never the index, so never stale). The caller computes it into
// Input.Joins["outline"] with Outline under OutlineDeadline; the engine
// stays pure.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// OutlineDeadline is the outline join's budget (design §5): past it the
// readout says OutlineDeadlineLine in place of the outline.
const OutlineDeadline = 150 * time.Millisecond

// OutlineDeadlineLine is the outline join's line when it missed its deadline.
const OutlineDeadlineLine = "outline: not computed (deadline)"

// outlineMaxSymbols bounds the outline's lines; the rest are counted.
const outlineMaxSymbols = 80

// outlineService is the service id the in-process extraction runs under: it
// names no repository, because the outline reads no index.
const outlineService = "outline"

// WholeFileRead returns the one source file a command prints whole: its last
// pipeline a lone `cat <file>` (no flag) or `sed -n 1,$p <file>`, one literal
// operand that fathom's extractors know by its path, no redirect. A range, a
// head or tail, several files or a pipe is not one.
func WholeFileRead(c *shellcmd.Command) (string, bool) {
	if c == nil || c.Unreadable || len(c.Pipelines) == 0 {
		return "", false
	}
	p := c.Pipelines[len(c.Pipelines)-1]
	if len(p.Units) != 1 {
		return "", false
	}
	return wholeRead(&p.Units[0])
}

// wholeRead is WholeFileRead for one unit: match.whole_read's test.
func wholeRead(u *shellcmd.Unit) (string, bool) {
	if u == nil || u.Kind != shellcmd.KindSimple || len(u.Redirects) > 0 || len(u.Words) == 0 {
		return "", false
	}
	var files []shellcmd.Word
	switch u.Name() {
	case "cat":
		for _, w := range u.Words[1:] {
			if strings.HasPrefix(w.Text, "-") {
				return "", false // -n, -A, -s and the rest change what prints
			}
			files = append(files, w)
		}
	case "sed":
		quiet, scripts, script := false, 0, ""
		for i := 1; i < len(u.Words); i++ {
			a := u.Words[i].Text
			switch {
			case a == "-n" || a == "--quiet" || a == "--silent":
				quiet = true
			case a == "-e":
				scripts++
				if i+1 < len(u.Words) {
					script = u.Words[i+1].Text
				}
				i++
			case strings.HasPrefix(a, "-") && a != "-":
				return "", false // -i, -f, -E and the rest: not a plain print
			case scripts == 0:
				scripts, script = 1, a
			default:
				files = append(files, u.Words[i])
			}
		}
		if !quiet || scripts != 1 || script != "1,$p" {
			return "", false
		}
	default:
		return "", false
	}
	if len(files) != 1 || !isSourceFile(files[0]) {
		return "", false
	}
	return files[0].Text, true
}

// Outline computes the outline join for path's printed text under ctx: the
// lines for Input.Joins["outline"] and whether the join missed. A ctx already
// done, or done by the time the extraction returns, gives
// OutlineDeadlineLine (missed); a file no extractor reads gives nil
// (missed, and the engine names it); otherwise the outline.
func Outline(ctx context.Context, path, text string) (lines []string, missed bool) {
	if ctx.Err() != nil {
		return []string{OutlineDeadlineLine}, true
	}
	out, err := joinOutline(path, text)
	if ctx.Err() != nil {
		return []string{OutlineDeadlineLine}, true
	}
	if err != nil {
		return nil, true
	}
	return out, false
}

// errNoOutline: no extractor reads the file (binary, oversize, generated,
// unclassified or no extractor for its language).
var errNoOutline = errors.New("no outline")

// joinOutline is the outline join, a pure function over (path, text): it runs
// the extractor the indexer dispatches for path (fusion's per-file entry,
// ExtractForBench: ingest's gates and dispatch, no store) over text and
// returns a header and one line per top-level symbol, `A-B kind name`, in
// line order. A symbol is top-level when no other symbol's range holds it;
// a node spanning the whole file (a module) holds nothing. The language
// comes from the path; the line numbers are text's, which is the file as
// printed.
func joinOutline(path, text string) ([]string, error) {
	rel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "/")
	nodes, err := extractOutline(rel, []byte(text))
	if err != nil {
		return nil, err
	}
	total := strings.Count(text, "\n")
	if text != "" && !strings.HasSuffix(text, "\n") {
		total++
	}
	type sym struct {
		a, b       int
		kind, name string
	}
	var syms []sym
	for _, n := range nodes {
		if n.Line <= 0 || n.Name == "" {
			continue
		}
		end := n.EndLine
		if end < n.Line {
			end = n.Line
		}
		if n.Line <= 1 && end >= total && total > 1 {
			continue // the module itself
		}
		syms = append(syms, sym{a: n.Line, b: end, kind: string(n.Kind), name: n.Name})
	}
	sort.Slice(syms, func(i, j int) bool {
		x, y := syms[i], syms[j]
		switch {
		case x.a != y.a:
			return x.a < y.a
		case x.b != y.b:
			return x.b > y.b
		case x.kind != y.kind:
			return x.kind < y.kind
		}
		return x.name < y.name
	})
	var top []sym
	for _, s := range syms {
		if k := len(top) - 1; k >= 0 && s.a <= top[k].b {
			continue // inside the previous top-level symbol
		}
		top = append(top, s)
	}
	if len(top) == 0 {
		return []string{fmt.Sprintf("outline of %s: no top-level symbols found", path)}, nil
	}
	out := []string{fmt.Sprintf("outline of %s, %d lines, %d top-level symbols (read one: sed -n A,Bp %s):", path, total, len(top), path)}
	for i, s := range top {
		if i == outlineMaxSymbols {
			out = append(out, fmt.Sprintf("  +%d more symbols", len(top)-outlineMaxSymbols))
			break
		}
		span := fmt.Sprintf("%d-%d", s.a, s.b)
		if s.a == s.b {
			span = fmt.Sprint(s.a)
		}
		out = append(out, fmt.Sprintf("  %s %s %s", span, s.kind, s.name))
	}
	return out, nil
}

// outlinePipe is the in-process extraction pipeline, built once per process
// on first use (no store, no connector); outlineMu serialises its use.
var (
	outlineOnce sync.Once
	outlinePipe *fusion.Pipeline
	outlineMu   sync.Mutex
)

// extractOutline runs the indexer's per-file dispatch over one file.
func extractOutline(rel string, src []byte) ([]core.SymbolNode, error) {
	outlineOnce.Do(func() { outlinePipe = fusion.New(nil, nil, fusion.Config{}) })
	outlineMu.Lock()
	defer outlineMu.Unlock()
	// The service names the file, so the sibling index the dispatch keeps
	// per service is this file's alone.
	ex, err := outlinePipe.ExtractForBench(outlineService+":"+rel, []core.SourceRef{{Service: outlineService, Path: rel}}, rel, src)
	if err != nil {
		return nil, err
	}
	if ex.Skipped != "" {
		return nil, fmt.Errorf("%w: %s", errNoOutline, ex.Skipped)
	}
	return ex.Nodes, nil
}
