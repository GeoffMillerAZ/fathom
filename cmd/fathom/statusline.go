package main

// statusline.go — `fathom statusline [--subagent] [--json]` (docs/design/
// claude-code-mods.md §7.3, ADR-072): one line for Claude Code's status
// line, or a segment of the person's own, saying whether the index matches
// the checkout the session works in.
//
// It reads the status-line JSON on stdin defensively (unknown fields
// ignored, malformed input still prints a state), makes one loopback GET of
// /statusz?format=json&root=<dir> under the hook guard (internal/hookguard:
// the kill switch, the shared per-URL breaker and a deadline), and prints
// exactly one of five states — never a blank line, never `0 behind` for an
// unknown (I38):
//
//	fathom ● @<sha>                 indexed at HEAD (+uncommitted / dirty unknown)
//	fathom ◐ <n> behind @<sha>      commits past the index (or behind unknown)
//	fathom ○ daemon down            the GET was sent and failed
//	fathom · not checked (<verdict>) the guard sent no GET: an open breaker is not a down daemon
//	fathom ? not indexed            no indexed service's root holds the directory
//
// The guard is built here rather than by beginHookGuard: a status line
// refreshes about once a second, and a hook-latency row or a beat per
// refresh would bury the hooks' own (doctor's hook budget reads those rows).
//
// A configured decision port adds its disclosure token after the state
// (JG-4, ADR-073 decision 6): J:off, J:on(<n>) or J:killed, plain text in no
// pass colour or glyph, and never part of the state's own line (--json
// carries it as decision_port), so it cannot enter the evidence band or the
// proof line. With no [decide] block in either config layer the output is
// unchanged.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/decide"
	"github.com/geoffmilleraz/fathom/internal/hookguard"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// statuslineTimeoutSec is the status line's budget in the hooks' terms:
// hookguard.HookDeadline keeps half of a one-second budget, 500 ms, for the
// GET and its answer (the daemon's git reads are bounded at 200 ms).
const statuslineTimeoutSec = 1

// statuslineStdinMax bounds the status-line JSON read from stdin.
const statuslineStdinMax = 64 << 10

// statuslineAnswerMax bounds the /statusz answer read.
const statuslineAnswerMax = 1 << 20

// The five states, as --json names them.
const (
	statuslineStIndexed    = "indexed"
	statuslineStBehind     = "behind"
	statuslineStDown       = "daemon_down"
	statuslineStNotChecked = "not_checked"
	statuslineStNotIndexed = "not_indexed"
)

// statuslineState is one run's answer; Line is the text the status line
// shows, and --json prints the whole value as one object.
type statuslineState struct {
	State   string `json:"state"`
	Line    string `json:"line"`
	Dir     string `json:"dir,omitempty"`
	Service string `json:"service,omitempty"`
	Commit  string `json:"indexed_commit,omitempty"`
	// Behind and Dirty are null when unknown or not read, never 0 or false.
	Behind  *int   `json:"behind"`
	Dirty   *bool  `json:"dirty"`
	Verdict string `json:"verdict,omitempty"`
	Reason  string `json:"reason,omitempty"`
	// DecisionPort is the port's token (J:off, J:on(<n>), J:killed), absent
	// with no [decide] block; KillSource names the gate's closing source
	// when killed. Neither is part of Line.
	DecisionPort string `json:"decision_port,omitempty"`
	KillSource   string `json:"kill_source,omitempty"`
}

// statuslineInput is the status-line JSON's fields this reads; every other
// field is ignored.
type statuslineInput struct {
	Cwd       string `json:"cwd"`
	Workspace struct {
		ProjectDir string `json:"project_dir"`
		CurrentDir string `json:"current_dir"`
	} `json:"workspace"`
}

// statuslineRunner runs `fathom statusline` (registered below).
type statuslineRunner struct{}

func (statuslineRunner) execVerb(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return cmdStatusline(args, stdin, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "statusline", Group: "hooks", Order: 95, Run: statuslineRunner{},
		Summary: "One status-line segment from the status-line JSON on stdin [--subagent] [--json]: indexed at HEAD, commits behind, daemon down, not checked (the hook guard sent no request) or not indexed"})
}

// cmdStatusline prints the status line's one state. It always exits 0 once
// its flags parse: a status line has no failure to report but its text.
func cmdStatusline(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("statusline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	urlFlag := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	_ = fs.Bool("subagent", false, "the input is Claude Code's subagent status-line payload (read the same way: its project directory, else its working directory)")
	asJSON := fs.Bool("json", false, "print the state as one JSON object instead of the line")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	start := time.Now()
	deadline := statuslineDeadline(start)
	dir := statuslineDir(stdin, deadline.At())
	st := statuslineCheck(*urlFlag, dir, deadline)
	st.DecisionPort, st.KillSource = statuslineJudgment(dir)
	if *asJSON {
		raw, err := json.Marshal(st)
		if err != nil {
			fmt.Fprintln(stdout, st.text())
			return 0
		}
		fmt.Fprintln(stdout, string(raw))
		return 0
	}
	fmt.Fprintln(stdout, st.text())
	return 0
}

// text is what the status line shows: the state's line, then the decision
// port's token when one is configured.
func (st statuslineState) text() string {
	if st.DecisionPort == "" {
		return st.Line
	}
	return st.Line + " " + st.DecisionPort
}

// statuslineJudgment is the decision port's token for dir (JG-4): "" when
// neither the user layer nor dir's .fathom/config.toml carries a [decide]
// block (the two files are read, capped, and loaded only when one names
// decide), J:off for provider none or no open site, J:killed with the
// closing source when KS-6's gate (decide.Check) closes the port, else
// J:on(<n>) with n the sites it may answer. It reads no key, execs nothing
// and dials nothing; a config that does not load adds nothing.
func statuslineJudgment(dir string) (token, killSource string) {
	userPath := config.DefaultUserPath()
	projPath := ""
	if dir != "" {
		projPath = projectdir.Config.Path(dir)
	}
	word := []byte("decide")
	if !bytes.Contains(readConfigLayer(userPath), word) && !bytes.Contains(readConfigLayer(projPath), word) {
		return "", ""
	}
	l, err := config.Load(userPath, projPath)
	if err != nil || !statuslineHasDecide(l.Provenance) {
		return "", ""
	}
	dc := l.Config.Decide
	if dc.Provider == "" || dc.Provider == "none" {
		return "J:off", ""
	}
	if g := decide.Check(dc, ""); g.Closed {
		return "J:killed", g.Source
	}
	n := 0
	for _, site := range decide.AllSites {
		if decide.SiteAllowed(dc, site) {
			n++
		}
	}
	if n == 0 {
		return "J:off", ""
	}
	return "J:on(" + strconv.Itoa(n) + ")", ""
}

// statuslineHasDecide reports whether a loaded layer set any [decide] key.
func statuslineHasDecide(prov config.Provenance) bool {
	for k := range prov {
		if strings.HasPrefix(k, "decide.") {
			return true
		}
	}
	return false
}

// statuslineDeadline is one run's deadline (hookBudgetScale shrinks it in
// tests, as it does every hook's).
func statuslineDeadline(start time.Time) hookguard.Deadline {
	return hookguard.HookDeadline(start, statuslineTimeoutSec).Scale(hookBudgetScale)
}

// statuslineDir is the directory the status line speaks for: the payload's
// project directory when present, else its working directory, else this
// process's. A relative path is skipped; a terminal on stdin is not read
// (a person running the verb by hand), and a pipe is read only until the
// deadline.
func statuslineDir(stdin io.Reader, until time.Time) string {
	var in statuslineInput
	if stdin != nil && !statuslineIsTerminal(stdin) {
		if f, ok := stdin.(*os.File); ok {
			_ = f.SetReadDeadline(until)
		}
		raw, _ := io.ReadAll(io.LimitReader(stdin, statuslineStdinMax))
		// A type mismatch leaves the other fields decoded; a syntax error
		// leaves none. Either way the fallbacks below still name a dir.
		_ = json.Unmarshal(raw, &in)
	}
	for _, d := range []string{in.Workspace.ProjectDir, in.Workspace.CurrentDir, in.Cwd} {
		if d != "" && filepath.IsAbs(d) {
			return filepath.Clean(d)
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// statuslineIsTerminal reports stdin attached to a terminal.
func statuslineIsTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// statuslineAnswer is the part of the root answer the status line reads.
type statuslineAnswer struct {
	Root     *StatuszRoot `json:"root"`
	Services []struct {
		Service       string `json:"service"`
		IndexedCommit string `json:"indexed_commit"`
		*StatuszHead
	} `json:"services"`
}

// statuslineCheck asks the daemon at base about dir and names the state.
func statuslineCheck(base, dir string, deadline hookguard.Deadline) statuslineState {
	if dir == "" {
		return statuslineState{State: statuslineStNotIndexed, Line: "fathom ? not indexed",
			Reason: "no directory: the input named none and the working directory could not be read"}
	}
	g := &hookguard.Guard{Deadline: deadline}
	switch d := hookguard.BreakerAllows(g, base); d.Verdict {
	case hookguard.Open, hookguard.Busy, hookguard.Disabled, hookguard.Paused:
		return statuslineSkipped(dir, d.Verdict, d.Reason)
	}
	restore := activateHookGuard(g)
	defer restore()
	ctx, cancel := deadline.Context(context.Background())
	defer cancel()
	target := base + "/statusz?format=json&root=" + url.QueryEscape(dir)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return statuslineDown(dir, "the request could not be built: "+err.Error())
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		var se *hookguard.SkipError
		if errors.As(err, &se) {
			return statuslineSkipped(dir, se.Verdict, se.Reason)
		}
		return statuslineDown(dir, err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return statuslineDown(dir, "HTTP "+strconv.Itoa(resp.StatusCode))
	}
	var ans statuslineAnswer
	if err := json.NewDecoder(io.LimitReader(resp.Body, statuslineAnswerMax)).Decode(&ans); err != nil {
		return statuslineDown(dir, "the answer could not be read: "+err.Error())
	}
	return statuslineFromAnswer(dir, ans)
}

// statuslineFromAnswer names the state a root answer carries. An answer
// without the root block (a daemon that predates the query) or one that
// could not tell whether the directory is indexed is a failed GET, not a
// guess.
func statuslineFromAnswer(dir string, ans statuslineAnswer) statuslineState {
	switch {
	case ans.Root == nil:
		return statuslineDown(dir, "the daemon's /statusz does not answer the root query")
	case ans.Root.Indexed == nil:
		return statuslineDown(dir, "the daemon could not tell whether the directory is indexed: "+ans.Root.Reason)
	case !*ans.Root.Indexed:
		return statuslineState{State: statuslineStNotIndexed, Line: "fathom ? not indexed", Dir: dir}
	}
	for _, row := range ans.Services {
		if row.Service != ans.Root.Service || row.StatuszHead == nil {
			continue
		}
		st := statuslineState{Dir: dir, Service: row.Service, Commit: row.IndexedCommit, Behind: row.Behind, Dirty: row.Dirty}
		at := ""
		if sha := statuslineSHA(row.IndexedCommit); sha != "" {
			at = " @" + sha
		}
		switch {
		case row.Behind == nil || *row.Behind < 0:
			st.Behind = nil
			st.State, st.Line, st.Reason = statuslineStBehind, "fathom ◐ behind unknown"+at, row.BehindReason
		case *row.Behind > 0:
			st.State, st.Line = statuslineStBehind, "fathom ◐ "+strconv.Itoa(*row.Behind)+" behind"+at
		default:
			st.State, st.Line = statuslineStIndexed, "fathom ●"+at
			switch {
			case row.Dirty == nil:
				st.Line += " dirty unknown"
				st.Reason = row.DirtyReason
			case *row.Dirty:
				st.Line += " +uncommitted"
			}
		}
		return st
	}
	return statuslineDown(dir, "the answer names service "+ans.Root.Service+" but carries no row for it")
}

func statuslineDown(dir, reason string) statuslineState {
	return statuslineState{State: statuslineStDown, Line: "fathom ○ daemon down", Dir: dir, Reason: reason}
}

func statuslineSkipped(dir string, v hookguard.Verdict, reason string) statuslineState {
	return statuslineState{State: statuslineStNotChecked, Line: "fathom · not checked (" + string(v) + ")",
		Dir: dir, Verdict: string(v), Reason: reason}
}

// statuslineSHA is the short commit the status line shows.
func statuslineSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
