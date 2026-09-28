package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdIntentADRRulesProposesNeverWrites (CX-19): `fathom intent
// adr-rules` proposes each dependency statement in the decision records as
// a cited archconformance rule, marks the ones the rule pack already holds,
// and never writes the pack — a human accepts each.
func TestCmdIntentADRRulesProposesNeverWrites(t *testing.T) {
	root := t.TempDir()
	writeCorpusFixture(t, root)
	adrDoc := "# Decisions\n\n## ADR-001 — Layers\n\nThe `internal/connector` package never imports `internal/query`.\nHandlers call services, never stores.\n"
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte(adrDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	pack := filepath.Join(root, ".fathom", "rules", "archconformance.cue")
	if err := os.MkdirAll(filepath.Dir(pack), 0o755); err != nil {
		t.Fatal(err)
	}
	packBody := "rules: [{from_pkg: \"connector\", to_pkg: \"query\", allow: false}]\n"
	if err := os.WriteFile(pack, []byte(packBody), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := cmdIntent([]string{"adr-rules", "--root", root, "--json"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var doc struct {
		Proposals []adrRuleRow `json:"proposals"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Proposals) != 2 {
		t.Fatalf("proposals = %+v", doc.Proposals)
	}
	if p := doc.Proposals[0]; p.FromPkg != "connector" || p.ToPkg != "query" || !p.InRulePack || p.Citation != "ADR.md:5" || p.Decision != "ADR-001" || p.Status != "proposed" {
		t.Errorf("first proposal = %+v", p)
	}
	if p := doc.Proposals[1]; p.FromPkg != "handlers" || p.ToPkg != "stores" || p.InRulePack {
		t.Errorf("second proposal = %+v", p)
	}
	out.Reset()
	if code := cmdIntent([]string{"adr-rules", "--root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	for _, want := range []string{"fathom never writes", "// already in .fathom/rules/archconformance.cue", `{from_pkg: "handlers", to_pkg: "stores", allow: false}`, "ADR.md:6"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, out.String())
		}
	}
	if b, _ := os.ReadFile(pack); string(b) != packBody {
		t.Errorf("the rule pack was written:\n%s", b)
	}
}
