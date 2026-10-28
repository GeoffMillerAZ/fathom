package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/benchprep"
)

func init() { offlineCommands["split"] = cmdSplit }

// cmdSplit computes the dev, held-out and post-cutoff task splits of a scenario
// corpus (BX-54, benchprep.SplitHeldOut) from the subject manifests and the
// held-out record's dev declarations. It computes and reports; it never writes
// the record (the "Held-out split sha256:" lines are BX-11, the controller's)
// and starts no daemon, clone or network call. The only file it writes is the
// SplitResult JSON at -out, which is scratch: nothing reads it back.
//
// -manifests, -record and -out resolve against -root when relative (an
// absolute value is used as is); -corpus is a user-given path and resolves
// against the cwd. Without -root the root is the git top level of the cwd,
// read with `git rev-parse --show-toplevel`.
//
// Exit codes: 0 on success (shortfalls, unassigned and undated tasks are
// reported, not failures), 2 for a usage error, an unreadable or absent
// manifest directory or record, a drifted dev line, or any SplitHeldOut
// refusal.
func cmdSplit(args []string) int {
	flags := newFlagSet("split")
	corpusPath := flags.String("corpus", "", "path to the scenario corpus JSON (required)")
	manifests := flags.String("manifests", filepath.Join("calibration", "bench.d"), "directory of subject manifests (relative to -root)")
	recordPath := flags.String("record", filepath.Join("docs", "bench", "heldout-reads.md"), "held-out record holding the dev declarations (relative to -root)")
	rootFlag := flags.String("root", "", "repository root (default: the git top level of the cwd)")
	outFlag := flags.String("out", "", "where to write the SplitResult JSON (default: workspace/bench/splits/<corpus file stem>.json; relative to -root)")
	asJSON := flags.Bool("json", false, "print the SplitResult as indented JSON instead of the text lines")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintf(os.Stderr, "scenariobench split: %v\n", err)
		return 2
	}
	if flags.NArg() > 0 {
		return fail(fmt.Errorf("unexpected argument %q", flags.Arg(0)))
	}
	if *corpusPath == "" {
		return fail(errors.New("-corpus is required"))
	}

	root := *rootFlag
	if root == "" {
		top, err := gitTopLevel()
		if err != nil {
			return fail(fmt.Errorf("no -root and the git top level of the cwd cannot be read: %v", err))
		}
		root = top
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fail(err)
	}
	underRoot := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(root, p)
	}
	manifestsDir := underRoot(*manifests)
	record := underRoot(*recordPath)
	outPath := *outFlag
	if outPath == "" {
		stem := strings.TrimSuffix(filepath.Base(*corpusPath), filepath.Ext(*corpusPath))
		outPath = filepath.Join("workspace", "bench", "splits", stem+".json")
	}
	outPath = underRoot(outPath)
	if samePath(outPath, record) {
		return fail(fmt.Errorf("-out %s is the held-out record; split never writes the record (BX-11 is the controller's)", outPath))
	}

	corpus, _, err := loadCorpus(*corpusPath)
	if err != nil {
		return fail(err)
	}
	subjects, err := benchprep.LoadSubjectManifests(manifestsDir)
	if err != nil {
		return fail(fmt.Errorf("manifests %s: %w", manifestsDir, err))
	}
	heldOut, err := benchprep.ParseHeldOutRecord(root, record)
	if err != nil {
		if errors.Is(err, benchprep.ErrNoHeldOutRecord) {
			return fail(fmt.Errorf("the held-out record is absent at %s; the dev declarations live there, so an absent record is not an empty one: %w", record, err))
		}
		return fail(err)
	}
	assignments, err := benchprep.AssignmentsFromSubjects(subjects)
	if err != nil {
		return fail(err)
	}
	result, err := benchprep.SplitHeldOut(corpus.Tasks, assignments, heldOut.Dev)
	if err != nil {
		return fail(err)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fail(err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fail(err)
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fail(err)
	}

	if *asJSON {
		fmt.Print(string(data))
		return 0
	}
	for _, set := range result.Splits {
		sha := set.SHA256
		if sha == "" {
			sha = "none"
		}
		line := fmt.Sprintf("%s sha256: %s (%d tasks)", set.Split, sha, len(set.IDs))
		if set.Split == "post-cutoff" && result.PostCutoffPending {
			line += ": pending Q2"
		}
		fmt.Println(line)
	}
	unassigned := make([]string, 0, len(result.Unassigned))
	for _, task := range result.Unassigned {
		unassigned = append(unassigned, task.ID+" "+task.Reason)
	}
	fmt.Println(countLine("unassigned", len(result.Unassigned), unassigned))
	fmt.Println(countLine("undated", len(result.Undated), result.Undated))
	fmt.Println(countLine("dropped", len(result.Dropped), result.Dropped))
	for _, shortfall := range result.Shortfalls {
		fmt.Printf("shortfall: %s\n", shortfall)
	}
	fmt.Printf("wrote %s\n", displayPath(root, outPath))
	return 0
}

// samePath reports whether a and b name one file: the same cleaned path, or
// (both existing) the same file through a symlink or another spelling.
func samePath(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ai, aerr := os.Stat(a)
	bi, berr := os.Stat(b)
	return aerr == nil && berr == nil && os.SameFile(ai, bi)
}

// countLine renders "<label>: <n> tasks", followed by " (a, b)" when n > 0.
func countLine(label string, n int, items []string) string {
	line := fmt.Sprintf("%s: %d tasks", label, n)
	if n > 0 {
		line += " (" + strings.Join(items, ", ") + ")"
	}
	return line
}

// displayPath is p relative to root, slash-separated, when p is under root;
// otherwise p unchanged.
func displayPath(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return p
	}
	return filepath.ToSlash(rel)
}

// gitTopLevel is `git rev-parse --show-toplevel` in the cwd: read-only.
func gitTopLevel() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	top := strings.TrimSpace(string(out))
	if top == "" {
		return "", errors.New("git printed no top level")
	}
	return top, nil
}
