package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps/boundaries"
)

const boundariesUsage = `usage:
  fathom-lang boundaries --rules FILE [--root DIR] [--strict]
                         [--trust-unmeasured ID] [PATH...]

Evaluate direct imports. PATHs are repository-relative slash paths. With no
PATHs, discover governed files below --root (default the working directory).
Print JSON to stdout and one line per finding to stderr. Exit 1 for violations
or failed reads, and with --strict also for unverified rules. Misuse exits 2.
`

var errBoundaryFindings = errors.New("boundary findings requested a nonzero exit")

func cmdBoundaries(ctx context.Context, args []string, out, errw io.Writer) error {
	flags := flag.NewFlagSet("boundaries", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	rulesFile := flags.String("rules", "", "boundary rules JSON document")
	root := flags.String("root", ".", "repository root")
	strict := flags.Bool("strict", false, "fail on unverified rules")
	trust := flags.String("trust-unmeasured", "", "unmeasured dialect ID to trust")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, err := fmt.Fprint(out, boundariesUsage); err != nil {
				return errors.New("cannot write boundary usage")
			}
			return nil
		}
		return usageError{errors.New("invalid boundaries flags; see fathom-lang boundaries -h")}
	}
	if *rulesFile == "" {
		return usageError{errors.New("boundaries requires --rules")}
	}
	rules, err := readBoundaryRules(*rulesFile)
	if err != nil {
		return err
	}
	var opts []deps.Option
	if *trust != "" {
		opts = append(opts, deps.TrustUnmeasured(*trust))
	}
	reader, err := deps.New(opts...)
	if err != nil {
		return usageError{err}
	}
	repo, err := os.OpenRoot(*root)
	if err != nil {
		return errors.New("cannot open boundary root")
	}
	defer repo.Close()

	names := flags.Args()
	if len(names) == 0 {
		names, err = discoverBoundaryFiles(ctx, repo.FS(), rules)
		if err != nil {
			return err
		}
	}
	var files []deps.FileDeps
	var readErr error
	if len(names) != 0 {
		inputs := make([]deps.Input, 0, len(names))
		for _, name := range names {
			inputs = append(inputs, deps.Input{Path: name})
		}
		files, readErr = reader.ReadFiles(ctx, repo.FS(), inputs)
		if errors.Is(readErr, deps.ErrInput) {
			return usageError{readErr}
		}
		// On cancellation ReadFiles returns a sorted prefix. Account for the
		// unread suffix so completed files cannot make that rule look clean.
		if len(files) < len(names) {
			ordered := slices.Clone(names)
			slices.Sort(ordered)
			for _, name := range ordered[len(files):] {
				files = append(files, deps.FileDeps{Path: name})
			}
		}
	}
	results := boundaries.EvaluateBoundaries(rules, files)
	doc := struct {
		SchemaVersion int                 `json:"schema_version"`
		Rules         []boundaries.Result `json:"rules"`
	}{schemaVersion, results}
	if err := writeJSON(out, doc); err != nil {
		return errors.New("cannot write boundary JSON")
	}
	failed := false
	for _, result := range results {
		if err := writeBoundaryFindings(errw, result); err != nil {
			return errors.New("cannot write boundary findings")
		}
		failed = failed || result.Verdict == boundaries.VerdictViolated ||
			(*strict && result.Verdict == boundaries.VerdictUnverified)
	}
	if readErr != nil {
		return readErr
	}
	if failed {
		return errBoundaryFindings
	}
	return nil
}

func readBoundaryRules(file string) (boundaries.Rules, error) {
	src, err := os.Open(file)
	if err != nil {
		return boundaries.Rules{}, errors.New("cannot open boundary rules")
	}
	defer src.Close()
	rules, err := boundaries.ParseRules(src)
	if errors.Is(err, boundaries.ErrRules) {
		return boundaries.Rules{}, usageError{err}
	}
	return rules, err
}

func discoverBoundaryFiles(ctx context.Context, repo fs.FS, rules boundaries.Rules) ([]string, error) {
	var names []string
	err := fs.WalkDir(repo, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return errors.New("cannot walk boundary root")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == ".git" && entry.IsDir() {
			return fs.SkipDir
		}
		if name != ".git" && !entry.IsDir() && rules.Governs(name) {
			names = append(names, name)
		}
		return nil
	})
	return names, err
}

func writeBoundaryFindings(out io.Writer, result boundaries.Result) error {
	for _, edge := range result.Violations {
		if _, err := fmt.Fprintf(out, "%s: violated %s:%d -> %s\n", result.ID, edge.From, edge.Line, edge.To); err != nil {
			return err
		}
	}
	for _, file := range result.Unverified {
		if _, err := fmt.Fprintf(out, "%s: unverified %s\n", result.ID, file.Path); err != nil {
			return err
		}
	}
	if result.MatchedFiles == 0 {
		_, err := fmt.Fprintf(out, "%s: unverified (no governed files in scope)\n", result.ID)
		return err
	}
	return nil
}
