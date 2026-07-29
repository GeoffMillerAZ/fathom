package main

import (
	"path/filepath"
	"testing"
	"time"
)

func withTempStampDir(t *testing.T) {
	t.Helper()
	orig := guardStampDir
	guardStampDir = filepath.Join(t.TempDir(), "fathom-guard-stamps")
	t.Cleanup(func() { guardStampDir = orig })
}

func TestGuardStamp_WriteThenRead(t *testing.T) {
	withTempStampDir(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	writeGuardStamp("sess-1", "svc", "a.go", []string{ruleKey("r1", "src1", "")}, false, now)

	st, ok := readGuardStamp("sess-1", "svc", "a.go", now.Add(1*time.Minute))
	if !ok {
		t.Fatal("expected a stamp to be readable shortly after write")
	}
	if len(st.RuleKeys) != 1 || st.RuleKeys[0] != ruleKey("r1", "src1", "") {
		t.Errorf("RuleKeys = %v", st.RuleKeys)
	}
}

func TestGuardStamp_AbsentWhenNeverWritten(t *testing.T) {
	withTempStampDir(t)
	if _, ok := readGuardStamp("sess-1", "svc", "never-written.go", time.Now()); ok {
		t.Fatal("expected no stamp for a file that was never stamped")
	}
}

func TestGuardStamp_ExpiresAfterTTL(t *testing.T) {
	withTempStampDir(t)
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	writeGuardStamp("sess-1", "svc", "a.go", []string{"k1"}, false, now)

	if _, ok := readGuardStamp("sess-1", "svc", "a.go", now.Add(guardStampTTL+time.Minute)); ok {
		t.Fatal("expected the stamp to be expired past the TTL")
	}
	if _, ok := readGuardStamp("sess-1", "svc", "a.go", now.Add(guardStampTTL-time.Minute)); !ok {
		t.Fatal("expected the stamp to still be valid just under the TTL")
	}
}

// TestGuardStamp_SessionScoped proves two different sessions on the SAME
// file never share a stamp key — the leakage bound design §2.3 requires.
func TestGuardStamp_SessionScoped(t *testing.T) {
	withTempStampDir(t)
	now := time.Now()
	writeGuardStamp("sess-A", "svc", "a.go", []string{"k1"}, false, now)
	if _, ok := readGuardStamp("sess-B", "svc", "a.go", now); ok {
		t.Fatal("a stamp written under sess-A must not be readable under sess-B")
	}
	if _, ok := readGuardStamp("sess-A", "svc", "a.go", now); !ok {
		t.Fatal("the same session should still read its own stamp")
	}
}

func TestDedupAgainstStamp_IdenticalRules_FullSuppression(t *testing.T) {
	stamp := guardStamp{RuleKeys: []string{ruleKey("r1", "s1", "")}, IndexStale: false}
	newRules := []semanticObligationLike{{Rule: "r1", Src: "s1"}}
	res := dedupAgainstStamp(stamp, newRules, false)
	if len(res.DeltaRules) != 0 || res.SuppressedCount != 1 || res.StaleChanged {
		t.Errorf("res = %+v, want full suppression", res)
	}
}

func TestDedupAgainstStamp_NewRule_ShowsOnlyDelta(t *testing.T) {
	stamp := guardStamp{RuleKeys: []string{ruleKey("r1", "s1", "")}, IndexStale: false}
	newRules := []semanticObligationLike{{Rule: "r1", Src: "s1"}, {Rule: "r2", Src: "s2"}}
	res := dedupAgainstStamp(stamp, newRules, false)
	if len(res.DeltaRules) != 1 || res.DeltaRules[0].Rule != "r2" || res.SuppressedCount != 1 {
		t.Errorf("res = %+v, want exactly r2 as delta", res)
	}
}

func TestDedupAgainstStamp_StalenessChangeAlwaysSurfaced(t *testing.T) {
	stamp := guardStamp{RuleKeys: []string{ruleKey("r1", "s1", "")}, IndexStale: false}
	newRules := []semanticObligationLike{{Rule: "r1", Src: "s1"}}
	res := dedupAgainstStamp(stamp, newRules, true) // staleness flipped
	if !res.StaleChanged {
		t.Error("expected StaleChanged=true even though rules are otherwise identical")
	}
}

func TestDedupAgainstStamp_ModuleTaggedRuleDisambiguated(t *testing.T) {
	// Same Rule text, different Module -- must NOT be treated as already-served.
	stamp := guardStamp{RuleKeys: []string{ruleKey("shared rule text", "s1", "module-a")}}
	newRules := []semanticObligationLike{{Rule: "shared rule text", Src: "s1", Module: "module-b"}}
	res := dedupAgainstStamp(stamp, newRules, false)
	if len(res.DeltaRules) != 1 {
		t.Errorf("res = %+v, want the module-b rule to show as a new delta rule", res)
	}
}
