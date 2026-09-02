package publicloc

import (
	"fmt"
	"sort"
	"strings"
)

// SelectionRule is the pre-registered instance-selection rule. The
// registered values live in RegisteredRule; a corpus built under any other
// rule carries pre_registered=false in its selection report.
type SelectionRule struct {
	MinFiles int `json:"min_files"`
	MaxFiles int `json:"max_files"`
	N        int `json:"n"`
}

// RegisteredRule is the rule ADR-031 committed before the first run: a gold
// patch touching 1–5 non-test Go files, sorted by instance_id, first 35
// (the plan's 30–40 window; 35 is the midpoint, fixed here so "first N"
// has one meaning).
var RegisteredRule = SelectionRule{MinFiles: 1, MaxFiles: 5, N: 35}

// RegisteredNRange is the window the plan pre-registered for N; an N
// outside it is an unregistered corpus, which the importer refuses unless
// told the output is a fixture or an exploration.
const (
	RegisteredNMin = 30
	RegisteredNMax = 40
)

// PreRegistered reports whether r is within the registered rule.
func (r SelectionRule) PreRegistered() bool {
	return r.MinFiles == RegisteredRule.MinFiles && r.MaxFiles == RegisteredRule.MaxFiles &&
		r.N >= RegisteredNMin && r.N <= RegisteredNMax
}

// SelectionReport is what selection did and to how many rows, in the
// corpus so a reader can check the rule was applied rather than trust it.
type SelectionReport struct {
	Rule          SelectionRule `json:"rule"`
	PreRegistered bool          `json:"pre_registered"`
	Statement     string        `json:"statement"`
	// Classes is the task-class subset a corpus was pre-registered with;
	// empty means every derived class.
	Classes    []string `json:"classes,omitempty"`
	Candidates int      `json:"candidates"`
	Selected   int      `json:"selected"`
	// Exclusions are counted, never silent: an instance the rule dropped
	// is part of the record of what the corpus is NOT.
	ExcludedNoGoldFiles   int      `json:"excluded_no_gold_files"`
	ExcludedTooManyFiles  int      `json:"excluded_too_many_files"`
	ExcludedDuplicate     int      `json:"excluded_duplicate"`
	ExcludedBeyondN       int      `json:"excluded_beyond_n"`
	UnparseableInstanceID []string `json:"unparseable,omitempty"`
}

// GoldFiles is the file_localization truth for one instance: the sorted,
// unique non-test Go source paths its gold patch modifies or deletes. An
// added file is not gold — it does not exist at base_commit, so no
// localization can find it. Without a patch, the line-level regions'
// paths stand in.
func GoldFiles(in Instance) ([]string, error) {
	seen := map[string]bool{}
	if in.Patch != "" {
		files, err := ParsePatch(in.Patch)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.Added || f.OldPath == "" || !IsGoSource(f.OldPath) {
				continue
			}
			seen[f.OldPath] = true
		}
	} else {
		for _, r := range in.Regions {
			if IsGoSource(r.Path) {
				seen[r.Path] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// Select applies rule to instances: dedupe, filter by gold-file count, sort
// by instance_id, take the first N. The input order is irrelevant to the
// result (TestDerivationIsDeterministic shuffles it).
func Select(instances []Instance, rule SelectionRule) ([]Instance, SelectionReport) {
	rep := SelectionReport{
		Rule:          rule,
		PreRegistered: rule.PreRegistered(),
		Statement: fmt.Sprintf("gold patch modifies or deletes %d–%d non-test Go source files present at base_commit; duplicates by (repo, base_commit, gold files) keep the source that sorts first; sort by instance_id; first %d",
			rule.MinFiles, rule.MaxFiles, rule.N),
		Candidates: len(instances),
	}

	type cand struct {
		in    Instance
		files []string
	}
	sorted := append([]Instance(nil), instances...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].InstanceID != sorted[j].InstanceID {
			return sorted[i].InstanceID < sorted[j].InstanceID
		}
		return sorted[i].Source < sorted[j].Source
	})

	// Duplicate detection runs over the source-sorted order so that, for
	// an instance both datasets carry, the winner is fixed by name and not
	// by which file the operator listed first.
	bySource := append([]Instance(nil), sorted...)
	sort.SliceStable(bySource, func(i, j int) bool {
		if bySource[i].Source != bySource[j].Source {
			return bySource[i].Source < bySource[j].Source
		}
		return bySource[i].InstanceID < bySource[j].InstanceID
	})
	dupKey := func(in Instance, files []string) string {
		return in.RepoURL() + "@" + in.BaseCommit + "#" + strings.Join(files, ",")
	}
	keep := map[string]bool{}
	seenKey := map[string]bool{}
	seenID := map[string]bool{}
	filesByID := map[string][]string{}
	for _, in := range bySource {
		files, err := GoldFiles(in)
		if err != nil {
			rep.UnparseableInstanceID = append(rep.UnparseableInstanceID, in.InstanceID)
			continue
		}
		filesByID[in.Source+"/"+in.InstanceID] = files
		key := dupKey(in, files)
		if seenID[in.InstanceID] || seenKey[key] {
			rep.ExcludedDuplicate++
			continue
		}
		seenID[in.InstanceID] = true
		seenKey[key] = true
		keep[in.Source+"/"+in.InstanceID] = true
	}
	sort.Strings(rep.UnparseableInstanceID)

	var eligible []cand
	for _, in := range sorted {
		k := in.Source + "/" + in.InstanceID
		if !keep[k] {
			continue
		}
		files := filesByID[k]
		switch {
		case len(files) < rule.MinFiles:
			rep.ExcludedNoGoldFiles++
		case len(files) > rule.MaxFiles:
			rep.ExcludedTooManyFiles++
		default:
			eligible = append(eligible, cand{in: in, files: files})
		}
	}
	if rule.N > 0 && len(eligible) > rule.N {
		rep.ExcludedBeyondN = len(eligible) - rule.N
		eligible = eligible[:rule.N]
	}
	out := make([]Instance, 0, len(eligible))
	for _, c := range eligible {
		out = append(out, c.in)
	}
	rep.Selected = len(out)
	return out, rep
}
