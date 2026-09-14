package contextbench

import (
	"encoding/json"
	"fmt"
	"os"
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
	tasks, err := LoadTasks(homeCorpusPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	root := filepath.Join("..", "..")

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

// checkAnchor re-derives one anchor against the tree rooted at root.
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
		body, err := os.ReadFile(filepath.Join(root, a.File))
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
		body, err := os.ReadFile(filepath.Join(root, a.File))
		if err != nil {
			return fmt.Errorf("token %q: %v", a.Token, err)
		}
		lines := strings.Split(string(body), "\n")
		line := 0
		for i, l := range lines {
			if strings.Contains(l, a.Needle) {
				line = i + 1
				break
			}
		}
		if line == 0 {
			return fmt.Errorf("token %q: needle %q not found in %s", a.Token, a.Needle, a.File)
		}
		if !strings.HasSuffix(a.Token, strconv.Itoa(line)) {
			return fmt.Errorf("token %q must cite line %d of %s (where %q now begins) — the ground truth is false at HEAD", a.Token, line, a.File, a.Needle)
		}
		// The substring hole: a token that is also a substring of another
		// line's citation would score a wrong answer as right.
		prefix := strings.TrimSuffix(a.Token, strconv.Itoa(line))
		for n := 1; n <= len(lines); n++ {
			if n != line && strings.Contains(prefix+strconv.Itoa(n), a.Token) {
				return fmt.Errorf("token %q is a substring of the citation for line %d as well as line %d — ambiguous; choose a form that is not", a.Token, n, line)
			}
		}
		return nil
	}
	return fmt.Errorf("token %q: unknown anchor kind %q", a.Token, a.Kind)
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
