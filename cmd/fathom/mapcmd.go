package main

// mapcmd.go — the exploration-map verbs (docs/design/exploration-maps.md,
// ADR-056): `map new|append` create a map with its purpose line and file
// entries (`thread note --map` is the same append), `map name|include|compose|
// show|list` shape and read maps (show: mapshow.go). CLI only: no MCP tool is
// added, and the default surface stays at its eight tools (XM-10). Every command
// is deterministic and needs no model (provider none).

import (
	"context"
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

// mapGitTimeout bounds the one git call that recovers a stale entry's filed
// text; past it the diff size is reported unknown, never waited for.
const mapGitTimeout = 5 * time.Second

// mapEntryFlags carries `thread note --map`'s flags.
type mapEntryFlags struct {
	mapID, anchor, why, lead, agent, thread, completes, repo, store, url, service string
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
		idx, idxErr := openMapAnchorIndex(root, f.service, f.store, f.url)
		if idxErr != nil {
			syms = downIndex{err: idxErr}
		} else {
			syms = idx
		}
	}
	res, err := appendMapEntry(mapEntryRequest{
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
	switch it := res.Item; res.Outcome {
	case mapAlready:
		fmt.Fprintf(stdout, "map %q already carries %s (entry %s) with that why: nothing filed\n", f.mapID, res.Entry.SpanRef(), res.Entry.ID)
	case mapAnnotated:
		fmt.Fprintf(stdout, "annotated entry %s on map %q: %s, a second why (row %s)%s\n", res.Entry.ID, f.mapID, it.Pointers["span"], it.ID, by)
	default:
		fmt.Fprintf(stdout, "filed %s on map %q: %s (hash %s)%s\n", it.ID, f.mapID, it.Pointers["span"], it.Pointers["hash"][:12], by)
	}
	return 0
}

// ctxSymbolResolver adapts the plans-context index (a store or the daemon) to
// a symbol anchor's span.
type ctxSymbolResolver struct{ idx ctxIndex }

func (r ctxSymbolResolver) symbolSpan(key string) (string, int, int, bool) {
	if r.idx == nil {
		return "", 0, 0, false
	}
	// The read side resolves a stored key the way an append does (XM-31), so a
	// key filed in any spelling, a Type.Method one included, finds its span.
	if _, ok := r.idx.(anchorIndex); ok {
		m, err := r.resolveAnchor(key)
		if err != nil {
			return "", 0, 0, false
		}
		return m.Node.FilePath, m.Node.Line, anchorEnd(m.Node), true
	}
	pkg, name := splitPointer(key)
	sctx, ok := r.idx.resolveSymbol(pkg, name)
	if !ok || sctx.Def.FilePath == "" || sctx.Def.Line < 1 {
		return "", 0, 0, false
	}
	return sctx.Def.FilePath, sctx.Def.Line, anchorEnd(sctx.Def), true
}

// anchorEnd is the last line of a symbol's span: its end line, or its
// declaration line for an extractor that computes none.
func anchorEnd(n core.SymbolNode) int {
	if n.EndLine < n.Line {
		return n.Line
	}
	return n.EndLine
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

// indexDown reports that a symbol lookup was needed and the index could not be
// opened: a symbol entry was then checked by its filed span only, and a reader
// is told so rather than that the symbol "no longer resolves".
func (s *repoMapSource) indexDown() bool { return s.opened && s.syms == nil }

func (s *repoMapSource) Filed(path, commit string, start, end int) (string, bool) {
	if commit == "" {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), mapGitTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", s.root, "show", commit+":"+path).Output()
	if err != nil {
		return "", false
	}
	return core.LinesText(out, start, end)
}

// ─── fathom map ──────────────────────────────────────────────────────────

func cmdMap(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, mapUsage)
		return 1
	}
	switch args[0] {
	case "new":
		return cmdMapNew(args[1:], stdout, stderr)
	case "append":
		return cmdMapAppend(args[1:], stdout, stderr)
	case "name":
		return cmdMapName(args[1:], stdout, stderr)
	case "include":
		return cmdMapInclude(args[1:], stdout, stderr)
	case "compose":
		return cmdMapCompose(args[1:], stdout, stderr)
	case "show":
		return cmdMapShow(args[1:], stdout, stderr)
	case "list":
		return cmdMapList(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown map verb %q (new|append|name|include|compose|show|list)\n", args[0])
	return 1
}

const mapUsage = "usage: fathom map new|append|name|include|compose|show|list … (an orchestrator: new ID --purpose \"…\", append ID --anchor A --why W, compose ID… --id-only, show ID --budget 1200)"

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

// cmdMapNew is `fathom map new ID --purpose "…" [--when "…"] [--completes-when
// …] [--agent A]`: create a map with the line that says what it is for. The
// purpose is the question the map answers, the one thing fathom cannot compute
// (XM-7), and a reader of a brief or a pointer judges relevance by it, so `new`
// requires it (the implicit creation by a first `append` files none). The map is
// ephemeral: it lives until its condition holds (default: the end of this
// session) and is kept while a live map includes it; `map name` makes it durable
// and the condition no longer ends it (XM-32).
func cmdMapNew(args []string, stdout, stderr io.Writer) int {
	ids, rest := splitLeadIDs(args)
	fs := newFlagSet("map new", stderr)
	purpose := fs.String("purpose", "", "the question the map answers (one line, required)")
	when := fs.String("when", "", "when to use it (one line, optional)")
	completes := fs.String("completes-when", "", `self-closing condition: "file_exists:<path>" or "plan_row_checked:<plan>#<ROW-ID>" (default: the end of this session)`)
	agent := fs.String("agent", "", "the creating sub-agent's id (provenance)")
	thread := fs.String("thread", "", "thread for the map (default: this repo's service)")
	repo := fs.String("repo", ".", "repository")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	ids = append(ids, fs.Args()...)
	if len(ids) != 1 {
		fmt.Fprintln(stderr, `usage: fathom map new ID --purpose "the question it answers" [--when "…"] [--completes-when …] [--agent A]`)
		return 1
	}
	if err := explmap.CheckPurpose(*purpose); err != nil {
		fmt.Fprintf(stderr, "error: map new needs --purpose, the question the map answers, as one line of at most %d bytes: %v\n", explmap.PurposeMaxBytes, err)
		return 1
	}
	cond, err := parseCondition(*completes, *repo)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	root, _ := filepath.Abs(*repo)
	now := time.Now()
	set := loadMapSet(root, now)
	if _, exists := set.maps[ids[0]]; exists {
		fmt.Fprintf(stderr, "error: map %q already exists (`fathom map append %s …` files an entry; `fathom map name %s --purpose …` restates its purpose)\n", ids[0], ids[0], ids[0])
		return 1
	}
	m := explmap.Map{
		ID: ids[0], Kind: explmap.KindEphemeral, Purpose: strings.TrimSpace(*purpose), WhenToUse: strings.TrimSpace(*when),
		CreatedBy: mapCreator(*agent), CreatedAt: now,
	}
	if err := writeMapHeader(set, m, *thread, root, defaultMapCondition(cond), now); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "map %q: %s\n", m.ID, m.Purpose)
	return 0
}

// cmdMapAppend is `fathom map append ID --anchor A --why W [--agent A]`: the
// entry verb, the same append as `thread note --map` (XM-1, XM-14), under the
// noun an orchestrator's brief already uses.
func cmdMapAppend(args []string, stdout, stderr io.Writer) int {
	ids, rest := splitLeadIDs(args)
	fs := newFlagSet("map append", stderr)
	anchor := fs.String("anchor", "", "the entry's anchor, a symbol key or path#Lx-Ly (hashed now, re-resolved at every read)")
	why := fs.String("why", "", "the one-line why this code matters for the map's purpose")
	agent := fs.String("agent", "", "the appending sub-agent's id, recorded as the entry's provenance")
	completes := fs.String("completes-when", "", "condition for a map this append creates (default: the end of this session)")
	thread := fs.String("thread", "", "thread for a map this append creates")
	repo := fs.String("repo", ".", "repository")
	storePath := fs.String("store", "", "resolve a symbol anchor from this store (default: the daemon)")
	url := fs.String("url", defaultFathomURL(), "fathom daemon URL for a symbol anchor")
	service := fs.String("service", "", "the service a symbol anchor resolves in (default: the one the daemon names for this repository; required with --store when it names none)")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	ids = append(ids, fs.Args()...)
	if len(ids) != 1 || strings.TrimSpace(*anchor) == "" {
		fmt.Fprintln(stderr, `usage: fathom map append ID --anchor <symbol|path#Lx-Ly> --why "…" [--agent A] [--completes-when …]`)
		return 1
	}
	return cmdThreadNoteMap(mapEntryFlags{
		mapID: ids[0], anchor: *anchor, why: *why, agent: *agent,
		thread: *thread, completes: *completes, repo: *repo, store: *storePath, url: *url, service: *service,
	}, stdout, stderr)
}

// cmdMapName is `fathom map name ID --purpose "…" [--as NAME] [--when "…"]`:
// a named map is durable, whatever condition it was filed under and whether the
// name comes before or after that condition held (XM-32, O143), and a named map
// without its purpose line is refused (XM-7, explmap.Validate).
func cmdMapName(args []string, stdout, stderr io.Writer) int {
	ids, rest := splitLeadIDs(args)
	fs := newFlagSet("map name", stderr)
	purpose := fs.String("purpose", "", "the question the map answers (one line, required)")
	as := fs.String("as", "", "the name (default: the name it has, else the map id)")
	when := fs.String("when", "", "when to use it (one line, optional; default: the line it has)")
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
	// The header is restated whole: a flag left out keeps what the map already
	// carries (a name set with --as, a when-to-use line).
	if name := strings.TrimSpace(*as); name != "" {
		m.Name = name
	} else if !m.Named() {
		m.Name = ids[0]
	}
	m.Purpose = *purpose
	if strings.TrimSpace(*when) != "" {
		m.WhenToUse = *when
	}
	if err := writeMapHeader(set, m, *thread, root, nil, now); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "named map %q %q: %s (durable: it is kept past the session that filed it)\n", m.ID, m.Name, strings.TrimSpace(m.Purpose))
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
	service := fs.String("service", "", "the service symbol anchors resolve in (default: the one the daemon names for this repository)")
	jsonOut := fs.Bool("json", false, "emit the composite id and its expansion as JSON")
	idOnly := fs.Bool("id-only", false, "print only the id (one line), for a brief or a script to capture")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	ids = append(ids, fs.Args()...)
	if len(ids) == 0 {
		fmt.Fprintln(stderr, "usage: fathom map compose ID… [--agent A] [--id-only] [--json] [--store S --service X]")
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
	if *idOnly {
		// The id is all a brief carries; nothing is expanded, so no index is opened.
		fmt.Fprintln(stdout, id)
		return 0
	}
	x, err := explmap.Expand(id, set.maps, newRepoMapSource(root, func() mapSymbolResolver {
		return openMapSymbols(root, *storePath, *url, *service)
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
func mapSourceSymbols(root, storePath, url, service string) mapSymbolResolver {
	if service == "" {
		service = mapServiceOf(root)
	}
	idx, err := mapOpenIndex(service, storePath, url)
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

// mapRunner runs `fathom map` (registered below; EW-45).
type mapRunner struct{}

func (mapRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdMap(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "map", Group: "govern", Order: 20, Run: mapRunner{},
		Summary: "Exploration maps: new | append | name | include | compose | show | list — anchored entries, composed by reference, one id per handoff, re-resolved at every read (show ID --budget 1200)"})
}
