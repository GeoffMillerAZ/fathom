package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

const depsUsage = `usage:
  fathom-lang deps [-root DIR] [-dialect ID] [-trust-unmeasured ID]
                   [-export] PATH...

Read direct dependencies from repository-relative slash paths. Output is a
JSON document with per-file edges and unknowns. -export prints only the
array of declared unit pairs. The default root is the working directory.
`

func cmdDeps(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("deps", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "repository root")
	dialect := flags.String("dialect", "", "dependency dialect ID")
	trust := flags.String("trust-unmeasured", "", "unmeasured dialect ID to trust")
	export := flags.Bool("export", false, "write a declared-edge list")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, err := fmt.Fprint(out, depsUsage); err != nil {
				return errors.New("cannot write dependency usage")
			}
			return nil
		}
		return usageError{errors.New("invalid deps flags; see fathom-lang deps -h")}
	}
	if flags.NArg() == 0 {
		return usageError{errors.New("deps needs at least one PATH")}
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
		return errors.New("cannot open dependency root")
	}
	defer repo.Close()
	inputs := make([]deps.Input, 0, flags.NArg())
	for _, name := range flags.Args() {
		inputs = append(inputs, deps.Input{Path: name, Dialect: *dialect})
	}
	files, err := reader.ReadFiles(ctx, repo.FS(), inputs)
	if errors.Is(err, deps.ErrInput) {
		return usageError{err}
	}
	if *export && err != nil {
		return err
	}
	var document any = struct {
		SchemaVersion int             `json:"schema_version"`
		Files         []deps.FileDeps `json:"files"`
	}{schemaVersion, files}
	if *export {
		document = declaredPairs(files)
	}
	if writeJSON(out, document) != nil {
		return errors.New("cannot write dependency JSON")
	}
	return err
}

type dependencyUnit struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
}

type dependencyPair struct {
	From dependencyUnit `json:"from"`
	To   dependencyUnit `json:"to"`
}

func declaredPairs(files []deps.FileDeps) []dependencyPair {
	pairs := []dependencyPair{}
	for _, f := range files {
		for _, e := range f.Edges {
			to := dependencyUnit{"repo", e.To}
			switch e.Resolution {
			case deps.ResolvedFile, deps.ResolvedDirectory:
			case deps.ResolvedExternal:
				to.Ecosystem, to.Name, _ = strings.Cut(e.To, ":")
			default:
				continue
			}
			pairs = append(pairs, dependencyPair{dependencyUnit{"repo", e.From}, to})
		}
	}
	slices.SortFunc(pairs, func(a, b dependencyPair) int {
		return cmp.Or(cmp.Compare(a.From.Ecosystem, b.From.Ecosystem), cmp.Compare(a.From.Name, b.From.Name),
			cmp.Compare(a.To.Ecosystem, b.To.Ecosystem), cmp.Compare(a.To.Name, b.To.Name))
	})
	return slices.Compact(pairs)
}
