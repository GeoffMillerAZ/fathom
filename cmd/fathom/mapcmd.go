package main

// mapcmd.go — the exploration-map verbs (docs/design/exploration-maps.md,
// ADR-056): `thread note --map` files an entry, `map name|include|compose|list`
// shape and inspect maps. CLI only: no MCP tool is added, and the default
// surface stays at its eight tools (XM-10). Every command is deterministic and
// needs no model (provider none).

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/explmap"
)

// mapEntryFlags carries `thread note --map`'s flags.
type mapEntryFlags struct {
	mapID, anchor, why, lead, agent, thread, completes, repo, store, url string
}

// cmdThreadNoteMap is `fathom thread note --map ID --anchor A --why W`.
func cmdThreadNoteMap(f mapEntryFlags, stdout, stderr io.Writer) int {
	why := f.why
	if strings.TrimSpace(why) == "" {
		why = f.lead
	}
	if strings.TrimSpace(f.anchor) == "" {
		fmt.Fprintln(stderr, "usage: fathom thread note --map ID --anchor <symbol|path#Lx-Ly> --why \"…\" [--agent ID] [--completes-when …]")
		return 1
	}
	cond, err := parseCondition(f.completes, f.repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	root, _ := filepath.Abs(f.repo)
	var syms mapSymbolResolver
	if _, _, _, symbol, ok := explmap.ParseAnchor(f.anchor); ok && symbol != "" {
		idx, idxErr := openCtxIndex(threadScopeFor(root).service, f.store, f.url)
		if idxErr == nil {
			syms = ctxSymbolResolver{idx: idx}
		}
	}
	it, err := appendMapEntry(mapEntryRequest{
		Map: f.mapID, Anchor: f.anchor, Why: why, AgentID: f.agent,
		Thread: f.thread, Root: root, Completes: cond, Now: time.Now(),
	}, syms)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	by := ""
	if f.agent != "" {
		by = " by " + f.agent
	}
	fmt.Fprintf(stdout, "filed %s on map %q: %s (hash %s)%s\n", it.ID, f.mapID, it.Pointers["span"], it.Pointers["hash"][:12], by)
	return 0
}

// ctxSymbolResolver adapts the plans-context index (a store or the daemon) to
// a symbol anchor's span.
type ctxSymbolResolver struct{ idx ctxIndex }

func (r ctxSymbolResolver) symbolSpan(key string) (string, int, int, bool) {
	if r.idx == nil {
		return "", 0, 0, false
	}
	pkg, name := splitPointer(key)
	sctx, ok := r.idx.resolveSymbol(pkg, name)
	if !ok || sctx.Def.FilePath == "" || sctx.Def.Line < 1 {
		return "", 0, 0, false
	}
	end := sctx.Def.EndLine
	if end < sctx.Def.Line {
		end = sctx.Def.Line // an extractor that computes no end line: the declaration line
	}
	return sctx.Def.FilePath, sctx.Def.Line, end, true
}

// repoMapSource is explmap.Source over a repository: its files now, the index
// for symbol spans (opened on the first symbol a drifted entry needs, so a map
// whose spans all still hash never probes the daemon), and git for the text at
// filing.
type repoMapSource struct {
	root     string
	openSyms func() mapSymbolResolver
	syms     mapSymbolResolver
	opened   bool
	files    map[string][]byte
	gone     map[string]bool
}

func newRepoMapSource(root string, openSyms func() mapSymbolResolver) *repoMapSource {
	return &repoMapSource{root: root, openSyms: openSyms, files: map[string][]byte{}, gone: map[string]bool{}}
}

func (s *repoMapSource) Lines(path string, start, end int) (string, bool) {
	c, ok := s.files[path]
	if !ok && !s.gone[path] {
		data, err := os.ReadFile(filepath.Join(s.root, filepath.FromSlash(path)))
		if err != nil {
			s.gone[path] = true
		} else {
			s.files[path], c, ok = data, data, true
		}
	}
	if !ok {
		return "", false
	}
	return core.LinesText(c, start, end)
}

func (s *repoMapSource) Symbol(key string) (string, int, int, bool) {
	if !s.opened && s.openSyms != nil {
		s.syms, s.opened = s.openSyms(), true
	}
	if s.syms == nil {
		return "", 0, 0, false
	}
	return s.syms.symbolSpan(key)
}

func (s *repoMapSource) Filed(path, commit string, start, end int) (string, bool) {
	if commit == "" {
		return "", false
	}
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	out, err := exec.Command("git", "-C", s.root, "show", commit+":"+path).Output()
	if err != nil {
		return "", false
	}
	return core.LinesText(out, start, end)
}

// ─── fathom map ──────────────────────────────────────────────────────────

func cmdMap(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: fathom map name|include|compose|list … (entries: fathom thread note --map ID --anchor A --why W)")
		return 1
	}
	switch args[0] {
	case "name":
		return cmdMapName(args[1:], stdout, stderr)
	case "include":
		return cmdMapInclude(args[1:], stdout, stderr)
	case "compose":
		return cmdMapCompose(args[1:], stdout, stderr)
	case "list":
		return cmdMapList(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown map verb %q (name|include|compose|list)\n", args[0])
	return 1
}

// splitLeadIDs lifts the leading positional ids off args, so `map include A B
// --drop C` parses (Go's flag package stops at the first positional).
func splitLeadIDs(args []string) (ids, rest []string) {
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			return ids, args[i:]
		}
		ids = append(ids, a)
	}
	return ids, nil
}

// cmdMapName is `fathom map name ID --purpose "…" [--as NAME] [--when "…"]`:
// naming a map makes it durable, and a named map without its purpose line is
// refused (XM-7, explmap.Validate).
func cmdMapName(args []string, stdout, stderr io.Writer) int {
	ids, rest := splitLeadIDs(args)
	fs := newFlagSet("map name", stderr)
	purpose := fs.String("purpose", "", "the question the map answers (one line, required)")
	as := fs.String("as", "", "the name (default: the map id)")
	when := fs.String("when", "", "when to use it (one line, optional)")
	repo := fs.String("repo", ".", "repository")
	thread := fs.String("thread", "", "thread for a map created here (default: this repo's service)")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	ids = append(ids, fs.Args()...)
	if len(ids) != 1 {
		fmt.Fprintln(stderr, "usage: fathom map name ID --purpose \"the question it answers\" [--as NAME] [--when \"…\"]")
		return 1
	}
	root, _ := filepath.Abs(*repo)
	now := time.Now()
	set := loadMapSet(root, now)
	m, exists := set.maps[ids[0]]
	if !exists {
		m = explmap.Map{ID: ids[0], Kind: explmap.KindEphemeral, CreatedBy: mapCreator(""), CreatedAt: now}
	}
	m.Name = strings.TrimSpace(*as)
	if m.Name == "" {
		m.Name = ids[0]
	}
	m.Purpose, m.WhenToUse = *purpose, *when
	if err := writeMapHeader(set, m, *thread, root, nil, now); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "named map %q %q: %s\n", m.ID, m.Name, strings.TrimSpace(m.Purpose))
	return 0
}

// cmdMapInclude is `fathom map include ID BASE… [--drop BASE…]`: includes form a
// DAG, so a cycle or a chain past the depth cap is refused before it is stored
// (XM-2); a dropped include restarts the base's idle clock (XM-6).
func cmdMapInclude(args []string, stdout, stderr io.Writer) int {
	ids, rest := splitLeadIDs(args)
	fs := newFlagSet("map include", stderr)
	drop := fs.String("drop", "", "space- or comma-separated includes to drop")
	repo := fs.String("repo", ".", "repository")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	ids = append(ids, fs.Args()...)
	drops := strings.FieldsFunc(*drop, func(r rune) bool { return r == ',' || r == ' ' })
	if len(ids) < 1 || (len(ids) < 2 && len(drops) == 0) {
		fmt.Fprintln(stderr, "usage: fathom map include ID BASE… [--drop BASE…]")
		return 1
	}
	root, _ := filepath.Abs(*repo)
	now := time.Now()
	set := loadMapSet(root, now)
	m, ok := set.maps[ids[0]]
	if !ok {
		fmt.Fprintf(stderr, "error: unknown map %q (file an entry with `fathom thread note --map %s …` or name it first)\n", ids[0], ids[0])
		return 1
	}
	if m.Kind == explmap.KindComposite {
		fmt.Fprintf(stderr, "error: map %q is a minted composite; its references are fixed — compose a new one\n", m.ID)
		return 1
	}
	var next []string
	for _, inc := range m.Includes {
		if !sliceContains(drops, inc) {
			next = append(next, inc)
		}
	}
	for _, base := range ids[1:] {
		if _, known := set.maps[base]; !known {
			fmt.Fprintf(stderr, "error: unknown map %q: a map includes only maps that exist\n", base)
			return 1
		}
		if !sliceContains(next, base) {
			next = append(next, base)
		}
	}
	m.Includes = next
	res, err := explmap.ResolveMap(m.ID, explmap.With(set.maps, m))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := writeMapHeader(set, m, "", root, nil, now); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "map %q includes %s (%d maps reachable, %d levels deep)\n", m.ID, strings.Join(orNone(m.Includes), " "), len(res.Nodes), res.Height)
	return 0
}

// cmdMapCompose is `fathom map compose ID… [--agent A]`: the ONE id a handoff
// carries (XM-3). Several maps become a minted composite made only of
// references; the command then expands it once, re-resolving every entry, and
// reports what the id will hand down (XM-4, XM-5).
func cmdMapCompose(args []string, stdout, stderr io.Writer) int {
	ids, rest := splitLeadIDs(args)
	fs := newFlagSet("map compose", stderr)
	agent := fs.String("agent", "", "the composing sub-agent's id (provenance)")
	repo := fs.String("repo", ".", "repository")
	storePath := fs.String("store", "", "resolve symbol anchors from this store (default: the daemon)")
	url := fs.String("url", defaultFathomURL(), "fathom daemon URL")
	jsonOut := fs.Bool("json", false, "emit the composite id and its expansion as JSON")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	ids = append(ids, fs.Args()...)
	if len(ids) == 0 {
		fmt.Fprintln(stderr, "usage: fathom map compose ID… [--agent A]")
		return 1
	}
	root, _ := filepath.Abs(*repo)
	now := time.Now()
	set := loadMapSet(root, now)
	id, minted, err := explmap.Compose(ids, set.maps, mapCreator(*agent), now)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if minted != nil {
		if _, stored := set.maps[id]; !stored {
			if err := writeMapHeader(set, *minted, "", root, nil, now); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			set.maps[id] = *minted
		}
	}
	x, err := explmap.Expand(id, set.maps, newRepoMapSource(root, func() mapSymbolResolver {
		return mapSourceSymbols(root, *storePath, *url)
	}))
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return jsonExit(enc.Encode(map[string]any{"id": id, "minted": minted != nil, "expansion": x}), stderr)
	}
	verb := "minted"
	if minted == nil {
		verb = "one map, already one id:"
	}
	fmt.Fprintf(stdout, "%s %s — hand this one id down\n", verb, id)
	fmt.Fprintf(stdout, "  %d maps, %d entries: %d fresh, %d re-resolved, %d stale; %d with conflicting annotations\n",
		x.Maps, len(x.Items), x.Fresh, x.ReResolved, x.Stale, x.Conflicts)
	if len(x.Missing) > 0 {
		fmt.Fprintf(stdout, "  missing includes: %s\n", strings.Join(x.Missing, " "))
	}
	return 0
}

// cmdMapList is `fathom map list [--all] [--json]`: this repo's maps with their
// lifetime under the reference rule (XM-6). Ended maps are counted, and listed
// with --all.
func cmdMapList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("map list", stderr)
	all := fs.Bool("all", false, "include ended maps")
	repo := fs.String("repo", ".", "repository")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, _ := filepath.Abs(*repo)
	set := loadMapSet(root, time.Now())
	lifetimes := explmap.Lifetimes(set.maps)
	type row struct {
		ID        string   `json:"id"`
		Kind      string   `json:"kind"`
		Name      string   `json:"name,omitempty"`
		Purpose   string   `json:"purpose,omitempty"`
		Entries   int      `json:"entries"`
		Includes  []string `json:"includes,omitempty"`
		Lifetime  string   `json:"lifetime"`
		PinnedBy  []string `json:"pinned_by,omitempty"`
		IdleSince string   `json:"idle_since,omitempty"`
	}
	rows := []row{}
	hidden := 0
	for _, id := range set.idsOwnedBy(threadScopeFor(root)) {
		m, lt := set.maps[id], lifetimes[id]
		if lt.State == explmap.Ended && !*all {
			hidden++
			continue
		}
		r := row{ID: id, Kind: string(m.Kind), Name: m.Name, Purpose: m.Purpose, Entries: len(m.Entries),
			Includes: m.Includes, Lifetime: string(lt.State), PinnedBy: lt.PinnedBy}
		if !lt.IdleSince.IsZero() {
			r.IdleSince = lt.IdleSince.UTC().Format(time.RFC3339)
		}
		rows = append(rows, r)
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return jsonExit(enc.Encode(map[string]any{"maps": rows, "ended_hidden": hidden}), stderr)
	}
	if len(rows) == 0 {
		fmt.Fprintf(stdout, "no maps (%d ended hidden; --all shows them) — `fathom thread note --map ID --anchor A --why W` files the first entry\n", hidden)
		return 0
	}
	for _, r := range rows {
		label := r.ID
		if r.Name != "" && r.Name != r.ID {
			label += " (" + r.Name + ")"
		}
		life := r.Lifetime
		switch {
		case len(r.PinnedBy) > 0:
			life += " by " + strings.Join(r.PinnedBy, " ")
		case r.IdleSince != "":
			life += ", idle since " + r.IdleSince
		}
		fmt.Fprintf(stdout, "%s  %s, %d entries, includes %s — %s\n", label, r.Kind, r.Entries, strings.Join(orNone(r.Includes), " "), life)
		if r.Purpose != "" {
			fmt.Fprintf(stdout, "    %s\n", r.Purpose)
		}
	}
	if hidden > 0 {
		fmt.Fprintf(stdout, "(%d ended hidden; --all shows them)\n", hidden)
	}
	return 0
}

// mapSourceSymbols opens the index for symbol re-resolution; nil (symbol
// entries then re-resolve only by their filed span) when it is unreachable.
func mapSourceSymbols(root, storePath, url string) mapSymbolResolver {
	idx, err := openCtxIndex(threadScopeFor(root).service, storePath, url)
	if err != nil {
		return nil
	}
	return ctxSymbolResolver{idx: idx}
}

// mapCreator is the provenance a new map records: the agent id when given,
// else the session, else who filed it.
func mapCreator(agent string) string {
	if agent != "" {
		return agent
	}
	if sid := currentSessionID(); sid != "" {
		return sid
	}
	return threadFiler()
}

func orNone(s []string) []string {
	if len(s) == 0 {
		return []string{"none"}
	}
	return s
}
