package contextbench

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// O1153: a `kind: line` truth_at_head anchor pins the line its needle
// begins on ("ADR.md#L728"), and that literal is the scored ground truth,
// so it cannot be resolved at test time without changing what the bench
// measures. Instead the drift is named (checkAnchor says where the text
// now begins) and `task fixtures:repin` rewrites the literal:
//
//	go test ./internal/contextbench -run '^TestHomeCorpusTruthAtHead$' -count=1 -args -repin
var repinFlag = flag.Bool("repin", false, "rewrite every stale kind:line pin in tasks-home.json to the line its needle now begins on (task fixtures:repin, O1153)")

const repinRemedy = "run `task fixtures:repin` to rewrite it"

// citedLine splits a line citation ("ADR.md#L728", "CLAUDE.md:176") into
// everything before its trailing line number and that number.
func citedLine(token string) (prefix string, line int, ok bool) {
	i := len(token)
	for i > 0 && token[i-1] >= '0' && token[i-1] <= '9' {
		i--
	}
	if i == len(token) {
		return "", 0, false
	}
	n, err := strconv.Atoi(token[i:])
	if err != nil {
		return "", 0, false
	}
	return token[:i], n, true
}

// needleLine is the 1-based line of body where needle first appears, 0
// when it appears nowhere.
func needleLine(body, needle string) int {
	for i, l := range strings.Split(body, "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	return 0
}

// quotedJSON is s exactly as the fixtures spell a JSON string.
func quotedJSON(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// rewriteQuoted replaces the quoted JSON string old with new in raw, only
// when raw holds exactly want of them: a prose field that mentions the
// pin is a longer string and never matches, and a second task holding the
// same literal makes the count disagree, so the pin is refused rather than
// rewritten under another task.
func rewriteQuoted(raw []byte, old, new string, want int) ([]byte, error) {
	q := quotedJSON(old)
	if got := bytes.Count(raw, q); got != want {
		return nil, fmt.Errorf("the fixture holds %d quoted %s, the pin accounts for %d: another field holds the same literal, re-derive it by hand", got, q, want)
	}
	return bytes.ReplaceAll(raw, q, quotedJSON(new)), nil
}

type pinMove struct {
	Task, Old, New, File, Needle string
	Line                         int
}

// repinLineAnchors rewrites, in the corpus at path, every kind:line
// anchor whose needle no longer begins on the line its token cites, in
// the anchor's token and in every ground_truth / answer_any_of member of
// the same task that holds the same literal. It writes the file only when
// a pin moved and returns the moves in corpus order.
func repinLineAnchors(path, root string) ([]pinMove, error) {
	tasks, err := LoadTasks(path)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var moves []pinMove
	for _, task := range tasks {
		done := map[string]bool{}
		for _, a := range task.TruthAtHead {
			if a.Kind != AnchorLine || done[a.Token] {
				continue
			}
			body, err := readAnchorBody(root, a)
			if err != nil {
				return nil, fmt.Errorf("%s: token %q: %v", task.ID, a.Token, err)
			}
			line := needleLine(string(body), a.Needle)
			if line == 0 {
				return nil, fmt.Errorf("%s: token %q: needle %q not found in %s — the text is gone, re-derive the ground truth by hand", task.ID, a.Token, a.Needle, a.File)
			}
			prefix, n, ok := citedLine(a.Token)
			if !ok {
				return nil, fmt.Errorf("%s: token %q does not end in a line number", task.ID, a.Token)
			}
			if n == line {
				continue
			}
			want := 0
			for _, t := range task.GroundTruth {
				if t == a.Token {
					want++
				}
			}
			for _, g := range task.AnswerAnyOf {
				for _, t := range g {
					if t == a.Token {
						want++
					}
				}
			}
			for _, b := range task.TruthAtHead {
				if b.Token == a.Token {
					want++
				}
			}
			next := prefix + strconv.Itoa(line)
			if raw, err = rewriteQuoted(raw, a.Token, next, want); err != nil {
				return nil, fmt.Errorf("%s: token %q: %v", task.ID, a.Token, err)
			}
			done[a.Token] = true
			moves = append(moves, pinMove{Task: task.ID, Old: a.Token, New: next, File: a.File, Needle: a.Needle, Line: line})
		}
	}
	if len(moves) == 0 {
		return nil, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return moves, os.WriteFile(path, raw, info.Mode().Perm())
}

// pinFixture writes a one-task corpus whose H1-T07-shaped task pins
// ADR.md#L2 for needle "Backend selection" (also mentioned in prose), and
// an ADR.md where that needle begins on line 4.
func pinFixture(t *testing.T, extraTask bool) (root, corpus string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte("# ADR\n\ninserted\nsame Store interface. Backend selection\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := `{"id":"H1-T07","class":"claim_provenance","prompt":"which line?","ground_truth":["ADR.md#L2"],"scoring_rule":"exact-match",
   "source":"hand-verified: ADR.md#L2 ends 'Backend selection'",
   "truth_at_head":[{"token":"ADR.md#L2","kind":"line","file":"ADR.md","needle":"Backend selection"}]}`
	if extraTask {
		task += `,{"id":"H1-T99","class":"claim_provenance","prompt":"other","ground_truth":["ADR.md#L2"],"scoring_rule":"exact-match",
   "truth_at_head":[{"token":"ADR.md#L2","kind":"form","why":"a second task holding the same literal"}]}`
	}
	corpus = filepath.Join(root, "tasks.json")
	if err := os.WriteFile(corpus, []byte(`{"version":1,"tasks":[`+task+`]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, corpus
}

// TestCheckAnchor_LineStaleNamesWhereTextMoved: an insertion above a line
// pin fails with the pin, where its text now begins, and the remedy, not
// "the ground truth is false" (O1153).
func TestCheckAnchor_LineStaleNamesWhereTextMoved(t *testing.T) {
	root, _ := pinFixture(t, false)
	err := checkAnchor(root, TruthAnchor{Token: "ADR.md#L2", Kind: AnchorLine, File: "ADR.md", Needle: "Backend selection"})
	if err == nil {
		t.Fatal("a pin two lines above its text passed")
	}
	for _, want := range []string{`"ADR.md#L2" is stale`, "now begins at ADR.md:4", "task fixtures:repin"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
}

// TestCheckAnchor_LineComparesTheWholeNumber: the check was a suffix test,
// so "ADR.md#L14" passed for a needle on line 4.
func TestCheckAnchor_LineComparesTheWholeNumber(t *testing.T) {
	root, _ := pinFixture(t, false)
	if err := checkAnchor(root, TruthAnchor{Token: "ADR.md#L14", Kind: AnchorLine, File: "ADR.md", Needle: "Backend selection"}); err == nil {
		t.Error("ADR.md#L14 passed for a needle on line 4")
	}
	if err := checkAnchor(root, TruthAnchor{Token: "ADR.md#L4", Kind: AnchorLine, File: "ADR.md", Needle: "Backend selection"}); err != nil {
		t.Errorf("the exact pin failed: %v", err)
	}
}

// TestRepinLineAnchors_RewritesThePinNotTheProse: -repin moves the ground
// truth and its anchor together, leaves prose that mentions the old line
// alone, and is a no-op once current.
func TestRepinLineAnchors_RewritesThePinNotTheProse(t *testing.T) {
	root, corpus := pinFixture(t, false)
	moves, err := repinLineAnchors(corpus, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(moves) != 1 || moves[0].Old != "ADR.md#L2" || moves[0].New != "ADR.md#L4" {
		t.Fatalf("moves = %+v, want one ADR.md#L2 -> ADR.md#L4", moves)
	}
	tasks, err := LoadTasks(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if got := tasks[0].GroundTruth[0]; got != "ADR.md#L4" {
		t.Errorf("ground truth = %q, want ADR.md#L4", got)
	}
	if err := checkAnchor(root, tasks[0].TruthAtHead[0]); err != nil {
		t.Errorf("after repin: %v", err)
	}
	raw, _ := os.ReadFile(corpus)
	if !strings.Contains(string(raw), "hand-verified: ADR.md#L2 ends") {
		t.Error("repin rewrote prose that only mentions the old line")
	}
	again, err := repinLineAnchors(corpus, root)
	if err != nil || len(again) != 0 {
		t.Errorf("second repin = %+v, %v; want a no-op", again, err)
	}
}

// TestRepinLineAnchors_RefusesASharedLiteral: another task holding the same
// literal is never rewritten under it.
func TestRepinLineAnchors_RefusesASharedLiteral(t *testing.T) {
	root, corpus := pinFixture(t, true)
	before, _ := os.ReadFile(corpus)
	if _, err := repinLineAnchors(corpus, root); err == nil || !strings.Contains(err.Error(), "re-derive it by hand") {
		t.Fatalf("err = %v, want a refusal", err)
	}
	after, _ := os.ReadFile(corpus)
	if !bytes.Equal(before, after) {
		t.Error("a refused repin wrote the fixture")
	}
}
