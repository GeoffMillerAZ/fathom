package scenariobench

import (
	"context"
	"testing"
)

// TestOracleAnswerMatches_H1T13FalsePass is O53 defect 5's own proof: the
// oracle's earlier defect (a plain case-insensitive substring over the
// WHOLE final answer) produced a false PASS when the gold token appeared
// only inside a quoted, rejected aside, while the answer's own concluding
// paragraph named the WRONG decision record. The fixed oracle must FAIL
// this answer against gold "ADR-010".
func TestOracleAnswerMatches_H1T13FalsePass(t *testing.T) {
	answer := "Checking the citation ADR.md#L158-L159 against HEAD.\n\n" +
		"A nearby historical note reads:\n\n" +
		"> Purely additive over ADR-010\n\n" +
		"But that note describes a different, superseded rule. At HEAD, the " +
		"companion-allowlist rule governing internal/core/model.go now lives " +
		"under a new decision record, whose distinctive identifier is **ADR-014**."

	if oracleAnswerMatches(answer, []string{"ADR-010"}, nil) {
		t.Error("expected oracleAnswerMatches to FAIL — gold ADR-010 appears only in a quoted aside, not the concluding paragraph (which names ADR-014)")
	}
	// The converse: the SAME shape, but gold matches what the answer
	// actually concludes.
	if !oracleAnswerMatches(answer, []string{"ADR-014"}, nil) {
		t.Error("expected oracleAnswerMatches to PASS for gold ADR-014 — that is the answer's own concluding claim")
	}
}

// TestOracleAnswerMatches_H1T02SpellingVariant is O53 defect 5a's own
// proof: an AnswerAnyOf group ["core.Edge", "Edge struct"] must be
// satisfied by either spelling — the false fail a plain AnswerAllOf
// literal ("core.Edge") produced when the agent legitimately wrote "Edge
// struct" for the same fact.
func TestOracleAnswerMatches_H1T02SpellingVariant(t *testing.T) {
	answer := "The typed call-edge kinds must be carried on the Edge struct, per ADR-021."
	allOf := []string{"ADR-021"}
	anyOf := [][]string{{"core.Edge", "Edge struct"}}

	if !oracleAnswerMatches(answer, allOf, anyOf) {
		t.Error("expected oracleAnswerMatches to PASS — \"Edge struct\" satisfies the answer_any_of group")
	}

	// Neither spelling present: the group must fail.
	missing := "The typed call-edge kinds must be carried somewhere, per ADR-021."
	if oracleAnswerMatches(missing, allOf, anyOf) {
		t.Error("expected oracleAnswerMatches to FAIL — neither group alternative is present")
	}

	// The OTHER spelling also satisfies the group.
	otherSpelling := "Per ADR-021, typed call-edge kinds must be carried on core.Edge, never mutating Calls."
	if !oracleAnswerMatches(otherSpelling, allOf, anyOf) {
		t.Error("expected oracleAnswerMatches to PASS — \"core.Edge\" also satisfies the group")
	}
}

// TestOracleAnswerMatches_AllOfPlusAnyOfBothRequired proves the two
// requirement kinds compose: an anyOf group being satisfied does not
// excuse a missing allOf token, and vice versa.
func TestOracleAnswerMatches_AllOfPlusAnyOfBothRequired(t *testing.T) {
	allOf := []string{"ADR-021"}
	anyOf := [][]string{{"core.Edge", "Edge struct"}}

	if oracleAnswerMatches("Edge struct only, no ADR cited", allOf, anyOf) {
		t.Error("expected FAIL — allOf token ADR-021 missing despite the group being satisfied")
	}
	if oracleAnswerMatches("ADR-021 only, no type named", allOf, anyOf) {
		t.Error("expected FAIL — anyOf group unsatisfied despite the allOf token being present")
	}
}

// TestTokenCounts_NegatedClauseDoesNotCount proves the negation reuse
// (clauseIsNegated, localization.go) directly, independent of
// concludingSpan.
func TestTokenCounts_NegatedClauseDoesNotCount(t *testing.T) {
	text := "This is not ADR-010, it is a different rule entirely."
	if tokenCounts(text, "ADR-010") {
		t.Error("expected tokenCounts to reject a negated-clause occurrence")
	}
}

// TestOracleAnswerMatches_BareNotButFailsOldFailsNewPasses is the
// fix-first review's own oracle-level proof: "not ADR-010 but ADR-014"
// must FAIL an oracle demanding ADR-010 (the bare "not" negationMarkers'
// fixed phrases alone missed) and PASS one demanding ADR-014 (the
// correction after "but" is not itself negated).
func TestOracleAnswerMatches_BareNotButFailsOldFailsNewPasses(t *testing.T) {
	answer := "Consider not ADR-010 but ADR-014 as the distinctive identifier."
	if oracleAnswerMatches(answer, []string{"ADR-010"}, nil) {
		t.Error("expected FAIL for gold ADR-010 — it is what the answer explicitly rejects")
	}
	if !oracleAnswerMatches(answer, []string{"ADR-014"}, nil) {
		t.Error("expected PASS for gold ADR-014 — it is what the answer concludes, after \"but\"")
	}
}

// TestOracleAnswerMatches_BareNever mirrors the same proof for "never".
func TestOracleAnswerMatches_BareNever(t *testing.T) {
	answer := "Typed call edges never core.Edge, always Node."
	if oracleAnswerMatches(answer, []string{"core.Edge"}, nil) {
		t.Error("expected FAIL for gold core.Edge — a bare \"never\" rejects it")
	}
}

// TestOracleAnswerMatches_NotOnlyButAlsoBothPass is the review's third
// required case at the oracle layer: "not only X but also Y" is a
// correlative idiom meaning BOTH — an oracle demanding either alone must
// PASS.
func TestOracleAnswerMatches_NotOnlyButAlsoBothPass(t *testing.T) {
	answer := "The rule covers not only ADR-010 but also ADR-014."
	if !oracleAnswerMatches(answer, []string{"ADR-010"}, nil) {
		t.Error("expected PASS for gold ADR-010 — \"not only\" is not a rejection")
	}
	if !oracleAnswerMatches(answer, []string{"ADR-014"}, nil) {
		t.Error("expected PASS for gold ADR-014")
	}
}

// TestTokenCounts_NonNegatedOccurrenceCounts is the converse — an
// ordinary assertive mention counts.
func TestTokenCounts_NonNegatedOccurrenceCounts(t *testing.T) {
	text := "The governing decision record is ADR-010."
	if !tokenCounts(text, "ADR-010") {
		t.Error("expected tokenCounts to accept a plain, non-negated mention")
	}
}

// TestTokenCounts_QuotedLineDoesNotCountAlone proves the quoted-line rule
// directly: a token appearing ONLY on a blockquote line does not count.
func TestTokenCounts_QuotedLineDoesNotCountAlone(t *testing.T) {
	text := "Background:\n\n> Purely additive over ADR-010\n\nNothing else mentions it."
	if tokenCounts(text, "ADR-010") {
		t.Error("expected tokenCounts to reject an occurrence that appears only on a quoted line")
	}
}

// TestTokenCounts_QuotedLinePlusAssertiveMentionCounts: "solely" is the
// operative word — a SECOND, non-quoted occurrence still counts even
// when a first one was quoted.
func TestTokenCounts_QuotedLinePlusAssertiveMentionCounts(t *testing.T) {
	text := "Background:\n\n> Purely additive over ADR-010\n\nThe governing record is ADR-010."
	if !tokenCounts(text, "ADR-010") {
		t.Error("expected tokenCounts to accept the second, non-quoted occurrence")
	}
}

// TestTokenCounts_BacktickWrappedDoesNotCountAlone covers the inline
// (non-blockquote) quoting shape.
func TestTokenCounts_BacktickWrappedDoesNotCountAlone(t *testing.T) {
	text := "The old code referenced `ADR-010` before the rewrite."
	if tokenCounts(text, "ADR-010") {
		t.Error("expected tokenCounts to reject a backtick-wrapped occurrence")
	}
}

// TestConcludingSpan covers the marker detection directly.
func TestConcludingSpan(t *testing.T) {
	t.Run("bold in last paragraph", func(t *testing.T) {
		answer := "Some discussion here.\n\nThe distinctive identifier is **ADR-014**."
		got := concludingSpan(answer)
		if got != "The distinctive identifier is **ADR-014**." {
			t.Errorf("concludingSpan = %q", got)
		}
	})
	t.Run("bold NOT in last paragraph is ignored", func(t *testing.T) {
		answer := "The **important** context is here.\n\nNo bold in this final paragraph though."
		if got := concludingSpan(answer); got != "" {
			t.Errorf("concludingSpan = %q, want \"\" (bold span is not in the last paragraph)", got)
		}
	})
	t.Run("the answer is marker", func(t *testing.T) {
		answer := "Some reasoning first. The answer is ADR-021, definitely."
		got := concludingSpan(answer)
		if got != "The answer is ADR-021, definitely." {
			t.Errorf("concludingSpan = %q", got)
		}
	})
	t.Run("Answer: marker", func(t *testing.T) {
		answer := "Reasoning.\n\nAnswer: ADR-021"
		got := concludingSpan(answer)
		if got != "Answer: ADR-021" {
			t.Errorf("concludingSpan = %q", got)
		}
	})
	t.Run("no marker at all", func(t *testing.T) {
		answer := "Plain prose with no conclusion marker whatsoever."
		if got := concludingSpan(answer); got != "" {
			t.Errorf("concludingSpan = %q, want \"\"", got)
		}
	})
}

// TestRunOracle_AnswerAnyOf is the end-to-end proof through RunOracle
// itself: a task carrying Oracle.AnswerAnyOf alongside AnswerAllOf scores
// via oracleAnswerMatches.
func TestRunOracle_AnswerAnyOf(t *testing.T) {
	wt := Worktree{Path: t.TempDir(), CellDir: t.TempDir()}
	task := Task{Oracle: Oracle{
		AnswerAllOf: []string{"ADR-021"},
		AnswerAnyOf: [][]string{{"core.Edge", "Edge struct"}},
	}}

	t.Run("spelling variant passes", func(t *testing.T) {
		sessions := []SessionResult{{FinalAnswer: "Per ADR-021, carried on the Edge struct."}}
		result, err := RunOracle(context.Background(), wt, task, sessions, t.TempDir())
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if !result.Passed {
			t.Errorf("expected Passed=true, got %+v", result)
		}
	})

	t.Run("neither alternative fails", func(t *testing.T) {
		sessions := []SessionResult{{FinalAnswer: "Per ADR-021, carried somewhere unnamed."}}
		result, err := RunOracle(context.Background(), wt, task, sessions, t.TempDir())
		if err != nil {
			t.Fatalf("RunOracle: %v", err)
		}
		if result.Passed {
			t.Errorf("expected Passed=false, got %+v", result)
		}
	})
}

// TestAnswerOnly_AnswerAnyOfAlone proves AnswerOnly recognizes an
// AnswerAnyOf-only oracle (no AnswerAllOf at all) as answer-only.
func TestAnswerOnly_AnswerAnyOfAlone(t *testing.T) {
	task := Task{Oracle: Oracle{AnswerAnyOf: [][]string{{"a", "b"}}}}
	if !AnswerOnly(task) {
		t.Error("expected AnswerOnly(task) = true for an AnswerAnyOf-only oracle")
	}
}
