package main

// readoutcmd_sizing.go — three sizing instruments `fathom readout replay`
// reads in the same pass over the same transcripts (design
// docs/design/readouts.md §20, the seam map), aggregates only:
//
//   - dedupe: Bash and Read result bytes, and the bytes of results that
//     exactly repeat (sha256 of the result text) or are wholly contained in
//     (a Read span inside a span of the same file read since its last Edit,
//     Write or MultiEdit, or inside a whole-file shell read of it) an
//     earlier result of the same transcript since its last compaction
//     boundary, split within one assistant message's batch and across
//     turns, with a repeat of the same command within five Bash calls (the
//     recovery repeat) excluded and counted apart;
//   - spawns: subagent transcripts (agent-*.jsonl under a session's
//     subagents/, workflow ones under subagents/workflows/<run>/) and the
//     ones whose first three tool calls are code-discovery reads or searches
//     (the Read, Grep and Glob tools, or a shell search or source read by
//     the discovery observer's classifier);
//   - compactions: over the latest 14 days of the window, each compaction
//     boundary (a `system` line of subtype `compact_boundary`), its pre set
//     (files read or edited in the 40 turns before, Bash reads through the
//     lexer included) and the distinct pre-set files re-read in the 20 turns
//     after; a turn is one assistant message id.
//
// Each section evaluates its bar as a verdict projection, never the
// reading's verdict. Result texts are hashed in memory and dropped at the
// transcript's end or its next compaction; no text, path, id or name
// reaches the output: a shape is a command head, everything else a count.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

const (
	sizingKeepBar       = 0.01 // dedupe: repeat bytes over Bash plus Read bytes
	sizingSpawnBar      = 0.30 // spawns: the discovery-first share of workflow subagents
	sizingFirstCalls    = 3    // spawns: a subagent's first calls judged
	sizingCompactDays   = 14   // compactions: the latest days of the window read
	sizingPreTurns      = 40   // compactions: turns before a boundary in its pre set
	sizingPostTurns     = 20   // compactions: turns after a boundary read for re-reads
	sizingPerDayBar     = 2.0  // compactions: events a day to BUILD
	sizingMedianBar     = 2.0  // compactions: median distinct pre-set files re-read to BUILD
	sizingRecoveryCalls = replayFollowers
)

const sizingDedupeNote = "repeats are results of the same transcript since its last compaction boundary: an exact repeat matches an earlier " +
	"result's sha256, a contained Read lies inside a span of the same file read since its last Edit, Write or MultiEdit or inside a " +
	"whole-file shell read of it; within_batch means the earlier call is in the same assistant message; a Bash exact repeat whose " +
	"command is one of the five Bash calls before it is a recovery, excluded from repeat and counted apart; failures and empty results never repeat"

const sizingSpawnsNote = "a subagent is an agent-*.jsonl under a session's subagents directory, a workflow subagent one under its workflows directory; " +
	"in the window by its first timestamp; discovery-first means each of its first three tool calls is a Read, Grep or Glob or a " +
	"shell search or source read; a subagent with fewer calls counts apart, never as discovery-first; the bar reads workflow subagents"

const sizingCompactionsNote = "an event is a compact_boundary line in the latest days of the window, counted once by its uuid; the pre set is the " +
	"files read (Read, read_source, Bash reads through the lexer) or edited (Edit, Write, MultiEdit, NotebookEdit) in the 40 assistant turns " +
	"before it; a re-read is a later Read, read_source, Grep on a path or Bash read of a pre-set file in the 20 turns after, cut at the " +
	"next boundary; an event the transcript ends inside is incomplete; medians read complete events"

// replayDedupe is the dedupe section.
type replayDedupe struct {
	Bash              replayDedupeBash `json:"bash"`
	Read              replayDedupeRead `json:"read"`
	Bytes             int64            `json:"bash_read_bytes"`
	Repeat            replayCount      `json:"repeat"`
	WithinBatch       replayCount      `json:"within_batch"`
	AcrossTurns       replayCount      `json:"across_turns"`
	Share             float64          `json:"share_of_bash_read_bytes"`
	RepeatShapes      []replayShapeRow `json:"repeat_shapes"` // Bash repeats by command head, shares of Bash plus Read bytes
	KeepBar           float64          `json:"keep_bar"`
	VerdictProjection string           `json:"verdict_projection"`
	Note              string           `json:"note"`
}

type replayDedupeBash struct {
	Results          int         `json:"results"`
	Bytes            int64       `json:"bytes"`
	Exact            replaySplit `json:"exact"`
	RecoveryExcluded replayCount `json:"recovery_excluded"`
}

type replayDedupeRead struct {
	Results   int         `json:"results"`
	Bytes     int64       `json:"bytes"`
	Exact     replaySplit `json:"exact"`
	Contained replaySplit `json:"contained"`
}

type replaySplit struct {
	WithinBatch replayCount `json:"within_batch"`
	AcrossTurns replayCount `json:"across_turns"`
}

type replayCount struct {
	Results int   `json:"results"`
	Bytes   int64 `json:"bytes"`
}

func (c *replayCount) add(n int64) {
	c.Results++
	c.Bytes += n
}

func (s *replaySplit) add(within bool, n int64) {
	if within {
		s.WithinBatch.add(n)
		return
	}
	s.AcrossTurns.add(n)
}

// replaySpawns is the spawns section.
type replaySpawns struct {
	Workflow          replaySpawnKind `json:"workflow"`
	Other             replaySpawnKind `json:"other"`
	FirstCalls        int             `json:"first_calls"`
	Bar               float64         `json:"bar"`
	VerdictProjection string          `json:"verdict_projection"`
	Note              string          `json:"note"`
}

type replaySpawnKind struct {
	Subagents      int     `json:"subagents"`
	DiscoveryFirst int     `json:"discovery_first"`
	Share          float64 `json:"share"`
	Fewer          int     `json:"fewer_than_first_calls"`
}

// replayCompactions is the compactions section.
type replayCompactions struct {
	Days              int                   `json:"days"`
	From              string                `json:"from"`
	To                string                `json:"to"`
	Events            int                   `json:"events"`
	PerDay            float64               `json:"per_day"`
	Complete          int                   `json:"complete"`
	PreTurns          int                   `json:"pre_turns"`
	PostTurns         int                   `json:"post_turns"`
	PerEvent          []replayCompactionRow `json:"per_event"`
	MedianPreSet      float64               `json:"median_pre_set"`
	MedianReread      float64               `json:"median_reread"`
	PerDayBar         float64               `json:"per_day_bar"`
	MedianBar         float64               `json:"median_bar"`
	VerdictProjection string                `json:"verdict_projection"`
	Note              string                `json:"note"`
}

type replayCompactionRow struct {
	PreSet    int  `json:"pre_set"`
	Reread    int  `json:"reread"`
	PostTurns int  `json:"post_turns"`
	Complete  bool `json:"complete"`
	Subagent  bool `json:"subagent"`
}

// replaySizing accumulates the three sections over one replay.
type replaySizing struct {
	cutoff, compactFrom time.Time
	compactDays         int
	seen                map[string]struct{} // Bash and Read tool-use ids counted (a resumed session copies its history)
	boundaries          map[string]struct{} // compaction boundary uuids counted
	dedupe              replayDedupe
	shapes              map[string]*replayShapeRow // Bash repeats by command head
	spawns              replaySpawns
	events              []replayCompactionEvent
}

type replayCompactionEvent struct {
	ts    time.Time
	tsRaw string
	row   replayCompactionRow
}

func newReplaySizing(days int, cutoff time.Time) *replaySizing {
	cd := min(days, sizingCompactDays)
	now := cutoff.Add(time.Duration(days) * 24 * time.Hour)
	return &replaySizing{cutoff: cutoff, compactFrom: now.Add(-time.Duration(cd) * 24 * time.Hour), compactDays: cd,
		seen: map[string]struct{}{}, boundaries: map[string]struct{}{}, shapes: map[string]*replayShapeRow{}}
}

// sizingFile is one transcript's sizing state.
type sizingFile struct {
	subagent, workflow bool
	// dedupe, since the last compaction boundary: calls awaiting their
	// result, result hashes (to the assistant message of the latest call),
	// read spans per file, and the last five Bash commands' hashes.
	pending map[string]sizingCall
	hashes  map[[sha256.Size]byte]string
	spans   map[string][]sizingSpan
	recent  [sizingRecoveryCalls][sha256.Size]byte
	nBash   int
	// turns: the files each of the last 40 read or edited, and the event
	// whose post turns are being read.
	lastMsg string
	turned  bool
	ring    [][]string
	open    *sizingOpen
	// spawns: the first timestamp, and the first calls and how many were discovery.
	first     time.Time
	timed     bool
	calls     int
	discovery int
}

type sizingCall struct {
	tool      string // Bash or Read
	msg       string // the assistant message carrying the call
	inWindow  bool
	recovery  bool
	head      string   // Bash: the command head
	kind      string   // Bash: simple, piped or list
	whole     []string // Bash: the files a whole-file read prints, resolved
	file      string   // Read: the resolved path
	offset    int      // Read: the input offset
	hasOffset bool
}

type sizingSpan struct {
	start, end int
	msg        string
}

type sizingOpen struct {
	ts     time.Time
	tsRaw  string
	post   int
	pre    map[string]struct{}
	reread map[string]struct{}
}

type sizingLine struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
	Message   *struct {
		ID      string          `json:"id"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

// open starts a transcript's state, classing it by its path: a subagent's
// transcript is <session>/subagents/agent-*.jsonl, a workflow subagent's
// <session>/subagents/workflows/<run>/agent-*.jsonl.
func (z *replaySizing) open(p string) *sizingFile {
	st := &sizingFile{pending: map[string]sizingCall{}, hashes: map[[sha256.Size]byte]string{}, spans: map[string][]sizingSpan{}}
	dir, base := filepath.Split(p)
	if !strings.HasPrefix(base, "agent-") || !strings.HasSuffix(base, ".jsonl") {
		return st
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(dir)), "/")
	n := len(parts)
	switch {
	case n >= 3 && parts[n-3] == "subagents" && parts[n-2] == "workflows":
		st.subagent, st.workflow = true, true
	case n >= 1 && parts[n-1] == "subagents":
		st.subagent = true
	}
	return st
}

// line reads one transcript line.
func (z *replaySizing) line(st *sizingFile, raw []byte) {
	if !st.subagent || st.timed {
		if !bytes.Contains(raw, []byte(`"tool_`)) && !bytes.Contains(raw, []byte(`"assistant"`)) &&
			!bytes.Contains(raw, []byte(`"compact_boundary"`)) {
			return
		}
	}
	var l sizingLine
	if json.Unmarshal(raw, &l) != nil {
		return // replay's corpus figures count the line
	}
	ts, err := time.Parse(time.RFC3339Nano, l.Timestamp)
	timed := err == nil
	if st.subagent && !st.timed && timed {
		st.first, st.timed = ts, true
	}
	if l.Type == "system" && l.Subtype == "compact_boundary" {
		z.boundary(st, &l, ts, timed)
		return
	}
	if l.Message == nil {
		return
	}
	if l.Type == "assistant" {
		z.turn(st, l.Message.ID)
	}
	if len(l.Message.Content) == 0 || l.Message.Content[0] != '[' {
		return
	}
	var blocks []replayBlock
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return
	}
	inWindow := !timed || !ts.Before(z.cutoff)
	results := 0
	for _, b := range blocks {
		if b.Type == "tool_result" {
			results++
		}
	}
	for i := range blocks {
		switch blocks[i].Type {
		case "tool_use":
			z.use(st, &l, &blocks[i], inWindow)
		case "tool_result":
			z.result(st, &l, &blocks[i], results == 1)
		}
	}
}

// turn starts a new turn at a new assistant message id (each line is its
// own turn when it carries none).
func (z *replaySizing) turn(st *sizingFile, id string) {
	if st.turned && id != "" && id == st.lastMsg {
		return
	}
	st.lastMsg, st.turned = id, true
	if o := st.open; o != nil {
		o.post++
		if o.post > sizingPostTurns {
			o.post = sizingPostTurns
			z.closeEvent(st, true)
		}
	}
	st.ring = append(st.ring, nil)
	if len(st.ring) > sizingPreTurns {
		st.ring = append(st.ring[:0], st.ring[1:]...)
	}
}

// use records one tool call: a spawn's first calls, the files the turn
// reads or edits, a re-read of an open event's pre set, an edit that ends
// the file's spans, and a Bash or Read call awaiting its result.
func (z *replaySizing) use(st *sizingFile, l *sizingLine, b *replayBlock, inWindow bool) {
	var command string
	var cmd *shellcmd.Command
	if b.Name == "Bash" {
		var in struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(b.Input, &in)
		command = in.Command
		c := shellcmd.ParseCommand(command)
		cmd = &c
	}
	if st.subagent && st.calls < sizingFirstCalls {
		st.calls++
		if sizingDiscovery(b.Name, command) {
			st.discovery++
		}
	}
	reads, edits, searched := sizingCallFiles(b, cmd, l.Cwd)
	if len(st.ring) == 0 {
		st.ring = append(st.ring, nil)
	}
	cur := len(st.ring) - 1
	st.ring[cur] = append(append(st.ring[cur], reads...), edits...)
	if o := st.open; o != nil && o.post > 0 {
		for _, group := range [][]string{reads, searched} {
			for _, f := range group {
				if _, ok := o.pre[f]; ok {
					o.reread[f] = struct{}{}
				}
			}
		}
	}
	msg := l.Message.ID
	switch b.Name {
	case "Edit", "Write", "MultiEdit":
		for _, f := range edits {
			delete(st.spans, f)
		}
	case "Bash":
		h := sha256.Sum256([]byte(command))
		call := sizingCall{tool: "Bash", msg: msg, inWindow: inWindow, recovery: st.recovered(h)}
		st.recent[st.nBash%len(st.recent)] = h
		st.nBash++
		call.head, call.kind = replayShape(cmd)
		for _, f := range wholeFileRead(cmd) {
			if p := replayReadPath(cmd, l.Cwd, f); p != "" {
				call.whole = append(call.whole, p)
			}
		}
		if b.ID != "" {
			st.pending[b.ID] = call
		}
	case "Read":
		var in struct {
			FilePath string          `json:"file_path"`
			Offset   json.RawMessage `json:"offset"`
		}
		_ = json.Unmarshal(b.Input, &in)
		call := sizingCall{tool: "Read", msg: msg, inWindow: inWindow, file: sizingPath(in.FilePath, l.Cwd)}
		var off float64
		if len(in.Offset) > 0 && json.Unmarshal(in.Offset, &off) == nil {
			call.offset, call.hasOffset = int(off), true
		}
		if b.ID != "" {
			st.pending[b.ID] = call
		}
	}
}

// recovered reports whether a Bash command's hash is one of the five Bash
// calls' before it since the last boundary.
func (st *sizingFile) recovered(h [sha256.Size]byte) bool {
	for i := 0; i < min(st.nBash, len(st.recent)); i++ {
		if st.recent[i] == h {
			return true
		}
	}
	return false
}

// result counts a Bash or Read result and matches it against the
// transcript's earlier results since the last boundary.
func (z *replaySizing) result(st *sizingFile, l *sizingLine, b *replayBlock, single bool) {
	call, ok := st.pending[b.ToolUseID]
	if !ok {
		return
	}
	delete(st.pending, b.ToolUseID)
	text := resultText(b.Content)
	n := int64(len(text))
	count := call.inWindow
	if count {
		if _, dup := z.seen[b.ToolUseID]; dup {
			count = false
		} else {
			z.seen[b.ToolUseID] = struct{}{}
		}
	}
	if count {
		if call.tool == "Bash" {
			z.dedupe.Bash.Results++
			z.dedupe.Bash.Bytes += n
		} else {
			z.dedupe.Read.Results++
			z.dedupe.Read.Bytes += n
		}
	}
	if b.IsError || n == 0 {
		return
	}
	h := sha256.Sum256([]byte(text))
	kind, within := "", false
	if last, ok := st.hashes[h]; ok {
		kind, within = "exact", last != "" && last == call.msg
	}
	st.hashes[h] = call.msg
	if call.tool == "Read" && call.file != "" {
		if sp, ok := sizingReadSpan(&call, l, single, text); ok {
			if m, covered := st.covered(call.file, sp); covered {
				if kind == "" {
					kind, within = "contained", m != "" && m == call.msg
				}
			} else {
				st.spans[call.file] = append(st.spans[call.file], sp)
			}
		}
	}
	for _, f := range call.whole {
		st.spans[f] = append(st.spans[f], sizingSpan{start: 1, end: math.MaxInt, msg: call.msg})
	}
	if count && kind != "" {
		z.repeat(&call, kind, within, n)
	}
}

// covered returns the message of the latest span of file holding sp.
func (st *sizingFile) covered(file string, sp sizingSpan) (string, bool) {
	spans := st.spans[file]
	for i := len(spans) - 1; i >= 0; i-- {
		if spans[i].start <= sp.start && sp.end <= spans[i].end {
			return spans[i].msg, true
		}
	}
	return "", false
}

// repeat tallies one repeated result in the window.
func (z *replaySizing) repeat(call *sizingCall, kind string, within bool, n int64) {
	d := &z.dedupe
	switch {
	case call.tool == "Bash" && call.recovery:
		d.Bash.RecoveryExcluded.add(n)
		return
	case call.tool == "Bash":
		d.Bash.Exact.add(within, n)
		countShape(z.shapes, call.head, call.kind, n)
	case kind == "exact":
		d.Read.Exact.add(within, n)
	default:
		d.Read.Contained.add(within, n)
	}
	d.Repeat.add(n)
	if within {
		d.WithinBatch.add(n)
	} else {
		d.AcrossTurns.add(n)
	}
}

// sizingReadSpan is the line span a Read printed: the result's startLine
// and numLines when the harness records them, else the input offset and
// the printed line count.
func sizingReadSpan(call *sizingCall, l *sizingLine, single bool, text string) (sizingSpan, bool) {
	var tr struct {
		Type string `json:"type"`
		File *struct {
			StartLine int `json:"startLine"`
			NumLines  int `json:"numLines"`
		} `json:"file"`
	}
	start, lines := 1, lineCount(text)
	if single && len(l.ToolUseResult) > 0 && l.ToolUseResult[0] == '{' && json.Unmarshal(l.ToolUseResult, &tr) == nil {
		if tr.Type != "text" || tr.File == nil {
			return sizingSpan{}, false
		}
		start, lines = max(tr.File.StartLine, 1), tr.File.NumLines
	} else if call.hasOffset {
		start = max(call.offset, 1)
	}
	if lines < 1 {
		return sizingSpan{}, false
	}
	return sizingSpan{start: start, end: start + lines - 1, msg: call.msg}, true
}

// boundary ends the transcript's dedupe context and any open event, and
// opens an event when the boundary is in the latest days of the window.
func (z *replaySizing) boundary(st *sizingFile, l *sizingLine, ts time.Time, timed bool) {
	if st.open != nil {
		z.closeEvent(st, true)
	}
	st.hashes = map[[sha256.Size]byte]string{}
	st.spans = map[string][]sizingSpan{}
	st.nBash = 0
	if !timed || ts.Before(z.compactFrom) {
		return
	}
	if l.UUID != "" {
		if _, dup := z.boundaries[l.UUID]; dup {
			return
		}
		z.boundaries[l.UUID] = struct{}{}
	}
	o := &sizingOpen{ts: ts, tsRaw: l.Timestamp, pre: map[string]struct{}{}, reread: map[string]struct{}{}}
	for _, files := range st.ring {
		for _, f := range files {
			o.pre[f] = struct{}{}
		}
	}
	st.open = o
}

// closeEvent records the open event; complete when its post turns ran out
// or the next boundary cut them.
func (z *replaySizing) closeEvent(st *sizingFile, complete bool) {
	o := st.open
	st.open = nil
	z.events = append(z.events, replayCompactionEvent{ts: o.ts, tsRaw: o.tsRaw, row: replayCompactionRow{
		PreSet: len(o.pre), Reread: len(o.reread), PostTurns: o.post,
		Complete: complete || o.post >= sizingPostTurns, Subagent: st.subagent}})
}

// close ends a transcript: an event it ends inside, and a subagent's spawn figures.
func (z *replaySizing) close(st *sizingFile) {
	if st.open != nil {
		z.closeEvent(st, false)
	}
	if !st.subagent || !st.timed || st.first.Before(z.cutoff) {
		return
	}
	k := &z.spawns.Other
	if st.workflow {
		k = &z.spawns.Workflow
	}
	k.Subagents++
	switch {
	case st.calls < sizingFirstCalls:
		k.Fewer++
	case st.discovery == sizingFirstCalls:
		k.DiscoveryFirst++
	}
}

// sizingDiscovery: a code-discovery call by the discovery observer's
// classes (a shell search or a source read), or the Read, Grep or Glob tool.
func sizingDiscovery(tool, command string) bool {
	switch tool {
	case "Read", "Grep", "Glob":
		return true
	case "Bash":
		k := shellDiscoveryKind(command)
		return k == "shell-search" || k == "shell-read"
	}
	return false
}

// sizingCallFiles returns the files a call reads (Read, read_source, Bash
// reads), edits (Edit, Write, MultiEdit, NotebookEdit) and searches (a
// Grep's path), resolved against the line's cwd.
func sizingCallFiles(b *replayBlock, cmd *shellcmd.Command, cwd string) (reads, edits, searched []string) {
	if b.Name == "Bash" {
		return sizingShellReads(cmd, cwd), nil, nil
	}
	var in struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Path         string `json:"path"`
		File         string `json:"file"`
	}
	if json.Unmarshal(b.Input, &in) != nil {
		return nil, nil, nil
	}
	one := func(p string) []string {
		if q := sizingPath(p, cwd); q != "" {
			return []string{q}
		}
		return nil
	}
	switch {
	case b.Name == "Read":
		return one(in.FilePath), nil, nil
	case b.Name == "Edit" || b.Name == "Write" || b.Name == "MultiEdit":
		return nil, one(in.FilePath), nil
	case b.Name == "NotebookEdit":
		return nil, one(in.NotebookPath), nil
	case b.Name == "Grep":
		return nil, nil, one(in.Path)
	case strings.HasSuffix(b.Name, "__read_source"):
		return one(in.File), nil, nil
	}
	return nil, nil, nil
}

// sizingShellReads returns the files a Bash command's read commands name:
// the literal path-like operands of cat, head, tail, nl, wc, less, diff and
// bat, and of sed, awk, grep, rg and jq past their script or pattern.
func sizingShellReads(c *shellcmd.Command, cwd string) []string {
	if c == nil || c.Unreadable {
		return nil
	}
	var out []string
	for _, u := range c.Units() {
		if u.Kind != shellcmd.KindSimple || len(u.Words) < 2 {
			continue
		}
		skip := 0
		switch u.Name() {
		case "cat", "head", "tail", "nl", "wc", "less", "diff", "bat":
		case "sed", "awk", "grep", "rg", "jq":
			skip = 1
		default:
			continue
		}
		for _, w := range u.Words[1:] {
			if !w.Literal() || w.Text == "" || strings.HasPrefix(w.Text, "-") {
				continue
			}
			if skip > 0 {
				skip--
				continue
			}
			if w.Text == "." || w.Text == ".." || (!strings.Contains(w.Text, "/") && path.Ext(w.Text) == "") {
				continue
			}
			if p := replayReadPath(c, cwd, w.Text); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// sizingPath is p made absolute against cwd and cleaned; "" when it cannot be.
func sizingPath(p, cwd string) string {
	switch {
	case p == "":
		return ""
	case filepath.IsAbs(p):
		return filepath.Clean(p)
	case cwd == "":
		return ""
	}
	return filepath.Clean(filepath.Join(cwd, p))
}

func sizingDedupeProjection(share float64) string {
	if share >= sizingKeepBar {
		return "KEEP"
	}
	return "STOP"
}

func sizingSpawnProjection(share float64) string {
	if share >= sizingSpawnBar {
		return "BUILD"
	}
	return "STOP"
}

func sizingCompactionProjection(perDay, median float64) string {
	if perDay >= sizingPerDayBar && median >= sizingMedianBar {
		return "BUILD"
	}
	return "STOP"
}

// sizingMedian is the median of xs, 0 when empty.
func sizingMedian(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	m := len(s) / 2
	if len(s)%2 == 1 {
		return float64(s[m])
	}
	return float64(s[m-1]+s[m]) / 2
}

// report fills the three sections in a fixed order.
func (z *replaySizing) report(rep *replayReport) {
	d := z.dedupe
	d.Bytes = d.Bash.Bytes + d.Read.Bytes
	d.Share = shareOf(d.Repeat.Bytes, d.Bytes)
	d.RepeatShapes = shapeRows(z.shapes, d.Bytes)
	d.KeepBar, d.VerdictProjection, d.Note = sizingKeepBar, sizingDedupeProjection(d.Share), sizingDedupeNote
	rep.Dedupe = d

	s := z.spawns
	s.Workflow.Share = shareOf(int64(s.Workflow.DiscoveryFirst), int64(s.Workflow.Subagents))
	s.Other.Share = shareOf(int64(s.Other.DiscoveryFirst), int64(s.Other.Subagents))
	s.FirstCalls, s.Bar, s.VerdictProjection, s.Note = sizingFirstCalls, sizingSpawnBar, sizingSpawnProjection(s.Workflow.Share), sizingSpawnsNote
	rep.Spawns = s

	c := replayCompactions{Days: z.compactDays, PreTurns: sizingPreTurns, PostTurns: sizingPostTurns,
		PerEvent: []replayCompactionRow{}, PerDayBar: sizingPerDayBar, MedianBar: sizingMedianBar, Note: sizingCompactionsNote}
	sort.SliceStable(z.events, func(i, j int) bool { return z.events[i].ts.Before(z.events[j].ts) })
	var pre, reread []int
	for _, e := range z.events {
		c.PerEvent = append(c.PerEvent, e.row)
		if e.row.Complete {
			c.Complete++
			pre, reread = append(pre, e.row.PreSet), append(reread, e.row.Reread)
		}
	}
	c.Events = len(z.events)
	if c.Events > 0 {
		c.From, c.To = z.events[0].tsRaw, z.events[c.Events-1].tsRaw
	}
	c.PerDay = shareOf(int64(c.Events), int64(c.Days))
	c.MedianPreSet, c.MedianReread = sizingMedian(pre), sizingMedian(reread)
	c.VerdictProjection = sizingCompactionProjection(c.PerDay, c.MedianReread)
	rep.Compactions = c
}

// printReplaySizing is the text view's block for the three sections.
func printReplaySizing(w io.Writer, rep replayReport) {
	pct := func(f float64) string { return fmt.Sprintf("%.2f%%", f*100) }
	d := rep.Dedupe
	fmt.Fprintf(w, "dedupe: %d Bash results, %d bytes; %d Read results, %d bytes; repeats %d, %d bytes, %s of Bash plus Read bytes "+
		"(within a batch %d, %d bytes; across turns %d, %d bytes; Bash exact %d, Read exact %d, Read contained %d); "+
		"recovery repeats excluded %d, %d bytes; keep bar %s: projects %s\n",
		d.Bash.Results, d.Bash.Bytes, d.Read.Results, d.Read.Bytes, d.Repeat.Results, d.Repeat.Bytes, pct(d.Share),
		d.WithinBatch.Results, d.WithinBatch.Bytes, d.AcrossTurns.Results, d.AcrossTurns.Bytes,
		d.Bash.Exact.WithinBatch.Results+d.Bash.Exact.AcrossTurns.Results, d.Read.Exact.WithinBatch.Results+d.Read.Exact.AcrossTurns.Results,
		d.Read.Contained.WithinBatch.Results+d.Read.Contained.AcrossTurns.Results,
		d.Bash.RecoveryExcluded.Results, d.Bash.RecoveryExcluded.Bytes, pct(d.KeepBar), d.VerdictProjection)
	for _, r := range d.RepeatShapes {
		fmt.Fprintf(w, "  repeat %s  %d  %d  %s\n", r.Shape, r.Calls, r.Bytes, pct(r.Share))
	}
	s := rep.Spawns
	fmt.Fprintf(w, "spawns: workflow subagents %d, discovery-first %d (%s), fewer than %d calls %d; other subagents %d, discovery-first %d (%s), fewer than %d calls %d; bar %s: projects %s\n",
		s.Workflow.Subagents, s.Workflow.DiscoveryFirst, pct(s.Workflow.Share), s.FirstCalls, s.Workflow.Fewer,
		s.Other.Subagents, s.Other.DiscoveryFirst, pct(s.Other.Share), s.FirstCalls, s.Other.Fewer, pct(s.Bar), s.VerdictProjection)
	c := rep.Compactions
	fmt.Fprintf(w, "compactions over the latest %d days: %d events (%s to %s), %.2f a day, %d complete; median pre set %.1f files, median re-read %.1f files (%d turns before, %d after); bars %.0f a day and a median of %.0f: projects %s\n",
		c.Days, c.Events, readoutOrNone(c.From), readoutOrNone(c.To), c.PerDay, c.Complete, c.MedianPreSet, c.MedianReread,
		c.PreTurns, c.PostTurns, c.PerDayBar, c.MedianBar, c.VerdictProjection)
	for i, e := range c.PerEvent {
		state := "complete"
		if !e.Complete {
			state = "incomplete"
		}
		where := "session"
		if e.Subagent {
			where = "subagent"
		}
		fmt.Fprintf(w, "  compaction %d (%s, %s): pre set %d, re-read %d, %d post turns\n", i+1, where, state, e.PreSet, e.Reread, e.PostTurns)
	}
}
