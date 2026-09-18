package scenariobench

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

// findingsloop_test.go — FL-24 (docs/plans/findings-loop.md, vision 30 /
// ADR-040 §6): testdata/scenariobench/findings-loop.json is committed but
// NOT run by this wave (the CLI it documents — fathom findings import, the
// pre-edit "open findings" block — is built by other waves against
// findings-loop.md sections A-D and is not yet present in this tree).
// TestFindingsLoopCorpusParses proves the corpus itself is not just prose:
// it loads and validates through the same gate every other scenario corpus
// passes (LoadScenarioCorpus — arms, oracle, derivation, prediction-per-arm,
// self-host exclude_paths, SB-10), then checks every review-kit projection
// event embedded in each task's subject.setup names a REAL anchor: the file
// exists in this tree and the anchor's own digest matches that file's
// current sha256 at HEAD. Anchor digests can be validated in-tree here
// (unlike a corpus whose subject is a foreign upstream repo not checked out
// locally) because this corpus's subject is fathom's own repository — the
// same self-hosting shape guards-fathom.json already uses — so "the file at
// the corpus commit" IS the file this test can read directly.

const findingsLoopCorpusPath = "../../testdata/scenariobench/findings-loop.json"

// findingsLoopEvent mirrors the review-kit projection event shape vision
// 30 §3.3 documents (schemaVersion/kind/outcome/producer/subject/
// visibility/sourceId/sourceVersion/payloadDigest/findingId/observedAt/
// anchor/rule) — only the fields this test checks are typed; anything else
// decodes into nothing and is ignored (this is a fixture reader, not the
// eventual strict importer FL-4 builds).
type findingsLoopEvent struct {
	SourceID string `json:"sourceId"`
	Anchor   struct {
		Path   string `json:"path"`
		Digest string `json:"digest"`
		Start  int    `json:"start"`
		End    int    `json:"end"`
		Symbol string `json:"symbol"`
	} `json:"anchor"`
	Rule struct {
		ID    string `json:"id"`
		Class string `json:"class"`
	} `json:"rule"`
}

type findingsLoopOutbox struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Events        []findingsLoopEvent `json:"events"`
}

// heredocBodyRE pulls a `cat > path <<'MARKER'` (or `>>`) heredoc's body out
// of one subject.setup shell command string: everything between the first
// newline and a line that is exactly MARKER.
var heredocHeaderRE = regexp.MustCompile(`^cat\s+>>?\s*(\S+)\s*<<'([A-Za-z0-9_]+)'\s*$`)

// extractHeredocBody returns the body of the first heredoc in cmd whose
// destination file is destPath (exact match on the heredoc's own header
// line, never a substring match against the body — a config.toml append
// whose VALUE happens to mention "projection.json" must never be mistaken
// for the outbox file itself), or ("", false) when cmd carries no such
// heredoc.
func extractHeredocBody(cmd, destPath string) (marker, body string, ok bool) {
	lines := strings.Split(cmd, "\n")
	if len(lines) == 0 {
		return "", "", false
	}
	m := heredocHeaderRE.FindStringSubmatch(lines[0])
	if m == nil || m[1] != destPath {
		return "", "", false
	}
	marker = m[2]
	var b strings.Builder
	for _, line := range lines[1:] {
		if line == marker {
			return marker, b.String(), true
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return marker, "", false // marker never closed
}

// findOutboxEvents scans a task's subject.setup for the projection.json
// heredoc and decodes it, failing the test with a precise reason when the
// corpus's own setup commands do not carry one.
func findOutboxEvents(t *testing.T, setup []string) []findingsLoopEvent {
	t.Helper()
	for _, cmd := range setup {
		_, body, ok := extractHeredocBody(cmd, ".reviewkit/outbox/projection.json")
		if !ok {
			continue
		}
		var ob findingsLoopOutbox
		if err := json.Unmarshal([]byte(body), &ob); err != nil {
			t.Fatalf("subject.setup projection.json heredoc did not parse as JSON: %v\nbody:\n%s", err, body)
		}
		if ob.SchemaVersion != 1 {
			t.Fatalf("projection outbox schemaVersion = %d, want 1", ob.SchemaVersion)
		}
		return ob.Events
	}
	return nil
}

func fileLineCount(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		n++
	}
	return n
}

func TestFindingsLoopCorpusParses(t *testing.T) {
	c, err := LoadScenarioCorpus(findingsLoopCorpusPath)
	if err != nil {
		t.Fatalf("LoadScenarioCorpus(%s): %v", findingsLoopCorpusPath, err)
	}
	if len(c.Tasks) < 4 {
		t.Fatalf("findings-loop.json has %d tasks, want >= 4 (FL-24)", len(c.Tasks))
	}
	if _, ok := c.Arms["full"]; !ok {
		t.Fatal(`findings-loop.json is missing the "full" arm`)
	}
	if _, ok := c.Arms["control"]; !ok {
		t.Fatal(`findings-loop.json is missing the "control" arm`)
	}

	seenRuleIDs := map[string]bool{}
	for _, task := range c.Tasks {
		events := findOutboxEvents(t, task.Subject.Setup)
		if len(events) == 0 {
			t.Errorf("task %s: subject.setup carries no projection.json outbox heredoc (or it decoded to zero events)", task.ID)
			continue
		}
		if task.Rule == nil || task.Rule.ID == "" {
			t.Errorf("task %s: no rule.id — a findings-loop task must cite the finding it is testing", task.ID)
			continue
		}
		if task.Rule.ID == "none" {
			// A control-shaped task (vision 27 §1): the treatment arm must
			// predict no effect, and no outbox event may anchor to a file the
			// task's own oracle touches — otherwise "no effect" is a guess,
			// not a structural fact.
			if !isNoEffectPrediction(task.Predictions["full"]) {
				t.Errorf("task %s: rule.id none but the full arm does not predict no effect", task.ID)
			}
			for _, det := range task.Oracle.Detectors {
				for _, path := range goPathsIn(det.Command) {
					for i := range events {
						if events[i].Anchor.Path == path {
							t.Errorf("task %s: rule.id none but outbox event %s anchors to %s, which the oracle touches", task.ID, events[i].Rule.ID, path)
						}
					}
				}
			}
			continue
		}

		var matched *findingsLoopEvent
		for i := range events {
			if events[i].Rule.ID == task.Rule.ID {
				matched = &events[i]
				break
			}
		}
		if matched == nil {
			t.Errorf("task %s: rule.id %s names no event in its own subject.setup outbox", task.ID, task.Rule.ID)
			continue
		}
		seenRuleIDs[task.Rule.ID] = true

		anchorPath := moduleRoot + "/" + matched.Anchor.Path
		raw, err := os.ReadFile(anchorPath)
		if err != nil {
			t.Errorf("task %s: anchor path %s does not exist in this tree: %v", task.ID, matched.Anchor.Path, err)
			continue
		}
		sum := sha256.Sum256(raw)
		gotDigest := hex.EncodeToString(sum[:])
		if gotDigest != matched.Anchor.Digest {
			t.Errorf("task %s: anchor %s digest = %s, want %s (file has drifted since the corpus was authored — re-anchor the finding, per vision 30 §3.3 point 6, anchor_current)", task.ID, matched.Anchor.Path, gotDigest, matched.Anchor.Digest)
		}
		if matched.Anchor.Start < 1 || matched.Anchor.End < matched.Anchor.Start {
			t.Errorf("task %s: anchor %s has a degenerate line range [%d,%d]", task.ID, matched.Anchor.Path, matched.Anchor.Start, matched.Anchor.End)
		}
		if n := fileLineCount(anchorPath); n > 0 && matched.Anchor.End > n {
			t.Errorf("task %s: anchor %s end line %d exceeds the file's %d lines", task.ID, matched.Anchor.Path, matched.Anchor.End, n)
		}

		// finding_cited: the task's own oracle must be able to score
		// whether the agent cited this finding's rule id (or the served
		// "open findings on this file" line) in its final answer.
		cited := false
		for _, group := range task.Oracle.AnswerAnyOf {
			for _, alt := range group {
				if alt == task.Rule.ID {
					cited = true
				}
			}
		}
		for _, tok := range task.Oracle.AnswerAllOf {
			if tok == task.Rule.ID {
				cited = true
			}
		}
		if !cited {
			t.Errorf("task %s: oracle carries no finding_cited check for rule id %s (answer_any_of/answer_all_of)", task.ID, task.Rule.ID)
		}

		// finding_addressed: at least one detector beyond change-present
		// and the no-unrelated-rewrite scope check must exist to score the
		// remedy itself.
		named := map[string]bool{}
		for _, d := range task.Oracle.Detectors {
			named[d.Name] = true
		}
		if !named["change-present"] {
			t.Errorf("task %s: missing the change-present detector every corpus task here carries", task.ID)
		}
		if len(task.Oracle.Detectors) < 3 {
			t.Errorf("task %s: has %d detectors, want >= 3 (change-present, finding_addressed, no-unrelated-rewrite)", task.ID, len(task.Oracle.Detectors))
		}
	}
	if len(seenRuleIDs) < 4 {
		t.Errorf("findings-loop.json: only %d distinct rule ids were exercised by a task, want >= 4 (FL-24's >=4-task floor)", len(seenRuleIDs))
	}
}

// goPathsIn returns every repo-relative .go path a detector command names.
func goPathsIn(command string) []string {
	var out []string
	for _, m := range goPathRe.FindAllString(command, -1) {
		out = append(out, strings.ReplaceAll(m, `\\.go`, ".go"))
	}
	return out
}

var goPathRe = regexp.MustCompile(`[A-Za-z0-9_./-]+(?:\\\\)?\.go`)
