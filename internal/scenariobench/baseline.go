// baseline.go is O53 defect 4: the "plain" (and every other non-wired)
// baseline was not actually isolated from fathom — the real fathom binary
// was on PATH and the operator daemon at localhost:7575 was reachable, so
// one grep-arm trial in pilot vs-pilot-3 ran `fathom verify` against the
// live index, measuring an arm that was never supposed to have fathom
// available at all. Two mechanisms close it: constructing a session
// environment for every non-wired arm that cannot reach fathom even if
// invoked (a `fathom` SHIM on PATH, unroutableFathomURL — wired into
// sessionEnv, sessions.go), and a post-session scan that proves isolation
// actually held (BaselineRanFathomScan) rather than trusting it on faith.
//
// Fix-first review (wave-n1): the first version of the PATH half
// (baselinePathExcludingFathom) dropped every PATH directory that
// contained a file named "fathom" — blunt enough to also drop any OTHER
// tool sharing that directory (a dev machine's ~/bin holding both
// `fathom` and `claude`, say), breaking the very session it was meant to
// isolate. A shim replaces it: a per-cell directory holding nothing but
// an executable `fathom` that refuses and exits 127, prepended to PATH —
// every other directory (and every other tool in it) stays exactly where
// it was.
package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// unroutableFathomURL is what FATHOM_HTTP/FATHOM_HTTP_URL are pinned to
// for every non-wired arm: 127.0.0.1 port 1 refuses a connection
// immediately (nothing listens there, and port 1 is a privileged port no
// unprivileged process can bind) rather than timing out, so a session
// that somehow still tries to reach a fathom daemon fails fast and
// visibly instead of silently talking to the operator's real one at the
// default localhost:7575.
const unroutableFathomURL = "http://127.0.0.1:1"

// fathomShimRefusal is both the shim script's own stderr line and the
// text BaselineRanFathomScan-adjacent tooling can grep a session's
// transcript for to confirm the shim (not a real binary) is what
// answered.
const fathomShimRefusal = "fathom is not available to this arm (scenariobench baseline isolation)"

// fathomShimScript is the executable placed at <shimDir>/fathom for
// every non-wired arm's session: refuse loudly (stderr) and exit 127 —
// the shell's own "command not found" convention — so a bare `fathom`
// invocation fails fast and distinguishably, never silently succeeding
// against a real binary and never merely timing out against nothing.
const fathomShimScript = "#!/bin/sh\n" +
	"echo '" + fathomShimRefusal + "' >&2\n" +
	"exit 127\n"

// installFathomShim writes the fathom shim executable into shimDir
// (creating the directory if needed) and returns shimDir for the caller
// to prepend to PATH. Idempotent — re-installing (a retried session, a
// re-run of the same cell) just overwrites the same file with the same
// bytes.
func installFathomShim(shimDir string) (string, error) {
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return "", fmt.Errorf("baseline isolation: creating shim dir %s: %w", shimDir, err)
	}
	path := filepath.Join(shimDir, "fathom")
	if err := os.WriteFile(path, []byte(fathomShimScript), 0o755); err != nil {
		return "", fmt.Errorf("baseline isolation: writing fathom shim %s: %w", path, err)
	}
	return shimDir, nil
}

// baselinePathWithFathomShim returns currentPath with shimDir PREPENDED —
// so PATH resolution finds the shim's own `fathom` before any real one
// that might sit later in PATH — leaving every other directory (and every
// other tool in it) exactly where it was. "" currentPath is the edge
// case of a session with no inherited PATH at all: shimDir alone.
func baselinePathWithFathomShim(currentPath, shimDir string) string {
	if currentPath == "" {
		return shimDir
	}
	return shimDir + string(os.PathListSeparator) + currentPath
}

// fathomCommandRE matches "fathom" invoked as a COMMAND: at the shell
// position a new command starts (string start, or right after ;, &, |, a
// subshell "(", a command-substitution backtick, or a newline — optional
// whitespace follows), as a bare name or via a relative/absolute path
// whose final segment is "fathom" — never as a substring of another
// identifier ("fathomless") or a path COMPONENT of something else
// ("cmd/fathom/main.go", this repo's own module path
// "github.com/geoffmilleraz/fathom/…") — both of which a session working
// in this very repository mentions constantly in ordinary commands (go
// build, go vet, grep -r) and must never false-positive on.
var fathomCommandRE = regexp.MustCompile("(?:^|[;&|`(\n])\\s*(?:\\.{1,2}/|[\\w./-]*/)?fathom(?:\\s|$)")

// BaselineRanFathomScan scans session's raw transcript (session.RawPath)
// for a Bash tool_use whose "command" input invokes fathom
// (fathomCommandRE) — the backstop for the PATH shim: proof that
// isolation held (or did not), read from what the agent actually ran
// rather than trusted on faith. A shimmed invocation still matches here
// (the shim IS "fathom" on PATH, so the command text still names it) —
// this scan is about whether the agent TRIED to run fathom at all, not
// whether the attempt reached a real binary. false whenever there is
// nothing to scan (no raw transcript) or nothing was found.
func BaselineRanFathomScan(session SessionResult) bool {
	if session.RawPath == "" {
		return false
	}
	data, err := os.ReadFile(session.RawPath)
	if err != nil {
		return false
	}

	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var ev streamToolEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "assistant" {
			continue
		}
		for _, c := range ev.Message.Content {
			if c.Type != "tool_use" || c.Name != "Bash" || len(c.Input) == 0 {
				continue
			}
			var p struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal(c.Input, &p); err != nil {
				continue
			}
			if fathomCommandRE.MatchString(p.Command) {
				return true
			}
		}
	}
	return false
}
