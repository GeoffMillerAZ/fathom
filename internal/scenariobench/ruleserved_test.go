package scenariobench

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeBeatsLog(t *testing.T, dir string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, "hooks.jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func beatLine(kind string, at time.Time, meta map[string]string) string {
	m := "{"
	first := true
	for k, v := range meta {
		if !first {
			m += ","
		}
		m += fmt.Sprintf("%q:%q", k, v)
		first = false
	}
	m += "}"
	return fmt.Sprintf(`{"channel":"edit-hook","kind":%q,"at":%q,"meta":%s}`, kind, at.UTC().Format(time.RFC3339Nano), m)
}

func TestRuleServed_NoLog(t *testing.T) {
	got := RuleServed(filepath.Join(t.TempDir(), "missing.jsonl"), "I31", 5)
	if got != nil {
		t.Fatalf("got %v, want nil (no log)", got)
	}
}

func TestRuleServed_ExplicitTurnBeforeClaim(t *testing.T) {
	base := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	path := writeBeatsLog(t, t.TempDir(), []string{
		beatLine("seen", base, map[string]string{"phase": "post"}),
		beatLine("answered", base.Add(time.Second), map[string]string{"rule": "I31", "turn": "2"}),
	})
	got := RuleServed(path, "I31", 5)
	if got == nil || !*got {
		t.Fatalf("got %v, want true", got)
	}
}

func TestRuleServed_ExplicitTurnAfterClaim(t *testing.T) {
	base := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	path := writeBeatsLog(t, t.TempDir(), []string{
		beatLine("answered", base, map[string]string{"rule": "I31", "turn": "10"}),
	})
	got := RuleServed(path, "I31", 5)
	if got == nil || *got {
		t.Fatalf("got %v, want false", got)
	}
}

func TestRuleServed_OrdinalFallbackNoTurnMeta(t *testing.T) {
	base := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	path := writeBeatsLog(t, t.TempDir(), []string{
		// No "turn" meta: the first matching beat stands in for turn 1.
		beatLine("answered", base, map[string]string{"rules": "I2,I31"}),
	})
	got := RuleServed(path, "I31", 2)
	if got == nil || !*got {
		t.Fatalf("got %v, want true (ordinal turn 1 < firstClaimTurn 2)", got)
	}
}

func TestRuleServed_NoMatchingBeat(t *testing.T) {
	base := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	path := writeBeatsLog(t, t.TempDir(), []string{
		beatLine("answered", base, map[string]string{"rule": "I2"}),
	})
	got := RuleServed(path, "I31", 5)
	if got == nil || *got {
		t.Fatalf("got %v, want false (no beat names I31)", got)
	}
}
