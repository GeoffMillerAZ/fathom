package recordids

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, root, path, body string) {
	t.Helper()
	p := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicatesInEveryFamily(t *testing.T) {
	for _, tc := range []struct{ family, path, text, id string }{
		{"adr", "ADR.md", "## ADR-001: first\n## ADR-001: second\n", "ADR-001"},
		{"invariant", "docs/intent/invariants.md", "| I1 | first |\n| I1 | second |\n", "I1"},
		{"future", "docs/intent/futures/f1.md", "## F1. first\n## F1. second\n", "F1"},
		{"incident", "docs/intent/incidents.md", "### INC-1 first\n- INC-1 closed\n", "INC-1"},
		{"plan", "docs/plans/one.md", "- [ ] first {id: P-1}\n- [x] second {id: P-1}\n", "P-1"},
	} {
		t.Run(tc.family, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, tc.path, tc.text)
			cfg := Defaults()
			var fs []Family
			for _, f := range cfg.Families {
				if f.Name == tc.family {
					fs = append(fs, f)
				}
			}
			cfg.Families = fs
			d, err := Check(root, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if len(d) != 1 || d[0].ID != tc.id || d[0].First.Line != 1 || d[0].Second.Line != 2 || !strings.Contains(d[0].String(), tc.path+":1") || !strings.Contains(d[0].String(), tc.path+":2") {
				t.Fatalf("%+v", d)
			}
		})
	}
}

func TestPlansArchiveAndGrandfather(t *testing.T) {
	root := t.TempDir()
	row := "- [ ] task {id: SHARED-1}\n"
	write(t, root, "docs/plans/a.md", row)
	write(t, root, "docs/plans/b.md", row)
	write(t, root, "docs/plans/archive/old.md", row+row)
	cfg := Defaults()
	cfg.Families = cfg.Families[len(cfg.Families)-1:]
	// Explicitly asking for archive must still exclude it.
	cfg.Families[0].Files = append(cfg.Families[0].Files, "docs/plans/archive/*.md")
	if d, err := Check(root, cfg); err != nil || len(d) != 0 {
		t.Fatalf("default per plan: %+v %v", d, err)
	}
	cfg.CrossPlan = true
	if d, err := Check(root, cfg); err != nil || len(d) != 1 {
		t.Fatalf("cross plan: %+v %v", d, err)
	}
	cfg.Grandfather = []Exception{{ID: "SHARED-1", Paths: []string{"docs/plans/a.md", "docs/plans/b.md"}}}
	if d, err := Check(root, cfg); err != nil || len(d) != 0 {
		t.Fatalf("grandfather: %+v %v", d, err)
	}
	write(t, root, "docs/plans/c.md", row)
	if d, err := Check(root, cfg); err != nil || len(d) == 0 {
		t.Fatalf("exception spread: %+v %v", d, err)
	}
	write(t, root, "docs/plans/a.md", row+row)
	if d, err := Check(root, cfg); err != nil || len(d) < 2 {
		t.Fatalf("same file still denied: %+v %v", d, err)
	}
}

func TestConfigurableFamilyAndDeterminism(t *testing.T) {
	root := t.TempDir()
	write(t, root, "records/a.md", "## Q1 title\n## Q1 another\n````\n## Q1 example\n````\n")
	cfg := Config{Families: []Family{{Name: "question", Files: []string{"records/*.md"}, Pattern: `^## (Q[0-9]+)\b`}}}
	a, err := Check(root, cfg)
	if err != nil || len(a) != 1 {
		t.Fatalf("%+v %v", a, err)
	}
	b, err := Check(root, cfg)
	if err != nil || len(b) != 1 || a[0] != b[0] {
		t.Fatal("non deterministic")
	}
	cfg.Families[0].Pattern = "["
	if _, err := Check(root, cfg); err == nil {
		t.Fatal("bad pattern accepted")
	}
	cfg.Families[0].Pattern = "no capture"
	if _, err := Check(root, cfg); err == nil {
		t.Fatal("no id capture")
	}
}

func TestRepositoryRecordIDs(t *testing.T) {
	cfg, err := Load("../../.fathom/record-ids.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Check("../..", cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, dup := range d {
		t.Error(dup.String())
	}
	cfg.CrossPlan = true
	d, err = Check("../..", cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, dup := range d {
		t.Error(dup.String())
	}
}

func TestMissingRootIsNotUnique(t *testing.T) {
	if _, err := Check(filepath.Join(t.TempDir(), "missing"), Defaults()); err == nil {
		t.Fatal("missing root reported unique")
	}
}

func TestDuplicateOutputIsSorted(t *testing.T) {
	root := t.TempDir()
	write(t, root, "records.md", "## Q1 first\n## Q1 second\n")
	cfg := Config{Families: []Family{{Name: "z", Files: []string{"records.md"}, Pattern: `^## (Q[0-9]+)\b`}, {Name: "a", Files: []string{"records.md"}, Pattern: `^## (Q[0-9]+)\b`}}}
	d, err := Check(root, cfg)
	if err != nil || len(d) != 6 || d[0].Family != "a" || d[1].Family != "z" {
		t.Fatalf("not sorted: %+v %v", d, err)
	}
}

func TestIncidentBoldHeading(t *testing.T) {
	root := t.TempDir()
	write(t, root, "docs/intent/incidents.md", "**INC-15 first**\n**INC-15 duplicate**\n")
	d, err := Check(root, Defaults())
	if err != nil || len(d) != 1 || d[0].ID != "INC-15" {
		t.Fatalf("bold incident duplicates: %+v %v", d, err)
	}
}

func TestGrandfatherRejectsStalePairs(t *testing.T) {
	for _, paths := range [][]string{{"docs/plans/a.md", "docs/plans/b.md"}, {"docs/plans/a.md", "docs/plans/b.md", "docs/plans/c.md"}} {
		root := t.TempDir()
		write(t, root, "docs/plans/a.md", "- [ ] one {id: P-1}\n")
		write(t, root, "docs/plans/b.md", "- [ ] another {id: P-2}\n")
		cfg := Defaults()
		cfg.CrossPlan = true
		cfg.Grandfather = []Exception{{ID: "P-1", Paths: paths}}
		if _, err := Check(root, cfg); err == nil || !strings.Contains(err.Error(), "stale exception") {
			t.Fatalf("stale pair accepted: %v", err)
		}
	}
}

func TestRepositoryEnforcesCrossPlanUniqueness(t *testing.T) {
	cfg, err := Load("../../.fathom/record-ids.json")
	if err != nil || !cfg.CrossPlan {
		t.Fatalf("cross-plan check disabled: %+v %v", cfg, err)
	}
}

func TestLaneRecordIDInventory(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".fathom/observations.md", "### O10000: legacy\nreference O10001\n")
	write(t, root, "docs/dogfood-observations.md", "### O10000: historical, not authoritative\n")
	write(t, root, ".fathom/observations/o.md", "### O10001: one\n### O10001: repeated\n")
	defs, e := Inventory(root, Defaults())
	if e != nil || len(defs) != 3 {
		t.Fatalf("inventory must preserve repetitions: %+v %v", defs, e)
	}
	d, e := CheckDefinitions(defs, Defaults())
	if e != nil || len(d) != 1 || d[0].ID != "O10001" {
		t.Fatalf("observations: %+v %v", d, e)
	}
	// Full ids cannot collide between families even with per-file policy.
	defs = []Definition{{Family: "adr", ID: "SAME-1", At: Location{Path: "a.md", Line: 1}}, {Family: "future", ID: "SAME-1", At: Location{Path: "b.md", Line: 2}}}
	if d, e = CheckDefinitions(defs, Defaults()); e != nil || len(d) != 1 {
		t.Fatalf("cross-family collision %+v %v", d, e)
	}
	for _, name := range []string{"duplicate-slug", "overlap-range"} {
		t.Run(name, func(t *testing.T) {
			grants := []LaneGrant{{ID: "a", OwnedPaths: []string{"a.md"}, IDRanges: []IDRange{{Family: "adr", Prefix: "ADR-", First: 10000, Last: 10001}}}, {ID: "b", OwnedPaths: []string{"b.md"}, IDRanges: []IDRange{{Family: "adr", Prefix: "ADR-", First: 10002, Last: 10003}}}}
			if name == "duplicate-slug" {
				grants[1].ID = "a"
			} else {
				grants[1].IDRanges[0] = IDRange{Family: "plan", Prefix: "ADR-", First: 10001, Last: 10002}
			}
			_, e := ValidateInventory(nil, Defaults(), TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: grants})
			if e == nil {
				t.Fatal("invalid grants accepted")
			}
		})
	}
}

func TestRecordDefiningLaneOwnership(t *testing.T) {
	grant := LaneGrant{ID: "demo", OwnedPaths: []string{"owned.md"}, IDRanges: []IDRange{{Family: "adr", Prefix: "ADR-", First: 10000, Last: 10001}}}
	legacy := Definition{Family: "adr", ID: "ADR-001", At: Location{Path: "legacy.md", Line: 1}}
	trust := TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: []LaneGrant{grant}, Legacy: []Definition{legacy}}
	for _, tc := range []struct{ name, id, path, want string }{{"valid", "ADR-10000", "owned.md", ""}, {"other-lane-file", "ADR-10000", "other.md", "demo"}, {"moved-legacy", "ADR-001", "moved.md", "unreserved"}, {"unique-unreserved", "ADR-20000", "owned.md", "unreserved"}} {
		t.Run(tc.name, func(t *testing.T) {
			defs := []Definition{{Family: "adr", ID: tc.id, At: Location{Path: tc.path, Line: 4}}}
			v, e := CheckOwnership(defs, trust)
			if e != nil {
				t.Fatal(e)
			}
			if tc.want == "" {
				if len(v) != 0 {
					t.Fatal(v)
				}
				return
			}
			if len(v) != 1 || v[0].Definition.At.Path != tc.path || !strings.Contains(v[0].LaneID+string(v[0].Reason), tc.want) {
				t.Fatalf("%+v", v)
			}
		})
	}
	trust.Lanes = append(trust.Lanes, LaneGrant{ID: "other", OwnedPaths: []string{"other.md"}, IDRanges: grant.IDRanges})
	if _, e := CheckOwnership(nil, trust); e == nil {
		t.Fatal("overlapping grants accepted")
	}
	// A moved legacy record is not legacy even if the original line shifts.
	trust.Lanes = trust.Lanes[:1]
	legacy.At.Line = 99
	if v, e := CheckOwnership([]Definition{legacy}, trust); e != nil || len(v) != 0 {
		t.Fatalf("line shift: %+v %v", v, e)
	}
}

func laneManifest(t *testing.T, root, name string, lane LaneDeclaration) {
	t.Helper()
	b, err := json.Marshal(lane)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "docs/lanes/"+name+".cue", "package lanes\nimport \"github.com/geoffmilleraz/fathom/schema/records\"\nlane: records.#Lane & "+string(b)+"\n")
}
func laneFixture(t *testing.T) (string, LaneDeclaration) {
	t.Helper()
	root := t.TempDir()
	schema, err := os.ReadFile("../../schema/records/lane.cue")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "schema/records/lane.cue", string(schema))
	lane := LaneDeclaration{Kind: KindLane, ID: "demo", Goal: "fixture", Class: ClassPackage, BaseCommit: strings.Repeat("a", 40), Rows: []string{"REC-1"}, OwnedPaths: []string{"ADR.md"}, ReadOnlyPaths: []string{}, ContractFiles: []string{"contract.md"}, ProofCommand: "test", Budget: "fixture", HarnessClass: "test", IDRanges: []IDRange{{Family: FamilyADR, Prefix: "ADR-", First: 10000, Last: 10001}}}
	return root, lane
}
func TestLaneManifestShapeAndRanges(t *testing.T) {
	for _, name := range []string{"valid", "shape", "derived-phase", "inverted-range", "overlap-range", "duplicate-key", "filename", "extra-declaration", "extra-import", "symlink-parent", "nil-lists"} {
		t.Run(name, func(t *testing.T) {
			root, lane := laneFixture(t)
			laneManifest(t, root, "demo", lane)
			switch name {
			case "shape":
				lane.Kind = "other"
				laneManifest(t, root, "demo", lane)
			case "derived-phase":
				p := filepath.Join(root, "docs/lanes/demo.cue")
				b, _ := os.ReadFile(p)
				write(t, root, "docs/lanes/demo.cue", strings.Replace(string(b), `"goal":"fixture"`, `"goal":"fixture","phase":"active"`, 1))
			case "inverted-range":
				lane.IDRanges[0].Last = 9999
				laneManifest(t, root, "demo", lane)
			case "overlap-range":
				other := lane
				other.ID = "other"
				other.OwnedPaths = []string{"other.md"}
				other.IDRanges = []IDRange{{Family: FamilyPlan, Prefix: "ADR-", First: 10001, Last: 10002}}
				laneManifest(t, root, "other", other)
			case "duplicate-key":
				laneManifest(t, root, "other", lane)
			case "filename":
				if err := os.Rename(filepath.Join(root, "docs/lanes/demo.cue"), filepath.Join(root, "docs/lanes/renamed.cue")); err != nil {
					t.Fatal(err)
				}
			case "extra-declaration":
				p := filepath.Join(root, "docs/lanes/demo.cue")
				b, _ := os.ReadFile(p)
				write(t, root, "docs/lanes/demo.cue", string(b)+"extra: {owned_paths: [\"outside.md\"]}\n")
			case "extra-import":
				p := filepath.Join(root, "docs/lanes/demo.cue")
				b, _ := os.ReadFile(p)
				write(t, root, "docs/lanes/demo.cue", strings.Replace(string(b), "import ", "import \"strings\"\nimport ", 1))
			case "symlink-parent":
				if err := os.Rename(filepath.Join(root, "docs/lanes"), filepath.Join(root, "elsewhere")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "elsewhere"), filepath.Join(root, "docs/lanes")); err != nil {
					t.Fatal(err)
				}
			case "nil-lists":
				lane.ReadOnlyPaths = nil
				laneManifest(t, root, "demo", lane)
			}
			lanes, err := ReadLanes(root)
			if name == "valid" {
				if err != nil || len(lanes) != 1 || lanes[0].ID != "demo" {
					t.Fatalf("%+v %v", lanes, err)
				}
				b, _ := json.Marshal(lanes)
				if strings.Contains(string(b), "null") {
					t.Fatalf("nil result list: %s", b)
				}
			} else if err == nil {
				t.Fatalf("%s accepted: %+v", name, lanes)
			}
		})
	}
}
func TestValidatedInventoryEffectiveRepositoryPolicy(t *testing.T) {
	cfg, err := Load("../../.fathom/record-ids.json")
	if err != nil {
		t.Fatal(err)
	}
	defs := []Definition{}
	for _, exception := range cfg.Grandfather {
		for _, path := range exception.Paths {
			defs = append(defs, Definition{Family: FamilyPlan, ID: exception.ID, At: Location{Path: path, Line: 1}})
		}
	}
	trust := TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: []LaneGrant{}, Legacy: append([]Definition{}, defs...)}
	v, err := ValidateInventory(defs, cfg, trust)
	if err != nil || len(v.Definitions()) != len(defs) {
		t.Fatalf("effective policy lost definitions: %v", err)
	}
	if len(defs) > 0 {
		original := defs[0]
		defs[0].At.Path = "third.md"
		if _, err := ValidateInventory(defs, cfg, trust); err == nil {
			t.Fatal("effective policy spread")
		}
		defs[0] = original
		defs = append(defs, original)
		if _, err := ValidateInventory(defs, cfg, trust); err == nil {
			t.Fatal("effective policy excused a local duplicate")
		}
	}
}

func TestLaneValidationAlwaysChecksFullKeys(t *testing.T) {
	cfg := Defaults() // legacy Check remains opt-in for cross-plan checks
	defs := []Definition{{Family: FamilyPlan, ID: "DEMO-10000", At: Location{Path: "a.md", Line: 1}}, {Family: FamilyPlan, ID: "DEMO-10000", At: Location{Path: "b.md", Line: 2}}}
	trust := TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: []LaneGrant{{ID: "demo", OwnedPaths: []string{"a.md", "b.md"}, IDRanges: []IDRange{{Family: FamilyPlan, Prefix: "DEMO-", First: 10000, Last: 10001}}}}, Legacy: []Definition{}}
	if duplicates, err := CheckDefinitions(defs, cfg); err != nil || len(duplicates) != 0 {
		t.Fatalf("legacy opt-in behavior changed: %+v %v", duplicates, err)
	}
	if v, err := ValidateInventory(defs, cfg, trust); err == nil || v.Valid() {
		t.Fatal("lane accepted new cross-file duplicate with legacy CrossPlan=false")
	}
	cfg.Grandfather = []Exception{{ID: "DEMO-10000", Paths: []string{"a.md", "b.md"}}}
	trust.Legacy = append([]Definition{}, defs...)
	if v, err := ValidateInventory(defs, cfg, trust); err != nil || len(v.Definitions()) != 2 {
		t.Fatalf("exact pair should survive lane mode: %v", err)
	}
	if _, err := ValidateInventory(defs[:1], cfg, trust); err == nil {
		t.Fatal("stale policy accepted in lane mode")
	}
}
func TestLaneRangesCannotRenderTheSameIdentity(t *testing.T) {
	for _, tc := range []struct {
		name        string
		first, last int
		bad         bool
	}{{"collision", 100, 100, true}, {"upper-bound-collision", 99, 100, true}, {"equal-numbers-different-prefixes", 1100, 1100, false}, {"outside-numeric-interval", 101, 101, false}} {
		t.Run(tc.name, func(t *testing.T) {
			trust := TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: []LaneGrant{{ID: "a", OwnedPaths: []string{"a.md"}, IDRanges: []IDRange{{Family: FamilyPlan, Prefix: "DEMO", First: 1100, Last: 1100}}}, {ID: "b", OwnedPaths: []string{"b.md"}, IDRanges: []IDRange{{Family: FamilyPlan, Prefix: "DEMO1", First: tc.first, Last: tc.last}}}}}
			_, err := CheckOwnership(nil, trust)
			if (err != nil) != tc.bad {
				t.Fatalf("collision=%v: %v", tc.bad, err)
			}
		})
	}
}

func TestLaneOwnedFilesHaveOneWriter(t *testing.T) {
	grants := []LaneGrant{{ID: "a", OwnedPaths: []string{"ADR.md"}, IDRanges: []IDRange{{Family: FamilyADR, Prefix: "ADR-", First: 10000, Last: 10001}}}, {ID: "b", OwnedPaths: []string{"ADR.md"}, IDRanges: []IDRange{{Family: FamilyADR, Prefix: "ADR-", First: 10002, Last: 10003}}}}
	_, err := CheckOwnership(nil, TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: grants})
	if err == nil || !strings.Contains(err.Error(), "ADR.md") || !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Fatalf("multiple writers accepted: %v", err)
	}
}
func TestLaneRangeIntersectionMatchesRenderedIDs(t *testing.T) {
	prefixes := []string{"A", "A0", "A1", "A10", "A01", "A-"}
	intervals := [][2]int{{1, 12}, {90, 110}, {1090, 1110}}
	for _, ap := range prefixes {
		for _, bp := range prefixes {
			for _, ab := range intervals {
				for _, bb := range intervals {
					a, b := IDRange{Family: FamilyPlan, Prefix: ap, First: ab[0], Last: ab[1]}, IDRange{Family: FamilyPlan, Prefix: bp, First: bb[0], Last: bb[1]}
					used := map[string]bool{}
					for n := a.First; n <= a.Last; n++ {
						used[a.Format(n)] = true
					}
					want := false
					for n := b.First; n <= b.Last; n++ {
						want = want || used[b.Format(n)]
					}
					if got := rangesIntersect(a, b); got != want {
						t.Fatalf("%+v / %+v: intersect %v want %v", a, b, got, want)
					}
				}
			}
		}
	}
	// ADR padding cannot combine with a numeric extension after its hyphen:
	// that candidate prefix violates the frozen grammar before overlap testing.
	if err := (IDRange{Family: FamilyPlan, Prefix: "ADR-00", First: 1, Last: 1}).Validate(); err == nil {
		t.Fatal("embedded-hyphen prefix accepted")
	}
}

func TestCritic252NonPlanPerFileCannotExcuseDuplicate(t *testing.T) {
	cfg := Defaults()
	for i := range cfg.Families {
		if cfg.Families[i].Name == "adr" {
			cfg.Families[i].PerFile = true
		}
	}
	defs := []Definition{{Family: FamilyADR, ID: "ADR-10000", At: Location{Path: "a.md", Line: 1}}, {Family: FamilyADR, ID: "ADR-10000", At: Location{Path: "b.md", Line: 1}}}
	trust := TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: []LaneGrant{{ID: "demo", OwnedPaths: []string{"a.md", "b.md"}, IDRanges: []IDRange{{Family: FamilyADR, Prefix: "ADR-", First: 10000, Last: 10001}}}}}
	if v, err := ValidateInventory(defs, cfg, trust); err == nil || v.Valid() {
		t.Fatalf("new duplicate ADR-10000 validated across a.md/b.md without grandfather exception: valid=%v error=%v", v.Valid(), err)
	}
}

func TestCritic252TrustedInventoryUsesGitBlobBytes(t *testing.T) {
	root, lane := laneFixture(t)
	laneManifest(t, root, "demo", lane)
	write(t, root, "ADR.md", "## ADR-001: existing\n")
	write(t, root, ".gitattributes", "ADR.md export-ignore\n")
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, b, err)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "test: immutable input")
	revision := git("rev-parse", "HEAD")
	if !strings.Contains(git("show", revision+":ADR.md"), "ADR-001") {
		t.Fatal("invalid fixture")
	}
	s, err := LoadTrusted(context.Background(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range s.Inventory.Legacy {
		if d.ID == "ADR-001" {
			return
		}
	}
	t.Fatalf("trusted legacy ADR-001 silently omitted despite presence in immutable git blob: %+v", s.Inventory.Legacy)
}

func rawTrustGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s %v", args, b, e)
	}
	return strings.TrimSpace(string(b))
}
func rawTrustCommit(t *testing.T, root string) string {
	t.Helper()
	rawTrustGit(t, root, "init", "-q")
	rawTrustGit(t, root, "add", ".")
	rawTrustGit(t, root, "commit", "-qm", "test: trusted raw input")
	return rawTrustGit(t, root, "rev-parse", "HEAD")
}
func TestTrustedRawBlobAttributesAndTypes(t *testing.T) {
	for _, name := range []string{"export-subst", "ignored-schema", "ignored-owned-target", "symlink", "gitlink"} {
		t.Run(name, func(t *testing.T) {
			root, lane := laneFixture(t)
			lane.Goal = "Keep literal $Format:%H$ bytes"
			lane.OwnedPaths = append(lane.OwnedPaths, "extra.md")
			laneManifest(t, root, "demo", lane)
			write(t, root, "ADR.md", "## ADR-001: existing\n")
			write(t, root, "extra.md", "## ADR-002: other\n")
			switch name {
			case "export-subst":
				write(t, root, ".gitattributes", "docs/lanes/demo.cue export-subst\n")
			case "ignored-schema":
				write(t, root, ".gitattributes", "schema/records/lane.cue export-ignore\n")
			case "ignored-owned-target":
				write(t, root, ".gitattributes", "extra.md export-ignore\n")
			case "symlink":
				if e := os.Remove(filepath.Join(root, "extra.md")); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink("ADR.md", filepath.Join(root, "extra.md")); e != nil {
					t.Fatal(e)
				}
			}
			rev := rawTrustCommit(t, root)
			if name == "gitlink" {
				rawTrustGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+rev+",extra.md")
				rawTrustGit(t, root, "commit", "-qm", "test: non-blob target")
				rev = rawTrustGit(t, root, "rev-parse", "HEAD")
			}
			expected, e := os.ReadFile(filepath.Join(root, "docs/lanes/demo.cue"))
			if e != nil {
				t.Fatal(e)
			}
			state, e := LoadTrusted(context.Background(), root, rev)
			if name == "symlink" || name == "gitlink" {
				if e == nil {
					t.Fatal("nonregular target accepted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if string(state.Manifests["demo"]) != string(expected) || state.Lanes[0].Goal != lane.Goal {
				t.Fatal("trusted manifest changed by export attributes")
			}
			for _, id := range []string{"ADR-001", "ADR-002"} {
				found := false
				for _, d := range state.Inventory.Legacy {
					found = found || d.ID == id
				}
				if !found {
					t.Fatalf("raw legacy %s missing: %+v", id, state.Inventory.Legacy)
				}
			}
		})
	}
}
func TestTrustedGitReadBoundsAndCancellation(t *testing.T) {
	root, lane := laneFixture(t)
	laneManifest(t, root, "demo", lane)
	write(t, root, "ADR.md", "## ADR-001: existing\n")
	rev := rawTrustCommit(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := LoadTrusted(ctx, root, rev); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation lost: %v", e)
	}
	oid := rawTrustGit(t, root, "rev-parse", rev+":ADR.md")
	if _, e := readTrustedGit(context.Background(), root, 4, "cat-file", "blob", oid); e == nil {
		t.Fatal("unbounded blob output")
	} else {
		var refusal *RefusalError
		if !errors.As(e, &refusal) || refusal.Code != ErrTrust || !strings.Contains(refusal.Detail, "limit") {
			t.Fatalf("wrong bound refusal: %v", e)
		}
	}
	b, e := readTrustedGit(context.Background(), root, 64, "cat-file", "blob", oid)
	if e != nil || string(b) != "## ADR-001: existing\n" {
		t.Fatalf("raw output: %q %v", b, e)
	}
}
func TestLaneFullKeyValidationDoesNotMutatePolicy(t *testing.T) {
	cfg := Defaults()
	for i := range cfg.Families {
		cfg.Families[i].PerFile = true
	}
	before, _ := json.Marshal(cfg)
	_, e := ValidateInventory(nil, cfg, TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: []LaneGrant{}, Legacy: []Definition{}})
	if e != nil {
		t.Fatal(e)
	}
	after, _ := json.Marshal(cfg)
	if string(before) != string(after) {
		t.Fatal("lane validation mutated caller policy")
	}
}

func TestTrustedGitReadCancelsRunningProcess(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	script := []byte("#!/bin/sh\n[ \"$GIT_NO_LAZY_FETCH\" = 1 ] || exit 42\nprintf started > \"$REC1_GIT_STARTED\"\nexec sleep 10\n")
	if err := os.WriteFile(filepath.Join(bin, "git"), script, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "started")
	t.Setenv("REC1_GIT_STARTED", marker)
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := readTrustedGit(ctx, root, 64, "cat-file", "blob", strings.Repeat("a", 40))
		done <- err
	}()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
started:
	for {
		select {
		case err := <-done:
			t.Fatalf("reader exited before subprocess start: %v", err)
		case <-deadline.C:
			t.Fatal("git subprocess did not start")
		case <-tick.C:
			if _, err := os.Stat(marker); err == nil {
				break started
			}
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost running-process cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("git subprocess not cancelled")
	}
}

func TestRecordIDDefaultObservationPathsRemainExact(t *testing.T) {
	for _, family := range Defaults().Families {
		if family.Name == "observation" {
			if len(family.Files) != 2 || family.Files[0] != ".fathom/observations.md" || family.Files[1] != ".fathom/observations/*.md" {
				t.Fatalf("observation paths changed: %v", family.Files)
			}
			return
		}
	}
	t.Fatal("observation family absent")
}

func TestTrustedGitInventoryExactArguments(t *testing.T) {
	root, lane := laneFixture(t)
	laneManifest(t, root, "demo", lane)
	write(t, root, "ADR.md", "## ADR-001: existing\n")
	revision := rawTrustCommit(t, root)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	prefix := []string{"--no-replace-objects", "-C", root}
	want := [][]string{
		append(append([]string{}, prefix...), "cat-file", "-e", revision+"^{commit}"),
		append(append([]string{}, prefix...), "ls-tree", "-r", "-z", revision),
	}
	for _, path := range []string{"ADR.md", "docs/lanes/demo.cue", "schema/records/lane.cue"} {
		oid := rawTrustGit(t, root, "rev-parse", revision+":"+path)
		want = append(want, append(append([]string{}, prefix...), "cat-file", "blob", oid))
	}
	spy := t.TempDir()
	logPath := filepath.Join(spy, "argv")
	script := `#!/bin/sh
printf '%s\000' "$@" >> "$REC1_ARGV_LOG"
printf '\000' >> "$REC1_ARGV_LOG"
case "$#:$4:$5" in
  6:cat-file:-e|6:cat-file:blob) ;;
  7:ls-tree:-r) [ "$6" = -z ] || exit 41 ;;
  *) exit 41 ;;
esac
exec "$REC1_REAL_GIT" "$@"
`
	if err := os.WriteFile(filepath.Join(spy, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REC1_ARGV_LOG", logPath)
	t.Setenv("REC1_REAL_GIT", realGit)
	t.Setenv("PATH", spy+string(os.PathListSeparator)+os.Getenv("PATH"))
	state, err := LoadTrusted(context.Background(), root, revision)
	raw, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err != nil {
		t.Fatalf("trusted read: %v; actual argv bytes %q", err, raw)
	}
	got := [][]string{}
	for _, frame := range bytes.Split(raw, []byte{0, 0}) {
		if len(frame) == 0 {
			continue
		}
		args := []string{}
		for _, arg := range bytes.Split(frame, []byte{0}) {
			args = append(args, string(arg))
		}
		got = append(got, args)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Git argv = %#v, want exactly %#v", got, want)
	}
	if len(state.Inventory.Legacy) != 1 || state.Inventory.Legacy[0].ID != "ADR-001" {
		t.Fatalf("fixture inventory %+v", state.Inventory.Legacy)
	}
}
