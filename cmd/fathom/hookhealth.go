package main

// Advisory transition stamps share CB-1's directory and nonblocking lock.
// Each channel has fixed-size state; invocation counts belong to the hook log.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

type hookHealthStamp struct {
	LastState  string    `json:"last_state"`
	ChangedAt  time.Time `json:"changed_at"`
	LastEmitAt time.Time `json:"last_emit_at"`
}

type hookHealthState struct {
	Version  int                                    `json:"version"`
	Channels map[harness.Capability]hookHealthStamp `json:"channels"`
}

func hookHealthPath(root, id string) string {
	return fathomhome.Path("hook-dedupe", "health-"+hookDigest(id+"\x00"+filepath.Clean(root))+".json")
}

func loadHookHealth(f *os.File, now time.Time) (hookHealthState, error) {
	empty := hookHealthState{Version: 2, Channels: map[harness.Capability]hookHealthStamp{}}
	raw, err := io.ReadAll(io.LimitReader(f, hookDedupeMaxBytes+1))
	if err != nil {
		return empty, err
	}
	if len(raw) == 0 {
		return empty, nil
	}
	var s hookHealthState
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if len(raw) > hookDedupeMaxBytes || dec.Decode(&s) != nil || dec.Decode(new(any)) != io.EOF || s.Version != 2 || s.Channels == nil || len(s.Channels) > len(hookReadinessRegistry()) {
		return empty, nil
	}
	for cap, stamp := range s.Channels {
		_, known := hookReadinessRegistry()[cap]
		if !known || stamp.ChangedAt.IsZero() || stamp.ChangedAt.After(now) || stamp.LastEmitAt.After(now) || !validHealthState(stamp.LastState) {
			return empty, nil
		}
	}
	return s, nil
}

func validHealthState(state string) bool {
	return state == "ready" || state == "not-ready:no-instructions" || state == "not-ready:hook-not-registered" || state == "not-ready:unsupported-here"
}

// Oversize/corrupt advisory contents are reset by loadHookHealth. File identity,
// ownership and permissions still fail closed for bookkeeping, never for advice.
func openHookHealth(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0) {
		err = os.ErrInvalid
	}
	if err == nil {
		if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
			err = os.ErrPermission
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func readHookHealth(root, id string, now time.Time) (hookHealthState, error) {
	f, err := openHookHealth(hookHealthPath(root, id))
	if err != nil {
		return hookHealthState{}, err
	}
	defer f.Close()
	return loadHookHealth(f, now)
}

func updateHookHealth(root, id string, cap harness.Capability, emitted bool, reason string, now time.Time) error {
	if _, ok := hookReadinessRegistry()[cap]; !ok {
		return os.ErrInvalid
	}
	f, err := openHookHealth(hookHealthPath(root, id))
	if err != nil {
		return err
	}
	defer f.Close()
	s, err := loadHookHealth(f, now)
	if err != nil {
		return err
	}
	state := "ready"
	if strings.HasPrefix(reason, "not-ready:") && validHealthState(reason) {
		state = reason
	}
	stamp := s.Channels[cap]
	changed := stamp.LastState != state
	if changed {
		stamp.LastState, stamp.ChangedAt = state, now
	}
	// Emission is coalesced by UTC day; skips never clear the last emit.
	dayChanged := emitted && (stamp.LastEmitAt.IsZero() || stamp.LastEmitAt.UTC().Format(time.DateOnly) != now.UTC().Format(time.DateOnly))
	if dayChanged {
		stamp.LastEmitAt = now
	}
	if !changed && !dayChanged {
		return nil
	}
	s.Channels[cap] = stamp
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if len(raw) > hookDedupeMaxBytes {
		return os.ErrInvalid
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err = f.Truncate(0); err != nil {
		return err
	}
	if n, err := f.Write(raw); err != nil {
		return err
	} else if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil // advisory state: never fsync
}

// One hook process runs one invocation; nested readout/test-run hooks
// restore their parent. Like beatHarness this is not daemon-owned state.
var activeAdviceHealth *adviceHealthInvocation

type adviceHealthInvocation struct {
	io.Writer
	root, id          string
	cap               harness.Capability
	emitted, recorded bool
	reason            string
	components        map[harness.Capability]bool
}

func (h *adviceHealthInvocation) Write(p []byte) (int, error) {
	n, err := h.Writer.Write(p)
	if err == nil && n == len(p) && n > 0 {
		h.emitted = true
	}
	return n, err
}

func beginAdviceHealth(root, id string, cap harness.Capability, out io.Writer) (io.Writer, func()) {
	h := &adviceHealthInvocation{Writer: out, root: root, id: id, cap: cap, reason: "unused"}
	prev := activeAdviceHealth
	activeAdviceHealth = h
	return h, func() {
		activeAdviceHealth = prev
		if !h.recorded {
			_ = updateHookHealth(root, id, cap, h.emitted, h.reason, nowFunc())
		}
		caps := make([]harness.Capability, 0, len(h.components))
		for cap := range h.components {
			caps = append(caps, cap)
		}
		sort.Slice(caps, func(i, j int) bool { return caps[i] < caps[j] })
		for _, cap := range caps {
			_ = updateHookHealth(root, id, cap, h.components[cap] && h.emitted, h.reason, nowFunc())
		}
	}
}

func noteAdviceComponent(cap harness.Capability, emitted bool) {
	if h := activeAdviceHealth; h != nil && h.cap != cap {
		if h.components == nil {
			h.components = map[harness.Capability]bool{}
		}
		h.components[cap] = h.components[cap] || emitted
	}
}

func noteAdviceSkip(meta map[string]string) {
	if h := activeAdviceHealth; h != nil && meta["emitted"] == "false" && meta["reason"] != "" {
		h.reason = meta["reason"]
	}
}

func adviceOn(root string, cap harness.Capability) bool {
	if assistiveChannelsDisabled() {
		return false
	}
	switch cap {
	case harness.CapReadout:
		return readoutsWiringOn()
	case harness.CapClaimCheck:
		return hooksKeyOn(root, hookOptInClaimCheck)
	case harness.CapReadOutlineHint:
		return hooksKeyOn(root, hookOptInReadOutlineHint)
	case harness.CapTestRunLine:
		return hooksKeyOn(root, hookOptInTestRun)
	case harness.CapObligationsPostEdit, harness.CapObligationsPostShell, harness.CapPromptObligations:
		return !capabilityOffAt(root, "obligations")
	case harness.CapGuardPreEdit, harness.CapGuardPreShell:
		return !capabilityOffAt(root, "guards")
	case harness.CapStopVerify:
		return !capabilityOffAt(root, "verify-gate")
	}
	return true
}

// Installation reads are capped and restricted to descriptor-owned paths.
// Parse/read uncertainty is ready (I9); absence is a known missing hook.
func readinessInstalled(root, id string, cap harness.Capability) readinessResult {
	ctx, cancel := testRunContext()
	defer cancel()
	d, ok := harness.Lookup(id)
	if !ok {
		return readinessResult{ready: true}
	}
	var bindings []hookSpec
	table := hookSpecsFor(id, hookOptInsAt(root)...)
	for _, b := range d.Hooks {
		if b.Capability != cap {
			continue
		}
		for _, spec := range table {
			direct := spec.event == b.Harness && spec.matcher == b.Matcher && spec.marker == b.Marker
			folded := cap == harness.CapTestRunLine && spec.event == b.Harness && spec.matcher == b.Matcher && spec.optIn == harness.OptInReadouts
			if direct || folded {
				bindings = append(bindings, spec)
				break
			}
		}
	}
	if len(bindings) == 0 {
		return readinessResult{reason: "unsupported-here"}
	}
	found := make([]bool, len(bindings))
	for _, file := range d.HookFiles {
		if hookDeadlineSpent() {
			return readinessResult{ready: true}
		}
		path := filepath.Join(root, file.Path)
		if strings.HasPrefix(file.Path, "~/") {
			var ok bool
			path, ok = harnessUserPath(d, file.Path)
			if !ok {
				return readinessResult{ready: true}
			}
		}
		// No injecting opt-in currently uses a globbed hook file. Keep an
		// unknown descriptor fail-open instead of enumerating directories.
		if strings.ContainsAny(path, "*?[") {
			return readinessResult{ready: true}
		}
		raw, err := readReadinessFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return readinessResult{ready: true}
		}
		var parsed map[string]any
		if json.Unmarshal(raw, &parsed) != nil {
			return readinessResult{ready: true}
		}
		// A plugin may supply this binding outside the settings file.
		// Do not diagnose an uninspected plugin as a missing hook.
		if plugins, _ := parsed["enabledPlugins"].(map[string]any); len(plugins) > 0 {
			return readinessResult{ready: true}
		}
		hooks, _ := parsed["hooks"].(map[string]any)
		for i, b := range bindings {
			evidence := doctorHookEvidence(ctx, root, hooks, b)
			if evidence.unknown {
				return readinessResult{ready: true}
			}
			found[i] = found[i] || evidence.found
		}
	}
	for _, present := range found {
		if !present {
			return readinessResult{reason: "hook-not-registered"}
		}
	}
	return readinessResult{ready: true}
}

func readReadinessFile(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > hookDedupeMaxBytes {
		return nil, os.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(f, hookDedupeMaxBytes+1))
	if len(raw) > hookDedupeMaxBytes {
		return nil, os.ErrInvalid
	}
	return raw, err
}

// Opt-in capabilities come from descriptor keys, the same keys used to build
// the installation table. Parent-hook advisory lines have no separate binding.
func hookOptInCapabilities() map[string][]harness.Capability {
	out := map[string][]harness.Capability{}
	for _, key := range hookOptInKeys() {
		for _, d := range harness.All() {
			for _, b := range d.Hooks {
				if b.OptIn != key {
					continue
				}
				found := false
				for _, cap := range out[key] {
					if cap == b.Capability {
						found = true
					}
				}
				if !found {
					out[key] = append(out[key], b.Capability)
				}
			}
		}
		sort.Slice(out[key], func(i, j int) bool { return out[key][i] < out[key][j] })
	}
	return out
}

func inactiveFeaturesLine(root, id string, now time.Time) string {
	if hookDeadlineSpent() {
		return ""
	}
	s, err := readHookHealth(root, id, now)
	if err != nil {
		s = hookHealthState{}
	}
	reasons := map[harness.Capability]string{}
	// A capability without a binding is unsupported here, not broken wiring.
	supported := map[harness.Capability]bool{}
	if d, ok := harness.Lookup(id); ok {
		for _, b := range d.Hooks {
			supported[b.Capability] = true
		}
	}
	optIns := hookOptInCapabilities()
	for _, key := range hookOptInKeys() {
		if !hooksKeyOn(root, key) {
			continue
		}
		for _, cap := range optIns[key] {
			if hookDeadlineSpent() {
				return ""
			}
			if !supported[cap] {
				continue
			}
			if r := readinessInstalled(root, id, cap); !r.ready && r.reason != "unsupported-here" {
				reasons[cap] = r.reason
			}
		}
	}
	for cap, stamp := range s.Channels {
		if !supported[cap] || !adviceOn(root, cap) || !strings.HasPrefix(stamp.LastState, "not-ready:") {
			continue
		}
		// Registration was rechecked above; an old stamp cannot undo recovery.
		if stamp.LastState == "not-ready:hook-not-registered" {
			continue
		}
		reasons[cap] = strings.TrimPrefix(stamp.LastState, "not-ready:")
	}
	var entries []string
	for cap := range reasons {
		if adviceOn(root, cap) {
			entries = append(entries, hookReadinessRegistry()[cap].name)
		}
	}
	if len(entries) == 0 {
		return ""
	}
	sort.Strings(entries)
	noun := "features"
	if len(entries) == 1 {
		noun = "feature"
	}
	shown := min(2, len(entries))
	text := fmt.Sprintf("%d %s inactive: %s", len(entries), noun, strings.Join(entries[:shown], ", "))
	if shown < len(entries) {
		text += fmt.Sprintf(" +%d more", len(entries)-shown)
	}
	return text
}
