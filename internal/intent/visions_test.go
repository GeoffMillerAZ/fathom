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
