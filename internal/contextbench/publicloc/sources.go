package publicloc

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// The two pinned upstream sources. Their URLs and licences are constants
// of the derivation, recorded beside every pin in the emitted corpus so a
// reader never has to look them up to know what they may redistribute.
const (
	SourceMultiSWEBench = "multi-swe-bench"
	SourceSWEExplore    = "swe-explore"
)

// SourceInfo is the fixed identity of one upstream dataset.
type SourceInfo struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	License string `json:"license"`
	// Fields documents the adapter's schema assumption so a pin whose
	// rows do not match fails loudly at the field named here.
	Fields string `json:"fields"`
}

// Sources lists what the importer knows how to read.
var Sources = map[string]SourceInfo{
	SourceMultiSWEBench: {
		Name:    SourceMultiSWEBench,
		URL:     "https://huggingface.co/datasets/ByteDance-Seed/Multi-SWE-bench",
		License: "CC0-1.0",
		Fields:  "JSONL rows: org, repo, number, title, body, base{sha}, resolved_issues[{title,body}], fix_patch (instance_id derived as org__repo-number when absent)",
	},
	SourceSWEExplore: {
		Name: SourceSWEExplore,
		URL:  "https://huggingface.co/datasets/SWE-Explore-Bench/SWE-Explore-Bench",
		// Verified 2026-09-13 against the dataset card: CC-BY-NC-ND-4.0, not
		// the MIT this pin carried since PB-14. ND forbids derived works, so
		// no committed corpus derives from it (the 2026-09-13 derivation
		// also found no base_commit/patch fields to derive from); the
		// importer keeps the source for localization truth only.
		License: "CC-BY-NC-ND-4.0",
		Fields:  "JSONL rows: instance_id, repo (org/repo), base_commit, problem_statement, language, patch and/or regions (map path -> [[start,end],...] or list of {file,start,end})",
	},
}

// Pin records the exact upstream snapshot a corpus was derived from.
type Pin struct {
	Source   string `json:"source"`
	URL      string `json:"url"`
	License  string `json:"license"`
	Revision string `json:"revision,omitempty"`
	File     string `json:"file"`
	SHA256   string `json:"sha256"`
	Rows     int    `json:"rows"`
	Selected int    `json:"selected"`
}

// Region is a line-level ground-truth span at base_commit.
type Region struct {
	Path  string `json:"path"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// Instance is one upstream row normalised to what derivation needs.
type Instance struct {
	Source     string   `json:"source"`
	InstanceID string   `json:"instance_id"`
	Org        string   `json:"org"`
	Repo       string   `json:"repo"`
	BaseCommit string   `json:"base_commit"`
	Title      string   `json:"title"`
	Problem    string   `json:"problem"`
	Patch      string   `json:"patch,omitempty"`
	Regions    []Region `json:"regions,omitempty"`
}

// RepoURL is the clone URL the runbook pins the subject from.
func (in Instance) RepoURL() string {
	return "https://github.com/" + in.Org + "/" + in.Repo
}

var unsafeID = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// SafeID is the instance id as a file-system and task-id component.
func (in Instance) SafeID() string {
	return unsafeID.ReplaceAllString(in.InstanceID, "_")
}

var unsafeService = regexp.MustCompile(`[^a-z0-9-]+`)

// Service is the deterministic fathom service name the runner ingests
// this instance's base_commit snapshot under (in a scratch store, never
// the fleet store).
func (in Instance) Service() string {
	sha := in.BaseCommit
	if len(sha) > 8 {
		sha = sha[:8]
	}
	return unsafeService.ReplaceAllString(strings.ToLower("pub-"+in.Org+"-"+in.Repo+"-"+sha), "-")
}

// SHA256Hex is the pin digest of one snapshot's bytes.
func SHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ReadJSONL splits r into non-blank lines and hands each to fn with its
// 1-based line number, so a malformed row is an error naming the row —
// never a silently shorter dataset (ADR-027).
func ReadJSONL(r io.Reader, fn func(line int, raw []byte) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	n := 0
	for sc.Scan() {
		n++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		if err := fn(n, []byte(raw)); err != nil {
			return err
		}
	}
	return sc.Err()
}

type multiSWERow struct {
	InstanceID string `json:"instance_id"`
	Org        string `json:"org"`
	Repo       string `json:"repo"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Base       struct {
		Sha string `json:"sha"`
	} `json:"base"`
	ResolvedIssues []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
	} `json:"resolved_issues"`
	FixPatch string `json:"fix_patch"`
}

// ReadMultiSWEBench reads a Multi-SWE-bench JSONL snapshot (one PR
// instance per line) into Instances. The problem statement is the resolved
// issues' text, or the PR title and body when no issue is linked.
func ReadMultiSWEBench(r io.Reader) ([]Instance, error) {
	var out []Instance
	err := ReadJSONL(r, func(line int, raw []byte) error {
		var row multiSWERow
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("%s line %d: %w", SourceMultiSWEBench, line, err)
		}
		id := row.InstanceID
		if id == "" {
			id = fmt.Sprintf("%s__%s-%d", row.Org, row.Repo, row.Number)
		}
		switch {
		case row.Org == "" || row.Repo == "":
			return fmt.Errorf("%s line %d (%s): org/repo missing", SourceMultiSWEBench, line, id)
		case row.Base.Sha == "":
			return fmt.Errorf("%s line %d (%s): base.sha missing", SourceMultiSWEBench, line, id)
		case row.FixPatch == "":
			return fmt.Errorf("%s line %d (%s): fix_patch missing", SourceMultiSWEBench, line, id)
		}
		var parts []string
		title := row.Title
		for _, iss := range row.ResolvedIssues {
			if title == row.Title && iss.Title != "" {
				title = iss.Title
			}
			parts = append(parts, strings.TrimSpace(iss.Title+"\n\n"+iss.Body))
		}
		if len(parts) == 0 {
			parts = append(parts, strings.TrimSpace(row.Title+"\n\n"+row.Body))
		}
		out = append(out, Instance{
			Source:     SourceMultiSWEBench,
			InstanceID: id,
			Org:        row.Org,
			Repo:       row.Repo,
			BaseCommit: row.Base.Sha,
			Title:      title,
			Problem:    strings.Join(parts, "\n\n---\n\n"),
			Patch:      row.FixPatch,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

type sweExploreRow struct {
	InstanceID       string          `json:"instance_id"`
	Repo             string          `json:"repo"`
	BaseCommit       string          `json:"base_commit"`
	ProblemStatement string          `json:"problem_statement"`
	Language         string          `json:"language"`
	Patch            string          `json:"patch"`
	Regions          json.RawMessage `json:"regions"`
	GroundTruth      json.RawMessage `json:"ground_truth"`
}

// ReadSWEExplore reads a SWE-Explore-Bench JSONL snapshot. Rows whose
// language is present and not Go are skipped (this is the Go subset by
// construction); a Go row missing its truth is an error.
func ReadSWEExplore(r io.Reader) ([]Instance, error) {
	var out []Instance
	err := ReadJSONL(r, func(line int, raw []byte) error {
		var row sweExploreRow
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("%s line %d: %w", SourceSWEExplore, line, err)
		}
		if row.Language != "" && !strings.EqualFold(row.Language, "go") {
			return nil
		}
		org, repo, ok := strings.Cut(row.Repo, "/")
		switch {
		case row.InstanceID == "":
			return fmt.Errorf("%s line %d: instance_id missing", SourceSWEExplore, line)
		case !ok || org == "" || repo == "":
			return fmt.Errorf("%s line %d (%s): repo must be org/repo, got %q", SourceSWEExplore, line, row.InstanceID, row.Repo)
		case row.BaseCommit == "":
			return fmt.Errorf("%s line %d (%s): base_commit missing", SourceSWEExplore, line, row.InstanceID)
		case row.ProblemStatement == "":
			return fmt.Errorf("%s line %d (%s): problem_statement missing", SourceSWEExplore, line, row.InstanceID)
		}
		regionsRaw := row.Regions
		if len(regionsRaw) == 0 {
			regionsRaw = row.GroundTruth
		}
		regions, err := parseRegions(regionsRaw)
		if err != nil {
			return fmt.Errorf("%s line %d (%s): %w", SourceSWEExplore, line, row.InstanceID, err)
		}
		if row.Patch == "" && len(regions) == 0 {
			return fmt.Errorf("%s line %d (%s): neither patch nor regions present", SourceSWEExplore, line, row.InstanceID)
		}
		title, _, _ := strings.Cut(strings.TrimSpace(row.ProblemStatement), "\n")
		out = append(out, Instance{
			Source:     SourceSWEExplore,
			InstanceID: row.InstanceID,
			Org:        org,
			Repo:       repo,
			BaseCommit: row.BaseCommit,
			Title:      title,
			Problem:    row.ProblemStatement,
			Patch:      row.Patch,
			Regions:    regions,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// parseRegions accepts the two shapes line-level truth is published in:
// a map from path to [[start,end],...] and a list of {file,start,end}.
func parseRegions(raw json.RawMessage) ([]Region, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var regions []Region
	var asMap map[string][][]int
	if err := json.Unmarshal(raw, &asMap); err == nil {
		for p, spans := range asMap {
			for _, sp := range spans {
				if len(sp) != 2 {
					return nil, fmt.Errorf("region for %s: want [start,end], got %v", p, sp)
				}
				regions = append(regions, Region{Path: p, Start: sp[0], End: sp[1]})
			}
		}
	} else {
		var asList []struct {
			File  string `json:"file"`
			Path  string `json:"path"`
			Start int    `json:"start"`
			End   int    `json:"end"`
		}
		if err := json.Unmarshal(raw, &asList); err != nil {
			return nil, fmt.Errorf("regions: neither a path map nor a list: %w", err)
		}
		for _, r := range asList {
			p := r.File
			if p == "" {
				p = r.Path
			}
			regions = append(regions, Region{Path: p, Start: r.Start, End: r.End})
		}
	}
	for _, r := range regions {
		if r.Path == "" || r.Start <= 0 || r.End < r.Start {
			return nil, fmt.Errorf("region %+v: path, start>=1 and end>=start are required", r)
		}
	}
	sort.Slice(regions, func(i, j int) bool {
		if regions[i].Path != regions[j].Path {
			return regions[i].Path < regions[j].Path
		}
		if regions[i].Start != regions[j].Start {
			return regions[i].Start < regions[j].Start
		}
		return regions[i].End < regions[j].End
	})
	return regions, nil
}
