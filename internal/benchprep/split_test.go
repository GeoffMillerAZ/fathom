package benchprep

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/scenariobench"
)

const (
	splitRepoA = "https://github.com/owner/alpha"
	splitRepoB = "https://github.com/owner/beta"
	splitRepoC = "https://github.com/owner/gamma"
)

func splitTask(id, repo, mergedAt string) scenariobench.Task {
	return scenariobench.Task{ID: id, Subject: scenariobench.Subject{RepoURL: repo}, MergedAt: mergedAt}
}

func splitRepo(name, url, language, split, fresh string) RepoAssignment {
	return RepoAssignment{URL: url, Language: language, Split: split, FreshAfter: fresh, Subject: name}
}

func splitIDs(t *testing.T, r SplitResult, name string) []string {
	t.Helper()
	for _, s := range r.Splits {
		if s.Split == name {
			return s.IDs
		}
	}
	t.Fatalf("result has no %q split: %+v", name, r.Splits)
	return nil
}

func TestSplitHeldOutPrecedence(t *testing.T) {
	devByPath := scenariobench.Task{
		ID:         "D2",
		Subject:    scenariobench.Subject{RepoURL: splitRepoA},
		Derivation: "Copied verbatim from testdata/contextbench/tasks-home.json id H1-T01",
		MergedAt:   "2026-01-01T00:00:00Z",
	}
	repos := []RepoAssignment{
		splitRepo("alpha", splitRepoA, "go", "dev", "2026-03-10"),
		splitRepo("beta", splitRepoB, "go", "held-out", "2026-03-10"),
		splitRepo("nosplit", splitRepoC, "go", "", ""),
	}
	dev := []DevSplit{
		{Path: "testdata/contextbench/tasks-home.json", IDs: []string{"D1"}},
	}
	cases := []struct {
		name string
		task scenariobench.Task
		want string // split name, or "unassigned"
	}{
		{"declared dev by id", splitTask("D1", splitRepoA, "2026-01-01T00:00:00Z"), "dev"},
		{"declared dev by id beats a later merged_at", splitTask("D1", splitRepoA, "2026-06-01T00:00:00Z"), "dev"},
		{"declared dev by derivation path", devByPath, "dev"},
		{"a post-cutoff date on a dev repo stays dev", splitTask("P1", splitRepoA, "2026-03-11T00:00:00Z"), "dev"},
		{"post-cutoff on a held-out repo", splitTask("P2", splitRepoB, "2026-04-01T12:00:00Z"), "post-cutoff"},
		{"same day is not later", splitTask("S1", splitRepoB, "2026-03-10T23:59:59Z"), "held-out"},
		{"UTC next day from a local cutoff day is later", splitTask("U1", splitRepoB, "2026-03-10T23:30:00-05:00"), "post-cutoff"},
		{"UTC cutoff day from a local next day is not later", splitTask("U2", splitRepoB, "2026-03-11T01:00:00+05:00"), "held-out"},
		{"repo split dev", splitTask("R1", splitRepoA, "2026-01-01T00:00:00Z"), "dev"},
		{"repo split held-out", splitTask("R2", splitRepoB, "2026-01-01T00:00:00Z"), "held-out"},
		{"no manifest", splitTask("N1", "https://github.com/owner/unknown", "2026-01-01T00:00:00Z"), "unassigned"},
		{"manifest without split", splitTask("N2", splitRepoC, "2026-01-01T00:00:00Z"), "unassigned"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SplitHeldOut([]scenariobench.Task{tc.task}, repos, dev)
			if err != nil {
				t.Fatalf("SplitHeldOut: %v", err)
			}
			if tc.want == "unassigned" {
				if len(got.Unassigned) != 1 || got.Unassigned[0].ID != tc.task.ID {
					t.Fatalf("Unassigned = %+v, want exactly %s", got.Unassigned, tc.task.ID)
				}
				for _, name := range []string{"dev", "held-out", "post-cutoff"} {
					if ids := splitIDs(t, got, name); len(ids) != 0 {
						t.Fatalf("split %s = %v, want empty", name, ids)
					}
				}
				return
			}
			if len(got.Unassigned) != 0 {
				t.Fatalf("Unassigned = %+v, want none", got.Unassigned)
			}
			for _, name := range []string{"dev", "held-out", "post-cutoff"} {
				ids := splitIDs(t, got, name)
				if name == tc.want {
					if !reflect.DeepEqual(ids, []string{tc.task.ID}) {
						t.Fatalf("split %s = %v, want [%s]", name, ids, tc.task.ID)
					}
				} else if len(ids) != 0 {
					t.Fatalf("split %s = %v, want empty", name, ids)
				}
			}
		})
	}

	t.Run("unassigned reasons", func(t *testing.T) {
		got, err := SplitHeldOut([]scenariobench.Task{
			splitTask("N2", splitRepoC, ""),
			splitTask("N1", "git@github.com:Owner/Unknown.git", ""),
		}, repos, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		want := []UnassignedTask{
			{ID: "N1", Repo: "https://github.com/owner/unknown", Reason: "no manifest"},
			{ID: "N2", Repo: splitRepoC, Reason: "manifest has no split"},
		}
		if !reflect.DeepEqual(got.Unassigned, want) {
			t.Fatalf("Unassigned = %+v, want %+v", got.Unassigned, want)
		}
	})
}

func TestSplitHeldOutRefusals(t *testing.T) {
	repos := []RepoAssignment{
		splitRepo("alpha", splitRepoA, "go", "dev", ""),
		splitRepo("beta", splitRepoB, "go", "held-out", ""),
	}
	cases := []struct {
		name  string
		tasks []scenariobench.Task
		repos []RepoAssignment
		dev   []DevSplit
		want  []string // substrings the error must carry
	}{
		{
			"declared dev task on a held-out repo",
			[]scenariobench.Task{splitTask("D1", splitRepoB, "")}, repos,
			[]DevSplit{{Path: "x.json", IDs: []string{"D1"}}},
			[]string{"D1", splitRepoB},
		},
		{
			"derivation-declared dev task on a held-out repo",
			[]scenariobench.Task{{ID: "D9", Subject: scenariobench.Subject{RepoURL: splitRepoB}, Derivation: "from testdata/x.json id 1"}}, repos,
			[]DevSplit{{Path: "testdata/x.json", IDs: []string{"1"}}},
			[]string{"D9", splitRepoB},
		},
		{
			"malformed merged_at",
			[]scenariobench.Task{splitTask("M1", splitRepoA, "2026-13-45")}, repos, nil,
			[]string{"M1", "merged_at"},
		},
		{
			"malformed merged_at on a declared dev task",
			[]scenariobench.Task{splitTask("M2", splitRepoA, "yesterday")}, repos,
			[]DevSplit{{Path: "x.json", IDs: []string{"M2"}}},
			[]string{"M2", "merged_at"},
		},
		{
			"malformed fresh_after",
			nil, []RepoAssignment{splitRepo("alpha", splitRepoA, "go", "dev", "March 2026")}, nil,
			[]string{"alpha", "fresh_after"},
		},
		{
			"duplicate task id",
			[]scenariobench.Task{splitTask("T1", splitRepoA, ""), splitTask("T1", splitRepoB, "")}, repos, nil,
			[]string{"duplicate", "T1"},
		},
		{
			"empty task id",
			[]scenariobench.Task{splitTask("", splitRepoA, "")}, repos, nil,
			[]string{"empty task id"},
		},
		{
			"two assignments with the same normalised url",
			nil,
			[]RepoAssignment{
				splitRepo("one", "https://github.com/Owner/Alpha.git", "go", "dev", ""),
				splitRepo("two", "git@github.com:owner/alpha", "go", "held-out", ""),
			}, nil,
			[]string{"one", "two", splitRepoA},
		},
		{
			"unknown assignment split",
			nil, []RepoAssignment{splitRepo("alpha", splitRepoA, "go", "post-cutoff", "")}, nil,
			[]string{"alpha", "post-cutoff"},
		},
		{
			"empty assignment url",
			nil, []RepoAssignment{splitRepo("blank", "", "go", "dev", "")}, nil,
			[]string{"blank"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SplitHeldOut(tc.tasks, tc.repos, tc.dev)
			if err == nil {
				t.Fatalf("SplitHeldOut accepted the input, want a refusal naming %v", tc.want)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("error %q does not name %q", err, w)
				}
			}
		})
	}

	t.Run("dropped task with a malformed merged_at is excluded, not refused", func(t *testing.T) {
		task := splitTask("X1", splitRepoA, "garbage")
		task.Disposition = "dropped"
		got, err := SplitHeldOut([]scenariobench.Task{task}, repos, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		if !reflect.DeepEqual(got.Dropped, []string{"X1"}) || len(got.Undated) != 0 {
			t.Fatalf("Dropped = %v, Undated = %v", got.Dropped, got.Undated)
		}
	})
}

func TestSplitHeldOutDates(t *testing.T) {
	t.Run("empty merged_at takes the repo split and is undated", func(t *testing.T) {
		repos := []RepoAssignment{
			splitRepo("alpha", splitRepoA, "go", "dev", "2026-03-10"),
			splitRepo("beta", splitRepoB, "go", "held-out", "2026-03-10"),
		}
		got, err := SplitHeldOut([]scenariobench.Task{
			splitTask("B2", splitRepoB, ""),
			splitTask("A1", splitRepoA, ""),
			splitTask("B1", splitRepoB, "2026-01-01T00:00:00Z"),
		}, repos, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		if ids := splitIDs(t, got, "dev"); !reflect.DeepEqual(ids, []string{"A1"}) {
			t.Fatalf("dev = %v, want [A1]", ids)
		}
		if ids := splitIDs(t, got, "held-out"); !reflect.DeepEqual(ids, []string{"B1", "B2"}) {
			t.Fatalf("held-out = %v, want [B1 B2]", ids)
		}
		if !reflect.DeepEqual(got.Undated, []string{"A1", "B2"}) {
			t.Fatalf("Undated = %v, want [A1 B2]", got.Undated)
		}
	})

	t.Run("pending without any fresh_after", func(t *testing.T) {
		repos := []RepoAssignment{splitRepo("alpha", splitRepoA, "go", "held-out", "")}
		got, err := SplitHeldOut([]scenariobench.Task{splitTask("T1", splitRepoA, "2030-01-01T00:00:00Z")}, repos, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		if !got.PostCutoffPending {
			t.Fatalf("PostCutoffPending = false, want true with no fresh_after anywhere")
		}
		if ids := splitIDs(t, got, "post-cutoff"); len(ids) != 0 {
			t.Fatalf("post-cutoff = %v, want empty", ids)
		}
		if ids := splitIDs(t, got, "held-out"); !reflect.DeepEqual(ids, []string{"T1"}) {
			t.Fatalf("held-out = %v, want [T1]", ids)
		}
	})

	t.Run("not pending with one fresh_after", func(t *testing.T) {
		repos := []RepoAssignment{
			splitRepo("alpha", splitRepoA, "go", "held-out", ""),
			splitRepo("beta", splitRepoB, "go", "held-out", "2026-03-10"),
		}
		got, err := SplitHeldOut([]scenariobench.Task{splitTask("T1", splitRepoA, "2030-01-01T00:00:00Z")}, repos, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		if got.PostCutoffPending {
			t.Fatalf("PostCutoffPending = true, want false when an assignment carries fresh_after")
		}
		// alpha has no fresh_after of its own, so its task is never post-cutoff.
		if ids := splitIDs(t, got, "held-out"); !reflect.DeepEqual(ids, []string{"T1"}) {
			t.Fatalf("held-out = %v, want [T1]", ids)
		}
	})

	t.Run("no tasks and no repos is pending", func(t *testing.T) {
		got, err := SplitHeldOut(nil, nil, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		if !got.PostCutoffPending {
			t.Fatalf("PostCutoffPending = false, want true")
		}
	})
}

func TestSplitHeldOutURLIdentity(t *testing.T) {
	repos := []RepoAssignment{splitRepo("repo", "https://github.com/owner/repo", "go", "held-out", "")}
	got, err := SplitHeldOut([]scenariobench.Task{
		splitTask("T1", "git@github.com:Owner/Repo.git", ""),
		splitTask("T2", "https://GitHub.com/owner/repo/", ""),
	}, repos, nil)
	if err != nil {
		t.Fatalf("SplitHeldOut: %v", err)
	}
	if ids := splitIDs(t, got, "held-out"); !reflect.DeepEqual(ids, []string{"T1", "T2"}) {
		t.Fatalf("held-out = %v, want [T1 T2]", ids)
	}
	if len(got.Unassigned) != 0 {
		t.Fatalf("Unassigned = %+v, want none", got.Unassigned)
	}

	t.Run("assignment url normalised too", func(t *testing.T) {
		raw := []RepoAssignment{splitRepo("repo", "git@github.com:Owner/Repo.git", "go", "held-out", "")}
		got, err := SplitHeldOut([]scenariobench.Task{splitTask("T1", "https://github.com/owner/repo", "")}, raw, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		if ids := splitIDs(t, got, "held-out"); !reflect.DeepEqual(ids, []string{"T1"}) {
			t.Fatalf("held-out = %v, want [T1]", ids)
		}
		if got.Languages[0].Language != "go" || !reflect.DeepEqual(got.Languages[0].HeldOut, []string{"https://github.com/owner/repo"}) {
			t.Fatalf("Languages[go] = %+v, want the normalised url", got.Languages[0])
		}
	})
}

// splitFleet is 3 dev + 3 held-out repositories for each P1 language.
func splitFleet() []RepoAssignment {
	var repos []RepoAssignment
	for _, lang := range []string{"go", "python", "typescript"} {
		for _, split := range []string{"dev", "held-out"} {
			for _, n := range []string{"1", "2", "3"} {
				name := lang + "-" + split + "-" + n
				repos = append(repos, splitRepo(name, "https://github.com/fleet/"+name, lang, split, ""))
			}
		}
	}
	return repos
}

func TestSplitHeldOutLanguagesAndShortfalls(t *testing.T) {
	t.Run("full fleet has no shortfall", func(t *testing.T) {
		got, err := SplitHeldOut(nil, splitFleet(), nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		if len(got.Shortfalls) != 0 {
			t.Fatalf("Shortfalls = %v, want none", got.Shortfalls)
		}
		var order []string
		for _, l := range got.Languages {
			order = append(order, l.Language)
			if len(l.Dev) != 3 || len(l.HeldOut) != 3 {
				t.Fatalf("%s dev %d held-out %d, want 3 and 3", l.Language, len(l.Dev), len(l.HeldOut))
			}
		}
		if !reflect.DeepEqual(order, []string{"go", "python", "typescript"}) {
			t.Fatalf("Languages order = %v", order)
		}
	})

	t.Run("counts come from assignments even with no tasks", func(t *testing.T) {
		var repos []RepoAssignment
		for _, r := range splitFleet() {
			switch r.Subject {
			case "go-held-out-2", "go-held-out-3", "python-dev-3":
				continue
			}
			repos = append(repos, r)
		}
		repos = append(repos,
			splitRepo("rust-dev", "https://github.com/fleet/rust-dev", "rust", "dev", ""),
			splitRepo("blank-lang", "https://github.com/fleet/blank-lang", "", "held-out", ""),
		)
		got, err := SplitHeldOut(nil, repos, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		want := []string{"go: 1 of 3 held-out repositories", "python: 2 of 3 dev repositories"}
		if !reflect.DeepEqual(got.Shortfalls, want) {
			t.Fatalf("Shortfalls = %v, want %v", got.Shortfalls, want)
		}
		if len(got.Languages) != 3 {
			t.Fatalf("Languages = %+v, want exactly go, python, typescript", got.Languages)
		}
	})

	t.Run("an absent language is a shortfall of both", func(t *testing.T) {
		got, err := SplitHeldOut(nil, nil, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		want := []string{
			"go: 0 of 3 dev repositories", "go: 0 of 3 held-out repositories",
			"python: 0 of 3 dev repositories", "python: 0 of 3 held-out repositories",
			"typescript: 0 of 3 dev repositories", "typescript: 0 of 3 held-out repositories",
		}
		if !reflect.DeepEqual(got.Shortfalls, want) {
			t.Fatalf("Shortfalls = %v, want %v", got.Shortfalls, want)
		}
	})

	t.Run("an assignment without a split counts for neither", func(t *testing.T) {
		repos := append(splitFleet()[:5:5], splitRepo("go-nosplit", "https://github.com/fleet/go-nosplit", "go", "", ""))
		got, err := SplitHeldOut(nil, repos, nil)
		if err != nil {
			t.Fatalf("SplitHeldOut: %v", err)
		}
		if got.Languages[0].Language != "go" || len(got.Languages[0].Dev) != 3 || len(got.Languages[0].HeldOut) != 2 {
			t.Fatalf("Languages[go] = %+v", got.Languages[0])
		}
	})
}

func TestAssignmentsFromSubjects(t *testing.T) {
	subject := func(name, url, language, split, fresh string) Subject {
		s := Subject{Name: name, URL: url}
		if language != "" {
			s.Bench.Toolchain.Versions = map[string]string{language: "1"}
		}
		s.Bench.Split = split
		s.Bench.Contamination.FreshAfter = fresh
		return s
	}

	t.Run("maps fields and node to typescript, sorted by url", func(t *testing.T) {
		got, err := AssignmentsFromSubjects([]Subject{
			subject("zeta", "git@github.com:Org/Zeta.git", "node", "held-out", "2026-05-01"),
			subject("alpha", "https://github.com/org/alpha/", "go", "dev", ""),
			subject("mid", "https://github.com/org/mid", "python", "", ""),
			subject("none", "https://github.com/org/none", "", "dev", ""),
		})
		if err != nil {
			t.Fatalf("AssignmentsFromSubjects: %v", err)
		}
		want := []RepoAssignment{
			{URL: "https://github.com/org/alpha", Language: "go", Split: "dev", Subject: "alpha"},
			{URL: "https://github.com/org/mid", Language: "python", Subject: "mid"},
			{URL: "https://github.com/org/none", Language: "", Split: "dev", Subject: "none"},
			{URL: "https://github.com/org/zeta", Language: "typescript", Split: "held-out", FreshAfter: "2026-05-01", Subject: "zeta"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("AssignmentsFromSubjects =\n%+v\nwant\n%+v", got, want)
		}
	})

	t.Run("go wins over python over node", func(t *testing.T) {
		s := subject("multi", "https://github.com/org/multi", "node", "dev", "")
		s.Bench.Toolchain.Versions["python"] = "3"
		got, err := AssignmentsFromSubjects([]Subject{s})
		if err != nil {
			t.Fatalf("AssignmentsFromSubjects: %v", err)
		}
		if got[0].Language != "python" {
			t.Fatalf("Language = %q, want python", got[0].Language)
		}
	})

	t.Run("duplicate normalised url refused naming both", func(t *testing.T) {
		_, err := AssignmentsFromSubjects([]Subject{
			subject("second", "git@github.com:org/dup.git", "go", "dev", ""),
			subject("first", "https://github.com/Org/Dup", "go", "held-out", ""),
		})
		if err == nil {
			t.Fatalf("AssignmentsFromSubjects accepted a duplicate normalised url")
		}
		for _, w := range []string{"first", "second", "https://github.com/org/dup"} {
			if !strings.Contains(err.Error(), w) {
				t.Fatalf("error %q does not name %q", err, w)
			}
		}
	})

	t.Run("empty input yields an empty non-nil slice", func(t *testing.T) {
		got, err := AssignmentsFromSubjects(nil)
		if err != nil {
			t.Fatalf("AssignmentsFromSubjects: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Fatalf("AssignmentsFromSubjects(nil) = %#v, want an empty non-nil slice", got)
		}
	})
}

func TestSplitHeldOutDroppedAndHashes(t *testing.T) {
	repos := []RepoAssignment{
		splitRepo("alpha", splitRepoA, "go", "dev", ""),
		splitRepo("beta", splitRepoB, "go", "held-out", ""),
	}
	dropped := splitTask("X1", splitRepoB, "")
	dropped.Disposition = "dropped"
	dropped.Reason = "oracle cannot be won"
	tasks := []scenariobench.Task{
		splitTask("B2", splitRepoB, ""),
		dropped,
		splitTask("A1", splitRepoA, ""),
		splitTask("B1", splitRepoB, ""),
	}
	got, err := SplitHeldOut(tasks, repos, nil)
	if err != nil {
		t.Fatalf("SplitHeldOut: %v", err)
	}
	if !reflect.DeepEqual(got.Dropped, []string{"X1"}) {
		t.Fatalf("Dropped = %v, want [X1]", got.Dropped)
	}
	for _, s := range got.Splits {
		for _, id := range s.IDs {
			if id == "X1" {
				t.Fatalf("dropped task X1 is in split %s", s.Split)
			}
		}
	}
	if !reflect.DeepEqual(got.Undated, []string{"A1", "B1", "B2"}) { // X1 is dropped, so not undated
		t.Fatalf("Undated = %v, want [A1 B1 B2]", got.Undated)
	}

	var names []string
	for _, s := range got.Splits {
		names = append(names, s.Split)
		if len(s.IDs) == 0 {
			if s.SHA256 != "" || s.IDs == nil {
				t.Fatalf("empty split %s = %+v, want [] and no sha", s.Split, s)
			}
			continue
		}
		want, err := SplitSHA256(s.IDs)
		if err != nil {
			t.Fatalf("SplitSHA256(%v): %v", s.IDs, err)
		}
		if s.SHA256 != want {
			t.Fatalf("split %s sha = %s, want %s", s.Split, s.SHA256, want)
		}
	}
	if !reflect.DeepEqual(names, []string{"dev", "held-out", "post-cutoff"}) {
		t.Fatalf("split order = %v", names)
	}
	if ids := splitIDs(t, got, "held-out"); !reflect.DeepEqual(ids, []string{"B1", "B2"}) {
		t.Fatalf("held-out = %v, want [B1 B2]", ids)
	}
}

func TestSplitHeldOutEmptyJSON(t *testing.T) {
	got, err := SplitHeldOut(nil, nil, nil)
	if err != nil {
		t.Fatalf("SplitHeldOut: %v", err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "null") {
		t.Fatalf("JSON contains null: %s", text)
	}
	for _, want := range []string{
		`"ids":[]`, `"unassigned":[]`, `"undated":[]`, `"dropped":[]`,
		`"post_cutoff_pending":true`, `"dev":[]`, `"held_out":[]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("JSON lacks %s: %s", want, text)
		}
	}
	if strings.Contains(text, `"sha256"`) {
		t.Fatalf("an empty split must omit sha256: %s", text)
	}
}

func TestSplitHeldOutOrderIndependence(t *testing.T) {
	repos := append(splitFleet(),
		splitRepo("alpha", splitRepoA, "go", "dev", "2026-03-10"),
		splitRepo("beta", splitRepoB, "go", "held-out", "2026-03-10"),
		splitRepo("gamma", splitRepoC, "go", "", ""),
	)
	dev := []DevSplit{
		{Path: "testdata/a.json", IDs: []string{"D1", "D2"}},
		{Path: "testdata/b.json", IDs: []string{"D3"}},
	}
	dropped := splitTask("X1", splitRepoB, "")
	dropped.Disposition = "dropped"
	viaPath := splitTask("D4", splitRepoA, "2026-01-01T00:00:00Z")
	viaPath.Derivation = "Copied verbatim from testdata/b.json id 4"
	tasks := []scenariobench.Task{
		splitTask("D1", splitRepoA, "2026-01-01T00:00:00Z"),
		splitTask("D2", splitRepoA, ""),
		splitTask("D3", splitRepoA, "2026-09-01T00:00:00Z"),
		viaPath,
		splitTask("H1", splitRepoB, "2026-02-01T00:00:00Z"),
		splitTask("H2", splitRepoB, ""),
		splitTask("P1", splitRepoB, "2026-04-01T00:00:00Z"),
		splitTask("P2", "git@github.com:Owner/Alpha.git", "2026-04-01T00:00:00Z"),
		splitTask("U1", splitRepoC, "2026-02-01T00:00:00Z"),
		splitTask("U2", "https://github.com/owner/unknown", ""),
		dropped,
	}
	want, err := SplitHeldOut(tasks, repos, dev)
	if err != nil {
		t.Fatalf("SplitHeldOut: %v", err)
	}
	if ids := splitIDs(t, want, "dev"); !reflect.DeepEqual(ids, []string{"D1", "D2", "D3", "D4", "P2"}) {
		t.Fatalf("fixture dev = %v", ids)
	}
	if ids := splitIDs(t, want, "post-cutoff"); !reflect.DeepEqual(ids, []string{"P1"}) {
		t.Fatalf("fixture post-cutoff = %v", ids)
	}

	rng := rand.New(rand.NewSource(7))
	for round := 0; round < 25; round++ {
		st := append([]scenariobench.Task(nil), tasks...)
		sr := append([]RepoAssignment(nil), repos...)
		sd := append([]DevSplit(nil), dev...)
		rng.Shuffle(len(st), func(i, j int) { st[i], st[j] = st[j], st[i] })
		rng.Shuffle(len(sr), func(i, j int) { sr[i], sr[j] = sr[j], sr[i] })
		rng.Shuffle(len(sd), func(i, j int) { sd[i], sd[j] = sd[j], sd[i] })
		got, err := SplitHeldOut(st, sr, sd)
		if err != nil {
			t.Fatalf("round %d: SplitHeldOut: %v", round, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: result depends on input order:\n got %+v\nwant %+v", round, got, want)
		}
	}
}

func TestSplitHeldOutDoesNotMutateInputs(t *testing.T) {
	tasks := []scenariobench.Task{splitTask("B", splitRepoA, ""), splitTask("A", splitRepoA, "")}
	repos := []RepoAssignment{
		splitRepo("b", splitRepoB, "go", "dev", ""),
		splitRepo("a", "git@github.com:Owner/Alpha.git", "go", "dev", ""),
	}
	dev := []DevSplit{{Path: "p.json", IDs: []string{"Z", "Y"}}}
	if _, err := SplitHeldOut(tasks, repos, dev); err != nil {
		t.Fatalf("SplitHeldOut: %v", err)
	}
	if tasks[0].ID != "B" || repos[0].Subject != "b" || repos[1].URL != "git@github.com:Owner/Alpha.git" || dev[0].IDs[0] != "Z" {
		t.Fatalf("SplitHeldOut mutated its inputs: %+v %+v %+v", tasks, repos, dev)
	}
}
