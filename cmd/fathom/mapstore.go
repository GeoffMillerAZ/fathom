package main

// mapstore.go — exploration maps stored as thread fields (XM-1, XM-14;
// docs/design/exploration-maps.md section 2, ADR-056). No new table, cache or
// background job (R5, ADR-049 decision 2): a map is rows in the existing
// threads log. Two kinds of row, both invisible to the thread verbs (they are
// the maps' own, and `thread brief` and `thread list` never serve them):
//
//   - a header ("map", id "map:<map id>"): kind, name, purpose, when-to-use,
//     includes, creator, and the map's `completes_when`. Every change
//     restates the header, latest row winning, so its history stays in the
//     log (an include dropped is read back from it: XM-6's clock);
//   - an entry ("map-entry", a freshly minted id): anchor, span, content
//     hash, why line, provenance. An append always mints a new id and never
//     rewrites a row, so no agent can edit another's entry (XM-14).
//
// The model (resolution, composites, freshness, lifetimes, validation) is
// internal/explmap; this file only reads and writes rows.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/explmap"
)

const (
	mapHeaderKind = "map"
	mapEntryKind  = "map-entry"
	// mapHeaderIDPrefix makes a header's row id stable per map, so each
	// restatement supersedes the last under the log's latest-row rule.
	mapHeaderIDPrefix = "map:"
)

// isMapItem reports whether a thread row is an exploration map's own.
func isMapItem(it threadItem) bool {
	return it.Kind == mapHeaderKind || it.Kind == mapEntryKind
}

// loadThreadRowsRaw reads every row of the log in file order, unfolded: the
// header history an include drop is read from.
func loadThreadRowsRaw(path string) []threadItem {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []threadItem
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var it threadItem
		if json.Unmarshal([]byte(line), &it) != nil || it.ID == "" {
			continue
		}
		out = append(out, it)
	}
	return out
}

// mapSet is every map in the log, as the model sees them, plus the header
// rows they were read from (for restating a header on change).
type mapSet struct {
	maps    explmap.Maps
	headers map[string]threadItem
}

func parseRFC3339(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// loadMapSet folds the log into maps. A header whose `completes_when` now
// holds is closed by the thread machinery's own read-time closure (a durable,
// attributed row, never a sweep), and that closure is the map's EndedAt.
func loadMapSet(root string, now time.Time) mapSet {
	path := threadsPath()
	folded := loadAllThreadRows(path)
	var headers []threadItem
	for _, it := range folded {
		if it.Kind == mapHeaderKind {
			headers = append(headers, it)
		}
	}
	// Close any header whose own condition now holds, against every row
	// (a session_ended condition reads the session-end notes).
	closed := map[string]threadItem{}
	for _, h := range evaluateMapConditions(headers, folded, root, now) {
		closed[h.ID] = h
	}

	set := mapSet{maps: explmap.Maps{}, headers: map[string]threadItem{}}
	for _, h := range closed {
		id := h.Pointers["map"]
		if id == "" {
			continue
		}
		m := explmap.Map{
			ID:        id,
			Kind:      explmap.Kind(h.Pointers["map_kind"]),
			Name:      h.Pointers["name"],
			Purpose:   h.Pointers["purpose"],
			WhenToUse: h.Pointers["when"],
			Includes:  strings.Fields(h.Pointers["includes"]),
			CreatedBy: h.Pointers["created_by"],
			CreatedAt: parseRFC3339(h.Pointers["created_at"]),
			LastUsed:  parseRFC3339(h.FiledAt),
			EndedAt:   parseRFC3339(h.ClosedAt),
		}
		if m.Kind == "" {
			m.Kind = explmap.KindEphemeral
		}
		set.maps[id] = m
		set.headers[id] = h
	}

	// The header history: an include a later restatement no longer carries
	// was dropped at that restatement.
	raw := loadThreadRowsRaw(path)
	prevIncludes := map[string][]string{}
	for _, r := range raw {
		if r.Kind != mapHeaderKind || r.Pointers == nil {
			continue
		}
		id := r.Pointers["map"]
		m, ok := set.maps[id]
		if !ok {
			continue
		}
		cur := strings.Fields(r.Pointers["includes"])
		for _, old := range prevIncludes[id] {
			if !sliceContains(cur, old) {
				m.Drops = append(m.Drops, explmap.IncludeDrop{ID: old, At: parseRFC3339(r.FiledAt)})
			}
		}
		prevIncludes[id] = cur
		set.maps[id] = m
	}

	// Entries, in filing order: the log's own order (an entry row is never
	// restated, so the first row with an id is the entry).
	seenEntry := map[string]bool{}
	for _, it := range raw {
		if it.Kind != mapEntryKind || seenEntry[it.ID] {
			continue
		}
		seenEntry[it.ID] = true
		id := it.Pointers["map"]
		m, ok := set.maps[id]
		if !ok {
			continue // an entry whose header is gone is not served
		}
		path, start, end, _, _ := explmap.ParseAnchor(it.Pointers["span"])
		e := explmap.Entry{
			ID:      it.ID,
			Anchor:  it.Pointers["anchor"],
			Path:    path,
			Start:   start,
			End:     end,
			Symbol:  it.Pointers["symbol"],
			Hash:    it.Pointers["hash"],
			Why:     it.Text,
			AgentID: it.Pointers["agent_id"],
			Session: it.Pointers["session_id"],
			FiledAt: parseRFC3339(it.FiledAt),
			Commit:  it.Commit,
		}
		m.Entries = append(m.Entries, e)
		if e.FiledAt.After(m.LastUsed) {
			m.LastUsed = e.FiledAt
		}
		set.maps[id] = m
	}
	return set
}

// evaluateMapConditions closes each open header whose condition now holds,
// through the same condition check and the same durable closure row the
// thread verbs use; the rows are checked against every folded row.
func evaluateMapConditions(headers, all []threadItem, root string, now time.Time) []threadItem {
	out := make([]threadItem, 0, len(headers))
	for _, h := range headers {
		if h.ClosedAt == "" && h.CompletesWhen != nil && threadConditionHolds(*h.CompletesWhen, root, all) {
			h.ClosedAt = now.UTC().Format(time.RFC3339)
			h.ClosedBy = "condition"
			h.ClosedWhy = describeCondition(*h.CompletesWhen) + " now holds"
			_ = appendThreadItem(threadItem{
				SchemaVersion: threadSchemaVersion, ID: h.ID,
				ClosedAt: h.ClosedAt, ClosedBy: h.ClosedBy, ClosedWhy: h.ClosedWhy,
			})
		}
		out = append(out, h)
	}
	return out
}

// mapHeaderRow renders m as a header row: every field restated, so the latest
// row is the whole header. base carries the thread, the scope tags, the
// creation time and the condition over from the previous header, if any.
func mapHeaderRow(m explmap.Map, base threadItem, thread string, root string, now time.Time) threadItem {
	ptr := map[string]string{
		"map":           m.ID,
		"map_kind":      string(m.Kind),
		"includes":      strings.Join(m.Includes, " "),
		"created_by":    m.CreatedBy,
		"repo":          strings.ToLower(filepath.Base(root)),
		"repo_identity": repoIdentity(root),
	}
	created := m.CreatedAt
	if created.IsZero() {
		created = now
	}
	ptr["created_at"] = created.UTC().Format(time.RFC3339)
	for k, v := range map[string]string{"name": m.Name, "purpose": m.Purpose, "when": m.WhenToUse} {
		if strings.TrimSpace(v) != "" {
			ptr[k] = strings.TrimSpace(v)
		}
	}
	if sid := currentSessionID(); sid != "" {
		ptr["session_id"] = sid
	}
	if base.Thread != "" {
		thread = base.Thread
	}
	text := strings.TrimSpace(m.Purpose)
	if text == "" {
		text = "exploration map " + m.ID
	}
	return threadItem{
		SchemaVersion: threadSchemaVersion,
		ID:            mapHeaderIDPrefix + m.ID,
		Thread:        thread,
		Kind:          mapHeaderKind,
		Text:          text,
		Pointers:      ptr,
		FiledAt:       now.UTC().Format(time.RFC3339),
		FiledBy:       threadFiler(),
		CompletesWhen: base.CompletesWhen,
		ClosedAt:      base.ClosedAt,
		ClosedBy:      base.ClosedBy,
		ClosedWhy:     base.ClosedWhy,
	}
}

// writeMapHeader validates m (explmap.Validate: the id, the kind's rules, a
// named map's purpose line) and appends its header row.
func writeMapHeader(set mapSet, m explmap.Map, thread, root string, cond *threadCondition, now time.Time) error {
	if err := explmap.Validate(m); err != nil {
		return err
	}
	base := set.headers[m.ID]
	if base.ID == "" {
		base.CompletesWhen = cond
	}
	if thread == "" && base.Thread == "" {
		thread = threadScopeFor(root).defaultThread()
	}
	row := mapHeaderRow(m, base, thread, root, now)
	row.Commit = gitHeadShort(root)
	return appendThreadItem(row)
}

// ─── the entry (XM-1, XM-14) ─────────────────────────────────────────────

// mapSymbolResolver resolves a symbol anchor to its current span: the index,
// in the product (ctxIndex); a table, in a test.
type mapSymbolResolver interface {
	symbolSpan(key string) (path string, start, end int, ok bool)
}

// mapEntryRequest is one append.
type mapEntryRequest struct {
	Map     string
	Anchor  string
	Why     string
	AgentID string
	Thread  string
	Root    string
	// Completes is the map's `completes_when`, used when this append
	// creates the map; nil means the current session's end, when a session
	// is known.
	Completes *threadCondition
	Now       time.Time
}

// appendMapEntry files one anchored entry on a map (XM-1): the anchor (a
// symbol key or `path#Lx-Ly`), the span it resolves to, that span's content
// hash (core.HashLines, the layer claims' contract), a why line, and the
// provenance (the appending agent's id, XM-14). A map that does not exist yet
// is created as an ephemeral map on the thread, with a `completes_when`: the
// one given, else the end of the current session. The entry row always gets a
// fresh id, so an append never edits an existing entry, anyone's; a composite
// is made only of references and refuses entries.
func appendMapEntry(req mapEntryRequest, syms mapSymbolResolver) (threadItem, error) {
	if !explmap.ValidID(req.Map) {
		return threadItem{}, fmt.Errorf("map id %q: want a letter or digit, then up to 63 letters, digits, '.', '_' or '-'", req.Map)
	}
	if strings.HasPrefix(req.Map, explmap.CompositePrefix) {
		return threadItem{}, fmt.Errorf("map %q is a composite: a composite carries references only — append to one of its maps, or to your own map and compose it (`fathom map compose`)", req.Map)
	}
	why := strings.TrimSpace(req.Why)
	if why == "" || strings.ContainsAny(why, "\r\n") {
		return threadItem{}, fmt.Errorf("a map entry needs a one-line --why: why this code matters for the map's purpose")
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	root := req.Root

	path, start, end, symbol, ok := explmap.ParseAnchor(req.Anchor)
	if !ok {
		return threadItem{}, fmt.Errorf("anchor %q: want a symbol key or path#Lx-Ly", req.Anchor)
	}
	if symbol != "" {
		if syms == nil {
			return threadItem{}, fmt.Errorf("anchor %q names a symbol, and no index is reachable to resolve it (pass a path#Lx-Ly anchor, or --store/--url)", req.Anchor)
		}
		p, s, e, found := syms.symbolSpan(symbol)
		if !found {
			return threadItem{}, fmt.Errorf("anchor %q does not resolve in the index", req.Anchor)
		}
		path, start, end = p, s, e
	}
	rel, err := repoRelative(root, path)
	if err != nil {
		return threadItem{}, fmt.Errorf("anchor %q: %v", req.Anchor, err)
	}
	content, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return threadItem{}, fmt.Errorf("anchor %q: %v", req.Anchor, err)
	}
	hash, ok := core.HashLines(content, start, end)
	if !ok {
		return threadItem{}, fmt.Errorf("anchor %q: %s has no lines %d-%d", req.Anchor, rel, start, end)
	}

	set := loadMapSet(root, now)
	thread := req.Thread
	if thread == "" {
		thread = threadScopeFor(root).defaultThread()
	}
	if m, exists := set.maps[req.Map]; exists {
		if m.Kind == explmap.KindComposite {
			return threadItem{}, fmt.Errorf("map %q is a composite: a composite carries references only", req.Map)
		}
	} else {
		m := explmap.Map{ID: req.Map, Kind: explmap.KindEphemeral, CreatedBy: mapCreator(req.AgentID), CreatedAt: now}
		if err := writeMapHeader(set, m, thread, root, defaultMapCondition(req.Completes), now); err != nil {
			return threadItem{}, err
		}
	}

	ptr := map[string]string{
		"map":           req.Map,
		"anchor":        strings.TrimSpace(req.Anchor),
		"span":          core.CiteRef(rel, start, end),
		"hash":          hash,
		"repo":          strings.ToLower(filepath.Base(root)),
		"repo_identity": repoIdentity(root),
	}
	if symbol != "" {
		ptr["symbol"] = symbol
	}
	if req.AgentID != "" {
		ptr["agent_id"] = req.AgentID
	}
	if sid := currentSessionID(); sid != "" {
		ptr["session_id"] = sid
	}
	taken := map[string]bool{}
	for _, r := range loadThreadRowsRaw(threadsPath()) {
		taken[r.ID] = true
	}
	id := newThreadID(now, "map-entry\x00"+req.Map+"\x00"+req.Anchor+"\x00"+req.AgentID+"\x00"+why)
	for n := 1; taken[id]; n++ {
		id = newThreadID(now, fmt.Sprintf("map-entry\x00%s\x00%s\x00%s\x00%s\x00%d", req.Map, req.Anchor, req.AgentID, why, n))
	}
	it := threadItem{
		SchemaVersion: threadSchemaVersion,
		ID:            id,
		Thread:        thread,
		Kind:          mapEntryKind,
		Text:          why,
		Pointers:      ptr,
		FiledAt:       now.UTC().Format(time.RFC3339),
		FiledBy:       threadFiler(),
		Commit:        gitHeadShort(root),
	}
	if err := appendThreadItem(it); err != nil {
		return threadItem{}, err
	}
	return it, nil
}

// defaultMapCondition is the `completes_when` a new map gets: the one given,
// else the end of the current session when a session is known (a map is a run's
// working set, and it ends with the run), else none.
func defaultMapCondition(cond *threadCondition) *threadCondition {
	if cond != nil {
		return cond
	}
	if sid := currentSessionID(); sid != "" {
		return &threadCondition{Kind: "session_ended", SessionID: sid}
	}
	return nil
}

// repoRelative makes an anchor path repo-relative, refusing one outside root.
func repoRelative(root, p string) (string, error) {
	if filepath.IsAbs(p) {
		r, err := filepath.Rel(root, p)
		if err != nil {
			return "", err
		}
		p = r
	}
	p = filepath.ToSlash(filepath.Clean(p))
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", fmt.Errorf("%s is outside the repository", p)
	}
	return p, nil
}

// idsOwnedBy returns the ids of the maps whose header belongs to scope's
// repository, sorted.
func (s mapSet) idsOwnedBy(scope threadScope) []string {
	var out []string
	for id, h := range s.headers {
		if scope.owns(h) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
