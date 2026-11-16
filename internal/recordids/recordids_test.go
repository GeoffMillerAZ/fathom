package recordids

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
