package main

// claimcheck_hooktable.go — RO-38 (docs/plans/readouts.md, design
// docs/design/readouts.md section 20, H2): the SubagentStop row is written
// only while `[hooks] claim_check` is on. The harness registry marks the
// binding opt-in (harness.EventBinding.OptIn); this file reads which opt-in
// keys a root's config switches on, so init, plugin render, reconcile and
// doctor carry the row from the one hook table (hookSpecsFor) exactly when
// the check would run.

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/harness"
)

// The opt-in [hooks] keys a hook binding or an advisory line names.
const (
	hookOptInClaimCheck    = "hooks.claim_check"
	hookOptInGatePreflight = "hooks.gate_preflight"
	hookOptInTestRun       = "hooks.test_run"
)

// hooksConfigAt loads the merged user layer and root's .fathom/config.toml
// when either names the [hooks] key; ok is false when neither does or the
// load fails. Cheapest first, as the test-run hook's gate is: a layer that
// never names the key cannot switch it on, so most calls pay two capped
// file reads and no config load, and a repository's file opens it only for
// a key a repository may set (SEC-2). root "" reads the user layer alone.
func hooksConfigAt(root, key string) (config.Config, bool) {
	word := []byte(strings.TrimPrefix(key, "hooks."))
	userPath := config.DefaultUserPath()
	projPath := ""
	if root != "" {
		projPath = filepath.Join(root, ".fathom", "config.toml")
	}
	if !bytes.Contains(readConfigLayer(userPath), word) &&
		!(config.RepoMaySet(key) && bytes.Contains(readConfigLayer(projPath), word)) {
		return config.Config{}, false
	}
	l, err := config.Load(userPath, projPath)
	if err != nil {
		return config.Config{}, false
	}
	return l.Config, true
}

// hooksKeyOn reports whether the [hooks] boolean key is on for root. Any
// load error reads off.
func hooksKeyOn(root, key string) bool {
	if key == harness.OptInReadouts {
		return readoutsWiringOn() // user scope only, never the repository (ADR-071)
	}
	cfg, ok := hooksConfigAt(root, key)
	if !ok {
		return false
	}
	switch key {
	case hookOptInClaimCheck:
		return cfg.Hooks.ClaimCheck
	case hookOptInGatePreflight:
		return cfg.Hooks.GatePreflight
	case hookOptInTestRun:
		return cfg.Hooks.TestRun
	}
	return false
}

// hookOptInKeys lists every opt-in key a Claude Code binding names, sorted.
func hookOptInKeys() []string {
	d, _ := harness.Lookup(harness.ClaudeCode)
	var keys []string
	for _, b := range d.Hooks {
		if b.OptIn != "" && !slices.Contains(keys, b.OptIn) {
			keys = append(keys, b.OptIn)
		}
	}
	slices.Sort(keys)
	return keys
}

// hookOptInsAt returns the opt-in keys root's config switches on, sorted,
// and, while the readout hook's key is among them, [hooks] test_run when it
// is on too: the key hookSpecsFor folds the test-run rows away by (RO-21).
func hookOptInsAt(root string) []string {
	var on []string
	for _, k := range hookOptInKeys() {
		if hooksKeyOn(root, k) {
			on = append(on, k)
		}
	}
	if slices.Contains(on, harness.OptInReadouts) && !slices.Contains(on, hookOptInTestRun) && hooksKeyOn(root, hookOptInTestRun) {
		on = append(on, hookOptInTestRun)
		slices.Sort(on)
	}
	return on
}

// fathomHookSpecsAt is the Claude Code hook table for root: fathomHookSpecs
// plus every opt-in row root's config switches on (RO-38's SubagentStop
// row while [hooks] claim_check is on). init writes it, reconcile adds what
// it lacks, and doctor's hooks_installed expects it.
func fathomHookSpecsAt(root string) []hookSpec {
	return hookSpecsFor(harness.ClaudeCode, hookOptInsAt(root)...)
}

// fathomHookSpecsAll is the table with every opt-in row, for detection
// only: an entry already wired at user scope is recognised whatever this
// root's config says, so init never duplicates it.
func fathomHookSpecsAll() []hookSpec {
	return hookSpecsFor(harness.ClaudeCode, hookOptInKeys()...)
}
