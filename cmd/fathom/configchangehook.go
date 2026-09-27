package main

// configchangehook.go — PC-24 (docs/vision/26-proof-credibility.md §4):
// `fathom config audit --hook`, the harness's ConfigChange event. A
// configuration file changed mid-session, so the custodian audit runs
// once and leaves one beat; the next session's attention line (EP-19,
// attentionLine) carries the violated count it recomputes from the same
// concerns. It never blocks — the verified contract says a block on this
// event surfaces NO message anywhere (debug log only), which would be a
// silent refusal, the one thing the custodian may not be (vision 22 §3b:
// fathom reports, the hand decides). Exit 0 always.

import (
	"io"
	"os"
	"strconv"

	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// configChangeBeatKind is the custodian channel's kind for this event;
// meta carries source, file_path, result and (when audited) violated.
const configChangeBeatKind = "config-change"

// cmdConfigAuditHook is the `--hook` branch of cmdConfigAudit: resolve the
// codec (I9 fail-open on an unknown id), read the envelope from stdin,
// run configChangeHook.
func cmdConfigAuditHook(harnessFlag, event string, stderr io.Writer) int {
	if fathomChannelsDisabled() {
		return 0 // FATHOM_CHANNELS=off: total silence
	}
	codec, ok := resolveHookHarness(harnessFlag, stderr)
	if !ok {
		return 0
	}
	marker := "fathom config audit --hook"
	if event == "file-changed" {
		marker += " --event file-changed"
	}
	endGuard, silent := beginHookGuard("config-change-hook", marker, codec.ID(), defaultFathomURL())
	if silent {
		return 0 // §9: the managed daemon is disabled or paused — no RPC, no output
	}
	defer endGuard()
	if event == "file-changed" {
		return fileChangedHook(codec, os.Stdin)
	}
	return configChangeHook(codec, os.Stdin)
}

// fileChangedHook is HP-28: the harness's FileChanged event — a governed
// configuration file changed ON DISK rather than through the harness (an
// edit by a sibling agent, a rebase, a merge). It is deliberately the
// SAME audit and the SAME beat ConfigChange writes, distinguished only by
// the beat's trigger meta: the custodian's question ("is what this repo
// instructs still true?") does not change with how the file got edited,
// and two divergent implementations of one question is how the two
// answers start disagreeing. Exit 0 always.
func fileChangedHook(codec harness.Codec, stdin io.Reader) int {
	return auditConfigChange(codec, stdin, harness.EventFileChanged, "file-changed")
}

// configChangeHook audits the repo the event belongs to and writes the
// beat. The root is the harness's project dir, else the envelope's cwd,
// else the process cwd. A concerns file that will not load is recorded
// as unable-to-check (a doctor finding, not a hook alarm); a repo with no
// concerns records that it had nothing to hold the change to.
func configChangeHook(codec harness.Codec, stdin io.Reader) int {
	return auditConfigChange(codec, stdin, harness.EventConfigChange, "")
}

// auditConfigChange is the shared body of both custodian events. trigger
// is "" for ConfigChange (the original event, whose beats must keep their
// exact shape) and names the other event otherwise.
func auditConfigChange(codec harness.Codec, stdin io.Reader, ev harness.Event, trigger string) int {
	if stdin == nil {
		return 0
	}
	raw, err := io.ReadAll(io.LimitReader(stdin, 1<<16))
	if err != nil {
		return 0
	}
	in, err := codec.DecodeInput(ev, raw)
	if err != nil {
		return 0
	}
	root := harnessProjectDir(codec.ID())
	if root == "" {
		root = in.Cwd
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	meta := map[string]string{}
	if trigger != "" {
		meta["trigger"] = trigger
	}
	if in.Source != "" {
		meta["source"] = in.Source
	}
	if in.FilePath != "" {
		meta["file_path"] = in.FilePath
	}
	concerns, loadErrs := loadConcernsBoth(root)
	switch {
	case len(loadErrs) > 0:
		meta["result"] = "unable-to-check"
	case len(concerns) == 0:
		meta["result"] = "no-concerns"
	default:
		violated := 0
		for _, f := range custodian.Audit(root, concerns) {
			if f.Status == custodian.StatusViolated {
				violated++
			}
		}
		meta["result"] = "audited"
		meta["violated"] = strconv.Itoa(violated)
	}
	emitBeat("custodian", configChangeBeatKind, firingMeta(meta, in.SessionID, ""))
	return 0
}
