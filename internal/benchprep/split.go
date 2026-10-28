package benchprep

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

// Split names SplitHeldOut assigns tasks to, and the values an assignment's
// Split may carry (the first two; "" means the manifest carries no split).
const (
	splitNameDev        = "dev"
	splitNameHeldOut    = "held-out"
	splitNamePostCutoff = "post-cutoff"
)

// splitTargetRepos is the number of dev and of held-out repositories each P1
// language is asked for.
const splitTargetRepos = 3

// splitLanguages are the P1 languages, in the order LanguageRepos reports them.
var splitLanguages = []string{"go", "python", "typescript"}

// RepoAssignment is one repository's split data, passed in as data.
type RepoAssignment struct {
	URL        string // normalised (NormaliseRepoURL); SplitHeldOut normalises again (idempotent)
	Language   string // "go", "python", "typescript", or another/"" (not a P1 language)
	Split      string // "dev", "held-out", or "" (the manifest carries no split)
	FreshAfter string // "" or a date 2006-01-02 (bench.contamination.fresh_after)
	Subject    string // the manifest name, for messages
}

// SplitSet is one split of the task corpus: its sorted ids and their identity.
type SplitSet struct {
	Split  string   `json:"split"`            // "dev" | "held-out" | "post-cutoff"
	IDs    []string `json:"ids"`              // sorted; [] (not null) when empty
	SHA256 string   `json:"sha256,omitempty"` // SplitSHA256(IDs); "" when the split is empty
}

// UnassignedTask is a task that belongs to no split because its repository's
// manifest is missing or carries no split.
type UnassignedTask struct {
	ID     string `json:"id"`
	Repo   string `json:"repo"`   // normalised
	Reason string `json:"reason"` // "no manifest" | "manifest has no split"
}

// LanguageRepos lists one P1 language's dev and held-out repositories, read
// from the assignments (the manifests), never from the tasks.
type LanguageRepos struct {
	Language string   `json:"language"` // go, python, typescript (always all three, in that order)
	Dev      []string `json:"dev"`      // normalised repo URLs, sorted
	HeldOut  []string `json:"held_out"`
}

// SplitResult is the computed split. Every slice is non-nil, so its JSON prints
// [] and never null.
type SplitResult struct {
	Splits            []SplitSet       `json:"splits"`              // exactly dev, held-out, post-cutoff, in that order
	Unassigned        []UnassignedTask `json:"unassigned"`          // sorted by ID
	Undated           []string         `json:"undated"`             // task ids with no merged_at, sorted
	Dropped           []string         `json:"dropped"`             // task ids with Disposition "dropped": excluded from every split, named
	PostCutoffPending bool             `json:"post_cutoff_pending"` // no assignment carries fresh_after: the slice reads "pending Q2"
	Languages         []LanguageRepos  `json:"languages"`
	Shortfalls        []string         `json:"shortfalls"` // e.g. "go: 1 of 3 held-out repositories"; sorted
}

// AssignmentsFromSubjects builds one RepoAssignment per manifest: URL through
// NormaliseRepoURL, Language from primaryLanguage mapped to the P1 name ("node"
// becomes "typescript", because vet's languageExtensions reads node as
// TypeScript; "go" and "python" are unchanged), Split and FreshAfter from the
// manifest's bench block, Subject from the manifest name. It refuses two
// manifests with the same normalised URL, naming both subjects and the URL. The
// output is sorted by URL (then subject name) and is non-nil; the input slice is
// not modified.
func AssignmentsFromSubjects(subjects []Subject) ([]RepoAssignment, error) {
	out := make([]RepoAssignment, 0, len(subjects))
	for _, s := range subjects {
		language := primaryLanguage(s)
		if language == "node" {
			language = "typescript"
		}
		out = append(out, RepoAssignment{
			URL:        NormaliseRepoURL(s.URL),
			Language:   language,
			Split:      s.Bench.Split,
			FreshAfter: s.Bench.Contamination.FreshAfter,
			Subject:    s.Name,
		})
	}
	sortAssignments(out)
	for i := 1; i < len(out); i++ {
		if out[i].URL == out[i-1].URL {
			return nil, fmt.Errorf("split: subjects %q and %q share the repository %s", out[i-1].Subject, out[i].Subject, out[i].URL)
		}
	}
	return out, nil
}

func sortAssignments(a []RepoAssignment) {
	sort.SliceStable(a, func(i, j int) bool {
		if a[i].URL != a[j].URL {
			return a[i].URL < a[j].URL
		}
		return a[i].Subject < a[j].Subject
	})
}

// SplitHeldOut computes the dev, held-out and post-cutoff task splits (BX-54).
// Every non-dropped task lands in exactly one split or in Unassigned. The task's
// repository is its Subject.RepoURL and every repository URL (tasks and
// assignments alike) is compared through NormaliseRepoURL. Precedence, first
// match wins:
//
//  1. Declared dev: the task id is in any dev[i].IDs, or the task's Derivation
//     contains any non-empty dev[i].Path as a substring (lookups-vs tasks say
//     "Copied verbatim from testdata/contextbench/tasks-home.json id H1-T01").
//     The task is dev. Dev wins over a post-cutoff date: a declared dev task is
//     never post-cutoff.
//  2. Post-cutoff: the repository's split is not dev, and the UTC calendar
//     date of merged_at (time.RFC3339, then .UTC(), compared as 2006-01-02) is
//     later than its fresh_after. The same day is not later. A dev repository
//     contributes no post-cutoff task: fathom tunes on it (D25), so a fresh
//     task there is new to the model but not to fathom, and stays dev.
//  3. Repository split: the task takes its repository's assignment ("dev" or
//     "held-out").
//  4. Unassigned: the repository has no assignment ("no manifest") or its
//     assignment has Split "" ("manifest has no split"). Such a task is counted
//     and named in Unassigned, in no split.
//
// A task with Disposition "dropped" is excluded from every split and named in
// Dropped; it is still checked for an empty or duplicate id, but its merged_at
// is not read. A task with an empty merged_at takes its repository's split
// (rules 3 and 4) and is listed in Undated. PostCutoffPending is true when no
// assignment carries fresh_after: the post-cutoff slice is then empty and reads
// "pending Q2".
//
// It refuses, with an error naming the offender: an empty task id; a duplicate
// task id; a malformed merged_at (never counted as undated), including on a
// declared dev task; a declared dev task whose repository's assignment is
// held-out (naming the task id and the repository); an assignment whose Split is
// not "", "dev" or "held-out", whose FreshAfter is not "" or a 2006-01-02 date
// (naming the subject), or whose URL is empty; and two assignments with the same
// normalised URL.
//
// Per P1 language (go, python, typescript) the dev and held-out repository
// counts come from the assignments, not the tasks, against the target of 3 dev
// and 3 held-out. Each missing count is one Shortfalls line, "<language>: <n> of
// 3 dev repositories" or "<language>: <n> of 3 held-out repositories". A
// shortfall is named, never refused; the controller decides at BX-11 and BX-12.
//
// SplitHeldOut computes and never commits: it writes no file and records
// nothing. The controller's "Held-out split sha256:" record lines are BX-11. Each
// split's ids are sorted and its SHA256 is SplitSHA256(ids) when non-empty.
// Output does not depend on input order, the inputs are not modified, and there
// is no clock, network, git or shell.
func SplitHeldOut(tasks []scenariobench.Task, repos []RepoAssignment, dev []DevSplit) (SplitResult, error) {
	assignments, err := indexAssignments(repos)
	if err != nil {
		return SplitResult{}, err
	}
	anyFresh := false
	for _, a := range assignments.list {
		if a.FreshAfter != "" {
			anyFresh = true
		}
	}

	devIDs := map[string]bool{}
	var devPaths []string
	for _, d := range dev {
		for _, id := range d.IDs {
			devIDs[id] = true
		}
		if d.Path != "" {
			devPaths = append(devPaths, d.Path)
		}
	}

	ordered := append([]scenariobench.Task(nil), tasks...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	result := SplitResult{
		Unassigned:        []UnassignedTask{},
		Undated:           []string{},
		Dropped:           []string{},
		PostCutoffPending: !anyFresh,
	}
	buckets := map[string][]string{splitNameDev: {}, splitNameHeldOut: {}, splitNamePostCutoff: {}}

	for i, task := range ordered {
		switch {
		case task.ID == "":
			return SplitResult{}, fmt.Errorf("split: a task has an empty task id")
		case i > 0 && ordered[i-1].ID == task.ID:
			return SplitResult{}, fmt.Errorf("split: duplicate task id %q", task.ID)
		}
		if task.Disposition == "dropped" {
			result.Dropped = append(result.Dropped, task.ID)
			continue
		}

		var mergedDate string
		if task.MergedAt == "" {
			result.Undated = append(result.Undated, task.ID)
		} else {
			merged, err := time.Parse(time.RFC3339, task.MergedAt)
			if err != nil {
				return SplitResult{}, fmt.Errorf("split: task %q has a malformed merged_at %q: %v", task.ID, task.MergedAt, err)
			}
			mergedDate = merged.UTC().Format("2006-01-02")
		}

		repo := NormaliseRepoURL(task.Subject.RepoURL)
		assignment, assigned := assignments.byURL[repo]

		declared := devIDs[task.ID]
		for _, p := range devPaths {
			declared = declared || strings.Contains(task.Derivation, p)
		}
		if declared {
			if assigned && assignment.Split == splitNameHeldOut {
				return SplitResult{}, fmt.Errorf("split: task %q is declared dev but its repository %s is held-out (subject %q)", task.ID, repo, assignment.Subject)
			}
			buckets[splitNameDev] = append(buckets[splitNameDev], task.ID)
			continue
		}
		if assigned && assignment.Split != splitNameDev && assignment.FreshAfter != "" && mergedDate != "" && mergedDate > assignment.FreshAfter {
			buckets[splitNamePostCutoff] = append(buckets[splitNamePostCutoff], task.ID)
			continue
		}
		switch {
		case !assigned:
			result.Unassigned = append(result.Unassigned, UnassignedTask{ID: task.ID, Repo: repo, Reason: "no manifest"})
		case assignment.Split == "":
			result.Unassigned = append(result.Unassigned, UnassignedTask{ID: task.ID, Repo: repo, Reason: "manifest has no split"})
		default:
			buckets[assignment.Split] = append(buckets[assignment.Split], task.ID)
		}
	}

	for _, name := range []string{splitNameDev, splitNameHeldOut, splitNamePostCutoff} {
		ids := buckets[name]
		sort.Strings(ids)
		set := SplitSet{Split: name, IDs: ids}
		if len(ids) > 0 {
			sha, err := SplitSHA256(ids)
			if err != nil {
				return SplitResult{}, fmt.Errorf("split: %s split: %w", name, err)
			}
			set.SHA256 = sha
		}
		result.Splits = append(result.Splits, set)
	}
	sort.Strings(result.Undated)
	sort.Strings(result.Dropped)
	sort.SliceStable(result.Unassigned, func(i, j int) bool { return result.Unassigned[i].ID < result.Unassigned[j].ID })

	result.Languages = []LanguageRepos{}
	result.Shortfalls = []string{}
	for _, language := range splitLanguages {
		entry := LanguageRepos{Language: language, Dev: []string{}, HeldOut: []string{}}
		for _, a := range assignments.list {
			if a.Language != language {
				continue
			}
			switch a.Split {
			case splitNameDev:
				entry.Dev = append(entry.Dev, a.URL)
			case splitNameHeldOut:
				entry.HeldOut = append(entry.HeldOut, a.URL)
			}
		}
		sort.Strings(entry.Dev)
		sort.Strings(entry.HeldOut)
		if n := len(entry.Dev); n < splitTargetRepos {
			result.Shortfalls = append(result.Shortfalls, fmt.Sprintf("%s: %d of %d dev repositories", language, n, splitTargetRepos))
		}
		if n := len(entry.HeldOut); n < splitTargetRepos {
			result.Shortfalls = append(result.Shortfalls, fmt.Sprintf("%s: %d of %d held-out repositories", language, n, splitTargetRepos))
		}
		result.Languages = append(result.Languages, entry)
	}
	sort.Strings(result.Shortfalls)
	return result, nil
}

// assignmentIndex is the validated, normalised, sorted assignment set.
type assignmentIndex struct {
	list  []RepoAssignment
	byURL map[string]RepoAssignment
}

// indexAssignments validates every assignment, normalises its URL through
// NormaliseRepoURL and refuses a duplicate normalised URL. It works on a sorted
// copy, so the error reported does not depend on input order.
func indexAssignments(repos []RepoAssignment) (assignmentIndex, error) {
	list := make([]RepoAssignment, 0, len(repos))
	for _, r := range repos {
		r.URL = NormaliseRepoURL(r.URL)
		list = append(list, r)
	}
	sortAssignments(list)
	idx := assignmentIndex{list: list, byURL: make(map[string]RepoAssignment, len(list))}
	for _, a := range list {
		label := a.Subject
		if label == "" {
			label = a.URL
		}
		switch {
		case a.URL == "":
			return assignmentIndex{}, fmt.Errorf("split: subject %q has an empty repository url", label)
		case a.Split != "" && a.Split != splitNameDev && a.Split != splitNameHeldOut:
			return assignmentIndex{}, fmt.Errorf("split: subject %q has split %q; want \"\", %q or %q", label, a.Split, splitNameDev, splitNameHeldOut)
		}
		if a.FreshAfter != "" {
			if _, err := time.Parse("2006-01-02", a.FreshAfter); err != nil {
				return assignmentIndex{}, fmt.Errorf("split: subject %q has a malformed fresh_after %q (want 2006-01-02): %v", label, a.FreshAfter, err)
			}
		}
		if prev, dup := idx.byURL[a.URL]; dup {
			return assignmentIndex{}, fmt.Errorf("split: subjects %q and %q share the repository %s", prev.Subject, a.Subject, a.URL)
		}
		idx.byURL[a.URL] = a
	}
	return idx, nil
}
