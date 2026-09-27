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
// partial — readHooksGenerations' contract.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// beatIndexSessions is how many sessions' lines stay cached.
const beatIndexSessions = 64

// beatGen is one hooks-log file's index.
type beatGen struct {
	f       tailFile
	offsets map[string][]int64 // session → offsets of its complete lines
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
	if path == "" {
		return nil, nil, os.ErrNotExist
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.gens == nil {
		ix.gens, ix.lru = map[string]*beatGen{}, map[string]beatLRUEntry{}
	}
	type opened struct {
		fh   *os.File
		g    *beatGen
		tail []byte // an unterminated last line, not indexed
	}
	var files [2]opened
	missing := 0
	marks := make([]string, 0, 2)
	for i, p := range []string{path + ".1", path} {
		fh, g, tail, ferr := ix.extend(p, path)
		switch {
		case ferr == nil:
			files[i] = opened{fh: fh, g: g, tail: tail}
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
	defer func() {
		for _, o := range files {
			if o.fh != nil {
				o.fh.Close()
			}
		}
	}()
	if err != nil {
		return nil, nil, err
	}
	if missing == 2 {
		return nil, nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	if partial != nil && missing == 1 {
		return nil, nil, errors.Unwrap(partial)
	}
	mark := strings.Join(marks, "|")
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

// extend opens p and indexes what was appended since its last read. A
// rotated generation that is the file the current path named before is
// that file's index, moved.
func (ix *beatOffsetIndex) extend(p, current string) (*os.File, *beatGen, []byte, error) {
	g := ix.gens[p]
	if g == nil {
		g = &beatGen{offsets: map[string][]int64{}}
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
			g = &beatGen{offsets: map[string][]int64{}}
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
		if s, ok := beatSession(line); ok {
			g.offsets[s] = append(g.offsets[s], g.f.offset)
		}
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

// reset forgets the index (tests; the governor's drops).
func (ix *beatOffsetIndex) reset() {
	ix.mu.Lock()
	ix.gens, ix.lru = nil, nil
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
