package main

// instructionregions_test.go — ADR-059 (docs/plans/agents-md.md AG-2..AG-4,
// AG-6): one stanza, in AGENTS.md, for every harness; a root CLAUDE file
// carries at most a marked @AGENTS.md import and is never created; rewire
// moves an old CLAUDE.md stanza into AGENTS.md with a receipt undo reverses.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// ownedRegionBody is the body between fathom's markers in content, "" when
// there is no region.
func ownedRegionBody(content string) string {
	i := strings.Index(content, fathomStanzaBeginPrefix)
	if i < 0 {
		return ""
	}
	return stanzaBodyOf(content[i:])
}

// TestInstructionRegionsWriteAgentsMDOnlyForEveryHarness pins AG-2: every
// --harness (none included) plans the stanza into AGENTS.md alone, and a
// repo with no CLAUDE file gets none.
func TestInstructionRegionsWriteAgentsMDOnlyForEveryHarness(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, id := range append(harness.IDs(), initHarnessNone) {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			targets, err := initTargetsFor(id, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			spec, _ := initWiringSpecFor(root, "svc", testBinPath, targets, hooksScopeAuto)
			if len(spec.Regions) != 1 || spec.Regions[0].File != "AGENTS.md" || !isStanzaBody(spec.Regions[0].Body) {
				t.Fatalf("regions = %+v, want the one stanza region in AGENTS.md", spec.Regions)
			}
			if _, err := wiring.ApplyWiring(root, spec, wiring.ApplyOptions{}); err != nil {
				t.Fatal(err)
			}
			for _, f := range claudeFiles() {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); !os.IsNotExist(err) {
					t.Errorf("init created %s (err=%v); fathom never creates a CLAUDE file", f, err)
				}
			}
			if !strings.Contains(readRepoFile(t, root, "AGENTS.md"), `service "svc"`) {
				t.Error("AGENTS.md does not carry the stanza")
			}
		})
	}
}

// TestClaudeImportRegionGoesIntoTheFirstPresentClaudeFile pins AG-3's
// precedence and the relative import from .claude/.
func TestClaudeImportRegionGoesIntoTheFirstPresentClaudeFile(t *testing.T) {
	cases := []struct {
		present  []string
		wantFile string
		wantBody string
	}{
		{[]string{"CLAUDE.md"}, "CLAUDE.md", "@AGENTS.md"},
		{[]string{".claude/CLAUDE.md"}, ".claude/CLAUDE.md", "@../AGENTS.md"},
		{[]string{"CLAUDE.local.md"}, "CLAUDE.local.md", "@AGENTS.md"},
		{[]string{"CLAUDE.local.md", ".claude/CLAUDE.md", "CLAUDE.md"}, "CLAUDE.md", "@AGENTS.md"},
		{[]string{"CLAUDE.local.md", ".claude/CLAUDE.md"}, ".claude/CLAUDE.md", "@../AGENTS.md"},
	}
	for _, c := range cases {
		root := t.TempDir()
		for _, f := range c.present {
			writeRepoFile(t, root, f, "# house rules\n")
		}
		r, ok := claudeImportRegion(root)
		if !ok || r.File != c.wantFile || r.Body != c.wantBody || r.Remove {
			t.Errorf("present %v: region = %+v ok=%v, want %s carrying %s", c.present, r, ok, c.wantFile, c.wantBody)
		}
		regions, _ := instructionRegions(root, harness.ClaudeCode, "svc", false)
		if _, err := wiring.ApplyWiring(root, wiring.Spec{Regions: regions}, wiring.ApplyOptions{}); err != nil {
			t.Fatal(err)
		}
		got := readRepoFile(t, root, c.wantFile)
		if !strings.HasPrefix(got, "# house rules\n") || ownedRegionBody(got) != c.wantBody {
			t.Errorf("present %v: %s = %q, want the prose kept and one region holding %s", c.present, c.wantFile, got, c.wantBody)
		}
		for _, f := range c.present {
			if f != c.wantFile && readRepoFile(t, root, f) != "# house rules\n" {
				t.Errorf("present %v: %s changed; only the first present CLAUDE file gets the import", c.present, f)
			}
		}
	}
	if _, ok := claudeImportRegion(t.TempDir()); ok {
		t.Error("a repo with no CLAUDE file got an import region")
	}
}

// TestClaudeImportRegionNoneWhenAnImportAlreadyExists: an unfenced import
// the adopter wrote is the import; fathom adds none. A fenced one is prose.
func TestClaudeImportRegionNoneWhenAnImportAlreadyExists(t *testing.T) {
	root := t.TempDir()
	prior := "# house rules\n\n" + "@AGENTS.md\n"
	writeRepoFile(t, root, "CLAUDE.md", prior)
	regions, _ := instructionRegions(root, harness.ClaudeCode, "svc", false)
	if _, err := wiring.ApplyWiring(root, wiring.Spec{Regions: regions}, wiring.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := readRepoFile(t, root, "CLAUDE.md"); got != prior {
		t.Fatalf("CLAUDE.md with its own import was changed:\n%q", got)
	}

	fenced := t.TempDir()
	writeRepoFile(t, fenced, "CLAUDE.md", "# rules\n\n```\n"+"@AGENTS.md\n```\n\nSee `@AGENTS.md`.\n")
	if r, _ := claudeImportRegion(fenced); r.Remove {
		t.Error("a fenced or inline-code import counted as an import")
	}
	if !hasUnfencedImport("see @./AGENTS.md here\n", "CLAUDE.md") || !hasUnfencedImport("@../AGENTS.md\n", ".claude/CLAUDE.md") {
		t.Error("an import spelled ./AGENTS.md, or ../AGENTS.md from .claude/, was not recognized")
	}
	if hasUnfencedImport("@AGENTS.md\n", ".claude/CLAUDE.md") {
		t.Error("@AGENTS.md from .claude/ names .claude/AGENTS.md, not the root file")
	}
}

// legacyTwoCopyRepo writes the pre-ADR-059 layout: the same v1-marked
// stanza in CLAUDE.md (after house prose) and in AGENTS.md.
func legacyTwoCopyRepo(t *testing.T, withAgents bool) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "demo")
	stanza := fathomStanza("demo", false)
	writeRepoFile(t, root, "CLAUDE.md", "# house rules\n\nkeep it tidy\n\n"+stanza+"\n")
	if withAgents {
		writeRepoFile(t, root, "AGENTS.md", stanza+"\n")
	}
	return root
}

// TestRewireMovesTheClaudeStanzaToAnImportAndUndoRestoresIt pins AG-4.
func TestRewireMovesTheClaudeStanzaToAnImportAndUndoRestoresIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, withAgents := range []bool{true, false} {
		root := legacyTwoCopyRepo(t, withAgents)
		priorClaude := readRepoFile(t, root, "CLAUDE.md")

		var out, errb strings.Builder
		unreachable := "http://127.0.0.1:1"
		if code := cmdFleetReconcile([]string{"--url", unreachable, "--repo", root}, &out, &errb); code != 0 {
			t.Fatalf("dry-run exit %d: %s%s", code, out.String(), errb.String())
		}
		if !strings.Contains(out.String(), "moves it into AGENTS.md") {
			t.Errorf("dry-run does not announce the move:\n%s", out.String())
		}
		out.Reset()
		if code := cmdFleetReconcile([]string{"--url", unreachable, "--apply", "--repo", root}, &out, &errb); code != 0 {
			t.Fatalf("apply exit %d: %s%s", code, out.String(), errb.String())
		}
		claude := readRepoFile(t, root, "CLAUDE.md")
		if !strings.HasPrefix(claude, "# house rules\n\nkeep it tidy\n\n") || ownedRegionBody(claude) != "@AGENTS.md" {
			t.Fatalf("withAgents=%v: CLAUDE.md after rewire = %q, want the prose and one import region", withAgents, claude)
		}
		if agents := readRepoFile(t, root, "AGENTS.md"); !strings.Contains(agents, `service "demo"`) {
			t.Fatalf("withAgents=%v: AGENTS.md does not carry the stanza after the move:\n%s", withAgents, agents)
		}

		var undoOut, undoErr strings.Builder
		if code := cmdFleetUndo([]string{"--repo", root, "--last"}, &undoOut, &undoErr); code != 0 {
			t.Fatalf("undo exit %d: %s%s", code, undoOut.String(), undoErr.String())
		}
		if got := readRepoFile(t, root, "CLAUDE.md"); got != priorClaude {
			t.Errorf("withAgents=%v: undo did not restore the CLAUDE.md stanza:\n got %q\nwant %q", withAgents, got, priorClaude)
		}
		if !withAgents {
			if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
				t.Errorf("undo left the AGENTS.md the move created (err=%v)", err)
			}
		}
	}
}

// TestRewireBlocksADriftedClaudeStanza: a tracked region edited by hand is
// never rewritten to the import (law 15).
func TestRewireBlocksADriftedClaudeStanza(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// A tracked (v3) stanza in CLAUDE.md, then a hand edit inside it.
	claudeRegion := wiring.Region{File: "CLAUDE.md", Syntax: wiring.HTMLMarkers, Version: fathomStanzaVersion, Body: fathomStanzaBody("demo", false), AdoptUntracked: true}
	agentsRegion := claudeRegion
	agentsRegion.File = "AGENTS.md"
	if _, err := wiring.ApplyWiring(root, wiring.Spec{Regions: []wiring.Region{claudeRegion, agentsRegion}}, wiring.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(readRepoFile(t, root, "CLAUDE.md"), "Read files to *edit* them", "Read files by hand", 1)
	writeRepoFile(t, root, "CLAUDE.md", edited)

	var out, errb strings.Builder
	if code := cmdFleetReconcile([]string{"--url", "http://127.0.0.1:1", "--apply", "--repo", root}, &out, &errb); code != 1 {
		t.Fatalf("apply over a drifted CLAUDE.md exit = %d, want 1:\n%s%s", code, out.String(), errb.String())
	}
	if got := readRepoFile(t, root, "CLAUDE.md"); got != edited {
		t.Fatal("a drifted CLAUDE.md region was rewritten")
	}
	if !strings.Contains(out.String(), "BLOCKED: CLAUDE.md") {
		t.Errorf("the block is not named:\n%s", out.String())
	}
}

// TestRewireRemovesFathomsRegionWhenTheAdopterImportsAgentsMD pins
// Region.Remove on the rewire path: an adopter-written import plus fathom's
// old stanza region leaves the import alone and removes the region.
func TestRewireRemovesFathomsRegionWhenTheAdopterImportsAgentsMD(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "demo")
	writeRepoFile(t, root, "CLAUDE.md", "# house rules\n\n"+"@AGENTS.md\n\n"+fathomStanza("demo", false)+"\n")
	writeRepoFile(t, root, "AGENTS.md", fathomStanza("demo", false)+"\n")
	var out, errb strings.Builder
	if code := cmdFleetReconcile([]string{"--url", "http://127.0.0.1:1", "--apply", "--repo", root}, &out, &errb); code != 0 {
		t.Fatalf("apply exit %d:\n%s%s", code, out.String(), errb.String())
	}
	if got := readRepoFile(t, root, "CLAUDE.md"); got != "# house rules\n\n"+"@AGENTS.md\n" {
		t.Fatalf("CLAUDE.md = %q, want the adopter's import and no fathom region", got)
	}
}

// TestInitGeminiWritesContextFileNameNotAGeminiStanza pins AG-6.
func TestInitGeminiWritesContextFileNameNotAGeminiStanza(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	targets, err := initTargetsFor(harness.GeminiCLI, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	spec, _ := initWiringSpecFor(root, "svc", testBinPath, targets, hooksScopeAuto)
	for _, r := range spec.Regions {
		if r.File == "GEMINI.md" {
			t.Fatal("init planned a stanza into GEMINI.md")
		}
	}
	if _, err := wiring.ApplyWiring(root, spec, wiring.ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	settings := readRepoFile(t, root, ".gemini/settings.json")
	if !strings.Contains(settings, `"fileName": [`) || !strings.Contains(settings, `"AGENTS.md"`) || !strings.Contains(settings, `"GEMINI.md"`) {
		t.Fatalf(".gemini/settings.json lacks context.fileName [AGENTS.md, GEMINI.md]:\n%s", settings)
	}
	if _, err := os.Stat(filepath.Join(root, "GEMINI.md")); !os.IsNotExist(err) {
		t.Errorf("init wrote GEMINI.md (err=%v)", err)
	}
}

// TestInitKeepsTheStanzaWhenClaudeAndAgentsAreOneFile: a repository whose
// CLAUDE file and AGENTS.md are one file (a symlink either way, the common
// way to serve both harnesses) keeps the stanza. Claude Code reads the
// stanza through the CLAUDE file already; an import region planned there
// would be written into AGENTS.md itself, replacing the stanza with a
// self-import (found by the ADR-059 critic on a scratch adopter).
func TestInitKeepsTheStanzaWhenClaudeAndAgentsAreOneFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cases := []struct {
		name       string
		real, link string // link is a symlink to real, relative to link's directory
		target     string
	}{
		{"AGENTS.md links to CLAUDE.md", "CLAUDE.md", "AGENTS.md", "CLAUDE.md"},
		{"CLAUDE.md links to AGENTS.md", "AGENTS.md", "CLAUDE.md", "AGENTS.md"},
		{".claude/CLAUDE.md links to AGENTS.md", "AGENTS.md", ".claude/CLAUDE.md", "../AGENTS.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeRepoFile(t, root, c.real, "# house rules\n")
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, c.link)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(c.target, filepath.Join(root, filepath.FromSlash(c.link))); err != nil {
				t.Fatal(err)
			}
			targets, err := initTargetsFor(harness.ClaudeCode, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			for pass := 1; pass <= 2; pass++ {
				spec, _ := initWiringSpecFor(root, "svc", testBinPath, targets, hooksScopeAuto)
				if _, err := wiring.ApplyWiring(root, spec, wiring.ApplyOptions{}); err != nil {
					t.Fatalf("pass %d: %v", pass, err)
				}
				got := readRepoFile(t, root, c.real)
				if n := strings.Count(got, fathomStanzaBeginPrefix); n != 1 {
					t.Fatalf("pass %d: %d fathom regions in the one file:\n%s", pass, n, got)
				}
				if body := ownedRegionBody(got); !isStanzaBody(body) || isImportBody(body) {
					t.Fatalf("pass %d: the one file's region is %q, want the stanza (never a self-import)", pass, body)
				}
			}
			if fi, err := os.Lstat(filepath.Join(root, filepath.FromSlash(c.link))); err != nil || fi.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s is no longer the adopter's link (err=%v)", c.link, err)
			}
		})
	}
}

// TestRewireKeepsTheStanzaOfALinkedPair: an adopter an older fathom wired
// through a CLAUDE.md -> AGENTS.md link carries one stanza region, which is
// AGENTS.md's own; rewire must not "move" it into an import and delete the
// stanza from the one file both harnesses read.
func TestRewireKeepsTheStanzaOfALinkedPair(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "demo")
	writeRepoFile(t, root, "AGENTS.md", "# house rules\n\n"+fathomStanza("demo", false)+"\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	var out, errb strings.Builder
	if code := cmdFleetReconcile([]string{"--url", "http://127.0.0.1:1", "--apply", "--repo", root}, &out, &errb); code != 0 {
		t.Fatalf("apply exit %d:\n%s%s", code, out.String(), errb.String())
	}
	got := readRepoFile(t, root, "AGENTS.md")
	if body := ownedRegionBody(got); !isStanzaBody(body) {
		t.Fatalf("rewire replaced the linked pair's stanza with %q:\n%s", body, out.String())
	}
	if strings.Contains(out.String(), "still carries the stanza") {
		t.Fatalf("rewire called AGENTS.md's own region a legacy CLAUDE.md copy:\n%s", out.String())
	}
}
