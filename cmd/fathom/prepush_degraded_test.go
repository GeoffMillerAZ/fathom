package main

// prepush_degraded_test.go — EW-65 (O780): the pre-push verify step's scratch
// ingest exits 1 for a degraded ingest (the store is written and committed,
// with a disclosed gap: one package the local toolchain cannot type-check, a
// file it could not read), and the step read every non-zero exit as "ingest
// failed — allowing push": one such package skipped the whole plan gate, the
// same as a daemon outage. A degraded ingest now names its gap in one line
// and the step verifies on that store; only an operational failure (exit 2
// or another code, an ingest that aborted, or no store) fails open. The
// lefthook body runs under every gate shell (CS-8) against a stub binary
// whose ingest behaves as $FATHOM_STUB_INGEST says.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// degradedGap is the line a real degraded ingest prints (O780's evidence).
const degradedGap = "typed-edge pass failed: 1/531 package(s) failed typed loading (calibration/oracles/argo/go.mod: go.mod requires go >= 1.26.5 (running go 1.26.3; GOTOOLCHAIN=local))"

// stubIngestFathom is bin/fathom for the degraded-ingest cases: ingest
// writes --store's file unless the case has none, prints what a real ingest
// prints for the case, and exits with its code; verify exits
// $FATHOM_STUB_VERIFY_EXIT.
const stubIngestFathom = `#!/bin/sh
echo "$*" >> "$FATHOM_STUB_LOG"
if [ "$1" = ingest ]; then
  store=""
  while [ $# -gt 0 ]; do
    if [ "$1" = --store ]; then store=$2; fi
    shift
  done
  case "${FATHOM_STUB_INGEST:-ok}" in
    ok) : > "$store"; exit 0 ;;
    degraded) : > "$store"; echo "error: ingest degraded: ` + degradedGap + `" >&2; exit 1 ;;
    readfail) : > "$store"; echo "error: ingest degraded: 1 file(s) could not be read: docs/x.md (permission denied)" >&2; exit 1 ;;
    nostore) echo "error: ingest degraded: ` + degradedGap + `" >&2; exit 1 ;;
    aborted) : > "$store"; echo "error: ingest failed: fusion: discover \"fathom\": walk: interrupted" >&2; exit 1 ;;
    operational) echo "gate-lease: perl is required for the gate lease (flock)" >&2; exit 2 ;;
    other) exit 3 ;;
  esac
  exit 0
fi
exit "${FATHOM_STUB_VERIFY_EXIT:-0}"
`

func newDegradedGateSandbox(t *testing.T) gateSandbox {
	t.Helper()
	g := newGateSandbox(t, true)
	if err := os.WriteFile(filepath.Join(g.root, "bin", "fathom"), []byte(stubIngestFathom), 0o755); err != nil {
		t.Fatal(err)
	}
	return g
}

// runIngestCase runs the step with the stub's ingest set to kind.
func (g gateSandbox) runIngestCase(t *testing.T, shell, body, kind, verifyExit string) (int, string, string) {
	t.Helper()
	t.Setenv("FATHOM_STUB_INGEST", kind)
	return g.run(t, shell, body, verifyExit, "")
}

// TestLefthook_PrePushVerifyRunsOnADegradedIngest is EW-65: a degraded
// scratch ingest (exit 1, the store written) is named in one line and
// verified on; verify's own verdict decides the push.
func TestLefthook_PrePushVerifyRunsOnADegradedIngest(t *testing.T) {
	body := lefthookStepRun(t, "pre-push", "verify")
	eachGateShell(t, func(t *testing.T, shell string) {
		g := newDegradedGateSandbox(t)
		for _, kind := range []string{"degraded", "readfail"} {
			code, out, calls := g.runIngestCase(t, shell, body, kind, "0")
			if code != 0 {
				t.Fatalf("%s ingest, verify exit 0: want the step to pass, got %d\n%s", kind, code, out)
			}
			if strings.Contains(out, "fail-open") || strings.Contains(out, "allowing push") {
				t.Errorf("%s ingest: the step failed open instead of verifying:\n%s", kind, out)
			}
			vc := verifyCall(t, calls)
			if !strings.Contains(vc, "--store workspace/tmp/gate-verify.sqlite") {
				t.Errorf("%s ingest: verify call %q, want it on the degraded scratch store", kind, vc)
			}
			var gapLines []string
			for _, l := range strings.Split(out, "\n") {
				if strings.Contains(l, "degraded") {
					gapLines = append(gapLines, l)
				}
			}
			if len(gapLines) != 1 || !strings.HasPrefix(gapLines[0], "fathom verify: ") {
				t.Errorf("%s ingest: want one line naming the gap, got %q\n%s", kind, gapLines, out)
			}
		}
		code, out, _ := g.runIngestCase(t, shell, body, "degraded", "1")
		if code != 1 {
			t.Errorf("degraded ingest, verify exit 1: a refused row must block the push, got %d\n%s", code, out)
		}
		if !strings.Contains(out, "argo/go.mod") {
			t.Errorf("the gap line must name the gap the ingest disclosed, got:\n%s", out)
		}
	})
}

// TestLefthook_PrePushVerifyFailsOpenOnlyOnOperationalIngestFailure: an
// ingest that left no store, aborted, or exited 2 or another code is
// operational: the step allows the push with a named fail-open line and
// runs no verify.
func TestLefthook_PrePushVerifyFailsOpenOnlyOnOperationalIngestFailure(t *testing.T) {
	body := lefthookStepRun(t, "pre-push", "verify")
	eachGateShell(t, func(t *testing.T, shell string) {
		g := newDegradedGateSandbox(t)
		for _, kind := range []string{"nostore", "aborted", "operational", "other"} {
			code, out, calls := g.runIngestCase(t, shell, body, kind, "1")
			if code != 0 || !strings.Contains(out, "fail-open") {
				t.Errorf("%s ingest: want a named fail-open (exit 0), got %d\n%s", kind, code, out)
			}
			if strings.Contains(calls, "verify ") {
				t.Errorf("%s ingest: verify ran on no usable store: %q", kind, calls)
			}
		}
	})
}
