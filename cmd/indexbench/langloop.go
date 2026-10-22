package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	"gopkg.in/yaml.v3"
)

type langLoopOps struct {
	measure func(langbench.QueueEntry, string, string, io.Writer) (string, error)
	tracker func(string, langbench.Reading) ([]byte, error)
}

func cmdLangLoop(args []string, stdout, stderr io.Writer) int {
	return runLangLoop(args, stdout, stderr, langLoopOps{})
}

func runLangLoop(args []string, stdout, stderr io.Writer, ops langLoopOps) int {
	flags := flag.NewFlagSet("lang-loop", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	subject := flags.String("subject", "", "one local calibration subject (default: all local shard checkouts)")
	write := flags.Bool("write", false, "write reading and tracker only after a non-regressing run (WRITE=1 in task)")
	config := flags.String("oracle-config", "", "local oracle runner JSON, keyed by queue oracle name; no installs")
	replay := flags.String("reading", "", "regrade a fixture/latest reading instead of a fresh measurement (offline replay)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(stderr, "usage: indexbench lang-loop [flags] <id> [SUBJECT=<path>] [WRITE=1]")
		return 2
	}
	for _, arg := range flags.Args()[1:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintln(stderr, "lang-loop: unexpected argument", arg)
			return 2
		}
		switch key {
		case "SUBJECT":
			*subject = value
		case "WRITE":
			if value != "0" && value != "1" {
				fmt.Fprintln(stderr, "lang-loop: WRITE must be 0 or 1")
				return 2
			}
			*write = value == "1"
		default:
			fmt.Fprintln(stderr, "lang-loop: unknown assignment", key)
			return 2
		}
	}
	e, err := loopEntry(*root, flags.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "lang-loop:", err)
		return 1
	}
	committed := filepath.Join(*root, "docs/bench/readings/lang", e.ID+".json")
	before, err := loadReading(committed)
	hasBefore := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stderr, "lang-loop:", err)
		return 1
	}
	work, err := loopWork(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer os.RemoveAll(work)
	var stages bytes.Buffer
	if ops.measure == nil {
		ops.measure = func(e langbench.QueueEntry, subject, work string, w io.Writer) (string, error) {
			return measureLanguageLoop(*root, e, subject, work, *config, w)
		}
	}
	path := *replay
	if path == "" {
		path, err = ops.measure(e, *subject, work, &stages)
	} else {
		fmt.Fprintln(&stages, "replay: probe and oracles skipped; recorded subject counts only")
	}
	if err != nil {
		fmt.Fprintln(stderr, "lang-loop:", err)
		printLoopNotes(stderr, stages.String())
		return 1
	}
	after, err := loadReading(path)
	if err != nil {
		fmt.Fprintln(stderr, "lang-loop:", err)
		return 1
	}
	if after.Dialect != e.ID {
		fmt.Fprintln(stderr, "lang-loop: reading dialect does not match queue item")
		return 1
	}
	after.TargetLevel = e.TargetLevel
	langbench.Regrade(&after)
	if err = after.Validate(); err != nil {
		fmt.Fprintln(stderr, "lang-loop: reading refused:", err)
		return 1
	}
	if ops.tracker == nil {
		ops.tracker = loopTracker
	}
	tracker, err := ops.tracker(*root, after)
	if err != nil {
		fmt.Fprintln(stderr, "lang-loop: tracker:", err)
		return 1
	}
	b, err := langbench.MarshalReading(after)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	printLoopSummary(stdout, before, after, hasBefore)
	for _, o := range after.Oracles {
		if o.Status != "" {
			fmt.Fprintf(&stages, "oracle %s: %s\n", o.Tool, o.Status)
		}
	}
	printLoopNotes(stdout, stages.String())
	regressed := []string{}
	if hasBefore {
		regressed = langbench.ReadingRegression(before, after)
	}
	if len(regressed) > 0 {
		fmt.Fprintln(stdout, "regression:", strings.Join(regressed, ", "))
		return 1
	}
	if *write {
		if err = os.WriteFile(committed, b, 0644); err == nil {
			err = os.WriteFile(filepath.Join(*root, "docs/languages.md"), tracker, 0644)
		}
		if err != nil {
			fmt.Fprintln(stderr, "lang-loop: write:", err)
			return 1
		}
		fmt.Fprintln(stdout, "wrote reading and tracker")
	} else {
		fmt.Fprintln(stdout, "no write (use WRITE=1 to save reading and tracker)")
	}
	return 0
}

func loopEntry(root, id string) (langbench.QueueEntry, error) {
	q, err := langbench.LoadQueue(filepath.Join(root, "docs/plans/language-breadth.queue.json"))
	if err != nil {
		return langbench.QueueEntry{}, err
	}
	for _, e := range q {
		if e.ID == id {
			return e, nil
		}
	}
	return langbench.QueueEntry{}, fmt.Errorf("unknown queue id %q", id)
}

func loopWork(root string) (string, error) {
	dir := filepath.Join(root, "workspace/tmp")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return os.MkdirTemp(dir, "lang-loop-")
}

func loopTracker(root string, r langbench.Reading) ([]byte, error) {
	in, err := langbench.TrackerInputFromRepo(root)
	if err != nil {
		return nil, err
	}
	in.Readings[r.Dialect] = r
	if errs := langbench.CheckDispatchColumn(in.Template, in.Dispatch); len(errs) > 0 {
		return nil, fmt.Errorf("%v", errs)
	}
	doc := langbench.RenderTracker(in)
	if errs := langbench.CheckTrackerAgainstReadings(doc, in.Readings); len(errs) > 0 {
		return nil, fmt.Errorf("%v", errs)
	}
	return []byte(doc), nil
}

func printLoopSummary(w io.Writer, before, after langbench.Reading, hasBefore bool) {
	fmt.Fprintf(w, "lang-loop %s: level_met %s (target %s)\n", after.Dialect, after.LevelMet, after.TargetLevel)
	for _, x := range []struct{ l, old, grade string }{{"L1", before.Grade.L1, after.Grade.L1}, {"L2", before.Grade.L2, after.Grade.L2}, {"L3", before.Grade.L3, after.Grade.L3}, {"L4", before.Grade.L4, after.Grade.L4}} {
		met := x.grade == langbench.GradeGood || x.grade == langbench.GradeExcellent
		fmt.Fprintf(w, "  %s met=%t grade=%s", x.l, met, x.grade)
		if hasBefore {
			fmt.Fprintf(w, " delta=%s -> %s", x.old, x.grade)
		}
		fmt.Fprintln(w)
	}
	if hasBefore {
		keys := make([]string, 0, len(after.Metrics))
		for k := range after.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var delta []string
		for _, k := range keys {
			if a, b := after.Metrics[k], before.Metrics[k]; a != nil && b != nil && *a != *b {
				delta = append(delta, fmt.Sprintf("%s %+.4f", k, *a-*b))
			}
		}
		fmt.Fprintf(w, "  delta level_met %s -> %s; metrics: %s\n", before.LevelMet, after.LevelMet, strings.Join(delta, ", "))
	} else {
		fmt.Fprintln(w, "  delta: no committed reading")
	}
	gaps := langbench.MismatchClasses(after)
	// The top three classes are ranked by blocker count, ties lexically.
	counts := map[string]int{}
	for _, g := range gaps {
		counts[g.Class] += g.Count
	}
	classes := make([]string, 0, len(counts))
	for k := range counts {
		classes = append(classes, k)
	}
	sort.Slice(classes, func(i, j int) bool {
		if counts[classes[i]] != counts[classes[j]] {
			return counts[classes[i]] > counts[classes[j]]
		}
		return classes[i] < classes[j]
	})
	if len(classes) > 3 {
		classes = classes[:3]
	}
	fmt.Fprintln(w, "  top mismatch classes:")
	if len(classes) == 0 {
		fmt.Fprintln(w, "    none blocking target")
	}
	for _, c := range classes {
		fmt.Fprintf(w, "    %s: %d\n", c, counts[c])
	}
}

func printLoopNotes(w io.Writer, s string) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, l := range lines {
		if i == 20 {
			fmt.Fprintf(w, "  ... %d more stage notes omitted\n", len(lines)-i)
			break
		}
		if l != "" {
			fmt.Fprintln(w, " ", firstLine(l, 240))
		}
	}
}

// loopOracle is an operator-provided, offline invocation for exactly one
// queue oracle. Args substitute {subject} and {out}; no shell is involved.
type loopOracle struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Format  string   `json:"format"`
	Defs    bool     `json:"defs,omitempty"`
}

func measureLanguageLoop(root string, e langbench.QueueEntry, override, work, configPath string, w io.Writer) (string, error) {
	runners := map[string]loopOracle{}
	if configPath != "" {
		b, err := os.ReadFile(configPath)
		if err != nil {
			return "", err
		}
		d := json.NewDecoder(bytes.NewReader(b))
		d.DisallowUnknownFields()
		if err = d.Decode(&runners); err != nil {
			return "", err
		}
		names := make([]string, 0, len(runners))
		for name := range runners {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			r := runners[name]
			if !slices.Contains(e.Oracle, name) {
				return "", fmt.Errorf("runner %q is not a queue oracle", name)
			}
			if r.Format != "scip" && r.Format != "tool" {
				return "", fmt.Errorf("runner %q: format must be scip or tool", name)
			}
		}
	}
	// Probe every fixture, without flooding stdout with trees.
	if e.Engine == "tags" || e.Engine == "tags+doc" {
		n := 0
		dir := filepath.Join(root, "internal/extractor/dialects", e.ID, "testdata")
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || strings.HasSuffix(path, ".golden.json") {
				return nil
			}
			var errb bytes.Buffer
			if code := cmdLangProbe([]string{"-dialect", e.ID, path}, io.Discard, &errb); code != 0 {
				return fmt.Errorf("probe %s: %s", path, strings.TrimSpace(errb.String()))
			}
			n++
			return nil
		})
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "", fmt.Errorf("no fixtures for %s", e.ID)
		}
		fmt.Fprintf(w, "probe: %d fixtures\n", n)
	} else {
		fmt.Fprintln(w, "probe: skipped tree probe for", e.Engine, "engine (recipe step 14)")
	}
	subjects, err := loopSubjects(root, e, override)
	if err != nil {
		return "", err
	}
	out := filepath.Join(work, "reading.json")
	for i, s := range subjects {
		if st, err := os.Stat(s.path); err != nil || !st.IsDir() {
			return "", fmt.Errorf("calibration subject %q unavailable locally: %s (no clone or fetch attempted)", s.name, s.path)
		}
		args := []string{"-dialect", e.ID, "-subject", s.path, "-subject-name", s.name, "-queue", filepath.Join(root, "docs/plans/language-breadth.queue.json"), "-out", out, "-unmatched", filepath.Join(work, fmt.Sprintf("unmatched-%d.json", i))}
		ctags := ""
		if slices.Contains(e.Oracle, "ctags") {
			ctags = filepath.Join(root, "workspace/tools/ctags")
			if _, err := os.Stat(ctags); err != nil {
				ctags, err = exec.LookPath("ctags")
				if err != nil {
					ctags = ""
				}
			}
		}
		if ctags == "" {
			ctags = filepath.Join(work, "ctags-unavailable")
		}
		args = append(args, "-ctags", ctags)
		for j, name := range e.Oracle {
			if name == "ctags" {
				if strings.HasSuffix(ctags, "ctags-unavailable") {
					fmt.Fprintf(w, "skip %s / ctags: tool not installed\n", s.name)
				}
				continue
			}
			runner, ok := runners[name]
			if !ok {
				fmt.Fprintf(w, "skip %s / %s: no offline runner configured (-oracle-config; recipe step 19)\n", s.name, name)
				continue
			}
			command, err := exec.LookPath(runner.Command)
			if err != nil {
				fmt.Fprintf(w, "skip %s / %s: tool %s not installed\n", s.name, name, runner.Command)
				continue
			}
			oracleOut := filepath.Join(work, fmt.Sprintf("oracle-%d-%d.json", i, j))
			argv := make([]string, len(runner.Args))
			for k, a := range runner.Args {
				argv[k] = strings.NewReplacer("{subject}", s.path, "{out}", oracleOut).Replace(a)
			}
			ctx, cancel := context.WithTimeout(context.Background(), oracleTimeout)
			cmd := exec.CommandContext(ctx, command, argv...)
			log, err := os.Create(filepath.Join(work, fmt.Sprintf("oracle-%d-%d.log", i, j)))
			if err != nil {
				cancel()
				return "", err
			}
			cmd.Stdout = log
			cmd.Stderr = log
			err = cmd.Run()
			log.Close()
			cancel()
			if err != nil {
				return "", fmt.Errorf("oracle %s on %s: %w", name, s.name, err)
			}
			if _, err = os.Stat(oracleOut); err != nil {
				return "", fmt.Errorf("oracle %s produced no output: %w", name, err)
			}
			if runner.Format == "scip" {
				args = append(args, "-scip", oracleOut)
				if runner.Defs {
					args = append(args, "-scip-defs")
				}
			} else {
				args = append(args, "-tool-oracle", oracleOut)
			}
			fmt.Fprintf(w, "oracle: %s / %s\n", s.name, name)
		}
		var errb bytes.Buffer
		if code := cmdLangOracle(args, io.Discard, &errb); code != 0 {
			return "", fmt.Errorf("measure %s: %s", s.name, strings.TrimSpace(errb.String()))
		}
		fmt.Fprintf(w, "measured: %s\n", s.name)
	}
	return out, nil
}

type loopSubject struct{ name, path string }

func loopSubjects(root string, e langbench.QueueEntry, override string) ([]loopSubject, error) {
	if override != "" {
		return []loopSubject{{filepath.Base(filepath.Clean(override)), override}}, nil
	}
	var shard struct {
		Repos []struct {
			Name    string `yaml:"name"`
			Subpath string `yaml:"subpath"`
		} `yaml:"repos"`
	}
	b, err := os.ReadFile(filepath.Join(root, "calibration/corpus.d", e.ID+".yaml"))
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(b, &shard); err != nil {
		return nil, err
	}
	out := []loopSubject{}
	for _, s := range shard.Repos {
		base := strings.ToLower(filepath.Base(s.Name))
		base, _, _ = strings.Cut(base, " (")
		base = strings.Map(func(r rune) rune {
			if r == ' ' {
				return '-'
			}
			if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune("._-", r) {
				return r
			}
			return -1
		}, base)
		out = append(out, loopSubject{s.Name, filepath.Join(root, "workspace/calibration", e.ID, base, s.Subpath)})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no calibration subjects in shard for %s", e.ID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}
