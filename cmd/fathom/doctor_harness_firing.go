package main

// doctor_harness_firing.go — HP-34: the two rows that let a NON-Claude
// harness be verified at all. Until this file, `hooks_installed` and
// `hooks_firing` hard-skipped every harness but Claude Code, which made
// the descriptor's `verified_against` unreachable by construction: the
// only evidence that could promote it was evidence doctor refused to
// look for. That is a circular gate, and this breaks the circle with the
// same discipline the Claude rows use — read real bytes, never guess.
//
// The evidence differs by necessity. Claude Code's firing evidence is the
// daemon's call log (origin `edit-hook`), which only tells you that SOME
// hook called the daemon; it cannot say which harness. The beat log can:
// every beat a hook verb writes now carries `harness` (beats.go), stamped
// from the `--harness` flag the wired command line spells, so a Codex beat
// and a Claude beat are distinguishable in one user-global log.

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"

	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// harnessHookEvidence is what the beat log knows about one harness.
type harnessHookEvidence struct {
	// channels maps a beat channel to how many beats that harness wrote on
	// it; the set of channels IS the set of hook verbs that ran.
	channels map[string]int
	total    int
	last     time.Time
	path     string
	readErr  error
}

// loadHarnessHookEvidence reads the central beat log and keeps the beats
// stamped for this harness id. A missing log is zero evidence, not an
// error (a machine that never ran the hooks is normal); a present but
// unreadable log is the error, reported as SKIPPED — never as absence
// (I38: an unreadable file is named, never counted as "nothing there").
func loadHarnessHookEvidence(id string) harnessHookEvidence {
	ev := harnessHookEvidence{channels: map[string]int{}, path: beatLogPath()}
	if ev.path == "" {
		return ev
	}
	f, err := os.Open(ev.path)
	if err != nil {
		if !os.IsNotExist(err) {
			ev.readErr = err
		}
		return ev
	}
	defer f.Close()
	beats, err := heartbeat.Load(f, true) // acceptPartial: a torn tail must not blank the history
	if err != nil {
		ev.readErr = err
		return ev
	}
	for _, b := range beats {
		if b.Meta["harness"] != id {
			continue
		}
		ev.channels[b.Channel]++
		ev.total++
		if b.At.After(ev.last) {
			ev.last = b.At
		}
	}
	return ev
}

// sortedChannels renders the evidence's channels deterministically.
func (e harnessHookEvidence) sortedChannels() string {
	names := make([]string, 0, len(e.channels))
	for k := range e.channels {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s×%d", n, e.channels[n]))
	}
	return strings.Join(parts, ", ")
}

// descriptorHooksVerifiable reports whether doctor may render a PASS for
// this harness's hook rows: a codec is wired AND a real session has
// verified the descriptor. An unverified descriptor keeps the wave-2
// behavior — checkHarnessHookFiles reads its files and reports
// `skipped: unverified`, never PASS (HP-12).
func descriptorHooksVerifiable(harnessLabel string) (harness.Descriptor, bool) {
	d, ok := harness.Lookup(doctorHarnessID(harnessLabel))
	if !ok || !d.HooksWired() || d.Unverified() {
		return harness.Descriptor{}, false
	}
	return d, true
}

// checkHarnessHooksInstalled is `hooks_installed` for a verified
// non-Claude descriptor: the descriptor's own hook files, read for
// fathom's markers, exactly as the Claude row reads .claude/settings.json.
func checkHarnessHooksInstalled(root string, d harness.Descriptor, parents ...context.Context) doctorCheck {
	const name = "hooks_installed"
	specs := hookSpecsFor(d.ID)
	present, fileNotes, uncertain, indirect := harnessHookEntriesByScope(root, d, specs, parents...)

	var missing, unknownLabels []string
	scopes := map[string]bool{}
	for _, spec := range specs {
		where := present[hookSpecKeyOf(spec)]
		if len(where) == 0 {
			if uncertain[hookSpecKeyOf(spec)] {
				unknownLabels = append(unknownLabels, specLabel(spec))
			}
			missing = append(missing, specLabel(spec))
			continue
		}
		for _, w := range where {
			scopes[w] = true
		}
	}
	found := len(specs) - len(missing)
	state := "configured"
	if indirect {
		state = "configured (indirect)"
	}
	reason := fmt.Sprintf("%s: %d of %d fathom hook entries wired for %s", state, found, len(specs), d.Display)
	if len(scopes) > 0 {
		// Name the file the entries were actually READ from, not the file
		// init would write: on Codex those are different files, and naming
		// the wrong one is how a doctor row sends someone to edit a file
		// that has nothing to do with the reading.
		names := make([]string, 0, len(scopes))
		for k := range scopes {
			names = append(names, k)
		}
		sort.Strings(names)
		reason += " (" + strings.Join(names, ", ") + ")"
	}
	if len(fileNotes) > 0 {
		reason += "; " + strings.Join(fileNotes, "; ")
	}
	if len(unknownLabels) > 0 {
		reason += "; uncertain project wrappers: " + strings.Join(unknownLabels, ", ")
	}
	if len(missing) > 0 {
		if len(unknownLabels) == len(missing) {
			return doctorCheck{Name: name, Status: doctorUnknown, Reason: "unknown: " + reason, Remedy: "inspect the named project wrappers manually"}
		}
		remedy := "fathom init --harness=" + d.ID + " --path " + root
		if _, writable := d.InitHookFile(); !writable {
			// init cannot write this harness's hook file (Codex loads only
			// the user-global layer), so prescribing init here would send
			// someone to a command that will not fix it.
			remedy = "fathom harness hooks --harness " + d.ID + " — merge the printed entries into the harness's own hooks file (fathom does not write user-global configuration)"
		}
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: reason + "; missing: " + strings.Join(missing, ", "),
			Remedy: remedy,
		}
	}
	for _, f := range d.HookFiles {
		if f.Kind != "" && f.Trust != "" {
			reason += "; note: " + f.Trust
			break
		}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// checkHarnessHooksFiring is `hooks_firing` for a verified non-Claude
// descriptor: has a REAL session of that harness ever run fathom's hooks
// on this machine? The beat log is the evidence and the `harness` stamp is
// what makes it about this harness rather than any other.
func checkHarnessHooksFiring(d harness.Descriptor) doctorCheck {
	const name = "hooks_firing"
	ev := loadHarnessHookEvidence(d.ID)
	switch {
	case ev.readErr != nil:
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("the beat log %s exists but could not be read (%v) — firings recorded there are invisible to this check, which is not the same as none", ev.path, ev.readErr),
			Remedy: "make " + ev.path + " readable, then re-run doctor",
		}
	case ev.total == 0:
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("no beat in %s carries harness=%s — no %s session has ever run fathom's hooks on this machine", ev.path, d.ID, d.Display),
			Remedy: fmt.Sprintf("open a %s session in this repo (the wired commands spell --harness %s) and edit a file, then re-run doctor", d.Display, d.ID),
		}
	}
	return doctorCheck{
		Name: name, Status: doctorPass,
		Reason: fmt.Sprintf("firing: %s: %d beat(s) stamped harness=%s across %s; last %s", ev.path, ev.total, d.ID, ev.sortedChannels(), ev.last.UTC().Format(time.RFC3339)),
	}
}
