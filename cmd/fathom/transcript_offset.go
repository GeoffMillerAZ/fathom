package main

// transcript_offset.go — the Stop hook reads the transcript from a
// per-session offset stamp (docs/design/responsible-tenant.md §11.3,
// RT-132). A Stop fires every turn; re-reading up to 16 MB of a long
// session's transcript each time was a per-turn cost that grew with the
// session. The stamp holds the offset folded so far and the fold itself
// (transcriptFold), so each Stop reads only what the transcript gained and
// the summary equals a whole-file read's. A stamp whose file changed
// identity or shrank is discarded and the transcript is folded from 0.
//
// A Codex session (PE-14) keeps its rollout fold (harness.CodexCostFold)
// in the same stamp: the harness port's fold resumes itself from its own
// offset, so the stamp only carries it between Stops.

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

	"github.com/geoffmilleraz/fathom/internal/harness"
)

// transcriptOffsetStamp is the per-session stamp.
type transcriptOffsetStamp struct {
	SchemaVersion int             `json:"schema_version"`
	Path          string          `json:"path"`
	Dev           uint64          `json:"dev"`
	Ino           uint64          `json:"ino"`
	Offset        int64           `json:"offset"` // bytes folded, always at a line boundary
	Fold          *transcriptFold `json:"fold"`
	// Codex is a Codex session's rollout fold (PE-14); nil for a Claude
	// Code session, whose fold is Fold.
	Codex *harness.CodexCostFold `json:"codex,omitempty"`
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

// codexSchemaNote is a Codex row's transcript_schema_note.
const codexSchemaNote = "figures above are folded from the Codex rollout (transcript_path) by the harness port " +
	"(internal/harness CodexSessionCost) — schema-coupled and may silently degrade to \"unavailable\" (never a " +
	"wrong number) if that format changes; turns, duration and the exploration ledger are not read from a rollout"

// readCodexRolloutFromOffset summarizes a Codex session's rollout at path
// for the Stop hook (PE-14): harness.CodexSessionCost folds what the
// rollout gained since the fold kept in the session's offset stamp, and
// the returned fold replaces it there. Tokens, cache, tool calls and the
// model fill the same summary fields a Claude Code transcript fills; what
// a rollout fold does not read is a fixed-string note, never a zero.
func readCodexRolloutFromOffset(sessionID, path string) transcriptSummary {
	const unavailableNoPath = "unavailable: no transcript_path on this Stop hook invocation"
	const unavailableUnreadable = "unavailable: transcript_path unreadable or not a regular file"
	if path == "" {
		return transcriptSummary{DurationNote: unavailableNoPath, TurnNote: unavailableNoPath, TokensNote: unavailableNoPath}
	}
	// The same refusal the fold makes (no symlink, only a regular file),
	// asked first so an unreadable rollout is named rather than read as a
	// rollout with nothing in it.
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return transcriptSummary{DurationNote: unavailableUnreadable, TurnNote: unavailableUnreadable, TokensNote: unavailableUnreadable}
	}
	stampPath := transcriptStampPath(sessionID)
	var prev *harness.CodexCostFold
	if stampPath != "" {
		if b, err := os.ReadFile(stampPath); err == nil {
			var st transcriptOffsetStamp
			if json.Unmarshal(b, &st) == nil && st.SchemaVersion == 1 && st.Path == path && st.Codex != nil {
				prev = st.Codex
			}
		}
	}
	c, fold := harness.CodexSessionCost(path, prev)
	if stampPath != "" {
		// A stamp that cannot be written costs the next Stop a fold from
		// the start, never a wrong summary.
		_ = writeTranscriptStamp(stampPath, transcriptOffsetStamp{SchemaVersion: 1, Path: path, Dev: fold.Dev, Ino: fold.Ino, Offset: fold.Offset, Codex: &fold})
	}
	return codexSummary(c)
}

// codexSummary maps a rollout's SessionCost onto the summary fields a
// Claude Code transcript fills.
func codexSummary(c harness.SessionCost) transcriptSummary {
	const notFolded = "unavailable: a Codex rollout's turns and duration are not folded"
	out := transcriptSummary{
		Parsed:          true,
		DurationNote:    notFolded,
		TurnNote:        notFolded,
		FirstEditNote:   "unavailable: a Codex rollout's edits are not folded",
		ExplorationNote: "not measured: the exploration ledger reads Claude Code transcripts only",
		ToolCallsTotal:  c.ToolCalls,
		Model:           c.Model,
		Truncated:       c.Truncated,
		SchemaNote:      codexSchemaNote,
	}
	if c.Note != "" && c.Available {
		out.SchemaNote += "; " + c.Note
	}
	if !c.Available {
		out.TokensNote = c.Note
		if out.TokensNote == "" {
			out.TokensNote = "unavailable: no usage in the Codex rollout"
		}
		return out
	}
	in, o, cr, cc := c.TokensIn, c.TokensOut, c.TokensCacheRead, c.TokensCacheCreate
	total := in + o + cr + cc
	out.TokensTotal = &total
	out.TokensIn, out.TokensOut, out.TokensCacheRead, out.TokensCacheCreate = &in, &o, &cr, &cc
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
