// Command unknownrate is the LL-M6 scorer (docs/design/lang-library.md section 10): the unknown rate of
// resource reads. It reads the pinned subjects of subjects.tsv with pkg/lang/iac, scores every (rule, matching
// resource) pair of rules.json under the definitions in PREREGISTRATION.md, and prints one of three views:
//
//	unknownrate -dest DIR -mode md     tables for the reading
//	unknownrate -dest DIR -mode json   every scored pair and every subject's read
//	unknownrate -dest DIR -mode audit  the 50 seeded audit pairs with their source
//
// DIR is where fetch.sh checked the subjects out. The scorer shares no code with the oracle tools and never
// passes WithValues.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/lang/iac"
)

// SubjectRun is one subject's read.
type SubjectRun struct {
	Subject      Subject `json:"subject"`
	Err          string  `json:"err,omitempty"`
	Resources    int     `json:"resources"`
	Matching     int     `json:"matching"`
	SyntaxErrors int     `json:"syntax_errors"`
	Dropped      int     `json:"dropped"`
}

// Run is a whole scoring run.
type Run struct {
	Subjects []SubjectRun `json:"subjects"`
	Pairs    []Pair       `json:"pairs"`
}

func score(ctx context.Context, dest string, subjects []Subject, rules []Rule) Run {
	var run Run
	for _, s := range subjects {
		sr := SubjectRun{Subject: s}
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		set, err := readSubject(cctx, dest, s)
		cancel()
		if err != nil {
			sr.Err = err.Error()
			run.Subjects = append(run.Subjects, sr)
			continue
		}
		ps := scoreSet(set, s, rules)
		sr.Resources, sr.SyntaxErrors, sr.Dropped, sr.Matching = len(set.Resources), set.SyntaxErrors, len(set.Dropped), len(ps)
		run.Subjects = append(run.Subjects, sr)
		run.Pairs = append(run.Pairs, ps...)
	}
	sort.SliceStable(run.Pairs, func(i, j int) bool {
		a, b := run.Pairs[i], run.Pairs[j]
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return run
}

func main() {
	dest := flag.String("dest", "", "directory fetch.sh checked the subjects out into")
	rulesFile := flag.String("rules", "rules.json", "rules file")
	subjectsFile := flag.String("subjects", "subjects.tsv", "subjects file")
	mode := flag.String("mode", "md", "md, json or audit")
	flag.Parse()
	if *dest == "" {
		fmt.Fprintln(os.Stderr, "unknownrate: -dest is required")
		os.Exit(2)
	}
	rules, err := loadRules(*rulesFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unknownrate:", err)
		os.Exit(2)
	}
	subjects, err := loadSubjects(*subjectsFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unknownrate:", err)
		os.Exit(2)
	}
	run := score(context.Background(), *dest, subjects, rules)
	switch *mode {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", " ")
		err = enc.Encode(run)
	case "audit":
		err = writeAudit(os.Stdout, *dest, subjects, run)
	case "md":
		err = writeTables(os.Stdout, run, rules)
	default:
		err = fmt.Errorf("unknown mode %q", *mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "unknownrate:", err)
		os.Exit(1)
	}
}

func pct(f float64) string { return fmt.Sprintf("%.1f%%", 100*f) }

func cellRow(label string, c Cell) string {
	return fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %d | %s | %s | %s |", label, c.Pairs, c.Literal, c.Reference, c.Defaulted, c.Unknown, c.Unset,
		pct(c.rate()), pct(c.ratePresent()), pct(c.rateWithDefaulted()))
}

const cellHead = "| | pairs | literal | reference | defaulted | unknown | unset | unknown rate | rate excl. unset | rate incl. defaulted |\n|---|---|---|---|---|---|---|---|---|---|"

var tools = []string{iac.ToolTerraform, iac.ToolCloudFormation, iac.ToolPulumi}

func writeTables(w io.Writer, run Run, rules []Rule) error {
	byTool := map[string]*Cell{}
	byKind := map[string]*Cell{}
	bySubject := map[string]*Cell{}
	byRule := map[string]*Cell{}
	cell := func(m map[string]*Cell, k string) *Cell {
		if m[k] == nil {
			m[k] = &Cell{}
		}
		return m[k]
	}
	for _, p := range run.Pairs {
		cell(byTool, p.Tool).add(p.Class)
		cell(byKind, p.Tool+"/"+p.Kind).add(p.Class)
		cell(bySubject, p.Subject).add(p.Class)
		cell(byRule, p.Tool+"/"+p.Rule).add(p.Class)
	}
	fmt.Fprintf(w, "## By tool\n\n%s\n", cellHead)
	for _, t := range tools {
		fmt.Fprintln(w, cellRow(t, *cell(byTool, t)))
	}
	fmt.Fprintf(w, "\n## By tool and kind\n\n%s\n", cellHead)
	for _, t := range tools {
		for _, k := range []string{"reusable", "deployed"} {
			fmt.Fprintln(w, cellRow(t+" "+k, *cell(byKind, t+"/"+k)))
		}
	}
	fmt.Fprintf(w, "\n## By subject\n\n| subject | tool | kind | resources read | syntax errors | dropped | read error |\n|---|---|---|---|---|---|---|\n")
	for _, s := range run.Subjects {
		fmt.Fprintf(w, "| %s | %s | %s | %d | %d | %d | %s |\n", s.Subject.ID, s.Subject.Tool, s.Subject.Kind, s.Resources, s.SyntaxErrors, s.Dropped, s.Err)
	}
	fmt.Fprintf(w, "\n%s\n", strings.Replace(cellHead, "| |", "| subject |", 1))
	for _, s := range run.Subjects {
		fmt.Fprintln(w, cellRow(s.Subject.ID, *cell(bySubject, s.Subject.ID)))
	}
	for _, t := range tools {
		fmt.Fprintf(w, "\n## By rule, %s\n\n%s\n", t, strings.Replace(cellHead, "| |", "| rule |", 1))
		for _, r := range rules {
			fmt.Fprintln(w, cellRow(r.ID+" "+r.Title, *cell(byRule, t+"/"+r.ID)))
		}
	}
	fmt.Fprintf(w, "\n## n/a (rule, subject) combinations with no matching resource\n\n| tool | n/a | of |\n|---|---|---|\n")
	has := map[string]bool{}
	for _, p := range run.Pairs {
		has[p.Subject+"/"+p.Rule] = true
	}
	for _, t := range tools {
		na, total := 0, 0
		for _, s := range run.Subjects {
			if s.Subject.Tool != t || s.Err != "" {
				continue
			}
			for _, r := range rules {
				total++
				if !has[s.Subject.ID+"/"+r.ID] {
					na++
				}
			}
		}
		fmt.Fprintf(w, "| %s | %d | %d |\n", t, na, total)
	}
	return nil
}
