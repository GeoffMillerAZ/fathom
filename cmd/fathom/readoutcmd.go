package main

// readoutcmd.go — `fathom readout`: the readouts CLI (ADR-071, goal G39;
// design docs/design/readouts.md §8 and §9). Three subverbs, none of which
// runs a command or installs anything:
//
//   - replay (readoutcmd_replay.go): the rules over the local Claude Code
//     transcripts, aggregates only;
//   - explain: one command string and its output on stdin, through the
//     engine: the rule, its stages, what would drop and why, the receipt;
//   - test: each rule file's fixtures and the contract (exit 0, 1, 2).

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/readout"
	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

// readoutStdinMax bounds what explain reads from stdin.
const readoutStdinMax = 16 << 20

// readoutRunner runs `fathom readout` (registered below).
type readoutRunner struct{}

func (readoutRunner) execVerb(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return cmdReadout(args, stdin, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "readout", Group: "operate", Order: 75, Run: readoutRunner{},
		Summary: "Readout rules, offline: replay [--days N] [--projects GLOB] [--rules FILE] [--json] aggregates them over local transcripts | explain [--rules FILE] '<command>' < output shows what a readout would drop and why | test [file...] runs rule fixtures and the contract | report [--since DURATION] [--json] totals served readouts"})
}

// cmdReadout dispatches the readout subverbs.
func cmdReadout(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: readout requires a subcommand: replay, explain, test, report")
		return 1
	}
	switch args[0] {
	case "replay":
		return cmdReadoutReplay(args[1:], stdout, stderr)
	case "explain":
		return cmdReadoutExplain(args[1:], stdin, stdout, stderr)
	case "test":
		return cmdReadoutTest(args[1:], stdout, stderr)
	case "report":
		return cmdReadoutReport(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown readout subcommand %q (want replay, explain, test, report)\n", args[0])
		return 1
	}
}

// loadReadoutRules is the rule set a subverb reads: one file alone with
// --rules, else the built-ins and the user's files under the fathom home.
func loadReadoutRules(file string) (*readout.RuleSet, error) {
	if file != "" {
		return readout.LoadFile(file)
	}
	return readout.LoadRules(fathomhome.FathomHome())
}

// readoutExplain is explain's JSON form.
type readoutExplain struct {
	Unreadable string               `json:"unreadable,omitempty"`
	Rule       string               `json:"rule,omitempty"`
	Mode       string               `json:"mode,omitempty"`
	Stream     string               `json:"stream,omitempty"`
	Lossiness  string               `json:"lossiness,omitempty"`
	Recovery   string               `json:"recovery,omitempty"`
	Stages     []string             `json:"stages,omitempty"`
	Raw        bool                 `json:"raw"`
	Reason     string               `json:"reason,omitempty"`
	Detail     string               `json:"detail,omitempty"`
	BytesIn    int                  `json:"bytes_in"`
	BytesOut   int                  `json:"bytes_out"`
	LinesIn    int                  `json:"lines_in"`
	LinesOut   int                  `json:"lines_out"`
	Dropped    []readout.ClassCount `json:"dropped_by_class,omitempty"`
	Trips      []readout.Trip       `json:"trips,omitempty"`
	Records    map[string]int       `json:"records,omitempty"`
	NotInView  []string             `json:"not_in_view,omitempty"`
	Receipt    string               `json:"receipt,omitempty"`
}

// cmdReadoutExplain renders one command's output through the engine and
// says what would happen: never runs the command.
func cmdReadoutExplain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := newFlagSet("readout explain", stderr)
	rulesFile := fs.String("rules", "", "a rule file to explain against alone (default: the built-ins and the files under the fathom home)")
	mode := fs.String("mode", "", "the mode to render in: observe, lean or rich (default: each rule's own)")
	jsonOut := fs.Bool("json", false, "emit the explanation as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	switch *mode {
	case "", readout.ModeObserve, readout.ModeLean, readout.ModeRich:
	default:
		fmt.Fprintf(stderr, "error: readout explain --mode %q: want observe, lean or rich\n", *mode)
		return 1
	}
	if fs.NArg() != 1 || strings.TrimSpace(fs.Arg(0)) == "" {
		fmt.Fprintln(stderr, "error: readout explain takes one command string, with its output on stdin")
		return 1
	}
	rs, err := loadReadoutRules(*rulesFile)
	if err != nil {
		fmt.Fprintf(stderr, "error: readout explain: %v\n", err)
		return 2
	}
	data, err := io.ReadAll(io.LimitReader(stdin, readoutStdinMax))
	if err != nil {
		fmt.Fprintf(stderr, "error: readout explain: reading stdin: %v\n", err)
		return 2
	}
	cmd := shellcmd.ParseCommand(fs.Arg(0))
	ro := readout.Render(rs, readout.Input{Command: &cmd, Stdout: string(data), Mode: *mode})
	ex := readoutExplain{
		Rule: ro.Rule, Mode: ro.Mode, Stream: string(ro.Stream), Raw: ro.Raw, Reason: ro.Reason, Detail: ro.Detail,
		BytesIn: ro.BytesIn, BytesOut: ro.BytesOut, LinesIn: ro.LinesIn, LinesOut: ro.LinesOut,
		Dropped: ro.Dropped, Trips: ro.Trips, Receipt: ro.Receipt,
	}
	if cmd.Unreadable {
		ex.Unreadable = cmd.Reason
	}
	if r := readoutRuleByRef(rs, ro.Rule); r != nil {
		ex.Lossiness, ex.Recovery = readout.Lossiness(r.Stages).String(), r.Recovery
		for _, s := range r.Stages {
			ex.Stages = append(ex.Stages, s.Kind)
		}
	}
	for _, rec := range ro.Records {
		if ex.Records == nil {
			ex.Records = map[string]int{}
		}
		ex.Records[rec.Kind]++
	}
	if !ro.Raw {
		ex.NotInView = linesNotInView(string(data), ro.Stdout)
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(ex); err != nil {
			return 2
		}
		return 0
	}
	printReadoutExplain(stdout, ex)
	return 0
}

func printReadoutExplain(w io.Writer, ex readoutExplain) {
	if ex.Unreadable != "" {
		fmt.Fprintf(w, "command: unreadable (%s): served raw\n", ex.Unreadable)
	} else {
		fmt.Fprintln(w, "command: lexed")
	}
	if ex.Rule == "" {
		fmt.Fprintf(w, "rule: none matched (%s)\n", ex.Reason)
		return
	}
	fmt.Fprintf(w, "rule: %s (mode %s, stream: %s, lossiness %s, recovery %s)\n", ex.Rule, ex.Mode, ex.Stream, ex.Lossiness, readoutOrNone(ex.Recovery))
	fmt.Fprintf(w, "stages: %s\n", strings.Join(ex.Stages, ", "))
	if ex.Raw {
		why := ex.Reason
		if ex.Detail != "" {
			why += ": " + ex.Detail
		}
		fmt.Fprintf(w, "result: raw (%s)\n", why)
	} else {
		fmt.Fprintln(w, "result: readout")
	}
	fmt.Fprintf(w, "bytes: %d -> %d; lines: %d -> %d\n", ex.BytesIn, ex.BytesOut, ex.LinesIn, ex.LinesOut)
	var parts []string
	for _, c := range ex.Dropped {
		parts = append(parts, fmt.Sprintf("%s %d", c.Class, c.N))
	}
	fmt.Fprintf(w, "dropped by class: %s\n", readoutOrNone(strings.Join(parts, ", ")))
	parts = parts[:0]
	for _, t := range ex.Trips {
		parts = append(parts, fmt.Sprintf("line %d (%s)", t.Line, t.Class))
	}
	fmt.Fprintf(w, "protected trips: %s\n", readoutOrNone(strings.Join(parts, ", ")))
	kinds := make([]string, 0, len(ex.Records))
	for k := range ex.Records {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	parts = parts[:0]
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s %d", k, ex.Records[k]))
	}
	fmt.Fprintf(w, "records: %s\n", readoutOrNone(strings.Join(parts, ", ")))
	if len(ex.NotInView) > 0 {
		fmt.Fprintln(w, "not in the view:")
		for _, l := range ex.NotInView {
			fmt.Fprintf(w, "  %s\n", l)
		}
	}
	if ex.Receipt != "" {
		fmt.Fprintf(w, "receipt: %s\n", ex.Receipt)
	}
}

func readoutOrNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// readoutRuleByRef finds a rule by its id@version.
func readoutRuleByRef(rs *readout.RuleSet, ref string) *readout.Rule {
	if rs == nil || ref == "" {
		return nil
	}
	for _, r := range rs.Rules {
		if r.Ref() == ref {
			return r
		}
	}
	return nil
}

// linesNotInView lists the input lines (numbered) the view does not show
// verbatim: dropped, or rewritten by a stage.
func linesNotInView(in, view string) []string {
	have := map[string]int{}
	for _, l := range strings.Split(view, "\n") {
		have[l]++
	}
	var out []string
	for i, l := range strings.Split(strings.TrimSuffix(in, "\n"), "\n") {
		if have[l] > 0 {
			have[l]--
			continue
		}
		out = append(out, fmt.Sprintf("%d: %s", i+1, l))
	}
	return out
}

// cmdReadoutTest runs each rule file through the loader: its fixtures
// (each twice, for determinism), the protected-line contract per stream,
// and every lossless stage's inverse. Exit 0 when every rule is served, 1
// when a rule is refused, 2 when a file cannot load.
func cmdReadoutTest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("readout test", stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	var sets []*readout.RuleSet
	code := 0
	if fs.NArg() == 0 {
		rs, err := readout.LoadRules(fathomhome.FathomHome())
		if err != nil {
			fmt.Fprintf(stdout, "cannot load  the built-in rules: %v\n", err)
			return 2
		}
		sets = append(sets, rs)
	}
	for _, f := range fs.Args() {
		rs, err := readout.LoadFile(f)
		if err != nil {
			fmt.Fprintf(stdout, "cannot load  %s: %v\n", f, err)
			code = 2
			continue
		}
		sets = append(sets, rs)
	}
	for _, rs := range sets {
		for _, r := range rs.Rules {
			fmt.Fprintf(stdout, "ok    %s  lossiness %s, recovery %s, %d fixtures  (%s)\n",
				r.Ref(), readout.Lossiness(r.Stages), readoutOrNone(r.Recovery), len(r.Tests), r.Source)
		}
		for _, ref := range rs.Refused {
			if ref.ID == "" {
				fmt.Fprintf(stdout, "cannot load  %s: %s\n", ref.Source, ref.Reason)
				code = 2
				continue
			}
			fmt.Fprintf(stdout, "FAIL  %s  %s  (%s)\n", ref.ID, ref.Reason, ref.Source)
			if code == 0 {
				code = 1
			}
		}
	}
	return code
}
