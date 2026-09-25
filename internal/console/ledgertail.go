package console

// ledgertail.go — the event bus observes append-only ledgers (the verify
// ledger, the session ledger) by polling their size (design §4: "observed by
// polling the verify ledger's size every 2 s"). LedgerTail remembers how far
// it has read and returns only whole lines appended since.

import (
	"bytes"
	"io"
	"os"
)

// maxTailRead bounds one poll's read: a ledger that grew by more than this
// between two polls reports the newest bytes only (the event names that a
// source changed; it never needed every row).
const maxTailRead = 1 << 20

// LedgerTail follows one append-only JSONL file.
type LedgerTail struct {
	Path    string
	offset  int64
	primed  bool
	pending []byte
}

// Prime records the current end of the file without returning anything, so
// the first real poll reports only rows appended after startup.
func (t *LedgerTail) Prime() {
	if fi, err := os.Stat(t.Path); err == nil {
		t.offset = fi.Size()
	}
	t.primed = true
}

// Poll returns the complete lines appended since the last poll. A file that
// shrank (rotated or truncated) is read again from the start. A missing file
// returns nothing and no error — no ledger yet is not a failure.
func (t *LedgerTail) Poll() ([][]byte, error) {
	if !t.primed {
		t.Prime()
		return nil, nil
	}
	fi, err := os.Stat(t.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	size := fi.Size()
	if size < t.offset {
		t.offset, t.pending = 0, nil
	}
	if size == t.offset {
		return nil, nil
	}
	start := t.offset
	if size-start > maxTailRead {
		start, t.pending = size-maxTailRead, nil
	}
	f, err := os.Open(t.Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return nil, err
	}
	t.offset = size
	data := append(t.pending, buf...)
	last := bytes.LastIndexByte(data, '\n')
	if last < 0 {
		t.pending = data
		return nil, nil
	}
	t.pending = append([]byte(nil), data[last+1:]...)
	var lines [][]byte
	for _, line := range bytes.Split(data[:last], []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) > 0 {
			lines = append(lines, line)
		}
	}
	return lines, nil
}
