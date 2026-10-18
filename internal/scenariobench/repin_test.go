package scenariobench

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

// O1153: a guards rule.source of the "path#Lnn" form is the guards
// gold-source token (goldsource.go), so it stays a literal line; the
// corpus gate instead names a stale pin from rule.needle, and
// `task fixtures:repin` rewrites it:
//
//	go test ./internal/scenariobench -run '^TestGuardsCorpusDetectorsExist$' -count=1 -args -repin
var repinFlag = flag.Bool("repin", false, "rewrite every stale path#Lnn guards rule.source to the line its rule.needle now begins on (task fixtures:repin, O1153)")

const repinRemedy = "run `task fixtures:repin` to rewrite it"

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

// quotedJSON is s exactly as the corpora spell a JSON string.
func quotedJSON(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// ruleSourcePin reports where r's needle begins now, for a "path#Lnn"
// source that carries one (ok false for a heading-form source, a source
// with no needle, or one citationRe does not parse — those are checked by
// ResolveCitationLine as before).
func ruleSourcePin(root string, r Rule) (path string, cited, now int, ok bool, err error) {
	m := citationRe.FindStringSubmatch(r.Source)
	if m == nil || m[2] == "" || r.Needle == "" {
		return "", 0, 0, false, nil
	}
	path = m[1]
	if cited, err = strconv.Atoi(m[2]); err != nil {
		return "", 0, 0, false, fmt.Errorf("rule source %q: %w", r.Source, err)
	}
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return "", 0, 0, false, fmt.Errorf("rule source %q: %w", r.Source, err)
	}
	if now = needleLine(string(data), r.Needle); now == 0 {
		return "", 0, 0, false, fmt.Errorf("rule source %q: needle %q not found in %s — the cited text is gone, re-derive the source by hand", r.Source, r.Needle, path)
	}
	return path, cited, now, true, nil
}

// checkRuleSourcePin is the drift half of O1153's fix: a line pin into
// ADR.md must say which text it cites, and a pin whose text moved fails
// naming where it went and the remedy.
func checkRuleSourcePin(root string, r Rule) error {
	if m := citationRe.FindStringSubmatch(r.Source); m != nil && m[2] != "" && r.Needle == "" {
		return fmt.Errorf("rule source %q pins a line of %s with no rule.needle: record the cited line's text as rule.needle, so an edit above the pin is named instead of failing the section check (O1153, O1157)", r.Source, m[1])
	}
	path, cited, now, ok, err := ruleSourcePin(root, r)
	if err != nil || !ok {
		return err
	}
	if now != cited {
		return fmt.Errorf("rule source %q is stale: its text (%q) now begins at %s:%d, not line %d — an edit above the pin moved it; %s", r.Source, r.Needle, path, now, cited, repinRemedy)
	}
	return nil
}

type pinMove struct {
	Task, Old, New, Needle string
	Line                   int
}

// repinRuleSources rewrites, in the corpus at path, every stale
// "path#Lnn" rule.source that carries a needle to the line the needle now
// begins on. Only the quoted source string changes (a derivation that
// mentions the old line is a longer string and never matches); a literal
// that more than one field holds is refused, never rewritten under another
// task. The file is written only when a pin moved.
func repinRuleSources(path, root string) ([]pinMove, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Corpus
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var moves []pinMove
	for _, task := range c.Tasks {
		if task.Rule == nil {
			continue
		}
		file, cited, now, ok, err := ruleSourcePin(root, *task.Rule)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", task.ID, err)
		}
		if !ok || now == cited {
			continue
		}
		next := file + "#L" + strconv.Itoa(now)
		q := quotedJSON(task.Rule.Source)
		if got := bytes.Count(raw, q); got != 1 {
			return nil, fmt.Errorf("%s: %s holds %d quoted %s, want 1: another field holds the same literal, re-derive it by hand", task.ID, filepath.Base(path), got, q)
		}
		raw = bytes.Replace(raw, q, quotedJSON(next), 1)
		moves = append(moves, pinMove{Task: task.ID, Old: task.Rule.Source, New: next, Needle: task.Rule.Needle, Line: now})
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

const pinADR = "# ADR\n\ninserted\nThe daemon's 8-day call log showed both agents\n"

func pinCorpus(t *testing.T, tasks string) (root, corpus string) {
	t.Helper()
	root = t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ADR.md"), []byte(pinADR), 0o644); err != nil {
		t.Fatal(err)
	}
	corpus = filepath.Join(root, "guards-x.json")
	if err := os.WriteFile(corpus, []byte(`{"version":1,"tasks":[`+tasks+`]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, corpus
}

const pinTask = `{"id":"guards-adr-014","rule":{"id":"ADR-014§x","text":"t","source":"ADR.md#L2","needle":"8-day call log"},
 "derivation":"Rule text lifted from ADR.md#L2 (the amendment)"}`

// TestCheckRuleSourcePin_StaleNamesWhereTextMoved: an insertion above a
// guards line pin fails naming the pin, where its text went and the remedy.
func TestCheckRuleSourcePin_StaleNamesWhereTextMoved(t *testing.T) {
	root, _ := pinCorpus(t, pinTask)
	err := checkRuleSourcePin(root, Rule{Source: "ADR.md#L2", Needle: "8-day call log"})
	if err == nil {
		t.Fatal("a pin two lines above its text passed")
	}
	for _, want := range []string{`"ADR.md#L2" is stale`, "now begins at ADR.md:4", "task fixtures:repin"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	if err := checkRuleSourcePin(root, Rule{Source: "ADR.md#L4", Needle: "8-day call log"}); err != nil {
		t.Errorf("the current pin failed: %v", err)
	}
}

// TestCheckRuleSourcePin_LinePinNeedsNeedle: every path#Lnn line pin, into
// ADR.md or any other file (O1157), must carry a needle; heading-form
// sources are insertion-proof and need none.
func TestCheckRuleSourcePin_LinePinNeedsNeedle(t *testing.T) {
	root, _ := pinCorpus(t, pinTask)
	for _, src := range []string{"ADR.md#L4", "docs/intent/invariants.md#L64", "AGENTS.md#L22"} {
		if err := checkRuleSourcePin(root, Rule{Source: src}); err == nil || !strings.Contains(err.Error(), "rule.needle") {
			t.Errorf("%s with no needle: err = %v, want one naming rule.needle", src, err)
		}
	}
	if err := checkRuleSourcePin(root, Rule{Source: "ADR.md#ADR-014"}); err != nil {
		t.Errorf("heading-form source: %v, want no needle required", err)
	}
}

// TestRepinRuleSources_RewritesTheSourceNotTheDerivation: -repin moves the
// quoted source, leaves the derivation prose alone, and is then a no-op.
func TestRepinRuleSources_RewritesTheSourceNotTheDerivation(t *testing.T) {
	root, corpus := pinCorpus(t, pinTask)
	moves, err := repinRuleSources(corpus, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(moves) != 1 || moves[0].New != "ADR.md#L4" {
		t.Fatalf("moves = %+v, want one ADR.md#L2 -> ADR.md#L4", moves)
	}
	raw, _ := os.ReadFile(corpus)
	if !strings.Contains(string(raw), `"source":"ADR.md#L4"`) || !strings.Contains(string(raw), "lifted from ADR.md#L2 (the amendment)") {
		t.Errorf("after repin:\n%s", raw)
	}
	if again, err := repinRuleSources(corpus, root); err != nil || len(again) != 0 {
		t.Errorf("second repin = %+v, %v; want a no-op", again, err)
	}
}

// TestRepinRuleSources_RefusesASharedLiteral: two tasks citing the same
// line are never rewritten by guess.
func TestRepinRuleSources_RefusesASharedLiteral(t *testing.T) {
	root, corpus := pinCorpus(t, pinTask+`,{"id":"guards-adr-099","rule":{"id":"ADR-014§y","text":"u","source":"ADR.md#L2","needle":"8-day call log"}}`)
	before, _ := os.ReadFile(corpus)
	if _, err := repinRuleSources(corpus, root); err == nil || !strings.Contains(err.Error(), "re-derive it by hand") {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if after, _ := os.ReadFile(corpus); !bytes.Equal(before, after) {
		t.Error("a refused repin wrote the corpus")
	}
}
