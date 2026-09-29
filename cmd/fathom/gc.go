package main

// gc.go — `fathom gc` (docs/design/responsible-tenant.md §12.1, RT-125):
// per-class retention for what fathom leaves on disk. Dry run by default:
// it reports what each class would reclaim and changes nothing; --apply
// acts. Evidence is archived, never deleted: the daemon's writer rotates
// the call log and gc gzips its pending generations (calllog_rotate.go),
// never renaming a live log; and the verify, session and intent
// ledgers and decisions.jsonl are reported, not rotated — their readers
// read only the live file, so archiving them would hide history (I33, I38)
// until those readers read archives too. What is deleted is what is not
// evidence: old session stamps, dead daemons' records, stale test temp
// dirs, old scenario-bench worktrees and build binaries, and diagnostic
// captures past their bound.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// Retention bounds (§12.1).
const (
	gcLedgerRotateBytes     = 32 << 20
	gcSessionStampAge       = 30 * 24 * time.Hour
	gcTmpAge                = 24 * time.Hour
	gcWorktreeKeepRuns      = 5
	gcWorktreeKeepAge       = 14 * 24 * time.Hour
	gcBinKeep               = 3
	gcScenarioWorktreesPath = "artifacts/scenariobench/worktrees"
)

// gcTmpCacheDirs are the hooks' shared $TMPDIR caches whose entries gc
// prunes after a day: FileChanged leases (one per root, file and mtime),
// the per-cwd toplevel cache, the ident cache, guard stamps and the
// Bash-write hook's once-per-session doubt markers.
var gcTmpCacheDirs = []string{"fathom-filechanged", "fathom-toplevel-cache", "fathom-ident-cache", "fathom-guard-stamps", "fathom-bash-doubt"}

// gcOptions are runGC's inputs; every location and effect is injectable.
type gcOptions struct {
	Home      string // the user's home (~/.fathom lives under it; FATHOM_HOME moves it, internal/fathomhome)
	TmpDir    string // $TMPDIR
	Workspace string // a repository's workspace/ ("" skips the workspace classes)
	CallLog   string // the resolved call log ("" skips it)
	Apply     bool
	Now       time.Time
	Alive     func(pid int) bool // nil: signal 0
}

// gcItem is one thing a class would reclaim or reports.
type gcItem struct {
	Class  string `json:"class"`
	Path   string `json:"path"`
	Action string `json:"action"` // delete | archive | report
	Bytes  int64  `json:"bytes"`
	Reason string `json:"reason"`
	Done   bool   `json:"done,omitempty"`
	Error  string `json:"error,omitempty"`
}

// gcReport is runGC's result.
type gcReport struct {
	Applied     bool             `json:"applied"`
	Items       []gcItem         `json:"items"`
	Reclaimable map[string]int64 `json:"reclaimable_by_class"`
	Errors      []string         `json:"errors,omitempty"`
}

// runGC plans every class and, with Apply, acts on it.
func runGC(o gcOptions) gcReport {
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	if o.Alive == nil {
		o.Alive = pidAliveSignal0
	}
	rep := gcReport{Applied: o.Apply, Reclaimable: map[string]int64{}}
	add := func(it gcItem) { rep.Items = append(rep.Items, it) }
	errf := func(format string, a ...any) { rep.Errors = append(rep.Errors, fmt.Sprintf(format, a...)) }

	// Call log (§12.1): the serving daemon's writer rotates the live log —
	// gc never renames a log a daemon holds open (its appends would follow
	// the renamed inode). gc archives the pending generations, under the
	// archive lock the daemon's own pass takes, and reports the rest.
	if o.CallLog != "" {
		if st, err := os.Stat(o.CallLog); err == nil && st.Size() > callLogRotateBytes {
			add(gcItem{Class: "call-log", Path: o.CallLog, Action: "report", Bytes: st.Size(), Reason: fmt.Sprintf("%s over %s: the serving daemon rotates it at its next row (gc never renames a live log)", tenancy.FormatBytes(st.Size()), tenancy.FormatBytes(callLogRotateBytes))})
		} else if err != nil && !os.IsNotExist(err) {
			errf("call log %s: %v", o.CallLog, err)
		}
		for _, p := range pendingCallLogs(o.CallLog) {
			var size int64
			if st, err := os.Stat(p); err == nil {
				size = st.Size()
			}
			add(gcItem{Class: "call-log", Path: p, Action: "archive", Bytes: size, Reason: "a rotated generation awaiting its archive: rolled up and gzipped, never deleted"})
		}
		if n := callLogArchiveBytes(o.CallLog); n > callLogArchiveWarnBytes {
			add(gcItem{Class: "call-log", Path: o.CallLog + ".*.jsonl.gz", Action: "report", Bytes: n, Reason: "archives over 1 GiB — never deleted; move them elsewhere"})
		}
	}

	// Ledgers: reported at 32 MiB, never rotated here (see the file comment).
	if o.Home != "" {
		fh := fathomhome.Under(o.Home)
		ledgers := []string{filepath.Join(fh, "verify-ledger.jsonl"), filepath.Join(fh, "session-ledger.jsonl"), filepath.Join(fh, "intent-ledger.jsonl")}
		if o.CallLog != "" {
			ledgers = append(ledgers, filepath.Join(filepath.Dir(o.CallLog), "decisions.jsonl"))
		}
		for _, p := range ledgers {
			if st, err := os.Stat(p); err == nil && st.Size() > gcLedgerRotateBytes {
				add(gcItem{Class: "ledgers", Path: p, Action: "report", Bytes: st.Size(), Reason: "over 32 MiB; kept whole — its readers read only the live file, so an archive would hide history"})
			}
		}

		// Session stamps older than 30 days.
		stamps := filepath.Join(fh, "session-stamps")
		gcEntries(stamps, func(e os.DirEntry, st os.FileInfo) (string, bool) {
			return "session stamp older than 30 days", o.Now.Sub(st.ModTime()) > gcSessionStampAge
		}, "session-stamps", add, errf)

		// Dead daemons' records (the directory keeps its lifecycle ledger).
		recs, rerrs := listDaemonRecords(o.Home)
		for _, e := range rerrs {
			errf("daemon records: %v", e)
		}
		for _, r := range recs {
			if r.Rec.PID <= 0 || o.Alive(r.Rec.PID) {
				continue
			}
			var size int64
			if st, err := os.Stat(r.Path); err == nil {
				size = st.Size()
			}
			add(gcItem{Class: "daemon-records", Path: r.Path, Action: "delete", Bytes: size, Reason: fmt.Sprintf("pid %d is not running", r.Rec.PID)})
		}

		// wazero's compilation cache (LB-F26): other wazero versions'
		// directories and entries unwritten for 30 days. Compiled machine
		// code, not evidence: the next compile recreates what is still used.
		for _, p := range tsengine.PrunableCompilationCache(o.Home, o.Now) {
			add(gcItem{Class: "wazero-cache", Path: p, Action: "delete", Bytes: dirSize(p), Reason: "compiled grammar code of another wazero version, or unwritten for 30 days"})
		}

		// Diagnostic captures past 10 / 200 MiB per daemon.
		ids, _ := os.ReadDir(daemonRecordsDir(o.Home))
		for _, id := range ids {
			if !id.IsDir() {
				continue
			}
			for _, p := range diagPrunable(filepath.Join(daemonRecordsDir(o.Home), id.Name(), "diag"), diagKeep, diagKeepBytes) {
				add(gcItem{Class: "diag", Path: p, Action: "delete", Bytes: dirSize(p), Reason: "past 10 captures / 200 MiB"})
			}
		}
	}

	// Test and hook temp files in $TMPDIR older than a day.
	if o.TmpDir != "" {
		gcEntries(o.TmpDir, func(e os.DirEntry, st os.FileInfo) (string, bool) {
			n := e.Name()
			match := strings.HasPrefix(n, "fathom-edit-set") || strings.HasPrefix(n, "fathom-stranger-check") ||
				(strings.HasPrefix(n, "fathom-") && strings.Contains(n, "-test-"))
			return "fathom temp older than a day", match && o.Now.Sub(st.ModTime()) > gcTmpAge
		}, "tmp", add, errf)
		// The hooks' shared caches: their entries (never the directories)
		// older than a day. Every entry outlives its own TTL by then.
		for _, d := range gcTmpCacheDirs {
			gcEntries(filepath.Join(o.TmpDir, d), func(e os.DirEntry, st os.FileInfo) (string, bool) {
				return d + " entry older than a day", o.Now.Sub(st.ModTime()) > gcTmpAge
			}, "tmp", add, errf)
		}
	}

	// Workspace: scenario-bench worktrees and build binaries.
	if o.Workspace != "" {
		wt := filepath.Join(o.Workspace, filepath.FromSlash(gcScenarioWorktreesPath))
		for _, p := range keepNewest(wt, "", gcWorktreeKeepRuns, gcWorktreeKeepAge, o.Now, true) {
			add(gcItem{Class: "scenariobench-worktrees", Path: p, Action: "delete", Bytes: dirSize(p), Reason: "beyond the last 5 runs and older than 14 days (a .pin exempts)"})
		}
		for _, p := range keepNewest(filepath.Join(o.Workspace, "tmp"), "fathom-bin-", gcBinKeep, 0, o.Now, false) {
			add(gcItem{Class: "workspace-bins", Path: p, Action: "delete", Bytes: dirSize(p), Reason: "beyond the newest 3 build binaries"})
		}
	}

	archived := map[string]bool{} // pending generations one archive pass handled
	archivedErr := error(nil)
	archivePass := false
	for i := range rep.Items {
		it := &rep.Items[i]
		if it.Action != "report" {
			rep.Reclaimable[it.Class] += it.Bytes
		}
		if !o.Apply || it.Action == "report" {
			continue
		}
		var err error
		switch it.Action {
		case "delete":
			err = os.RemoveAll(it.Path)
		case "archive":
			if !archivePass {
				archivePass = true
				pass, perr := archiveCallLogGenerations(o.CallLog)
				archivedErr = perr
				for _, a := range pass.Archived {
					archived[strings.TrimSuffix(a, ".jsonl.gz")+".pending"] = true
				}
			}
			if !archived[it.Path] {
				if _, serr := os.Stat(it.Path); os.IsNotExist(serr) {
					// Archived meanwhile by the daemon's own pass.
				} else if archivedErr != nil {
					err = archivedErr
				} else {
					err = fmt.Errorf("still pending after the archive pass")
				}
			}
		}
		if err != nil {
			it.Error = err.Error()
			errf("%s %s: %v", it.Action, it.Path, err)
			continue
		}
		it.Done = true
	}
	return rep
}

// gcEntries adds each entry of dir that match accepts.
func gcEntries(dir string, match func(os.DirEntry, os.FileInfo) (string, bool), class string, add func(gcItem), errf func(string, ...any)) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			errf("%s: %v", dir, err)
		}
		return
	}
	for _, e := range ents {
		st, err := e.Info()
		if err != nil {
			continue
		}
		if why, ok := match(e, st); ok {
			p := filepath.Join(dir, e.Name())
			size := st.Size()
			if e.IsDir() {
				size = dirSize(p)
			}
			add(gcItem{Class: class, Path: p, Action: "delete", Bytes: size, Reason: why})
		}
	}
}

// keepNewest lists dir's entries (named with prefix) beyond the newest
// keep by mtime and, when maxAge > 0, also older than maxAge; honorPin
// exempts an entry holding a .pin file.
func keepNewest(dir, prefix string, keep int, maxAge time.Duration, now time.Time, honorPin bool) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type ent struct {
		path string
		mod  time.Time
	}
	var es []ent
	for _, e := range ents {
		if prefix != "" && !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		st, err := e.Info()
		if err != nil {
			continue
		}
		es = append(es, ent{filepath.Join(dir, e.Name()), st.ModTime()})
	}
	sort.Slice(es, func(i, j int) bool {
		if !es[i].mod.Equal(es[j].mod) {
			return es[i].mod.After(es[j].mod)
		}
		return es[i].path < es[j].path
	})
	var out []string
	for i, e := range es {
		if i < keep || (maxAge > 0 && now.Sub(e.mod) <= maxAge) {
			continue
		}
		if honorPin {
			if _, err := os.Stat(filepath.Join(e.path, ".pin")); err == nil {
				continue
			}
		}
		out = append(out, e.path)
	}
	sort.Strings(out)
	return out
}

// diagPrunable is what pruneDiagCaptures would remove from dir.
func diagPrunable(dir string, keep int, keepBytes int64) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	var total int64
	var out []string
	for i, name := range names {
		p := filepath.Join(dir, name)
		size := dirSize(p)
		if i == 0 || (i < keep && total+size <= keepBytes) {
			total += size
			continue
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// pidAliveSignal0 reports whether pid is a live process (signal 0).
func pidAliveSignal0(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func cmdGC(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("gc", stderr)
	apply := fs.Bool("apply", false, "act on the plan (default: dry run — report what each class would reclaim and change nothing)")
	jsonOut := fs.Bool("json", false, "emit JSON")
	workspace := fs.String("workspace", "", "the repository workspace/ to include (default: the enclosing repository's)")
	callLogFlag := fs.String("call-log", "", "the call log (default: the daemon's recorded log, then [daemon].call_log, then "+defaultCallLogPath()+")")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	home, _ := os.UserHomeDir()
	ws := *workspace
	if ws == "" {
		if cwd, err := os.Getwd(); err == nil {
			if top := gitToplevel(cwd); top != "" {
				ws = filepath.Join(top, "workspace")
			}
		}
	}
	rep := runGC(gcOptions{Home: home, TmpDir: os.TempDir(), Workspace: ws, CallLog: resolveConsoleCallLog(*callLogFlag, "").Path, Apply: *apply})
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	} else {
		renderGC(stdout, rep)
	}
	if len(rep.Errors) > 0 {
		return 1
	}
	return 0
}

func renderGC(w io.Writer, rep gcReport) {
	mode := "dry run — nothing changed; pass --apply to act"
	if rep.Applied {
		mode = "applied"
	}
	fmt.Fprintf(w, "fathom gc — %s\n", mode)
	if len(rep.Items) == 0 {
		fmt.Fprintln(w, "nothing to reclaim")
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, it := range rep.Items {
		state := ""
		switch {
		case it.Error != "":
			state = "FAILED: " + it.Error
		case it.Done:
			state = "done"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s %s\n", it.Class, it.Action, tenancy.FormatBytes(it.Bytes), it.Path, it.Reason, state)
	}
	_ = tw.Flush()
	classes := make([]string, 0, len(rep.Reclaimable))
	for c := range rep.Reclaimable {
		classes = append(classes, c)
	}
	sort.Strings(classes)
	for _, c := range classes {
		fmt.Fprintf(w, "reclaimable  %s %s\n", c, tenancy.FormatBytes(rep.Reclaimable[c]))
	}
	for _, e := range rep.Errors {
		fmt.Fprintf(w, "error  %s\n", e)
	}
}
