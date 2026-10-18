package main

// The readout hook (ADR-071, docs/design/readouts.md §3-§5 and §10):
// `fathom readout-hook` on PostToolUse(Bash) and PostToolUseFailure(Bash).
// It lexes the command, runs the readouts engine over its output and writes
// one row to the hook log on channel `readout`; this build observes only,
// so every mode emits nothing and the agent sees the raw output. Off by
// default ([readouts] mode, user scope only); any error exits 0 with
// nothing emitted (I9).

import (
	"bytes"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// readoutChannel is the hook log channel every readout row is written to
// (no table: the rows ride the existing hook log).
const readoutChannel = "readout"

// readoutStdinCap bounds the envelope read, as the test-run hook does.
const readoutStdinCap = 1 << 20

// readoutHookCeiling is the hook's internal ceiling (design §5): a run that
// spent longer says `ceiling` in its row and emits nothing.
const readoutHookCeiling = time.Second

// readoutHookClock is the hook's clock; a test pins it.
var readoutHookClock = time.Now

// readoutHookTrace, when set, is told each step the hook reaches
// ("envelope", "config", "rules", "render", "row"); tests read it, and nil
// costs nothing.
var readoutHookTrace func(step string)

func readoutStep(step string) {
	if readoutHookTrace != nil {
		readoutHookTrace(step)
	}
}

// readoutHookEvents maps --event to the harness event it answers.
func readoutHookEvent(name string) (harness.Event, bool) {
	switch name {
	case "post-tool":
		return harness.EventPostTool, true
	case "tool-failure":
		return harness.EventToolFailure, true
	}
	return "", false
}

// cmdReadoutHook is the hook body for one harness event: decode, gate,
// lex, render, log. It always returns 0 and writes nothing to stdout.
func cmdReadoutHook(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	fs := newFlagSet("readout-hook", stderr)
	eventFlag := fs.String("event", "post-tool", "the harness event this run answers: post-tool or tool-failure")
	harnessFlag := addHarnessFlag(fs)
	if err := fs.Parse(args); err != nil {
		return 0
	}
	defer func() {
		if recover() != nil {
			code = 0 // fail open: a panic leaves the raw output standing
		}
	}()
	if fathomChannelsDisabled() || stdin == nil {
		return 0
	}
	ev, ok := readoutHookEvent(*eventFlag)
	if !ok {
		return 0
	}
	codec, ok := resolveHookHarness(*harnessFlag, stderr)
	if !ok {
		return 0
	}
	start := readoutHookClock()
	raw, err := io.ReadAll(io.LimitReader(stdin, readoutStdinCap))
	if err != nil {
		return 0
	}
	in, err := codec.DecodeInput(ev, raw)
	if err != nil || in.Tool != harness.ToolShell || strings.TrimSpace(in.Command) == "" {
		return 0
	}
	readoutStep("envelope")
	cfg, ok := readoutsUserConfig()
	readoutStep("config")
	if !ok || cfg.Mode == config.ReadoutsOff || !underReadoutRoots(cfg.Roots, readoutHookDir(codec, in)) {
		return 0
	}
	rs, err := readout.LoadRules(fathomhome.FathomHome())
	if err != nil {
		return 0
	}
	readoutStep("rules")
	rs = readoutRulesFor(rs, cfg.Rules)
	cmd := shellcmd.ParseCommand(in.Command)
	readoutStep("render")
	// lean and rich build their emission later; until then every mode is
	// observe, and the row names the configured one beside it.
	ro := readout.Render(rs, readout.Input{
		Command:     &cmd,
		Stdout:      in.Shell.Stdout,
		Stderr:      in.Shell.Stderr,
		Interrupted: in.Shell.Interrupted,
		Persisted:   in.Shell.Persisted,
		Mode:        readout.ModeObserve,
	})
	readoutStep("row")
	logReadoutRow(readoutRow{
		event: *eventFlag, configured: cfg.Mode, in: in, cmd: &cmd, ro: ro,
		rule: matchedRule(rs, ro.Rule), elapsed: readoutHookClock().Sub(start),
	})
	return 0
}

// readoutsUserConfig reads the user's [readouts] table, the only layer it
// lives in (every key is user-only). A file that never names readouts is
// mode off without a parse; ok is false when the file does not load.
func readoutsUserConfig() (config.Readouts, bool) {
	path := config.DefaultUserPath()
	if !bytes.Contains(readConfigLayer(path), []byte("readouts")) {
		return config.Readouts{Mode: config.ReadoutsOff}, true
	}
	l, err := config.Load(path, "")
	if err != nil {
		return config.Readouts{}, false
	}
	return l.Config.Readouts, true
}

// readoutsWiringOn reports whether the readout-hook rows are written and
// expected: the user's [readouts] mode is set and not off.
func readoutsWiringOn() bool {
	cfg, ok := readoutsUserConfig()
	return ok && cfg.Mode != "" && cfg.Mode != config.ReadoutsOff
}

// readoutHookDir is the directory [readouts] roots is matched against: the
// command's working directory, else the harness's project directory.
func readoutHookDir(codec harness.Codec, in harness.HookInput) string {
	if in.Cwd != "" {
		return in.Cwd
	}
	return harnessProjectDir(codec.ID())
}

// underReadoutRoots reports whether dir is one of roots or below one, on
// a path boundary; no roots means every directory.
func underReadoutRoots(roots []string, dir string) bool {
	if len(roots) == 0 {
		return true
	}
	if dir == "" {
		return false
	}
	d := filepath.Clean(dir)
	sep := string(filepath.Separator)
	for _, r := range roots {
		if r == "" {
			continue
		}
		r = filepath.Clean(r)
		if r == sep || d == r || strings.HasPrefix(d, r+sep) {
			return true
		}
	}
	return false
}

// readoutRulesFor drops the rules the user's `[readouts.rules.<id>] mode =
// "off"` turns off; every other per-rule mode reads as observe this build.
func readoutRulesFor(rs *readout.RuleSet, overrides map[string]config.ReadoutRule) *readout.RuleSet {
	if len(overrides) == 0 {
		return rs
	}
	out := &readout.RuleSet{Refused: rs.Refused}
	for _, r := range rs.Rules {
		if o, ok := overrides[r.ID]; ok && o.Mode == config.ReadoutsOff {
			continue
		}
		out.Rules = append(out.Rules, r)
	}
	return out
}

// matchedRule is the served rule whose id@version Render named; the last
// one wins, as at Render.
func matchedRule(rs *readout.RuleSet, ref string) *readout.Rule {
	if ref == "" {
		return nil
	}
	for i := len(rs.Rules) - 1; i >= 0; i-- {
		if rs.Rules[i].Ref() == ref {
			return rs.Rules[i]
		}
	}
	return nil
}

// readoutRuleSHA is the sha256 of the rule's text in the user file format
// (its canonical TOML encoding), what a report matches a row's rule by.
func readoutRuleSHA(r *readout.Rule) string {
	b, err := toml.Marshal(struct {
		Schema int            `toml:"schema"`
		Rule   []readout.Rule `toml:"rule"`
	}{1, []readout.Rule{*r}})
	if err != nil {
		return ""
	}
	return sha256Hex(string(b))
}

// readoutRow is what one hook run logs.
type readoutRow struct {
	event      string
	configured string
	in         harness.HookInput
	cmd        *shellcmd.Command
	ro         readout.Readout
	rule       *readout.Rule
	elapsed    time.Duration
}

// readoutRowMeta is the row's meta: sizes, the rule and its hash, the
// command's hash, never the output or the command text.
func readoutRowMeta(r readoutRow) map[string]string {
	ro := r.ro
	mode := ro.Mode
	if mode == "" {
		mode = readout.ModeObserve
	}
	meta := map[string]string{
		"event":      r.event,
		"mode":       mode,
		"configured": r.configured,
		"raw":        boolStr(ro.Raw),
		"bytes_in":   strconv.Itoa(ro.BytesIn),
		"bytes_out":  strconv.Itoa(ro.BytesOut),
		"lines_in":   strconv.Itoa(ro.LinesIn),
		"lines_out":  strconv.Itoa(ro.LinesOut),
		"trips":      strconv.Itoa(len(ro.Trips)),
		"ms":         strconv.FormatInt(r.elapsed.Milliseconds(), 10),
		"cmd_sha":    sha256Hex(r.in.Command),
		"emitted":    "false",
	}
	if ro.Reason != "" {
		meta["reason"] = ro.Reason
	}
	if r.rule != nil {
		meta["rule"] = ro.Rule
		meta["class"] = string(ro.Stream)
		if sha := readoutRuleSHA(r.rule); sha != "" {
			meta["rule_sha"] = sha
		}
		joins := readoutJoins(r.rule)
		meta["joins_ran"] = strconv.Itoa(len(joins) - len(ro.JoinsMissed))
		if len(ro.JoinsMissed) > 0 {
			missed := append([]string(nil), ro.JoinsMissed...)
			sort.Strings(missed)
			meta["joins_missed"] = strings.Join(missed, ",")
		}
	}
	if len(ro.Dropped) > 0 {
		parts := make([]string, 0, len(ro.Dropped))
		for _, c := range ro.Dropped {
			parts = append(parts, c.Class+":"+strconv.Itoa(c.N))
		}
		meta["dropped"] = strings.Join(parts, ",")
	}
	if p := r.cmd.ReadoutPrefix(); p != "" {
		meta["bypass"] = p
	}
	if r.in.ToolUseID != "" {
		meta["tool_use_id"] = r.in.ToolUseID
	}
	if r.elapsed > readoutHookCeiling {
		meta["ceiling"] = "missed"
	}
	return meta
}

// readoutJoins names the distinct joins the rule's stages ask for.
func readoutJoins(r *readout.Rule) []string {
	var out []string
	for _, s := range r.Stages {
		if s.Join != "" && !containsString(out, s.Join) {
			out = append(out, s.Join)
		}
	}
	return out
}

// logReadoutRow writes the run's one row. A failed write is swallowed
// (emitBeat), so logging never breaks the hook's silence.
func logReadoutRow(r readoutRow) {
	kind := "observed"
	if r.ro.Raw {
		kind = "raw"
	}
	emitBeat(readoutChannel, kind, firingMeta(readoutRowMeta(r), r.in.SessionID, ""))
}

// readoutHookRunner runs `fathom readout-hook`.
type readoutHookRunner struct{}

func (readoutHookRunner) execVerb(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return cmdReadoutHook(args, stdin, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "readout-hook", Group: "hooks", Order: 75, Run: readoutHookRunner{},
		Summary: "PostToolUse and PostToolUseFailure hook on Bash: compute the readout of a command's output and log one row (observe only; off unless [readouts] mode is set)"})
}
