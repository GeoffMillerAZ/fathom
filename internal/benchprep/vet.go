package benchprep

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Criterion statuses. A pending criterion keeps the verdict pending, never kept.
const (
	VetPass    = "pass"
	VetFail    = "fail"
	VetPending = "pending"
)

// Subject verdicts: refused on any failure, otherwise pending until every
// criterion passes.
const (
	VerdictKept    = "kept"
	VerdictRefused = "refused"
	VerdictPending = "pending"
)

// c3Ceiling is C3's limit on the longest timed run (bench-prep section 5).
const c3Ceiling = 900.0

// SuiteRequest is one offline run of a subject's test command.
type SuiteRequest struct {
	Subject  string // manifest name
	Clone    string // operator-prepared clone, mounted read-only
	Language string // C2's primary language: go, python or node
	Stage    string // warmup, run1, run2, ...
	Command  string // the manifest's test commands, joined with &&
}

// SuiteRun is a run's receipt. A nonzero exit is an outcome, not an error.
type SuiteRun struct {
	Stage    string  `json:"stage"`
	Seconds  float64 `json:"seconds"`
	ExitCode int     `json:"exit_code"`
	Network  string  `json:"network"`
}

// SuiteRunner runs a request without network; an error means the run could
// not be attempted at all.
type SuiteRunner func(ctx context.Context, req SuiteRequest) (SuiteRun, error)

// GitReader runs git in clone and returns stdout.
type GitReader func(ctx context.Context, clone string, args ...string) ([]byte, error)

// C6Row is one recount.py row (D26): a symbol, its defining unit, its product
// call count and the distinct caller units.
type C6Row struct {
	Symbol       string   `json:"symbol"`
	DefiningUnit string   `json:"defining_unit"`
	Calls        int      `json:"calls"`
	Units        []string `json:"units"`
	Qualifies    bool     `json:"qualifies"`
}

// VetDeps carries everything VetSubject reads besides the manifest.
type VetDeps struct {
	// SubjectsDir holds the operator's full clones, one per manifest name.
	SubjectsDir string
	Git         GitReader // defaults to the git binary
	Suite       SuiteRunner
	// Miner defaults to MineFixCommits with its Go runner.
	Miner func(ctx context.Context, clone string) ([]FixCommit, error)
	// C6Rows returns the subject's recount rows; an fs.ErrNotExist error
	// leaves C6 pending.
	C6Rows func(name string) ([]C6Row, error)
	// SelfURL is the checkout's own repository; CorpusURLs are the repo urls
	// fathom's calibration corpus indexes; Peers are every bench subject.
	SelfURL    string
	CorpusURLs []string
	Peers      []Subject
}

// VetCriterion is one of C1 to C7.
type VetCriterion struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

// VetReceipt is the C1 to C7 record for one subject at its pinned sha.
type VetReceipt struct {
	Subject   string         `json:"subject"`
	URL       string         `json:"url"`
	SHA       string         `json:"sha"`
	Verdict   string         `json:"verdict"`
	Criteria  []VetCriterion `json:"criteria"`
	SuiteRuns []SuiteRun     `json:"suite_runs,omitempty"`
}

// VetSubject reads a subject's operator-prepared clone and its injected
// evidence and records C1 to C7. It never clones or touches the network.
// C4 is always pending on the controller's L0 reading. Criteria are sorted
// and no wall clock enters the receipt.
func VetSubject(ctx context.Context, s Subject, deps VetDeps) (VetReceipt, error) {
	if deps.Git == nil {
		deps.Git = execGit
	}
	if deps.Miner == nil {
		deps.Miner = func(ctx context.Context, clone string) ([]FixCommit, error) {
			return MineFixCommits(ctx, clone, nil)
		}
	}
	clone := filepath.Join(deps.SubjectsDir, s.Name)
	if info, err := os.Stat(clone); err != nil || !info.IsDir() {
		return VetReceipt{}, fmt.Errorf("vetting %s: no clone at %s", s.Name, clone)
	}
	if _, err := deps.Git(ctx, clone, "cat-file", "-e", s.SHA+"^{commit}"); err != nil {
		return VetReceipt{}, fmt.Errorf("vetting %s: sha %s is not in the clone: %w", s.Name, s.SHA, err)
	}
	listing, err := deps.Git(ctx, clone, "ls-tree", "-r", "-z", "--name-only", s.SHA)
	if err != nil {
		return VetReceipt{}, fmt.Errorf("vetting %s: %w", s.Name, err)
	}
	var files []string
	for _, f := range strings.Split(string(listing), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	r := VetReceipt{Subject: s.Name, URL: s.URL, SHA: s.SHA}
	add := func(id, status, evidence string) {
		r.Criteria = append(r.Criteria, VetCriterion{ID: id, Status: status, Evidence: evidence})
	}

	status, evidence, err := vetC1(ctx, s, clone, files, deps.Git)
	if err != nil {
		return VetReceipt{}, fmt.Errorf("vetting %s C1: %w", s.Name, err)
	}
	add("C1", status, evidence)

	language := primaryLanguage(s)
	status, evidence, err = vetC2(ctx, s, clone, files, language, deps.Git)
	if err != nil {
		return VetReceipt{}, fmt.Errorf("vetting %s C2: %w", s.Name, err)
	}
	add("C2", status, evidence)

	status, evidence, r.SuiteRuns, err = vetC3(ctx, s, clone, language, deps.Suite)
	if err != nil {
		return VetReceipt{}, fmt.Errorf("vetting %s C3: %w", s.Name, err)
	}
	add("C3", status, evidence)

	add("C4", VetPending, s.Bench.L0Reading)

	status, evidence, err = vetC5(ctx, s, clone, language, deps.Miner)
	if err != nil {
		return VetReceipt{}, fmt.Errorf("vetting %s C5: %w", s.Name, err)
	}
	add("C5", status, evidence)

	status, evidence, err = vetC6(s, deps.C6Rows)
	if err != nil {
		return VetReceipt{}, fmt.Errorf("vetting %s C6: %w", s.Name, err)
	}
	add("C6", status, evidence)

	status, evidence = vetC7(s, deps)
	add("C7", status, evidence)

	sort.Slice(r.Criteria, func(i, j int) bool { return r.Criteria[i].ID < r.Criteria[j].ID })
	r.Verdict = VerdictKept
	for _, c := range r.Criteria {
		if c.Status == VetFail {
			r.Verdict = VerdictRefused
			break
		}
		if c.Status == VetPending {
			r.Verdict = VerdictPending
		}
	}
	return r, nil
}

func execGit(ctx context.Context, clone string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", clone}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

var licenceFile = regexp.MustCompile(`(?i)^licen[cs]e`)

// classifyLicence applies static.py's signatures, plus CC0's title.
func classifyLicence(text string) string {
	switch {
	case strings.Contains(text, "Permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(text, "Apache License") && strings.Contains(text, "Version 2.0"):
		return "Apache-2.0"
	case strings.Contains(text, "Redistribution and use in source and binary forms"):
		if strings.Contains(text, "Neither the name") || strings.Contains(text, "neither the name") {
			return "BSD-3-Clause"
		}
		return "BSD-2-Clause"
	case strings.Contains(text, "CC0 1.0 Universal"):
		return "CC0-1.0"
	}
	return ""
}

func vetC1(ctx context.Context, s Subject, clone string, files []string, git GitReader) (string, string, error) {
	var found []string
	for _, f := range files {
		if strings.Contains(f, "/") || !licenceFile.MatchString(f) {
			continue
		}
		b, err := git(ctx, clone, "show", s.SHA+":"+f)
		if err != nil {
			return "", "", err
		}
		spdx := classifyLicence(string(b))
		if spdx == "" {
			found = append(found, f+": unrecognised")
			continue
		}
		if !c1Licences[spdx] {
			return VetFail, fmt.Sprintf("%s reads %s, off the C1 list", f, spdx), nil
		}
		if spdx != s.Licence {
			return VetFail, fmt.Sprintf("%s reads %s but the manifest declares %s", f, spdx, s.Licence), nil
		}
		return VetPass, fmt.Sprintf("%s reads %s at %s", f, spdx, s.SHA), nil
	}
	if len(found) == 0 {
		return VetFail, "no top-level LICENSE file at " + s.SHA, nil
	}
	return VetFail, "no C1 licence recognised: " + strings.Join(found, "; "), nil
}

// primaryLanguage reads the toolchain versions key; go wins over python over
// node when a manifest pins more than one.
func primaryLanguage(s Subject) string {
	for _, language := range []string{"go", "python", "node"} {
		if _, ok := s.Bench.Toolchain.Versions[language]; ok {
			return language
		}
	}
	return ""
}

var languageExtensions = map[string][]string{
	"go":     {".go"},
	"python": {".py"},
	"node":   {".ts", ".tsx", ".mts", ".cts"},
}

func vetC2(ctx context.Context, s Subject, clone string, files []string, language string, git GitReader) (string, string, error) {
	exts := languageExtensions[language]
	if len(exts) == 0 {
		return VetFail, "no primary language among toolchain versions (go, python, node)", nil
	}
	lines, count := 0, 0
	for _, f := range files {
		ext := path.Ext(f)
		matched := false
		for _, e := range exts {
			matched = matched || ext == e
		}
		if !matched {
			continue
		}
		b, err := git(ctx, clone, "show", s.SHA+":"+f)
		if err != nil {
			return "", "", err
		}
		count++
		lines += bytes.Count(b, []byte("\n"))
		if len(b) > 0 && b[len(b)-1] != '\n' {
			lines++
		}
	}
	evidence := fmt.Sprintf("%d %s lines in %d tracked files (%s) at %s, tests included", lines, language, count, strings.Join(exts, " "), s.SHA)
	if lines < 5000 || lines > 150000 {
		return VetFail, evidence + "; outside 5,000 to 150,000", nil
	}
	return VetPass, evidence, nil
}

func vetC3(ctx context.Context, s Subject, clone, language string, runner SuiteRunner) (string, string, []SuiteRun, error) {
	excluded := ""
	if n := len(s.Bench.Excluded); n > 0 || s.Bench.SuiteTests > 0 {
		excluded = fmt.Sprintf("; excluded %d", n)
		if s.Bench.SuiteTests > 0 {
			excluded += fmt.Sprintf(" of %d suite tests (%.1f%%)", s.Bench.SuiteTests, 100*float64(n)/float64(s.Bench.SuiteTests))
		}
	}
	if runner == nil {
		return VetPending, "no suite runner" + excluded, nil, nil
	}
	req := SuiteRequest{Subject: s.Name, Clone: clone, Language: language, Command: strings.Join(s.Bench.Test, " && ")}
	stages := []string{"warmup"}
	for i := 1; i <= s.Bench.FlakeRuns; i++ {
		stages = append(stages, fmt.Sprintf("run%d", i))
	}
	var runs []SuiteRun
	var problems, timings []string
	longest := 0.0
	for _, stage := range stages {
		req.Stage = stage
		run, err := runner(ctx, req)
		if err != nil {
			return "", "", nil, fmt.Errorf("%s: %w", stage, err)
		}
		run.Stage = stage
		runs = append(runs, run)
		timings = append(timings, fmt.Sprintf("%s %.3fs exit %d", stage, run.Seconds, run.ExitCode))
		if run.ExitCode != 0 {
			problems = append(problems, fmt.Sprintf("%s exited %d", stage, run.ExitCode))
		}
		if run.Network != "none" {
			problems = append(problems, fmt.Sprintf("%s ran with network %q", stage, run.Network))
		}
		if stage != "warmup" && run.Seconds > longest {
			longest = run.Seconds
		}
	}
	if longest > c3Ceiling {
		problems = append(problems, fmt.Sprintf("longest timed run %.3fs exceeds %.0fs", longest, c3Ceiling))
	}
	evidence := strings.Join(timings, ", ") + "; network none" + excluded
	if len(problems) > 0 {
		return VetFail, strings.Join(problems, "; ") + " (" + strings.Join(timings, ", ") + ")" + excluded, runs, nil
	}
	return VetPass, evidence, runs, nil
}

func vetC5(ctx context.Context, s Subject, clone, language string, miner func(context.Context, string) ([]FixCommit, error)) (string, string, error) {
	if language != "go" {
		return VetPending, "no miner for " + language, nil
	}
	commits, err := miner(ctx, clone)
	if err != nil {
		return "", "", err
	}
	total := len(commits)
	if s.Bench.Contamination.FreshAfter == "" {
		evidence := fmt.Sprintf("%d mined fix commits; post-cutoff count pending Q2 (no fresh_after)", total)
		if total >= 30 {
			return VetPending, evidence, nil
		}
		return VetFail, evidence + "; fewer than 30", nil
	}
	cutoff, err := time.Parse("2006-01-02", s.Bench.Contamination.FreshAfter)
	if err != nil {
		return "", "", fmt.Errorf("fresh_after: %w", err)
	}
	next := cutoff.AddDate(0, 0, 1)
	fresh := 0
	for _, c := range commits {
		created, err := time.Parse(time.RFC3339, c.CreatedAt)
		if err != nil {
			return "", "", fmt.Errorf("commit %s created_at: %w", c.Commit, err)
		}
		if !created.Before(next) {
			fresh++
		}
	}
	evidence := fmt.Sprintf("%d mined fix commits, %d after %s", total, fresh, s.Bench.Contamination.FreshAfter)
	switch {
	case total < 30:
		return VetFail, evidence + "; fewer than 30", nil
	case fresh < 15:
		return VetFail, evidence + "; fewer than 15 after the cutoff", nil
	}
	return VetPass, evidence, nil
}

func vetC6(s Subject, read func(string) ([]C6Row, error)) (string, string, error) {
	if read == nil {
		return VetPending, "no C6 recount supplied", nil
	}
	rows, err := read(s.Name)
	if errors.Is(err, fs.ErrNotExist) {
		return VetPending, "no C6 recount for " + s.Name, nil
	}
	if err != nil {
		return "", "", err
	}
	var qualifying []string
	for _, row := range rows {
		self := false
		for _, unit := range row.Units {
			self = self || unit == row.DefiningUnit
		}
		if row.Calls >= 5 && len(row.Units) >= 3 && !self {
			qualifying = append(qualifying, row.Symbol)
		}
	}
	sort.Strings(qualifying)
	evidence := fmt.Sprintf("%d of %d recount symbols have product fan-in of 5 or more calls from 3 or more other units", len(qualifying), len(rows))
	if len(qualifying) > 0 {
		shown := qualifying
		if len(shown) > 5 {
			shown = shown[:5]
		}
		evidence += ": " + strings.Join(shown, ", ")
	}
	if len(qualifying) < 3 {
		return VetFail, evidence + "; fewer than 3", nil
	}
	return VetPass, evidence, nil
}

// NormaliseRepoURL lowercases a repository url, rewrites scp-style git
// remotes to https, and trims a trailing slash and .git.
func NormaliseRepoURL(raw string) string {
	u := strings.ToLower(strings.TrimSpace(raw))
	if !strings.Contains(u, "://") {
		if at := strings.Index(u, "@"); at >= 0 {
			if host, rest, ok := strings.Cut(u[at+1:], ":"); ok {
				u = "https://" + host + "/" + rest
			}
		}
	}
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, ".git")
	return strings.TrimSuffix(u, "/")
}

// repoOrg is the first path segment after the host.
func repoOrg(u string) string {
	_, rest, ok := strings.Cut(u, "://")
	if !ok {
		rest = u
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "/" + parts[1]
}

func vetC7(s Subject, deps VetDeps) (string, string) {
	url := NormaliseRepoURL(s.URL)
	var problems []string
	if deps.SelfURL != "" && url == NormaliseRepoURL(deps.SelfURL) {
		problems = append(problems, "is the checkout's own repository")
	}
	for _, c := range deps.CorpusURLs {
		if url == NormaliseRepoURL(c) {
			problems = append(problems, "is indexed by the calibration corpus")
			break
		}
	}
	org := repoOrg(url)
	var same []string
	for _, p := range deps.Peers {
		if repoOrg(NormaliseRepoURL(p.URL)) == org {
			same = append(same, p.Name)
		}
	}
	sort.Strings(same)
	if len(same) > 2 {
		problems = append(problems, fmt.Sprintf("%d bench subjects share %s: %s", len(same), org, strings.Join(same, ", ")))
	}
	if len(problems) > 0 {
		return VetFail, url + " " + strings.Join(problems, "; ") + "; fleet check: unchecked"
	}
	return VetPass, fmt.Sprintf("%s: not the checkout's repository, not in the calibration corpus, %d of at most 2 bench subjects in %s; fleet check: unchecked", url, len(same), org)
}

// LoadCorpusURLs collects every url nested under a repos: list in
// corpusYAML and corpusDir/*.yaml, normalised and sorted.
func LoadCorpusURLs(corpusYAML, corpusDir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(corpusDir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	paths = append([]string{corpusYAML}, paths...)
	seen := map[string]bool{}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var doc any
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		collectRepoURLs(doc, seen)
	}
	urls := make([]string, 0, len(seen))
	for u := range seen {
		urls = append(urls, u)
	}
	sort.Strings(urls)
	return urls, nil
}

func collectRepoURLs(node any, seen map[string]bool) {
	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			if repos, ok := child.([]any); ok && key == "repos" {
				for _, repo := range repos {
					if entry, ok := repo.(map[string]any); ok {
						if u, ok := entry["url"].(string); ok {
							seen[NormaliseRepoURL(u)] = true
						}
					}
				}
			}
			collectRepoURLs(child, seen)
		}
	case []any:
		for _, child := range v {
			collectRepoURLs(child, seen)
		}
	}
}
