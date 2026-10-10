package certify

// riskcoverage.go — TQ-26 (docs/plans/test-intelligence.md wave 5,
// ADR-067): coverage reaches the risk rank. Every caller (the portfolio,
// test_impact rank:"risk" and the console) reads coverage from a NAMED
// source, never from a store (ADR-051 decision 1, the join rule): the
// files a repository names under `[tests] results_dir` — the attested Go
// profile pair `task tests:attest` writes (internal/conserve's
// coverage.meta.json and the profile it binds) and any foreign report of
// a known name (TQ-10 to TQ-13's formats) — read at request time, joined
// to symbols by joinForeignBlocks (TQ-14), and named in the answer. With
// none named, the answer says so, as before.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/coverage"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// coverageReadMax caps one coverage file's read, the cap `fathom coverage
// ingest` applies to the same reports (cmd/fathom coverageIngestMaxBytes).
const coverageReadMax int64 = 256 << 20

// GoProfileFormat is the attested Go profile pair's format name: the
// source's Path is its coverage.meta.json, which binds the profile.
const GoProfileFormat = "go-profile"

// CoverageSource is one named coverage file the risk rank reads.
type CoverageSource struct {
	// Format is GoProfileFormat or a foreign report format TQ-10 to TQ-13
	// parse: lcov, cobertura, jacoco, coverage-py, istanbul, llvm-cov.
	Format string `json:"format"`
	// Path is repo-relative: the report, or the Go pair's
	// coverage.meta.json.
	Path string `json:"path"`
	// Problem names a source the repository names that could not be
	// resolved (its configuration unreadable, a results_dir outside the
	// repository): reported, never read.
	Problem string `json:"problem,omitempty"`
}

// String is the source's name in an answer: "<format>=<path>".
func (s CoverageSource) String() string { return s.Format + "=" + s.Path }

// knownCoverageReports are the report names CoverageSources reads in
// results_dir, each the default output name of the tool that writes that
// format, in the order they are read.
var knownCoverageReports = []struct{ name, format string }{
	{"lcov.info", "lcov"},
	{"coverage-final.json", "istanbul"},
	{"coverage.json", "coverage-py"},
	{"coverage.xml", "cobertura"},
	{"cobertura.xml", "cobertura"},
	{"cobertura-coverage.xml", "cobertura"},
	{"jacoco.xml", "jacoco"},
	{"jacocoTestReport.xml", "jacoco"},
	{"llvm-cov.json", "llvm-cov"},
}

// CoverageSources names the coverage files root's own configuration
// points at: `[tests] results_dir` (the directory `task tests:attest`
// writes and a plain `fathom ingest` reads envelopes from) holding
// coverage.meta.json (the attested Go pair) and any report of a known name.
// nil when results_dir is unset or holds none. A results_dir that cannot
// be resolved is one source carrying its Problem.
func CoverageSources(root string) []CoverageSource {
	if root == "" {
		return nil
	}
	cfgPath := filepath.Join(root, ".fathom", "config.toml")
	loaded, err := config.Load(config.DefaultUserPath(), cfgPath)
	if err != nil {
		return []CoverageSource{{Format: "config", Path: ".fathom/config.toml", Problem: err.Error()}}
	}
	dir := loaded.Config.Tests.ResultsDir
	if dir == "" {
		return nil
	}
	if _, err := config.RepoPath(root, dir); err != nil {
		return []CoverageSource{{Format: "config", Path: ".fathom/config.toml", Problem: "[tests] results_dir " + err.Error()}}
	}
	rel := path.Clean(filepath.ToSlash(dir))
	var out []CoverageSource
	if fileExists(root, path.Join(rel, "coverage.meta.json")) {
		out = append(out, CoverageSource{Format: GoProfileFormat, Path: path.Join(rel, "coverage.meta.json")})
	}
	for _, k := range knownCoverageReports {
		if fileExists(root, path.Join(rel, k.name)) {
			out = append(out, CoverageSource{Format: k.format, Path: path.Join(rel, k.name)})
		}
	}
	return out
}

func fileExists(root, rel string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && info.Mode().IsRegular()
}

// riskCoverage is coverageForRisk's answer: the covered share per symbol
// key (a range join) or per file (a file-level join), each with its
// citation, the names of the sources read and why any named source was
// not.
type riskCoverage struct {
	bySymbol map[string]float64
	byFile   map[string]float64
	cite     map[string]string // symbol key or file → citation
	read     []string
	unusable []string
}

// supplied reports whether any named source was read.
func (rc riskCoverage) supplied() bool { return len(rc.read) > 0 }

// share is the covered share for a symbol in file, with its citation.
func (rc riskCoverage) share(symbol, file string) (float64, string, bool) {
	if v, ok := rc.bySymbol[symbol]; ok {
		return v, rc.cite[symbol], true
	}
	if v, ok := rc.byFile[file]; ok {
		return v, rc.cite[file], true
	}
	return 0, "", false
}

// coverageForRisk reads sources under root, joins every usable one's
// blocks to symbols with joinForeignBlocks — by line range where a file's
// function symbols all carry one, else at file level, named — and returns
// the covered share per symbol. A source that cannot be read or used (a
// missing file, an unbound, dirty or failed Go pair, a malformed report)
// is named in unusable and contributes nothing: never a guessed number.
func coverageForRisk(root string, sources []CoverageSource, symbols []core.SymbolNode) riskCoverage {
	rc := riskCoverage{bySymbol: map[string]float64{}, byFile: map[string]float64{}, cite: map[string]string{}}
	var blocks []coverage.ForeignBlock
	var names []string
	for _, src := range sources {
		if src.Problem != "" {
			rc.unusable = append(rc.unusable, src.String()+": "+src.Problem)
			continue
		}
		b, name, err := readCoverageSource(root, src)
		if err != nil {
			rc.unusable = append(rc.unusable, src.String()+": "+err.Error())
			continue
		}
		blocks = append(blocks, b...)
		names = append(names, name)
	}
	if len(names) == 0 {
		return rc
	}
	rc.read = names
	label := strings.Join(names, ", ")
	join := coverage.JoinForeignBlocks(blocks, symbols)
	for _, s := range join.BySymbol {
		if s.Statements <= 0 {
			continue
		}
		rc.bySymbol[s.Symbol] = float64(s.Covered) / float64(s.Statements)
		rc.cite[s.Symbol] = fmt.Sprintf("%s: %.0f%% of %d statements covered", label, s.Percent, s.Statements)
	}
	for _, f := range join.ByFile {
		if f.Statements <= 0 {
			continue
		}
		rc.byFile[f.File] = float64(f.Covered) / float64(f.Statements)
		rc.cite[f.File] = fmt.Sprintf("%s: file-level %.0f%% of %d statements in %s covered (this file's symbols carry no line ranges)", label, f.Percent, f.Statements, f.File)
	}
	return rc
}

// readCoverageSource reads one named source into the shared block model
// and returns the name the answer cites. The Go pair's meta must be bound
// to a commit, from a clean tree and a run that passed (I44, the bar
// internal/conserve's CoverageMetaUsable applies), and its profile must
// stay inside the repository.
func readCoverageSource(root string, src CoverageSource) ([]coverage.ForeignBlock, string, error) {
	if src.Format != GoProfileFormat {
		data, err := readRepoFile(root, src.Path)
		if err != nil {
			return nil, "", err
		}
		blocks, err := parseForeignReport(src.Format, bytes.NewReader(data))
		if err != nil {
			return nil, "", err
		}
		return blocks, src.String(), nil
	}
	raw, err := readRepoFile(root, src.Path)
	if err != nil {
		return nil, "", err
	}
	var meta struct {
		Commit  string `json:"commit"`
		Dirty   bool   `json:"dirty"`
		Profile string `json:"profile"`
		Passed  bool   `json:"passed"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, "", fmt.Errorf("parsing: %w", err)
	}
	if usable, why := coverageMetaUsable(meta.Commit, meta.Dirty, meta.Passed); !usable {
		return nil, "", errors.New(why)
	}
	if meta.Profile == "" {
		return nil, "", errors.New("names no profile")
	}
	if filepath.IsAbs(meta.Profile) || path.IsAbs(meta.Profile) {
		return nil, "", fmt.Errorf("names a profile outside the repository (%s)", meta.Profile)
	}
	data, err := readRepoFile(root, path.Join(path.Dir(src.Path), filepath.ToSlash(meta.Profile)))
	if err != nil {
		return nil, "", err
	}
	blocks, err := goProfileBlocks(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	commit := meta.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	return blocks, fmt.Sprintf("%s (commit %s)", src.String(), commit), nil
}

// coverageMetaUsable is the attested pair's bar: bound to a commit, a
// clean tree, a run that passed. It mirrors internal/conserve's
// CoverageMetaUsable (which imports this package, so it cannot be called
// from here); TestCoverageMetaBarMatchesConserve pins the two together.
func coverageMetaUsable(commit string, dirty, passed bool) (bool, string) {
	switch {
	case commit == "":
		return false, "coverage meta names no bound commit"
	case dirty:
		return false, "coverage meta was captured on a dirty working tree"
	case !passed:
		return false, "coverage meta's run did not pass"
	}
	return true, ""
}

// readRepoFile reads rel under root, refusing a path that leaves the
// repository (config.RepoPath) or a file over coverageReadMax.
func readRepoFile(root, rel string) ([]byte, error) {
	p, err := config.RepoPath(root, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, errors.New("no such file")
		}
		return nil, err
	}
	if info.Size() > coverageReadMax {
		return nil, fmt.Errorf("%d bytes, over the %d MiB coverage cap", info.Size(), coverageReadMax>>20)
	}
	f, err := os.Open(p) //nolint:gosec — a repo-relative path config.RepoPath kept inside the repository
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, coverageReadMax+1))
}

// parseForeignReport dispatches a foreign report to its TQ-10 to TQ-13
// parser, the formats `fathom coverage ingest --format` names.
func parseForeignReport(format string, r io.Reader) ([]coverage.ForeignBlock, error) {
	switch format {
	case "lcov":
		return coverage.ParseLcov(r)
	case "cobertura", "jacoco":
		return coverage.ParseCoberturaJaCoCo(r)
	case "coverage-py", "istanbul":
		return coverage.ParseCoveragePyIstanbul(r)
	case "llvm-cov":
		return coverage.ParseLLVMCov(r)
	}
	return nil, fmt.Errorf("unsupported coverage format %q", format)
}

// goProfileBlocks parses a Go cover profile into the shared block model:
// one block per distinct span, its count the maximum across the duplicate
// sections a `-coverpkg=./...` run writes (the block ran if any test
// binary ran it), each statement covered when its block ran.
func goProfileBlocks(r io.Reader) ([]coverage.ForeignBlock, error) {
	prof, err := coverage.ParseProfile(r)
	if err != nil {
		return nil, err
	}
	type span struct {
		file                 string
		sl, sc, el, ec, stmt int
	}
	count := map[span]int{}
	var order []span
	for _, b := range prof.Blocks {
		k := span{b.ImportPath, b.StartLine, b.StartCol, b.EndLine, b.EndCol, b.NumStmt}
		c, seen := count[k]
		if !seen {
			order = append(order, k)
		}
		if !seen || b.Count > c {
			count[k] = b.Count
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].file != order[j].file {
			return order[i].file < order[j].file
		}
		return order[i].sl < order[j].sl
	})
	out := make([]coverage.ForeignBlock, 0, len(order))
	for _, k := range order {
		covered := 0
		if count[k] > 0 {
			covered = k.stmt
		}
		out = append(out, coverage.ForeignBlock{File: k.file, StartLine: k.sl, EndLine: k.el, Statements: k.stmt, Covered: covered})
	}
	return out, nil
}
