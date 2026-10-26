package main

// prepush_stalebin_test.go — RT-161 on a Linux cloud VM (#71): the pre-push
// steps built bin/fathom only when it was missing, so a binary from an older
// commit ran the gate: a cloud snapshot's, or one left from before a merge
// that brought Go changes in without a commit of its own touching Go (the
// post-commit rebuild keys on the commit's own diff). That verify predated
// the platform check and refused RT-161, a row the pushed tree's verify
// reads as operational. A present bin/fathom whose `fathom version` stamp
// names another commit is now rebuilt first; one stamped with HEAD, or
// carrying no stamp, runs as before.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staleBinStub is a present bin/fathom stamped with $STUB_REVISION (no stamp
// when it is empty) that logs every other call as "old <args>".
const staleBinStub = `#!/bin/sh
if [ "$1" = version ]; then
  [ -n "${STUB_REVISION:-}" ] && echo "fathom v0.0.0 @ $STUB_REVISION [2026-10-25T00:00:00Z]"
  exit 0
fi
echo "old $*" >> "$FATHOM_STUB_LOG"
[ "$1" = ingest ] && exit 0
exit "${FATHOM_STUB_VERIFY_EXIT:-0}"
`

// stubBuildWrapper stands in for scripts/fathom-go.sh: it logs the build and
// writes a fresh bin/fathom that logs its calls as "new <args>".
const stubBuildWrapper = `#!/usr/bin/env bash
echo "build $*" >> "$FATHOM_STUB_LOG"
cat > bin/fathom <<'EOF'
#!/bin/sh
[ "$1" = version ] && exit 0
echo "new $*" >> "$FATHOM_STUB_LOG"
[ "$1" = ingest ] && exit 0
exit "${FATHOM_STUB_VERIFY_EXIT:-0}"
EOF
chmod +x bin/fathom
`

func newStaleBinSandbox(t *testing.T) gateSandbox {
	t.Helper()
	g := newGateSandbox(t, true)
	for rel, body := range map[string]string{"bin/fathom": staleBinStub, "scripts/fathom-go.sh": stubBuildWrapper} {
		if err := os.WriteFile(filepath.Join(g.root, rel), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func TestLefthook_PrePushRebuildsABinaryBuiltFromAnotherCommit(t *testing.T) {
	steps := []struct{ step, verb string }{
		{"verify", "verify"},
		{"review", "review"},
		{"refactor-conserve", "conserve"},
	}
	for _, s := range steps {
		body := lefthookStepRun(t, "pre-push", s.step)
		t.Run(s.step, func(t *testing.T) {
			eachGateShell(t, func(t *testing.T, shell string) {
				for _, c := range []struct {
					name, revision string
					rebuild        bool
				}{
					{"stamped with another commit", "0123456789ab", true},
					{"stamped with HEAD", "", false}, // the sandbox HEAD, read below
					{"no stamp", "", false},
				} {
					g := newStaleBinSandbox(t)
					rev := c.revision
					if c.name == "stamped with HEAD" {
						rev = gitRevParse(t, g.root, "HEAD")[:12]
					}
					t.Setenv("STUB_REVISION", rev)
					code, out, calls := g.run(t, shell, body, "0", "")
					if code != 0 {
						t.Fatalf("%s: want the step to pass, got %d\n%s\ncalls:\n%s", c.name, code, out, calls)
					}
					built := strings.Contains(calls, "build -o bin/fathom ./cmd/fathom")
					ranOld := strings.Contains(calls, "old "+s.verb+" ")
					ranNew := strings.Contains(calls, "new "+s.verb+" ")
					if c.rebuild {
						if !built || ranOld || !ranNew {
							t.Errorf("%s: want a rebuild and the step run by the new binary (built %v, old ran %v, new ran %v)\n%s\ncalls:\n%s", c.name, built, ranOld, ranNew, out, calls)
						}
						if !strings.Contains(out, "built from 0123456789ab, not HEAD") {
							t.Errorf("%s: the rebuild must be named, got:\n%s", c.name, out)
						}
						if strings.Contains(calls, "old ingest") {
							t.Errorf("%s: the stale binary ran the scratch ingest:\n%s", c.name, calls)
						}
					} else if built || !ranOld {
						t.Errorf("%s: want the present binary run as is (built %v, old ran %v)\n%s\ncalls:\n%s", c.name, built, ranOld, out, calls)
					}
				}
			})
		})
	}
}
