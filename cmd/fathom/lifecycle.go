package main

// lifecycle.go — serve-http's lifecycle ledger and crash-loop breaker
// (docs/design/responsible-tenant.md §2.3, §8.5; RT-37, RT-38, RT-156,
// RT-158). INC-23: launchd relaunched a 30 GB daemon five times in 17
// minutes and nothing recorded why any of them ended. Every start and exit
// is a row in ~/.fathom/daemon/<id>/lifecycle.jsonl; the next start names an
// unclean predecessor (with the jetsam or resource report the system kept);
// and the breaker — evaluated before the store and the bind — starts a
// daemon that keeps dying in safe mode, then stops it.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// lifecycleID resolves the ledger id (§2.3): the launchd label when
// managed; else --lifecycle-id, which gives an unmanaged daemon (the stress
// harness's) one persistent ledger across its relaunches; else "" — an
// unmanaged daemon's per-process addr-<port>-<pid>, known after the bind.
// A --lifecycle-id can never name a managed label (a scratch daemon's
// ledger never trips the managed breaker), an addr-* id, or a path.
func lifecycleID(flagID string, managed bool, label string, reserved []string) (string, error) {
	if managed {
		return label, nil
	}
	if flagID == "" {
		return "", nil
	}
	if _, err := daemonRecordPath("/", flagID); err != nil {
		return "", fmt.Errorf("--lifecycle-id %q: not a directory name under ~/.fathom/daemon", flagID)
	}
	if flagID == defaultDaemonLabel || slices.Contains(reserved, flagID) {
		return "", fmt.Errorf("--lifecycle-id %q is a managed daemon's label: an unmanaged daemon never writes the managed ledger (its breaker would stop the operator's daemon)", flagID)
	}
	if strings.HasPrefix(flagID, "addr-") {
		return "", fmt.Errorf("--lifecycle-id %q: addr-* ids are per-process scratch ids", flagID)
	}
	return flagID, nil
}

// ledgerPathFor is ~/.fathom/daemon/<id>/lifecycle.jsonl under home.
func ledgerPathFor(home, id string) (string, error) {
	rec, err := daemonRecordPath(home, id)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(rec), "lifecycle.jsonl"), nil
}

// appendLedgerBytes appends one line to a ledger, rotating it first when it
// has reached tenancy.LedgerRotateBytes (one generation kept: <ledger>.1).
func appendLedgerBytes(ledger string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(ledger), 0o700); err != nil {
		return err
	}
	if fi, err := os.Stat(ledger); err == nil && fi.Size() >= tenancy.LedgerRotateBytes {
		if err := os.Rename(ledger, ledger+".1"); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(ledger, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// appendLifecycle appends one row.
func appendLifecycle(ledger string, row tenancy.LifecycleRow) error {
	b, err := json.Marshal(row)
	if err != nil {
		return err
	}
	return appendLedgerBytes(ledger, b)
}

// openLedgerGenerations reads the rotated generation then the current one.
// A missing file is empty; any other open error is returned (I38).
func openLedgerGenerations(ledger string) (io.Reader, func(), error) {
	var readers []io.Reader
	var files []*os.File
	closeAll := func() {
		for _, f := range files {
			_ = f.Close()
		}
	}
	for _, p := range []string{ledger + ".1", ledger} {
		f, err := os.Open(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			closeAll()
			return nil, func() {}, err
		}
		files = append(files, f)
		// A torn last line of the old generation must not join the first
		// line of the new one.
		readers = append(readers, f, strings.NewReader("\n"))
	}
	return io.MultiReader(readers...), closeAll, nil
}

// readLifecycle reads both generations of a ledger, oldest first.
func readLifecycle(ledger string) ([]tenancy.LifecycleRow, int, error) {
	r, done, err := openLedgerGenerations(ledger)
	if err != nil {
		return nil, 0, err
	}
	defer done()
	return tenancy.ReadLedger(r)
}

// Evidence scan bounds (§2.3): the system's diagnostic reports, newest
// first, read up to evidenceReadCap each.
var (
	evidenceDirs = func() []string {
		dirs := []string{"/Library/Logs/DiagnosticReports"}
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dirs = append(dirs, filepath.Join(home, "Library", "Logs", "DiagnosticReports"))
		}
		return dirs
	}
	evidenceReadCap int64 = 4 << 20
)

// scanUncleanEvidence reads the JetsamEvent-* and fathom_*.diag reports in
// dirs modified since `since`, newest first, at most
// tenancy.MaxEvidenceFiles. Best effort: an unreadable directory or file
// is skipped (the row then carries no evidence, never a false one).
func scanUncleanEvidence(dirs []string, since time.Time) []tenancy.EvidenceFile {
	type cand struct {
		path, name string
		mod        time.Time
	}
	var cands []cand
	for _, dir := range dirs {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			n := e.Name()
			if e.IsDir() || !(strings.HasPrefix(n, "JetsamEvent-") || (strings.HasPrefix(n, "fathom_") && strings.HasSuffix(n, ".diag"))) {
				continue
			}
			info, err := e.Info()
			if err != nil || info.ModTime().Before(since) {
				continue
			}
			cands = append(cands, cand{filepath.Join(dir, n), n, info.ModTime()})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if !cands[i].mod.Equal(cands[j].mod) {
			return cands[i].mod.After(cands[j].mod)
		}
		return cands[i].name < cands[j].name
	})
	var out []tenancy.EvidenceFile
	for _, c := range cands {
		if len(out) == tenancy.MaxEvidenceFiles {
			break
		}
		f, err := os.Open(c.path)
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(f, evidenceReadCap))
		_ = f.Close()
		if err != nil {
			continue
		}
		out = append(out, tenancy.EvidenceFile{Name: c.name, Content: b})
	}
	return out
}

// lifecycleStart is beginLifecycle's input; every OS effect is injected.
type lifecycleStart struct {
	ledger      string // "" = no ledger (HOME unresolvable): nothing recorded, breaker off
	id          string
	managed     bool
	addr        string
	pid         int
	commit      string
	version     string
	plistSchema int
	now         time.Time
	breaker     tenancy.BreakerSettings
	alive       func(pid int) bool
	evidence    func(since time.Time) []tenancy.EvidenceFile
	switchDir   string                                  // ~/.fathom/daemon: where a managed breaker stop writes disabled.json
	relaunch    func(rec tenancy.SwitchRecord) []string // what launchd will do after the stop (RT-157's clause)
	logf        func(string)
}

// lifecycleOutcome is what serve-http does next.
type lifecycleOutcome struct {
	stop    bool // exit now with code (a breaker stop)
	code    int
	safe    *tenancy.SafeModeInfo
	verdict tenancy.BreakerVerdict
	unclean *tenancy.LifecycleRow
}

// beginLifecycle runs a start's ledger work in order: name an unclean
// predecessor, evaluate the breaker, act on it, and write the start row.
// An unreadable ledger is named and the breaker is skipped — fail open
// (the kill switch still holds), never a silent "no exits".
func beginLifecycle(in lifecycleStart) lifecycleOutcome {
	var out lifecycleOutcome
	if in.ledger == "" {
		return out
	}
	logf := in.logf
	if logf == nil {
		logf = func(string) {}
	}
	write := func(r tenancy.LifecycleRow) {
		if err := appendLifecycle(in.ledger, r); err != nil {
			logf(fmt.Sprintf("lifecycle: ledger %s not written: %v", in.ledger, err))
		}
	}
	rows, skipped, err := readLifecycle(in.ledger)
	if err != nil {
		logf(fmt.Sprintf("lifecycle: ledger %s unreadable (%v) — the breaker is not evaluated this start", in.ledger, err))
		rows = nil
	} else if skipped > 0 {
		logf(fmt.Sprintf("lifecycle: %d unreadable ledger line(s) skipped in %s", skipped, in.ledger))
	}
	if err == nil {
		if pred, ok := tenancy.UncleanPredecessor(rows, in.pid); ok && (in.alive == nil || !in.alive(pred.PID)) {
			var files []tenancy.EvidenceFile
			if in.evidence != nil {
				files = in.evidence(pred.Time())
			}
			ex := tenancy.SynthesizeUncleanExit(pred, rows, files, in.now, in.version)
			write(ex)
			rows = append(rows, ex)
			out.unclean = &ex
			msg := "lifecycle: predecessor exited unclean — " + ex.Detail
			if len(ex.Evidence) > 0 {
				msg += "; evidence: " + strings.Join(ex.Evidence, "; ")
			}
			if ex.NotCounted != "" {
				msg += "; not counted: " + ex.NotCounted
			}
			logf(msg)
		}
		out.verdict = tenancy.EvaluateBreaker(rows, in.now, in.breaker)
	}
	switch out.verdict.Action {
	case tenancy.BreakerStop:
		br := tenancy.NewLifecycleRow(tenancy.RowBreaker, in.now, in.pid, in.version)
		br.Reason, br.To = out.verdict.Reason, "stop"
		write(br)
		logf("lifecycle: breaker STOP — " + out.verdict.Reason + " — exiting 0")
		if in.managed && in.switchDir != "" {
			rec := tenancy.SwitchRecord{By: fmt.Sprintf("fathom breaker (pid %d)", in.pid), Reason: "breaker", Detail: out.verdict.Reason, Since: in.now.UTC(), Addr: in.addr}
			if p, werr := writeSwitchFile(in.switchDir, tenancy.DisabledFile, rec); werr != nil {
				logf(fmt.Sprintf("lifecycle: disabled.json not written (%v) — launchd may relaunch this daemon", werr))
			} else {
				logf("lifecycle: kill switch " + p + " written (fathom daemon start clears it and resets the breaker)")
			}
			if in.relaunch != nil {
				for _, l := range in.relaunch(rec) {
					logf("lifecycle: " + l)
				}
			}
		}
		out.stop = true
		return out
	case tenancy.BreakerSafe:
		br := tenancy.NewLifecycleRow(tenancy.RowBreaker, in.now, in.pid, in.version)
		br.Reason, br.To = out.verdict.Reason, tenancy.ModeSafe
		write(br)
		out.safe = &tenancy.SafeModeInfo{Since: in.now, Reason: out.verdict.Reason}
		logf("lifecycle: " + out.safe.Message())
	}
	st := tenancy.NewLifecycleRow(tenancy.RowStart, in.now, in.pid, in.version)
	st.BinaryCommit, st.Managed, st.PlistSchema, st.LedgerID = in.commit, in.managed, in.plistSchema, in.id
	st.Mode = tenancy.ModeNormal
	if out.safe != nil {
		st.Mode = tenancy.ModeSafe
	}
	write(st)
	return out
}

// recordDeployIntent writes a `deploy` row (the pid being replaced and the
// new binary's hash) before `task deploy` kickstarts the daemon (§8.5,
// RT-156): a kickstart whose SIGTERM does not reach a drained exit is then
// not counted by the breaker.
func recordDeployIntent(ledger string, replacedPID int, binary string, now time.Time) error {
	row := tenancy.NewLifecycleRow(tenancy.RowDeploy, now, 0, Version)
	row.ReplacedPID = replacedPID
	row.By = "task deploy"
	if h, err := hashFile(binary); err == nil {
		row.BinaryHash = shortHash(h)
	} else {
		row.Detail = "binary unreadable: " + err.Error()
	}
	return appendLifecycle(ledger, row)
}

// runDaemonDeployIntent is `fathom daemon deploy-intent [--label L]
// [--binary path]`: the deploy row for the managed daemon's ledger, the
// replaced pid read from its record. It never fails a deploy: every
// problem is printed and it exits 0, except a write error (1).
func runDaemonDeployIntent(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("daemon deploy-intent", flag.ContinueOnError)
	fs.SetOutput(stderr)
	label := fs.String("label", "", "launchd label (default: the installed fathom agent)")
	home, _ := os.UserHomeDir()
	binary := fs.String("binary", filepath.Join(home, ".local", "libexec", "fathom", "fathom"), "the binary being deployed")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if home == "" {
		fmt.Fprintln(stdout, "deploy-intent: HOME unresolvable — no intent row written")
		return 0
	}
	lbl := firstNonEmpty(*label, firstNonEmpty(labelInstalled(), defaultDaemonLabel))
	ledger, err := ledgerPathFor(home, lbl)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	pid := 0
	if recPath, rerr := daemonRecordPath(home, lbl); rerr == nil {
		if rec, err := console.ReadDaemonRecord(recPath); err == nil {
			pid = rec.PID
		}
	}
	if pid == 0 {
		fmt.Fprintf(stdout, "deploy-intent: no running daemon recorded for %s — nothing to replace, no intent row written\n", lbl)
		return 0
	}
	if err := recordDeployIntent(ledger, pid, *binary, time.Now()); err != nil {
		fmt.Fprintf(stderr, "error: deploy intent not written to %s: %v\n", ledger, err)
		return 1
	}
	fmt.Fprintf(stdout, "deploy-intent: pid %d will be replaced by %s — its death is not counted by the breaker (%s)\n", pid, *binary, ledger)
	return 0
}
