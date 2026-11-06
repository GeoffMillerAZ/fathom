package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// numbered prints lines from..to (1-based, inclusive) of src with line numbers, at most max of them.
func numbered(src []string, from, to, max int) string {
	if from < 1 {
		from = 1
	}
	if to > len(src) {
		to = len(src)
	}
	var b strings.Builder
	for i := from; i <= to; i++ {
		if i-from >= max {
			fmt.Fprintf(&b, "   ... (%d more lines)\n", to-i+1)
			break
		}
		fmt.Fprintf(&b, "%5d  %s\n", i, src[i-1])
	}
	return b.String()
}

// leaf is the last segment of an attribute path, the text a source search looks for.
func leaf(p string) string {
	if i := strings.LastIndexAny(p, ".]"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// writeAudit prints the seeded audit pairs, each with the source the pair points at, for a reader to mark agree or
// disagree. An unset pair lists where the attribute's last segment occurs inside the resource.
func writeAudit(w io.Writer, dest string, subjects []Subject, run Run) error {
	byID := map[string]Subject{}
	for _, s := range subjects {
		byID[s.ID] = s
	}
	for i, p := range auditSelect(run.Pairs, 25) {
		s := byID[p.Subject]
		src, err := os.ReadFile(filepath.Join(s.root(dest), filepath.FromSlash(p.File)))
		if err != nil {
			return err
		}
		lines := strings.Split(string(src), "\n")
		fmt.Fprintf(w, "### A%02d %s %s %s.%s\n%s:%d-%d  path %s  scored %s\n", i+1, p.Rule, p.Subject, p.Type, p.Name, p.File, p.Line, p.EndLine, p.Path, p.Class)
		for _, a := range p.Attrs {
			fmt.Fprintf(w, "attr %s\n", a)
		}
		if len(p.AttrSpans) == 0 {
			var hits []string
			for n := p.Line; n <= p.EndLine && n <= len(lines); n++ {
				if strings.Contains(lines[n-1], leaf(p.Path)) {
					hits = append(hits, fmt.Sprint(n))
				}
			}
			fmt.Fprintf(w, "occurrences of %q inside the resource: %s\n", leaf(p.Path), strings.Join(append(hits, ""), " "))
			fmt.Fprint(w, numbered(lines, p.Line, p.EndLine, 14))
		}
		for _, sp := range p.AttrSpans {
			fmt.Fprint(w, numbered(lines, sp[0]-1, sp[1]+1, 14))
		}
		fmt.Fprintln(w)
	}
	return nil
}
