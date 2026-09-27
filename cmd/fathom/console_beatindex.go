package main

// console_beatindex.go — session pages over a per-session offset index into
// the hooks log (RT-114, docs/design/responsible-tenant.md §14). A session
// replay and the legacy /observatory/session story used to read hooks.jsonl
// (and its rotated generation) whole on every view — the log grows with
// every edit and holds up to two 64 MiB generations. The index follows each
// generation as a tail: it reads only what was appended since its last
// read, recording each complete line's offset under the session the line
// names, so a page reads its session's lines by offset. A rotation (the
// current file renamed to hooks.jsonl.1) carries the file's index with it.
// The index is bounded by the log's own rotation cap (an offset per beat,
// for at most two generations); the lines of the 64 most recently read
// sessions are kept, keyed by both generations' watermarks.
//
// A generation that cannot be read is an error, never "no beats": the
// current one failing fails the read, the rotated one failing marks it
// partial.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// beatIndexSessions is how many sessions' lines stay cached.
const beatIndexSessions = 64

// beatGen is one hooks-log file's index.
type beatGen struct {
	f       tailFile
	offsets map[string][]int64 // session → offsets of its complete lines
	// The digests every other console reader of the hooks log reads, kept
	// as the lines are indexed so none of them reads the log whole: the
	// guard hook's pre beats (the catches timeline), each (session,
	// project) beat-window (the impact model's derived commits) and each
	// project's latest custodian audit (the flywheel).
	guard     []beatGuard
	spans     map[beatSpanKey]beatSpan
	custodian map[string]time.Time
}

// beatGuard is one PreToolUse edit-hook beat, before its project is named
// as a service (the reader's labelKey does that).
type beatGuard struct {
	at                                time.Time
	kind, target, projectDir, session string
}

type beatSpanKey struct{ session, projectDir string }

// beatSpan is the first and last beat time of one session in one project.
type beatSpan struct{ start, end time.Time }

func newBeatGen() *beatGen {
	return &beatGen{offsets: map[string][]int64{}, spans: map[beatSpanKey]beatSpan{}, custodian: map[string]time.Time{}}
}

var (
	beatPreNeedle       = []byte(`"phase":"pre"`)
	beatEditHookNeedle  = []byte(`"edit-hook"`)
	beatCustodianNeedle = []byte(`"custodian"`)
)

// digest folds one line into the generation's digests. A line is decoded
// only when it can matter to one: it names a session, or it is a guard or
// custodian beat. The decode is readRetroBeats' (a beat with a channel).
func (g *beatGen) digest(line []byte, hasSession bool) {
	guard := bytes.Contains(line, beatPreNeedle) && bytes.Contains(line, beatEditHookNeedle)
	cust := bytes.Contains(line, beatCustodianNeedle)
	if !hasSession && !guard && !cust {
		return
	}
	b, ok := decodeRetroBeat(line)
	if !ok {
		return
	}
	g.fold(b)
}

// fold adds one decoded beat to the digests.
func (g *beatGen) fold(b retroBeat) {
	if b.Channel == "edit-hook" && b.Meta["phase"] == "pre" {
		g.guard = append(g.guard, beatGuard{at: b.At, kind: b.Kind, target: b.Meta["target"], projectDir: b.Meta["project_dir"], session: b.Meta["session"]})
	}
	if sid := b.Meta["session"]; sid != "" {
		k := beatSpanKey{sid, b.Meta["project_dir"]}
		sp, seen := g.spans[k]
		if !seen || b.At.Before(sp.start) {
			sp.start = b.At
		}
		if !seen || b.At.After(sp.end) {
			sp.end = b.At
		}
		g.spans[k] = sp
	}
	if b.Channel == "custodian" && (b.Kind == "audit" || b.Kind == "attention") {
		dir := b.Meta["project_dir"]
		if b.At.After(g.custodian[dir]) {
			g.custodian[dir] = b.At
		}
	}
}

// decodeRetroBeat is readRetroBeats' decode of one line.
func decodeRetroBeat(line []byte) (retroBeat, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return retroBeat{}, false
	}
	var b retroBeat
	if json.Unmarshal(line, &b) != nil || b.Channel == "" {
		return retroBeat{}, false
	}
	return b, true
}

// beatOffsetIndex indexes the hooks log's generations by session.
type beatOffsetIndex struct {
	mu   sync.Mutex
	gens map[string]*beatGen // by file path
	lru  map[string]beatLRUEntry
	tick uint64
	// parsed counts the bytes indexed over the index's life (tests).
	parsed int64
}

type beatLRUEntry struct {
	mark  string
	lines [][]byte
	used  uint64
}

// hooksBeatIndex is the daemon's one index of the hooks log.
var hooksBeatIndex = &beatOffsetIndex{}

// sessionBeatIndex is one session's beat lines from the hooks log at path
// and its rotated generation, read by offset through the daemon's index —
// what every session page reads (the replay, the legacy story).
func sessionBeatIndex(path, session string) (lines [][]byte, partial, err error) {
	return hooksBeatIndex.lines(path, session)
}

// lines returns session's beat lines from path's rotated generation
// (older) and then path, each in file order. partial names a rotated
// generation that could not be read while the current one was.
func (ix *beatOffsetIndex) lines(path, session string) (out [][]byte, partial, err error) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	files, mark, partial, err := ix.refresh(path)
	defer files.close()
	if err != nil {
		return nil, nil, err
	}
	ix.tick++
	if e, ok := ix.lru[session]; ok && e.mark == mark {
		e.used = ix.tick
		ix.lru[session] = e
		return e.lines, partial, nil
	}
	for _, o := range files {
		if o.fh == nil {
			continue
		}
		for _, off := range o.g.offsets[session] {
			line, rerr := readLineAt(o.fh, off)
			if rerr != nil {
				return nil, nil, fmt.Errorf("read hooks log at %d: %w", off, rerr)
			}
			out = append(out, line)
		}
		if len(o.tail) > 0 {
			if s, ok := beatSession(o.tail); ok && s == session {
				out = append(out, o.tail)
			}
		}
	}
	ix.remember(session, mark, out)
	return out, partial, nil
}

// beatGenRead is one generation as refresh left it: open, indexed to its
// last complete line, and its unterminated last line (not indexed).
type beatGenRead struct {
	fh   *os.File
	g    *beatGen
	tail []byte
}

// beatGenReads are the rotated generation (older) and the current one; an
// entry with no file is a generation that does not exist or failed.
type beatGenReads [2]beatGenRead

func (r beatGenReads) close() {
	for _, o := range r {
		if o.fh != nil {
			o.fh.Close()
		}
	}
}

// refresh indexes what was appended to path's rotated generation and to
// path itself. mark is both generations' watermarks. err is the current
// generation's failure, or not-exist when neither exists; partial names a
// rotated generation that failed while the current one was read —
// the caller holds ix.mu and closes files.
func (ix *beatOffsetIndex) refresh(path string) (files beatGenReads, mark string, partial, err error) {
	if path == "" {
		return files, "", nil, os.ErrNotExist
	}
	if ix.gens == nil {
		ix.gens, ix.lru = map[string]*beatGen{}, map[string]beatLRUEntry{}
	}
	missing := 0
	marks := make([]string, 0, 2)
	for i, p := range []string{path + ".1", path} {
		fh, g, tail, ferr := ix.extend(p, path)
		switch {
		case ferr == nil:
			files[i] = beatGenRead{fh: fh, g: g, tail: tail}
			marks = append(marks, p+"@"+strconv.FormatInt(g.f.offset, 10)+"+"+strconv.Itoa(len(tail)))
		case errors.Is(ferr, os.ErrNotExist):
			missing++
			marks = append(marks, p+"@-")
		case i == 0:
			partial = fmt.Errorf("the rotated hooks log could not be read: %w", ferr)
			marks = append(marks, p+"@err")
		default:
			err = ferr
		}
	}
	switch {
	case err != nil:
		return files, "", nil, err
	case missing == 2:
		return files, "", nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	case partial != nil && missing == 1:
		return files, "", nil, errors.Unwrap(partial)
	}
	return files, strings.Join(marks, "|"), partial, nil
}

// guardBeats is readGuardBeats over the index: the guard hook's pre beats
// from the rotated generation and then the current one, in file order,
// each project named as a service through labelKey.
func (ix *beatOffsetIndex) guardBeats(path string, labelKey func(string) string) (beats []console.GuardBeat, partial, err error) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	files, _, partial, err := ix.refresh(path)
	defer files.close()
	if err != nil {
		return nil, nil, err
	}
	add := func(r beatGuard) {
		svc := ""
		if r.projectDir != "" && labelKey != nil {
			svc = labelKey(filepath.Base(r.projectDir))
		}
		beats = append(beats, console.GuardBeat{At: r.at, Kind: r.kind, Target: r.target, Service: svc, Session: r.session})
	}
	for _, o := range files {
		if o.g == nil {
			continue
		}
		for _, r := range o.g.guard {
			add(r)
		}
		if tail := tailDigest(o.tail); tail != nil {
			for _, r := range tail.guard {
				add(r)
			}
		}
	}
	return beats, partial, nil
}

// tailDigest is the digest of an unterminated last line (a whole-file
// read counts it; the index reads it again next time), nil when it adds
// nothing.
func tailDigest(tail []byte) *beatGen {
	if len(tail) == 0 {
		return nil
	}
	_, hasSession := beatSession(tail)
	g := newBeatGen()
	g.digest(tail, hasSession)
	return g
}

// currentGen is the current generation's digest with its unterminated
// last line folded in — what readRetroBeats(path) reads (the current file
// only). nil when the log does not exist or cannot be read (readRetroBeats'
// callers read either as no beats).
func (ix *beatOffsetIndex) currentGen(path string) *beatGen {
	files, _, _, err := ix.refresh(path)
	defer files.close()
	cur := files[1]
	if err != nil || cur.g == nil {
		return nil
	}
	t := tailDigest(cur.tail)
	if t == nil {
		return cur.g
	}
	merged := &beatGen{spans: maps.Clone(cur.g.spans), custodian: maps.Clone(cur.g.custodian)}
	for k, sp := range t.spans {
		m, seen := merged.spans[k]
		if !seen || sp.start.Before(m.start) {
			m.start = sp.start
		}
		if !seen || sp.end.After(m.end) {
			m.end = sp.end
		}
		merged.spans[k] = m
	}
	for dir, at := range t.custodian {
		if at.After(merged.custodian[dir]) {
			merged.custodian[dir] = at
		}
	}
	return merged
}

// sessionSpan is the session's beat-window in root, from the current
// generation (derivedCommitsForSession's input, without a whole read).
func (ix *beatOffsetIndex) sessionSpan(path, sid, root string) (start, end time.Time, ok bool) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	g := ix.currentGen(path)
	if g == nil {
		return time.Time{}, time.Time{}, false
	}
	sp, ok := g.spans[beatSpanKey{sid, root}]
	return sp.start, sp.end, ok
}

// lastCustodianAudit is root's latest custodian audit or attention beat in
// the current generation (zero when none).
func (ix *beatOffsetIndex) lastCustodianAudit(path, root string) time.Time {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	g := ix.currentGen(path)
	if g == nil {
		return time.Time{}
	}
	return g.custodian[root]
}

// extend opens p and indexes what was appended since its last read. A
// rotated generation that is the file the current path named before is
// that file's index, moved.
func (ix *beatOffsetIndex) extend(p, current string) (*os.File, *beatGen, []byte, error) {
	g := ix.gens[p]
	if g == nil {
		g = newBeatGen()
	}
	fh, info, restart, err := g.f.open(p)
	if err != nil {
		return nil, nil, nil, err
	}
	if restart {
		if cur := ix.gens[current]; p != current && cur != nil && cur.f.info != nil && os.SameFile(cur.f.info, info) {
			// The current file was rotated to p: its index moves with it.
			g = cur
			delete(ix.gens, current)
			g.f.path = p
		} else {
			g = newBeatGen()
		}
		ix.gens[p] = g
	}
	ix.gens[p] = g
	if _, err := fh.Seek(g.f.offset, io.SeekStart); err != nil {
		fh.Close()
		return nil, nil, nil, err
	}
	br := bufio.NewReaderSize(fh, 256*1024)
	var tail []byte
	for {
		line, rerr := br.ReadSlice('\n')
		if rerr == bufio.ErrBufferFull {
			long := append([]byte(nil), line...)
			for rerr == bufio.ErrBufferFull && len(long) <= 4*1024*1024 {
				line, rerr = br.ReadSlice('\n')
				long = append(long, line...)
			}
			if rerr == bufio.ErrBufferFull {
				fh.Close()
				return nil, nil, nil, fmt.Errorf("a hooks-log line longer than 4 MiB at %d", g.f.offset)
			}
			line = long
		}
		if rerr == io.EOF {
			tail = append([]byte(nil), line...)
			break
		}
		if rerr != nil {
			fh.Close()
			return nil, nil, nil, rerr
		}
		s, hasSession := beatSession(line)
		if hasSession {
			g.offsets[s] = append(g.offsets[s], g.f.offset)
		}
		g.digest(line, hasSession)
		ix.parsed += int64(len(line))
		g.f.offset += int64(len(line))
		g.f.keep(line)
	}
	g.f.path, g.f.info = p, info
	return fh, g, tail, nil
}

// remember caches a session's lines, evicting the least recently read
// past beatIndexSessions.
func (ix *beatOffsetIndex) remember(session, mark string, lines [][]byte) {
	ix.lru[session] = beatLRUEntry{mark: mark, lines: lines, used: ix.tick}
	for len(ix.lru) > beatIndexSessions {
		oldest, at := "", ^uint64(0)
		for k, e := range ix.lru {
			if e.used < at || (e.used == at && k < oldest) {
				oldest, at = k, e.used
			}
		}
		delete(ix.lru, oldest)
	}
}

// reset forgets the index (tests).
func (ix *beatOffsetIndex) reset() {
	ix.mu.Lock()
	ix.gens, ix.lru = nil, nil
	ix.mu.Unlock()
}

// dropLines frees the cached session lines, keeping the offsets and
// digests (the governor's shed: the next read seeks, never re-indexes).
func (ix *beatOffsetIndex) dropLines() {
	ix.mu.Lock()
	ix.lru = map[string]beatLRUEntry{}
	ix.mu.Unlock()
}

// readLineAt reads the line starting at off (without its newline).
func readLineAt(f *os.File, off int64) ([]byte, error) {
	br := bufio.NewReaderSize(io.NewSectionReader(f, off, 4*1024*1024+1), 4096)
	var out []byte
	for {
		chunk, err := br.ReadSlice('\n')
		out = append(out, chunk...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && err != io.EOF {
			return nil, err
		}
		return bytes.TrimRight(out, "\n"), nil
	}
}

var beatSessionKey = []byte(`"session":"`)

// beatSession is the session id a hooks-log line's meta names.
func beatSession(line []byte) (string, bool) {
	i := bytes.Index(line, beatSessionKey)
	if i < 0 {
		return "", false
	}
	rest := line[i+len(beatSessionKey):]
	j := bytes.IndexByte(rest, '"')
	if j < 0 {
		return "", false
	}
	id := rest[:j]
	if bytes.IndexByte(id, '\\') < 0 {
		return string(id), true
	}
	var b struct {
		Meta map[string]string `json:"meta"`
	}
	if json.Unmarshal(bytes.TrimSpace(line), &b) != nil || b.Meta["session"] == "" {
		return "", false
	}
	return b.Meta["session"], true
}
