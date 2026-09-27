package main

// daemon_lifecycle.go — `fathom daemon stop|start|pause|resume` and
// `daemon install --reconcile` (design §8.3, §9; RT-4, RT-5, RT-6, RT-14,
// RT-157).
//
// INC-23: the operator could not stop a 30 GB daemon. `stop` writes the
// kill switch FIRST (so any relaunch exits before config, store or bind),
// then kills the managed pid behind a pid-reuse guard — SIGTERM, 10 s,
// SIGKILL — and reports success only once the process is gone AND the port
// is closed, with the proof printed. Under an installed schema-1 plist it
// says plainly that launchd will keep relaunching (each start exiting in
// under 50 ms) and prints the bootout and reconcile lines instead of
// claiming the daemon stays down.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// Stop timings (design §9): SIGTERM, then SIGKILL after daemonStopTermWait;
// then up to daemonStopGoneWait for the process to vanish and the port to
// close. Variables so a test can shorten them.
var (
	daemonStopTermWait = 10 * time.Second
	daemonStopGoneWait = 5 * time.Second
	daemonStopPoll     = 100 * time.Millisecond
)

// tenancyContract is the responsible-tenant contract this binary speaks
// (`fathom version --json`). `daemon install --reconcile` refuses to write a
// schema-2 plist for a binary below 1: an older binary exits 0 on adoption,
// which a schema-2 KeepAlive would treat as "stay down". Contract 1 is true
// of this binary because watchOwnBinary exits 75 on adoption
// (TestSelfRestartAdoptsAVerifiedBinary holds the two together).
const tenancyContract = 1

// writeSwitchFile writes a switch record atomically under dir.
func writeSwitchFile(dir, name string, rec tenancy.SwitchRecord) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	p := filepath.Join(dir, name)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, tenancy.MarshalSwitch(rec), 0o600); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

// installedPosture reads the installed plist for label ("" = discover).
func installedPosture(label string) (lbl, plistPath string, post plistPostureReport, ok bool) {
	lbl = label
	if lbl == "" {
		lbl = labelInstalled()
	}
	if lbl == "" {
		return "", "", plistPostureReport{}, false
	}
	plistPath = filepath.Join(launchAgentsDir(), lbl+".plist")
	raw, err := os.ReadFile(plistPath)
	if err != nil {
		return lbl, plistPath, plistPostureReport{}, false
	}
	post, err = plistPosture(raw)
	if err != nil {
		return lbl, plistPath, plistPostureReport{}, false
	}
	return lbl, plistPath, post, true
}

// reconcileCommand is the exact remedy line for a drifted plist.
func reconcileCommand(label string, post plistPostureReport) string {
	cmd := "fathom daemon install --reconcile --label " + label
	if !post.FathomWritten {
		cmd += " --adopt"
	}
	return cmd
}

// reloadLines are the launchctl lines that load a rewritten plist.
func reloadLines(uid int, label, plistPath string) []string {
	return []string{
		fmt.Sprintf("launchctl bootout gui/%d/%s", uid, label),
		fmt.Sprintf("launchctl bootstrap gui/%d %s", uid, plistPath),
	}
}

// relaunchClause says what launchd will do after a stop, read from the
// installed plist rather than assumed (RT-157).
func relaunchClause(ops daemonOps, label string, rec tenancy.SwitchRecord) []string {
	lbl, _, post, ok := installedPosture(label)
	if ok && post.UnconditionalKeepAlive && !isFathomLaunchdLabel(post.Label) {
		// A schema-1 relaunch knows it is managed only by XPC_SERVICE_NAME
		// (the plist's Label key); one that is not recognised starts a FULL
		// daemon every throttle, so the fast-exit claim would be false.
		svc := firstNonEmpty(post.Label, lbl)
		return []string{
			fmt.Sprintf("launchd will relaunch every %d s and each relaunch will NOT see the kill switch: the plist's Label %q is not recognised as the managed daemon (schema %d: KeepAlive true, no --managed); stop it now from your terminal:", post.ThrottleSeconds, post.Label, post.Schema),
			fmt.Sprintf("  launchctl bootout gui/%d/%s", ops.UID(), svc),
			"  " + reconcileCommand(lbl, post) + "   # adds --managed; then reload",
		}
	}
	if ok && post.UnconditionalKeepAlive {
		return []string{
			fmt.Sprintf("launchd will relaunch every %d s and each start exits in < 50 ms (the installed plist is schema %d: KeepAlive true); to stop fully, run from your terminal:", post.ThrottleSeconds, post.Schema),
			fmt.Sprintf("  launchctl bootout gui/%d/%s", ops.UID(), lbl),
			"  " + reconcileCommand(lbl, post) + "   # then reload, so exit 0 stays down",
		}
	}
	if !rec.Until.IsZero() {
		return []string{fmt.Sprintf("launchd retries after its throttle; each start exits %d until %s", tenancy.ExitRetryLater, rec.Until.UTC().Format(time.RFC3339))}
	}
	return []string{"launchd will not relaunch: " + tenancy.DisabledFile}
}

// findManagedDaemon resolves the pid to stop for addr: the daemon record's
// pid when that process is still the one that wrote it (command is a
// serve-http and it started no later than the record — the pid-reuse
// guard), else the port's listener when it is a fathom serve-http.
func findManagedDaemon(ops daemonOps, addr string) (int, []string) {
	var notes []string
	home, _ := os.UserHomeDir()
	if p, err := console.DaemonRecordPath(home, addr); err == nil {
		if rec, err := console.ReadDaemonRecord(p); err == nil && rec.PID > 0 {
			pi, alive := ops.Proc(rec.PID)
			recorded, perr := time.Parse(time.RFC3339, rec.StartedAt)
			switch {
			case !alive:
				notes = append(notes, fmt.Sprintf("the record's pid %d is not running", rec.PID))
			case !strings.Contains(pi.Command, "serve-http"):
				notes = append(notes, fmt.Sprintf("pid %d from the record is now %q — pid reused, not signalled", rec.PID, pi.Command))
			case perr == nil && pi.Start.After(recorded.Add(2*time.Second)):
				notes = append(notes, fmt.Sprintf("pid %d started %s, after the record was written %s — pid reused, not signalled", rec.PID, pi.Start.UTC().Format(time.RFC3339), rec.StartedAt))
			default:
				return rec.PID, notes
			}
		}
	}
	if lp := ops.ListenerPID(addr); lp > 0 {
		pi, alive := ops.Proc(lp)
		if alive && strings.Contains(pi.Command, "serve-http") && strings.Contains(pi.Command, "fathom") {
			return lp, notes
		}
		cmd := pi.Command
		if !alive {
			cmd = "unknown"
		}
		notes = append(notes, fmt.Sprintf("%s is held by pid %d (%s) — not a fathom serve-http, not signalled", addr, lp, cmd))
	}
	return 0, notes
}

// waitGone polls until pid is not running or d elapses.
func waitGone(ops daemonOps, pid int, d time.Duration) bool {
	deadline := ops.Now().Add(d)
	for {
		if _, alive := ops.Proc(pid); !alive {
			return true
		}
		if !ops.Now().Before(deadline) {
			return false
		}
		ops.Sleep(daemonStopPoll)
	}
}

// waitPortClosed polls until nothing accepts on addr or d elapses.
func waitPortClosed(ops daemonOps, addr string, d time.Duration) bool {
	deadline := ops.Now().Add(d)
	for {
		if !ops.PortOpen(addr) {
			return true
		}
		if !ops.Now().Before(deadline) {
			return false
		}
		ops.Sleep(daemonStopPoll)
	}
}

func cmdDaemonStop(args []string, stdout, stderr io.Writer) int {
	return runDaemonStop(args, stdout, stderr, realDaemonOps{})
}

// runDaemonStop is `fathom daemon stop [--for dur] [--reason text]`.
func runDaemonStop(args []string, stdout, stderr io.Writer, ops daemonOps) int {
	fs := newFlagSet("daemon stop", stderr)
	forDur := fs.Duration("for", 0, "stop only for this long: until then each start exits 75 (retried later); after it, a normal start")
	reason := fs.String("reason", "", "why (recorded in disabled.json)")
	url := fs.String("url", defaultFathomURL(), "the managed daemon's base URL")
	label := fs.String("label", "", "launchd label (default: the installed fathom agent)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *forDur < 0 {
		fmt.Fprintln(stderr, "error: --for must be positive")
		return 2
	}
	dir, err := daemonStateDir()
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot resolve ~/.fathom/daemon: %v\n", err)
		return 1
	}
	addr, err := console.DaemonAddrFromURL(*url)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	now := ops.Now()
	rec := tenancy.SwitchRecord{By: fmt.Sprintf("fathom daemon stop (pid %d)", os.Getpid()), Reason: firstNonEmpty(*reason, "operator stop"), Since: now.UTC(), Addr: addr}
	if *forDur > 0 {
		rec.Until = now.Add(*forDur).UTC()
	}
	path, err := writeSwitchFile(dir, tenancy.DisabledFile, rec)
	if err != nil {
		fmt.Fprintf(stderr, "error: writing the kill switch: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "kill switch: %s written — every managed start now exits before config, store or bind\n", path)

	pid, notes := findManagedDaemon(ops, addr)
	for _, n := range notes {
		fmt.Fprintf(stdout, "note: %s\n", n)
	}
	_, port, _ := net.SplitHostPort(addr)
	if pid == 0 {
		if ops.PortOpen(addr) {
			fmt.Fprintf(stdout, "NOT STOPPED: :%s is still open and no fathom serve-http could be verified behind it\n", port)
			return 1
		}
		fmt.Fprintf(stdout, "no daemon running on %s; :%s closed\n", addr, port)
		for _, l := range relaunchClause(ops, *label, rec) {
			fmt.Fprintln(stdout, l)
		}
		return 0
	}

	start := ops.Now()
	sig := "SIGTERM"
	if err := ops.Signal(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		fmt.Fprintf(stderr, "error: SIGTERM pid %d: %v\n", pid, err)
		return 1
	}
	gone := waitGone(ops, pid, daemonStopTermWait)
	if !gone {
		sig = "SIGKILL"
		fmt.Fprintf(stdout, "pid %d ignored SIGTERM for %s — sending SIGKILL\n", pid, daemonStopTermWait)
		if err := ops.Signal(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			fmt.Fprintf(stderr, "error: SIGKILL pid %d: %v\n", pid, err)
			return 1
		}
		gone = waitGone(ops, pid, daemonStopGoneWait)
	}
	elapsed := ops.Now().Sub(start)
	closed := waitPortClosed(ops, addr, daemonStopGoneWait)
	if !gone || !closed {
		var what []string
		if !gone {
			what = append(what, fmt.Sprintf("pid %d is still running after %s", pid, sig))
		}
		if !closed {
			what = append(what, fmt.Sprintf(":%s is still open", port))
		}
		fmt.Fprintf(stdout, "NOT STOPPED: %s — the kill switch stays written\n", strings.Join(what, "; "))
		return 1
	}
	fmt.Fprintf(stdout, "pid %d gone after %.1f s (%s); :%s closed\n", pid, elapsed.Seconds(), sig, port)
	for _, l := range relaunchClause(ops, *label, rec) {
		fmt.Fprintln(stdout, l)
	}
	return 0
}

// appendLifecycleRow appends one row to the ledger for id
// (~/.fathom/daemon/<id>/lifecycle.jsonl, design §2.3; the row type and
// writer are selfrestart_tenant.go's, shared with the adopt row).
func appendLifecycleRow(dir, id string, row lifecycleRow) error {
	return appendLedgerRow(filepath.Join(dir, id, "lifecycle.jsonl"), row)
}

func cmdDaemonStart(args []string, stdout, stderr io.Writer) int {
	return runDaemonStart(args, stdout, stderr, realDaemonOps{})
}

// runDaemonStart is `fathom daemon start`: remove the kill switch, reset the
// breaker window (an operator_reset row) and kickstart the label — printed
// instead of run from an agent shell.
func runDaemonStart(args []string, stdout, stderr io.Writer, ops daemonOps) int {
	fs := newFlagSet("daemon start", stderr)
	label := fs.String("label", "", "launchd label (default: the installed fathom agent)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir, err := daemonStateDir()
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot resolve ~/.fathom/daemon: %v\n", err)
		return 1
	}
	p := filepath.Join(dir, tenancy.DisabledFile)
	switch err := os.Remove(p); {
	case err == nil:
		fmt.Fprintf(stdout, "kill switch: removed %s\n", p)
	case os.IsNotExist(err):
		fmt.Fprintf(stdout, "kill switch: none set (%s absent)\n", p)
	default:
		fmt.Fprintf(stderr, "error: removing %s: %v\n", p, err)
		return 1
	}
	lbl := *label
	if lbl == "" {
		lbl = labelInstalled()
	}
	id := firstNonEmpty(lbl, defaultDaemonLabel)
	if err := appendLifecycleRow(dir, id, lifecycleRow{SchemaVersion: 1, FathomVersion: Version, Row: "operator_reset", At: ops.Now().UTC().Format(time.RFC3339), By: "fathom daemon start"}); err != nil {
		fmt.Fprintf(stderr, "warning: the breaker window was not reset (%v)\n", err)
	} else {
		fmt.Fprintln(stdout, "breaker: window reset (operator_reset)")
	}
	if runtime.GOOS != "darwin" || lbl == "" {
		fmt.Fprintf(stdout, "no launchd agent installed — start it with: %s\n", daemonRestartHint(""))
		return 0
	}
	kick := []string{"kickstart", fmt.Sprintf("gui/%d/%s", ops.UID(), lbl)}
	if ops.AgentShell() {
		fmt.Fprintf(stdout, "run from your terminal (launchctl from an agent shell wedges):\n  launchctl %s\n", strings.Join(kick, " "))
		return 0
	}
	if out, err := ops.Run("launchctl", kick...); err != nil {
		fmt.Fprintf(stderr, "launchctl %s failed: %v %s\n", strings.Join(kick, " "), err, strings.TrimSpace(string(out)))
		return 1
	}
	fmt.Fprintf(stdout, "kickstarted %s\n", lbl)
	return 0
}

// runDaemonPause is `fathom daemon pause|resume`: paused.json sets (and its
// removal clears) the managed daemon's safe-mode flag on its next 2 s tick,
// without stopping it. In wave A nothing reads the flag yet — its effects
// are RT-39 (applySafeMode) and RT-150 (the ensure skip) — so the output
// says pause records the intent only.
func runDaemonPause(pause bool, args []string, stdout, stderr io.Writer, ops daemonOps) int {
	name := "daemon resume"
	if pause {
		name = "daemon pause"
	}
	fs := newFlagSet(name, stderr)
	reason := fs.String("reason", "", "why (recorded in paused.json)")
	url := fs.String("url", defaultFathomURL(), "the managed daemon's base URL (recorded as addr: hooks addressing it go quiet)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	dir, err := daemonStateDir()
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot resolve ~/.fathom/daemon: %v\n", err)
		return 1
	}
	if pause {
		addr, err := console.DaemonAddrFromURL(*url)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		p, err := writeSwitchFile(dir, tenancy.PausedFile, tenancy.SwitchRecord{By: "fathom daemon pause", Reason: firstNonEmpty(*reason, "operator pause"), Since: ops.Now().UTC(), Addr: addr})
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "paused: %s written — the managed daemon marks itself paused within 2 s (logged) and stays up; `fathom daemon resume` clears it\n", p)
		fmt.Fprintln(stdout, "note: pausing records the intent only in this release — auto-refresh and heavy requests are not yet curtailed (RT-39, RT-150); `fathom daemon stop` is the switch that takes load off now")
		return 0
	}
	p := filepath.Join(dir, tenancy.PausedFile)
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "error: removing %s: %v\n", p, err)
		return 1
	}
	fmt.Fprintf(stdout, "resumed: %s removed — the managed daemon clears its paused mark within 2 s\n", p)
	return 0
}

// installedTenancyContract asks the installed binary for its contract.
func installedTenancyContract(ops daemonOps, bin string) (int, string) {
	out, err := ops.Run(bin, "version", "--json")
	var v struct {
		Version         string `json:"version"`
		TenancyContract int    `json:"tenancy_contract"`
	}
	if err != nil || json.Unmarshal(out, &v) != nil {
		return 0, strings.TrimSpace(firstLine(string(out)))
	}
	return v.TenancyContract, v.Version
}

// plistReceipt is the record `--reconcile` leaves of what it changed.
type plistReceipt struct {
	SchemaVersion   int      `json:"schema_version"`
	At              string   `json:"at"`
	FathomVersion   string   `json:"fathom_version"`
	Label           string   `json:"label"`
	Plist           string   `json:"plist"`
	Backup          string   `json:"backup"`
	Adopted         bool     `json:"adopted"`
	Binary          string   `json:"binary"`
	TenancyContract int      `json:"tenancy_contract"`
	Changes         []string `json:"changes"`
	SHA256Before    string   `json:"sha256_before"`
	SHA256After     string   `json:"sha256_after"`
	Reload          []string `json:"reload"`
}

// runDaemonReconcile is `fathom daemon install --reconcile [--adopt]`.
func runDaemonReconcile(args []string, stdout, stderr io.Writer, ops daemonOps) int {
	fs := newFlagSet("daemon install --reconcile", stderr)
	label := fs.String("label", "", "launchd label (default: the installed fathom agent)")
	adopt := fs.Bool("adopt", false, "reconcile a plist fathom did not write (keeps every non-tenancy key)")
	fs.Bool("reconcile", true, "rewrite only the tenancy keys of the installed plist to schema 2")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	lbl := *label
	if lbl == "" {
		lbl = labelInstalled()
	}
	if lbl == "" {
		fmt.Fprintln(stderr, "fathom daemon install --reconcile: no fathom launchd agent is installed — `fathom daemon install` writes a schema-2 one")
		return 1
	}
	plistPath := filepath.Join(launchAgentsDir(), lbl+".plist")
	raw, err := os.ReadFile(plistPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: reading %s: %v\n", plistPath, err)
		return 1
	}
	post, err := plistPosture(raw)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s: %v\n", plistPath, err)
		return 1
	}
	if len(post.Drift) == 0 {
		fmt.Fprintf(stdout, "%s is already schema %d — nothing to reconcile\n", plistPath, plistSchemaCurrent)
		return 0
	}
	// Order (design §8.3): the binary the plist runs must speak the tenancy
	// contract first — an older binary exits 0 on self-restart, which a
	// schema-2 KeepAlive reads as "stay down".
	contract, ver := installedTenancyContract(ops, post.Binary)
	if contract < tenancyContract {
		fmt.Fprintf(stderr, "fathom daemon install --reconcile: refused — the installed binary %s reports tenancy_contract %d (%s), want >= %d.\n", post.Binary, contract, firstNonEmpty(ver, "no version --json"), tenancyContract)
		fmt.Fprintln(stderr, "  order: task deploy (install the new binary), confirm with `fathom daemon status`, then reconcile, then reload")
		return 1
	}
	out, changes, err := reconcilePlist(raw, *adopt)
	if errors.Is(err, errPlistNeedsAdopt) {
		fmt.Fprintf(stderr, "fathom daemon install --reconcile: %s: %v\n", plistPath, err)
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %s: %v\n", plistPath, err)
		return 1
	}
	ts := ops.Now().UTC().Format("20060102T150405Z")
	// The backup goes with the receipt under ~/.fathom/daemon/<label>/,
	// never beside the live plist: LaunchAgents is launchd's directory, and
	// a copy there carries the same Label with the v1 KeepAlive.
	stateDir, err := daemonStateDir()
	if err != nil {
		fmt.Fprintf(stderr, "error: cannot resolve ~/.fathom/daemon for the backup: %v — the plist is untouched\n", err)
		return 1
	}
	backup := filepath.Join(stateDir, lbl, lbl+".plist.bak."+ts)
	if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
		fmt.Fprintf(stderr, "error: %v — the plist is untouched\n", err)
		return 1
	}
	if err := os.WriteFile(backup, raw, 0o600); err != nil {
		fmt.Fprintf(stderr, "error: writing the backup %s: %v — the plist is untouched\n", backup, err)
		return 1
	}
	tmp := plistPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: %v — the plist is untouched\n", err)
		return 1
	}
	if err := os.Rename(tmp, plistPath); err != nil {
		fmt.Fprintf(stderr, "error: %v — the plist is untouched\n", err)
		return 1
	}
	reload := reloadLines(ops.UID(), lbl, plistPath)
	rec := plistReceipt{
		SchemaVersion: 1, At: ops.Now().UTC().Format(time.RFC3339), FathomVersion: Version,
		Label: lbl, Plist: plistPath, Backup: backup, Adopted: *adopt && !post.FathomWritten,
		Binary: post.Binary, TenancyContract: contract, Changes: changes,
		SHA256Before: sha256Hex(string(raw)), SHA256After: sha256Hex(string(out)), Reload: reload,
	}
	receiptPath := filepath.Join(stateDir, lbl, "reconcile-"+ts+".json")
	body, _ := json.MarshalIndent(rec, "", "  ")
	if err := os.WriteFile(receiptPath, append(body, '\n'), 0o600); err != nil {
		fmt.Fprintf(stderr, "warning: the receipt was not written (%v); the backup is %s\n", err, backup)
		receiptPath = ""
	}
	fmt.Fprintf(stdout, "reconciled %s to schema %d (backup %s)\n", plistPath, plistSchemaCurrent, backup)
	for _, c := range changes {
		fmt.Fprintf(stdout, "  %s\n", c)
	}
	if receiptPath != "" {
		fmt.Fprintf(stdout, "receipt: %s\n", receiptPath)
	}
	if ops.AgentShell() {
		fmt.Fprintln(stdout, "reload from your terminal (launchctl from an agent shell wedges):")
		for _, l := range reload {
			fmt.Fprintf(stdout, "  %s\n", l)
		}
		return 0
	}
	_, _ = ops.Run("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", ops.UID(), lbl))
	if o, err := ops.Run("launchctl", "bootstrap", fmt.Sprintf("gui/%d", ops.UID()), plistPath); err != nil {
		fmt.Fprintf(stderr, "launchctl bootstrap failed: %v %s\n  run it yourself: %s\n", err, strings.TrimSpace(string(o)), reload[1])
		return 1
	}
	fmt.Fprintf(stdout, "reloaded %s\n", lbl)
	return 0
}
