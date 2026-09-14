// baseline.go is O53 defect 4: the "plain" (and every other non-wired)
// baseline was not actually isolated from fathom — the real fathom binary
// was on PATH and the operator daemon at localhost:7575 was reachable, so
// one grep-arm trial in pilot vs-pilot-3 ran `fathom verify` against the
// live index, measuring an arm that was never supposed to have fathom
// available at all. Two mechanisms close it: constructing a session
// environment for every non-wired arm that cannot reach fathom even if
// invoked (baselinePathExcludingFathom, unroutableFathomURL — wired into
// sessionEnv, sessions.go), and a post-session scan that proves isolation
// actually held (BaselineRanFathomScan) rather than trusting it on faith.
package scenariobench

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// unroutableFathomURL is what FATHOM_HTTP/FATHOM_HTTP_URL are pinned to
// for every non-wired arm: 127.0.0.1 port 1 refuses a connection
// immediately (nothing listens there, and port 1 is a privileged port no
// unprivileged process can bind) rather than timing out, so a session
// that somehow still tries to reach a fathom daemon fails fast and
// visibly instead of silently talking to the operator's real one at the
// default localhost:7575.
const unroutableFathomURL = "http://127.0.0.1:1"

// baselinePathExcludingFathom returns currentPath (a PATH-shaped,
// os.PathListSeparator-joined value) with every directory removed that
// either equals cellBinDir (a wired cell's own binary lives there;
// excluded defensively for every arm, even though wireScratchDaemon execs
// it by absolute path, never through PATH) or itself contains a file
// literally named "fathom" — so a non-wired arm's session can never
// resolve a bare `fathom` invocation to a real binary, however it reached
// PATH (the operator's shell, a Go bin directory, a previous cell's
// build).
func baselinePathExcludingFathom(currentPath, cellBinDir string) string {
	var kept []string
	for _, dir := range filepath.SplitList(currentPath) {
		if dir == "" {
			continue
		}
		if cellBinDir != "" && dir == cellBinDir {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, "fathom")); err == nil {
			continue
		}
		kept = append(kept, dir)
	}
	return strings.Join(kept, string(os.PathListSeparator))
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
// (fathomCommandRE) — the backstop for baselinePathExcludingFathom: proof
// that isolation held (or did not), read from what the agent actually
// ran rather than trusted on faith. false whenever there is nothing to
// scan (no raw transcript) or nothing was found.
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
