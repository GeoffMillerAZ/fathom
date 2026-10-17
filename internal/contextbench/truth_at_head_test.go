package contextbench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const homeCorpusPath = "../../testdata/contextbench/tasks-home.json"

// TestHomeCorpusTruthAtHead is the gate for PB-1 (docs/plans/bench-public.md):
// every ground-truth token in the home corpus names the evidence it was
// derived from, and that evidence still holds in THIS working tree. The
// corpus was found false at HEAD on 2026-09-01 — one rule had moved, one
// staleness verdict had inverted, three premises had moved, and one token
// was the bare substring "29" — after those tokens had already been spent
// on. This test turns that kind of drift into a red gate instead of an
// audit finding: a file edit that invalidates a ground truth fails
// `task validate` in the same change.
func TestHomeCorpusTruthAtHead(t *testing.T) {
	root := filepath.Join("..", "..")
	if *repinFlag {
		moves, err := repinLineAnchors(homeCorpusPath, root)
		if err != nil {
			t.Fatalf("repin: %v", err)
		}
		for _, m := range moves {
			t.Logf("repinned %s: %s -> %s (%q now begins at %s:%d)", m.Task, m.Old, m.New, m.Needle, m.File, m.Line)
		}
	}
	tasks, err := LoadTasks(homeCorpusPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}

	// BP-1: the two corpus-gate questions (ADR-032 §1, design §3.3). A gold
	// token already in the agent's pre-prompt context measures recall, not
	// discovery (O25); a prompt naming its own anchor file collapses
	// discovery to one `cat` (O23/E1).
	baselines := map[string]string{}
	for _, p := range corpusBaselineFiles(root) {
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading auto-loaded baseline %s: %v", p, err)
		}
		baselines[p] = string(body)
	}
	if len(baselines) == 0 {
		t.Fatal("corpusBaselineFiles found nothing to check — a repo whose agents auto-load no file at all is not this one")
	}

	for _, task := range tasks {
		// VB-36/O47: every plain token AND every answer_any_of alternative
		// is a candidate "the none arm could just quote this" leak — a
		// spelling variant sitting verbatim in the baseline is exactly as
		// much a recall shortcut as the canonical token would be.
		allTokens := append([]string(nil), task.GroundTruth...)
		for _, group := range task.AnswerAnyOf {
			allTokens = append(allTokens, group...)
		}
		for _, tok := range allTokens {
			for path, body := range baselines {
				if strings.Contains(body, tok) {
					t.Errorf("%s: ground-truth token %q appears verbatim in %s, which every `claude -p` session loads before the prompt — the task scores recall, not discovery (O25); re-derive the truth",
						task.ID, tok, filepath.Base(path))
				}
			}
		}
		if f := promptNamesAnchorFile(task); f != "" {
			t.Errorf("%s: the prompt names %s, the file its own ground truth is derived from — discovery collapses to reading that file (O23); ask the question without naming the file",
				task.ID, f)
		}

		anchors := map[string]TruthAnchor{}
		for _, a := range task.TruthAtHead {
			anchors[a.Token] = a
		}
		for _, tok := range task.GroundTruth {
			a, ok := anchors[tok]
			if !ok {
				t.Errorf("%s: ground-truth token %q has no truth_at_head anchor — say which file it was derived from", task.ID, tok)
				continue
			}
			if err := checkAnchor(root, a); err != nil {
				t.Errorf("%s: %v", task.ID, err)
			}
		}
		// VB-36: an answer_any_of group is satisfied by ANY member, so only
		// one of its members needs a derivation anchor — the other
		// spellings are the same fact, not independently-derived ones. The
		// group as a whole must still be anchored to something, or its
		// premise could quietly go false with nothing to catch it.
		for i, group := range task.AnswerAnyOf {
			anchored := false
			for _, tok := range group {
				if a, ok := anchors[tok]; ok {
					anchored = true
					if err := checkAnchor(root, a); err != nil {
						t.Errorf("%s: %v", task.ID, err)
					}
				}
			}
			if !anchored {
				t.Errorf("%s: answer_any_of[%d] %v has no truth_at_head anchor on any member — say which file the fact was derived from", task.ID, i, group)
			}
		}
		known := map[string]bool{}
		for _, g := range task.GroundTruth {
			known[g] = true
		}
		for _, group := range task.AnswerAnyOf {
			for _, tok := range group {
				known[tok] = true
			}
		}
		for tok := range anchors {
			if !known[tok] {
				t.Errorf("%s: truth_at_head names %q, which is not a ground-truth token or answer_any_of member", task.ID, tok)
			}
		}
	}
}

// TestHomeCorpusTruthAtHead_RevisionRecorded pins that the re-sweep is a
// committed revision, not a private one: the corpus names the commit and
// date at which every ground truth was last re-derived.
func TestHomeCorpusTruthAtHead_RevisionRecorded(t *testing.T) {
	raw, err := os.ReadFile(homeCorpusPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		TruthAtHead struct {
			Revision string   `json:"rederived_at_commit"`
			Date     string   `json:"rederived_at"`
			Findings []string `json:"findings"`
		} `json:"truth_at_head_revision"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.TruthAtHead.Revision == "" || doc.TruthAtHead.Date == "" {
		t.Fatal("truth_at_head_revision must name the commit and date of the last full re-derivation")
	}
	if len(doc.TruthAtHead.Findings) == 0 {
		t.Fatal("truth_at_head_revision.findings must list what the re-sweep changed (or state that nothing did)")
	}
}

// headOrCommit names what a failed check was measured against, for the
// error message: the pinned commit when BaseCommit is set, else "HEAD".
func headOrCommit(baseCommit string) string {
	if baseCommit == "" {
		return "HEAD"
	}
	return baseCommit
}

// readAnchorBody returns a.File's content: from the git blob at
// a.BaseCommit ("git show <base_commit>:<file>", run rooted at root) when
// BaseCommit is pinned, else from the current working tree exactly as
// every anchor kind read it before that field existed (wave n13).
func readAnchorBody(root string, a TruthAnchor) ([]byte, error) {
	if a.BaseCommit == "" {
		return os.ReadFile(filepath.Join(root, a.File))
	}
	cmd := exec.Command("git", "-C", root, "show", a.BaseCommit+":"+a.File)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", a.BaseCommit, a.File, err)
	}
	return out, nil
}

// checkAnchor re-derives one anchor against the tree rooted at root — or,
// when a.BaseCommit is pinned, against that historical commit's own file
// content instead (readAnchorBody).
func checkAnchor(root string, a TruthAnchor) error {
	switch a.Kind {
	case AnchorForm:
		return nil
	case AnchorPath:
		p := a.File
		if p == "" {
			p = a.Token
		}
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			return fmt.Errorf("token %q: path %s does not exist at HEAD", a.Token, p)
		}
		return nil
	case AnchorFile:
		body, err := readAnchorBody(root, a)
		if err != nil {
			return fmt.Errorf("token %q: %v", a.Token, err)
		}
		if !strings.Contains(string(body), a.Token) {
			return fmt.Errorf("token %q no longer appears in %s — the ground truth is false at HEAD", a.Token, a.File)
		}
		if a.Needle != "" {
			// A needle narrows "somewhere in the file" to "on the line that
			// carries this text" — for tokens that appear elsewhere too.
			for _, l := range strings.Split(string(body), "\n") {
				if strings.Contains(l, a.Needle) {
					if strings.Contains(l, a.Token) {
						return nil
					}
					return fmt.Errorf("token %q is not on the line of %s that carries %q — the ground truth is false at HEAD", a.Token, a.File, a.Needle)
				}
			}
			return fmt.Errorf("token %q: needle %q not found in %s", a.Token, a.Needle, a.File)
		}
		return nil
	case AnchorLine:
		body, err := readAnchorBody(root, a)
		if err != nil {
			return fmt.Errorf("token %q: %v", a.Token, err)
		}
		line := needleLine(string(body), a.Needle)
		if line == 0 {
			return fmt.Errorf("token %q: needle %q not found in %s — the cited text is gone, re-derive the ground truth by hand", a.Token, a.Needle, a.File)
		}
		// O1153: the whole cited number, never a suffix test (ADR.md#L728
		// once passed for a needle on line 28), and a stale pin names where
		// its text moved and the target that rewrites it.
		_, cited, ok := citedLine(a.Token)
		if !ok {
			return fmt.Errorf("token %q: a line anchor's token must end in the line it cites", a.Token)
		}
		if cited != line {
			return fmt.Errorf("token %q is stale: its text (%q) now begins at %s:%d, not line %d — an edit above the pin moved it; %s", a.Token, a.Needle, a.File, line, cited, repinRemedy)
		}
		// The substring hole ("CLAUDE.md#L10" inside "CLAUDE.md#L100") is
		// closed at the scorer: tokenPresent refuses a digit continuation,
		// so a line citation can never score a longer line's citation.
		// Pin that here so the corpus never silently depends on it.
		if tokenPresent(a.File+"#L"+strconv.Itoa(line)+"0", a.Token) {
			return fmt.Errorf("token %q would still match the citation for line %d0 — the scorer's digit-boundary rule is broken", a.Token, line)
		}
		return nil
	case AnchorHeading:
		// resolveHeadingLine (citation.go) looks up Needle's heading FRESH
		// against the current file (or, when a.BaseCommit is pinned,
		// against that historical commit's own file — readAnchorBody) —
		// the check is just that the heading still exists there, never a
		// stored line to compare against, so an ADR.md insertion above it
		// cannot break this the way it would an AnchorLine citation.
		body, err := readAnchorBody(root, a)
		if err != nil {
			return fmt.Errorf("token %q: %v", a.Token, err)
		}
		if _, err := resolveHeadingLine(string(body), a.Needle); err != nil {
			return fmt.Errorf("token %q: %s: %v — the ground truth is false at %s", a.Token, a.File, err, headOrCommit(a.BaseCommit))
		}
		return nil
	}
	return fmt.Errorf("token %q: unknown anchor kind %q", a.Token, a.Kind)
}

// pinnedAnchorFixtureRepo builds a tiny git repo with TWO commits: the
// first ("base") carries "## ADR-999: Original title" near the top of
// ADR.md; the second inserts an unrelated ADR record ABOVE it (the exact
// "every ADR.md insertion above it" shape H1-T07's own defect is) and
// CONSOLIDATES ADR-999 away entirely (a different record, sharing no
// substring with "ADR-999") — so a check against the current working tree
// (HEAD) finds no trace of it at all, while the pinned base commit still
// carries it. Returns the repo root and the base commit's short SHA.
func pinnedAnchorFixtureRepo(t *testing.T) (root, baseCommit string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(
			os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	adr := filepath.Join(dir, "ADR.md")
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(adr, []byte("# ADR log\n\n## ADR-999: Original title\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "--short", "HEAD")

	// The heading (and any trace of its id) at "base" is GONE at HEAD —
	// checking the working tree (no BaseCommit) must fail; checking the
	// pinned base commit must not.
	if err := os.WriteFile(adr, []byte("# ADR log\n\n## ADR-001: Inserted later\n\nnew body\n\n## ADR-500: Consolidated elsewhere\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "grow the file, consolidate the record away")

	return dir, base
}

// TestCheckAnchor_HeadingResolvesAtPinnedBaseCommit is wave n13's own
// proof (vs-pilot-5's second reading, H1-T07): a TruthAnchor pinning
// BaseCommit resolves an AnchorHeading against THAT commit's own ADR.md,
// never the current working tree — the fix for a ground truth mirrored
// from a scenariobench task that pins Subject.BaseCommit, where the
// heading (or line) it names may since have moved, or in this fixture's
// case been renamed away entirely, at HEAD.
func TestCheckAnchor_HeadingResolvesAtPinnedBaseCommit(t *testing.T) {
	root, base := pinnedAnchorFixtureRepo(t)

	unpinned := TruthAnchor{Token: "ADR.md#ADR-999", Kind: AnchorHeading, File: "ADR.md", Needle: "ADR-999"}
	if err := checkAnchor(root, unpinned); err == nil {
		t.Fatal("expected the unpinned (working-tree/HEAD) check to fail — 'ADR-999' was renamed away at HEAD")
	}

	pinned := unpinned
	pinned.BaseCommit = base
	if err := checkAnchor(root, pinned); err != nil {
		t.Errorf("expected the base_commit-pinned check to pass against %s's own ADR.md, got: %v", base, err)
	}
}

// TestCheckAnchor_FileResolvesAtPinnedBaseCommit covers the same fix for
// AnchorFile — a plain substring check pinned to a historical commit
// rather than HEAD.
func TestCheckAnchor_FileResolvesAtPinnedBaseCommit(t *testing.T) {
	root, base := pinnedAnchorFixtureRepo(t)

	unpinned := TruthAnchor{Token: "ADR-999", Kind: AnchorFile, File: "ADR.md"}
	if err := checkAnchor(root, unpinned); err == nil {
		t.Fatal("expected the unpinned check to fail — 'ADR-999' no longer appears anywhere at HEAD (consolidated away)")
	}

	pinned := unpinned
	pinned.BaseCommit = base
	if err := checkAnchor(root, pinned); err != nil {
		t.Errorf("expected the base_commit-pinned check to pass against %s's own ADR.md, got: %v", base, err)
	}
}

func TestRequiresSatisfied(t *testing.T) {
	det := Task{ID: "D"}
	needs := Task{ID: "N", Requires: []string{"compiled-layer"}}
	if !RequiresSatisfied(det, nil) {
		t.Error("a task with no requirements runs in any environment")
	}
	if RequiresSatisfied(needs, nil) {
		t.Error("a task requiring compiled-layer must not run in an environment without it")
	}
	if !RequiresSatisfied(needs, []string{"compiled-layer", "fleet"}) {
		t.Error("requirements covered by env must be satisfied")
	}
}

// TestHomeCorpusSplitsDeterministicFromCompiledLayer pins PB-12's split:
// the tasks whose premise is what the compiled obligations layer serves
// declare it, and a deterministic subset that needs no LLM-compiled state
// remains — so a stranger with a scratch daemon can score something.
func TestHomeCorpusSplitsDeterministicFromCompiledLayer(t *testing.T) {
	tasks, err := LoadTasks(homeCorpusPath)
	if err != nil {
		t.Fatal(err)
	}
	var det, compiled int
	for _, task := range tasks {
		if RequiresSatisfied(task, nil) {
			det++
		} else {
			compiled++
		}
		// A task whose prompt rests on what the compiled layer serves must
		// say so; otherwise a scratch daemon would score it as a miss.
		if strings.Contains(task.Prompt, "compiled obligations layer") && RequiresSatisfied(task, nil) {
			t.Errorf("%s: the prompt rests on the compiled obligations layer but declares no requires: [compiled-layer]", task.ID)
		}
	}
	if det == 0 || compiled == 0 {
		t.Fatalf("want both a deterministic subset and a compiled-layer subset, got %d deterministic / %d compiled", det, compiled)
	}
}

func TestMaxPredictedOrDefault(t *testing.T) {
	if got := (Task{GroundTruth: []string{"a"}}).MaxPredictedOrDefault(); got != 5 {
		t.Errorf("one gold token: want floor 5, got %d", got)
	}
	if got := (Task{GroundTruth: make([]string, 9)}).MaxPredictedOrDefault(); got != 18 {
		t.Errorf("nine gold tokens: want 18, got %d", got)
	}
	if got := (Task{GroundTruth: make([]string, 9), MaxPredicted: 4}).MaxPredictedOrDefault(); got != 4 {
		t.Errorf("explicit cap wins, got %d", got)
	}
}
