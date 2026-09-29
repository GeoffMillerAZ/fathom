package conserve

// pinloss_cite_test.go — PR-12 (ADR-057's K4 revisit clause, fired by O161):
// a retired pin is exempt only with a decision cite in the range. Every case
// is the real i28 retirement (guards-fathom-r2 i28/full/t2's head test file
// with guards-fathom-r2 i28/full/t1's head invariants.md, whose I28 row names
// the new test) plus one file the case adds or edits; the r2 amendment text
// is copied verbatim from guards-fathom-r2 i28/full/t1's agent.diff.

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

const pinLostTest = "TestBackendForPath_DuckDBLoudlyRefused"

// r2Amendment is guards-fathom-r2 i28/full/t1's ADR-025 amendment: dated, but
// it names BackendForPath, never the lost test, and says neither decision nor
// retire.
const r2Amendment = "**Amendment (2026-09-21): `.duckdb` paths open as SQLite.** A user reported\n" +
	"`-store <file>.duckdb` \"hangs\" instead of doing anything useful, and asked\n" +
	"that the command run rather than error out. `BackendForPath` therefore no\n" +
	"longer refuses `.duckdb`: it opens the path as a `SQLiteStore`\n" +
	"(`docs/plans/duckdb-path-as-sqlite.md`, DPS-1). This reverses the\n" +
	"\"stays loud\" paragraph above for `BackendForPath`/`Open` only.\n"

const adrBase = "# Architecture decisions\n\n## ADR-025: store format\n\n**Decision.** `.duckdb` stays loud.\n"

func TestPinLossRetirementNeedsDecisionCite(t *testing.T) {
	baseBackend := readPinFixture(t, "backend_test.go.base.txt")
	headBackend := readPinFixture(t, "backend_test.go.head.txt")
	baseInv := readPinFixture(t, "invariants.md.base.txt")
	retiredInv := readPinFixture(t, "invariants.md.retired.txt")

	type file struct{ base, head string } // "" base: new in the range
	cases := []struct {
		name     string
		files    map[string]file
		wantCite string // "" = no cite, the retirement blocks
	}{
		{name: "edited source, no cite", files: nil},
		{
			name:     "ADR.md added line names the test",
			files:    map[string]file{"ADR.md": {adrBase, adrBase + "\nRetires `" + pinLostTest + "`: `.duckdb` now opens as SQLite.\n"}},
			wantCite: "ADR.md:7",
		},
		{
			name:     "docs/intent/decisions record names the test",
			files:    map[string]file{"docs/intent/decisions/0042-duckdb-opens.md": {"", "# 0042\n\nThe loud refusal and " + pinLostTest + " go.\n"}},
			wantCite: "docs/intent/decisions/0042-duckdb-opens.md:3",
		},
		{
			name:     "docs/adr record names the test",
			files:    map[string]file{"docs/adr/0042.md": {"", "# 0042\n\n" + pinLostTest + " is replaced.\n"}},
			wantCite: "docs/adr/0042.md:3",
		},
		{
			// O161: the r1 cells' only lines naming the lost test are plan rows,
			// dated or not, and a plan row never exempts.
			name: "docs/plans row names the test",
			files: map[string]file{"docs/plans/duckdb-as-sqlite.md": {"", "# DuckDB as SQLite\n\n" +
				"- [x] Replace " + pinLostTest + " with a test for the new behavior {id: DS-2}\n" +
				"- [x] 2026-09-21 decision: retire " + pinLostTest + " {id: DS-3}\n"}},
		},
		{
			name: "dated decision line in invariants.md names the test",
			files: map[string]file{pinInvariants: {baseInv, retiredInv +
				"\n- 2026-09-21 decision: " + pinLostTest + " retired with ADR-025's amendment.\n"}},
			wantCite: pinInvariants + ":" + strconv.Itoa(strings.Count(retiredInv, "\n")+2),
		},
		{
			name: "undated decision line outside a decision record",
			files: map[string]file{pinInvariants: {baseInv, retiredInv +
				"\n- Decision: " + pinLostTest + " retired with ADR-025's amendment.\n"}},
		},
		{
			name:  "dated line without a decision word",
			files: map[string]file{"NOTES.md": {"", "2026-09-21: " + pinLostTest + " now opens as SQLite.\n"}},
		},
		{
			name:  "ADR amendment that does not name the test (r2 shape)",
			files: map[string]file{"ADR.md": {adrBase, adrBase + "\n" + r2Amendment}},
		},
		{
			name: "pre-existing ADR line naming the test",
			files: map[string]file{"ADR.md": {
				adrBase + "\n`" + pinLostTest + "` pins the refusal.\n",
				adrBase + "\n`" + pinLostTest + "` pins the refusal.\n\n" + r2Amendment,
			}},
		},
		{
			name: "a line moved within a decision record is not added",
			files: map[string]file{"ADR.md": {
				adrBase + "\n`" + pinLostTest + "` pins the refusal.\n",
				"`" + pinLostTest + "` pins the refusal.\n\n" + adrBase,
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := map[string]string{pinBackendTest: baseBackend, pinInvariants: baseInv}
			head := map[string]string{pinBackendTest: headBackend, pinInvariants: retiredInv}
			changed := []string{pinBackendTest, pinInvariants}
			for p, f := range tc.files {
				if f.base != "" {
					base[p] = f.base
				}
				head[p] = f.head
				if p != pinInvariants {
					changed = append(changed, p)
				}
			}
			got, err := PinLosses(context.Background(), PinLossInput{
				Base: "base", Head: "head", ChangedFiles: changed,
				Read: pinRevs{"base": base, "head": head}.read,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Test != pinLostTest {
				t.Fatalf("PinLosses = %+v, want the one i28 loss", got)
			}
			l := got[0]
			if l.StillNamedAtHead || !l.SourceEdited || l.Silent() {
				t.Fatalf("still named = %v, source edited = %v; want a retirement (I28 rewritten to name the new test)", l.StillNamedAtHead, l.SourceEdited)
			}
			if l.DecisionCite != tc.wantCite {
				t.Errorf("decision cite = %q, want %q", l.DecisionCite, tc.wantCite)
			}
			if wantBlock := tc.wantCite == ""; l.Blocks() != wantBlock {
				t.Errorf("Blocks() = %v, want %v (a retirement blocks unless cited)", l.Blocks(), wantBlock)
			}
		})
	}
}

// TestPinLossSilentLossBlocksEvenWhenCited: a cite exempts a retirement only;
// a loss whose source still names the test at head blocks whatever the range
// also says.
func TestPinLossSilentLossBlocksEvenWhenCited(t *testing.T) {
	baseInv := readPinFixture(t, "invariants.md.base.txt")
	got, err := PinLosses(context.Background(), PinLossInput{
		Base: "base", Head: "head",
		ChangedFiles: []string{pinBackendTest, "ADR.md"},
		Read: pinRevs{
			"base": {pinBackendTest: readPinFixture(t, "backend_test.go.base.txt"), pinInvariants: baseInv, "ADR.md": adrBase},
			"head": {pinBackendTest: readPinFixture(t, "backend_test.go.head.txt"), pinInvariants: baseInv,
				"ADR.md": adrBase + "\n2026-09-21: decided to retire " + pinLostTest + ".\n"},
		}.read,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].StillNamedAtHead || !got[0].Blocks() || got[0].DecisionCite != "" {
		t.Fatalf("PinLosses = %+v, want one silent loss that blocks, no cite looked for", got)
	}
}
