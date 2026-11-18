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

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

const pathFiltersUsage = `usage:
  fathom-lang pathfilters --map FILE [--root DIR] [--trust-unmeasured ID] [WORKFLOW...]

Check GitHub Actions push and pull request path filters against every known
local dependency of the declared entry files. FILE and optional WORKFLOW paths
are repository-relative. Without WORKFLOW, check all declarations in FILE.
Print versioned JSON. Exit 0 when covered, 1 on misses, unknowns or failed
reads, 2 on misuse or an invalid map. Unused patterns are advisory.
`

var errPathFilterFindings = errors.New("path filter findings requested a nonzero exit")

type pathFilterReport struct {
	SchemaVersion  int                       `json:"schema_version"`
	Results        []pathFilterResult        `json:"results"`
	UnusedPatterns []unusedPathFilterPattern `json:"unused_patterns"`
}

type pathFilterResult struct {
	Workflow string              `json:"workflow"`
	Job      string              `json:"job,omitempty"`
	Event    string              `json:"event,omitempty"`
	Entry    string              `json:"entry,omitempty"`
	Verdict  string              `json:"verdict"`
	Paths    []string            `json:"paths"`
	Missed   []string            `json:"missed"`
	Unknowns []pathFilterUnknown `json:"unknowns"`
	Trusted  []string            `json:"trusted,omitempty"`
}

type pathFilterUnknown struct {
	Path        string             `json:"path"`
	Code        string             `json:"code"`
	Detail      string             `json:"detail,omitempty"`
	Target      string             `json:"target,omitempty"`
	Line        int                `json:"line,omitempty"`
	Raw         string             `json:"raw,omitempty"`
	DecidableBy []deps.DecidableBy `json:"decidable_by,omitempty"`
}

type unusedPathFilterPattern struct {
	Workflow string `json:"workflow"`
	Event    string `json:"event"`
	Pattern  string `json:"pattern"`
	Position int    `json:"position"`
}

func cmdPathFilters(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("pathfilters", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "repository root")
	mapFile := flags.String("map", "", "repository-relative coverage map JSON")
	trust := flags.String("trust-unmeasured", "", "unmeasured dialect ID to trust")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := fmt.Fprint(out, pathFiltersUsage)
			return err
		}
		return usageError{errors.New("invalid pathfilters flags; see fathom-lang pathfilters -h")}
	}
	if !validPathFilterPath(*mapFile) {
		return usageError{errors.New("pathfilters requires a repository-relative --map file")}
	}
	for _, workflow := range flags.Args() {
		if !validPathFilterWorkflow(workflow) {
			return usageError{errors.New("expected a workflow path under .github/workflows")}
		}
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
		return errors.New("cannot open pathfilters root")
	}
	defer repo.Close()

	const maxMapBytes = 1 << 20
	src, err := readPathFilterFile(repo, *mapFile, maxMapBytes)
	if errors.Is(err, errPathFilterSize) {
		return usageError{errors.New("coverage map exceeds 1 MiB")}
	}
	if err != nil {
		return errors.New("cannot read coverage map")
	}
	scopes, err := parsePathFilterMap(src)
	if err != nil {
		return usageError{err}
	}
	report, err := checkPathFilterCoverage(ctx, repo, reader, selectPathFilterScopes(scopes, flags.Args()))
	if err != nil {
		return err
	}
	if err := writeJSON(out, report); err != nil {
		return errors.New("cannot write pathfilters JSON")
	}
	for _, result := range report.Results {
		if result.Verdict != "covered" {
			return errPathFilterFindings
		}
	}
	return nil
}

func checkPathFilterCoverage(ctx context.Context, repo *os.Root, reader *deps.Reader, scopes []pathFilterScope) (pathFilterReport, error) {
	view := pathFilterFS{repo}
	inventory, inventoryErr := inventoryPathFilterFiles(ctx, view)
	graph, err := readPathFilterDependencies(ctx, view, reader, inventory)
	if err != nil {
		return pathFilterReport{}, err
	}
	report := pathFilterReport{SchemaVersion: schemaVersion, Results: []pathFilterResult{}, UnusedPatterns: []unusedPathFilterPattern{}}
	for i := 0; i < len(scopes); {
		end := i + 1
		for end < len(scopes) && scopes[end].Workflow == scopes[i].Workflow {
			end++
		}
		evaluatePathFilterWorkflow(repo, scopes[i:end], inventory, inventoryErr, graph, &report)
		i = end
	}
	// A cancellation during dependency analysis or workflow checks invalidates
	// any coverage claim, even if the inventory itself completed first.
	if ctx.Err() != nil {
		report.UnusedPatterns = []unusedPathFilterPattern{}
		for i := range report.Results {
			report.Results[i].addUnknown(pathFilterUnknown{Code: "canceled"})
			report.Results[i].finish()
		}
	}
	return report, nil
}

func evaluatePathFilterWorkflow(repo *os.Root, scopes []pathFilterScope, inventory []string, inventoryErr error, graph *pathFilterGraph, report *pathFilterReport) {
	workflow := scopes[0].Workflow
	var filters extractor.GitHubPathFilters
	var workflowUnknown pathFilterUnknown
	switch {
	case len(scopes[0].Entries) == 0:
		workflowUnknown = pathFilterUnknown{Path: workflow, Code: "unmapped-workflow"}
	case !pathFilterInventoryHas(inventory, workflow):
		workflowUnknown = pathFilterUnknown{Path: workflow, Code: "workflow-not-in-inventory"}
	default:
		src, err := readPathFilterFile(repo, workflow, 2<<20)
		if err != nil {
			workflowUnknown = pathFilterUnknown{Path: workflow, Code: "unreadable-workflow"}
		} else if filters, err = extractor.ReadGitHubPathFilters(src); err != nil {
			workflowUnknown = pathFilterUnknown{Path: workflow, Code: "invalid-workflow", Detail: err.Error()}
		} else if len(filters.Events) == 0 {
			workflowUnknown = pathFilterUnknown{Path: workflow, Code: "unsupported-events"}
		}
	}
	if inventoryErr == nil {
		report.UnusedPatterns = append(report.UnusedPatterns, unusedPathFilterPatterns(workflow, filters.Events, inventory)...)
	}

	for _, scope := range scopes {
		scopeUnknowns := []pathFilterUnknown{}
		if inventoryErr != nil {
			scopeUnknowns = append(scopeUnknowns, pathFilterUnknown{Code: "incomplete-inventory"})
		}
		if workflowUnknown.Code != "" {
			scopeUnknowns = append(scopeUnknowns, workflowUnknown)
		} else {
			scopeUnknowns = append(scopeUnknowns, pathFilterJobUnknowns(scope, filters.Jobs)...)
		}
		entries := scope.Entries
		if len(entries) == 0 {
			entries = []string{""}
		}
		events := filters.Events
		if len(events) == 0 {
			events = []extractor.GitHubPathEvent{{}}
		}
		for _, entry := range entries {
			paths, trusted, unknowns := graph.closure(entry)
			for _, event := range events {
				result := pathFilterResult{
					Workflow: workflow, Job: scope.Job, Event: event.Name, Entry: entry,
					Paths: paths, Trusted: trusted, Missed: []string{},
					Unknowns: append(slices.Clone(scopeUnknowns), unknowns...),
				}
				if event.Condition != "" {
					result.addUnknown(pathFilterUnknown{Path: workflow, Code: "event-condition", Detail: event.Condition})
				}
				for _, name := range paths {
					if event.Name != "" && pathFilterInventoryHas(inventory, name) && !event.Includes(name) {
						result.Missed = append(result.Missed, name)
					}
				}
				result.finish()
				report.Results = append(report.Results, result)
			}
		}
	}
}

func pathFilterJobUnknowns(scope pathFilterScope, jobs map[string]string) []pathFilterUnknown {
	if scope.Job != "" {
		condition, exists := jobs[scope.Job]
		switch {
		case !exists:
			return []pathFilterUnknown{{Path: scope.Workflow, Code: "unmapped-job", Detail: scope.Job}}
		case condition != "":
			return []pathFilterUnknown{{Path: scope.Workflow, Code: "job-condition", Detail: scope.Job + ": " + condition}}
		}
		return nil
	}
	if len(jobs) == 0 {
		return []pathFilterUnknown{{Path: scope.Workflow, Code: "missing-jobs"}}
	}
	var unknowns []pathFilterUnknown
	for job, condition := range jobs {
		if condition != "" {
			unknowns = append(unknowns, pathFilterUnknown{Path: scope.Workflow, Code: "job-condition", Detail: job + ": " + condition})
		}
	}
	return unknowns
}

func (r *pathFilterResult) addUnknown(unknown pathFilterUnknown) {
	r.Unknowns = append(r.Unknowns, unknown)
}

func (r *pathFilterResult) finish() {
	slices.SortFunc(r.Unknowns, func(a, b pathFilterUnknown) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Code, b.Code),
			cmp.Compare(a.Detail, b.Detail), cmp.Compare(a.Target, b.Target), cmp.Compare(a.Raw, b.Raw),
			slices.Compare(a.DecidableBy, b.DecidableBy))
	})
	if r.Unknowns == nil {
		r.Unknowns = []pathFilterUnknown{}
	}
	switch {
	case len(r.Missed) > 0:
		r.Verdict = "missed"
	case len(r.Unknowns) > 0:
		r.Verdict = "unverified"
	default:
		r.Verdict = "covered"
	}
}

func unusedPathFilterPatterns(workflow string, events []extractor.GitHubPathEvent, inventory []string) []unusedPathFilterPattern {
	var unused []unusedPathFilterPattern
	for _, event := range events {
		for i, pattern := range event.Patterns {
			if !slices.ContainsFunc(inventory, pattern.Match) {
				unused = append(unused, unusedPathFilterPattern{Workflow: workflow, Event: event.Name, Pattern: pattern.Text, Position: i + 1})
			}
		}
	}
	return unused
}
