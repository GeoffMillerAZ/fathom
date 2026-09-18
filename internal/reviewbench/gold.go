package reviewbench

import (
	"regexp"
	"sort"
	"strings"
)

// GoldEntry is one (file, function) pair B's fix diff names — what a review
// of A's diff would have to surface, ahead of the fix, to count as a hit.
type GoldEntry struct {
	File string
	Func string
}

// funcNameRE matches a Go function/method declaration's name from a single
// line of diff text: "func Name(", "func (r *T) Name(", or the trailing
// context of a unified-diff hunk header "@@ -1,2 +1,2 @@ func Name(...) {".
// The receiver clause (if any) is skipped non-greedily so the captured group
// is always the function/method's own name, never its receiver type.
var funcNameRE = regexp.MustCompile(`\bfunc\s+(?:\([^)]*\)\s*)?(\w+)\s*\(`)

// ExtractGold extracts every (file, function) pair referenced by PR B's
// diff (docs/bench/readings/vb24-review-historical-prs.md "Gold" — every
// (file, function name) pair extracted from B's fix-PR diff): the function
// name from a hunk header's "@@ ... @@ func Name(...)" trailing context,
// plus a regex over every +/-/context line that is itself a "func ..."
// declaration. Deduplicated and sorted for determinism.
func ExtractGold(pr PRBRef) []GoldEntry {
	seen := map[GoldEntry]bool{}
	for _, f := range pr.FilesChanged {
		if f.Patch == nil {
			continue
		}
		for _, name := range funcNamesInPatch(*f.Patch) {
			seen[GoldEntry{File: f.Filename, Func: name}] = true
		}
	}
	out := make([]GoldEntry, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Func < out[j].Func
	})
	return out
}

// funcNamesInPatch scans every line of a unified-diff patch for a function
// name: hunk headers ("@@ -a,b +c,d @@ <context>") are scanned on their
// trailing context, every other line is scanned as-is (covers a full "func
// Name(...)" declaration added, removed, or sitting in context).
func funcNamesInPatch(patch string) []string {
	var names []string
	for _, line := range strings.Split(patch, "\n") {
		text := line
		if strings.HasPrefix(line, "@@") {
			if idx := strings.Index(line[2:], "@@"); idx >= 0 {
				text = line[2+idx+2:]
			}
		}
		if m := funcNameRE.FindStringSubmatch(text); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}
