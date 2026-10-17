package main

// intentindex.go — IV-2 (ADR-074 decision 1): `fathom intent index --check`,
// the row/heading parity gate for the hand-written decisions index.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var (
	adrHeadingRe  = regexp.MustCompile(`(?m)^## (ADR-[0-9]+)\b`)
	adrIndexRowRe = regexp.MustCompile(`(?m)^\| (ADR-[0-9]+) \|`)
)

// idCounts counts each id's matches of re in text.
func idCounts(re *regexp.Regexp, text string) map[string]int {
	out := map[string]int{}
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		out[m[1]]++
	}
	return out
}

// adrIndexDrift returns one sorted line per id that is not in exact parity:
// absent from a side or present more than once.
func adrIndexDrift(adrText, indexText string) []string {
	heads, rows := idCounts(adrHeadingRe, adrText), idCounts(adrIndexRowRe, indexText)
	ids := map[string]bool{}
	for id := range heads {
		ids[id] = true
	}
	for id := range rows {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	var out []string
	for _, id := range sorted {
		switch h, r := heads[id], rows[id]; {
		case h == 0:
			out = append(out, fmt.Sprintf("%s: row in docs/intent/decisions/README.md has no heading in ADR.md", id))
		case r == 0:
			out = append(out, fmt.Sprintf("%s: heading in ADR.md has no row in docs/intent/decisions/README.md", id))
		case h > 1:
			out = append(out, fmt.Sprintf("%s: %d headings in ADR.md (want exactly one)", id, h))
		case r > 1:
			out = append(out, fmt.Sprintf("%s: %d rows in docs/intent/decisions/README.md (want exactly one)", id, r))
		}
	}
	return out
}

// cmdIntentIndex implements `fathom intent index --check`: exit 1 naming each
// drifting id and the side that lacks it, 0 on parity. It never writes.
func cmdIntentIndex(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent index", stderr)
	root := fs.String("root", ".", "repository root")
	check := fs.Bool("check", false, "compare ADR.md's ADR-NNN headings with docs/intent/decisions/README.md's rows; exit 1 naming each drifting id")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*check {
		fmt.Fprintln(stderr, "error: intent index needs a mode: --check")
		return 1
	}
	adr, err := os.ReadFile(filepath.Join(*root, "ADR.md"))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	idx, err := os.ReadFile(filepath.Join(*root, "docs", "intent", "decisions", "README.md"))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	drift := adrIndexDrift(string(adr), string(idx))
	for _, l := range drift {
		fmt.Fprintln(stderr, l)
	}
	if len(drift) > 0 {
		fmt.Fprintf(stderr, "intent index: %d id(s) out of parity; edit docs/intent/decisions/README.md\n", len(drift))
		return 1
	}
	fmt.Fprintf(stdout, "intent index: %d decisions, headings and rows in parity\n", len(idCounts(adrHeadingRe, string(adr))))
	return 0
}
