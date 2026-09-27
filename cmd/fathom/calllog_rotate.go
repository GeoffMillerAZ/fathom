package main

// calllog_rotate.go — the call log's size-triggered rotation (docs/design/
// responsible-tenant.md §12.1, RT-126). At 64 MiB the daemon's writer
// renames — nothing more on the request path: the previous rotated
// generation <log>.1 becomes a pending generation <log>.<ts>.pending, and
// the live log becomes <log>.1 (which console tails read). A background
// pass then rolls each pending generation up (<log>.<ts>.rollup.json: rows
// by day, tool and outcome), gzips it to <log>.<ts>.jsonl.gz and only then
// removes it — its bytes live on in the archive. The pass holds
// <log>.archive.lock, which `fathom gc --apply` takes too, so the two never
// archive the same generation at once. Archives are never deleted: past
// 1 GiB of them the pass reports it (the serve-http log, doctor
// disk_retention, `fathom gc`) and keeps them all. Every usage reader
// reads every generation (openCallLogGenerations), so a rotation hides no
// history.

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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// Call-log retention bounds (§12.1).
const (
	callLogRotateBytes      = 64 << 20
	callLogArchiveWarnBytes = 1 << 30
)

// callLogRotation is what one rotation did (the renames only).
type callLogRotation struct {
	Rotated string // the rotated generation (<log>.1)
	Pending string // the previous generation, awaiting its archive ("" when none)
}

// callLogArchivePass is what one archive pass did.
type callLogArchivePass struct {
	Archived []string // the archives written, oldest first
	Rollups  []string
	Archives int64  // bytes of every archive after the pass
	Report   string // non-empty when the archives are over callLogArchiveWarnBytes
}

// callLogRotateRetry is how long a failed rotation waits before the
// writer tries again: never once per row.
const callLogRotateRetry = time.Minute

// rotateCallLog rotates path by renames alone: an existing <path>.1 moves
// to <path>.<ts>.pending (archived later by archiveCallLogGenerations),
// then the live log becomes <path>.1. now names the pending generation.
func rotateCallLog(path string, now time.Time) (callLogRotation, error) {
	rot := callLogRotation{Rotated: path + ".1"}
	if _, err := os.Stat(rot.Rotated); err == nil {
		ts := now.UTC().Format("20060102T150405Z")
		pending := path + "." + ts + ".pending"
		for i := 1; ; i++ {
			if _, err := os.Lstat(pending); os.IsNotExist(err) {
				break
			}
			pending = fmt.Sprintf("%s.%s-%d.pending", path, ts, i)
		}
		if err := os.Rename(rot.Rotated, pending); err != nil {
			return rot, err
		}
		rot.Pending = pending
	} else if !os.IsNotExist(err) {
		return rot, err
	}
	if err := os.Rename(path, rot.Rotated); err != nil && !os.IsNotExist(err) {
		return rot, err
	}
	return rot, nil
}

// pendingCallLogs lists path's generations awaiting their archive, oldest
// first.
func pendingCallLogs(path string) []string {
	m, _ := filepath.Glob(path + ".*.pending")
	sort.Strings(m)
	return m
}

// archiveCallLogGenerations rolls up and gzips every pending generation of
// path, oldest first, under <path>.archive.lock (shared with `fathom gc
// --apply`). A generation is removed only once its archive is complete;
// the first failure stops the pass and leaves the rest pending.
func archiveCallLogGenerations(path string) (callLogArchivePass, error) {
	var pass callLogArchivePass
	unlock, err := lockCallLogArchive(path)
	if err != nil {
		return pass, err
	}
	defer unlock()
	for _, p := range pendingCallLogs(path) {
		stem := strings.TrimSuffix(p, ".pending")
		archive, rollup := stem+".jsonl.gz", stem+".rollup.json"
		if err := writeCallLogRollup(p, rollup); err != nil {
			return pass, fmt.Errorf("roll up %s: %w", p, err)
		}
		if err := gzipFile(p, archive); err != nil {
			return pass, fmt.Errorf("archive %s: %w", p, err)
		}
		// Archived: the generation's bytes are in archive.
		if err := os.Remove(p); err != nil {
			return pass, err
		}
		pass.Archived = append(pass.Archived, archive)
		pass.Rollups = append(pass.Rollups, rollup)
	}
	pass.Archives = callLogArchiveBytes(path)
	if pass.Archives > callLogArchiveWarnBytes {
		pass.Report = fmt.Sprintf("call-log archives hold %s, over %s — nothing is deleted; move or compress them elsewhere (%s.*.jsonl.gz)",
			tenancy.FormatBytes(pass.Archives), tenancy.FormatBytes(callLogArchiveWarnBytes), path)
	}
	return pass, nil
}

// lockCallLogArchive takes the exclusive archive lock for path's
// generations (flock on <path>.archive.lock).
func lockCallLogArchive(path string) (func(), error) {
	f, err := os.OpenFile(path+".archive.lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
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
// one Write per row, rotated (renames only) when a row would take the file
// past max; the archive pass runs in the background.
type rotatingCallLog struct {
	mu      sync.Mutex
	path    string
	f       *os.File
	size    int64
	max     int64
	now     func() time.Time
	logf    func(string) // called from the archive pass's goroutine too
	retryAt time.Time    // a failed rotation is not retried before this

	// rotate is rotateCallLog (tests inject a failure); archiveHook runs at
	// the start of each background pass (tests hold it to prove no row
	// write waits on it).
	rotate      func(string, time.Time) (callLogRotation, error)
	archiveHook func()
	archiving   sync.WaitGroup
	archiveBusy atomic.Bool
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
	r := &rotatingCallLog{path: path, f: f, size: st.Size(), max: max, now: time.Now, logf: logf, rotate: rotateCallLog}
	if len(pendingCallLogs(path)) > 0 {
		r.startArchive() // a pass a previous process did not finish
	}
	return r, nil
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
		if now := r.now(); !now.Before(r.retryAt) {
			r.rotateLocked(now)
		}
	}
	n, err := r.f.Write(b)
	r.size += int64(n)
	return n, err
}

// rotateLocked renames; a failed rotation keeps appending to the live log
// (a row is never dropped for retention's sake), says so, and waits
// callLogRotateRetry before trying again.
func (r *rotatingCallLog) rotateLocked(now time.Time) {
	_ = r.f.Close()
	rot, err := r.rotate(r.path, now)
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
		r.retryAt = now.Add(callLogRotateRetry)
		r.logf(fmt.Sprintf("call log: rotation of %s failed (%v) — appending to the live log, retried in %s; nothing was deleted", r.path, err, callLogRotateRetry))
		return
	}
	r.retryAt = time.Time{}
	r.logf(fmt.Sprintf("call log: rotated %s at %s → %s", r.path, tenancy.FormatBytes(r.max), rot.Rotated))
	if rot.Pending != "" {
		r.startArchive()
	}
}

// startArchive runs one background archive pass unless one is running
// (which re-checks for pending generations before it ends).
func (r *rotatingCallLog) startArchive() {
	if !r.archiveBusy.CompareAndSwap(false, true) {
		return
	}
	r.archiving.Add(1)
	go func() {
		defer r.archiving.Done()
		for {
			if r.archiveHook != nil {
				r.archiveHook()
			}
			pass, err := archiveCallLogGenerations(r.path)
			for i, a := range pass.Archived {
				r.logf(fmt.Sprintf("call log: previous generation archived to %s (rollup %s)", a, pass.Rollups[i]))
			}
			if pass.Report != "" {
				r.logf("call log: " + pass.Report)
			}
			if err != nil {
				r.logf(fmt.Sprintf("call log: archiving %s failed (%v) — the generation stays pending (read by every usage reader, archived by the next pass or `fathom gc --apply`); nothing was deleted", r.path, err))
			}
			r.archiveBusy.Store(false)
			if err != nil || len(pendingCallLogs(r.path)) == 0 || !r.archiveBusy.CompareAndSwap(false, true) {
				return
			}
		}
	}()
}

func (r *rotatingCallLog) Close() error {
	r.mu.Lock()
	var err error
	if r.f != nil {
		err = r.f.Close()
		r.f = nil
	}
	r.mu.Unlock()
	r.archiving.Wait()
	return err
}
