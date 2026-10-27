package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/labs"
	"github.com/geoffmilleraz/fathom/internal/readcoverage"
	"github.com/geoffmilleraz/fathom/internal/research/bundle"
	"github.com/geoffmilleraz/fathom/internal/transcript"
	readmanifest "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/read-manifest"
)

const (
	// labsSessionsRel is where the corpus lands, resolved under the
	// repository's gitignored workspace/ by workspacePath (I31): one
	// file per session hash.
	labsSessionsRel = "labs/sessions"
	// labsLedgerPath is the one ledger AppendLabResult writes.
	labsLedgerPath = "docs/bench/labs-ledger.jsonl"
	// labsCardsDir holds the committed lab cards.
	labsCardsDir = "calibration/labs.d"
)

// labsCommit is the revision a written replay row is stamped with; tests
// replace it, since a test binary carries no vcs revision of its own.
var labsCommit = labsFathomCommit

// labsRunner runs `fathom labs` (registered below).
type labsRunner struct{}

func (labsRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdLabs(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "labs", Group: "operate", Order: 76, Run: labsRunner{},
		Summary: "Feature labs, offline: corpus [--projects GLOB] [--out DIR] [--json] cuts this repository's local transcripts into decision points under workspace/labs/sessions/ and prints aggregates | fire [--projects GLOB] [--json] reads the firing funnel per lab surface, and with --write --run-id ID --criterion ID --prereg-sha SHA appends one replay row per surface to docs/bench/labs-ledger.jsonl"})
}

// cmdLabs dispatches the labs subverbs. It reads local files only: no
// daemon, no provider, no spend.
func cmdLabs(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: labs requires a subcommand: corpus, fire")
		return 1
	}
	switch args[0] {
	case "corpus":
		return cmdLabsCorpus(args[1:], stdout, stderr)
	case "fire":
		return cmdLabsFire(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown labs subcommand %q (want corpus, fire)\n", args[0])
		return 1
	}
}

// labsCorpusReading is one corpus walk: the files read, the points cut,
// their classification and every count the decoder could not read (I38).
type labsCorpusReading struct {
	Projects int         `json:"projects"`
	Files    int         `json:"files"`
	Sessions int         `json:"sessions"`
	Tally    labs.Tally  `json:"tally"`
	Counts   labs.Counts `json:"counts"`
	// CorpusSHA is sha256 over the sorted per-file sha256 list, the
	// provenance a replay row carries in place of arm pins.
	CorpusSHA string       `json:"corpus_sha"`
	Points    []labs.Point `json:"-"`
	// Written is how many session files the walk wrote, and Out the
	// directory it wrote them to as the caller named it.
	Written int    `json:"written"`
	Out     string `json:"out,omitempty"`
}

// cmdLabsCorpus is `fathom labs corpus`.
func cmdLabsCorpus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("labs corpus", stderr)
	projects := fs.String("projects", "", "a glob over the transcript project directory names (default: this repository's own project directories)")
	out := fs.String("out", "", "the directory the per-session decision points are written to (default: workspace/labs/sessions)")
	jsonOut := fs.Bool("json", false, "emit the aggregates as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: labs corpus takes no arguments (got %q)\n", fs.Arg(0))
		return 1
	}
	dir := *out
	if dir == "" {
		dir = workspacePath(labsSessionsRel)
	}
	reading, code := readLabsCorpus(*projects, dir, stderr)
	if code != 0 {
		return code
	}
	if *jsonOut {
		return emitLabsJSON(stdout, stderr, reading)
	}
	printLabsCorpus(stdout, reading)
	return 0
}

// cmdLabsFire is `fathom labs fire`.
func cmdLabsFire(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("labs fire", stderr)
	projects := fs.String("projects", "", "a glob over the transcript project directory names (default: this repository's own project directories)")
	cards := fs.String("cards", labsCardsDir, "the directory of committed lab cards")
	ledger := fs.String("ledger", labsLedgerPath, "the results ledger a --write appends to")
	write := fs.Bool("write", false, "append one replay row per (run, surface) to the ledger")
	runID := fs.String("run-id", "", "the run id a written row is keyed by (required with --write)")
	criterion := fs.String("criterion", "", "the one criterion the run was preregistered to decide (required with --write)")
	preregSHA := fs.String("prereg-sha", "", "the sha256 of the committed preregistration (required with --write)")
	jsonOut := fs.Bool("json", false, "emit the readings as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: labs fire takes no arguments (got %q)\n", fs.Arg(0))
		return 1
	}
	if *write {
		// A fixed order, so the error names the same flag every time.
		for _, required := range []struct{ name, value string }{{"--run-id", *runID}, {"--criterion", *criterion}, {"--prereg-sha", *preregSHA}} {
			if strings.TrimSpace(required.value) == "" {
				fmt.Fprintf(stderr, "error: labs fire --write requires %s\n", required.name)
				return 1
			}
		}
	}
	// Relative paths belong to the repository, not to wherever the verb ran.
	if top := gitToplevel("."); top != "" {
		if !filepath.IsAbs(*cards) {
			*cards = filepath.Join(top, *cards)
		}
		if !filepath.IsAbs(*ledger) {
			*ledger = filepath.Join(top, *ledger)
		}
	}
	surfaces, err := shippedLabSurfaces()
	if err != nil {
		fmt.Fprintf(stderr, "error: labs fire: the shipped surfaces: %v\n", err)
		return 2
	}
	cardList, err := labs.LoadLabs(*cards, surfaces)
	if err != nil {
		fmt.Fprintf(stderr, "error: labs fire: reading the cards: %v\n", err)
		return 2
	}
	reading, code := readLabsCorpus(*projects, "", stderr)
	if code != 0 {
		return code
	}
	rows := labs.FiringReading(reading.Points, cardList)
	if *write {
		provenance := labs.ReplayProvenance{CorpusSHA: reading.CorpusSHA, FathomCommit: labsCommit()}
		if provenance.CorpusSHA == "" || provenance.FathomCommit == "" {
			fmt.Fprintln(stderr, "error: labs fire --write: a replay row needs the corpus sha and the fathom commit that read it")
			return 2
		}
		results := make([]any, 0, len(rows))
		for _, row := range rows {
			results = append(results, row.ResultRow(*runID, *criterion, *preregSHA, provenance))
		}
		// All rows or none: a refused row must not leave a partial run behind.
		if err := labs.AppendLabResults(*ledger, results); err != nil {
			fmt.Fprintf(stderr, "error: labs fire --write: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "labs fire: appended %d row(s) to %s under run %s\n", len(rows), *ledger, *runID)
	}
	if *jsonOut {
		return emitLabsJSON(stdout, stderr, struct {
			Corpus   labsCorpusReading       `json:"corpus"`
			Readings []labs.FiringReadingRow `json:"readings"`
		}{reading, rows})
	}
	printLabsFiring(stdout, reading, rows)
	return 0
}

// labsFathomCommit is the revision of the binary that read the corpus, so
// a replay row names what produced it. A dirty tree is marked.
func labsFathomCommit() string {
	stamp := buildinfo.Read()
	if stamp.Revision == "" {
		return ""
	}
	revision := stamp.Revision
	if len(revision) > 12 {
		revision = revision[:12]
	}
	if stamp.Dirty {
		return revision + "-dirty"
	}
	return revision
}

// readLabsCorpus walks this repository's local transcripts in sorted
// order, cuts each into decision points and, when out is set, writes one
// file per session hash under it. Only aggregates and hashes are kept:
// no transcript path, session id or text reaches the reading.
func readLabsCorpus(projects, out string, stderr io.Writer) (labsCorpusReading, int) {
	var reading labsCorpusReading
	root := readoutProjectsDir()
	if root == "" {
		fmt.Fprintln(stderr, "error: labs: no home directory, so no transcripts")
		return reading, 2
	}
	repoRoot := gitToplevel(".")
	if repoRoot == "" {
		fmt.Fprintln(stderr, "error: labs: not inside a git repository, so the corpus has no owner")
		return reading, 2
	}
	// Default: this repository's own project directories, encoded the way
	// Claude Code encodes a path, exactly or as one of its worktrees (owner
	// ruling R5). A linked worktree reads the main worktree's directories.
	// A glob given with --projects replaces the rule.
	match := func(name string) bool { return labsProjectDirMatches(labsEncodeProject(labsMainRoot(repoRoot)), name) }
	if projects != "" {
		if _, err := filepath.Match(projects, ""); err != nil {
			fmt.Fprintf(stderr, "error: labs --projects: %v\n", err)
			return reading, 1
		}
		match = func(name string) bool { ok, _ := filepath.Match(projects, name); return ok }
	}
	salt, err := bundle.LoadOrCreateSalt(repoRoot)
	if err != nil {
		fmt.Fprintf(stderr, "error: labs: the research salt: %v\n", err)
		return reading, 2
	}
	files, projectCount, walkIssues, err := labsTranscriptFiles(root, match)
	if err != nil {
		fmt.Fprintf(stderr, "error: labs: reading the transcripts: %v\n", err)
		return reading, 2
	}
	reading.Projects, reading.Files = projectCount, len(files)
	// A directory the walk could not open hid transcripts: count it (I38).
	reading.Counts.Issues = append(reading.Counts.Issues, walkIssues...)
	reading.Counts.UnreadableFiles += len(walkIssues)
	var digests []string
	for index, path := range files {
		res := transcript.DecodeTranscriptEvents(path, salt, index)
		session := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		points, counts := labs.ExtractDecisionPoints(res, salt, session)
		reading.Counts.Issues = append(reading.Counts.Issues, counts.Issues...)
		reading.Counts.MalformedLines += counts.MalformedLines
		reading.Counts.OversizeLines += counts.OversizeLines
		reading.Counts.UnreadableFiles += counts.UnreadableFiles
		reading.Counts.Duplicates += counts.Duplicates
		reading.Counts.Points += counts.Points
		if len(points) > 0 {
			reading.Sessions++
			reading.Points = append(reading.Points, points...)
			if out != "" {
				if werr := writeLabsSession(out, points); werr != nil {
					fmt.Fprintf(stderr, "error: labs: writing the corpus: %v\n", werr)
					return reading, 2
				}
				reading.Written++
			}
		}
		if digest, derr := labsFileDigest(path); derr == nil {
			digests = append(digests, digest)
		} else if res.UnreadableFiles == 0 {
			// The decoder read it but the digest could not: the corpus sha
			// leaves this file out, so say so.
			reading.Counts.UnreadableFiles++
			reading.Counts.Issues = append(reading.Counts.Issues, transcript.Issue{File: fmt.Sprintf("digest:%d", index), Kind: "unreadable_file"})
		}
	}
	reading.Out = filepath.ToSlash(out)
	reading.Tally = labs.ClassifyPoints(reading.Points)
	reading.CorpusSHA = labsCorpusSHA(digests)
	return reading, 0
}

// labsTranscriptFiles lists every *.jsonl transcript under the project
// directories the glob matches, in sorted path order, skipping a workflow
// run's step journal (which is no transcript).
func labsTranscriptFiles(root string, match func(string) bool) ([]string, int, []transcript.Issue, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, 0, nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && match(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var files []string
	var issues []transcript.Issue
	projects := 0
	for _, name := range names {
		before := len(files)
		err := filepath.WalkDir(filepath.Join(root, name), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				// The label is a class and the walk's order, never a path.
				issues = append(issues, transcript.Issue{File: fmt.Sprintf("dir:%d", len(issues)), Kind: "unreadable_dir"})
				return nil
			}
			if d.IsDir() || !strings.HasSuffix(p, ".jsonl") || replayWorkflowJournal(p) {
				return nil
			}
			files = append(files, p)
			return nil
		})
		if err != nil {
			return nil, 0, nil, err
		}
		if len(files) > before {
			projects++
		}
	}
	sort.Strings(files)
	return files, projects, issues, nil
}

var labsNonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// labsEncodeProject is Claude Code's project directory name for a path:
// every character that is not a letter or digit becomes "-".
func labsEncodeProject(path string) string {
	return labsNonAlnum.ReplaceAllString(filepath.ToSlash(path), "-")
}

// labsProjectDirMatches reports a project directory that is the encoded
// repository or one of its Claude Code worktrees, and no sibling sharing
// the prefix. Sessions started from a subdirectory need --projects.
func labsProjectDirMatches(enc, name string) bool {
	return name == enc || strings.HasPrefix(name, enc+"--claude-worktrees-")
}

// labsMainRoot is the main worktree's root for a repository root, which is
// the root itself unless top is a linked worktree. Its transcripts live
// under the main worktree's project directory.
func labsMainRoot(top string) string {
	out, err := gitro.Output(context.Background(), top, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return top
	}
	common := strings.TrimSpace(string(out))
	if filepath.Base(common) != ".git" {
		return top
	}
	return filepath.Dir(common)
}

// writeLabsSession writes one session's points, named by its session hash
// so no session id reaches the filesystem.
func writeLabsSession(dir string, points []labs.Point) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := points[0].Session
	if name == "" {
		return fmt.Errorf("a decision point has no session hash")
	}
	data, err := json.MarshalIndent(points, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".json"), append(data, '\n'), 0o644)
}

// labsFileDigest is one transcript's sha256.
func labsFileDigest(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// labsCorpusSHA is sha256 over the sorted per-file digests, newline
// separated: the same corpus always reads the same, in any walk order.
func labsCorpusSHA(digests []string) string {
	if len(digests) == 0 {
		return ""
	}
	sorted := append([]string(nil), digests...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n") + "\n"))
	return hex.EncodeToString(sum[:])
}

// emitLabsJSON writes one indented JSON document.
func emitLabsJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "error: labs: %v\n", err)
		return 2
	}
	return 0
}

func printLabsCorpus(w io.Writer, reading labsCorpusReading) {
	coverage := labsCoverage(reading.Counts)
	fmt.Fprintf(w, "labs corpus: %s, %s, %s, %s\n",
		readcoverage.RenderCount(coverage, reading.Projects, "project(s)"), readcoverage.RenderCount(coverage, reading.Files, "transcript(s)"),
		readcoverage.RenderCount(coverage, reading.Sessions, "session(s)"), readcoverage.RenderCount(coverage, reading.Tally.Points, "decision point(s)"))
	fmt.Fprintf(w, "  eligibility: %d should-fire, %d should-not, %d unclassified (%d point(s) with no recoverable tree)\n",
		reading.Tally.ShouldFire, reading.Tally.ShouldNot, reading.Tally.Unclassified, reading.Tally.TreeUnrecovered)
	for _, reason := range reading.Tally.Reasons {
		fmt.Fprintf(w, "    %s: %d\n", reason.Reason, reason.Count)
	}
	fmt.Fprintf(w, "  unread: %d malformed line(s), %d oversize, %d unreadable file(s), %d duplicate(s)\n",
		reading.Counts.MalformedLines, reading.Counts.OversizeLines, reading.Counts.UnreadableFiles, reading.Counts.Duplicates)
	if reading.CorpusSHA != "" {
		fmt.Fprintf(w, "  corpus sha: %s\n", reading.CorpusSHA)
	}
	if reading.Written > 0 {
		fmt.Fprintf(w, "  wrote %d session file(s) under %s\n", reading.Written, reading.Out)
	}
}

// labsCoverage turns the decoder's unreadable files and directories into a
// read manifest, so every total is written as a named lower bound when any
// input could not be read (I38). Labels are classes and indexes, no paths.
func labsCoverage(counts labs.Counts) readmanifest.Manifest {
	set := readmanifest.NewSet()
	for _, issue := range counts.Issues {
		if strings.HasPrefix(issue.Kind, "unreadable_") || issue.Kind == "unhashed_session" {
			set.Record(issue.File, readmanifest.Unreadable(errors.New(issue.Kind)))
		}
	}
	return set.Manifest()
}

func printLabsFiring(w io.Writer, reading labsCorpusReading, rows []labs.FiringReadingRow) {
	fmt.Fprintf(w, "labs fire: %s over %s, %d surface(s)\n",
		readcoverage.RenderCount(labsCoverage(reading.Counts), reading.Tally.Points, "decision point(s)"),
		readcoverage.RenderCount(labsCoverage(reading.Counts), reading.Sessions, "session(s)"), len(rows))
	for _, row := range rows {
		fmt.Fprintf(w, "  %s %s: %s\n", row.Lab, row.Surface, labsStageLine(row))
	}
}

// labsStageLine prints the funnel as it was read: a measured stage with
// its margin, or the reason it was not measured.
func labsStageLine(row labs.FiringReadingRow) string {
	switch stage := row.Stages.Fired.(type) {
	case string:
		return stage
	case labs.Stage:
		line := fmt.Sprintf("fired at %d of %d eligible point(s) (recall %.2f [%.2f, %.2f]), %d false fire(s) at %d should-not point(s), delivered %d of %d",
			row.FiredAtEligible, row.Eligible, stage.Value, stage.Interval[0], stage.Interval[1], row.FalseFires, row.ShouldNot, row.Delivered, row.DeliveredOf)
		for _, missed := range row.Missed {
			line += fmt.Sprintf("; %s served %d missed point(s)", missed.Tool, missed.Count)
		}
		return line
	default:
		return "not measured: no reading"
	}
}
