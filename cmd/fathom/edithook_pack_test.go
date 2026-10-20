package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func packWrite(t *testing.T, root, rel, data string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// packRepo makes a repository under t.TempDir() that pins the given kit
// modules by local replace; each kit is name -> {pack.cue, fixtures/all.cue}.
func packRepo(t *testing.T, kits map[string][2]string) string {
	t.Helper()
	t.Setenv("GOFLAGS", "")
	base := t.TempDir()
	root := filepath.Join(base, "repo")
	gomod := "module example.test/app\n\ngo 1.22\n\nrequire (\n"
	repl := ""
	for name, f := range kits {
		gomod += fmt.Sprintf("\texample.test/%s v1.0.0\n", name)
		repl += fmt.Sprintf("replace example.test/%s => ../%s\n", name, name)
		packWrite(t, base, name+"/go.mod", "module example.test/"+name+"\n\ngo 1.22\n")
		packWrite(t, base, name+"/fathom-pack/pack.cue", f[0])
		if f[1] != "" {
			packWrite(t, base, name+"/fathom-pack/fixtures/all.cue", f[1])
		}
	}
	packWrite(t, root, "go.mod", gomod+")\n\n"+repl)
	gitInitTempRepo(t, root)
	return root
}

func packHeader(name string) string {
	return `pack: {name: "` + name + `", version: 1, about: "test"}` + "\n"
}

// obligationsServer answers `rules` rules, the last of which is bound to a
// module, capped by the asked max_rules.
func obligationsServer(t *testing.T) string {
	t.Helper()
	srv, _ := countingServer(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string `json:"name"`
				Arguments struct {
					File     string `json:"file"`
					MaxRules int    `json:"max_rules"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		inner := `{}`
		switch req.Params.Name {
		case "list_services":
			inner = `{"services":[]}`
		case "obligations":
			n := req.Params.Arguments.MaxRules
			if n <= 0 || n > semantic.MaxObligations {
				n = semantic.MaxObligations
			}
			rules := []any{}
			for i := 0; i < n; i++ {
				r := map[string]any{"rule": fmt.Sprintf("Repo rule %d.", i+1), "src": "AGENTS.md#L1-L2", "tier": "doc-claimed", "layer": "obligations"}
				if i == n-1 {
					r["rule"], r["module"] = fmt.Sprintf("Module rule %d.", i+1), "mod"
				}
				rules = append(rules, r)
			}
			b, _ := json.Marshal(map[string]any{"service": "svc", "file": req.Params.Arguments.File, "rules": rules})
			inner = string(b)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": inner}}}})
	})
	t.Cleanup(srv.Close)
	return srv.URL
}

func guardsPack(name string, notes ...string) [2]string {
	pack, fx := packHeader(name)+"guards: [\n", "guards: [\n"
	for i, n := range notes {
		pack += fmt.Sprintf("\t{scopes: [\"internal/**\"], note: %q, max_rules: 1},\n", n)
		fx += fmt.Sprintf("\t{guard: %d, match: [\"internal/a.go\"], nomatch: [\"cmd/a.go\"]},\n", i)
	}
	return [2]string{pack + "]\n", fx + "]\n"}
}

// FD-14: repo rules first, then bound module rules, then pack lines tagged
// name@version; at most 2 lines per pack; a displaced line is counted per
// source in the existing displaced line; an unfixtured section is not served.
func TestEditHookPreServesPackGuardsBelowRepoRules(t *testing.T) {
	guardStampsIsolated(t)
	t.Setenv("HOME", t.TempDir())
	root := packRepo(t, map[string][2]string{
		"alpha": guardsPack("alpha", "Alpha one.", "Alpha two.", "Alpha three."),
		"beta":  guardsPack("beta", "Beta one."),
		"gamma": {packHeader("gamma") + `guards: [{scopes: ["internal/**"], note: "Gamma unfixtured.", max_rules: 1}]` + "\n", ""},
	})
	writeGuardCue(t, root, `guards: [{scopes: ["internal/**"], note: "hand note", max_rules: 4, include_modules: true}]`)
	file := filepath.Join(root, "internal", "a.go")
	packWrite(t, root, "internal/a.go", "package internal\n")

	res, ok := renderPreEdit(obligationsServer(t), root, file, "s-pack")
	if !ok {
		t.Fatal("expected a served payload")
	}
	text := res.text
	idx := func(s string) int { return strings.Index(text, s) }
	for _, want := range []string{"Repo rule 1.", "Module rule 4.", "[alpha@1] Alpha one.", "[alpha@1] Alpha two."} {
		if idx(want) < 0 {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	// 4 repo/module rules leave 2 of I35's 6 slots: alpha takes both, beta
	// is displaced.
	if strings.Contains(text, "Beta one.") {
		t.Fatalf("beta must be displaced (no slot left):\n%s", text)
	}
	if !(idx("Repo rule 1.") < idx("Module rule 4.") && idx("Module rule 4.") < idx("[alpha@1] Alpha one.") && idx("[alpha@1] Alpha one.") < idx("[alpha@1] Alpha two.")) {
		t.Fatalf("order must be repo rules, module rules, packs:\n%s", text)
	}
	if strings.Contains(text, "Alpha three.") {
		t.Fatalf("more than 2 lines for one pack:\n%s", text)
	}
	if strings.Contains(text, "Gamma") {
		t.Fatalf("an unfixtured pack section must not be served:\n%s", text)
	}
	// alpha: 1 over its own cap, beta: 1 for want of a slot; both counted by source.
	if !strings.Contains(text, "pack line(s) not shown (cap): alpha@1 1, beta@1 1") {
		t.Fatalf("displacement must be counted per source:\n%s", text)
	}
	bullets := 0
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "• ") {
			bullets++
		}
	}
	if bullets > semantic.MaxObligations {
		t.Fatalf("%d bullets exceed I35's %d:\n%s", bullets, semantic.MaxObligations, text)
	}
	if strings.Contains(text, "verified") {
		t.Fatalf("a pack line carries no verified tier:\n%s", text)
	}
}

// A repository with no pin on a pack serves exactly what it did before.
func TestEditHookPreNoPackLeavesPayloadUnchanged(t *testing.T) {
	guardStampsIsolated(t)
	t.Setenv("HOME", t.TempDir())
	root := packRepo(t, nil)
	writeGuardCue(t, root, `guards: [{scopes: ["internal/**"], note: "hand note", max_rules: 2}]`)
	packWrite(t, root, "internal/a.go", "package internal\n")
	res, ok := renderPreEdit(obligationsServer(t), root, filepath.Join(root, "internal", "a.go"), "s-nopack")
	if !ok || strings.Contains(res.text, "pack line") || strings.Contains(res.text, "@") || res.generated != nil {
		t.Fatalf("no pack, no pack output:\n%s", res.text)
	}
}

const genDecl = `generated: [{
	output:     "ui/{name}_gen.go"
	source:     ["ui/{name}.src"]
	regenerate: %q
}]
`

// FD-15: an edit to a declared generated output gets one line naming the
// source and the regenerate command, rendered and never run; a file no
// declaration names gets none.
func TestEditHookPreGeneratedLineNeverRuns(t *testing.T) {
	guardStampsIsolated(t)
	t.Setenv("HOME", t.TempDir())
	root := packRepo(t, map[string][2]string{
		"kit": {
			packHeader("kit") + `generated: [{output: "gen/{name}.out", source: ["gen/{name}.in"], regenerate: "kitgen $(touch PACKMARK)"}]` + "\n",
			`generated: [{output: "gen/a.out", source: ["gen/a.in"]}]` + "\n",
		},
	})
	marker := filepath.Join(root, "MARKER")
	regen := "gen; touch " + marker + " && $(touch " + marker + ") `touch " + marker + "`\nrm -rf /"
	packWrite(t, root, ".fathom/gen.cue", fmt.Sprintf(genDecl, regen))
	url := obligationsServer(t)

	res, ok := renderPreEdit(url, root, filepath.Join(root, "ui", "x_gen.go"), "s-gen")
	if !ok || res.generated == nil {
		t.Fatalf("a declared output must be served a line (ok=%v)", ok)
	}
	want := "`ui/x_gen.go` is generated from `ui/x.src` by `gen; touch " + marker + " && $(touch " + marker + ") `touch " + marker + "` rm -rf /`: edit the source"
	if !strings.Contains(res.text, want) {
		t.Fatalf("exact line missing:\nwant %s\ngot  %s", want, res.text)
	}
	if strings.Count(res.text, "is generated from") != 1 {
		t.Fatalf("exactly one line:\n%s", res.text)
	}

	// A pack's declaration (fixtured) serves its tagged line.
	res, ok = renderPreEdit(url, root, filepath.Join(root, "gen", "a.out"), "s-gen")
	if !ok || !strings.Contains(res.text, "[kit@1] `gen/a.out` is generated from `gen/a.in` by `kitgen $(touch PACKMARK)`: edit the source") {
		t.Fatalf("pack generated line missing (ok=%v):\n%s", ok, res.text)
	}

	// A file nothing declares gets no line.
	if res, ok = renderPreEdit(url, root, filepath.Join(root, "ui", "x.src"), "s-gen"); ok {
		t.Fatalf("a source file is not an output:\n%s", res.text)
	}
	for _, m := range []string{marker, filepath.Join(root, "PACKMARK")} {
		if _, err := os.Stat(m); err == nil {
			t.Fatalf("regenerate ran: %s exists", m)
		}
	}
}

func outcomeRows(t *testing.T, path string) []ruleOutcomeEvent {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []ruleOutcomeEvent
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var e ruleOutcomeEvent
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("row %q: %v", l, err)
		}
		rows = append(rows, e)
	}
	return rows
}

// FD-22: the log records a served generated line, then the session's next
// edit as source, output or other, at zero spend and with no new file.
func TestRuleOutcomeNextEditAfterGeneratedLine(t *testing.T) {
	guardStampsIsolated(t)
	t.Setenv("HOME", t.TempDir())
	logPath := filepath.Join(t.TempDir(), "rule-outcomes.jsonl")
	t.Setenv("FATHOM_RULE_OUTCOME_LOG", logPath)
	root := packRepo(t, nil)
	packWrite(t, root, ".fathom/gen.cue", fmt.Sprintf(genDecl, "gen"))
	url := obligationsServer(t)

	edit := func(session, rel string) {
		t.Helper()
		hookStdin(t, fmt.Sprintf(`{"tool_name":"Edit","tool_input":{"file_path":%q},"cwd":%q,"session_id":%q}`, filepath.Join(root, rel), root, session))
		var out, errBuf strings.Builder
		if code := runWithStdin([]string{"edit-hook", "--pre", "--url=" + url}, nil, &out, &errBuf); code != 0 {
			t.Fatalf("exit %d: %s", code, errBuf.String())
		}
	}
	edit("s1", "ui/a_gen.go")     // served
	edit("s2", "ui/other.go")     // another session's edit never resolves s1
	edit("s1", "ui/a.src")        // next_edit_source
	edit("s1", "ui/a_gen.go")     // served
	edit("s1", "ui/a_gen.go")     // next_edit_output, served
	edit("s1", "ui/elsewhere.go") // next_edit_other
	edit("s1", "ui/a.src")        // nothing pending: no row

	var got []string
	for _, r := range outcomeRows(t, logPath) {
		got = append(got, r.Event+" "+r.RuleID+" "+r.File+" "+r.Session)
		if r.Event == ruleOutcomeServed && (r.Output != r.File || len(r.Sources) != 1 || r.Sources[0] != "ui/a.src") {
			t.Errorf("served row must name its output and sources: %+v", r)
		}
	}
	want := []string{
		"served generated:ui/a_gen.go ui/a_gen.go s1",
		"next_edit_source generated:ui/a_gen.go ui/a.src s1",
		"served generated:ui/a_gen.go ui/a_gen.go s1",
		"next_edit_output generated:ui/a_gen.go ui/a_gen.go s1",
		"served generated:ui/a_gen.go ui/a_gen.go s1",
		"next_edit_other generated:ui/a_gen.go ui/elsewhere.go s1",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
