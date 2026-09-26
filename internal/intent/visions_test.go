package intent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// writeTree writes files (repo-relative path → body) under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// visionFixture is a small repo whose vision series exercises every stage
// and every finding kind the pipeline computes.
func visionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# Project intent — router\n\n## Docs map\n\n" +
			"| Question | Read |\n|---|---|\n" +
			"| Why this exists | `../design/product-vision.md` |\n" +
			"| Decisions, alternatives | `../../ADR.md` |\n" +
			"| Design records and the vision series | `../design/`, `../vision/README.md` (status per doc) |\n",
		"ADR.md": "# Decisions\n\n## ADR-001 — a thing\n\n**Revisit when.** never\n",
		"docs/plans/alpha.md": "# alpha\n\n" +
			"- [x] A-1 one {id: A-1, symbol: X, package: p}\n" +
			"- [ ] A-2 two {id: A-2}\n" +
			"- [-] A-3 three {id: A-3, disposition: deferred, reason: \"later\", trigger: \"x\"}\n",
		"docs/plans/archive/done.md": "# done\n\n- [x] D-1 d {id: D-1, symbol: Y, package: p}\n",
		"docs/vision/README.md": "# Visions\n\n**Status: FOR REVIEW.** Nothing here is binding.\n\n" +
			"| Question | Doc |\n|---|---|\n| Where? | [01](01-accepted.md) |\n\n" +
			"1. **[01-accepted.md](01-accepted.md)** — ACCEPTED 2026-09-01 (ADR-001).\n" +
			"2. **[02-review.md](02-review.md)** *(added 2026-06-01)* — the review one.\n" +
			"| 03 | [Planned](03-planned.md) | accepted 2026-09-20 (ADR-099) — planned |\n" +
			"- [04](04-bare.md) the bare one, rejected / accepted asks inside prose\n" +
			"- **[05](05-archived.md)** — see [the plan](../plans/work-gone.md)\n",
		"docs/vision/01-accepted.md": "# 01 — Accepted\n\nThe preamble; relates to platform-engineering ADR-0098.\n\n## 1. Section\n\n" +
			"A body [link](../plans/alpha.md) and a [bad one](../design/missing.md) and [web](https://example.com).\n",
		"docs/vision/02-review.md":  "# 02 — Review\n\n> Status: FOR REVIEW · Date: 2026-06-01\n\n## 1\n",
		"docs/vision/03-planned.md": "# 03 — Planned\n\nStatus: accepted 2026-09-20. Plan: `docs/plans/alpha.md`.\n\n## 1\n",
		"docs/vision/04-bare.md":    "# 04 — Bare\n\nNo status here, and no date.\n\n## 1\n",
		"docs/vision/05-archived.md": "# 05 — Archived\n\n> docs/plans/done.md is its binding promotion (2026-08-01).\n\n## 1\n\n" +
			"Revisit when: the plan reopens.\n",
	})
	return root
}

func visionByNumber(t *testing.T, vp intent.VisionPipeline, n string) intent.Vision {
	t.Helper()
	for _, v := range vp.Visions {
		if v.Number == n {
			return v
		}
	}
	t.Fatalf("vision %s not in the pipeline: %+v", n, vp.Visions)
	return intent.Vision{}
}

func findingsFor(vp intent.VisionPipeline, file, kind string) []intent.Claim {
	var out []intent.Claim
	for _, f := range vp.Findings {
		if f.Kind == kind && strings.HasPrefix(f.Ref, file) {
			out = append(out, f)
		}
	}
	return out
}

// TestVisionPipeline_StatusPlansLinksAndStaleness (DP-3): the promotion
// pipeline is COMPUTED — every vision's status/trigger or plan rows, whether
// its links resolve, and how old its newest dated statement is — from the
// vision home the router names. Nothing is asserted by hand, and nothing in
// a vision's prose is evaluated: only its status line, its plan and ADR
// pointers, its links, its dates, and any condition label its author wrote.
func TestVisionPipeline_StatusPlansLinksAndStaleness(t *testing.T) {
	root := visionFixture(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.VisionsHome != "docs/vision" {
		t.Fatalf("visions home = %q, want docs/vision (the router's docs map names it)", c.VisionsHome)
	}
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	vp := intent.LoadVisions(c, now)

	if vp.Index != "docs/vision/README.md" {
		t.Errorf("index = %q", vp.Index)
	}
	if len(vp.Visions) != 5 {
		t.Fatalf("got %d visions, want 5 (README.md is the index, not a vision)", len(vp.Visions))
	}
	for i := 1; i < len(vp.Visions); i++ {
		if vp.Visions[i-1].File >= vp.Visions[i].File {
			t.Fatalf("visions not in file order: %s then %s", vp.Visions[i-1].File, vp.Visions[i].File)
		}
	}

	// 01: status from the INDEX row (the doc has none); its ADR resolves;
	// a broken markdown link in the body is a finding with file:line.
	v1 := visionByNumber(t, vp, "01")
	if v1.Stage != intent.StageAccepted || !strings.HasPrefix(v1.Status, "ACCEPTED 2026-09-01") || v1.StatusFrom != "index" {
		t.Errorf("01: stage=%q status=%q from=%q, want accepted from the index row", v1.Stage, v1.Status, v1.StatusFrom)
	}
	if v1.Dated != "2026-09-01" || v1.AgeDays != 24 {
		t.Errorf("01: dated %q age %d, want 2026-09-01 / 24", v1.Dated, v1.AgeDays)
	}
	if got := findingsFor(vp, "docs/vision/01-accepted.md", "broken-link"); len(got) != 1 || !strings.Contains(got[0].Detail, "../design/missing.md") {
		t.Errorf("01: want exactly one broken-link finding for ../design/missing.md, got %+v", got)
	}
	// "platform-engineering ADR-0098" is another repository's numbering
	// (zero-padded wider than this home's ids): named as not checked, never
	// reported as a missing record of ours.
	if !strings.Contains(strings.Join(vp.Uncomputable, " "), "ADR-0098") {
		t.Errorf("a foreign-numbered decision id must be named as unchecked, got %v", vp.Uncomputable)
	}

	// 02: FOR REVIEW 116 days, no plan → stale, dated.
	v2 := visionByNumber(t, vp, "02")
	if v2.Stage != intent.StageForReview || v2.StatusFrom != "doc" {
		t.Errorf("02: stage=%q from=%q, want for-review from the doc's own Status line", v2.Stage, v2.StatusFrom)
	}
	if got := findingsFor(vp, "docs/vision/02-review.md", "stale"); len(got) != 1 || !strings.Contains(got[0].Detail, "116") {
		t.Errorf("02: want one stale finding naming 116 days, got %+v", got)
	}

	// 03: the doc's status wins over the index row's; its plan resolves and
	// its rows are counted statically; the index row's ADR-099 does not
	// resolve and is named.
	v3 := visionByNumber(t, vp, "03")
	if v3.Stage != intent.StagePlanned || v3.StatusFrom != "doc" {
		t.Errorf("03: stage=%q from=%q, want planned (a named plan with open rows)", v3.Stage, v3.StatusFrom)
	}
	if len(v3.Plans) != 1 || v3.Plans[0].Path != "docs/plans/alpha.md" {
		t.Fatalf("03: plans = %+v, want docs/plans/alpha.md", v3.Plans)
	}
	if p := v3.Plans[0]; p.Total != 3 || p.Claimed != 1 || p.Open != 1 || p.Skipped != 1 || p.Archived {
		t.Errorf("03: plan counts = %+v, want total 3, claimed 1, open 1, skipped 1", p)
	}
	if got := findingsFor(vp, "docs/vision/README.md", "broken-link"); !claimsMention(got, "ADR-099") {
		t.Errorf("03: the index row's ADR-099 names no record and must be a finding, got %+v", got)
	}

	// 04: names nothing — unstated AND undated, both named. Prose words in
	// its index row ("rejected / accepted asks") are NOT a status: only a
	// row that LEADS with a status word states one.
	v4 := visionByNumber(t, vp, "04")
	if v4.Stage != intent.StageUnstated || v4.Status != "" {
		t.Errorf("04: stage=%q status=%q, want unstated", v4.Stage, v4.Status)
	}
	if len(findingsFor(vp, "docs/vision/04-bare.md", "unstated")) != 1 || len(findingsFor(vp, "docs/vision/04-bare.md", "undated")) != 1 {
		t.Errorf("04: want one unstated and one undated finding, got %+v", vp.Findings)
	}

	// 05: a plain-text plan pointer to an ARCHIVED plan resolves (the plan
	// was proven and archived — the promise landed); a trigger is counted;
	// the index row's markdown link to a moved plan is still a broken link.
	v5 := visionByNumber(t, vp, "05")
	if v5.Stage != intent.StageLanded {
		t.Errorf("05: stage=%q, want landed (its only plan is archived)", v5.Stage)
	}
	if len(v5.Plans) != 1 || !v5.Plans[0].Archived || v5.Plans[0].Path != "docs/plans/archive/done.md" {
		t.Errorf("05: plans = %+v, want the archived docs/plans/archive/done.md", v5.Plans)
	}
	if len(v5.Triggers) != 1 || !strings.HasPrefix(v5.Triggers[0], "Revisit when") {
		t.Errorf("05: triggers = %v, want its Revisit when", v5.Triggers)
	}
	if got := findingsFor(vp, "docs/vision/README.md", "broken-link"); !claimsMention(got, "../plans/work-gone.md") {
		t.Errorf("05: the index row's dead plan link must be a finding, got %+v", got)
	}

	// Every finding carries a remedy — a finding without one is a complaint.
	for _, f := range vp.Findings {
		if f.Remedy == "" || f.Detail == "" || f.Ref == "" {
			t.Errorf("finding without ref/detail/remedy: %+v", f)
		}
	}
	if !vp.Coverage.Complete() {
		t.Errorf("a readable vision home is complete coverage")
	}
}

func claimsMention(cs []intent.Claim, s string) bool {
	for _, c := range cs {
		if strings.Contains(c.Detail, s) {
			return true
		}
	}
	return false
}

// TestVisionPipeline_UnreadableVisionIsNamedNotUnstated (DP-3, ADR-027): a
// vision the reader could not open is an instrument failure, not an
// unstated promise — it must never read as a finding about the author.
func TestVisionPipeline_UnreadableVisionIsNamedNotUnstated(t *testing.T) {
	root := visionFixture(t)
	p := filepath.Join(root, "docs", "vision", "04-bare.md")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	if _, err := os.ReadFile(p); err == nil {
		t.Skip("running with privileges that ignore file modes")
	}
	c, _ := intent.Load(root)
	vp := intent.LoadVisions(c, time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC))
	if vp.Coverage.Complete() {
		t.Fatal("an unreadable vision must make coverage incomplete")
	}
	if len(findingsFor(vp, "docs/vision/04-bare.md", "unstated")) != 0 {
		t.Error("an unreadable vision was reported as unstated — blindness read as a finding about the author")
	}
	if len(findingsFor(vp, "docs/vision/04-bare.md", "unreadable")) != 1 {
		t.Errorf("want one unreadable finding naming the file, got %+v", vp.Findings)
	}
}

// TestVisionPipeline_NoHomeIsANamedAbsence: a repo with no vision series is
// not a defect; the pipeline says so rather than reporting zero visions as
// a healthy pipeline.
func TestVisionPipeline_NoHomeIsANamedAbsence(t *testing.T) {
	c, _ := intent.Load(t.TempDir())
	vp := intent.LoadVisions(c, time.Now())
	if vp.Home != "" || len(vp.Visions) != 0 || vp.Absent == "" {
		t.Errorf("no vision home: want Home \"\", no visions, and a named absence; got %+v", vp)
	}
}

// TestVisionPipeline_RealSeries (DP-3) runs the pipeline over THIS repo's
// vision series. Floors, not equalities: ordinary editing must not break
// it, a shape change must.
func TestVisionPipeline_RealSeries(t *testing.T) {
	root := repoRoot(t)
	c, err := intent.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	vp := intent.LoadVisions(c, time.Now().UTC())
	if vp.Home != "docs/vision" || vp.Index != "docs/vision/README.md" {
		t.Fatalf("home=%q index=%q — the router's docs map names ../vision/README.md", vp.Home, vp.Index)
	}
	if len(vp.Visions) < 30 {
		t.Fatalf("parsed %d visions, want ≥30", len(vp.Visions))
	}
	stages := map[intent.VisionStage]int{}
	for _, v := range vp.Visions {
		stages[v.Stage]++
		if v.Title == "" {
			t.Errorf("%s has no title", v.File)
		}
	}
	if stages[intent.StageAccepted]+stages[intent.StagePlanned]+stages[intent.StageLanded] < 5 {
		t.Errorf("stages %v — fewer than five accepted/planned/landed visions means the status reader is not reading", stages)
	}
	if !vp.Coverage.Complete() {
		t.Errorf("coverage incomplete on the real series: %v", vp.Coverage.Unreadable())
	}
	t.Logf("stages %v, %d finding(s)", stages, len(vp.Findings))
}

// lockOut makes p (a file or a directory) unreadable for the rest of the
// test and restores mode after it; it skips when the process ignores file
// modes, as root does.
func lockOut(t *testing.T, p string, restore os.FileMode) {
	t.Helper()
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, restore) })
	if f, err := os.Open(p); err == nil {
		_ = f.Close()
		t.Skip("running with privileges that ignore file modes")
	}
}

var reviewNow = time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

// TestVisionPipeline_UnreadableHomeIsALowerBoundNotZero (ADR-027, I38): a
// vision home that exists and cannot be listed is blindness named on the
// manifest — never a confident zero visions with complete coverage, and
// never "no vision series".
func TestVisionPipeline_UnreadableHomeIsALowerBoundNotZero(t *testing.T) {
	root := visionFixture(t)
	lockOut(t, filepath.Join(root, "docs", "vision"), 0o755)
	c, _ := intent.Load(root)
	vp := intent.LoadVisions(c, reviewNow)
	if vp.Home != "docs/vision" || vp.Absent != "" {
		t.Errorf("home=%q absent=%q — the router names docs/vision; an unreadable home is not an absent one", vp.Home, vp.Absent)
	}
	if vp.Coverage.Complete() {
		t.Fatalf("an unlistable vision home must make coverage incomplete; got %d vision(s) with complete coverage", len(vp.Visions))
	}
	if !strings.Contains(strings.Join(vp.Uncomputable, " "), "docs/vision") {
		t.Errorf("the unlistable home must be named as not computed, got %v", vp.Uncomputable)
	}
	for _, f := range vp.Findings {
		if f.Kind == "unstated" || f.Kind == "broken-link" {
			t.Errorf("blindness reported as a finding about the author: %+v", f)
		}
	}
}

// TestVisionPipeline_UnreadableDecisionDirIsNotABrokenLink (ADR-027, I38): a
// decisions DIRECTORY that cannot be listed leaves cited ids unchecked and
// named — never every id "carried by no record".
func TestVisionPipeline_UnreadableDecisionDirIsNotABrokenLink(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# R\n\n## Docs map\n\n| Question | Read |\n|---|---|\n" +
			"| Decisions | `../adr/` |\n" +
			"| The vision series | `../vision/README.md` |\n",
		"docs/adr/001-thing.md": "# ADR-001 a thing\n",
		"docs/adr/002-other.md": "# ADR-002 another\n",
		"docs/vision/README.md": "# V\n",
		"docs/vision/01-a.md":   "# 01 — A\n\nStatus: accepted 2026-09-20 (ADR-001).\n\n## 1\n",
	})
	c, _ := intent.Load(root)
	if c.DecisionsHome != "docs/adr" {
		t.Fatalf("decisions home = %q, want docs/adr", c.DecisionsHome)
	}
	lockOut(t, filepath.Join(root, "docs", "adr"), 0o755)
	vp := intent.LoadVisions(c, reviewNow)
	for _, f := range vp.Findings {
		if strings.Contains(f.Detail, "ADR-001") {
			t.Errorf("an unlistable decisions home reported a cited id as missing: %+v", f)
		}
	}
	if vp.Coverage.Complete() {
		t.Error("an unlistable decisions home must make coverage incomplete")
	}
	if !strings.Contains(strings.Join(vp.Uncomputable, " "), "docs/adr") {
		t.Errorf("the unlistable decisions home must be named, got %v", vp.Uncomputable)
	}
}

// TestVisionPipeline_UnreadablePlanDirIsBlindNotBroken (ADR-027, I38): a
// plan or a link behind a directory the reader cannot search exists and
// could not be read — named on the manifest, the vision's stage unknown,
// never "not in this tree" and never a stage it did not earn.
func TestVisionPipeline_UnreadablePlanDirIsBlindNotBroken(t *testing.T) {
	root := visionFixture(t)
	lockOut(t, filepath.Join(root, "docs", "plans"), 0o755)
	c, _ := intent.Load(root)
	vp := intent.LoadVisions(c, reviewNow)
	for _, f := range vp.Findings {
		if f.Kind == "broken-link" && strings.Contains(f.Detail, "plans/") {
			t.Errorf("a plan behind an unreadable directory was reported absent: %+v", f)
		}
	}
	if vp.Coverage.Complete() {
		t.Error("an unsearchable plans directory must make coverage incomplete")
	}
	v3 := visionByNumber(t, vp, "03")
	if v3.Stage != intent.StageUnreadable {
		t.Errorf("03: stage=%q, want unreadable — its plan exists and could not be read, so where it stands is unknown", v3.Stage)
	}
	if got := findingsFor(vp, "docs/vision/03-planned.md", "unreadable"); len(got) != 1 || !strings.Contains(got[0].Detail, "docs/plans/alpha.md") {
		t.Errorf("03: want one unreadable finding naming docs/plans/alpha.md, got %+v", got)
	}
}

// TestVisionPipeline_LandedOnlyFromEvidence (vision 23 §5: fathom computes
// status or does not show it): `landed` is computed from the plans that
// CARRY a vision — declared as its plan — with no open row and claimed
// work. A status word, a plan whose rows are all deferred or refused, and a
// plan the vision merely cites or builds on are not that evidence.
func TestVisionPipeline_LandedOnlyFromEvidence(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# R\n\n## Docs map\n\n| Question | Read |\n|---|---|\n" +
			"| The vision series | `../vision/README.md` |\n",
		"docs/plans/allskipped.md": "# s\n\n" +
			"- [-] X-1 one {id: X-1, disposition: deferred, reason: \"later\", trigger: \"never\"}\n" +
			"- [-] X-2 two {id: X-2, disposition: refused, reason: \"no\"}\n",
		"docs/plans/done.md":    "# d\n\n- [x] D-1 d {id: D-1, symbol: Y, package: p}\n",
		"docs/vision/README.md": "# V\n",
		"docs/vision/01-a.md":   "# 01 — A\n\nStatus: accepted 2026-09-20. Plan: `docs/plans/allskipped.md`.\n\n## 1\n",
		"docs/vision/02-b.md":   "# 02 — B\n\nStatus: shipped 2026-09-20.\n\n## 1\n",
		"docs/vision/03-c.md": "# 03 — C\n\n**Status:** founding doc, owner-commissioned 2026-09-20.\n" +
			"**Builds on:** the shipped machinery, and the falsifiability\nrecord (docs/plans/done.md D-1).\n\n## 1\n",
		"docs/vision/04-d.md": "# 04 — D\n\n**Status:** Phase F is landed; Phases G–I are a founding doc, awaiting owner greenlight (2026-09-20).\n\n## 1\n",
		"docs/vision/05-e.md": "# 05 — E\n\nStatus: accepted 2026-09-20. Plan:\n`docs/plans/done.md`. Supersedes nothing; it closes the trigger\n`docs/plans/allskipped.md` X-1.\n\n## 1\n",
	})
	c, _ := intent.Load(root)
	vp := intent.LoadVisions(c, reviewNow)

	want := map[string]intent.VisionStage{
		"01": intent.StageAccepted,  // its only plan carries no claimed or open row
		"02": intent.StageStated,    // "shipped" with no plan: shown, never computed as landed
		"03": intent.StageForReview, // the plan it builds on is not its carrier
		"04": intent.StageForReview, // a prose "landed" is not a stage; "founding doc" is the stated one
		"05": intent.StageLanded,    // its declared plan is fully claimed; the plan it closes a trigger on is cited
	}
	for n, stage := range want {
		if v := visionByNumber(t, vp, n); v.Stage != stage {
			t.Errorf("%s: stage=%q, want %q (plans %+v, status %q)", n, v.Stage, stage, v.Plans, v.Status)
		}
	}
	if v := visionByNumber(t, vp, "03"); len(v.Plans) != 0 {
		t.Errorf("03: a plan named under `Builds on:` is cited, not the vision's carrier: %+v", v.Plans)
	}
	if v := visionByNumber(t, vp, "05"); len(v.Plans) != 1 || v.Plans[0].Path != "docs/plans/done.md" {
		t.Errorf("05: carriers = %+v, want only its declared plan docs/plans/done.md", v.Plans)
	}
}

// TestVisionPipeline_StaleNamesTheDecisionItCites: a for-review vision that
// cites a decision is never told it has "no decision" — the remedy names the
// decision it already cites.
func TestVisionPipeline_StaleNamesTheDecisionItCites(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# R\n\n## Docs map\n\n| Question | Read |\n|---|---|\n" +
			"| Decisions | `../../ADR.md` |\n| The vision series | `../vision/README.md` |\n",
		"ADR.md":                "# Decisions\n\n## ADR-001 — a thing\n",
		"docs/vision/README.md": "# V\n",
		"docs/vision/03-x.md":   "# 03 — X\n\nStatus: for review 2026-06-01. Accepted in ADR-001.\n\n## 1\n",
		"docs/vision/04-y.md":   "# 04 — Y\n\nStatus: for review 2026-06-01.\n\n## 1\n",
	})
	c, _ := intent.Load(root)
	vp := intent.LoadVisions(c, reviewNow)
	got := findingsFor(vp, "docs/vision/03-x.md", "stale")
	if len(got) != 1 {
		t.Fatalf("03: want one stale finding (for review 116 days, no plan), got %+v", vp.Findings)
	}
	if strings.Contains(got[0].Detail, "no decision") || !strings.Contains(got[0].Detail+got[0].Remedy, "ADR-001") {
		t.Errorf("03 cites ADR-001 — the finding must name it, never say \"no decision\": %+v", got[0])
	}
	if got := findingsFor(vp, "docs/vision/04-y.md", "stale"); len(got) != 1 || !strings.Contains(got[0].Detail, "no decision") {
		t.Errorf("04 cites nothing — \"no plan and no decision\" is true of it: %+v", got)
	}
}

// TestVisionPipeline_HomeOnlyFromTheRouter (ADR-026 amendment 2026-09-25,
// part 2): the pipeline reads the vision series the router's docs map
// names, and only that — a docs/vision/ the router does not route to is not
// read.
func TestVisionPipeline_HomeOnlyFromTheRouter(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# R\n\n## Docs map\n\n| Question | Read |\n|---|---|\n| Decisions | `../../ADR.md` |\n",
		"ADR.md":                "# Decisions\n\n## ADR-001 — a thing\n",
		"docs/vision/01-x.md":   "# 01 — X\n\nStatus: accepted 2026-09-01.\n\n## 1\n",
	})
	c, _ := intent.Load(root)
	vp := intent.LoadVisions(c, reviewNow)
	if vp.Home != "" || len(vp.Visions) != 0 || vp.Absent == "" {
		t.Errorf("the router routes to no vision series: want no home, no visions and a named absence; got home=%q visions=%d absent=%q",
			vp.Home, len(vp.Visions), vp.Absent)
	}
}

// TestVisionPipeline_RowNamingAVisionFirstStillFindsTheIndex: a docs-map
// row that names a starting vision before the index makes neither the
// vision the index nor the index a vision.
func TestVisionPipeline_RowNamingAVisionFirstStillFindsTheIndex(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# R\n\n## Docs map\n\n| Question | Read |\n|---|---|\n" +
			"| The vision series (start with the thesis) | `../vision/01-thesis.md`, then `../vision/README.md` |\n",
		"docs/vision/README.md":    "# Visions\n\nStatus: for review.\n",
		"docs/vision/01-thesis.md": "# 01 — Thesis\n\nStatus: accepted 2026-09-20.\n\n## 1\n",
		"docs/vision/02-b.md":      "# 02 — B\n\nStatus: accepted 2026-09-20.\n\n## 1\n",
	})
	c, _ := intent.Load(root)
	if c.VisionsHome != "docs/vision" || c.VisionsIndex != "docs/vision/README.md" {
		t.Fatalf("home=%q index=%q, want docs/vision with index docs/vision/README.md", c.VisionsHome, c.VisionsIndex)
	}
	vp := intent.LoadVisions(c, reviewNow)
	if len(vp.Visions) != 2 {
		t.Errorf("want 01 and 02 as visions, got %+v", vp.Visions)
	}
	visionByNumber(t, vp, "01")
	if got := findingsFor(vp, "docs/vision/README.md", "unstated"); len(got) != 0 {
		t.Errorf("the index was read as a vision: %+v", got)
	}
}

// TestVisionPipeline_DeadIndexPointerStillNamesTheSeries: a router that
// points at a vision index which is not there still routes to the vision
// directory — the series is read and the missing index is named, never
// "no vision series".
func TestVisionPipeline_DeadIndexPointerStillNamesTheSeries(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"docs/intent/README.md": "# R\n\n## Docs map\n\n| Question | Read |\n|---|---|\n" +
			"| The vision series | `../vision/README.md` |\n",
		"docs/vision/01-a.md": "# 01 — A\n\nStatus: accepted 2026-09-20.\n\n## 1\n",
	})
	c, _ := intent.Load(root)
	vp := intent.LoadVisions(c, reviewNow)
	if vp.Home != "docs/vision" || len(vp.Visions) != 1 {
		t.Fatalf("home=%q visions=%d absent=%q — the router names docs/vision through its (missing) index", vp.Home, len(vp.Visions), vp.Absent)
	}
	if !strings.Contains(strings.Join(vp.Uncomputable, " "), "docs/vision/README.md") {
		t.Errorf("the missing index must be named, got %v", vp.Uncomputable)
	}
}
