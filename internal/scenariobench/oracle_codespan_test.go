package scenariobench

import "testing"

// TestOracleAnswer_SingleTokenCodeSpanLineIsAnAssertion (O100): an answer
// whose first line is nothing but the backticked symbol — the direct
// answer-first shape both arms used on lookups-py PY-03 — matches the
// oracle token; only a wholly-quoted PROSE line (spaces inside the span)
// or a plain blockquote stays excluded as cited evidence.
func TestOracleAnswer_SingleTokenCodeSpanLineIsAnAssertion(t *testing.T) {
	answer := "`APIRoute.__init__`\n\nThe only call outside `fastapi/utils.py` is at `fastapi/routing.py:603`."
	if !oracleAnswerMatches(answer, []string{"APIRoute.__init__"}, nil) {
		t.Fatal("a single-token code-span line must count as the answer's own assertion")
	}
	quotedProse := "> the rule says use `Charge` here\n\nSo the answer is Refund."
	if oracleAnswerMatches(quotedProse, []string{"Charge"}, nil) {
		t.Fatal("a token only inside a plain-prose quoted line must still not count")
	}
	wholeLineProse := "`use Charge for this path`\n\nRefund is the one."
	if tokenCounts(wholeLineProse, "Charge") {
		t.Fatal("a wholly backticked prose line (spaces inside) stays excluded")
	}
	later := "Background:\n\n`Charge`\n\nRefund is the one."
	if tokenCounts(later, "Charge") {
		t.Fatal("a single-token code-span line that is NOT the answer's first line stays a citation")
	}
}
