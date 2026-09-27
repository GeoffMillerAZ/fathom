package main

// console_tails.go — the observatory model's reads of the call log and the
// session ledger as tails (RT-113, docs/design/responsible-tenant.md §14).
// The model rebuilt every 30 s by re-reading and re-decoding the whole call
// log (tens of megabytes) and the whole session ledger; the Operate
// projection read the log whole again. Both logs are append-only, so each
// reader here keeps what it read and, on the next read, reads only the
// bytes appended since — decoded by the same line reading every usage
// reader shares (decodeCallLogLine), so a tail never counts differently
// from a whole-file read.
//
// A file replaced (another inode), truncated, or rewritten in place (the
// last bytes read no longer match) is read again from the start. A file
// that cannot be opened now is an error now — the external volume's wedge
// makes stat succeed and open fail — never the content of an earlier read.

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"
)

// tailCheckBytes is how much of what was read last is compared on the next
// read, to notice a file rewritten in place with the same size.
const tailCheckBytes = 256

// tailFile is one followed file's identity and read position.
type tailFile struct {
	path   string
	info   os.FileInfo
	offset int64
	check  []byte // the last bytes consumed (≤ tailCheckBytes)
}

// open stats and opens path and reports whether the read must start over.
func (f *tailFile) open(path string) (*os.File, os.FileInfo, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, false, err
	}
	fh, err := os.Open(path)
	if err != nil {
		return nil, nil, false, fmt.Errorf("%w%s", err, tccWedgeHint(path))
	}
	restart := f.path != path || f.info == nil || !os.SameFile(f.info, info) || info.Size() < f.offset
	if !restart && len(f.check) > 0 {
		buf := make([]byte, len(f.check))
		if _, err := fh.ReadAt(buf, f.offset-int64(len(f.check))); err != nil || !bytes.Equal(buf, f.check) {
			restart = true
		}
	}
	return fh, info, restart, nil
}

// consumed records that the bytes b, ending at the new offset, were read.
func (f *tailFile) consumed(path string, info os.FileInfo, b []byte) {
	f.path, f.info = path, info
	f.offset += int64(len(b))
	f.keep(b)
}

// keep appends b to the check window, keeping its last tailCheckBytes.
func (f *tailFile) keep(b []byte) {
	if len(b) >= tailCheckBytes {
		f.check = append(f.check[:0], b[len(b)-tailCheckBytes:]...)
		return
	}
	f.check = append(f.check, b...)
	if n := len(f.check); n > tailCheckBytes {
		f.check = append(f.check[:0], f.check[n-tailCheckBytes:]...)
	}
}

// bytesTail follows an append-only file and returns its whole content,
// reading only what was appended since the last read.
type bytesTail struct {
	mu   sync.Mutex
	f    tailFile
	data []byte
}

// read returns the file's content (shared: callers must not modify it).
func (t *bytesTail) read(path string) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fh, info, restart, err := t.f.open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	if restart {
		t.f, t.data = tailFile{}, nil
	}
	if info.Size() == t.f.offset {
		t.f.path, t.f.info = path, info
		return t.data, nil
	}
	if _, err := fh.Seek(t.f.offset, io.SeekStart); err != nil {
		return nil, err
	}
	more, err := io.ReadAll(fh)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	// A new backing array on growth: a slice handed out earlier is never
	// written under its reader.
	t.data = append(slices.Clip(t.data), more...)
	t.f.consumed(path, info, more)
	return t.data, nil
}

func (t *bytesTail) reset() {
	t.mu.Lock()
	t.f, t.data = tailFile{}, nil
	t.mu.Unlock()
}

// callLogEntryTail follows the call log with callLogReadCounting's
// answer: every counted entry, the malformed lines and the console's own
// reads left out.
type callLogEntryTail struct {
	mu                    sync.Mutex
	f                     tailFile
	entries               []adoptionCallEntry
	skipped, consoleReads int
	// parsed counts the bytes decoded over the tail's life (tests).
	parsed int64
}

// read returns what callLogReadCounting(path) returns. The entries are a
// copy the caller owns.
func (t *callLogEntryTail) read(path string) ([]adoptionCallEntry, int, int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fh, info, restart, err := t.f.open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer fh.Close()
	if restart {
		t.f, t.entries, t.skipped, t.consoleReads = tailFile{}, nil, 0, 0
	}
	if _, err := fh.Seek(t.f.offset, io.SeekStart); err != nil {
		return nil, 0, 0, fmt.Errorf("read call log %s: %w", path, err)
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
				return nil, 0, 0, fmt.Errorf("read call log %s: a line longer than 4 MiB", path)
			}
			line = long
		}
		if rerr == io.EOF {
			tail = append([]byte(nil), line...)
			break
		}
		if rerr != nil {
			return nil, 0, 0, fmt.Errorf("read call log %s: %w", path, rerr)
		}
		t.parsed += int64(len(line))
		t.count(line)
		t.f.offset += int64(len(line))
		t.f.keep(line)
	}
	t.f.path, t.f.info = path, info
	out := slices.Clone(t.entries)
	skipped, consoleReads := t.skipped, t.consoleReads
	// An unterminated last line counts as a whole-file read counts it,
	// and is read again next time (it may still grow).
	if len(tail) > 0 {
		switch e, kind := decodeCallLogLine(tail); kind {
		case callLogLineMalformed:
			skipped++
		case callLogLineConsole:
			consoleReads++
		case callLogLineEntry:
			out = append(out, e)
		}
	}
	return out, skipped, consoleReads, nil
}

func (t *callLogEntryTail) count(line []byte) {
	switch e, kind := decodeCallLogLine(line); kind {
	case callLogLineMalformed:
		t.skipped++
	case callLogLineConsole:
		t.consoleReads++
	case callLogLineEntry:
		t.entries = append(t.entries, e)
	}
}

func (t *callLogEntryTail) reset() {
	t.mu.Lock()
	t.f, t.entries, t.skipped, t.consoleReads, t.parsed = tailFile{}, nil, 0, 0, 0
	t.mu.Unlock()
}

// The model's and the Operate projection's tails: one per source.
var (
	consoleCallLogTail callLogEntryTail
	sessionLedgerTail  bytesTail
)

// resetConsoleTails forgets every tail (tests; the governor's drops).
func resetConsoleTails() {
	consoleCallLogTail.reset()
	sessionLedgerTail.reset()
}
