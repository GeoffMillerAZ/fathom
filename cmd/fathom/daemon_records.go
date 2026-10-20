package main

// daemon_records.go — RT-166 (docs/design/responsible-tenant.md §10.2): a
// serve-http records itself at ~/.fathom/daemon/<id>/record.json (schema 2)
// beside its lifecycle ledger; `fathom daemon ps` joins these records with
// the live census. The legacy ~/.fathom/daemons/<host>-<port>.json is read
// for one release: a verb that talks to an address finds the newest
// record for it in either place, and a daemon writing a new record prunes
// the legacy one for its address.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// daemonMeta is what serve-http knows about itself beyond its address.
type daemonMeta struct {
	Managed  bool
	Label    string // the launchd label when managed ("" otherwise)
	PPID     int
	IdleExit time.Duration
	TTL      time.Duration
	// LifecycleID is --lifecycle-id: where the ledger lives when it is not
	// the record's own directory (RT-158).
	LifecycleID string
	// Supervised is a managed start's supervision (detectSupervised);
	// nil when unmanaged.
	Supervised *bool
}

// daemonID names the record directory: the label when managed, else
// addr-<port>-<pid> (unique per process; a scratch daemon on a reused
// port never overwrites another's record).
func daemonID(meta daemonMeta, addr string, pid int) string {
	if meta.Managed && meta.Label != "" {
		return meta.Label
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "unknown"
	}
	return "addr-" + port + "-" + strconv.Itoa(pid)
}

// daemonRecordsDir is ~/.fathom/daemon.
func daemonRecordsDir(home string) string {
	return filepath.Join(fathomhome.Under(home), "daemon")
}

// daemonRecordPath is ~/.fathom/daemon/<id>/record.json; an id that could
// escape the directory is refused.
func daemonRecordPath(home, id string) (string, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return "", errors.New("daemon record: id " + id + " cannot name a record directory")
	}
	return filepath.Join(daemonRecordsDir(home), id, "record.json"), nil
}

// cmdlineSHA is the hex sha256 of argv joined by NUL.
func cmdlineSHA(argv []string) string {
	h := sha256.Sum256([]byte(strings.Join(argv, "\x00")))
	return hex.EncodeToString(h[:])
}

// exeInode is the inode of path, 0 when it cannot be read.
func exeInode(path string) uint64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return uint64(st.Ino)
	}
	return 0
}

// recordServingDaemonAs writes this daemon's schema-2 record and returns it
// with its path; it prunes the legacy record for the same address.
func recordServingDaemonAs(meta daemonMeta, addr, callLog, storePath, commit string, now time.Time) (console.DaemonRecord, string, error) {
	abs := callLog
	if abs != "" && !filepath.IsAbs(abs) {
		if a, err := filepath.Abs(abs); err == nil {
			abs = a
		}
	}
	pid := os.Getpid()
	rec := console.NewDaemonRecord(addr, pid, abs, absStorePath(storePath), commit, now)
	rec.ID = daemonID(meta, addr, pid)
	rec.Managed = meta.Managed
	rec.PPID = meta.PPID
	rec.LifecycleID = meta.LifecycleID
	rec.Supervised = meta.Supervised
	if exe, err := os.Executable(); err == nil {
		rec.Exe = exe
		rec.ExeInode = exeInode(exe)
	}
	rec.CmdlineSHA = cmdlineSHA(os.Args)
	if !meta.Managed {
		if meta.IdleExit > 0 {
			rec.IdleExit = meta.IdleExit.String()
		}
		if meta.TTL > 0 {
			rec.TTL = meta.TTL.String()
		}
	}
	rec.HeartbeatAt = now.UTC().Format(time.RFC3339)
	rec.LastActivityAt = rec.HeartbeatAt
	// The record's own provenance: a verb run from a terminal names this
	// log through the record (resolveConsoleCallLogFor), so the pages must
	// use the same words or their JSON differs from the verb's (OC-8).
	setServingCallLog(abs, rec.Note())
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return rec, "", err
	}
	p, err := daemonRecordPath(home, rec.ID)
	if err != nil {
		return rec, "", err
	}
	rec.StateRoot = filepath.Dir(p)
	if err := console.WriteDaemonRecord(p, rec); err != nil {
		return rec, p, err
	}
	// The legacy record lives under the real home's .fathom (RT-166). Under
	// FATHOM_HOME this daemon's state is elsewhere, and the real home's
	// files are not its to prune (PE-26).
	if os.Getenv(fathomhome.Env) == "" {
		if legacy, lerr := console.DaemonRecordPath(home, addr); lerr == nil {
			_ = os.Remove(legacy)
		}
	}
	pruneDeadScratchRecords(home, rec.ID, pidGone)
	return rec, p, nil
}

// pidGone reports whether no process has pid (ESRCH); a pid that exists
// but is not ours to signal (EPERM) is alive.
func pidGone(pid int) bool {
	return pid > 0 && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}

// pruneDeadScratchRecords removes the records of scratch daemons
// (addr-<port>-<pid>) whose pid is gone, keeping self. A scratch record id
// is unique per process, so without this every daemon that died without
// its clean exit (SIGKILL, a crash) would leave one for good, and every
// lookup would pay for it (I48). A managed daemon's record is kept: a verb
// names its call log through it while launchd restarts it. The pid comes
// from the directory name; the record is not read.
func pruneDeadScratchRecords(home, self string, gone func(int) bool) {
	dir := daemonRecordsDir(home)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !e.IsDir() || e.Name() == self {
			continue
		}
		_, pid, ok := scratchRecordID(e.Name())
		if !ok || !gone(pid) {
			continue
		}
		removeDaemonRecord(filepath.Join(dir, e.Name(), "record.json"))
	}
}

// scratchRecordID parses addr-<port>-<pid>.
func scratchRecordID(id string) (port string, pid int, ok bool) {
	rest, found := strings.CutPrefix(id, "addr-")
	if !found {
		return "", 0, false
	}
	i := strings.LastIndex(rest, "-")
	if i <= 0 {
		return "", 0, false
	}
	pid, err := strconv.Atoi(rest[i+1:])
	if err != nil || pid <= 0 {
		return "", 0, false
	}
	return rest[:i], pid, true
}

// removeDaemonRecord removes a record on clean exit, and its directory
// when nothing else (a lifecycle ledger) lives there. A scratch daemon's
// resource ring (addr-<port>-<pid>, §13.1) goes with its record: its id is
// unique per process, so a ring left behind would never be written or read
// again and every dead scratch daemon would leave ~270 KB for good. A
// managed daemon's ring is kept: it is how a stopped daemon stays visible.
func removeDaemonRecord(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
	_ = os.Remove(path + ".tmp")
	if _, _, scratch := scratchRecordID(filepath.Base(filepath.Dir(path))); scratch {
		_ = os.Remove(filepath.Join(filepath.Dir(path), tenancy.RingFileName))
	}
	_ = os.Remove(filepath.Dir(path)) // fails, harmlessly, when not empty
}

// storedRecord is a record with where it was read from.
type storedRecord struct {
	Rec    console.DaemonRecord
	Path   string
	Legacy bool
}

// listDaemonRecords reads every record under ~/.fathom/daemon/*/ and the
// legacy ~/.fathom/daemons/*.json, sorted by path. An unreadable record is
// returned in errs (named, never silently absent — I38).
func listDaemonRecords(home string) (recs []storedRecord, errs []error) {
	return listDaemonRecordsWhere(home, nil)
}

// listDaemonRecordsWhere is listDaemonRecords reading only the record
// directories keep accepts by name (nil keeps all). It reads this process's
// fathom directory: home/.fathom, or FATHOM_HOME when that is set.
func listDaemonRecordsWhere(home string, keep func(id string) bool) (recs []storedRecord, errs []error) {
	return listDaemonRecordsAt(fathomhome.Under(home), keep)
}

// listDaemonRecordsAt is listDaemonRecordsWhere over an explicit fathom
// directory fh (the one holding daemon/ and the legacy daemons/), which the
// environment never redirects: the operator-state guard reads the
// operator's own records with it.
func listDaemonRecordsAt(fh string, keep func(id string) bool) (recs []storedRecord, errs []error) {
	newDir := filepath.Join(fh, "daemon")
	if ents, err := os.ReadDir(newDir); err == nil {
		for _, e := range ents {
			if !e.IsDir() || (keep != nil && !keep(e.Name())) {
				continue
			}
			p := filepath.Join(newDir, e.Name(), "record.json")
			rec, rerr := console.ReadDaemonRecord(p)
			if rerr != nil {
				if !os.IsNotExist(rerr) {
					errs = append(errs, &recordReadError{Path: p, Err: rerr})
				}
				continue
			}
			recs = append(recs, storedRecord{Rec: rec, Path: p})
		}
	} else if !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	legacyDir := filepath.Join(fh, "daemons")
	if ents, err := os.ReadDir(legacyDir); err == nil {
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			p := filepath.Join(legacyDir, e.Name())
			rec, rerr := console.ReadDaemonRecord(p)
			if rerr != nil {
				errs = append(errs, &recordReadError{Path: p, Err: rerr})
				continue
			}
			recs = append(recs, storedRecord{Rec: rec, Path: p, Legacy: true})
		}
	} else if !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Path < recs[j].Path })
	return recs, errs
}

// sameDaemonAddr compares two host:port addresses with the loopback and
// wildcard hosts folded, as console.DaemonRecordPath folds them.
func sameDaemonAddr(a, b string) bool {
	fold := func(addr string) string {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return addr
		}
		switch host {
		case "", "localhost", "::1", "0.0.0.0", "::":
			host = "127.0.0.1"
		}
		return host + ":" + port
	}
	return fold(a) == fold(b)
}

// findDaemonRecord returns the record for addr: a schema-2 record first
// (newest started_at, then path), else the legacy one.
// Only the directories that can be addr's are read: a managed label's, and
// addr-<port>-* for addr's port — it sits on the console's request path.
func findDaemonRecord(home, addr string) (console.DaemonRecord, bool) {
	_, wantPort, _ := net.SplitHostPort(addr)
	recs, _ := listDaemonRecordsWhere(home, func(id string) bool {
		port, _, scratch := scratchRecordID(id)
		return !scratch || port == wantPort
	})
	var best *storedRecord
	for i := range recs {
		r := &recs[i]
		if !sameDaemonAddr(r.Rec.Addr, addr) {
			continue
		}
		if best == nil || (best.Legacy && !r.Legacy) ||
			(best.Legacy == r.Legacy && r.Rec.StartedAt > best.Rec.StartedAt) {
			best = r
		}
	}
	if best == nil {
		return console.DaemonRecord{}, false
	}
	return best.Rec, true
}
