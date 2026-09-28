package semantic

// rules_readers_test.go — GL-2..GL-6 (docs/plans/governance-learning.md,
// design doc §3): every client reader hands its files to the SAME compile
// path, and a file's declared scope (Cursor globs, Copilot applyTo, Kiro
// fileMatchPattern) replaces the model's proposed scope there, while a
// declared audience (manual, review) decides who the claim is served to.

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// rulesIndexed is the indexed corpus every reader test verifies scope
// against: two TypeScript trees, a Go tree and docs.
var rulesIndexed = []string{
	"cmd/tool/main.go",
	"docs/guide.md",
	"internal/a/x/a.go",
	"internal/b/pure/b.go",
	"services/api/h/handler.go",
	"services/api/h/handler_test.go",
	"web/src/c/app.ts",
	"web/src/c/view.tsx",
	"web/lib/util.ts",
}

// writeTree writes files (repo-relative path → content) under a fresh root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// compiledRule is one stored claim's scope and audience.
type compiledRule struct {
	Scope    []string
	Audience string
}

// compileRulesDoc compiles one rules document whose only rule sits on
// citeLine, the model proposing `proposed` as scope, and returns the stored
// claim's scope and audience — or ok=false when the claim was rejected.
func compileRulesDoc(t *testing.T, doc core.SourceFile, citeLine int, proposed string) (compiledRule, bool) {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	// Every section gets the same answer; only the section holding
	// citeLine can keep it (a citation outside a section is rejected).
	entry := llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Follow the rule in this file.","scope":["` + proposed + `"],"cite_start":` + itoa(citeLine) + `,"cite_end":` + itoa(citeLine) + `}]}`,
	}}
	c := newCompiler(t, st, entry, entry, entry, entry)
	run, err := c.Compile(ctx, CompileInput{ServiceID: "svc", Docs: []core.SourceFile{doc}, IndexedFiles: rulesIndexed, Watermark: "w"})
	if err != nil {
		t.Fatalf("Compile(%s): %v", doc.Path, err)
	}
	if run.ClaimsEmitted != 1 {
		t.Logf("%s: emitted %d, rejections %+v", doc.Path, run.ClaimsEmitted, run.RejectionReasons)
		return compiledRule{}, false
	}
	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v (%d claims)", err, len(claims))
	}
	links, err := st.ClaimLinks(ctx, "svc", []string{claims[0].ClaimID})
	if err != nil {
		t.Fatal(err)
	}
	return compiledRule{Scope: claims[0].Scope, Audience: audienceOf(links)[claims[0].ClaimID]}, true
}

// lineOf returns the 1-based line number of the first line containing s.
func lineOf(t *testing.T, content, s string) int {
	t.Helper()
	for i, l := range strings.Split(content, "\n") {
		if strings.Contains(l, s) {
			return i + 1
		}
	}
	t.Fatalf("%q not in content", s)
	return 0
}

// compileFromTree reads rel out of files and compiles it, citing the line
// holding "RULE".
func compileFromTree(t *testing.T, files map[string]string, rel, proposed string) (compiledRule, bool) {
	t.Helper()
	return compileRulesDoc(t, core.SourceFile{Path: rel, Content: []byte(files[rel])}, lineOf(t, files[rel], "RULE"), proposed)
}

func TestCursorRulesScopes(t *testing.T) {
	files := map[string]string{
		".cursor/rules/web.mdc":    "---\ndescription: web code\nglobs: web/src/**/*.ts, web/src/**/*.tsx\nalwaysApply: false\n---\n\n- RULE: never import from web/lib directly.\n",
		".cursor/rules/always.mdc": "---\ndescription: everywhere\nglobs: web/**\nalwaysApply: true\n---\n\n- RULE: keep functions small.\n",
		".cursor/rules/asked.mdc":  "---\ndescription: on request\n---\n\n- RULE: explain migrations.\n",
		".cursor/rules/notes.md":   "not a rule file\n",
	}
	root := writeTree(t, files)

	src := cursorRulesSource{}
	if !src.Detect(root) || src.Detect(t.TempDir()) {
		t.Fatal("Detect must see .cursor/rules and nothing in an empty root")
	}
	got, err := cursorRulesRead(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".cursor/rules/always.mdc", ".cursor/rules/asked.mdc", ".cursor/rules/web.mdc"}; !reflect.DeepEqual(paths(got), want) {
		t.Fatalf("cursorRulesRead = %v, want %v (only .mdc, sorted)", paths(got), want)
	}

	// globs become the claim scope, replacing the model's proposal.
	r, ok := compileFromTree(t, files, ".cursor/rules/web.mdc", "docs/**")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"web/src/**/*.ts", "web/src/**/*.tsx"}) || r.Audience != "" {
		t.Errorf("globs rule = %+v ok=%v, want scope [web/src/**/*.ts web/src/**/*.tsx], no audience", r, ok)
	}
	// alwaysApply is repo scope: the model's verified scope stands (and
	// wins over globs, as in Cursor).
	r, ok = compileFromTree(t, files, ".cursor/rules/always.mdc", "internal/**")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"internal/**"}) || r.Audience != "" {
		t.Errorf("alwaysApply rule = %+v ok=%v, want the model's scope [internal/**]", r, ok)
	}
	// Neither: compiled, audience manual.
	r, ok = compileFromTree(t, files, ".cursor/rules/asked.mdc", "internal/**")
	if !ok || r.Audience != core.AudienceManual {
		t.Errorf("on-request rule = %+v ok=%v, want audience manual", r, ok)
	}

	// I27 still applies to a declared scope: a glob matching no indexed
	// file rejects the claim, never a bypass.
	bad := core.SourceFile{Path: ".cursor/rules/bad.mdc", Content: []byte("---\nglobs: nowhere/**\n---\n- RULE: x\n")}
	if _, ok := compileRulesDoc(t, bad, 4, "internal/**"); ok {
		t.Error("a declared glob matching no indexed file must be rejected")
	}

	// The registry routes the source through RulesSourceCorpus.
	corpus, applied := RulesSourceCorpus(root, map[string]RulesSourceLevel{"cursor": RulesLevelError})
	if len(corpus) != 3 || applied["cursor"] != RulesLevelError {
		t.Errorf("RulesSourceCorpus = %v %v, want the three .mdc files at error", paths(corpus), applied)
	}
}

func TestCopilotInstructionsApplyTo(t *testing.T) {
	files := map[string]string{
		".github/copilot-instructions.md":           "# Repo\n\n- RULE: use conventional commits.\n",
		".github/instructions/ts.instructions.md":   "---\napplyTo: \"web/src/**/*.ts,web/lib/**\"\n---\n# TS\n\n- RULE: no default exports.\n",
		".github/instructions/all.instructions.md":  "---\napplyTo: \"**\"\n---\n- RULE: be kind.\n",
		".github/instructions/none.instructions.md": "---\ndescription: ask me\n---\n- RULE: only when asked.\n",
		".github/instructions/README.md":            "not instructions\n",
	}
	root := writeTree(t, files)

	if !(copilotRulesSource{}).Detect(root) {
		t.Fatal("Detect must see .github instructions")
	}
	got, err := copilotInstructionsRead(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".github/copilot-instructions.md", ".github/instructions/all.instructions.md", ".github/instructions/none.instructions.md", ".github/instructions/ts.instructions.md"}
	if !reflect.DeepEqual(paths(got), want) {
		t.Fatalf("copilotInstructionsRead = %v, want %v", paths(got), want)
	}

	r, ok := compileFromTree(t, files, ".github/instructions/ts.instructions.md", "docs/**")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"web/lib/**", "web/src/**/*.ts"}) {
		t.Errorf("applyTo rule = %+v ok=%v, want scope [web/lib/** web/src/**/*.ts]", r, ok)
	}
	// The repo-wide file and applyTo "**" are repo scope.
	for _, rel := range []string{".github/copilot-instructions.md", ".github/instructions/all.instructions.md"} {
		r, ok = compileFromTree(t, files, rel, "internal/**")
		if !ok || !reflect.DeepEqual(r.Scope, []string{"internal/**"}) || r.Audience != "" {
			t.Errorf("%s = %+v ok=%v, want the model's scope [internal/**]", rel, r, ok)
		}
	}
	r, ok = compileFromTree(t, files, ".github/instructions/none.instructions.md", "internal/**")
	if !ok || r.Audience != core.AudienceManual {
		t.Errorf("no-applyTo rule = %+v ok=%v, want audience manual", r, ok)
	}
}

func TestKiroSteeringInclusion(t *testing.T) {
	files := map[string]string{
		".kiro/steering/tech.md":   "# Tech\n\n- RULE: Go 1.22.\n",
		".kiro/steering/api.md":    "---\ninclusion: fileMatch\nfileMatchPattern: \"services/api/**\"\n---\n# API\n\n- RULE: every handler takes a context.\n",
		".kiro/steering/list.md":   "---\ninclusion: fileMatch\nfileMatchPattern:\n  - \"internal/a/**\"\n  - \"internal/b/**\"\n---\n- RULE: both.\n",
		".kiro/steering/manual.md": "---\ninclusion: manual\n---\n# Release\n\n- RULE: tag after the changelog.\n",
	}
	root := writeTree(t, files)
	got, err := kiroSteeringRead(root)
	if err != nil || len(got) != 4 {
		t.Fatalf("kiroSteeringRead = %v, %v; want 4 files", paths(got), err)
	}

	r, ok := compileFromTree(t, files, ".kiro/steering/api.md", "docs/**")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"services/api/**"}) {
		t.Errorf("fileMatch rule = %+v ok=%v, want scope [services/api/**]", r, ok)
	}
	r, ok = compileFromTree(t, files, ".kiro/steering/list.md", "docs/**")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"internal/a/**", "internal/b/**"}) {
		t.Errorf("block-list fileMatch rule = %+v ok=%v", r, ok)
	}
	r, ok = compileFromTree(t, files, ".kiro/steering/tech.md", "internal/**")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"internal/**"}) || r.Audience != "" {
		t.Errorf("always (default) rule = %+v ok=%v, want repo scope", r, ok)
	}
	r, ok = compileFromTree(t, files, ".kiro/steering/manual.md", "internal/**")
	if !ok || r.Audience != core.AudienceManual {
		t.Fatalf("manual rule = %+v ok=%v, want compiled with audience manual", r, ok)
	}

	// manual is compiled but not served unasked.
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedAudienceClaim(t, st, "c-manual", core.AudienceManual, []string{"internal/**"})
	seedAudienceClaim(t, st, "c-plain", "", []string{"internal/**"})
	ob, err := ObligationsForOpts(ctx, st, "svc", "internal/a/x/a.go", 10, ObligationsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ids := obligationIDs(ob); !reflect.DeepEqual(ids, []string{"c-plain"}) {
		t.Errorf("unasked serve = %v, want only the unmarked claim", ids)
	}
	ob, err = ObligationsForOpts(ctx, st, "svc", "internal/a/x/a.go", 10, ObligationsOptions{IncludeManual: true})
	if err != nil {
		t.Fatal(err)
	}
	if ids := obligationIDs(ob); !reflect.DeepEqual(ids, []string{"c-manual", "c-plain"}) {
		t.Errorf("asked serve = %v, want the manual claim too", ids)
	}
}

func TestBugbotRulesDirectoryScope(t *testing.T) {
	files := map[string]string{
		"BUGBOT.md":                     "# Review\n\n- RULE: flag unchecked errors.\n",
		"services/api/BUGBOT.md":        "# API review\n\n- RULE: flag handlers without auth.\n",
		"internal/a/.cursor/BUGBOT.md":  "# A review\n\n- RULE: flag panics.\n",
		"node_modules/x/BUGBOT.md":      "# vendored\n",
		"services/.hidden/BUGBOT.md":    "# hidden\n",
		"internal/a/.cursor/rules/x.md": "noise\n",
	}
	root := writeTree(t, files)
	got := bugbotRulesRead(root)
	want := []string{"BUGBOT.md", "internal/a/.cursor/BUGBOT.md", "services/api/BUGBOT.md"}
	if !reflect.DeepEqual(paths(got), want) {
		t.Fatalf("bugbotRulesRead = %v, want %v (skip node_modules and hidden dirs other than .cursor)", paths(got), want)
	}

	// Nested: floored to its own directory, audience review.
	r, ok := compileFromTree(t, files, "services/api/BUGBOT.md", "**/*.go")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"services/api/**/*.go"}) || r.Audience != core.AudienceReview {
		t.Errorf("nested BUGBOT.md = %+v ok=%v, want [services/api/**/*.go] audience review", r, ok)
	}
	// <dir>/.cursor/BUGBOT.md governs <dir>.
	r, ok = compileFromTree(t, files, "internal/a/.cursor/BUGBOT.md", "**/*.go")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"internal/a/**/*.go"}) {
		t.Errorf(".cursor/BUGBOT.md = %+v ok=%v, want [internal/a/**/*.go]", r, ok)
	}
	// Out of its subtree: rejected, never re-homed.
	if _, ok := compileFromTree(t, files, "services/api/BUGBOT.md", "internal/**"); ok {
		t.Error("a nested BUGBOT.md claim scoped outside its directory must be rejected")
	}

	// Served to review verbs, not the edit hook, by default.
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedAudienceClaim(t, st, "c-review", core.AudienceReview, []string{"services/api/**"})
	seedAudienceClaim(t, st, "c-plain", "", []string{"services/api/**"})
	ob, err := ObligationsForOpts(ctx, st, "svc", "services/api/h/handler.go", 10, ObligationsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if ids := obligationIDs(ob); !reflect.DeepEqual(ids, []string{"c-plain"}) {
		t.Errorf("edit serve = %v, want no review rule", ids)
	}
	ob, err = ObligationsForOpts(ctx, st, "svc", "services/api/h/handler.go", 10, ObligationsOptions{Review: true})
	if err != nil {
		t.Fatal(err)
	}
	if ids := obligationIDs(ob); !reflect.DeepEqual(ids, []string{"c-plain", "c-review"}) {
		t.Errorf("review serve = %v, want the review rule too", ids)
	}
}

func TestGeminiRulesNested(t *testing.T) {
	files := map[string]string{
		"GEMINI.md":            "# Project\n\n- RULE: run the gate.\n",
		"internal/b/GEMINI.md": "# B\n\n- RULE: b is pure.\n",
		".gemini/GEMINI.md":    "# settings dir, not a context file\n",
		"vendor/GEMINI.md":     "# vendored\n",
	}
	root := writeTree(t, files)
	got := geminiRulesRead(root)
	if want := []string{"GEMINI.md", "internal/b/GEMINI.md"}; !reflect.DeepEqual(paths(got), want) {
		t.Fatalf("geminiRulesRead = %v, want %v", paths(got), want)
	}
	// Root: the model's verified scope stands, as for a root AGENTS.md.
	r, ok := compileFromTree(t, files, "GEMINI.md", "internal/**")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"internal/**"}) || r.Audience != "" {
		t.Errorf("root GEMINI.md = %+v ok=%v", r, ok)
	}
	// Nested: floored to its subtree, exactly as a nested AGENTS.md is.
	r, ok = compileFromTree(t, files, "internal/b/GEMINI.md", "**/*.go")
	if !ok || !reflect.DeepEqual(r.Scope, []string{"internal/b/**/*.go"}) {
		t.Errorf("nested GEMINI.md = %+v ok=%v, want [internal/b/**/*.go]", r, ok)
	}
	agents, ok := compileRulesDoc(t, core.SourceFile{Path: "internal/b/AGENTS.md", Content: []byte(files["internal/b/GEMINI.md"])}, 3, "**/*.go")
	if !ok || !reflect.DeepEqual(agents, r) {
		t.Errorf("GEMINI.md compiled %+v, AGENTS.md compiled %+v — they must match", r, agents)
	}
}

func TestParseFrontmatterShapes(t *testing.T) {
	cases := []struct {
		in   string
		key  string
		want []string
	}{
		{"---\nglobs: *.ts, src/**\n---\n", "globs", []string{"*.ts", "src/**"}},
		{"---\nglobs: \"**/*.{ts,tsx}\"\n---\n", "globs", []string{"**/*.{ts,tsx}"}},
		{"---\nglobs: [\"a/**\", 'b/**']\n---\n", "globs", []string{"a/**", "b/**"}},
		{"---\nglobs:\n  - a/**\n  - \"b/**\"\n---\n", "globs", []string{"a/**", "b/**"}},
		{"---\r\nglobs: a/**\r\n---\r\n", "globs", []string{"a/**"}},
		{"---\nglobs:\n---\n", "globs", nil},
	}
	for _, c := range cases {
		fm, ok := parseFrontmatter([]byte(c.in))
		if !ok || !reflect.DeepEqual(fm[c.key], c.want) {
			t.Errorf("parseFrontmatter(%q)[%s] = %v ok=%v, want %v", c.in, c.key, fm[c.key], ok, c.want)
		}
	}
	for _, in := range []string{"# no frontmatter\n", "---\nglobs: a\n(never closed)\n"} {
		if _, ok := parseFrontmatter([]byte(in)); ok {
			t.Errorf("parseFrontmatter(%q) ok, want not-frontmatter", in)
		}
	}
}

// seedAudienceClaim stores a fresh obligations claim with an optional
// RelAudience link.
func seedAudienceClaim(t *testing.T, st store.Store, id, audience string, scope []string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: id, ServiceID: "svc", Layer: LayerObligations, Kind: KindObligation,
		Subject: "RULES.md#" + id, Rule: "rule " + id, Scope: scope,
		Tier: core.TierDocClaimed, Status: core.ClaimFresh,
	}
	links := []core.ClaimLink{{ClaimID: id, Rel: core.RelCites, TargetRef: "RULES.md#L1-L1", TargetHash: "h"}}
	if audience != "" {
		links = append(links, core.ClaimLink{ClaimID: id, Rel: core.RelAudience, TargetRef: audience})
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatal(err)
	}
}

// obligationIDs names the served claims by the id seedAudienceClaim wrote
// into their rule text.
func obligationIDs(ob Obligations) []string {
	var out []string
	for _, o := range ob.Rules {
		out = append(out, strings.TrimPrefix(o.Rule, "rule "))
	}
	sort.Strings(out)
	return out
}

// TestWithRulesSourcesMergesIntoCorpus: the compile path's corpus gains the
// client files, deduplicated, excluded by the layer's own globs, and an
// off source contributes nothing.
func TestWithRulesSourcesMergesIntoCorpus(t *testing.T) {
	root := writeTree(t, map[string]string{
		"GEMINI.md":                "# G\n",
		"third_party/x/GEMINI.md":  "# vendored\n",
		".cursor/rules/a.mdc":      "---\nglobs: a/**\n---\nbody\n",
		".kiro/steering/always.md": "# K\n",
	})
	base := []core.SourceFile{{Path: "CLAUDE.md"}, {Path: "GEMINI.md"}}
	got := WithRulesSources(base, root, []string{"third_party/**"}, map[string]RulesSourceLevel{"kiro": RulesLevelOff})
	want := []string{".cursor/rules/a.mdc", "CLAUDE.md", "GEMINI.md"}
	if !reflect.DeepEqual(paths(got), want) {
		t.Fatalf("WithRulesSources = %v, want %v", paths(got), want)
	}
	if got := WithRulesSources(base, "", nil, nil); !reflect.DeepEqual(paths(got), []string{"CLAUDE.md", "GEMINI.md"}) {
		t.Fatalf("no root must return the corpus unchanged, got %v", paths(got))
	}
}
