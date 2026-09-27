package main

// calllog_rotate.go — the call log's size-triggered rotation (docs/design/
// responsible-tenant.md §12.1, RT-126). At 64 MiB the live log becomes the
// rotated generation <log>.1 (which console tails read), and the previous
// generation is rolled up (<log>.<ts>.rollup.json: rows by day, tool and
// outcome), gzipped to <log>.<ts>.jsonl.gz and only then removed — its
// bytes live on in the archive. Archives are never deleted: past 1 GiB of
// them the rotation reports it (the serve-http log, doctor disk_retention,
// `fathom gc`) and keeps them all.

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// Call-log retention bounds (§12.1).
const (
	callLogRotateBytes      = 64 << 20
	callLogArchiveWarnBytes = 1 << 30
)

// callLogRotation is what one rotation did.
type callLogRotation struct {
	Rotated  string // the rotated generation (<log>.1)
	Archive  string // the gzip archive of the previous generation ("" when none)
	Rollup   string // its rollup
	Archives int64  // bytes of every archive after this rotation
	Report   string // non-empty when the archives are over callLogArchiveWarnBytes
}

// rotateCallLog rotates path: the previous generation is rolled up and
// archived, the live log becomes <path>.1. now names the archive.
func rotateCallLog(path string, now time.Time) (callLogRotation, error) {
	rot := callLogRotation{Rotated: path + ".1"}
	if _, err := os.Stat(rot.Rotated); err == nil {
		ts := now.UTC().Format("20060102T150405Z")
		rot.Archive = path + "." + ts + ".jsonl.gz"
		rot.Rollup = path + "." + ts + ".rollup.json"
		if err := writeCallLogRollup(rot.Rotated, rot.Rollup); err != nil {
			return rot, fmt.Errorf("roll up %s: %w", rot.Rotated, err)
		}
		if err := gzipFile(rot.Rotated, rot.Archive); err != nil {
			return rot, fmt.Errorf("archive %s: %w", rot.Rotated, err)
		}
		// Archived: the generation's bytes are in rot.Archive.
		if err := os.Remove(rot.Rotated); err != nil {
			return rot, err
		}
	} else if !os.IsNotExist(err) {
		return rot, err
	}
	if err := os.Rename(path, rot.Rotated); err != nil && !os.IsNotExist(err) {
		return rot, err
	}
	rot.Archives = callLogArchiveBytes(path)
	if rot.Archives > callLogArchiveWarnBytes {
		rot.Report = fmt.Sprintf("call-log archives hold %s, over %s — nothing is deleted; move or compress them elsewhere (%s.*.jsonl.gz)",
			tenancy.FormatBytes(rot.Archives), tenancy.FormatBytes(callLogArchiveWarnBytes), path)
	}
	return rot, nil
}

// callLogArchives lists path's gzip archives, oldest first.
func callLogArchives(path string) []string {
	m, _ := filepath.Glob(path + ".*.jsonl.gz")
	sort.Strings(m)
	return m
}

func callLogArchiveBytes(path string) int64 {
	var n int64
	for _, p := range callLogArchives(path) {
		if st, err := os.Stat(p); err == nil {
			n += st.Size()
		}
	}
	return n
}

// gzipFile streams src into dst (written to a temp name and renamed, so a
// half-written archive never looks complete).
func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(out)
	if _, err := io.Copy(zw, in); err != nil {
		zw.Close()
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := zw.Close(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// callLogRollup is one archived generation's summary.
type callLogRollup struct {
	SchemaVersion int                       `json:"schema_version"`
	Source        string                    `json:"source"`
	Rows          int                       `json:"rows"`
	Unparsed      int                       `json:"unparsed"`
	First         string                    `json:"first,omitempty"`
	Last          string                    `json:"last,omitempty"`
	ByDay         map[string]map[string]int `json:"by_day"` // day → tool → rows
	ByOutcome     map[string]int            `json:"by_outcome"`
}

func writeCallLogRollup(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	r := callLogRollup{SchemaVersion: 1, Source: filepath.Base(src), ByDay: map[string]map[string]int{}, ByOutcome: map[string]int{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e struct {
			TS      string `json:"ts"`
			Tool    string `json:"tool"`
			Outcome string `json:"outcome"`
		}
		if json.Unmarshal([]byte(line), &e) != nil {
			r.Unparsed++
			continue
		}
		r.Rows++
		day := "unknown"
		if len(e.TS) >= 10 {
			day = e.TS[:10]
			if r.First == "" || e.TS < r.First {
				r.First = e.TS
			}
			if e.TS > r.Last {
				r.Last = e.TS
			}
		}
		if r.ByDay[day] == nil {
			r.ByDay[day] = map[string]int{}
		}
		r.ByDay[day][e.Tool]++
		r.ByOutcome[firstNonEmpty(e.Outcome, "unrecorded")]++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dst, append(b, '\n'), 0o644)
}

// rotatingCallLog is the serving daemon's call-log writer: O_APPEND writes,
// one Write per row, rotated when a row would take the file past max.
type rotatingCallLog struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
	max  int64
	now  func() time.Time
	logf func(string)
}

func openRotatingCallLog(path string, max int64, logf func(string)) (*rotatingCallLog, error) {
	f, err := openCallLog(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if logf == nil {
		logf = func(string) {}
	}
	return &rotatingCallLog{path: path, f: f, size: st.Size(), max: max, now: time.Now, logf: logf}, nil
}

// Name is the live log's path (mcp.NewCallLogger reads it).
func (r *rotatingCallLog) Name() string { return r.path }

func (r *rotatingCallLog) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return 0, os.ErrClosed
	}
	if r.size > 0 && r.size+int64(len(b)) > r.max {
		r.rotateLocked()
	}
	n, err := r.f.Write(b)
	r.size += int64(n)
	return n, err
}

// rotateLocked rotates; a failed rotation keeps appending to the live log
// (a row is never dropped for retention's sake) and says so.
func (r *rotatingCallLog) rotateLocked() {
	_ = r.f.Close()
	rot, err := rotateCallLog(r.path, r.now())
	f, oerr := openCallLog(r.path)
	if oerr != nil {
		r.f = nil
		r.logf(fmt.Sprintf("call log: reopen %s after rotation failed: %v — call logging stopped", r.path, oerr))
		return
	}
	r.f = f
	r.size = 0
	if st, serr := f.Stat(); serr == nil {
		r.size = st.Size()
	}
	if err != nil {
		r.logf(fmt.Sprintf("call log: rotation of %s failed (%v) — appending to the live log; nothing was deleted", r.path, err))
		return
	}
	msg := fmt.Sprintf("call log: rotated %s at %s → %s", r.path, tenancy.FormatBytes(r.max), rot.Rotated)
	if rot.Archive != "" {
		msg += fmt.Sprintf("; previous generation archived to %s (rollup %s)", rot.Archive, rot.Rollup)
	}
	r.logf(msg)
	if rot.Report != "" {
		r.logf("call log: " + rot.Report)
	}
}

func (r *rotatingCallLog) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}
