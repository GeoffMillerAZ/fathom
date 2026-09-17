// lca_bugloc_corpus_test.go is VB-27's own prep proof (docs/plans/verbs.md
// VB-27; vision 29 §3 row G, §5): testdata/scenariobench/lca-bugloc.json is
// a curated, committed selection of real Long Code Arena bug-localization
// records (docs/bench/scenarios.md "External corpus (LCA bug
// localization)") — Python, Java and Kotlin issues from small public
// repos, each with <= 3 changed files and a real textual bug report,
// selected 2026-09-16 so a full VB-27 run needs no fresh download and no
// re-derivation of the task set. This test proves the committed file
// actually parses and builds through the SAME adapter (LoadLCARecords +
// BuildLCACorpus) a real run uses, and that VB-27's own floor — n >= 3
// trials over >= 30 tasks (docs/vision/29-verbs-not-lookups.md §5) — is
// met with margin, never merely claimed.
package scenariobench

import "testing"

// lcaBuglocCorpusPath is committed under the repo-wide testdata/scenariobench/lca/
// subdirectory, not directly at testdata/scenariobench/ alongside the
// committed Corpus documents (public-go.json, reuse-go.json, guards-*.json,
// lookups-vs.json): this file is raw LCARecord JSONL (LoadLCARecords'
// own -dataset input), never a validated Corpus, and
// TestEveryScenarioTaskHasPredictions / TestCrossPackageTaskHasEnoughWorkerBudget
// (predictions_test.go) glob "testdata/scenariobench/*.json" expecting
// every match to parse as one — a flat placement here failed both
// (confirmed directly, 2026-09-16) exactly the way predictions_test.go's
// own comment already documents streams/*.json escaping that glob one
// directory deeper. Nor is this
// internal/scenariobench/testdata/lca/fixture.jsonl (the small synthetic
// fauxtown fixture RunExternalCorpus's own tests replay) — this is the
// operator's real, curated "-dataset" input for `scenariobench
// plan/run/ledger -corpus lca -dataset <path>`.
const lcaBuglocCorpusPath = "../../testdata/scenariobench/lca/lca-bugloc.json"

// lcaBuglocMinTasks is VB-27's own selection floor (docs/vision/29 §5's ">=
// 30 tasks"), plus the margin the prep task asked for so a couple of
// dropped/lost cells during the real run still clear the floor.
const lcaBuglocMinTasks = 32

func TestLCABuglocCorpus_LoadsAndBuilds(t *testing.T) {
	records, err := LoadLCARecords(lcaBuglocCorpusPath)
	if err != nil {
		t.Fatalf("LoadLCARecords(%s): %v", lcaBuglocCorpusPath, err)
	}
	if len(records) < lcaBuglocMinTasks {
		t.Fatalf("expected >= %d records, got %d", lcaBuglocMinTasks, len(records))
	}

	seenRepos := map[string]bool{}
	langs := map[string]int{}
	for _, rec := range records {
		if rec.RepoOwner == "" || rec.RepoName == "" {
			t.Errorf("record %s: missing repo_owner/repo_name", rec.TextID)
		}
		if rec.BaseSHA == "" {
			t.Errorf("record %s: missing base_sha", rec.TextID)
		}
		if len(rec.ChangedFiles) == 0 || len(rec.ChangedFiles) > 3 {
			t.Errorf("record %s: changed_files = %v, want 1-3 (the selection's own <= 3 filter)", rec.TextID, rec.ChangedFiles)
		}
		repo := rec.RepoOwner + "/" + rec.RepoName
		if seenRepos[repo] {
			t.Errorf("record %s: repo %s selected more than once", rec.TextID, repo)
		}
		seenRepos[repo] = true
		langs[rec.RepoLanguage]++
	}
	if langs["Python"] == 0 {
		t.Error("expected at least one Python task (the smoke cell's own language)")
	}

	corpus, err := BuildLCACorpus(records, lcaBuglocCorpusPath)
	if err != nil {
		t.Fatalf("BuildLCACorpus: %v", err)
	}
	if len(corpus.Tasks) != len(records) {
		t.Fatalf("expected %d tasks, got %d", len(records), len(corpus.Tasks))
	}

	fathomArm, ok := corpus.Arms[LCAArmFathom]
	if !ok {
		t.Fatal("expected a fathom arm")
	}
	if !fathomArm.Wired {
		t.Error("expected the fathom arm to be wired")
	}
	if fathomArm.Surface != "fathom" {
		t.Errorf("fathom arm Surface = %q, want \"fathom\" (the promoted surface, including localize/VB-26)", fathomArm.Surface)
	}
	controlArm, ok := corpus.Arms[LCAArmControl]
	if !ok {
		t.Fatal("expected a control arm")
	}
	if controlArm.Wired {
		t.Error("expected the control arm to be unwired")
	}
	if controlArm.Surface != "" {
		t.Errorf("control arm Surface = %q, want \"\" (no agent surface)", controlArm.Surface)
	}

	if corpus.Criterion.ID != LCACriterionID {
		t.Errorf("Criterion.ID = %q, want %q", corpus.Criterion.ID, LCACriterionID)
	}
	if corpus.Criterion.MinTrials != 3 {
		t.Errorf("Criterion.MinTrials = %d, want 3 (VB-27's own n >= 3 floor)", corpus.Criterion.MinTrials)
	}

	// The files oracle needs at least one gold file per task to score
	// anything at all — BuildLCACorpus already refuses a record with none
	// (LCATaskFromRecord), but this asserts it holds on every TASK the
	// committed file actually produced, not merely on the loader's own
	// refusal path.
	for _, task := range corpus.Tasks {
		if len(task.Gold.Files) == 0 {
			t.Errorf("task %s: oracle names no gold files", task.ID)
		}
		if !task.Oracle.Localization {
			t.Errorf("task %s: expected Oracle.Localization set", task.ID)
		}
		if task.Predictions[LCAArmFathom] == "" || task.Predictions[LCAArmControl] == "" {
			t.Errorf("task %s: missing a registered prediction for one arm", task.ID)
		}
	}

	if _, err := CanonicalCorpusSHA256(corpus); err != nil {
		t.Errorf("CanonicalCorpusSHA256: %v", err)
	}
}
