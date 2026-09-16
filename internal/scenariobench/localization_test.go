package scenariobench

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseAnswerFiles_OrderAndDedup(t *testing.T) {
	answer := "Most likely: `pkg/cmd/run/download/download.go`. Also check " +
		"pkg/cmd/run/download/http.go and, less likely, pkg/cmd/run/download/zip.go. " +
		"Re-reading pkg/cmd/run/download/download.go confirms it (mentioned twice)."
	got := ParseAnswerFiles(answer)
	want := []string{
		"pkg/cmd/run/download/download.go",
		"pkg/cmd/run/download/http.go",
		"pkg/cmd/run/download/zip.go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseAnswerFiles = %v, want %v (first-mention order, deduplicated)", got, want)
	}
}

func TestParseAnswerFiles_BareFilenamesNeedKnownExtension(t *testing.T) {
	// "e.g." and "i.e." must never parse as a one-letter-extension file;
	// a bare filename with a recognized extension (no slash) still counts.
	answer := "e.g. see errors.go for the sentinel, i.e. do not touch apiPlatform.Download."
	got := ParseAnswerFiles(answer)
	want := []string{"errors.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseAnswerFiles = %v, want %v", got, want)
	}
}

func TestParseAnswerFiles_NumberedAndBulletedLists(t *testing.T) {
	answer := "1. ledger.go\n2. errors.go\n- payment.go\n* main.go"
	got := ParseAnswerFiles(answer)
	want := []string{"ledger.go", "errors.go", "payment.go", "main.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseAnswerFiles = %v, want %v", got, want)
	}
}

func TestParseAnswerFiles_Empty(t *testing.T) {
	if got := ParseAnswerFiles(""); len(got) != 0 {
		t.Errorf("expected no files from an empty answer, got %v", got)
	}
	if got := ParseAnswerFiles("I am not sure which file is responsible."); len(got) != 0 {
		t.Errorf("expected no files from a prose answer naming none, got %v", got)
	}
}

func TestParseAnswerFiles_NegatedMentionDemoted(t *testing.T) {
	answer := "The bug is not in ledger.go, check handler.go."
	got := ParseAnswerFiles(answer)
	want := []string{"handler.go", "ledger.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseAnswerFiles = %v, want %v (the negated mention demoted below the non-negated one)", got, want)
	}
}

func TestParseAnswerFiles_NumberedListOverridesProseOrder(t *testing.T) {
	answer := "Possibly related: payment.go was touched recently.\n1. ledger.go\n2. errors.go"
	got := ParseAnswerFiles(answer)
	want := []string{"ledger.go", "errors.go", "payment.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseAnswerFiles = %v, want %v (the explicit list leads; the earlier prose mention trails)", got, want)
	}
}

func TestParseAnswerFiles_Deterministic(t *testing.T) {
	answer := "Check ledger.go, then errors.go, then payment.go."
	a := ParseAnswerFiles(answer)
	b := ParseAnswerFiles(answer)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("ParseAnswerFiles is not deterministic: %v vs %v", a, b)
	}
}

func TestScoreLocalization_HitAt1(t *testing.T) {
	gold := []string{"ledger.go", "errors.go"}
	reading := ScoreLocalization("The bug is in ledger.go, specifically Balance.", gold)
	if !reading.HitAt1 {
		t.Error("expected HitAt1 true when the top-ranked file is gold")
	}
	if !reading.HitAt5 {
		t.Error("expected HitAt5 true whenever HitAt1 is true")
	}
	if reading.GoldFiles != 2 {
		t.Errorf("GoldFiles = %d, want 2", reading.GoldFiles)
	}
	if reading.AnsweredFiles != 1 {
		t.Errorf("AnsweredFiles = %d, want 1", reading.AnsweredFiles)
	}
	if reading.Recall != 0.5 {
		t.Errorf("Recall = %v, want 0.5", reading.Recall)
	}
	if reading.Precision != 1.0 {
		t.Errorf("Precision = %v, want 1.0", reading.Precision)
	}
}

func TestScoreLocalization_NegatedMentionNeverHitAt1(t *testing.T) {
	gold := []string{"handler.go"}
	reading := ScoreLocalization("The bug is not in ledger.go, check handler.go.", gold)
	if !reading.HitAt1 {
		t.Error("expected HitAt1 true — the negated mention (ledger.go) must rank below handler.go, the answer's real top pick")
	}
	if reading.Recall != 1.0 {
		t.Errorf("Recall = %v, want 1.0 (handler.go is still named and matched)", reading.Recall)
	}
}

func TestScoreLocalization_MissAt1HitAt5(t *testing.T) {
	gold := []string{"errors.go"}
	answer := "1. ledger.go\n2. payment.go\n3. main.go\n4. http.go\n5. errors.go\n6. zip.go"
	reading := ScoreLocalization(answer, gold)
	if reading.HitAt1 {
		t.Error("expected HitAt1 false — the top-ranked file is not gold")
	}
	if !reading.HitAt5 {
		t.Error("expected HitAt5 true — a gold file appears within the top 5")
	}
}

func TestScoreLocalization_MissEntirely(t *testing.T) {
	gold := []string{"errors.go"}
	reading := ScoreLocalization("Try ledger.go and payment.go.", gold)
	if reading.HitAt1 || reading.HitAt5 {
		t.Error("expected both hit flags false when gold is never named")
	}
	if reading.Recall != 0 {
		t.Errorf("Recall = %v, want 0", reading.Recall)
	}
	if reading.Precision != 0 {
		t.Errorf("Precision = %v, want 0", reading.Precision)
	}
}

func TestScoreLocalization_EmptyAnswer(t *testing.T) {
	reading := ScoreLocalization("", []string{"errors.go"})
	if reading.NotMeasurable != "" {
		t.Errorf("an empty ANSWER is a legitimate zero, not unmeasurable: %+v", reading)
	}
	if reading.HitAt1 || reading.HitAt5 || reading.Recall != 0 || reading.Precision != 0 || reading.AnsweredFiles != 0 {
		t.Errorf("expected an all-zero reading, got %+v", reading)
	}
}

func TestScoreLocalization_NoGoldIsNotMeasurable(t *testing.T) {
	reading := ScoreLocalization("ledger.go", nil)
	if reading.NotMeasurable == "" {
		t.Error("expected NotMeasurable when the task carries no gold files")
	}
}

func TestScoreLocalization_PathSuffixMatch(t *testing.T) {
	// sameFile (readings_overlap.go) matches an absolute-vs-repo-relative
	// mismatch by suffix — ScoreLocalization must use the same rule.
	gold := []string{"pkg/cmd/run/download/download.go"}
	reading := ScoreLocalization("The fix is in download.go.", gold)
	if !reading.HitAt1 {
		t.Error("expected a bare filename to path-suffix-match a longer gold path")
	}
}

func TestScoreLocalization_DuplicateGoldMatchCountsOnce(t *testing.T) {
	// Two different answered tokens both matching the SAME gold file must
	// not inflate recall past 1.0 or double-count the intersection.
	gold := []string{"errors.go"}
	reading := ScoreLocalization("pkg/a/errors.go and pkg/b/errors.go both look related.", gold)
	if reading.Recall != 1.0 {
		t.Errorf("Recall = %v, want 1.0 (one gold file, matched)", reading.Recall)
	}
	if reading.AnsweredFiles != 2 {
		t.Errorf("AnsweredFiles = %d, want 2 (two distinct answered tokens)", reading.AnsweredFiles)
	}
}

func TestScoreLocalization_Deterministic(t *testing.T) {
	gold := []string{"ledger.go", "errors.go"}
	answer := "Check ledger.go first, then errors.go, then payment.go."
	a := ScoreLocalization(answer, gold)
	b := ScoreLocalization(answer, gold)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("ScoreLocalization is not deterministic: %+v vs %+v", a, b)
	}
}

// TestClauseIsNegated_BareNotButConstruction is the fix-first review's
// own proof: negationMarkers' fixed phrases alone missed a bare "not X
// but Y" — X is negated (rejected), but the correction ("but Y") means Y
// is NOT negated, even though it sits in the same clause as "not".
func TestClauseIsNegated_BareNotButConstruction(t *testing.T) {
	text := "Consider not ADR-010 but ADR-014 as the distinctive identifier."
	xPos := strings.Index(text, "ADR-010")
	yPos := strings.Index(text, "ADR-014")
	if !clauseIsNegated(text, xPos) {
		t.Error("expected ADR-010 (before \"but\") to be negated")
	}
	if clauseIsNegated(text, yPos) {
		t.Error("expected ADR-014 (after \"but\") to NOT be negated")
	}
}

// TestClauseIsNegated_BareNever covers "never" alone, with no fixed
// negationMarkers phrase involved at all.
func TestClauseIsNegated_BareNever(t *testing.T) {
	text := "Typed call edges never core.Edge, always Node."
	pos := strings.Index(text, "core.Edge")
	if !clauseIsNegated(text, pos) {
		t.Error("expected a bare \"never\" to negate what follows it")
	}
}

// TestClauseIsNegated_BareNor covers "nor".
func TestClauseIsNegated_BareNor(t *testing.T) {
	text := "The rule names neither ADR-010 nor ADR-014 directly."
	pos := strings.Index(text, "ADR-014")
	if !clauseIsNegated(text, pos) {
		t.Error("expected \"nor\" to negate what follows it")
	}
}

// TestClauseIsNegated_NotOnlyButAlsoIsNotNegation is the review's third
// required case: "not only X but also Y" is a correlative idiom meaning
// BOTH X and Y — never a rejection of X — so neither mention is negated.
func TestClauseIsNegated_NotOnlyButAlsoIsNotNegation(t *testing.T) {
	text := "The rule covers not only ADR-010 but also ADR-014."
	xPos := strings.Index(text, "ADR-010")
	yPos := strings.Index(text, "ADR-014")
	if clauseIsNegated(text, xPos) {
		t.Error("expected \"not only\" to NOT negate ADR-010 — it is a correlative idiom, not a rejection")
	}
	if clauseIsNegated(text, yPos) {
		t.Error("expected ADR-014 (\"but also\") to NOT be negated either")
	}
}
