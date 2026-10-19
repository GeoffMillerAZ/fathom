package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// intentexport.go — edit-time intent, generated (ADR-074 decision 6, IV-3a).
//
// The router's sensitive-surface table already says which globs matter and
// what evidence a change to them needs. `.fathom/guard.cue` already serves
// exactly that shape before an edit. So the edit-time channel for intent is
// not a second injector and not a second map: guard.cue's intent entries are
// GENERATED from the table. EP-21 printed them for a human to paste; 15
// pasted entries reached 15 of 66 invariants and drifted by hand (O1134).
//
//   - no flag: print the entries, as before (the proposal).
//   - --write: write them inside fathom's own markers in .fathom/guard.cue,
//     every hand-written entry untouched (P15: fathom writes only inside its
//     markers, a hand-edited region stops and reports, the write leaves a
//     receipt line).
//   - --check: exit 1 naming the first entry that differs from the router;
//     a step of `task validate`, so the table and the file are one map.

const (
	// intentGuardBegin / intentGuardEnd delimit the generated region. The
	// begin line carries the body's digest, so a hand edit is detectable.
	intentGuardBegin = "// fathom:intent-guards begin"
	intentGuardEnd   = "// fathom:intent-guards end"
	// intentGuardMaxRules is a generated entry's governing-rule cap when no
	// hand guard matches the file: the cap the pasted EP-21 entries carried,
	// so an edit under a surface costs about the bytes it did and K1
	// (IV-M1) reads delivery, not a bigger payload. With the at most two
	// intent lines it stays inside I35's six; behind a hand guard,
	// intentRuleCap applies.
	intentGuardMaxRules = 2
)

// intentGuardEntry is one generated guard entry: a router row's globs and
// the pointer note the edit hook renders into its intent slots.
type intentGuardEntry struct {
	Scopes []string
	Note   string
}

func cmdIntentExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("intent export", stderr)
	root := fs.String("root", ".", "repository root")
	guard := fs.Bool("guard", false, ".fathom/guard.cue entries from the router's sensitive surfaces (printed unless --write or --check)")
	write := fs.Bool("write", false, "with --guard: write the entries inside fathom's markers in .fathom/guard.cue, hand-written entries untouched")
	check := fs.Bool("check", false, "with --guard: exit 1 naming the first entry that differs from the router (writes nothing)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !*guard {
		fmt.Fprintln(stderr, "error: intent export needs --guard (the only export that exists today)")
		return 1
	}
	if *write && *check {
		fmt.Fprintln(stderr, "error: --write and --check are exclusive")
		return 1
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	c, err := intent.Load(abs)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	entries := intentGuardEntries(c)
	switch {
	case *write:
		return writeIntentGuards(abs, c.RouterPath, entries, stdout, stderr)
	case *check:
		return checkIntentGuards(abs, c.RouterPath, entries, stdout, stderr)
	}
	if len(entries) == 0 {
		fmt.Fprintln(stderr, "no sensitive surfaces in the router — nothing to propose")
		return 0
	}
	fmt.Fprintln(stdout, "// PROPOSED guard entries, derived from the router's sensitive-surface table.")
	fmt.Fprintln(stdout, "// `fathom intent export --guard --write` writes them inside fathom's markers; the")
	fmt.Fprintln(stdout, "// @ stamp lets `fathom intent audit` tell a stale entry from a covered one.")
	fmt.Fprintf(stdout, "// Source: %s\n\n", c.RouterPath)
	fmt.Fprintln(stdout, "guards: [")
	fmt.Fprint(stdout, renderIntentGuardEntries(entries))
	fmt.Fprintln(stdout, "]")
	return 0
}

// intentGuardEntries derives one entry per router surface, in table order.
func intentGuardEntries(c intent.Corpus) []intentGuardEntry {
	byID := map[string]intent.Invariant{}
	for _, inv := range c.Invariants {
		byID[inv.ID] = inv
	}
	out := make([]intentGuardEntry, 0, len(c.Surfaces))
	for _, s := range c.Surfaces {
		out = append(out, intentGuardEntry{Scopes: s.Globs, Note: guardNote(s, byID)})
	}
	return out
}

// wordInvariantIDRe finds an invariant id as a whole word: `I1` must not match
// inside `I12` (EP-21's substring match credited I1 to every row naming I1x).
var wordInvariantIDRe = regexp.MustCompile(`\bI\d+\b`)

// surfaceInvariantIDs returns the known invariant ids a surface row names
// (either column), sorted by number.
func surfaceInvariantIDs(s intent.Surface, byID map[string]intent.Invariant) []string {
	seen := map[string]bool{}
	var ids []string
	for _, m := range wordInvariantIDRe.FindAllString(s.Why+" "+s.DoneNeeds, -1) {
		if _, ok := byID[m]; ok && !seen[m] {
			seen[m] = true
			ids = append(ids, m)
		}
	}
	sortInvariantIDs(ids)
	return ids
}

// sortInvariantIDs orders ids by their number (I2 before I12).
func sortInvariantIDs(ids []string) {
	num := func(id string) int {
		n, _ := strconv.Atoi(strings.TrimPrefix(id, "I"))
		return n
	}
	sort.SliceStable(ids, func(i, j int) bool {
		if a, b := num(ids[i]), num(ids[j]); a != b {
			return a < b
		}
		return ids[i] < ids[j]
	})
}

// guardNote renders one surface as a pointer, never a copy: its globs (the
// provenance), the invariant IDS and their rungs, the Done-needs tests, and
// the row's stamp. The statements live in invariants.md and stay there.
func guardNote(s intent.Surface, byID map[string]intent.Invariant) string {
	var ids []string
	for _, id := range surfaceInvariantIDs(s, byID) {
		ids = append(ids, fmt.Sprintf("%s [%s]", id, byID[id].Rung))
	}
	parts := []string{}
	if len(ids) > 0 {
		parts = append(parts, strings.Join(ids, " · "))
	}
	if len(s.Tests) > 0 {
		tests := append([]string(nil), s.Tests...)
		sort.Strings(tests)
		parts = append(parts, "done needs: "+strings.Join(tests, ", "))
	}
	if len(parts) == 0 {
		parts = append(parts, "done needs: "+strings.Join(strings.Fields(s.DoneNeeds), " "))
	}
	return fmt.Sprintf("intent: %s — %s @ %s", strings.Join(s.Globs, ", "), strings.Join(parts, "; "), intent.SurfaceStamp(s))
}

// renderIntentGuardEntries renders entries as CUE list elements, one tab in.
func renderIntentGuardEntries(entries []intentGuardEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "\t{\n\t\tscopes: [%s]\n\t\tnote: %q\n\t\tmax_rules: %d\n\t},\n",
			quoteJoin(e.Scopes), e.Note, intentGuardMaxRules)
	}
	return sb.String()
}

// intentGuardBody is what sits between the markers: a two-line comment
// naming the source and the entries.
func intentGuardBody(routerPath string, entries []intentGuardEntry) string {
	return fmt.Sprintf("\t// Generated by `fathom intent export --guard --write` from %s's sensitive-surface\n"+
		"\t// table (ADR-074 decision 6): edit the router and re-run, never these entries;\n"+
		"\t// `--check` (task intent-guard-check, in task validate) fails on any difference.\n", routerPath) +
		renderIntentGuardEntries(entries)
}

func intentGuardDigest(body string) string {
	sum := sha256.Sum256([]byte(body))
	return "sha256:" + hex.EncodeToString(sum[:])[:16]
}

func renderIntentGuardRegion(routerPath string, entries []intentGuardEntry) string {
	body := intentGuardBody(routerPath, entries)
	return "\t" + intentGuardBegin + " " + intentGuardDigest(body) + "\n" + body + "\t" + intentGuardEnd + "\n"
}

// intentRegion is the generated region as it sits in a guard.cue today.
type intentRegion struct {
	present        bool
	start, end     int // byte span of the whole region, marker lines included
	recordedDigest string
	body           string
}

func findIntentRegion(content string) (intentRegion, error) {
	b, e := strings.Count(content, intentGuardBegin), strings.Count(content, intentGuardEnd)
	switch {
	case b == 0 && e == 0:
		return intentRegion{}, nil
	case b != 1 || e != 1:
		return intentRegion{}, fmt.Errorf("found %d %q and %d %q markers — fathom owns exactly one region; fix the file by hand", b, intentGuardBegin, e, intentGuardEnd)
	}
	bi := strings.Index(content, intentGuardBegin)
	ei := strings.Index(content, intentGuardEnd)
	if ei < bi {
		return intentRegion{}, fmt.Errorf("%q appears before %q — fix the file by hand", intentGuardEnd, intentGuardBegin)
	}
	start := strings.LastIndexByte(content[:bi], '\n') + 1
	nl := strings.IndexByte(content[bi:], '\n')
	if nl < 0 || bi+nl+1 > ei {
		return intentRegion{}, fmt.Errorf("the %q line is malformed — fix the file by hand", intentGuardBegin)
	}
	marker := content[bi : bi+nl]
	bodyStart := bi + nl + 1
	bodyEnd := strings.LastIndexByte(content[:ei], '\n') + 1
	end := len(content)
	if j := strings.IndexByte(content[ei:], '\n'); j >= 0 {
		end = ei + j + 1
	}
	r := intentRegion{present: true, start: start, end: end, body: content[bodyStart:bodyEnd]}
	for _, tok := range strings.Fields(strings.TrimPrefix(marker, intentGuardBegin)) {
		if strings.HasPrefix(tok, "sha256:") {
			r.recordedDigest = tok
		}
	}
	return r, nil
}

// pastedStampRe finds a note carrying fathom's intent provenance stamp.
var pastedStampRe = regexp.MustCompile(`note:\s*"intent:[^"\n]*@ [0-9a-f]{12}"`)

// spliceIntentGuards returns content with the generated region replaced (or
// inserted before the guards list closes, or a new file when content is
// empty). It refuses, never resolves, a conflict (P15): a region a human
// edited, or stamped entries pasted outside the markers.
func spliceIntentGuards(content, routerPath string, entries []intentGuardEntry) (string, error) {
	region := renderIntentGuardRegion(routerPath, entries)
	if strings.TrimSpace(content) == "" {
		return "// .fathom/guard.cue — pre-edit guards (fathom).\n\nguards: [\n" + region + "]\n", nil
	}
	r, err := findIntentRegion(content)
	if err != nil {
		return "", err
	}
	outside := content
	if r.present {
		outside = content[:r.start] + content[r.end:]
	}
	if n := len(pastedStampRe.FindAllString(outside, -1)); n > 0 {
		return "", fmt.Errorf("%d entr(ies) carrying intent provenance (`intent: … @ <stamp>`) sit outside fathom's markers, pasted from an earlier proposal — delete them (the generated region carries them) or drop their @ stamp to keep them as your own, then re-run", n)
	}
	if r.present {
		if r.recordedDigest != "" && r.recordedDigest != intentGuardDigest(r.body) {
			return "", fmt.Errorf("the generated region was edited by hand (its digest no longer matches %s) — move your change outside the markers or into the router, delete the region, then re-run", r.recordedDigest)
		}
		return content[:r.start] + region + content[r.end:], nil
	}
	gi := strings.Index(content, "guards: [")
	if gi < 0 {
		return "", fmt.Errorf("no `guards: [` list in .fathom/guard.cue — add the markers by hand")
	}
	lines := strings.SplitAfter(content[gi:], "\n")
	off := gi
	for _, l := range lines {
		if strings.TrimRight(l, " \t\r\n") == "]" {
			return content[:off] + region + content[off:], nil
		}
		off += len(l)
	}
	return "", fmt.Errorf("could not find the line closing the guards list (a lone `]`) — add the markers by hand")
}

func guardCuePath(root string) string { return projectdir.Guard.Path(root) }

func writeIntentGuards(root, routerPath string, entries []intentGuardEntry, stdout, stderr io.Writer) int {
	path := guardCuePath(root)
	before, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if len(entries) == 0 && len(before) == 0 {
		fmt.Fprintln(stdout, "no sensitive surfaces in the router — nothing to write")
		return 0
	}
	after, err := spliceIntentGuards(string(before), routerPath, entries)
	if err != nil {
		fmt.Fprintf(stderr, "error: .fathom/guard.cue: %v\n", err)
		return 1
	}
	gs, err := guardspec.ParseGuards(path, []byte(after))
	if err != nil {
		fmt.Fprintf(stderr, "error: the regenerated .fathom/guard.cue would not load (%v) — nothing written\n", err)
		return 1
	}
	hand := len(gs.Guards) - len(entries)
	if after == string(before) {
		fmt.Fprintf(stdout, ".fathom/guard.cue: %d intent entries already match %s; %d hand-written entries untouched\n", len(entries), routerPath, hand)
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(after), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	noun := "entries"
	if len(entries) == 1 {
		noun = "entry"
	}
	fmt.Fprintf(stdout, "wrote .fathom/guard.cue: %d intent %s from %s inside fathom's markers; %d hand-written entries untouched\n",
		len(entries), noun, routerPath, hand)
	return 0
}

// checkIntentGuards compares the region on disk with what --write would
// produce and names the first entry that differs.
func checkIntentGuards(root, routerPath string, entries []intentGuardEntry, stdout, stderr io.Writer) int {
	path := guardCuePath(root)
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	content := string(b)
	r, err := findIntentRegion(content)
	if err != nil {
		fmt.Fprintf(stderr, "drift: .fathom/guard.cue: %v\n", err)
		return 1
	}
	fix := "run `fathom intent export --guard --write` in the same commit as the router change"
	if !r.present {
		if len(entries) == 0 {
			fmt.Fprintln(stdout, "no sensitive surfaces and no generated region — nothing to check")
			return 0
		}
		fmt.Fprintf(stderr, "drift: .fathom/guard.cue has no generated intent region; the router names %d surface(s), first [%s] — %s\n",
			len(entries), strings.Join(entries[0].Scopes, ", "), fix)
		return 1
	}
	if r.recordedDigest != "" && r.recordedDigest != intentGuardDigest(r.body) {
		fmt.Fprintf(stderr, "drift: .fathom/guard.cue's generated region was edited by hand — move the change into the router, delete the region, then %s\n", fix)
		return 1
	}
	have, err := guardspec.ParseGuards(path, []byte("guards: [\n"+r.body+"]\n"))
	if err != nil {
		fmt.Fprintf(stderr, "drift: .fathom/guard.cue's generated region does not load (%v) — %s\n", err, fix)
		return 1
	}
	for i := 0; i < len(entries) || i < len(have.Guards); i++ {
		switch {
		case i >= len(have.Guards):
			fmt.Fprintf(stderr, "drift: router surface [%s] has no generated entry — %s\n", strings.Join(entries[i].Scopes, ", "), fix)
			return 1
		case i >= len(entries):
			fmt.Fprintf(stderr, "drift: generated entry [%s] names no router surface any more — %s\n", strings.Join(have.Guards[i].Scopes, ", "), fix)
			return 1
		}
		g, want := have.Guards[i], entries[i]
		if strings.Join(g.Scopes, "\x00") != strings.Join(want.Scopes, "\x00") || g.Note != want.Note || g.MaxRules != intentGuardMaxRules {
			fmt.Fprintf(stderr, "drift: entry [%s] differs from router surface [%s]: have %q (max_rules %d), want %q (max_rules %d) — %s\n",
				strings.Join(g.Scopes, ", "), strings.Join(want.Scopes, ", "), g.Note, g.MaxRules, want.Note, intentGuardMaxRules, fix)
			return 1
		}
	}
	if want := renderIntentGuardRegion(routerPath, entries); content[r.start:r.end] != want {
		fmt.Fprintf(stderr, "drift: .fathom/guard.cue's generated region differs from a fresh render (comments or layout) — %s\n", fix)
		return 1
	}
	fmt.Fprintf(stdout, ".fathom/guard.cue: %d generated intent entries match %s\n", len(entries), routerPath)
	return 0
}

func quoteJoin(in []string) string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, fmt.Sprintf("%q", s))
	}
	return strings.Join(out, ", ")
}

func sortStringsLocal(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
