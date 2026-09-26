package main

// daemon_census.go — RT-23/RT-24 (docs/design/responsible-tenant.md §10.2):
// `fathom daemon ps` joins the daemon records with a live census of
// serve-http processes and names what is wrong with each; `fathom daemon
// reap` is dry-run by default and, with --apply, kills only unmanaged
// daemons whose verdict is not ok — never the managed daemon — and prunes
// stale records. Before this, 15 stale records and two 22-hour scratch
// daemons were found by hand (INC-23).

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// censusIdleAfter is when an unmanaged daemon with no request reads idle
// (doctor's scratch_daemons threshold; its own --idle-exit should have
// fired long before).
const censusIdleAfter = time.Hour

// psProc is one serve-http process from `ps -axo pid=,ppid=,etime=,rss=,command=`.
type psProc struct {
	PID, PPID int
	Elapsed   time.Duration
	RSSKiB    int64
	Command   string
}

// parseEtime parses ps's [[dd-]hh:]mm:ss.
func parseEtime(s string) time.Duration {
	var days int
	if d, rest, ok := strings.Cut(s, "-"); ok {
		days, _ = strconv.Atoi(d)
		s = rest
	}
	parts := strings.Split(s, ":")
	var secs int
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		secs = secs*60 + n
	}
	return time.Duration(days)*24*time.Hour + time.Duration(secs)*time.Second
}

// parsePsLine parses one ps line; ok only for a fathom serve-http process
// (argv[0]'s base name contains "fathom" and an argument is serve-http).
func parsePsLine(line string) (psProc, bool) {
	f := strings.Fields(line)
	if len(f) < 6 {
		return psProc{}, false
	}
	pid, err1 := strconv.Atoi(f[0])
	ppid, err2 := strconv.Atoi(f[1])
	rss, err3 := strconv.ParseInt(f[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return psProc{}, false
	}
	argv := f[4:]
	if !strings.Contains(filepath.Base(argv[0]), "fathom") {
		return psProc{}, false
	}
	serve := false
	for _, a := range argv[1:] {
		serve = serve || a == "serve-http"
	}
	if !serve {
		return psProc{}, false
	}
	return psProc{PID: pid, PPID: ppid, Elapsed: parseEtime(f[2]), RSSKiB: rss, Command: strings.Join(argv, " ")}, true
}

// cmdFlagValue returns the value of --name/-name in a command line.
func cmdFlagValue(command, name string) (string, bool) {
	f := strings.Fields(command)
	for i, a := range f {
		for _, p := range []string{"--" + name, "-" + name} {
			if a == p && i+1 < len(f) {
				return f[i+1], true
			}
			if v, ok := strings.CutPrefix(a, p+"="); ok {
				return v, true
			}
		}
	}
	return "", false
}

func cmdHasFlag(command, name string) bool {
	for _, a := range strings.Fields(command) {
		if a == "--"+name || a == "-"+name || a == "--"+name+"=true" || a == "-"+name+"=true" {
			return true
		}
	}
	return false
}

// censusRow is one `daemon ps` line: a live process (with or without a
// record) or a record whose process is gone.
type censusRow struct {
	PID         int           `json:"pid"`
	PPID        int           `json:"ppid,omitempty"`
	Addr        string        `json:"addr,omitempty"`
	Managed     bool          `json:"managed"`
	Age         time.Duration `json:"age_ns,omitempty"`
	RSSKiB      int64         `json:"rss_kib,omitempty"`
	Idle        time.Duration `json:"idle_ns"`      // -1 = unknown (no schema-2 record)
	ParentAlive bool          `json:"parent_alive"` // ppid != 1
	Unlinked    bool          `json:"unlinked"`     // its executable is gone or replaced
	Record      string        `json:"record"`       // "record", "legacy", "none", "stale"
	RecordPath  string        `json:"record_path,omitempty"`
	Verdict     string        `json:"verdict"` // ok, orphaned, idle, unlinked, no-record, stale-record
	Command     string        `json:"command,omitempty"`
}

// daemonCensus joins records with processes. managedAddr is the managed
// daemon's address (a schema-1 record, or a record-less process with no
// --addr, there is the managed daemon's); statExe returns an executable's
// inode. Rows are sorted by pid, then record path. Pure.
func daemonCensus(recs []storedRecord, procs []psProc, now time.Time, managedAddr string, statExe func(string) (uint64, bool)) []censusRow {
	byPID := map[int][]storedRecord{}
	for _, r := range recs {
		byPID[r.Rec.PID] = append(byPID[r.Rec.PID], r)
	}
	live := map[int]bool{}
	var rows []censusRow
	for _, p := range procs {
		live[p.PID] = true
		row := censusRow{PID: p.PID, PPID: p.PPID, Age: p.Elapsed, RSSKiB: p.RSSKiB, Idle: -1, ParentAlive: p.PPID != 1, Record: "none", Command: p.Command}
		addr, hasAddr := cmdFlagValue(p.Command, "addr")
		if !hasAddr {
			addr = managedAddr
		}
		row.Addr = addr
		row.Managed = cmdHasFlag(p.Command, "managed") || sameDaemonAddr(addr, managedAddr)
		var rec *storedRecord
		for i := range byPID[p.PID] {
			r := &byPID[p.PID][i]
			if rec == nil || (rec.Legacy && !r.Legacy) {
				rec = r
			}
		}
		exe := strings.Fields(p.Command)[0]
		if rec != nil {
			row.Record, row.RecordPath = "record", rec.Path
			if rec.Legacy {
				row.Record = "legacy"
			}
			if rec.Rec.Addr != "" {
				row.Addr = rec.Rec.Addr
			}
			row.Managed = rec.Rec.Managed || cmdHasFlag(p.Command, "managed") ||
				(rec.Rec.SchemaVersion == 1 && sameDaemonAddr(rec.Rec.Addr, managedAddr))
			if t, err := time.Parse(time.RFC3339, rec.Rec.LastActivityAt); err == nil {
				row.Idle = now.Sub(t)
			}
			if rec.Rec.Exe != "" {
				exe = rec.Rec.Exe
			}
			if rec.Rec.ExeInode != 0 {
				ino, ok := statExe(exe)
				row.Unlinked = !ok || ino != rec.Rec.ExeInode
			}
		}
		if !row.Unlinked && filepath.IsAbs(exe) {
			if _, ok := statExe(exe); !ok {
				row.Unlinked = true
			}
		}
		switch {
		case row.Unlinked:
			row.Verdict = "unlinked"
		case !row.Managed && !row.ParentAlive:
			row.Verdict = "orphaned"
		case !row.Managed && row.Idle >= censusIdleAfter:
			row.Verdict = "idle"
		case rec == nil:
			row.Verdict = "no-record"
		default:
			row.Verdict = "ok"
		}
		rows = append(rows, row)
	}
	for _, r := range recs {
		if live[r.Rec.PID] {
			continue
		}
		rows = append(rows, censusRow{PID: r.Rec.PID, PPID: r.Rec.PPID, Addr: r.Rec.Addr, Managed: r.Rec.Managed, Idle: -1,
			Record: "stale", RecordPath: r.Path, Verdict: "stale-record"})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].PID != rows[j].PID {
			return rows[i].PID < rows[j].PID
		}
		return rows[i].RecordPath < rows[j].RecordPath
	})
	return rows
}

func shortDur(d time.Duration) string {
	switch {
	case d < 0:
		return "?"
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd%dh", int(d/(24*time.Hour)), int(d%(24*time.Hour)/time.Hour))
	case d >= time.Hour:
		return fmt.Sprintf("%dh%dm", int(d/time.Hour), int(d%time.Hour/time.Minute))
	default:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
}

// renderDaemonCensus prints the ps table.
func renderDaemonCensus(w io.Writer, rows []censusRow, now time.Time) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PID\tADDR\tMANAGED\tAGE\tRSS\tIDLE\tPARENT\tUNLINKED\tRECORD\tVERDICT")
	for _, r := range rows {
		age, rss, parent := "-", "-", "-"
		if r.Record != "stale" {
			age = shortDur(r.Age)
			rss = fmt.Sprintf("%dMiB", r.RSSKiB/1024)
			parent = "alive"
			if !r.ParentAlive {
				parent = "gone"
			}
		}
		fmt.Fprintf(tw, "%d\t%s\t%v\t%s\t%s\t%s\t%s\t%v\t%s\t%s\n", r.PID, r.Addr, r.Managed, age, rss, shortDur(r.Idle), parent, r.Unlinked, r.Record, r.Verdict)
	}
	_ = tw.Flush()
	_ = now
}

// liveCensus runs ps and reads the records.
func liveCensus() ([]censusRow, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, fmt.Errorf("daemon census: no home directory: %v", err)
	}
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,etime=,rss=,command=").Output()
	if err != nil {
		return nil, fmt.Errorf("daemon census: ps: %w", err)
	}
	var procs []psProc
	for _, line := range strings.Split(string(out), "\n") {
		if p, ok := parsePsLine(line); ok {
			procs = append(procs, p)
		}
	}
	recs, errs := listDaemonRecords(home)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "warning: unreadable daemon record: %v\n", e)
	}
	return daemonCensus(recs, procs, time.Now(), managedDaemonAddr(), exeInodeOK), nil
}

func exeInodeOK(p string) (uint64, bool) {
	ino := exeInode(p)
	return ino, ino != 0
}

// managedDaemonAddr is the managed daemon's address: the installed
// plist's --addr, else [daemon].addr, else the conventional 127.0.0.1:7575.
func managedDaemonAddr() string {
	configured := ""
	if loaded, err := config.Load(config.DefaultUserPath(), ""); err == nil {
		configured = loaded.Config.Daemon.Addr
	}
	home, _ := os.UserHomeDir()
	return managedDaemonAddrIn(home, configured)
}

// cmdDaemonPs implements `fathom daemon ps [--json]`.
func cmdDaemonPs(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("daemon ps", stderr)
	jsonOut := fs.Bool("json", false, "emit the census as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rows, err := liveCensus()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if rows == nil {
			rows = []censusRow{}
		}
		_ = enc.Encode(rows)
		return 0
	}
	if len(rows) == 0 {
		fmt.Fprintln(stdout, "no serve-http processes and no daemon records")
		return 0
	}
	renderDaemonCensus(stdout, rows, time.Now())
	return 0
}

// reapEnv is runDaemonReap's world: the census, signals and records.
type reapEnv struct {
	census       func() ([]censusRow, error)
	signal       func(pid int, sig syscall.Signal) error
	alive        func(pid int) bool
	removeRecord func(path string)
	sleep        func(time.Duration)
	self         int // never reaped
	selfParent   int // never reaped
}

func liveReapEnv() reapEnv {
	return reapEnv{
		census: liveCensus,
		signal: syscall.Kill,
		alive: func(pid int) bool {
			if syscall.Kill(pid, 0) != nil {
				return false
			}
			out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
			return err == nil && !strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
		},
		removeRecord: func(p string) {
			if filepath.Base(p) == "record.json" {
				removeDaemonRecord(p)
				return
			}
			_ = os.Remove(p) // a legacy ~/.fathom/daemons/<host>-<port>.json
		},
		sleep:      time.Sleep,
		self:       os.Getpid(),
		selfParent: os.Getppid(),
	}
}

// cmdDaemonReap implements `fathom daemon reap [--apply] [--grace 5s]`.
func cmdDaemonReap(args []string, stdout, stderr io.Writer) int {
	return runDaemonReap(args, stdout, stderr, liveReapEnv())
}

// runDaemonReap plans (and with --apply, carries out) the reap: SIGTERM,
// grace, SIGKILL for each unmanaged daemon whose verdict is not ok, the
// managed daemon never; the pid is re-checked against a fresh census
// before each kill (pid reuse); stale records are pruned.
func runDaemonReap(args []string, stdout, stderr io.Writer, env reapEnv) int {
	fs := newFlagSet("daemon reap", stderr)
	apply := fs.Bool("apply", false, "carry out the plan (default: dry run — nothing is killed or removed)")
	grace := fs.Duration("grace", 5*time.Second, "time between SIGTERM and SIGKILL")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rows, err := env.census()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	var kills, prunes []censusRow
	for _, r := range rows {
		switch {
		case r.Verdict == "stale-record":
			prunes = append(prunes, r)
		case r.Verdict == "ok":
		case r.Managed:
			fmt.Fprintf(stdout, "keep pid %d: the managed daemon (%s) is never reaped\n", r.PID, r.Verdict)
		case r.PID == env.self || r.PID == env.selfParent:
			fmt.Fprintf(stdout, "keep pid %d: this reap's own process tree\n", r.PID)
		default:
			kills = append(kills, r)
		}
	}
	if !*apply {
		for _, r := range kills {
			fmt.Fprintf(stdout, "would kill pid %d (%s, %s, age %s): SIGTERM, %s, SIGKILL\n", r.PID, r.Verdict, r.Addr, shortDur(r.Age), *grace)
		}
		for _, r := range prunes {
			fmt.Fprintf(stdout, "would prune %s (pid %d is gone)\n", r.RecordPath, r.PID)
		}
		fmt.Fprintf(stdout, "dry run: %d to kill, %d stale record(s); nothing was killed or removed — re-run with --apply\n", len(kills), len(prunes))
		return 0
	}
	killed := 0
	for _, r := range kills {
		fresh, err := env.census()
		if err != nil {
			fmt.Fprintf(stderr, "error: re-checking pid %d: %v\n", r.PID, err)
			return 1
		}
		var now *censusRow
		for i := range fresh {
			if fresh[i].PID == r.PID && fresh[i].Record != "stale" {
				now = &fresh[i]
			}
		}
		if now == nil || now.Command != r.Command || now.Managed {
			fmt.Fprintf(stdout, "skip pid %d changed since the census (gone, reused or managed)\n", r.PID)
			continue
		}
		if err := env.signal(r.PID, syscall.SIGTERM); err != nil {
			fmt.Fprintf(stdout, "skip pid %d: SIGTERM: %v\n", r.PID, err)
			continue
		}
		const step = 100 * time.Millisecond
		for waited := time.Duration(0); waited < *grace && env.alive(r.PID); waited += step {
			env.sleep(step)
		}
		how := "exited on SIGTERM"
		if env.alive(r.PID) {
			_ = env.signal(r.PID, syscall.SIGKILL)
			how = "SIGKILLed after " + grace.String()
		}
		killed++
		fmt.Fprintf(stdout, "reaped pid %d (%s, %s): %s\n", r.PID, r.Verdict, r.Addr, how)
		if r.RecordPath != "" {
			env.removeRecord(r.RecordPath)
		}
	}
	for _, r := range prunes {
		env.removeRecord(r.RecordPath)
		fmt.Fprintf(stdout, "pruned %s\n", r.RecordPath)
	}
	fmt.Fprintf(stdout, "reaped %d daemon(s), pruned %d stale record(s)\n", killed, len(prunes))
	return 0
}
