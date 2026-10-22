package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

func TestRuleContextRenderedChangesResend(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex} {
		for _, change := range []string{"text", "tag", "citation", "drift"} {
			t.Run(id+"/"+change, func(t *testing.T) {
				root, transport := ruleDeliveryEnv(t)
				file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
				rule := cbRule("Sort outputs.")
				rule["claim_id"] = "rule-one"
				transport.rules = []any{rule}
				if got := ruleDeliveryPost(t, id, root, file, "", "", ""); !strings.Contains(got, "Sort outputs.") {
					t.Fatalf("first delivery: %q", got)
				}
				switch change {
				case "text":
					rule["rule"] = "Always sort outputs."
				case "tag":
					rule["status"] = "stale"
				case "citation":
					rule["citation_status"] = map[string]any{"state": "stale"}
				case "drift":
					rule["code_drifted"] = true
				}
				if got := ruleDeliveryPost(t, id, root, file, "", "", ""); !strings.Contains(got, rule["rule"].(string)) || strings.Contains(got, "already shown") {
					t.Fatalf("changed rendered rule withheld: %q", got)
				}
				if got := ruleDeliveryPost(t, id, root, file, "", "", ""); strings.Contains(got, "\n• "+rule["rule"].(string)) {
					t.Fatalf("unchanged rendered rule repeated: %q", got)
				}
			})
		}
	}
}

func TestRuleContextDistinctSuffix(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex} {
		root := cbHookEnv(t)
		now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
		in := harness.HookInput{SessionID: "session-a"}
		a := semantic.Obligation{ClaimID: "rule-one", Rule: "Sort outputs.", Src: "AGENTS.md#L1-L1", Tier: "doc-claimed"}
		b := semantic.Obligation{ClaimID: "rule-two", Rule: "Inject clocks.", Src: "AGENTS.md#L2-L2", Tier: "doc-claimed"}
		emit := func(target string, rules []semantic.Obligation) string {
			t.Helper()
			var sb strings.Builder
			writeRuleBullets(&sb, rules, "svc")
			out, d := dedupeHookRules(id, root, in, "post-edit", target, "svc", sb.String(), rules, now)
			defer d.close()
			d.commit(now)
			return out
		}
		if got := emit("one", []semantic.Obligation{a, a}); strings.Count(got, "Sort outputs.") != 1 || strings.Contains(got, "already shown") {
			t.Fatalf("%s duplicates counted as prior rules: %q", id, got)
		}
		if got := emit("two", []semantic.Obligation{a, a, b, b}); strings.Contains(got, "Sort outputs.") || strings.Count(got, "Inject clocks.") != 1 || !strings.HasSuffix(got, "(1 rule already shown in this session)") {
			t.Fatalf("%s singular distinct count: %q", id, got)
		}
		if got := emit("three", []semantic.Obligation{a, a, b, b}); strings.Contains(got, "Sort outputs.") || strings.Contains(got, "Inject clocks.") || !strings.HasSuffix(got, "(2 rules already shown in this session)") {
			t.Fatalf("%s plural distinct count: %q", id, got)
		}
	}
}

func TestRuleContextCapacityEvictsOldest(t *testing.T) {
	for _, id := range []string{harness.ClaudeCode, harness.Codex} {
		t.Run(id, func(t *testing.T) {
			root, _ := ruleDeliveryEnv(t)
			file := writeEditedFile(t, root, "pkg/a.go", "package pkg\n")
			now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
			previous := nowFunc
			nowFunc = func() time.Time { return now }
			t.Cleanup(func() { nowFunc = previous })
			state := hookDedupeState{Version: 2, Epoch: now.Format(time.RFC3339), At: now, Digests: map[string]hookDedupeEntry{}}
			oldest := hookDigest("oldest-rule")
			state.Digests[oldest] = hookDedupeEntry{Digest: oldest, EmittedAt: now.Add(-time.Hour)}
			// Legacy per-rule records stored their identity as the digest.
			for i := 0; i < hookDedupeMaxEntries; i++ {
				key := hookDigest(fmt.Sprintf("old-rule-%d", i))
				state.Digests[key] = hookDedupeEntry{Digest: key, EmittedAt: now}
				raw, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if len(raw) > hookDedupeMaxBytes {
					delete(state.Digests, key)
					break
				}
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			f, _, err := openHookDedupe("session-a")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.Write(raw); err != nil {
				t.Fatal(err)
			}
			f.Close()
			if got := ruleDeliveryPost(t, id, root, file, "", "", ""); !strings.Contains(got, "Sort outputs.") {
				t.Fatalf("first rule at capacity: %q", got)
			}
			raw, err = os.ReadFile(cbDedupePath("session-a"))
			if err != nil {
				t.Fatal(err)
			}
			state = hookDedupeState{}
			if len(raw) > hookDedupeMaxBytes || json.Unmarshal(raw, &state) != nil || len(state.Digests) > hookDedupeMaxEntries {
				t.Fatal("state exceeded capacity")
			}
			if _, ok := state.Digests[oldest]; ok {
				t.Fatal("oldest rule was not evicted")
			}
			if got := ruleDeliveryPost(t, id, root, file, "", "", ""); got != "" {
				t.Fatalf("capacity disabled dedupe permanently: %q", got)
			}
		})
	}
}
