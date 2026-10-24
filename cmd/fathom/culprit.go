package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/landing"
	"github.com/geoffmilleraz/fathom/internal/store"
)

type culpritSource func(context.Context, string, string) (landing.Evidence, error)

func cmdCulprit(args []string, stdout, stderr io.Writer) int {
	return runCulprit(args, "", loadCulpritEvidence, stdout, stderr)
}

// loadCulpritEvidence never creates or migrates a store, starts a daemon,
// refreshes an index, or reconstructs historical trees.
func loadCulpritEvidence(ctx context.Context, service, path string) (landing.Evidence, error) {
	st, err := store.MountReadOnly(path, store.LevelL0)
	if err != nil {
		return landing.Evidence{}, err
	}
	defer st.Close()
	nodes, err := st.ListSymbolNodes(ctx, service)
	if err != nil {
		return landing.Evidence{}, err
	}
	typed, err := st.TypedEdges(ctx, service)
	if err != nil {
		return landing.Evidence{}, err
	}
	meta, _, err := st.GetServiceMeta(ctx, service)
	if err != nil {
		return landing.Evidence{}, err
	}
	return landing.Evidence{Nodes: nodes, Typed: typed, IndexedCommit: meta.IndexedCommit}, nil
}

func runCulprit(args []string, root string, source culpritSource, stdout, stderr io.Writer) int {
	fs := newFlagSet("culprit", stderr)
	failing := fs.String("failing", "", "comma-separated failing test names or canonical symbol keys")
	service := fs.String("service", "", "service ID (default: resolved from the current repository)")
	storePath := fs.String("store", "", "existing index store to read (.sqlite/.db); defaults to [daemon].store")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: fathom culprit <good>..<bad> [--failing TestA,TestB] [--service S] [--store DB]")
		fs.PrintDefaults()
	}
	// The documented range-first form also accepts flags after the range.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string(nil), args[1:]...), args[0])
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	if *storePath == "" {
		*storePath = resolveStorePath()
	}
	goodRef, badRef, ok := strings.Cut(fs.Arg(0), "..")
	if !ok || goodRef == "" || badRef == "" || strings.Contains(badRef, "..") || strings.HasPrefix(goodRef, "-") || strings.HasPrefix(badRef, "-") || strings.HasPrefix(badRef, ".") {
		fmt.Fprintln(stderr, "error: culprit requires a two-dot <good>..<bad> commit range")
		return 2
	}
	if root == "" {
		root = verifyRepoRoot()
	}
	git := func(args ...string) ([]byte, error) {
		return gitro.Output(context.Background(), root, args...)
	}
	resolve := func(ref string) (string, error) {
		out, err := git("rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
		return strings.TrimSpace(string(out)), err
	}
	good, err := resolve(goodRef)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve good commit: %v\n", err)
		return 2
	}
	bad, err := resolve(badRef)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve bad commit: %v\n", err)
		return 2
	}
	if _, err := git("merge-base", "--is-ancestor", good, bad); err != nil {
		fmt.Fprintln(stderr, "error: good commit must be an ancestor of bad")
		return 2
	}
	commits, err := culpritCommits(git, good, bad)
	if err != nil {
		fmt.Fprintf(stderr, "error: read culprit range: %v\n", err)
		return 2
	}
	if len(commits) == 0 {
		fmt.Fprintln(stdout, "No commits in the range.")
		return 0
	}
	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
	}
	evidence, err := source(context.Background(), svc, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "note: no reach evidence: %v\n", err)
		evidence = landing.Evidence{}
	}
	var tests []string
	for _, name := range strings.Split(*failing, ",") {
		if name = strings.TrimSpace(name); name != "" {
			tests = append(tests, name)
		}
	}
	ranked := landing.RankCulprits(commits, tests, evidence)
	fmt.Fprintln(stdout, "Culprit candidates from the current index (file-level changes), not a causal verdict.")
	indexed := evidence.IndexedCommit
	if indexed == "" {
		indexed = "unknown"
	}
	fmt.Fprintf(stdout, "Index: store %q; service %q; indexed commit %s (%s).\n", *storePath, svc, indexed, culpritIndexRelation(git, evidence.IndexedCommit, bad))
	if len(evidence.Nodes) == 0 {
		fmt.Fprintf(stdout, "No index evidence exists for service %q.\n", svc)
	}
	if _, unresolved := landing.ResolveFailingTests(tests, evidence.Nodes); len(unresolved) > 0 {
		fmt.Fprintln(stdout, "Unresolved failing tests: "+strings.Join(unresolved, ", "))
	}
	fmt.Fprintln(stdout, "Order: failing-test reach, missing evidence before indexed non-reaching commits, descending blast radius (>=N when incomplete), then recency in the range.")
	for i, c := range ranked {
		label := ""
		if c.Commit.Merge {
			label = " [merge]"
		}
		fmt.Fprintf(stdout, "%d. %s%s %s\n   %s\n", i+1, c.Commit.Hash, label, c.Commit.Subject, c.Reason)
	}
	if ranked[0].Commit.Hash == bad {
		fmt.Fprintln(stdout, "Top candidate is bad itself; its failure is already known. Test its parent to confirm the boundary.")
	}
	fmt.Fprintln(stdout, "\nTo try the top candidate first, run these yourself (this command ran no build, test or bisect):")
	fmt.Fprintf(stdout, "git bisect start %s %s\ngit checkout %s\n", bad, good, ranked[0].Commit.Hash)
	fmt.Fprintln(stdout, "# Run the failing tests yourself, then choose ONE result:")
	fmt.Fprintln(stdout, "git bisect good")
	fmt.Fprintln(stdout, "# OR, if the tests fail:")
	fmt.Fprintln(stdout, "git bisect bad")
	fmt.Fprintln(stdout, "# Confirm the candidate by testing its parent:")
	fmt.Fprintf(stdout, "git checkout %s^\n", ranked[0].Commit.Hash)
	fmt.Fprintln(stdout, "# Run the same failing tests. A passing parent supports this boundary; otherwise keep bisecting.")
	return 0
}

func culpritIndexRelation(git func(...string) ([]byte, error), indexed, bad string) string {
	if indexed == "" {
		return "relation unknown (no indexed commit recorded)"
	}
	out, err := git("rev-parse", "--verify", "--end-of-options", indexed+"^{commit}")
	if err != nil {
		return "relation unknown (indexed commit unavailable locally)"
	}
	hash := strings.TrimSpace(string(out))
	if hash == bad {
		return "same as bad"
	}
	if _, err := git("merge-base", "--is-ancestor", hash, bad); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return "not an ancestor of bad"
		}
		return "relation unknown"
	}
	out, err = git("rev-list", "--count", hash+".."+bad, "--")
	if err != nil {
		return "relation unknown"
	}
	return "behind bad by " + strings.TrimSpace(string(out))
}

// culpritCommits uses NUL-delimited file lists so spaces and git quoting cannot
// turn a path into missing evidence. Combined merge diffs (-c) count only the
// merge's own resolution, rather than charging it for its parents' changes.
func culpritCommits(git func(...string) ([]byte, error), good, bad string) ([]landing.Commit, error) {
	out, err := git("rev-list", "--reverse", "--topo-order", good+".."+bad, "--")
	if err != nil {
		return nil, err
	}
	var commits []landing.Commit
	for _, hash := range strings.Fields(string(out)) {
		detail, err := git("show", "-s", "--no-show-signature", "--format=%P%x00%s", hash, "--")
		if err != nil {
			return nil, err
		}
		parents, subject, _ := strings.Cut(string(detail), "\x00")
		files, err := git("diff-tree", "--root", "--no-commit-id", "--name-only", "--no-renames", "-r", "-c", "-z", hash, "--")
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool)
		var paths []string
		for _, file := range strings.Split(string(files), "\x00") {
			if file != "" && !seen[file] {
				seen[file] = true
				paths = append(paths, file)
			}
		}
		sort.Strings(paths)
		commits = append(commits, landing.Commit{Hash: hash, Subject: strings.TrimSpace(subject), Files: paths, Merge: len(strings.Fields(parents)) > 1})
	}
	return commits, nil
}

type culpritRunner struct{}

func (culpritRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdCulprit(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "culprit", Group: "review", Order: 120, Run: culpritRunner{},
		Summary: "Rank a red commit range by failing-test reach and blast radius; print bisect commands"})
}
