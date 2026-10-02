package main

// coverage_ingest.go — "fathom coverage ingest <file>" (TQ-9,
// docs/design/test-intelligence.md §3 T3, ADR-051 decision 2): parses a
// foreign-format coverage report into the shared ForeignBlock model (one
// of TQ-10..TQ-13's parsers, internal/analysis/coverage, chosen by
// --format) and reports it bound to a commit, or "unbound".
//
// No store write here — ADR-051 decision 1's join rule ("a product that
// needs a new engine, a new store or a new analyzer is sent back") — this
// mirrors the shape Go's own coverage already uses end to end
// (internal/conserve's coverage.meta.json + profile file pair, read and
// joined at request time, never persisted to the index): the file this
// command reports on, plus the --commit a caller names, is the same
// durable artifact a future T1/T4 consumer reads and joins against the
// index itself (joinForeignBlocks, TQ-14) — nothing here needs a table.
//
// Never gating: a successful parse always exits 0 regardless of the
// coverage numbers it reports, bound or not — a coverage threshold is a
// CI policy decision, never fathom's (design doc §3 T3's own "never
// gating"). None of the six T3 formats embeds a commit SHA in any
// standard field, so binding comes only from an explicit --commit;
// omitting it always reports "unbound" (I43: only an attested run bound
// to a commit ever counts toward a gate — an unbound coverage report
// stays advisory-only, by construction, not by a check this command
// itself has to enforce).
import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
)

// coverageIngestMaxBytes is the streamed read cap (design doc §4
// tenancy: "streamed, capped at 256 MiB") — enforced twice: a fast
// os.Stat rejection before any read, and a boundedReader wrapped around
// the open file so a size that lies (a FIFO, a growing file) cannot
// stream past it either.
const coverageIngestMaxBytes int64 = 256 << 20

const coverageIngestSchema = "fathom.coverage.ingest/v1"

// coverageIngestReport is cmdCoverageIngest's --json shape.
type coverageIngestReport struct {
	Schema     string  `json:"schema"`
	Format     string  `json:"format"`
	File       string  `json:"file"`
	Commit     string  `json:"commit"`
	Bound      bool    `json:"bound"`
	Files      int     `json:"files"`
	Blocks     int     `json:"blocks"`
	Statements int     `json:"statements"`
	Covered    int     `json:"covered"`
	Percent    float64 `json:"percent"`
}

// boundedReader wraps r, returning an error the moment more than max
// bytes have been read — unlike io.LimitReader, which truncates silently
// at the limit (io.EOF, no error), a parser reading a boundedReader sees
// a real failure, so a report that exceeds the cap is refused rather than
// silently parsed as if it were merely shorter than it is.
type boundedReader struct {
	r    io.Reader
	max  int64
	read int64
}

func (b *boundedReader) Read(p []byte) (int, error) {
	if b.read >= b.max {
		return 0, fmt.Errorf("coverage: report exceeds the %d MiB cap", b.max/(1<<20))
	}
	// Clamp the request to what remains under the cap, so a single big
	// underlying Read (a strings/bytes Reader fills the whole buffer in
	// one call) cannot land past it before this method ever gets to
	// check — the next call then sees b.read == b.max and refuses.
	if remaining := b.max - b.read; int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := b.r.Read(p)
	b.read += int64(n)
	return n, err
}

// parseForeignCoverage dispatches --format to the matching TQ-10..TQ-13
// parser. cobertura/jacoco share ParseCoberturaJaCoCo (it self-detects
// from the XML root); coverage-py/istanbul share ParseCoveragePyIstanbul
// (it self-detects from the JSON's top-level keys) — both names are
// accepted as --format values anyway, so a caller who already knows which
// of the pair they have can say so, and the parser's own detection is a
// second, independent check rather than the only one.
func parseForeignCoverage(format string, r io.Reader) ([]coverage.ForeignBlock, error) {
	switch format {
	case "lcov":
		return coverage.ParseLcov(r)
	case "cobertura", "jacoco":
		return coverage.ParseCoberturaJaCoCo(r)
	case "coverage-py", "istanbul":
		return coverage.ParseCoveragePyIstanbul(r)
	case "llvm-cov":
		return coverage.ParseLLVMCov(r)
	default:
		return nil, fmt.Errorf("unsupported --format %q (want lcov, cobertura, jacoco, coverage-py, istanbul, or llvm-cov)", format)
	}
}

func cmdCoverageIngest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("coverage ingest", stderr)
	format := fs.String("format", "", "report format: lcov | cobertura | jacoco | coverage-py | istanbul | llvm-cov (required)")
	commit := fs.String("commit", "", "commit SHA to bind this report to (omit: reported unbound, never gating)")
	jsonOut := fs.Bool("json", false, "emit the fathom.coverage.ingest/v1 JSON report instead of a one-line summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	file := fs.Arg(0)
	if file == "" {
		fmt.Fprintln(stderr, "usage: fathom coverage ingest <file> --format lcov|cobertura|jacoco|coverage-py|istanbul|llvm-cov [--commit SHA] [--json]")
		return 1
	}
	if *format == "" {
		fmt.Fprintln(stderr, "error: coverage ingest requires --format (lcov, cobertura, jacoco, coverage-py, istanbul, or llvm-cov)")
		return 1
	}

	info, err := os.Stat(file)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if info.Size() > coverageIngestMaxBytes {
		fmt.Fprintf(stderr, "error: %s is %d bytes, over the %d MiB coverage ingest cap\n", file, info.Size(), coverageIngestMaxBytes/(1<<20))
		return 1
	}

	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer f.Close()

	blocks, err := parseForeignCoverage(*format, &boundedReader{r: f, max: coverageIngestMaxBytes})
	if err != nil {
		fmt.Fprintf(stderr, "error: parsing %s as %s: %v\n", file, *format, err)
		return 1
	}

	report := summarizeCoverageIngest(*format, file, *commit, blocks)

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		return 0
	}
	boundNote := "unbound"
	if report.Bound {
		boundNote = "commit " + report.Commit
	}
	fmt.Fprintf(stdout, "coverage ingest: %s (%s): %d block(s) across %d file(s), %d/%d statements covered (%.1f%%), %s\n",
		file, report.Format, report.Blocks, report.Files, report.Covered, report.Statements, report.Percent, boundNote)
	return 0
}

// summarizeCoverageIngest totals blocks into a coverageIngestReport.
// Commit is the literal string "unbound" (never empty) when no --commit
// was given, so both the human line and the JSON shape name the state
// explicitly rather than leaving a reader to infer it from an empty
// field.
func summarizeCoverageIngest(format, file, commit string, blocks []coverage.ForeignBlock) coverageIngestReport {
	files := map[string]bool{}
	statements, covered := 0, 0
	for _, b := range blocks {
		files[b.File] = true
		statements += b.Statements
		covered += b.Covered
	}
	pct := 0.0
	if statements > 0 {
		pct = float64(covered) / float64(statements) * 100
	}
	bound := commit != ""
	c := commit
	if !bound {
		c = "unbound"
	}
	return coverageIngestReport{
		Schema:     coverageIngestSchema,
		Format:     format,
		File:       file,
		Commit:     c,
		Bound:      bound,
		Files:      len(files),
		Blocks:     len(blocks),
		Statements: statements,
		Covered:    covered,
		Percent:    pct,
	}
}

// runCoverage dispatches `fathom coverage ingest`.
func runCoverage(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "ingest" {
		return cmdCoverageIngest(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "usage: fathom coverage ingest <file> --format lcov|cobertura|jacoco|coverage-py|istanbul|llvm-cov [--commit SHA] [--json]")
	return 1
}

// coverageRunner runs `fathom coverage` (registered below; EW-45).
type coverageRunner struct{}

func (coverageRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return runCoverage(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "coverage", Group: "review", Order: 70, Run: coverageRunner{},
		Summary: "coverage ingest <file> --format lcov|cobertura|jacoco|coverage-py|istanbul|llvm-cov [--commit SHA] [--json]: streams a foreign-language coverage report (256 MiB cap) into the shared ForeignBlock model and reports its block/statement/covered totals, bound to --commit or `unbound` — never a store write, never gating (TQ-9, docs/design/test-intelligence.md §3 T3)"})
}
