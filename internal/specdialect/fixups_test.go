package specdialect

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// TestWriteFixupsOnlyInsideMarkers (SI-6, design §5, law 15): fix-up tasks
// land only inside the fathom converge block, in the dialect's own
// checkbox shape; every byte outside the markers is unchanged, and a
// re-run replaces the block's contents rather than growing it.
func TestWriteFixupsOnlyInsideMarkers(t *testing.T) {
	spec := "# Tasks\n\n" +
		"- [x] T001 Charge {symbol: Charge, package: svc/pay}\n" +
		"- [x] T002 Ghost {symbol: Ghost}\n" +
		"- [ ] T003 Write the runbook" // no final newline
	plan, _ := specKitParse("specs/001/tasks.md", []byte(spec))
	r := ConvergeReport{
		ClaimedUnproven: []ConvergeItem{{TaskID: "T002", Title: "Ghost", Line: 4, Criterion: `symbol_present: "Ghost" not found {x}`}},
		Unclaimed:       []ChangedSymbol{{Name: "Client.Void", Package: "pay", File: "svc/pay/charge.go", Line: 40}},
	}

	out, err := WriteFixups("speckit", []byte(spec), plan, r)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.HasPrefix(got, spec+"\n") {
		t.Fatalf("text before the block changed:\n%s", got)
	}
	block := got[len(spec):]
	for _, want := range []string{
		ConvergeBegin + "\n",
		"- [ ] T004 Make T002 pass — symbol_present: \"Ghost\" not found (x) {symbol: Ghost}\n",
		"- [ ] T005 Claim or remove Client.Void (svc/pay/charge.go:40), changed on this branch with no task anchoring it {symbol: Client.Void, package: svc/pay}\n",
		ConvergeEnd + "\n",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("block lacks %q:\n%s", want, block)
		}
	}
	// The dialect reads its own fix-ups back as anchored tasks.
	back, _ := specKitParse("specs/001/tasks.md", out)
	if len(back.Tasks) != 5 || back.Tasks[3].Expect.Symbol != "Ghost" || back.Tasks[4].Expect.Symbol != "Client.Void" || back.Tasks[4].Expect.Package != "svc/pay" {
		t.Fatalf("fix-ups did not parse back as anchored Spec Kit tasks: %+v", back.Tasks)
	}

	// The author adds text after the block and edits above it; a re-run
	// with a different report replaces only the block's contents, and
	// numbers from the tasks outside it (T004 again, not T006).
	edited := strings.Replace(string(out), "# Tasks", "# Tasks (edited)", 1) + "\n## Author notes after the block\n"
	r2 := ConvergeReport{Unclaimed: []ChangedSymbol{{Name: "Refund", File: "svc/pay/refund.go", Line: 5}}}
	plan2, _ := specKitParse("specs/001/tasks.md", []byte(edited))
	out2, err := WriteFixups("speckit", []byte(edited), plan2, r2)
	if err != nil {
		t.Fatal(err)
	}
	b := strings.Index(edited, ConvergeBegin)
	e := strings.Index(edited, ConvergeEnd)
	b2 := strings.Index(string(out2), ConvergeBegin)
	e2 := strings.Index(string(out2), ConvergeEnd)
	if string(out2[:b2]) != edited[:b] || string(out2[e2:]) != edited[e:] {
		t.Fatalf("text outside the markers changed on re-run:\n%s", out2)
	}
	inside := string(out2[b2:e2])
	if !strings.Contains(inside, "- [ ] T004 Claim or remove Refund") || strings.Contains(inside, "Ghost") {
		t.Fatalf("re-run must replace the block's contents:\n%s", inside)
	}

	// An empty report empties the block but keeps the markers.
	out3, err := WriteFixups("speckit", out2, plan2, ConvergeReport{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out3), ConvergeBegin+"\n"+ConvergeEnd+"\n") {
		t.Fatalf("empty report: %s", out3)
	}

	// A lone or misordered marker is refused and nothing changes.
	for _, bad := range []string{
		spec + "\n" + ConvergeBegin + "\n- [ ] T009 x\n",
		spec + "\n" + ConvergeEnd + "\n" + ConvergeBegin + "\n",
	} {
		got, err := WriteFixups("speckit", []byte(bad), plan, r)
		if err == nil || string(got) != bad {
			t.Fatalf("malformed markers must be refused unchanged; err=%v", err)
		}
	}

	// OpenSpec: a new top-level number; native: plain rows.
	os := "## 1. Impl\n- [x] 1.1 a\n- [ ] 2.3 b\n"
	osPlan, _ := openSpecParse("openspec/changes/c/tasks.md", []byte(os), nil)
	lines := FixupLines("openspec", []byte(os), osPlan, r2)
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "- [ ] 3.1 Claim or remove Refund") {
		t.Fatalf("openspec fix-up shape: %v", lines)
	}
	nat := FixupLines(NativeID, nil, verify.Plan{}, r2)
	if len(nat) != 1 || !strings.HasPrefix(nat[0], "- [ ] Claim or remove Refund") {
		t.Fatalf("native fix-up shape: %v", nat)
	}
}
