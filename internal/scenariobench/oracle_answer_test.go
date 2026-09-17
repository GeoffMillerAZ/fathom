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

// TestTokenCounts_InlineBacktickCounts is the fix-first review's own PIN:
// a backticked identifier inside otherwise ordinary prose — the way a
// model routinely formats an identifier, path, or symbol name — must
// COUNT. Only a line that is ENTIRELY a quotation (isQuotedPosition) is
// excluded; an inline backtick span within a longer sentence never was
// that, by design.
func TestTokenCounts_InlineBacktickCounts(t *testing.T) {
	text := "The old code referenced `ADR-010` before the rewrite."
	if !tokenCounts(text, "ADR-010") {
		t.Error("expected an inline backtick-wrapped identifier in ordinary prose to count")
	}
}

// TestTokenCounts_ConcludingSpanBacktickedIdentifierCounts is the exact
// case the review names: "whose distinctive identifier is `ADR-014`" —
// a backticked identifier inside the concluding span must count.
func TestTokenCounts_ConcludingSpanBacktickedIdentifierCounts(t *testing.T) {
	text := "whose distinctive identifier is `ADR-014`"
	if !tokenCounts(text, "ADR-014") {
		t.Error("expected a backticked identifier inside the concluding span to count")
	}
}

// TestOracleAnswerMatches_BacktickedConclusionPasses proves the same PIN
// at the oracle layer: an answer whose entire conclusion is a backticked
// identifier must PASS for that gold token.
func TestOracleAnswerMatches_BacktickedConclusionPasses(t *testing.T) {
	answer := "Some reasoning here.\n\nwhose distinctive identifier is `ADR-014`"
	if !oracleAnswerMatches(answer, []string{"ADR-014"}, nil) {
		t.Error("expected PASS — a backticked identifier in the concluding prose is the answer's own claim, not a citation")
	}
}

// TestTokenCounts_WholeLineBacktickFenceStillExcluded covers the
// surviving exclusion: a line that is NOTHING but a backtick-fenced
// quotation (unlike an inline span inside a longer sentence) is still a
// quoted LINE.
func TestTokenCounts_WholeLineBacktickFenceStillExcluded(t *testing.T) {
	text := "Background:\n\n`ADR-010`\n\nNothing else mentions it."
	if tokenCounts(text, "ADR-010") {
		t.Error("expected a line that is nothing but a backtick-fenced quotation to still be excluded")
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

// --- wave-n13: vs-pilot-5 second reading, the 22-cell scorer_false_fail
// class (§ defects / § voids) --------------------------------------------
//
// Every fixture below is the MINIMAL final-answer text copied from the
// named raw session's own last assistant turn (never the session log
// itself — see docs/bench/README.md's raw-session convention). Each one
// is a documented false FAIL in the second reading: the ledger scored it
// FAIL, the reader confirmed it a correct, substantive answer. Zero
// scorer_false_pass cells were found in that run, so every EXISTING
// negation/quote regression test above (TestTokenCounts_QuotedLineDoes-
// NotCountAlone, TestOracleAnswerMatches_H1T13FalsePass, the bare-not/
// never/nor family, …) must still FAIL exactly as before — this section
// only ADDS coverage, it narrows nothing already proven.

// TestOracleAnswerMatches_H1T01QuotedBlockquoteRuleCounts is H1-T01/
// full/t1's own false fail: the prompt asked the model to quote
// internal/CLAUDE.md's extractor-registration rule in full, and it did —
// but the sole "golden-test" occurrence sits on that quoted (">") line,
// which the old isQuotedPosition excluded unconditionally. The blockquote
// itself carries backticked identifiers and a **bold** lead-in — the
// same way the source rule text is formatted — so it must count as the
// answer's own assertion, not a passing aside.
func TestOracleAnswerMatches_H1T01QuotedBlockquoteRuleCounts(t *testing.T) {
	answer := "`internal/CLAUDE.md:35-36` governs this:\n\n" +
		"> **Extractor**: implement `extractor.Extractor`, register by extension in `fusion.New()`, add its ext→kind in `connector`, golden-test it.\n\n" +
		"So a new extractor must be registered in two places:\n" +
		"1. **`fusion.New()`** — registered there by file extension.\n" +
		"2. **`connector`** — given an ext→kind mapping entry there.\n\n" +
		"(It should also implement the `extractor.Extractor` interface and have a golden test, but the two *registration* points the rule names are `fusion.New()` and `connector`.)"

	if !oracleAnswerMatches(answer, []string{"fusion.New", "golden-test"}, nil) {
		t.Error("expected PASS — 'golden-test' appears in the quoted rule text the prompt asked the model to quote in full")
	}
}

// TestOracleAnswerMatches_H1T02BacktickBrokenPhraseCounts is H1-T02/
// full/t1's own false fail: the model's own markdown wraps "Edge" in
// backticks — "the `Edge` struct's `Kind EdgeKind` field" — so the plain
// two-word phrase "Edge struct" never appears as one contiguous
// substring. The backtick between the two words must not break the
// match.
func TestOracleAnswerMatches_H1T02BacktickBrokenPhraseCounts(t *testing.T) {
	answer := "Typed call-edge kinds (`EdgeDirect`, `EdgeIfaceDispatch`, `EdgeUnresolved`, `EdgeFuncValue`) " +
		"must be carried on the `Edge` struct's `Kind EdgeKind` field (internal/core/model.go:371-376). " +
		"This comes from **ADR-021 §3** (internal/core/model.go:326, 343)."

	allOf := []string{"ADR-021"}
	anyOf := [][]string{{"core.Edge", "Edge struct"}}
	if !oracleAnswerMatches(answer, allOf, anyOf) {
		t.Error("expected PASS — the backtick between `Edge` and struct must not break the 'Edge struct' literal match")
	}
}

// TestOracleAnswerMatches_H1T04MultiParagraphConclusionAndUnpunctuated-
// Negation is H1-T04/full/t1's own false fail, TWO compounding bugs at
// once: concludingSpan's old bold-last-paragraph rule returned ONLY the
// last paragraph, missing "DEMO-H1" (stated, bolded, in paragraph 1); and
// within that last paragraph, clauseIsNegated's old clause boundary
// (only ".!?,") let an EARLIER, unrelated "does not exist" bleed — across
// an em dash and a colon, with no comma or period anywhere between them
// — into the bolded "not found" conclusion, marking it negated.
func TestOracleAnswerMatches_H1T04MultiParagraphConclusionAndUnpunctuatedNegation(t *testing.T) {
	answer := "Row **DEMO-H1** is the one that fails on the merits. `cmdVerify` (DEMO-H2) genuinely exists at " +
		"`cmd/fathom/verify.go:37` and passes the symbol/tier checks — its only failure is a plan-hygiene issue " +
		"(bare `skip_test: true` with no reason), not a code-truth problem.\n\n" +
		"`GetGovernedClaimByRepro`, however, does not exist anywhere under `internal/store` (or the index at all) " +
		"— the one-phrase reason: **symbol not found / never defined** (it's a ghost accessor, not actually wired)."

	if !oracleAnswerMatches(answer, []string{"DEMO-H1", "not found"}, nil) {
		t.Error("expected PASS — DEMO-H1 (paragraph 1) and the bolded, non-negated 'not found' (paragraph 2) are both the answer's own assertions")
	}
}

// TestOracleAnswerMatches_H1T04SingleParagraphUnpunctuatedNegation is
// H1-T04/serena/t2's own false fail — the same clause-boundary bug,
// entirely within a SINGLE unpunctuated paragraph: "does not exist ...
// (not just `internal/store`) — **symbol not found**" carries no comma
// or period anywhere before the bolded conclusion, only an em dash.
func TestOracleAnswerMatches_H1T04SingleParagraphUnpunctuatedNegation(t *testing.T) {
	answer := "**Row A (DEMO-H1)** is the one that doesn't hold up: `GetGovernedClaimByRepro` does not exist " +
		"anywhere in the repo (not just `internal/store`) — **symbol not found**."

	if !oracleAnswerMatches(answer, []string{"DEMO-H1", "not found"}, nil) {
		t.Error("expected PASS — the bolded 'not found' is the answer's own conclusion, not negated by the earlier, unrelated 'does not exist'")
	}
}

// TestOracleAnswerMatches_H1T11QuotedBlockquoteRuleCounts is H1-T11's own
// false fail, hit independently by grep/serena/gopls/graphify (t1, plus
// graphify t2): both required substrings sit only inside the model's own
// quoted excerpt of docs/guides/fleet-index.md — a blockquote carrying
// its own **bold** emphasis and backticked identifiers, so (exactly like
// H1-T01) it counts as the answer's own evidence, not a passing aside.
func TestOracleAnswerMatches_H1T11QuotedBlockquoteRuleCounts(t *testing.T) {
	answer := "Yes, the proposed line directly contradicts existing rules in fathom's CLAUDE.md and the guide it points to.\n\n" +
		"**In CLAUDE.md itself** (the Index-first row):\n" +
		"> \"refresh via the `ingest` MCP tool, **never a shell `fathom ingest` against the shared store**\"\n\n" +
		"**In `docs/guides/fleet-index.md`** (the full text that row points to), point 2 is even more explicit about why:\n" +
		"> \"**Refresh through the server, never the CLI.** SQLite (ADR-020) allows many concurrent readers, but writes still serialize " +
		"— **a shell `fathom ingest` racing the daemon's own writes can hit its busy_timeout and fail with a lock error.**\"\n\n" +
		"So the proposed line contradicts both the blanket prohibition and the stated reason."

	if !oracleAnswerMatches(answer, []string{"lock error", "busy_timeout"}, nil) {
		t.Error("expected PASS — both tokens appear verbatim inside the model's own quoted, formatted evidence")
	}
}

// TestOracleAnswerMatches_H1T13EarlyVerdictLineCounts is H1-T13/full/t2's
// own false fail: the model's overall verdict ("verdict: stale") is its
// own first paragraph, while the required ADR-010 citation sits, bolded,
// inside a LATER paragraph's own nested quotation of the current ADR.md
// text — concludingSpan's old bold-last-paragraph rule returned only
// that later paragraph, silently dropping the first-line verdict.
func TestOracleAnswerMatches_H1T13EarlyVerdictLineCounts(t *testing.T) {
	answer := "verdict: stale\n\n" +
		"The rule's words (\"new hot-loop tool must be added\") now live at line 176, not 158-159; that line reads " +
		"\"...new hot-loop tool must be added. Purely additive over **ADR-010** — removing the...\"."

	if !oracleAnswerMatches(answer, []string{"verdict: stale", "ADR-010"}, nil) {
		t.Error("expected PASS — the opening 'verdict: stale' line and the later bolded ADR-010 citation are both the answer's own claims")
	}
}
