package main

// loopback_bind.go — ADR-083: the daemon has no authentication, so it
// listens on loopback only. `daemon install` defaults to 127.0.0.1:7575 and
// refuses a wider address (exit 2); serve-http given a wider --addr or
// [daemon].addr binds 127.0.0.1 on the same port in its place and warns,
// so an install written before (--addr :7575) keeps serving after an
// upgrade; `--reconcile` rewrites a wide plist to loopback; and doctor's
// daemon_bind row fails on a wide plist or a running listener beyond
// loopback. No flag or key widens the bind: remote serving waits on token
// auth (Q5). requireLoopbackHost refuses a network peer whatever Host it
// sends (admission_http.go).

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
)

// defaultDaemonAddr is the address `daemon install` writes and the
// off-macOS supervisor line names.
const defaultDaemonAddr = "127.0.0.1:7575"

// remoteServingReason is why fathom listens on loopback only.
const remoteServingReason = "remote serving needs token auth, which fathom does not have yet (open question Q5)"

// loopbackAddrFor is addr's port on 127.0.0.1 (7575 when addr names none).
func loopbackAddrFor(addr string) string {
	if _, port, err := net.SplitHostPort(addr); err == nil && port != "" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return defaultDaemonAddr
}

// nonLoopbackInstallRefusal is `daemon install`'s refusal of a wide --addr.
func nonLoopbackInstallRefusal(addr string) string {
	return fmt.Sprintf("--addr %s is not loopback — refused: %s. The daemon has no authentication, so every peer that reaches the port could call every tool (ingest and remove_service included). Install with --addr %s",
		addr, remoteServingReason, loopbackAddrFor(addr))
}

// nonLoopbackRebindWarning is serve-http's one warning when it binds
// loopback in place of the configured address; source is "--addr" or
// "[daemon].addr".
func nonLoopbackRebindWarning(source, addr string) string {
	return fmt.Sprintf("%s %s is not loopback: listening on %s instead (%s; the daemon has no authentication). Run `fathom daemon install --reconcile` to rewrite the installed plist's --addr, or set [daemon].addr in the user config to a loopback address",
		source, addr, loopbackAddrFor(addr), remoteServingReason)
}

// plistAddrArg finds serve-http's --addr in ProgramArguments: the element
// holding the value (the flag's own element for the inline --addr=V form),
// the value, and whether it is inline.
func plistAddrArg(args *plistNode) (node *plistNode, value string, inline, ok bool) {
	for i, c := range args.Children {
		if c.Kind != "string" {
			continue
		}
		for _, p := range []string{"--addr", "-addr"} {
			if c.Text == p && i+1 < len(args.Children) && args.Children[i+1].Kind == "string" {
				return args.Children[i+1], args.Children[i+1].Text, false, true
			}
			if v, cut := strings.CutPrefix(c.Text, p+"="); cut {
				return c, v, true, true
			}
		}
	}
	return nil, "", false, false
}

// wideAddrDrift is the posture's drift line for a wide --addr.
func wideAddrDrift(addr string) string {
	return fmt.Sprintf("ProgramArguments: --addr %s is not loopback (want %s: the daemon has no authentication, and serve-http binds that in its place until reconciled)", addr, loopbackAddrFor(addr))
}

// daemonBindFacts is what doctor's daemon_bind row judges.
type daemonBindFacts struct {
	Label, PlistPath string
	// Post is the installed plist's posture; nil when no agent is
	// installed or it cannot be read (PlistErr says why).
	Post     *plistPostureReport
	PlistErr string
	// Rows is the daemon census (ps joined with the records).
	Rows        []censusRow
	CensusTaken bool
	CensusErr   string
	// ConfigAddr is [daemon].addr in the user config at ConfigPath (a
	// repository's is dropped at load); ConfigErr is why it did not load.
	ConfigPath, ConfigAddr, ConfigErr string
}

// daemonBindCheck is doctor's daemon_bind row: FAIL when the installed
// plist passes a wide --addr or a running serve-http listens beyond
// loopback, naming each with the one command that fixes it; WARN when it
// cannot read the installed plist or the user config, or the user config's
// [daemon].addr is wide (serve-http binds loopback in its place), so no
// unread input passes silently (I38). Pure.
func daemonBindCheck(f daemonBindFacts) doctorCheck {
	const name = "daemon_bind"
	var warns []string
	warnRemedy := ""
	warn := func(line, fix string) {
		warns = append(warns, line)
		if warnRemedy == "" {
			warnRemedy = fix
		}
	}
	if f.PlistErr != "" {
		warn(fmt.Sprintf("cannot read the installed agent's plist %s: %s", f.PlistPath, f.PlistErr), "plutil -lint "+f.PlistPath)
	}
	if f.ConfigErr != "" {
		warn(fmt.Sprintf("the user config %s did not load, so its [daemon].addr is unread: %s", f.ConfigPath, f.ConfigErr), "fathom config")
	}
	if f.ConfigAddr != "" && !hostIsLoopback(f.ConfigAddr) {
		to := loopbackAddrFor(f.ConfigAddr)
		warn(fmt.Sprintf("%s sets [daemon].addr %s, which is not loopback: serve-http binds %s in its place", f.ConfigPath, f.ConfigAddr, to),
			fmt.Sprintf("set addr = %q under [daemon] in %s", to, f.ConfigPath))
	}
	if f.Post == nil && !f.CensusTaken && len(warns) == 0 {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no fathom launchd agent installed and the daemon census was not taken"}
	}
	var fails []string
	remedy := ""
	fail := func(line, fix string) {
		fails = append(fails, line)
		if remedy == "" {
			remedy = fix
		}
	}
	if f.Post != nil && f.Post.Addr != "" && !hostIsLoopback(f.Post.Addr) {
		fail(fmt.Sprintf("%s passes --addr %s (serve-http binds %s in its place until reconciled)", f.PlistPath, f.Post.Addr, loopbackAddrFor(f.Post.Addr)), reconcileCommand(f.Label, *f.Post))
	}
	var wideRows []censusRow
	for _, r := range f.Rows {
		if r.Record == "stale" || r.Addr == "" || hostIsLoopback(r.Addr) {
			continue
		}
		wideRows = append(wideRows, r)
	}
	sort.Slice(wideRows, func(i, j int) bool { return wideRows[i].PID < wideRows[j].PID })
	for _, r := range wideRows {
		if r.Managed {
			fix := fmt.Sprintf("kill %d", r.PID)
			if f.Label != "" {
				fix = fmt.Sprintf("launchctl kickstart -k gui/$(id -u)/%s", f.Label)
			}
			fail(fmt.Sprintf("pid %d listens on %s (managed)", r.PID, r.Addr), fix)
			continue
		}
		fail(fmt.Sprintf("pid %d listens on %s (unmanaged)", r.PID, r.Addr), fmt.Sprintf("kill %d", r.PID))
	}
	if len(fails) > 0 {
		return doctorCheck{Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("fathom listens, or is installed to ask to listen, beyond loopback, and the daemon has no authentication: %s", strings.Join(append(fails, warns...), "; ")),
			Remedy: remedy}
	}
	if f.CensusErr != "" {
		warn("the daemon census failed: "+f.CensusErr, "fathom daemon ps")
	}
	if len(warns) > 0 {
		return doctorCheck{Name: name, Status: doctorWarn, Reason: strings.Join(warns, "; "), Remedy: warnRemedy}
	}
	reason := "every fathom listener is loopback"
	if f.Post != nil {
		reason = fmt.Sprintf("the installed agent listens on %s and no running serve-http listens beyond loopback", firstNonEmpty(f.Post.Addr, "its [daemon].addr, else "+defaultDaemonAddr))
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// checkDaemonBind is daemon_bind over this machine: the installed plist,
// the census doctor's resources projection took, and the user config.
func checkDaemonBind(rep resourcesReport) doctorCheck {
	f := daemonBindFacts{CensusTaken: rep.Scratch.Taken, CensusErr: rep.Scratch.Error, Rows: rep.Scratch.rows}
	if lbl, plistPath, post, err := installedPostureErr(""); lbl != "" {
		f.Label, f.PlistPath = lbl, plistPath
		if err != nil {
			f.PlistErr = err.Error()
		} else {
			f.Post = &post
		}
	}
	f.ConfigPath = config.DefaultUserPath()
	if loaded, err := config.Load(f.ConfigPath, ""); err != nil {
		f.ConfigErr = err.Error()
	} else {
		f.ConfigAddr = loaded.Config.Daemon.Addr
	}
	return daemonBindCheck(f)
}
