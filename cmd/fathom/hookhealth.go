package main

// Bounded hook-owned delivery stamps share CB-1's directory, lock, byte
// limit and seven-day GC. No daemon/store state or background work.

import (
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

type hookHealthEvent struct {
	At      time.Time `json:"at"`
	Emitted bool      `json:"emitted,omitempty"`
}

type hookHealthStamp struct {
	LastEmit   time.Time         `json:"last_emit,omitempty"`
	LastSkip   string            `json:"last_skip,omitempty"`
	At         time.Time         `json:"at"`
	Inactive   string            `json:"inactive,omitempty"`
	Uncompiled int               `json:"uncompiled,omitempty"`
	Emitted    int               `json:"emitted"`
	Skipped    int               `json:"skipped"`
	Events     []hookHealthEvent `json:"events"`
}

type hookHealthState struct {
	Version  int                                    `json:"version"`
	Channels map[harness.Capability]hookHealthStamp `json:"channels"`
}

func hookHealthPath(root, id string) string {
	return fathomhome.Path("hook-dedupe", "health-"+hookDigest(id+"\x00"+filepath.Clean(root))+".json")
}

func loadHookHealth(f *os.File, now time.Time) (hookHealthState, error) {
	s := hookHealthState{Version: 1, Channels: map[harness.Capability]hookHealthStamp{}}
	raw, err := io.ReadAll(io.LimitReader(f, hookDedupeMaxBytes+1))
	if err != nil {
		return s, err
	}
	if len(raw) == 0 {
		return s, nil
	}
	if len(raw) > hookDedupeMaxBytes || json.Unmarshal(raw, &s) != nil || s.Version != 1 || s.Channels == nil || len(s.Channels) > len(hookReadinessRegistry()) {
		return s, os.ErrInvalid
	}
	total := 0
	for cap, stamp := range s.Channels {
		if _, ok := hookReadinessRegistry()[cap]; !ok || stamp.At.IsZero() || stamp.Uncompiled < 0 || len(stamp.LastSkip) > 128 || len(stamp.Inactive) > 128 {
			return s, os.ErrInvalid
		}
		if stamp.At.After(now) || now.Sub(stamp.At) > hookDedupeAge {
			delete(s.Channels, cap)
			continue
		}
		kept := make([]hookHealthEvent, 0, len(stamp.Events))
		stamp.Emitted, stamp.Skipped = 0, 0
		for _, ev := range stamp.Events {
			if ev.At.IsZero() {
				return s, os.ErrInvalid
			}
			if ev.At.After(now) || now.Sub(ev.At) > hookDedupeAge {
				continue
			}
			kept = append(kept, ev)
			if ev.Emitted {
				stamp.Emitted++
			} else {
				stamp.Skipped++
			}
		}
		stamp.Events = kept
		total += len(kept)
		s.Channels[cap] = stamp
	}
	if total > hookDedupeMaxEntries {
		return s, os.ErrInvalid
	}
	return s, nil
}

func readHookHealth(root, id string, now time.Time) (hookHealthState, error) {
	f, _, err := openHookState(hookHealthPath(root, id))
	if err != nil {
		return hookHealthState{}, err
	}
	defer f.Close()
	return loadHookHealth(f, now)
}

func updateHookHealth(root, id string, cap harness.Capability, emitted bool, reason string, uncompiled int, now time.Time) error {
	f, _, err := openHookState(hookHealthPath(root, id))
	if err != nil {
		return err
	}
	defer f.Close()
	s, err := loadHookHealth(f, now)
	if err != nil {
		return err
	}
	n := 0
	for _, stamp := range s.Channels {
		n += len(stamp.Events)
	}
	if n >= hookDedupeMaxEntries {
		return os.ErrInvalid
	}
	stamp := s.Channels[cap]
	stamp.At, stamp.Uncompiled = now, uncompiled
	stamp.Inactive = ""
	if emitted {
		stamp.LastEmit = now
		stamp.Emitted++
	} else {
		stamp.LastSkip = reason
		stamp.Skipped++
		if strings.HasPrefix(reason, "not-ready:") || reason == "breaker_open" {
			stamp.Inactive = reason
		}
	}
	stamp.Events = append(stamp.Events, hookHealthEvent{At: now, Emitted: emitted})
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
	return f.Sync()
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
	uncompiled        int
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
		if g := activeHookGuard(); g != nil {
			skips := g.Skips()
			keys := make([]string, 0, len(skips))
			for key := range skips {
				keys = append(keys, string(key))
			}
			sort.Strings(keys)
			for _, key := range keys {
				if key == "breaker_open" || key == "deadline_spent" {
					h.reason = key
					break
				}
			}
		}
		if !h.recorded {
			_ = updateHookHealth(root, id, cap, h.emitted, h.reason, h.uncompiled, nowFunc())
		}
		caps := make([]harness.Capability, 0, len(h.components))
		for cap := range h.components {
			caps = append(caps, cap)
		}
		sort.Slice(caps, func(i, j int) bool { return caps[i] < caps[j] })
		for _, cap := range caps {
			_ = updateHookHealth(root, id, cap, h.components[cap] && h.emitted, h.reason, 0, nowFunc())
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
	var bindings []harness.EventBinding
	for _, b := range d.Hooks {
		if b.Capability == cap {
			bindings = append(bindings, b)
		}
	}
	if len(bindings) == 0 {
		return readinessResult{reason: "hook-not-registered"}
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
			evidence := doctorHookEvidence(ctx, root, hooks, hookSpec{event: b.Harness, matcher: b.Matcher, marker: b.Marker, sub: b.Verb})
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

func inactiveFeaturesLine(root, id string, now time.Time) string {
	if hookDeadlineSpent() {
		return ""
	}
	s, err := readHookHealth(root, id, now)
	if err != nil {
		return ""
	}
	reasons := map[harness.Capability]string{}
	names := map[harness.Capability]string{}
	ctx := readinessInput{root: root, harnessID: id, now: now}
	// These local prerequisites can be checked even when the missing hook
	// has never fired. All other channels report from their latest stamp.
	if d, ok := harness.Lookup(id); ok && d.Vehicles[harness.CapSubagentPrime] == harness.VehicleHookInject {
		if r := hookReadinessRegistry()[harness.CapSubagentPrime].check(ctx); !r.ready {
			reasons[harness.CapSubagentPrime] = r.reason
		}
	}
	for _, cap := range []harness.Capability{harness.CapReadout, harness.CapClaimCheck, harness.CapReadOutlineHint, harness.CapTestRunLine} {
		if hookDeadlineSpent() {
			return ""
		}
		if !adviceOn(root, cap) {
			continue
		}
		if r := readinessInstalled(root, id, cap); !r.ready {
			reasons[cap] = r.reason
		}
	}
	// These opt-in lines ride a parent binding instead of owning a
	// descriptor row. Check that row, without inventing a new capability.
	seams := []struct {
		cap       harness.Capability
		key, name string
	}{
		{harness.CapGuardPreShell, hookOptInGatePreflight, "gate preflight"},
		{harness.CapPromptObligations, hookOptInPromptDefinitions, "prompt definitions"},
		{harness.CapObligationsPostEdit, "hooks.tests_to_run", "tests to run"},
	}
	for _, seam := range seams {
		if hookDeadlineSpent() {
			return ""
		}
		cfg, ok := hooksConfigAt(root, seam.key)
		on := ok && (seam.key == hookOptInGatePreflight && cfg.Hooks.GatePreflight || seam.key == hookOptInPromptDefinitions && cfg.Hooks.PromptDefinitions || seam.key == "hooks.tests_to_run" && cfg.Hooks.TestsToRun)
		if on {
			if r := readinessInstalled(root, id, seam.cap); !r.ready {
				reasons[seam.cap], names[seam.cap] = r.reason, seam.name
			}
		}
	}
	for cap, stamp := range s.Channels {
		if stamp.Inactive != "" && adviceOn(root, cap) {
			// A prerequisite rechecked above beats an older skip stamp.
			if stamp.Inactive == "not-ready:no-maps" || stamp.Inactive == "not-ready:hook-not-registered" {
				continue
			}
			if _, fresh := reasons[cap]; !fresh {
				reasons[cap] = strings.TrimPrefix(stamp.Inactive, "not-ready:")
			}
		}
	}
	var entries []string
	for cap, reason := range reasons {
		if !adviceOn(root, cap) {
			continue
		}
		label := strings.ReplaceAll(reason, "-", " ")
		switch reason {
		case "no-maps":
			label = "no maps filed in 7 days"
		case "hook-not-registered":
			label = "hook not registered"
		case "breaker_open":
			label = "daemon breaker open"
		}
		name := hookReadinessRegistry()[cap].name
		if names[cap] != "" {
			name = names[cap]
		}
		entries = append(entries, name+" ("+label+")")
	}
	if len(entries) == 0 {
		return ""
	}
	sort.Strings(entries)
	return fmt.Sprintf("fathom: %d features inactive: %s. Run fathom doctor.", len(entries), strings.Join(entries, ", "))
}
