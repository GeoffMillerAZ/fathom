package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// probeMaxLines caps lang-probe's whole output (design §10): the probe's
// output lands in a worker's context, so it is bounded like any answer.
const probeMaxLines = 200

// probeMaxCaptureLines bounds the captures section so the tree always gets
// the rest of the budget.
const probeMaxCaptureLines = 96

// cmdLangProbe is `indexbench lang-probe -dialect <id> <file>` (LB-F20): the
// tree of a small file as an S-expression and every capture the dialect's
// queries make on it — the tool a worker writes queries against (§13 step
// 12), instead of reading grammar.js or node-types.json.
func cmdLangProbe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-probe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dialect := fs.String("dialect", "", "dialect id (a tags-engine dialect: "+strings.Join(extractor.TagsDialects(), ", ")+")")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dialect == "" || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: indexbench lang-probe -dialect <id> <file>")
		return 2
	}
	path := fs.Arg(0)
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, "lang-probe:", err)
		return 2
	}
	target, err := extractor.ProbeTargetForSource(*dialect, path, src)
	if err != nil {
		fmt.Fprintln(stderr, "lang-probe:", err)
		return 2
	}
	lines, err := probeLines(target, src)
	if err != nil {
		fmt.Fprintln(stderr, "lang-probe:", err)
		return 1
	}
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
	return 0
}

func probeLines(target extractor.ProbeTarget, src []byte) ([]string, error) {
	ctx := context.Background()
	p, err := tsengine.NewParser(ctx, target.Blob, target.EntryPoint)
	if err != nil {
		return nil, err
	}
	defer p.Close()
	q, err := p.CompileQuery(target.Query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", target.QueryFile, err)
	}
	defer q.Close()
	var note string
	if target.Preprocess != nil {
		if texts := target.Preprocess(src); len(texts) > 0 {
			src, note = texts[0], "; preprocess: the dialect's pre-parse step rewrote the file (blanked spans, same lines)"
		}
	}
	tree, err := p.Parse(src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	matches, err := q.Run(tree)
	if err != nil {
		return nil, err
	}

	out := []string{fmt.Sprintf("; lang-probe dialect=%s grammar=%s entry=%s query=%s",
		target.Dialect, target.Blob, target.EntryPoint, target.QueryFile)}
	if note != "" {
		out = append(out, note)
	}
	var caps []string
	for _, m := range matches {
		for _, c := range m.Captures {
			caps = append(caps, fmt.Sprintf("pattern %d @%s L%d %q", m.PatternIndex, c.Name, c.Node.Line(), firstLine(c.Node.Text(src), 60)))
		}
	}
	out = append(out, fmt.Sprintf("; captures (%d)", len(caps)))
	if len(caps) > probeMaxCaptureLines {
		out = append(out, caps[:probeMaxCaptureLines]...)
		out = append(out, fmt.Sprintf("; … %d more captures not shown", len(caps)-probeMaxCaptureLines))
	} else {
		out = append(out, caps...)
	}
	out = append(out, "; tree")
	treeLines, dropped, err := tree.SExpr(probeMaxLines - len(out) - 1)
	if err != nil {
		return nil, err
	}
	out = append(out, treeLines...)
	if dropped > 0 {
		out = append(out, fmt.Sprintf("; … %d more nodes not shown", dropped))
	}
	return out, nil
}

// firstLine returns s's first line, capped at n bytes with an ellipsis.
func firstLine(s string, n int) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + "…"
	}
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}
