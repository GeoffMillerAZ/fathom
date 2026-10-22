package main

// mapshow.go — `fathom map show ID [--budget N]` (XM-9, docs/design/
// exploration-maps.md section 2, ADR-056): the read side of a map. It expands
// the map's include DAG once (a base reached by two routes is one set of
// entries), re-resolves every entry against the code as it is now (fresh, moved
// and found again by its symbol key, or stale with its diff size: never served
// as if it were still true), and prints the result within a byte budget, naming
// what it cut. Deterministic, read-only, no model: the same map over the same
// tree prints the same bytes.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/internal/explmap"
)

// mapShowDefaultBudget is `map show`'s default cap in bytes: RC-28's handoff
// cap (docs/vision/28-receipts.md), the size a map's read is designed to fit.
const mapShowDefaultBudget = 1200

// mapShowWhyMax bounds one why line in the text rendering (the JSON carries it
// whole): a why is one line, and one long line must not eat the budget.
const mapShowWhyMax = 160

// mapShowNameMax bounds the name printed in the header.
const mapShowNameMax = 64

// mapShowIndexDownNote is what a read says when a drifted symbol entry needed
// the index and it could not be reached.
const mapShowIndexDownNote = "the index is unreachable, so symbol entries were checked by their filed span only"

// openMapSymbols opens the symbol lookup a drifted symbol entry is re-resolved
// by, for the service named (empty: the one the daemon names for root); a
// variable so a test can supply a table instead of the index.
var openMapSymbols = mapSourceSymbols

// mapShowAnnotation is one why line with its provenance.
type mapShowAnnotation struct {
	Why     string   `json:"why"`
	Map     string   `json:"map"`
	AgentID string   `json:"agent_id,omitempty"`
	Also    []string `json:"also_in,omitempty"`
}

// mapShowItem is one entry of an expansion, as `map show` reports it.
type mapShowItem struct {
	// Anchor is where the entry's code is now (`path#Lx-Ly`); Filed is the
	// span it was filed at, when that differs.
	Anchor      string              `json:"anchor"`
	Filed       string              `json:"filed,omitempty"`
	Symbol      string              `json:"symbol,omitempty"`
	Why         string              `json:"why"`
	AgentID     string              `json:"agent_id,omitempty"`
	State       string              `json:"state"`
	Added       int                 `json:"added,omitempty"`
	Removed     int                 `json:"removed,omitempty"`
	DiffKnown   bool                `json:"diff_known,omitempty"`
	Note        string              `json:"note,omitempty"`
	Maps        []string            `json:"maps"`
	Annotations []mapShowAnnotation `json:"annotations"`
	// OutsideFocus marks an entry whose file sits under no --focus path
	// (XM-35); never set without a focus.
	OutsideFocus bool `json:"outside_focus,omitempty"`
}

// mapShowCut is an entry the budget left out.
type mapShowCut struct {
	Anchor string `json:"anchor"`
	State  string `json:"state"`
}

// mapShowCounts are the numbers over the whole map, shown or cut.
type mapShowCounts struct {
	Entries    int `json:"entries"`
	Maps       int `json:"maps"`
	Fresh      int `json:"fresh"`
	ReResolved int `json:"re_resolved"`
	Stale      int `json:"stale"`
	Conflicts  int `json:"conflicts"`
}

// mapShowView is `map show`'s answer: the text a reader gets, and the same
// facts as data.
type mapShowView struct {
	ID        string        `json:"id"`
	Kind      string        `json:"kind"`
	Name      string        `json:"name,omitempty"`
	Purpose   string        `json:"purpose,omitempty"`
	WhenToUse string        `json:"when_to_use,omitempty"`
	Includes  []string      `json:"includes,omitempty"`
	Budget    int           `json:"budget"`
	Bytes     int           `json:"bytes"`
	Counts    mapShowCounts `json:"counts"`
	Missing   []string      `json:"missing,omitempty"`
	Notes     []string      `json:"notes,omitempty"`
	Items     []mapShowItem `json:"items"`
	Cut       []mapShowCut  `json:"cut"`
	Text      string        `json:"-"`

	// Focus is the --focus paths, repo-relative and sorted (XM-35), and
	// InFocus how many of the map's entries sit under one of them.
	Focus   []string `json:"focus,omitempty"`
	InFocus int      `json:"in_focus,omitempty"`
}

// cmdMapShow is `fathom map show ID [--budget N] [--json]`.
func cmdMapShow(args []string, stdout, stderr io.Writer) int {
	ids, rest := splitLeadIDs(args)
	fs := newFlagSet("map show", stderr)
	budget := fs.Int("budget", mapShowDefaultBudget, "byte budget for the output; the entries that do not fit are named, not shown (0: no cap)")
	repo := fs.String("repo", ".", "repository the anchors resolve in")
	storePath := fs.String("store", "", "resolve symbol anchors from this store (default: the daemon)")
	url := fs.String("url", defaultFathomURL(), "fathom daemon URL")
	service := fs.String("service", "", "the service symbol anchors resolve in (default: the one the daemon names for this repository; needed with --store when it names none)")
	brief := fs.Bool("brief", false, "print at most 1500 bytes of anchored excerpts and why lines for an orchestrator brief")
	jsonOut := fs.Bool("json", false, "emit JSON: the entries shown, the entries cut, the counts (the budget picks the entries; the JSON itself is not capped)")
	focus := fs.String("focus", "", "comma-separated paths or package directories your task touches: the entries under them print first and every other entry is marked outside focus, none hidden")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	ids = append(ids, fs.Args()...)
	if len(ids) != 1 {
		fmt.Fprintln(stderr, "usage: fathom map show ID [--budget 1200] [--focus path,pkg/dir] [--json] [--repo .] [--store S --service X]")
		return 1
	}
	if *budget < 0 {
		fmt.Fprintf(stderr, "error: --budget %d: a byte budget is 0 (no cap) or more\n", *budget)
		return 1
	}
	root, _ := filepath.Abs(*repo)
	focusPaths := mapFocusPaths(root, *focus)
	set := loadMapSet(root, time.Now())
	m, ok := set.maps[ids[0]]
	if !ok {
		fmt.Fprintf(stderr, "error: %v %q (`fathom map list` shows this repo's maps)\n", explmap.ErrUnknownMap, ids[0])
		return 1
	}
	if *brief {
		cap := subagentMapBriefBudget
		explicitBudget := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "budget" {
				explicitBudget = true
			}
		})
		if explicitBudget && *budget > 0 && *budget < cap {
			cap = *budget
		}
		fmt.Fprint(stdout, renderMapBrief(set, []string{m.ID}, root, cap))
		return 0
	}
	src := newRepoMapSource(root, func() mapSymbolResolver {
		return openMapSymbols(root, *storePath, *url, *service)
	})
	x, err := explmap.Expand(m.ID, set.maps, src)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	var notes []string
	if src.indexDown() {
		notes = append(notes, mapShowIndexDownNote)
	}
	v := buildMapShow(m, x, notes, *budget, focusPaths)
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return jsonExit(enc.Encode(v), stderr)
	}
	fmt.Fprint(stdout, v.Text)
	return 0
}

// buildMapShow renders m's expansion within budget bytes (0: no cap): the
// header, the counts over the whole map, the entries nearest first, and, when
// some do not fit, one footer naming them. The output never exceeds the budget.
// With focus paths (XM-35), the entries under one of them lead, in the same
// order, and the rest follow marked outside focus: a stable reorder of the
// entries already expanded, so the focus reads nothing new.
func buildMapShow(m explmap.Map, x explmap.Expansion, notes []string, budget int, focus []string) mapShowView {
	v := mapShowView{
		ID: m.ID, Kind: string(m.Kind), Name: m.Name, Purpose: strings.TrimSpace(m.Purpose),
		WhenToUse: strings.TrimSpace(m.WhenToUse), Includes: m.Includes, Budget: budget, Missing: x.Missing, Notes: notes,
		Counts: mapShowCounts{
			Entries: len(x.Items), Maps: x.Maps, Fresh: x.Fresh, ReResolved: x.ReResolved, Stale: x.Stale, Conflicts: x.Conflicts,
		},
		Focus: focus,
	}
	all := make([]mapShowItem, 0, len(x.Items))
	var outside []mapShowItem
	for _, it := range x.Items {
		si := showItem(it)
		if len(focus) > 0 && !underMapFocus(si.Anchor, focus) {
			si.OutsideFocus = true
			outside = append(outside, si)
			continue
		}
		all = append(all, si)
	}
	if len(focus) > 0 {
		v.InFocus = len(all)
	}
	all = append(all, outside...)
	blocks := make([]string, len(all))
	for i := range all {
		blocks[i] = showBlock(all[i])
	}

	head := showHeader(v)
	total := len(head)
	for _, b := range blocks {
		total += len(b)
	}
	shown := len(all)
	footer := ""
	if budget > 0 && total > budget {
		shown, footer = fitCut(head, blocks, all, budget)
	}
	var out strings.Builder
	out.WriteString(head)
	for _, b := range blocks[:shown] {
		out.WriteString(b)
	}
	out.WriteString(footer)
	text := out.String()
	if budget > 0 && len(text) > budget {
		// A budget below the header itself: the cap still holds, at the price
		// of the header's tail.
		text = truncateUTF8(text, budget)
	}
	v.Text = text
	v.Bytes = len(text)
	v.Items = all[:shown]
	v.Cut = []mapShowCut{}
	for _, it := range all[shown:] {
		v.Cut = append(v.Cut, mapShowCut{Anchor: it.Anchor, State: it.State})
	}
	return v
}

// showItem flattens one expanded item.
func showItem(it explmap.Item) mapShowItem {
	f := it.Fresh
	si := mapShowItem{
		Anchor:  f.Anchor,
		Symbol:  it.Entry.Symbol,
		Why:     it.Annotations[0].Why,
		AgentID: it.Annotations[0].AgentID,
		State:   string(f.State),
		Note:    f.Note,
		Maps:    it.Maps,
	}
	if f.Anchor == "" {
		si.Anchor = it.Entry.SpanRef()
	}
	if filed := it.Entry.SpanRef(); filed != si.Anchor {
		si.Filed = filed
	}
	if f.State == explmap.Stale && f.DiffKnown {
		si.Added, si.Removed, si.DiffKnown = f.Added, f.Removed, true
	}
	for _, a := range it.Annotations {
		si.Annotations = append(si.Annotations, mapShowAnnotation{Why: a.Why, Map: a.MapID, AgentID: a.AgentID, Also: a.Also})
	}
	return si
}

// showHeader is the header and the counts over the whole map.
func showHeader(v mapShowView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "map %s", v.ID)
	if v.Name != "" && v.Name != v.ID {
		fmt.Fprintf(&b, " %q", clipRunes(v.Name, mapShowNameMax))
	}
	switch {
	case v.Purpose != "":
		fmt.Fprintf(&b, " — %s", v.Purpose)
	case v.Kind == string(explmap.KindComposite):
		fmt.Fprintf(&b, " — composite of %s", strings.Join(v.Includes, ", "))
	}
	b.WriteString("\n")
	if v.WhenToUse != "" {
		fmt.Fprintf(&b, "use when: %s\n", v.WhenToUse)
	}
	c := v.Counts
	fmt.Fprintf(&b, "%d %s from %d %s: %d fresh, %d re-resolved, %d stale", c.Entries, mapPlural(c.Entries, "entry", "entries"),
		c.Maps, mapPlural(c.Maps, "map", "maps"), c.Fresh, c.ReResolved, c.Stale)
	if c.Conflicts > 0 {
		fmt.Fprintf(&b, "; %d with conflicting annotations", c.Conflicts)
	}
	if n := len(v.Missing); n > 0 {
		names := v.Missing
		more := ""
		if n > 3 {
			names, more = names[:3], fmt.Sprintf(" +%d more", n-3)
		}
		fmt.Fprintf(&b, "; missing includes: %s%s", strings.Join(names, " "), more)
	}
	b.WriteString("\n")
	if len(v.Focus) > 0 {
		fmt.Fprintf(&b, "focus %s: %d of %d %s inside, listed first\n", strings.Join(v.Focus, ", "), v.InFocus, c.Entries, mapPlural(c.Entries, "entry", "entries"))
	}
	for _, n := range v.Notes {
		fmt.Fprintf(&b, "note: %s\n", n)
	}
	return b.String()
}

// showBlock is one entry's lines: the entry, then one `also` line per other
// annotation, each kept with its provenance.
func showBlock(it mapShowItem) string {
	var b strings.Builder
	b.WriteString(it.Anchor)
	if it.Symbol != "" {
		b.WriteString(" " + it.Symbol)
	}
	fmt.Fprintf(&b, " — %s", clipRunes(it.Why, mapShowWhyMax))
	if it.AgentID != "" {
		fmt.Fprintf(&b, " (%s)", it.AgentID)
	}
	state := showState(it)
	if it.OutsideFocus {
		state += ", outside focus"
	}
	fmt.Fprintf(&b, " [%s]\n", state)
	for _, a := range it.Annotations[1:] {
		fmt.Fprintf(&b, "  also: %s (map %s", clipRunes(a.Why, mapShowWhyMax), a.Map)
		if a.AgentID != "" {
			fmt.Fprintf(&b, " by %s", a.AgentID)
		}
		b.WriteString(")\n")
	}
	return b.String()
}

// mapFocusPaths parses --focus: comma-separated paths or package directories,
// absolute ones made relative to root, cleaned, deduplicated and sorted, so the
// same focus spelled two ways prints the same bytes. A path outside root, or
// one naming the root itself, focuses nothing and is dropped.
func mapFocusPaths(root, spec string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range strings.Split(spec, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if filepath.IsAbs(f) {
			rel, err := filepath.Rel(root, f)
			if err != nil {
				continue
			}
			f = rel
		}
		f = path.Clean(filepath.ToSlash(f))
		if f == "." || f == ".." || strings.HasPrefix(f, "../") || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// underMapFocus reports whether an anchor's file is a focus path or sits in a
// focus directory (whole path segments: internal/y is not internal/yy).
func underMapFocus(anchor string, focus []string) bool {
	p, _, _, _, ok := explmap.ParseAnchor(anchor)
	if !ok || p == "" {
		return false
	}
	for _, f := range focus {
		if p == f || strings.HasPrefix(p, f+"/") {
			return true
		}
	}
	return false
}

// showState is the bracketed freshness: what re-resolving found.
func showState(it mapShowItem) string {
	switch explmap.State(it.State) {
	case explmap.Fresh:
		return "fresh"
	case explmap.ReResolved:
		return "moved from " + it.Filed
	}
	if it.DiffKnown {
		return fmt.Sprintf("STALE +%d/-%d lines since filing: %s", it.Added, it.Removed, it.Note)
	}
	return "STALE: " + it.Note
}

// fitCut chooses how many entries (a prefix, nearest first) fit beside the
// header and a footer naming the rest, and renders the footer: how many were
// cut of how many, how many of them are stale, and their names as far as the
// budget allows, the remainder counted.
func fitCut(head string, blocks []string, all []mapShowItem, budget int) (shown int, footer string) {
	n := len(all)
	used := len(head)
	prefix := make([]int, n+1) // prefix[k] = bytes of the head plus the first k blocks
	prefix[0] = used
	for i, b := range blocks {
		prefix[i+1] = prefix[i] + len(b)
	}
	stale := make([]int, n+1) // stale[k] = stale entries among the first k
	for i, it := range all {
		stale[i+1] = stale[i]
		if it.State == string(explmap.Stale) {
			stale[i+1]++
		}
	}
	base := func(k int) string {
		s := fmt.Sprintf("cut %d of %d entries to stay within %d bytes", n-k, n, budget)
		if cutStale := stale[n] - stale[k]; cutStale > 0 {
			s += fmt.Sprintf(" (%d stale)", cutStale)
		}
		return s
	}
	shown = 0
	for k := n - 1; k >= 0; k-- {
		if prefix[k]+len(base(k))+1 <= budget {
			shown = k
			break
		}
	}
	line := base(shown)
	room := budget - prefix[shown] - 1 // the newline
	var names []string
	for _, it := range all[shown:] {
		nm := it.Anchor
		if it.State == string(explmap.Stale) {
			nm += " (stale)"
		}
		names = append(names, nm)
	}
	best := line
	for j := 1; j <= len(names); j++ {
		cand := line + ": " + strings.Join(names[:j], ", ")
		if rest := len(names) - j; rest > 0 {
			cand += fmt.Sprintf(" +%d more", rest)
		}
		if len(cand) > room {
			break
		}
		best = cand
	}
	return shown, best + "\n"
}

func mapPlural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// clipRunes cuts s to at most max bytes on a rune boundary, marking the cut.
func clipRunes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return truncateUTF8(s, max-len("…")) + "…"
}

// truncateUTF8 cuts s to at most max bytes without splitting a rune.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 0 {
		max = 0
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}
