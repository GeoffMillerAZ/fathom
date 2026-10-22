package main

// maphooks.go — how exploration maps ARRIVE rather than being looked up
// (docs/design/exploration-maps.md section 4, ADR-056, P16: delivery is
// configuration, not agent habit). Four channels, each a hook the harness
// already fires, each deterministic and model-free (provider none):
//
//   - the SubagentStart handoff carries ONE map id inside RC-28's 1,200-byte
//     cap (XM-8, subagentMapLine);
//   - relevance by location: a prompt or an edit that touches code inside a
//     map's anchored footprint gets one pointer line, once per session per
//     map (XM-11, mapPointerLine);
//   - the session brief names at most three maps (XM-12, briefMaps);
//   - a thread note may reference maps, the pre-compact pointer carries the
//     active map ids, and the brief re-surfaces them when work on the thread
//     resumes (XM-13, activeMapIDs, mapsPointerKey).
//
// No new table, cache or background job (R5, ADR-049 decision 2): maps are the
// rows mapstore.go reads, and the once-per-session memory is the per-session
// marker every hook that speaks once already keeps under $TMPDIR (bashdoubt.go,
// guardstamp.go), pruned by gc. Every channel is silent when it has nothing
// true to say, and a repository whose threads log holds no map row pays one
// file read and no parsing.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/explmap"
)

// mapsPointerKey is the threadItem.Pointers key that names maps: a thread
// note's references (`thread note --map-ref`) and the pre-compact pointer's
// active maps (XM-13), space-separated ids.
const mapsPointerKey = "maps"

// mapShowHint is how a reader expands a map to its handoff budget.
func mapShowHint(id string) string { return "fathom map show " + id + " --budget 1200" }

// threadsHoldMaps is the gate every map channel takes first: does the threads
// log carry a single map row. A repository with no maps costs one read.
func threadsHoldMaps() bool {
	data, err := readThreadsFile(threadsPath())
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(`"kind":"map`))
}

// hookMapRoot is the repository root a hook resolves maps against: projDir when
// it is the work tree's top (however it is spelled through a symlink), else the
// work tree's top, found by stat and never by a fork.
func hookMapRoot(projDir string) string {
	projDir = strings.TrimRight(projDir, "/")
	if projDir == "" {
		return ""
	}
	top := gitToplevelByStat(projDir)
	if top == "" || top == projDir || resolveSymlinks(projDir) == top {
		return projDir
	}
	return top
}

// hookMapScope is the scope a hook filters maps by: the repository's identity
// and basename, with no daemon round trip (a row filed by the map verbs carries
// the exact identity tag, which is what owns matches).
func hookMapScope(root string) threadScope {
	return threadScope{identity: repoIdentity(root), repo: strings.ToLower(filepath.Base(root))}
}

// loadAliveMaps reads this repository's maps that have not ended, most recently
// used first. A composite is left out: it only references, and a channel that
// names one names what it references. A log without maps costs one file read
// and no JSON decoding.
func loadAliveMaps(root string, now time.Time) (mapSet, []string) {
	if root == "" || hookDeadlineSpent() {
		return mapSet{}, nil
	}
	set := loadMapSet(root, now)
	if hookDeadlineSpent() {
		return mapSet{}, nil
	}
	life := explmap.Lifetimes(set.maps)
	var ids []string
	for _, id := range set.idsOwnedBy(hookMapScope(root)) {
		if set.maps[id].Kind == explmap.KindComposite || life[id].State == explmap.Ended {
			continue
		}
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := set.maps[ids[i]], set.maps[ids[j]]
		if !a.LastUsed.Equal(b.LastUsed) {
			return a.LastUsed.After(b.LastUsed)
		}
		return ids[i] < ids[j]
	})
	return set, ids
}

// mapFiledInSession reports whether the session filed the map's header or any
// of its entries: what a hook keyed by a session id can know about a map.
func mapFiledInSession(set mapSet, id, sessionID string) bool {
	if sessionID == "" {
		return false
	}
	if set.headers[id].Pointers["session_id"] == sessionID {
		return true
	}
	for _, e := range set.maps[id].Entries {
		if e.Session == sessionID {
			return true
		}
	}
	return false
}

// hookOneLine collapses s to a single line of at most max bytes, cut on a rune
// boundary with an ellipsis, so a purpose line can never break the line that
// carries it.
func hookOneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return ""
	}
	return truncateBytes(s, max-3) + "…"
}

// ─── XM-8: the SubagentStart handoff carries one map id ─────────────────────

const (
	// subagentMapLineMax bounds the handoff's map line. The line leads the
	// discoveries chunk, so a cut at RC-28's cap takes the brief's tail and
	// never the id (the reading: 28 of 325 briefed payloads were cut at exactly
	// 1,200 bytes, so an id placed after the brief would have been lost on 9
	// percent of them).
	subagentMapLineMax = 300
	// subagentMapCap is how many of the parent's maps one composite references.
	subagentMapCap = 6
	// subagentMapLead begins the line, and lets the primer tell it from a brief.
	subagentMapLead = "exploration map "
)

// subagentMapLine is the line a SubagentStart hands the new sub-agent: ONE map
// id (XM-8) chosen from the state the parent session filed, with how to read
// it and the sub-agent's own id for `--agent` (the payload is the only place
// fathom sees that id: Claude Code does not export it to a Bash environment).
// The input carries no prompt (XM-V1), so the choice cannot follow the task:
// it is the parent's alive maps, and when there are several a minted
// composite, made only of references and ending with the session, so the brief
// still carries one id (XM-3). "" when the parent filed nothing.
func subagentMapLine(root, parentSessionID, agentID string, now time.Time) string {
	if parentSessionID == "" || root == "" {
		return ""
	}
	set, alive := loadAliveMaps(root, now)
	var filed []string
	for _, id := range alive {
		if mapFiledInSession(set, id, parentSessionID) {
			filed = append(filed, id)
		}
	}
	roots := mapRoots(set.maps, filed)
	if len(roots) == 0 {
		return ""
	}
	if len(roots) > subagentMapCap {
		roots = roots[:subagentMapCap]
	}
	id := roots[0]
	if len(roots) > 1 {
		if cid, ok := mintHandoffComposite(set, roots, parentSessionID, root, now); ok {
			id = cid
		}
	}
	return renderSubagentMapLine(set.maps, id, agentID)
}

// mapRoots keeps the maps in ids that no other map in ids includes: what an
// include reaches is already reached through its includer. The order of ids is
// kept.
func mapRoots(maps explmap.Maps, ids []string) []string {
	included := map[string]bool{}
	for _, id := range ids {
		for _, inc := range maps[id].Includes {
			included[inc] = true
		}
	}
	var out []string
	for _, id := range ids {
		if !included[id] {
			out = append(out, id)
		}
	}
	return out
}

// mintHandoffComposite composes ids into the one id a handoff carries. The id
// derives from the references alone, taken in id order so a change in which map
// was used last never changes it, and an id already stored is not written
// again: every spawn in a session with the same maps hands down the same
// composite. The composite carries the parent session's end as its condition,
// so it does not pin the maps it references past that session.
func mintHandoffComposite(set mapSet, ids []string, parentSessionID, root string, now time.Time) (string, bool) {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	id, minted, err := explmap.Compose(sorted, set.maps, "hook:subagent-start", now)
	if err != nil {
		return "", false
	}
	if minted == nil {
		return id, true
	}
	if _, stored := set.maps[id]; stored {
		return id, true
	}
	cond := &threadCondition{Kind: "session_ended", SessionID: parentSessionID}
	if writeMapHeader(set, *minted, "", root, cond, now) != nil {
		return "", false
	}
	return id, true
}

// renderSubagentMapLine renders the handoff line for id within
// subagentMapLineMax. The read command and the provenance hint are the line;
// the purpose is what yields bytes.
func renderSubagentMapLine(maps explmap.Maps, id, agentID string) string {
	m := maps[id]
	entries, others := len(m.Entries), len(m.Includes)
	if res, err := explmap.ResolveMap(id, maps); err == nil {
		entries, others = 0, len(res.Nodes)-1
		for _, n := range res.Nodes {
			entries += len(maps[n.ID].Entries)
		}
	}
	size := countNoun(entries, "entry", "entries")
	if m.Kind == explmap.KindComposite {
		size = countNoun(others, "map", "maps") + ", " + size
	}
	core := fmt.Sprintf("%s%s (%s): %s", subagentMapLead, id, size, mapShowHint(id))
	add := ` · add findings: fathom thread note --map <your-map>`
	if agentID != "" {
		add += " --agent " + agentID
	}
	add += ` --anchor <symbol|path#Lx-Ly> --why "…"`
	line := core
	if len(line)+len(add) <= subagentMapLineMax {
		line += add
	}
	if purpose := hookOneLine(m.Purpose, subagentMapLineMax-len(line)-len(" — ")); purpose != "" {
		line += " — " + purpose
	}
	return line
}

func countNoun(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// isSubagentMapLine reports whether s is a handoff map line.
func isSubagentMapLine(s string) bool { return strings.HasPrefix(s, subagentMapLead) }

// joinSubagentPrimer appends the discoveries chunk to the base primer inside
// budget (RC-28's 1,200 bytes). The chunk leads with the map line when there is
// one (XM-8), and a cut takes the tail, so the brief yields bytes before the id
// ever does; only a base too long to leave the id its room yields to it, and
// then the brief goes too. Every cut is named (law 3).
func joinSubagentPrimer(base, disc string, budget int) string {
	if disc == "" {
		return capBytes(base, budget, subagentPrimerCut)
	}
	first, rest, hasRest := strings.Cut(disc, "\n")
	if !isSubagentMapLine(first) {
		return capBytes(base+"\n"+disc, budget, subagentPrimerCut)
	}
	if len(base)+1+len(first) > budget {
		room := budget - 1 - len(first) - len(subagentPrimerCut)
		if room < 0 {
			room = 0
		}
		return truncateBytes(base, room) + subagentPrimerCut + "\n" + first
	}
	head := base + "\n" + first
	if !hasRest || len(head)+1+len(rest) <= budget {
		if !hasRest {
			return head
		}
		return head + "\n" + rest
	}
	left := budget - len(head) - 1 - len(subagentPrimerCut)
	if left <= 0 {
		if len(head)+len(subagentPrimerCut) <= budget {
			return head + subagentPrimerCut
		}
		return head
	}
	return head + "\n" + truncateBytes(rest, left) + subagentPrimerCut
}

// capBytes cuts s to budget bytes, the cut named (law 3).
func capBytes(s string, budget int, cut string) string {
	if len(s) <= budget {
		return s
	}
	keep := budget - len(cut)
	if keep < 0 {
		keep = 0
	}
	return truncateBytes(s, keep) + cut
}

// ─── XM-11: relevance by location ───────────────────────────────────────────

const (
	// mapPointerLineMax bounds one pointer line.
	mapPointerLineMax = 280
	// mapPointerMaxLines is how many maps one firing may name.
	mapPointerMaxLines = 2
	// mapPointerFreshCap bounds the entries whose spans are re-hashed for the
	// freshness in a pointer: the hook does a bounded amount of work.
	mapPointerFreshCap = 60
	// mapPointerTTL matches gc's retention of the once-per-session markers.
	mapPointerTTL = 24 * time.Hour
)

// mapPointerDir is os.TempDir()/fathom-map-pointer — a package var (not a
// const) so tests can point it at a t.TempDir().
var mapPointerDir = filepath.Join(os.TempDir(), "fathom-map-pointer")

// mapTouch is the code a prompt or an edit touches, as the footprint check
// reads it: files and directories are repo-relative, slash separated, and
// symbols are as written.
type mapTouch struct {
	Files, Dirs, Symbols []string
}

func (t mapTouch) empty() bool { return len(t.Files)+len(t.Dirs)+len(t.Symbols) == 0 }

// touches reports whether an entry's anchor lies inside what was touched: its
// file is a touched file, or under a touched directory, or its symbol key is a
// touched symbol (in full, or by the name after its last package separator).
func (t mapTouch) touches(e explmap.Entry) bool {
	for _, f := range t.Files {
		if e.Path == f {
			return true
		}
	}
	for _, d := range t.Dirs {
		if e.Path == d || strings.HasPrefix(e.Path, d+"/") {
			return true
		}
	}
	if e.Symbol != "" {
		for _, s := range t.Symbols {
			if e.Symbol == s || strings.HasSuffix(e.Symbol, "."+s) || strings.HasSuffix(e.Symbol, "/"+s) {
				return true
			}
		}
	}
	return false
}

// mapPointerLine is the relevance-by-location channel (XM-11): when a prompt or
// an edit touches code inside the anchored footprint of an alive map, one line
// names the map (its purpose, its size, its freshness, how to expand it),
// once per session per map and within mapPointerLineMax bytes a line. "" when
// nothing touched lies in a map, when every map that does was already named
// this session, or with no session id to remember it by (a line that could not
// be once-only is not sent). A footprint is the map's own entries: a map that
// merely includes another is not named for the base's files.
func mapPointerLine(root, sessionID string, touch mapTouch, now time.Time) string {
	if sessionID == "" || touch.empty() {
		return ""
	}
	set, alive := loadAliveMaps(root, now)
	type hit struct {
		id      string
		matched int
	}
	var hits []hit
	for _, id := range alive {
		n := 0
		for _, e := range set.maps[id].Entries {
			if touch.touches(e) {
				n++
			}
		}
		if n > 0 {
			hits = append(hits, hit{id, n})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].matched > hits[j].matched })
	marks := sessionMarker{dir: mapPointerDir, ttl: mapPointerTTL}
	var lines []string
	for _, h := range hits {
		if len(lines) == mapPointerMaxLines {
			break
		}
		if marks.seen(sessionID, now, h.id) {
			continue
		}
		lines = append(lines, renderMapPointer(root, set.maps[h.id]))
		marks.mark(sessionID, now, h.id)
	}
	return strings.Join(lines, "\n")
}

// renderMapPointer renders the one line for m: the id, the authored purpose,
// the size, the freshness of its own entries re-hashed now, and the command
// that expands it. Freshness needs no index and no git: a moved symbol reads as
// changed here, and `map show` is what re-resolves it and sizes the diff.
func renderMapPointer(root string, m explmap.Map) string {
	checked := m
	checked.Includes = nil
	if len(checked.Entries) > mapPointerFreshCap {
		checked.Entries = checked.Entries[:mapPointerFreshCap]
	}
	fresh := "freshness not checked"
	if x, err := explmap.Expand(m.ID, explmap.Maps{m.ID: checked}, hookMapSource{newRepoMapSource(root, nil)}); err == nil {
		switch changed := x.Stale + x.ReResolved; {
		case changed == 0:
			fresh = "all fresh"
		default:
			fresh = fmt.Sprintf("%d of %d changed since filed", changed, len(x.Items))
		}
	}
	core := fmt.Sprintf("fathom map %s: %s (%s); expand: %s", m.ID, countNoun(len(m.Entries), "entry", "entries"), fresh, mapShowHint(m.ID))
	purpose := hookOneLine(m.Purpose, mapPointerLineMax-len(core)-len(" — "))
	if purpose == "" {
		return core
	}
	return core + " — " + purpose
}

// hookMapSource is the freshness source a hook uses: the repository's files
// only. A hook forks no git and asks no daemon, so a symbol is not re-resolved
// and a stale entry's diff size is not measured.
type hookMapSource struct{ *repoMapSource }

func (hookMapSource) Symbol(string) (string, int, int, bool)        { return "", 0, 0, false }
func (hookMapSource) Filed(string, string, int, int) (string, bool) { return "", false }

// withMapPointer composes the pointer for an edited file onto the edit hook's
// payload, after whatever the hook already says; alone, it turns a silent hook
// into an answering one. Needs no daemon.
func withMapPointer(projDir, filePath, sessionID, text string, ok bool) (string, bool) {
	root := hookMapRoot(projDir)
	if root == "" {
		return text, ok
	}
	rel, inside := relativeToRoot(root, filePath)
	if !inside {
		return text, ok
	}
	line := mapPointerLine(root, sessionID, mapTouch{Files: []string{filepath.ToSlash(rel)}}, time.Now())
	return appendHookLine(text, ok, line)
}

// withPromptMapPointer is withMapPointer for a prompt: the paths, packages and
// bare symbols the prompt names, judged against the working tree by the same
// extraction the obligations half of the hook uses.
func withPromptMapPointer(projDir, prompt, sessionID, text string, ok bool) (string, bool) {
	root := hookMapRoot(projDir)
	if root == "" || sessionID == "" || !threadsHoldMaps() {
		return text, ok
	}
	var touch mapTouch
	for _, c := range extractPromptCandidates(prompt, workingTreeNames([]string{root})) {
		switch c.kind {
		case candidatePath:
			touch.Files = append(touch.Files, c.text)
		case candidatePackage:
			touch.Dirs = append(touch.Dirs, c.text)
		case candidateSymbol:
			touch.Symbols = append(touch.Symbols, c.text)
		}
	}
	return appendHookLine(text, ok, mapPointerLine(root, sessionID, touch, time.Now()))
}

// appendHookLine puts line after text, or on its own when the hook had nothing.
func appendHookLine(text string, ok bool, line string) (string, bool) {
	if line == "" {
		return text, ok
	}
	if ok && text != "" {
		return text + "\n" + line, true
	}
	return line, true
}

// sessionMarker is the once-per-session memory a hook keeps of what it already
// said: a file per (session, key) under dir, in the shape bashdoubt.go and
// guardstamp.go use. A 0700 directory under the temp dir (never a repository
// artifact), a 0600 file created O_EXCL|O_NOFOLLOW, and an owner, permission
// and regular-file check on the open descriptor when reading. A session with no
// id has no marker: a key with no session scope could be planted by anyone who
// can write the shared temp dir. Every failure reads as "not seen", the cheap
// one.
type sessionMarker struct {
	dir string
	ttl time.Duration
}

func (m sessionMarker) path(sessionID, key string) string {
	h := sha256.Sum256([]byte(sessionID + "\x1f" + key))
	return filepath.Join(m.dir, hex.EncodeToString(h[:16])+".stamp")
}

func (m sessionMarker) mark(sessionID string, now time.Time, key string) {
	if sessionID == "" {
		return
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return
	}
	p := m.path(sessionID, key)
	_ = os.Remove(p) // never write through a pre-existing entry
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write([]byte(now.UTC().Format(time.RFC3339)))
}

func (m sessionMarker) seen(sessionID string, now time.Time, key string) bool {
	if sessionID == "" {
		return false
	}
	f, err := os.OpenFile(m.path(sessionID, key), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return false
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(f, 64))
	if err != nil {
		return false
	}
	at, err := time.Parse(time.RFC3339, string(data))
	return err == nil && now.Sub(at) <= m.ttl
}

// ─── XM-12: the session brief names at most three maps ──────────────────────

// maxBriefMaps is the most maps the session brief names, however many are
// alive: never a full list.
const maxBriefMaps = 3

// briefMapPurposeMax bounds the purpose a brief bullet carries.
const briefMapPurposeMax = 72

// briefMap is one map the session brief names.
type briefMap struct {
	ID      string
	Purpose string
	Entries int
	// Overlap is how many of the thread's recent files the map's entries lie in.
	Overlap int
	// Named is true when an open thread note (or the pre-compact pointer)
	// references the map: resuming the thread re-surfaces it (XM-13).
	Named bool
}

// briefMaps chooses the maps the session brief names, at most maxBriefMaps, and
// how many alive maps it left out. Ranked by: named by an open note of the
// thread, then overlap with the thread's recent files, then recent use (alive
// is already ordered most recently used first, and the sort is stable), then
// id. alive is the caller's ordered candidates, named the ids the thread's open
// notes reference, and recent the repo-relative files the thread touched
// recently.
func briefMaps(maps explmap.Maps, alive []string, named map[string]bool, recent []string) (shown []briefMap, more int) {
	recentSet := map[string]bool{}
	for _, f := range recent {
		recentSet[f] = true
	}
	all := make([]briefMap, 0, len(alive))
	for _, id := range alive {
		m := maps[id]
		bm := briefMap{ID: id, Purpose: strings.TrimSpace(m.Purpose), Entries: len(m.Entries), Named: named[id]}
		seen := map[string]bool{}
		for _, e := range m.Entries {
			if recentSet[e.Path] && !seen[e.Path] {
				seen[e.Path] = true
				bm.Overlap++
			}
		}
		all = append(all, bm)
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Named != all[j].Named {
			return all[i].Named
		}
		if all[i].Overlap != all[j].Overlap {
			return all[i].Overlap > all[j].Overlap
		}
		return false
	})
	if len(all) > maxBriefMaps {
		return all[:maxBriefMaps], len(all) - maxBriefMaps
	}
	return all, 0
}

// briefMapLines renders the brief's map bullets for the items the brief serves.
// Empty when no map is alive: a brief with nothing to name costs nothing.
func briefMapLines(items []threadItem, root string, now time.Time) []string {
	set, alive := loadAliveMaps(root, now)
	if len(alive) == 0 {
		return nil
	}
	scope := hookMapScope(root)
	named := map[string]bool{}
	for _, it := range items {
		if it.ClosedAt != "" || threadDormant(it, now) || !scope.owns(it) {
			continue
		}
		for _, id := range strings.Fields(it.Pointers[mapsPointerKey]) {
			named[id] = true
		}
	}
	shown, more := briefMaps(set.maps, alive, named, threadRecentFiles(items, scope, root))
	return renderBriefMapLines(shown, more)
}

// renderBriefMapLines renders the bullets: one per named map, one closing line
// with the expand command, and a count of the maps left out.
func renderBriefMapLines(shown []briefMap, more int) []string {
	if len(shown) == 0 {
		return nil
	}
	var lines []string
	for _, m := range shown {
		why := countNoun(m.Entries, "entry", "entries")
		if m.Named {
			why += ", named by a thread note"
		}
		if m.Overlap > 0 {
			why += ", " + countNoun(m.Overlap, "recent file", "recent files")
		}
		line := "  • (map " + m.ID + ") "
		if p := hookOneLine(m.Purpose, briefMapPurposeMax); p != "" {
			line += p + " — "
		}
		lines = append(lines, line+why)
	}
	tail := "  • expand a map: " + mapShowHint("<id>")
	if more > 0 {
		tail += fmt.Sprintf(" · %d more: fathom map list", more)
	}
	return append(lines, tail)
}

// porcelainPaths reads the paths out of `git status --porcelain` output that
// has been trimmed as a whole: sessionFactsGitExec trims its result, which takes
// the leading space off the first line (" M x" reads "M x"), so a line is a
// two-column status, a space and the path, or one column short on that first
// line. A rename names its new path; a quoted path (one with a space) is unquoted.
func porcelainPaths(out string) []string {
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		var p string
		switch {
		case len(line) > 3 && line[2] == ' ':
			p = line[3:]
		case len(line) > 2 && line[1] == ' ':
			p = line[2:]
		default:
			continue
		}
		if _, to, ok := strings.Cut(p, " -> "); ok {
			p = to
		}
		if p = strings.Trim(strings.TrimSpace(p), `"`); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// threadRecentFiles is the files the thread touched recently: the working tree's
// changed files (git status, one fork) and the file-shaped pointers the open
// notes carry (a session-end note's discoveries). Repo-relative, slash
// separated, sorted, deduplicated.
func threadRecentFiles(items []threadItem, scope threadScope, root string) []string {
	seen := map[string]bool{}
	add := func(rel string) {
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel != "." && !strings.HasPrefix(rel, "../") {
			seen[rel] = true
		}
	}
	for _, p := range porcelainPaths(sessionFactsGitExec(root, "status", "--porcelain")) {
		add(p)
	}
	for _, it := range items {
		if it.ClosedAt != "" || !scope.owns(it) {
			continue
		}
		for _, p := range strings.Fields(it.Pointers[discoveriesPointerKey]) {
			f, _ := stripDiscoveryCitation(p)
			if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err == nil && !fi.IsDir() {
				add(f)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// ─── XM-13: the pre-compact pointer carries the active map ids ──────────────

// preCompactMapCap is how many map ids the pre-compact pointer carries.
const preCompactMapCap = 5

// activeMapIDs is the alive maps a session filed or added to, most recently
// used first, at most preCompactMapCap: what the pre-compact pointer carries so
// a session that resumes the thread finds them (the brief re-surfaces them).
func activeMapIDs(root, sessionID string, now time.Time) []string {
	if sessionID == "" {
		return nil
	}
	set, alive := loadAliveMaps(root, now)
	var out []string
	for _, id := range alive {
		if mapFiledInSession(set, id, sessionID) {
			out = append(out, id)
		}
	}
	if len(out) > preCompactMapCap {
		out = out[:preCompactMapCap]
	}
	return out
}

// mapRefsSuffix renders the maps a thread item references, for the brief and
// the handoff: "" when it references none.
func mapRefsSuffix(it threadItem) string {
	if ids := strings.Fields(it.Pointers[mapsPointerKey]); len(ids) > 0 {
		return " [maps " + strings.Join(ids, " ") + "]"
	}
	return ""
}
