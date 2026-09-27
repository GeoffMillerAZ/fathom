package main

// transcript_offset.go — the Stop hook reads the transcript from a
// per-session offset stamp (docs/design/responsible-tenant.md §11.3,
// RT-132). A Stop fires every turn; re-reading up to 16 MB of a long
// session's transcript each time was a per-turn cost that grew with the
// session. The stamp holds the offset folded so far and the fold itself
// (transcriptFold), so each Stop reads only what the transcript gained and
// the summary equals a whole-file read's. A stamp whose file changed
// identity or shrank is discarded and the transcript is folded from 0.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// transcriptOffsetStamp is the per-session stamp.
type transcriptOffsetStamp struct {
	SchemaVersion int             `json:"schema_version"`
	Path          string          `json:"path"`
	Dev           uint64          `json:"dev"`
	Ino           uint64          `json:"ino"`
	Offset        int64           `json:"offset"` // bytes folded, always at a line boundary
	Fold          *transcriptFold `json:"fold"`
}

// transcriptStampPath is where sessionID's offset stamp lives ("" when no
// stamp directory resolves).
func transcriptStampPath(sessionID string) string {
	dir := sessionStampDir()
	if dir == "" || sessionID == "" {
		return ""
	}
	h := sha256.Sum256([]byte(sessionID))
	return filepath.Join(dir, "transcript-"+hex.EncodeToString(h[:8])+".json")
}

// readTranscriptFromOffset summarizes the transcript at path for
// sessionID, reading only the bytes after the session's stamp. It keeps
// transcriptSummaryFrom's hardening (no symlink, no FIFO, a bounded read)
// and its fixed-string notes. A read that stops at the byte cap before
// the end of the file is Truncated; the next Stop continues from there.
func readTranscriptFromOffset(sessionID, path string) transcriptSummary {
	stampPath := transcriptStampPath(sessionID)
	if stampPath == "" || path == "" {
		return transcriptSummaryFrom(path)
	}
	const unavailableUnreadable = "unavailable: transcript_path unreadable or not a regular file"
	unreadable := transcriptSummary{DurationNote: unavailableUnreadable, TurnNote: unavailableUnreadable, TokensNote: unavailableUnreadable}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return unreadable
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return unreadable
	}
	var dev, ino uint64
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		dev, ino = uint64(st.Dev), uint64(st.Ino)
	}

	stamp := transcriptOffsetStamp{SchemaVersion: 1, Path: path, Dev: dev, Ino: ino, Fold: newTranscriptFold()}
	if b, err := os.ReadFile(stampPath); err == nil {
		var prev transcriptOffsetStamp
		if json.Unmarshal(b, &prev) == nil && prev.SchemaVersion == 1 && prev.Path == path && prev.Dev == dev && prev.Ino == ino &&
			prev.Offset <= fi.Size() && prev.Fold != nil {
			if prev.Fold.Uses == nil {
				prev.Fold.Uses = map[string]foldUse{}
			}
			if prev.Fold.ResultsSeen == nil {
				prev.Fold.ResultsSeen = map[string]int{}
			}
			stamp = prev
		}
	}
	if _, err := f.Seek(stamp.Offset, io.SeekStart); err != nil {
		return unreadable
	}
	data, err := io.ReadAll(io.LimitReader(f, transcriptReadMaxBytes+1))
	if err != nil {
		return unreadable
	}
	truncated := false
	if int64(len(data)) > transcriptReadMaxBytes {
		data = data[:transcriptReadMaxBytes]
		truncated = true
	}
	// Fold every complete line into the stamp; an unterminated tail (a
	// write in progress, or the cap's cut) is folded into a copy for this
	// summary only and read again next time.
	complete := data
	var tail []byte
	if i := lastNewline(data); i < len(data)-1 {
		complete, tail = data[:i+1], data[i+1:]
	}
	for _, l := range splitLines(complete) {
		stamp.Fold.add(l, nil)
	}
	stamp.Offset += int64(len(complete))
	view := stamp.Fold
	if len(tail) > 0 && !truncated {
		view = stamp.Fold.clone()
		view.add(tail, nil)
	}
	out := view.summary()
	out.Truncated = truncated
	if err := writeTranscriptStamp(stampPath, stamp); err != nil {
		// A stamp that cannot be written costs the next Stop a full read,
		// never a wrong summary.
		_ = err
	}
	return out
}

func lastNewline(b []byte) int {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == '\n' {
			return i
		}
	}
	return -1
}

func writeTranscriptStamp(path string, st transcriptOffsetStamp) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d.%d", path, os.Getpid(), time.Now().UnixNano())
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
