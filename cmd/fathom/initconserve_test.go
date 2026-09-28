package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestInitWiresConservePrePush proves SR-63 (docs/design/surface-reckoning.md
// §6): fathom init's printed pre-push guidance carries a non-blocking
// `fathom conserve` step by default, not only the verify gate — the same
// advisory step this repo's own lefthook.yml already runs, so every
// adopter following init's manual steps gets conservation reporting wired
// in from the start instead of only this repo having it.
func TestInitWiresConservePrePush(t *testing.T) {
	var out bytes.Buffer
	initPrePushConserve(&out, "fathom")
	stdout := out.String()

	if !strings.Contains(stdout, "fathom conserve") {
		t.Errorf("conserve manual step must name the conserve verb:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--since") {
		t.Errorf("conserve must run on the pushed range (--since), not the whole tree:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--service fathom") {
		t.Errorf("conserve manual step must name the adopter's service:\n%s", stdout)
	}

	// printGateAndConfigManualSteps is the shared harness-neutral printer
	// every init path calls; the conserve step must land inside the same
	// pre-push block the verify gate prints, so no harness (Claude Code,
	// Codex, or a future one) gets the verify gate without conserve
	// alongside it.
	var full bytes.Buffer
	printGateAndConfigManualSteps(&full, "fathom")
	fullBody := full.String()
	verifyAt := strings.Index(fullBody, "fathom-verify:")
	conserveAt := strings.Index(fullBody, "fathom-conserve:")
	prePushAt := strings.Index(fullBody, "pre-push")
	postCommitAt := strings.Index(fullBody, "post-commit")
	if verifyAt < 0 || conserveAt < 0 || prePushAt < 0 || postCommitAt < 0 {
		t.Fatalf("printGateAndConfigManualSteps must print pre-push, fathom-verify, fathom-conserve and post-commit:\n%s", fullBody)
	}
	if !(prePushAt < verifyAt && verifyAt < conserveAt && conserveAt < postCommitAt) {
		t.Errorf("fathom-conserve must sit inside the same pre-push block as fathom-verify, before post-commit:\n%s", fullBody)
	}
}
