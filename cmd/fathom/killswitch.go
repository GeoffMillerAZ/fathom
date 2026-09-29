package main

// killswitch.go — fathom's own off switch (design §9, RT-3, RT-167, RT-6).
//
// INC-23 (2026-09-26): the daemon reached ~30 GB RSS and the operator could
// not stop it — launchd's unconditional KeepAlive relaunched every kill and
// `launchctl bootout` failed from a broken session. The switch is a file:
// `~/.fathom/daemon/disabled.json`, read by the managed daemon as its FIRST
// action — before config, store or bind — so a relaunch costs a stat, a small
// read and an exit (< 50 ms), and nothing else has to work for it to hold.
//
// The switch files govern only the MANAGED daemon (RT-167): a proof agent's
// scratch serve-http never reads them, so a breaker stop cannot block it.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// daemonStateDir is ~/.fathom/daemon — where the switch files live.
func daemonStateDir() (string, error) {
	dir := fathomhome.Path("daemon")
	if dir == "" {
		return "", fmt.Errorf("cannot resolve the fathom home directory (no $HOME, no %s)", fathomhome.Env)
	}
	return dir, nil
}

// isManagedInvocation reports whether this serve-http is the managed daemon:
// started with `--managed` (plist schema 2), or under launchd with an
// XPC_SERVICE_NAME equal to the label of an installed fathom agent (a
// schema-1 plist predates the flag; design §10.1). It scans raw args
// because it runs before flag parsing.
func isManagedInvocation(args []string, getenv func(string) string) bool {
	managed := false
	for _, a := range args {
		if a == "--" {
			break
		}
		name, val, hasVal := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if !strings.HasPrefix(a, "-") || name != "managed" {
			continue
		}
		managed = !hasVal || val == "true" || val == "1" || val == "t" || val == "T" || val == "TRUE" || val == "True"
	}
	if managed {
		return true
	}
	return isFathomLaunchdLabel(getenv("XPC_SERVICE_NAME"))
}

// isFathomLaunchdLabel reports whether a launchd job label is the managed
// fathom daemon's: the default label, or the Label key of an installed
// fathom serve-http agent in ~/Library/LaunchAgents. The label is the
// adopter's (`daemon install --label`, ADR-011), so no substring of it is
// assumed. A terminal's XPC_SERVICE_NAME ("0", "application.…") is never
// an installed agent's label. Cost: one directory read and a small read
// per fathom plist — well inside the switch's 50 ms.
func isFathomLaunchdLabel(label string) bool {
	if label == "" || label == "0" || strings.HasPrefix(label, "application.") {
		return false
	}
	if label == defaultDaemonLabel {
		return true
	}
	return slices.Contains(installedAgentLabels(), label)
}

// installedAgentLabels returns the Label key of every installed fathom
// serve-http agent (the value launchd gives a job as XPC_SERVICE_NAME),
// sorted. A plist that does not parse or has no Label is skipped.
func installedAgentLabels() []string {
	dir := launchAgentsDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var labels []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || !bytes.Contains(raw, []byte("serve-http")) || !bytes.Contains(raw, []byte("fathom")) {
			continue
		}
		if post, err := plistPosture(raw); err == nil && post.Label != "" {
			labels = append(labels, post.Label)
		}
	}
	sort.Strings(labels)
	return labels
}

// readSwitchFile stats then reads one switch file. Only "no such file"
// (or a non-directory in the path, under which none can exist) is absent:
// any other stat or read failure is present with the reason unknown and the
// error named (I38) — the operator's switch still holds rather than being
// ignored because it is unreadable.
func readSwitchFile(path string) tenancy.SwitchFile {
	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return tenancy.SwitchFile{}
		}
		return tenancy.SwitchFile{Present: true, ReadErr: err.Error()}
	}
	if fi.Size() > 64<<10 {
		return tenancy.SwitchFile{Present: true, ReadErr: fmt.Sprintf("%d bytes, over the 64 KiB read cap", fi.Size())}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return tenancy.SwitchFile{Present: true, ReadErr: err.Error()}
	}
	return tenancy.SwitchFile{Present: true, Raw: raw}
}

// readKillSwitch evaluates the switch files under dir at now.
func readKillSwitch(dir string, now time.Time) tenancy.KillSwitchVerdict {
	return tenancy.EvaluateKillSwitch(tenancy.KillSwitchInput{
		Disabled: readSwitchFile(filepath.Join(dir, tenancy.DisabledFile)),
		Paused:   readSwitchFile(filepath.Join(dir, tenancy.PausedFile)),
		Now:      now,
	})
}

// daemonSafeMode is true while paused.json holds (RT-6); it is mirrored
// into tenancy.SetPausedSafeMode, which the request path reads (RT-39:
// tree and fleet refused, auto-refresh, feeders and the decision port off).
var daemonSafeMode atomic.Bool

// safeModeActive reports whether the managed daemon is paused.
func safeModeActive() bool { return daemonSafeMode.Load() }

// checkKillSwitch is serve-http's first action. It returns (code, true) when
// the process must exit now with code, having written one line to stderr.
// It writes nothing to disk (RT-157: under a schema-1 plist launchd relaunches
// every 10 s, and ~360 starts an hour must not flood a ledger).
func checkKillSwitch(args []string, getenv func(string) string, now time.Time, stderr io.Writer) (int, bool) {
	if !isManagedInvocation(args, getenv) {
		return 0, false
	}
	dir, err := daemonStateDir()
	if err != nil {
		return 0, false // I9: fail open — no HOME, no switch to read
	}
	v := readKillSwitch(dir, now)
	switch v.Action {
	case tenancy.ActionExitStay, tenancy.ActionExitRetry:
		fmt.Fprintln(stderr, v.Message)
		return v.ExitCode, true
	case tenancy.ActionSafeMode:
		daemonSafeMode.Store(true)
		tenancy.SetPausedSafeMode(true) // RT-39: the modes the request path reads
		fmt.Fprintln(stderr, v.Message)
	case tenancy.ActionRun:
		if v.Expired {
			fmt.Fprintln(stderr, v.Message)
		}
	}
	return 0, false
}

// watchPauseSwitch re-reads the switch files every tick (2 s) while the
// managed daemon serves, moving to and from safe mode on paused.json and
// logging each transition once. A disabled.json that appears while serving
// is `fathom daemon stop`'s job (it signals the process); the tick does not
// exit on it.
func watchPauseSwitch(dir string, tick time.Duration, now func() time.Time, log *dlog.Logger, stop <-chan struct{}) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		v := readKillSwitch(dir, now())
		paused := v.Action == tenancy.ActionSafeMode
		tenancy.SetPausedSafeMode(paused)
		if daemonSafeMode.Swap(paused) != paused && log != nil {
			if paused {
				log.Warn(v.Message)
			} else {
				log.Info("fathom: resumed — paused.json removed; safe-mode flag cleared")
			}
		}
	}
}
