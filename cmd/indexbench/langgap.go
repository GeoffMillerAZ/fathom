package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

func cmdLangGap(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-gap", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "repository root")
	id := fs.String("id", "", "queue id (required)")
	reading := fs.String("reading", "", "latest scratch reading (default: committed reading)")
	fresh := fs.Bool("fresh", false, "probe and measure local calibration subjects first; never writes committed data")
	subject := fs.String("subject", "", "one local calibration subject for -fresh")
	config := fs.String("oracle-config", "", "offline oracle runners for -fresh (see lang-loop)")
	jsonOut := fs.Bool("json", false, "emit deterministic JSON")
	var unmatched multiFlag
	fs.Var(&unmatched, "unmatched", "mismatch sidecar from lang-oracle (repeatable; locations only where recorded)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *id == "" || fs.NArg() != 0 || (*fresh && *reading != "") {
		fmt.Fprintln(stderr, "usage: indexbench lang-gap -id <id> [-reading <json> | -fresh] [-unmatched <json>] [--json]")
		return 2
	}
	e, err := loopEntry(*root, *id)
	if err != nil {
		fmt.Fprintln(stderr, "lang-gap:", err)
		return 1
	}
	path := *reading
	if path == "" {
		path = filepath.Join(*root, "docs/bench/readings/lang", e.ID+".json")
	}
	if *fresh {
		work, err := loopWork(*root)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer os.RemoveAll(work)
		var notes bytes.Buffer
		path, err = measureLanguageLoop(*root, e, *subject, work, *config, &notes)
		printLoopNotes(stderr, notes.String())
		if err != nil {
			fmt.Fprintln(stderr, "lang-gap:", err)
			return 1
		}
		matches, _ := filepath.Glob(filepath.Join(work, "unmatched-*.json"))
		unmatched = append(unmatched, matches...)
	}
	r, err := loadReading(path)
	if err != nil {
		fmt.Fprintln(stderr, "lang-gap:", err)
		return 1
	}
	if r.Dialect != *id {
		fmt.Fprintln(stderr, "lang-gap: reading dialect differs from queue id")
		return 1
	}
	if *fresh {
		langbench.Regrade(&r)
	}
	r.TargetLevel = e.TargetLevel
	report := langbench.ExplainGap(r)
	if err = gapExamples(&report, unmatched); err != nil {
		fmt.Fprintln(stderr, "lang-gap:", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err = enc.Encode(report); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%s: %s of %s\n", report.ID, report.LevelMet, report.Target)
	if len(report.Gaps) == 0 {
		fmt.Fprintln(stdout, "no measured blocker below target")
	}
	for _, g := range report.Gaps {
		fmt.Fprintf(stdout, "  %s %s (%s): %d %s", g.Level, g.Class, g.Metric, g.Count, g.Unit)
		if g.Value != nil && g.Bar != nil {
			fmt.Fprintf(stdout, "; metric %.4f, bar %.4f", *g.Value, *g.Bar)
		}
		fmt.Fprintln(stdout)
		for _, ex := range g.Examples {
			fmt.Fprintln(stdout, "    ", ex)
		}
		if len(g.Examples) == 0 {
			fmt.Fprintln(stdout, "    locations unavailable in recorded reading; pass -unmatched or -fresh")
		}
		fmt.Fprintln(stdout, "    next:", g.Next)
	}
	return 0
}

func gapExamples(report *langbench.GapReport, paths []string) error {
	byClass := map[string][]string{}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var u unmatchedFile
		if err = json.Unmarshal(b, &u); err != nil {
			return err
		}
		for _, side := range []struct {
			defs  *[]langbench.OracleDef
			class string
		}{{u.OracleOnly, "missing definition"}, {u.DialectOnly, "extra definition"}} {
			if side.defs != nil {
				for _, d := range *side.defs {
					if d.Path != "" && d.Line > 0 {
						byClass[side.class] = append(byClass[side.class], fmt.Sprintf("%s:%d %s", d.Path, d.Line, d.Name))
					}
				}
			}
		}
		for _, imp := range u.Imports {
			if imp.Path != "" && imp.Line > 0 {
				byClass["unresolved import"] = append(byClass["unresolved import"], fmt.Sprintf("%s:%d", imp.Path, imp.Line))
			}
		}
	}
	for i := range report.Gaps {
		g := &report.Gaps[i]
		ex := byClass[g.Class]
		sort.Strings(ex)
		ex = slices.Compact(ex)
		if len(ex) > 3 {
			ex = ex[:3]
		}
		if len(ex) > 0 {
			g.Examples = ex
		}
	}
	return nil
}

// cmdLangNearMiss does not require eligibility: refused items can be the
// cheapest next experiment when their measured parser is close to the line.
func cmdLangNearMiss(root string, margin float64, jsonOut bool, stdout, stderr io.Writer) int {
	if margin < 0 || margin > 1 || math.IsNaN(margin) || math.IsInf(margin, 0) {
		fmt.Fprintln(stderr, "lang-next: -margin must be a finite ratio in [0,1]")
		return 2
	}
	q, err := langbench.LoadQueue(filepath.Join(root, "docs/plans/language-breadth.queue.json"))
	if err != nil {
		fmt.Fprintln(stderr, "lang-next:", err)
		return 1
	}
	rows := []langbench.NearMiss{}
	plan, err := langbench.LoadBreadthPlan(root)
	if err != nil {
		fmt.Fprintln(stderr, "lang-next:", err)
		return 1
	}
	refused := langbench.RefusedItems(q, plan)

	for _, e := range q {
		r, err := loadReading(filepath.Join(root, "docs/bench/readings/lang", e.ID+".json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			fmt.Fprintln(stderr, "lang-next:", err)
			return 1
		}
		if r.Dialect != e.ID {
			fmt.Fprintln(stderr, "lang-next: reading dialect differs from queue id", e.ID)
			return 1
		}
		r.TargetLevel = e.TargetLevel
		near := langbench.NearMisses(r, margin)
		if why, ok := refused[e.ID]; ok && r.Tenancy.FilesWithError == nil {
			if refusal, ok := refusalParseNearMiss(e.ID, why, margin); ok {
				near = append(near, refusal)
			}
		}
		if len(near) > 0 {
			commit, err := nearGrammarCommit(root, e.ID, r)
			if err != nil {
				fmt.Fprintln(stderr, "lang-next:", err)
				return 1
			}
			for i := range near {
				if near[i].GrammarCommit == "unknown" {
					near[i].GrammarCommit = commit
				}
			}
			rows = append(rows, near...)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Level != b.Level {
			return a.Level < b.Level
		}
		return a.Metric < b.Metric
	})
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err = enc.Encode(rows); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	for _, r := range rows {
		fmt.Fprintf(stdout, "%s\t%s %s=%.4f bar=%.4f grammar=%s", r.ID, r.Level, r.Metric, r.Value, r.Bar, r.GrammarCommit)
		if r.Source != "" {
			fmt.Fprint(stdout, " source=", r.Source)
		}
		fmt.Fprintln(stdout)
	}
	return 0
}

// Prefer the measured commit. The current local pin is a disclosed fallback
// for older readings which do not record one; never silently label it measured.
func nearGrammarCommit(root, id string, r langbench.Reading) (string, error) {
	if r.Grammar != nil && r.Grammar.Commit != "" {
		return r.Grammar.Commit, nil
	}
	blob := id
	if r.Grammar != nil && r.Grammar.Blob != "" {
		blob = r.Grammar.Blob
	} else {
		b, err := os.ReadFile(filepath.Join(root, "internal/extractor/dialects", id, "dialect.json"))
		if err == nil {
			var d struct {
				Grammar struct {
					Blob string `json:"blob"`
				} `json:"grammar"`
			}
			if err = json.Unmarshal(b, &d); err != nil {
				return "", err
			}
			if d.Grammar.Blob != "" {
				blob = d.Grammar.Blob
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	b, err := os.ReadFile(filepath.Join(root, "internal/extractor/tsengine/grammars/grammars.lock.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "unknown (no measured or local pin)", nil
	}
	if err != nil {
		return "", err
	}
	var lock struct {
		Grammars []struct {
			Blob   string `json:"blob"`
			Commit string `json:"commit"`
		} `json:"grammars"`
	}
	if err = json.Unmarshal(b, &lock); err != nil {
		return "", err
	}
	for _, g := range lock.Grammars {
		if g.Blob == blob {
			return g.Commit + " (current pin; measured pin unknown)", nil
		}
	}
	return "unknown (no measured or local pin)", nil
}

// This narrow legacy format is counted evidence in a refusal row, not a
// percentage inferred from arbitrary prose. Its own measured grammar pin wins.
var refusalParseCount = regexp.MustCompile(`([0-9]+) of the ([0-9]+) calibration files \(([0-9.]+)%[^)]*\) parse with an error under [^ ]+/tree-sitter-[^ ]+ ([0-9a-f]{7,40})`)

func refusalParseNearMiss(id, why string, margin float64) (langbench.NearMiss, bool) {
	m := refusalParseCount.FindStringSubmatch(why)
	if m == nil {
		return langbench.NearMiss{}, false
	}
	n, _ := strconv.Atoi(m[1])
	total, _ := strconv.Atoi(m[2])
	percent, _ := strconv.ParseFloat(m[3], 64)
	if total <= 0 || n > total || n < 0 {
		return langbench.NearMiss{}, false
	}
	value := float64(n) / float64(total)
	if math.Abs(value*100-percent) > .051 || value <= .05 || value-.05 > margin+1e-12 {
		return langbench.NearMiss{}, false
	}
	row, _, _ := strings.Cut(why, ":")
	return langbench.NearMiss{ID: id, Level: "L1", Metric: "parse_error_rate", Value: value, Bar: .05, GrammarCommit: m[4], Source: row + " (refusal measurement)"}, true
}
