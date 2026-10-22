package main

// readoutfit.go — `fathom readout fit`: the policy table's cut points fitted
// offline on the outcome labels the readout report's replay computes
// (design docs/design/readouts.md §13, ADR-073's first decision model,
// provider none). Each readout row's call is found in the local transcripts
// by its tool-use id: a served readout replayed with its rule is a lean
// label (lean was right when nothing dropped was referenced and nothing was
// recovered within five calls); a rich row the table answered under rule 1
// or 2 is an enrichment label (right when the next call touched a file or
// symbol a compile record named). internal/readout fits the cut points on a
// seeded half and scores both. --write writes the user-scope policy file
// rich mode reads; nothing else is written, and no model is asked.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	"github.com/geoffmilleraz/fathom/internal/readout"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
)

// roFitSkipped counts the rows the fit could not label, apart (never as a
// label either way).
type roFitSkipped struct {
	TranscriptGone int `json:"transcript_gone"`
	RuleChanged    int `json:"rule_changed"`
	OutputDiffers  int `json:"output_differs"`
	NoNextCall     int `json:"no_next_call"`
}

// roFit is the fit's JSON form.
type roFit struct {
	Rows       int               `json:"rows"`
	Skipped    roFitSkipped      `json:"skipped"`
	Fit        readout.FitResult `json:"fit"`
	DecidedBy  string            `json:"decided_by"`
	PolicyPath string            `json:"policy_path,omitempty"`
	Written    bool              `json:"written"`
}

// roFitRow is a readout row as the fit reads it: the report's row plus the
// event and the table's logged features.
type roFitRow struct {
	roRow
	event string
	meta  map[string]string
}

func cmdReadoutFit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("readout fit", stderr)
	seed := fs.Int64("seed", 1, "the split's seed: the same seed and rows give the same halves and cut points")
	sinceFlag := fs.String("since", "", "bound the window to the last duration (e.g. 30d; default: both hook log generations whole)")
	write := fs.Bool("write", false, "write the fitted cut points to the user's readouts policy file, which rich mode then reads in place of the built-in ones")
	jsonOut := fs.Bool("json", false, "emit the fit as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: readout fit takes no arguments (got %q)\n", fs.Arg(0))
		return 1
	}
	var since time.Time
	if *sinceFlag != "" {
		d, err := parseHumaneDuration(*sinceFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error: readout fit --since: %v\n", err)
			return 1
		}
		since = readoutNow().Add(-d).UTC()
	}
	rs, err := readout.LoadRules(fathomhome.FathomHome())
	if err != nil {
		fmt.Fprintf(stderr, "error: readout fit: %v\n", err)
		return 2
	}
	labels, rows, skipped := readoutFitLabels(rs, beatLogPath(), readoutProjectsDir(), since)
	out := roFit{Rows: rows, Skipped: skipped, Fit: readout.Fit(labels, *seed), DecidedBy: readout.DecidedByTable,
		PolicyPath: readout.PolicyPath(fathomhome.FathomHome())}
	if *write {
		if err := readout.WritePolicy(fathomhome.FathomHome(), out.Fit); err != nil {
			fmt.Fprintf(stderr, "error: readout fit --write: %v\n", err)
			return 2
		}
		out.Written = true
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return 2
		}
		return 0
	}
	printReadoutFit(stdout, out)
	return 0
}

// readoutFitLabels reads both hook-log generations' readout rows at or
// after since and labels each one its call can be replayed for, in log
// order (the split hashes the ids, so order never reaches the fit).
func readoutFitLabels(rs *readout.RuleSet, logPath, projects string, since time.Time) (labels []readout.FitLabel, rows int, skipped roFitSkipped) {
	all := readoutFitRows(logPath, since)
	rows = len(all)
	bySHA := map[string]*readout.Rule{}
	for _, r := range rs.Rules {
		if sha := readoutRuleSHA(r); sha != "" {
			bySHA[sha] = r
		}
	}
	want, sessions := map[string]bool{}, map[string]bool{}
	allSessions := false
	var from time.Time
	var cand []roFitRow
	for _, r := range all {
		lean := r.rule != "" && !r.raw
		enrich := r.meta["policy_rule"] == "1" || r.meta["policy_rule"] == "2"
		if r.toolUseID == "" || want[r.toolUseID] || !(lean || enrich) {
			continue
		}
		if lean && bySHA[r.ruleSHA] == nil && !enrich {
			skipped.RuleChanged++
			continue
		}
		want[r.toolUseID] = true
		cand = append(cand, r)
		if from.IsZero() || r.at.Before(from) {
			from = r.at
		}
		if r.session == "" {
			allSessions = true
		}
		sessions[r.session] = true
	}
	if len(want) == 0 || projects == "" {
		skipped.TranscriptGone += len(cand)
		return nil, rows, skipped
	}
	if allSessions {
		sessions = nil
	}
	calls := readoutReportCalls(projects, from, want, sessions)
	for _, r := range cand {
		call := calls[r.toolUseID]
		if call == nil {
			skipped.TranscriptGone++
			continue
		}
		l, why := readoutFitLabelOf(r, call, bySHA[r.ruleSHA])
		switch why {
		case "":
			labels = append(labels, l)
		case "rule":
			skipped.RuleChanged++
		case "output":
			skipped.OutputDiffers++
		case "next":
			skipped.NoNextCall++
		}
	}
	return labels, rows, skipped
}

// readoutFitLabelOf labels one row's call: its features from a re-render
// (or the row's logged ones), the lean outcome from the report's replay,
// the enrichment outcome from the next call. why names the reason when
// neither outcome is known.
func readoutFitLabelOf(r roFitRow, call *roCall, rule *readout.Rule) (l readout.FitLabel, why string) {
	l.ID = r.toolUseID
	cmd := parseShellCommand(call.command).Transparent()
	f := readout.Features{Event: readout.EventSuccess, Bytes: r.bytesIn, Recovery: -1, Records: map[string]int{}}
	if r.event == "tool-failure" || call.interrupted {
		f.Event = readout.EventFailure
	}
	f.Lines, _ = strconv.Atoi(r.meta["lines_in"])
	_, f.WholeRead = readout.WholeFileRead(&cmd)
	f.Class = readout.StreamOf(&cmd, nil)
	if f.Class == readout.StreamDiagnostic {
		f.Protected = readout.ProtectedLines(call.stdout)
	}
	f.Protected += readout.ProtectedLines(call.stderr)
	why = "output"
	if rule != nil && r.rule != "" && !r.raw {
		f.RuleFloor = rule.When.BytesGT
		ro := readout.Render(&readout.RuleSet{Rules: []*readout.Rule{rule}}, readout.Input{
			Command: &cmd, Stdout: call.stdout, Stderr: call.stderr, Interrupted: call.interrupted,
			Persisted: call.persisted, Mode: r.mode,
		})
		f.Class, f.Lines = ro.Stream, ro.LinesIn
		for _, rec := range ro.Records {
			f.Records[rec.Kind]++
		}
		if referenced, _, recovered, ok := call.replay(rule, r.roRow); ok {
			l.LeanKnown, l.LeanRight = true, !referenced && !recovered
		}
	} else if r.rule != "" && !r.raw {
		why = "rule"
	}
	if r.meta["policy_rule"] == "1" || r.meta["policy_rule"] == "2" {
		f.EditedOverlap, _ = strconv.Atoi(r.meta["edited_overlap"])
		f.FailingOverlap, _ = strconv.Atoi(r.meta["failing_overlap"])
		for _, kv := range strings.Split(r.meta["records"], ",") {
			if k, v, ok := strings.Cut(kv, ":"); ok {
				n, _ := strconv.Atoi(v)
				f.Records[k] += n
			}
		}
		if len(call.followers) == 0 {
			if !l.LeanKnown {
				why = "next"
			}
		} else {
			l.EnrichKnown, l.EnrichRight = true, readoutFitTouched(call, call.followers[0])
		}
	}
	l.Features = f
	if l.LeanKnown || l.EnrichKnown {
		why = ""
	}
	return l, why
}

// readoutFitTouched reports whether next names a file or symbol one of the
// call's compile records names: the same path or a '/'-suffix of it, the
// identifier, or its last part.
func readoutFitTouched(call *roCall, next *roFollower) bool {
	text := call.stdout
	if call.stderr != "" {
		text += "\n" + call.stderr
	}
	for _, rec := range readout.CompileErrors(text, nil) {
		for _, fd := range rec.Fields {
			if fd.Value == "" || (fd.Name != "file" && fd.Name != "ident") {
				continue
			}
			names := []string{fd.Value}
			if fd.Name == "ident" {
				if i := strings.LastIndexByte(fd.Value, '.'); i >= 0 && len(fd.Value)-i > 3 {
					names = append(names, fd.Value[i+1:])
				}
			}
			for t := range next.tokens {
				for _, n := range names {
					if t == n || strings.HasSuffix(t, "/"+n) || strings.HasSuffix(n, "/"+t) {
						return true
					}
				}
			}
		}
	}
	return false
}

// readoutFitRows streams the readout rows of both generations at or after
// since that name a rule or the table's answer: the report's row plus the
// event and the meta the fit reads.
func readoutFitRows(path string, since time.Time) []roFitRow {
	if path == "" {
		return nil
	}
	var out []roFitRow
	marker := []byte(`"channel":"` + readoutChannel + `"`)
	for _, p := range []string{path + ".1", path} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		br := bufio.NewReaderSize(f, 1<<20)
		for {
			line, rerr := br.ReadBytes('\n')
			if bytes.Contains(line, marker) {
				if r, ok := readoutFitRowOf(line, since); ok {
					out = append(out, r)
				}
			}
			if rerr != nil {
				break
			}
		}
		f.Close()
	}
	return out
}

func readoutFitRowOf(line []byte, since time.Time) (roFitRow, bool) {
	var b heartbeat.Beat
	if json.Unmarshal(line, &b) != nil || b.Channel != readoutChannel || (!since.IsZero() && b.At.Before(since)) {
		return roFitRow{}, false
	}
	m := b.Meta
	if m["rule"] == "" && m["policy_rule"] == "" {
		return roFitRow{}, false
	}
	r := roRow{at: b.At, rule: m["rule"], ruleSHA: m["rule_sha"], toolUseID: m["tool_use_id"], session: m["session"],
		mode: m["mode"], cmdSHA: m["cmd_sha"], raw: m["raw"] == "true"}
	r.bytesIn, _ = strconv.Atoi(m["bytes_in"])
	r.bytesOut, _ = strconv.Atoi(m["bytes_out"])
	if r.mode == "" {
		r.mode = readout.ModeObserve
	}
	return roFitRow{roRow: r, event: m["event"], meta: m}, true
}

// printReadoutFit prints the fit: the labels and what was skipped, the cut
// points against the built-ins, both halves' accuracy with Wilson bounds,
// and where the policy file is or would be written.
func printReadoutFit(w io.Writer, f roFit) {
	r := f.Fit
	sk := f.Skipped
	fmt.Fprintf(w, "readout fit (seed %d): %d labels from %d readout rows, %d fitted and %d held out; not labelled: transcript gone %d, rule changed %d, output differs %d, no next call %d\n",
		r.Seed, r.Labels, f.Rows, r.Fit.Labels, r.HeldOut.Labels, sk.TranscriptGone, sk.RuleChanged, sk.OutputDiffers, sk.NoNextCall)
	floor := "each rule's own"
	if r.Policy.LeanFloorBytes > 0 {
		floor = readoutFitInt(r.Policy.LeanFloorBytes) + " bytes"
	}
	fmt.Fprintf(w, "cut points: lean floor %s (built-in: each rule's own); long read %d lines (built-in %d)\n",
		floor, r.Policy.LongReadLines, r.Builtin.LongReadLines)
	for _, h := range []struct {
		name string
		half readout.FitHalf
	}{{"fitted half", r.Fit}, {"held-out half", r.HeldOut}} {
		fmt.Fprintf(w, "%s: %s; contested band %s\n", h.name, readoutFitAccuracy(h.half.All), readoutFitAccuracy(h.half.Contested))
	}
	fmt.Fprintln(w, "scored as the table would serve a promoted rule; rich mode serves lean for none today")
	fmt.Fprintf(w, "decided_by: %s\n", f.DecidedBy)
	switch {
	case f.Written:
		fmt.Fprintf(w, "written: %s (rich mode reads it in place of the built-in cut points)\n", f.PolicyPath)
	case f.PolicyPath != "":
		fmt.Fprintf(w, "not written: --write writes %s\n", f.PolicyPath)
	}
}

func readoutFitAccuracy(a readout.FitAccuracy) string {
	if a.N == 0 {
		return "no labels"
	}
	return fmt.Sprintf("accuracy %.4f (%d/%d), 95%% Wilson %.4f-%.4f", a.Accuracy, a.Right, a.N, a.Lower, a.Upper)
}

// readoutFitInt is n with thousands separators.
func readoutFitInt(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
