package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

const retimeNotePrefix = "throughput (design 1.9 T1, T2)"
const retimeUsage = "usage: indexbench lang-retime -reading <reading.json> -subject <name>=<dir> [-subject ...] [-runs 3] [-write]"

func cmdLangRetime(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-retime", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("reading", "", "reading to re-time (required)")
	runs := fs.Int("runs", 3, "number of runs (at least 3)")
	write := fs.Bool("write", false, "save the re-timed reading")
	var subjects multiFlag
	fs.Var(&subjects, "subject", "recorded subject name=checkout directory (repeatable, required for every subject)")
	fs.Usage = func() { fmt.Fprintln(stderr, retimeUsage); fs.PrintDefaults() }
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *path == "" || *runs < 3 || fs.NArg() != 0 {
		fs.Usage()
		return 2
	}
	roots := map[string]string{}
	for _, value := range subjects {
		name, dir := splitNamed(value)
		if name == "" || dir == "" || roots[name] != "" {
			fmt.Fprintln(stderr, "lang-retime: -subject requires a unique name=dir")
			return 2
		}
		roots[name] = dir
	}
	refuse := func(err error) int { fmt.Fprintln(stderr, "lang-retime: refused:", err); return 1 }
	r, err := loadReading(*path)
	if err != nil {
		return refuse(err)
	}
	if len(r.Subjects) == 0 {
		return refuse(fmt.Errorf("reading has no subjects"))
	}
	for _, sub := range r.Subjects {
		if roots[sub.Name] == "" {
			fmt.Fprintf(stderr, "lang-retime: missing -subject mapping for %s\n", sub.Name)
			return 2
		}
	}
	ctx := context.Background()
	spec, source, err := resolveBenchSpec(r.Dialect, "")
	if err != nil {
		return refuse(err)
	}
	with, closeExtractor, err := langOracleExtractor(spec.Manifest, source)
	if err != nil {
		return refuse(err)
	}
	defer closeExtractor()
	// Subject order is only a traversal order; the saved reading retains its order.
	order := make([]int, len(r.Subjects))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return r.Subjects[order[i]].Name < r.Subjects[order[j]].Name })
	scopes := map[string][]string{}
	for _, sub := range r.Subjects {
		for _, note := range r.Notes {
			if globs, ok := parseOnlyScopeNote(sub.Name, note); ok {
				if old, exists := scopes[sub.Name]; exists && !slices.Equal(old, globs) {
					return refuse(fmt.Errorf("%s: conflicting -only disclosures", sub.Name))
				}
				scopes[sub.Name] = globs
			}
		}
	}
	type timedRun struct {
		subjects   []langbench.Subject
		throughput float64
		number     int
	}
	timings := make([]timedRun, *runs)
	values := make([]string, *runs)
	for run := range timings {
		timed := timedRun{subjects: make([]langbench.Subject, len(r.Subjects)), number: run + 1}
		var totalBytes int64
		var cpuMS float64
		for _, i := range order {
			original := r.Subjects[i]
			root := roots[original.Name]
			sha := gitOut(ctx, root, "rev-parse", "HEAD")
			if sha == "" || sha != original.SHA {
				return refuse(fmt.Errorf("run %d %s: sha recorded %q, measured %q", run+1, original.Name, original.SHA, sha))
			}
			measured, err := measureSubject(ctx, spec, root, original.Name, oracleOpts{only: scopes[original.Name], noOracles: true}, with)
			if err != nil {
				return refuse(fmt.Errorf("run %d %s: %w", run+1, original.Name, err))
			}
			if measured.subject.SHA != original.SHA {
				return refuse(fmt.Errorf("run %d %s: sha changed during measurement", run+1, original.Name))
			}
			if diff := retimeIdentityDiff(original, measured.subject); len(diff) > 0 {
				return refuse(fmt.Errorf("run %d %s:\n  %s\nextraction changed; the reading needs a full re-measure, not a re-time", run+1, original.Name, strings.Join(diff, "\n  ")))
			}
			t := measured.subject.Tenancy
			if t.ExtractCPUMS <= 0 || math.IsNaN(t.ExtractCPUMS) || math.IsInf(t.ExtractCPUMS, 0) {
				return refuse(fmt.Errorf("run %d %s: CPU timing unavailable", run+1, original.Name))
			}
			timed.subjects[i] = measured.subject
			totalBytes += measured.subject.Bytes
			cpuMS += t.ExtractCPUMS
		}
		// Match langbench.Regrade's pooled CPU throughput and rounding.
		timed.throughput = math.Round(float64(totalBytes)/1e6/(cpuMS/1000)*100) / 100
		timings[run] = timed
		values[run] = fmt.Sprintf("run %d: %.2f MB/CPU-s", run+1, timed.throughput)
	}
	sort.SliceStable(timings, func(i, j int) bool { return timings[i].throughput < timings[j].throughput })
	// For an even number of runs choose the upper central run: timings must
	// come from one actual run, never an average of unrelated subject timings.
	median := timings[len(timings)/2]
	before := r.LevelMet
	for i := range r.Subjects {
		old, fresh := r.Subjects[i].Tenancy, median.subjects[i].Tenancy
		old.ExtractMS, old.ExtractCPUMS, old.MaxFileMS = fresh.ExtractMS, fresh.ExtractCPUMS, fresh.MaxFileMS
	}
	var earlier []string
	kept := make([]string, 0, len(r.Notes)+1)
	for _, note := range r.Notes {
		if strings.HasPrefix(note, retimeNotePrefix) {
			earlier = append(earlier, "earlier: "+note)
		} else {
			kept = append(kept, note)
		}
	}
	note := fmt.Sprintf("%s: %s; %s; median run %d: %.2f MB/CPU-s; every other field is the original measurement at its recorded fathom_commit %s", retimeNotePrefix, retimeMachineClass(), strings.Join(values, "; "), median.number, median.throughput, r.FathomCommit)
	if len(earlier) > 0 {
		note += "; " + strings.Join(earlier, "; ")
	}
	r.Notes = append(kept, note)
	sort.Strings(r.Notes)
	langbench.Regrade(&r)
	if err := r.Validate(); err != nil {
		return refuse(err)
	}
	if *write {
		data, err := langbench.MarshalReading(r)
		if err != nil {
			return refuse(err)
		}
		if err := os.WriteFile(*path, data, 0644); err != nil {
			return refuse(err)
		}
	}
	for _, value := range values {
		fmt.Fprintln(stdout, value)
	}
	fmt.Fprintf(stdout, "median run %d: %.2f MB/CPU-s\n%s: level_met %s -> %s grade %+v\n", median.number, median.throughput, r.Dialect, before, r.LevelMet, r.Grade)
	return 0
}

// retimeIdentityDiff compares only facts measured before any oracle is read.
// Oracle-scored counts cannot be reconstructed by a re-time.
func retimeIdentityDiff(recorded, measured langbench.Subject) []string {
	var diff []string
	compare := func(field string, a, b any) {
		if fmt.Sprint(a) != fmt.Sprint(b) {
			diff = append(diff, fmt.Sprintf("%s: recorded %v -> measured %v", field, a, b))
		}
	}
	compare("files", recorded.Files, measured.Files)
	compare("bytes", recorded.Bytes, measured.Bytes)
	if recorded.Tenancy == nil || measured.Tenancy == nil {
		return append(diff, "tenancy: missing extraction identity")
	}
	compare("faults", recorded.Tenancy.Faults, measured.Tenancy.Faults)
	a, b := slices.Clone(recorded.Tenancy.FaultingFiles), slices.Clone(measured.Tenancy.FaultingFiles)
	sort.Strings(a)
	sort.Strings(b)
	compare("faulting_files", fmt.Sprintf("%q", a), fmt.Sprintf("%q", b))
	parseErrors := func(p *int) string {
		if p == nil {
			return "not measured"
		}
		return fmt.Sprint(*p)
	}
	compare("files_with_error", parseErrors(recorded.Tenancy.FilesWithError), parseErrors(measured.Tenancy.FilesWithError))
	// type_refs is the pre-oracle counter, including when its measured value
	// is zero and absent from the sparse map. Include any other counters the
	// extraction-only path computes so future additions are also checked.
	keys := map[string]bool{"type_refs": true}
	for k := range measured.Counts {
		keys[k] = true
	}
	for _, k := range sortedKeys(keys) {
		compare("counts."+k, recorded.Counts[k], measured.Counts[k])
	}
	return diff
}

// retimeMachineClass never includes machine identity or raw command output.
func retimeMachineClass() string {
	family := "CPU family unavailable"
	if raw, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
		if apple := regexp.MustCompile(`^Apple (M[0-9]+|A[0-9]+)\b`).FindString(strings.TrimSpace(string(raw))); apple != "" {
			family = apple
		}
	}
	return runtime.GOOS + "/" + runtime.GOARCH + " " + family
}
