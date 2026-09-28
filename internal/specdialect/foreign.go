package specdialect

import (
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

// foreignTask turns one foreign checkbox row into a verify.Task.
//
// body is the row text after the dialect's own task id and markers were
// removed; it may carry a fathom annotation block (design §3.1), which is
// read by the plan grammar's OWN tokenizer — the row is re-framed as a
// native checklist line and handed to verify.ParseMarkdownPlan, so there
// is one quoting rule and every grammar key (symbol, oracle, gone,
// certified_by, weight, accept, …) means exactly what it means natively.
//
// The author-owned `id:` in an annotation wins; otherwise the dialect's id
// is the task id. SpecIDs are cleared: a foreign bracket label ([US1],
// [P]) is the scaffold's vocabulary, not a fathom spec id.
//
// When the annotation carries no contract, the first repo-relative file
// path the text names becomes a lexical-oracle contract ("the file this
// task names exists and is non-empty") — the design's anchor source for
// Spec Kit and OpenSpec — and every other named path becomes a ctx
// pointer. A Markdown path is never made a contract (a lexical oracle on a
// .md requires a pattern, and a document that merely exists proves
// nothing). No path, no annotation: the task has no contract and verifies
// no-contract, exactly like a native row; nothing is guessed.
func foreignTask(id, body string, claimed bool, line int) verify.Task {
	check := " "
	if claimed {
		check = "x"
	}
	parsed := verify.ParseMarkdownPlan("", []byte("- ["+check+"] "+body+"\n"))
	var t verify.Task
	if len(parsed.Tasks) == 1 {
		t = parsed.Tasks[0]
	} else {
		// An empty body: the native row regex needs a title.
		t = verify.Task{Claimed: claimed, Weight: 1}
	}
	if !annotatedID(body) {
		t.ID = id
	}
	t.Line = line
	t.SpecIDs = nil
	if !hasContract(t) {
		paths := namedPaths(t.Title)
		for i, p := range paths {
			if i == 0 && !strings.HasSuffix(strings.ToLower(p), ".md") {
				t.Oracle = "lexical"
				t.OraclePath = p
				continue
			}
			t.Ctx = append(t.Ctx, p)
		}
	}
	return t
}

// annotatedID reports whether body's annotation block sets id:.
func annotatedID(body string) bool {
	ann, _, _ := verify.ParseAnnotation(body)
	return ann["id"] != ""
}

// hasContract reports whether a task carries any contract the gate can
// evaluate: a symbol, an oracle, a gone: form, or a certificate.
func hasContract(t verify.Task) bool {
	return t.Expect.Symbol != "" || t.Oracle != "" || t.Gone != "" || len(t.Expect.CertifiedBy) > 0
}

// namedPathRe matches a repo-relative file path in prose: at least one
// directory segment and a file extension, optionally in backticks. It
// needs a start-of-text, space, "(" or backtick before it, so a URL's
// "//host/x.y" never matches.
var namedPathRe = regexp.MustCompile("(?:^|[\\s(`\"'])((?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]*[A-Za-z0-9_-]\\.[A-Za-z0-9]{1,10})")

// namedPaths returns the repo-relative file paths text names, in order,
// deduplicated; paths climbing out of the repo ("..") are dropped.
func namedPaths(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range namedPathRe.FindAllStringSubmatch(text, -1) {
		p := strings.TrimLeft(m[1], "./")
		if p == "" || strings.Contains(p, "..") || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// checkboxLine is the shared foreign-row shape: bullet, a [ ]/[x]/[X]
// checkbox, then the row text. Group 1 is everything before the bracket
// (its length gives the column), group 2 the checkbox char, group 3 the
// text.
var checkboxLine = regexp.MustCompile(`^(\s*[-*]\s+)\[([ xX])\]\s+(.*\S)\s*$`)
