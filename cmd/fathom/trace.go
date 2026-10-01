package main

// trace.go — U8's evidence pack (docs/design/change-and-compliance.md §3,
// plan CX-16 / CX-17):
//
//	fathom trace export --standard <id> --out <dir> [--service S] [--store DB] [--commit REF]
//	fathom trace verify <dir> [--service S] [--store DB]
//
// Export is only ever on request (nothing writes a pack on its own). It
// reads the plans (docs/plans/*.md), the repository-authored mapping
// (.fathom/trace/<id>.cue — fathom ships no standard's clauses, ADR-011)
// and the decision records AT the commit (git show), joins them with the
// store's verify and certificate readings through review.BuildTraceMatrix,
// and writes the matrix (JSON + Markdown), each proving artifact's digest,
// the readings, the commit, and a sha256 manifest.
//
// Verify recomputes the pack at its own recorded commit and compares every
// file byte for byte (determinism is a contract); a difference is named by
// file and, for the matrix, by cell.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/review"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

const traceUsage = "usage: fathom trace export --standard <id> --out <dir> [--service S] [--store DB] [--commit REF]\n       fathom trace verify <dir> [--service S] [--store DB]"

func cmdTrace(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, traceUsage)
		return 2
	}
	switch args[0] {
	case "export":
		return cmdTraceExport(args[1:], stdout, stderr)
	case "verify":
		return cmdTraceVerify(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, traceUsage)
	return 2
}

// tracePackFiles are the pack's files other than the manifest, in the
// order they are written.
var tracePackFiles = []string{"COMMIT", "matrix.json", "matrix.md", "digests.json", "readings.json"}

// cmdTraceExport writes an evidence pack (CX-16).
func cmdTraceExport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("trace export", stderr)
	standard := fs.String("standard", "", "the standard mapping id (.fathom/trace/<id>.cue)")
	out := fs.String("out", "", "directory to write the pack into")
	service := fs.String("service", "", "service ID (default: resolved from the repo directory name)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	commitRef := fs.String("commit", "HEAD", "the commit whose plans, mapping and decisions the pack reads")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *standard == "" || *out == "" {
		fmt.Fprintln(stderr, "error: trace export requires --standard and --out")
		fmt.Fprintln(stderr, traceUsage)
		return 2
	}
	svc := *service
	if svc == "" {
		svc = defaultTestsCoverageService()
	}
	if svc == "" {
		fmt.Fprintln(stderr, "error: trace export requires --service (could not guess one from the current directory)")
		return 2
	}
	ctx := context.Background()
	repo := verifyRepoRoot()
	commit, err := traceGit(ctx, repo, "rev-parse", "--verify", *commitRef+"^{commit}")
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve commit %q: %v\n", *commitRef, err)
		return 2
	}
	files, err := buildTracePack(ctx, repo, strings.TrimSpace(commit), svc, *storePath, *standard)
	if err != nil {
		fmt.Fprintf(stderr, "error: trace export: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: create %s: %v\n", *out, err)
		return 1
	}
	names := append(append([]string(nil), tracePackFiles...), review.ManifestName)
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(*out, n), files[n], 0o644); err != nil {
			fmt.Fprintf(stderr, "error: write %s: %v\n", n, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "trace export: %s pack for %s at %s written to %s (%d files, manifest %s)\n",
		*standard, svc, shortCommit(strings.TrimSpace(commit)), *out, len(names), review.ManifestName)
	return 0
}

// cmdTraceVerify re-verifies a pack byte for byte at its commit (CX-17).
func cmdTraceVerify(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("trace verify", stderr)
	service := fs.String("service", "", "service ID (default: the pack's own)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	// Accept the directory before or after the flags.
	var dir string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		dir, args = args[0], args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if dir == "" && fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if dir == "" {
		fmt.Fprintln(stderr, traceUsage)
		return 2
	}
	onDisk := map[string][]byte{}
	for _, n := range append(append([]string(nil), tracePackFiles...), review.ManifestName) {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // named as missing by the diff below
			}
			fmt.Fprintf(stderr, "error: read %s: %v\n", n, err)
			return 2
		}
		onDisk[n] = b
	}
	commit := strings.TrimSpace(string(onDisk["COMMIT"]))
	var head review.TracePack
	if err := json.Unmarshal(onDisk["matrix.json"], &head); err != nil || commit == "" {
		fmt.Fprintf(stderr, "error: %s is not a trace pack (COMMIT and matrix.json are required)\n", dir)
		return 2
	}
	svc := *service
	if svc == "" {
		svc = head.Service
	}
	ctx := context.Background()
	recomputed, err := buildTracePack(ctx, verifyRepoRoot(), commit, svc, *storePath, head.Standard)
	if err != nil {
		fmt.Fprintf(stderr, "error: recompute at %s: %v\n", shortCommit(commit), err)
		return 2
	}
	diffs := review.DiffTracePacks(onDisk, recomputed)
	// The pack must also agree with its own manifest: a file edited after
	// export no longer matches the sha256 the manifest recorded for it.
	if m, ok := onDisk[review.ManifestName]; ok && !bytes.Equal(m, review.RenderManifest(onDisk)) {
		diffs = append(diffs, review.PackDifference{File: review.ManifestName, Cells: []string{"does not match the pack's own files"}})
	}
	if len(diffs) == 0 {
		fmt.Fprintf(stdout, "trace verify: %s pack at %s re-verified byte-identical (%d files)\n", head.Standard, shortCommit(commit), len(recomputed))
		return 0
	}
	fmt.Fprintf(stdout, "trace verify: %s pack at %s DIFFERS in %d file(s):\n", head.Standard, shortCommit(commit), len(diffs))
	for _, d := range diffs {
		if len(d.Cells) == 0 {
			fmt.Fprintf(stdout, "  %s\n", d.File)
			continue
		}
		fmt.Fprintf(stdout, "  %s: %s\n", d.File, strings.Join(d.Cells, ", "))
	}
	return 1
}

// traceGit runs one read-only git command in repo, contexted.
func traceGit(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

var (
	traceADRHeading = regexp.MustCompile(`(?m)^#{2,3} (ADR-\d+)\b`)
	traceInvariant  = regexp.MustCompile(`(?m)^\| (I\d+) \|`)
)

// traceDigest is one proving artifact read at the commit.
type traceDigest struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// traceReadings is readings.json: the verify and certificate readings the
// matrix was computed from.
type traceReadings struct {
	Verify       []traceVerifyReading      `json:"verify"`
	Certificates []review.TraceCertificate `json:"certificates"`
	Unread       []string                  `json:"unread,omitempty"`
}

type traceVerifyReading struct {
	Plan    string `json:"plan"`
	Total   int    `json:"total"`
	Pass    int    `json:"pass"`
	Fail    int    `json:"fail"`
	Unknown int    `json:"unknown"`
}

// buildTracePack computes every file of a pack at commit. It reads only
// what the matrix names: the plans, the mapping and the decision records
// at the commit (git show), and the service's own verify and certificate
// readings from the store.
func buildTracePack(ctx context.Context, repo, commit, svc, storePath, standard string) (map[string][]byte, error) {
	show := func(path string) ([]byte, error) {
		s, err := traceGit(ctx, repo, "show", commit+":"+path)
		return []byte(s), err
	}
	var digests []traceDigest
	digest := func(path string, b []byte) {
		sum := sha256.Sum256(b)
		digests = append(digests, traceDigest{Path: path, SHA256: hex.EncodeToString(sum[:])})
	}

	mappingPath := ".fathom/trace/" + standard + ".cue"
	mappingSrc, err := show(mappingPath)
	if err != nil {
		return nil, fmt.Errorf("no standard mapping %s at %s (fathom ships none — the repository authors it, schema/trace/schema.cue): %w", mappingPath, shortCommit(commit), err)
	}
	mapping, err := review.ParseStandardMapping(mappingPath, mappingSrc)
	if err != nil {
		return nil, err
	}
	digest(mappingPath, mappingSrc)

	listing, err := traceGit(ctx, repo, "ls-tree", "-r", "--name-only", commit, "--", "docs/plans")
	if err != nil {
		return nil, err
	}
	var planPaths []string
	for _, p := range strings.Split(strings.TrimSpace(listing), "\n") {
		if strings.HasSuffix(p, ".md") && filepath.Base(p) != "README.md" && filepath.Dir(p) == "docs/plans" {
			planPaths = append(planPaths, p)
		}
	}
	sort.Strings(planPaths)
	var sources []review.TraceSource
	var plans []verify.Plan
	for _, p := range planPaths {
		b, err := show(p)
		if err != nil {
			return nil, err
		}
		digest(p, b)
		plan := verify.ParseMarkdownPlan(svc, b)
		sources = append(sources, review.TraceSource{Path: p, Plan: plan})
		plans = append(plans, plan)
	}

	decisions := map[string]bool{}
	var unread []string
	for _, p := range []string{"ADR.md", "docs/intent/invariants.md"} {
		b, err := show(p)
		if err != nil {
			unread = append(unread, p)
			continue
		}
		digest(p, b)
		for _, re := range []*regexp.Regexp{traceADRHeading, traceInvariant} {
			for _, m := range re.FindAllSubmatch(b, -1) {
				decisions[string(m[1])] = true
			}
		}
	}
	if len(unread) == 2 {
		decisions = nil // nothing resolvable: cited ids read "cited", unchecked
	}

	reports := map[string]verify.Report{}
	var readings traceReadings
	readings.Unread = unread
	if len(plans) > 0 {
		reps, err := verifyAllAgainstStore(plans, svc, storePath, "", verify.DirFiles(repo), "", false)
		if err != nil {
			return nil, err
		}
		for i, rep := range reps {
			reports[planPaths[i]] = rep
			readings.Verify = append(readings.Verify, traceVerifyReading{
				Plan: planPaths[i], Total: rep.Summary.Total, Pass: rep.Summary.Pass, Fail: rep.Summary.Fail, Unknown: rep.Summary.Unknown,
			})
		}
	}

	certs, certUnread, err := traceCertificates(ctx, storePath, svc, repo)
	if err != nil {
		return nil, err
	}
	readings.Certificates = certs
	readings.Unread = append(readings.Unread, certUnread...)
	sort.Strings(readings.Unread)

	mx := review.BuildTraceMatrix(review.TraceInput{
		Sources: sources, Reports: reports, Certificates: certs, Commit: commit, Decisions: decisions,
	})
	pack := review.TracePack{Standard: mapping.Standard, Commit: commit, Service: svc, Clauses: review.ApplyMapping(mapping, mx), Matrix: mx}

	sort.Slice(digests, func(i, j int) bool { return digests[i].Path < digests[j].Path })
	files := map[string][]byte{
		"COMMIT":        []byte(commit + "\n"),
		"matrix.json":   review.RenderTraceJSON(pack),
		"matrix.md":     review.RenderTraceMarkdown(pack),
		"digests.json":  traceJSON(digests),
		"readings.json": traceJSON(readings),
	}
	files[review.ManifestName] = review.RenderManifest(files)
	return files, nil
}

func traceJSON(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return buf.Bytes()
}

// traceCertificates flattens the certifications layer's compiled
// certificates for svc, sorted.
func traceCertificates(ctx context.Context, storePath, svc, repo string) ([]review.TraceCertificate, []string, error) {
	st, err := store.Open(storePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open store %q: %w", storePath, err)
	}
	defer st.Close()
	report, err := certify.Gaps(ctx, st, svc, repo, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("certificates for %q: %w", svc, err)
	}
	var out []review.TraceCertificate
	for _, t := range report.Targets {
		for _, c := range t.Certificates {
			out = append(out, review.TraceCertificate{TargetKind: t.Kind, TargetID: t.ID, Test: c.Test, State: c.State, RunDigest: c.RunDigest})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.TargetKind != b.TargetKind {
			return a.TargetKind < b.TargetKind
		}
		if a.TargetID != b.TargetID {
			return a.TargetID < b.TargetID
		}
		return a.Test < b.Test
	})
	return out, report.Unread, nil
}

// traceRunner runs `fathom trace` (registered below; EW-45).
type traceRunner struct{}

func (traceRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdTrace(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "trace", Group: "review", Order: 60, Run: traceRunner{},
		Summary: "trace export --standard <id> --out <dir> [--service S] [--store DB] [--commit REF]: the evidence pack, only on request — the traceability matrix (requirement, code, test, certificate, attested run, decision; each cell with its live state and reading) as JSON and Markdown, grouped by the repository-authored .fathom/trace/<id>.cue mapping, with artifact digests, readings, the commit and a sha256 manifest | trace verify <dir>: recompute the pack at its commit; byte-identical or each difference named by file and cell (CX-16, CX-17)"})
}
