package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestDecideSpawn(t *testing.T) {
	builtin := builtinPolicy()
	mapped := policyWith(t, codexPolicy)

	tests := []struct {
		name      string
		policy    *Policy
		req       spawnRequest
		wantDeny  bool
		wantCodes []string
		wantTier  string
		wantClass string
	}{
		{
			name: "claude: unset model is denied", policy: builtin,
			req:      spawnRequest{Host: hostClaude, Brief: "find the config loader"},
			wantDeny: true, wantCodes: []string{codeNoModel}, wantClass: classWorker,
		},
		{
			name: "claude: inherit counts as unset", policy: builtin,
			req:      spawnRequest{Host: hostClaude, Model: "inherit", Brief: "find it"},
			wantDeny: true, wantCodes: []string{codeNoModel},
		},
		{
			name: "claude: fast tier alias is allowed", policy: builtin,
			req:       spawnRequest{Host: hostClaude, Model: "haiku", Brief: "find the config loader"},
			wantCodes: []string{codeAllowed}, wantTier: "fast",
		},
		{
			name: "claude: full model id resolves by family token", policy: builtin,
			req:       spawnRequest{Host: hostClaude, Model: "Claude-Sonnet-5-5", Brief: "implement the flag"},
			wantCodes: []string{codeAllowed}, wantTier: "standard",
		},
		{
			name: "claude: unknown model is denied", policy: builtin,
			req:      spawnRequest{Host: hostClaude, Model: "mystery-model", Brief: "x"},
			wantDeny: true, wantCodes: []string{codeUnknownModel},
		},
		{
			name: "claude: a family word inside another token does not match", policy: builtin,
			req:      spawnRequest{Host: hostClaude, Model: "haikus-1", Brief: "x"},
			wantDeny: true, wantCodes: []string{codeUnknownModel},
		},
		{
			name: "claude: worker above its ceiling is denied", policy: builtin,
			req:      spawnRequest{Host: hostClaude, Model: "opus", Brief: "implement the parser"},
			wantDeny: true, wantCodes: []string{codeTierCeiling}, wantTier: "deep", wantClass: classWorker,
		},
		{
			name: "claude: worker escalates with the tier marker", policy: builtin,
			req:       spawnRequest{Host: hostClaude, Model: "opus", Brief: "[deep] untangle the race in the scheduler"},
			wantCodes: []string{codeAllowed}, wantTier: "deep",
		},
		{
			name: "claude: critic by pattern may use deep", policy: builtin,
			req:       spawnRequest{Host: hostClaude, Model: "opus", Brief: "Adversarially verify the migration plan"},
			wantCodes: []string{codeAllowed}, wantTier: "deep", wantClass: classCritic,
		},
		{
			name: "claude: critic marker forces the critic class", policy: builtin,
			req:       spawnRequest{Host: hostClaude, Model: "opus", Brief: "[critic] weigh the two designs"},
			wantCodes: []string{codeAllowed}, wantClass: classCritic,
		},
		{
			name: "claude: worker marker beats critic words", policy: builtin,
			req:      spawnRequest{Host: hostClaude, Model: "opus", Brief: "[worker] review and fix the lint errors"},
			wantDeny: true, wantCodes: []string{codeTierCeiling}, wantClass: classWorker,
		},
		{
			name: "claude: plain review or verify wording stays a worker", policy: builtin,
			req:      spawnRequest{Host: hostClaude, Model: "opus", Brief: "implement the flag, then verify with go test and review the diff"},
			wantDeny: true, wantCodes: []string{codeTierCeiling}, wantClass: classWorker,
		},
		{
			name: "claude: an exact id in a stronger tier beats a family word",
			policy: policyWith(t, func(m map[string]any) {
				m["tiers"].([]any)[2].(map[string]any)["claude"] = []any{"opus", "claude-haiku-9-thinking"}
			}),
			req:       spawnRequest{Host: hostClaude, Model: "claude-haiku-9-thinking", Brief: "[critic] judge it"},
			wantCodes: []string{codeAllowed}, wantTier: "deep",
		},
		{
			name: "claude: critic above its ceiling needs the frontier marker", policy: builtin,
			req:      spawnRequest{Host: hostClaude, Model: "fable", Brief: "[critic] review the release"},
			wantDeny: true, wantCodes: []string{codeTierCeiling}, wantTier: "frontier",
		},
		{
			name: "claude: frontier marker admits the frontier tier", policy: builtin,
			req:       spawnRequest{Host: hostClaude, Model: "fable", Brief: "[frontier] final review of the security model"},
			wantCodes: []string{codeAllowed}, wantTier: "frontier",
		},
		{
			name: "claude: effort is not required", policy: builtin,
			req:       spawnRequest{Host: hostClaude, Model: "sonnet", Brief: "write the tests"},
			wantCodes: []string{codeAllowed},
		},
		{
			name: "codex: missing model and effort are reported together", policy: mapped,
			req:      spawnRequest{Host: hostCodex, Brief: "find it"},
			wantDeny: true, wantCodes: []string{codeNoModel, codeNoEffort},
		},
		{
			name: "codex: model and effort on the fast tier are allowed", policy: mapped,
			req:       spawnRequest{Host: hostCodex, Model: "cx-small", Effort: "low", Brief: "find it"},
			wantCodes: []string{codeAllowed}, wantTier: "fast",
		},
		{
			name: "codex: names are matched exactly, never by token", policy: mapped,
			req:      spawnRequest{Host: hostCodex, Model: "cx-small-2", Effort: "low", Brief: "find it"},
			wantDeny: true, wantCodes: []string{codeUnknownModel},
		},
		{
			name: "codex: ceiling applies", policy: mapped,
			req:      spawnRequest{Host: hostCodex, Model: "cx-large", Effort: "high", Brief: "implement it"},
			wantDeny: true, wantCodes: []string{codeTierCeiling},
		},
		{
			name: "codex: a context fork still takes a model", policy: mapped,
			req:       spawnRequest{Host: hostCodex, Model: "cx-mid", Effort: "medium", ContextFork: true, Brief: "continue"},
			wantCodes: []string{codeAllowed},
		},
		{
			name: "codex: unmapped host still requires model and effort", policy: builtin,
			req:      spawnRequest{Host: hostCodex, Model: "anything", Brief: "x"},
			wantDeny: true, wantCodes: []string{codeNoEffort},
		},
		{
			name: "codex: unmapped host skips tier checks", policy: builtin,
			req:       spawnRequest{Host: hostCodex, Model: "anything", Effort: "low", Brief: "x"},
			wantCodes: []string{codeAllowed},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := decideSpawn(tt.policy, tt.req)
			if v.Deny != tt.wantDeny {
				t.Fatalf("deny = %v, want %v (codes %v, reason %q)", v.Deny, tt.wantDeny, v.Codes, v.Reason)
			}
			if !slices.Equal(v.Codes, tt.wantCodes) {
				t.Errorf("codes = %v, want %v", v.Codes, tt.wantCodes)
			}
			if tt.wantTier != "" && v.Tier != tt.wantTier {
				t.Errorf("tier = %q, want %q", v.Tier, tt.wantTier)
			}
			if tt.wantClass != "" && v.Class != tt.wantClass {
				t.Errorf("class = %q, want %q", v.Class, tt.wantClass)
			}
			if v.Deny && !strings.Contains(v.Reason, "Pick the cheapest tier") {
				t.Errorf("deny reason lacks the routing table:\n%s", v.Reason)
			}
		})
	}
}

func TestDecideSpawnForkModes(t *testing.T) {
	fork := spawnRequest{Host: hostClaude, Fork: true, Brief: "carry on with the plan"}
	marked := spawnRequest{Host: hostClaude, Fork: true, Brief: "[fork] carry on with the plan"}
	tests := []struct {
		mode     string
		req      spawnRequest
		wantDeny bool
		wantCode string
	}{
		{forkMarker, fork, true, codeForkMarker},
		{forkMarker, marked, false, codeAllowed},
		{forkAllow, fork, false, codeAllowed},
		{forkDeny, marked, true, codeForkDenied},
	}
	for _, tt := range tests {
		t.Run(tt.mode+" "+tt.req.Brief, func(t *testing.T) {
			p := policyWith(t, func(m map[string]any) { m["spawn"].(map[string]any)["fork"] = tt.mode })
			v := decideSpawn(p, tt.req)
			if v.Deny != tt.wantDeny || v.Codes[0] != tt.wantCode {
				t.Fatalf("got deny=%v codes=%v, want deny=%v %s", v.Deny, v.Codes, tt.wantDeny, tt.wantCode)
			}
			if v.Tier != "inherit" {
				t.Errorf("fork tier = %q, want inherit", v.Tier)
			}
		})
	}
}

func TestAdviceMatching(t *testing.T) {
	p := builtinPolicy()
	critic := decideSpawn(p, spawnRequest{Host: hostClaude, Model: "opus", Brief: "[critic] /caveman verify the proof"})
	if len(critic.Notes) != 1 || critic.Notes[0].Key != "advice-critic-uncompressed" {
		t.Fatalf("critic with caveman: notes = %+v, want the critic-uncompressed advice", critic.Notes)
	}
	worker := decideSpawn(p, spawnRequest{Host: hostClaude, Model: "haiku", Brief: "/caveman find the loader"})
	if len(worker.Notes) != 0 {
		t.Fatalf("worker with caveman: notes = %+v, want none (the rule is critic-only)", worker.Notes)
	}

	withUnless := policyWith(t, func(m map[string]any) {
		m["advice"] = []any{map[string]any{"id": "ui", "when": "(?i)css", "unless": "(?i)no-ui", "message": "check it in a browser"}}
	})
	if n := decideSpawn(withUnless, spawnRequest{Host: hostClaude, Model: "haiku", Brief: "fix the css"}).Notes; len(n) != 1 {
		t.Errorf("when matched: notes = %+v, want 1", n)
	}
	if n := decideSpawn(withUnless, spawnRequest{Host: hostClaude, Model: "haiku", Brief: "fix the css [no-ui]"}).Notes; len(n) != 0 {
		t.Errorf("unless matched: notes = %+v, want none", n)
	}
}

func TestRoutingTableListsHostModels(t *testing.T) {
	p := policyWith(t, codexPolicy)
	claude := routingTable(p, hostClaude)
	if !strings.Contains(claude, "fast = haiku") || strings.Contains(claude, "effort") {
		t.Errorf("claude table:\n%s", claude)
	}
	codex := routingTable(p, hostCodex)
	if !strings.Contains(codex, "fast = cx-small (effort low)") || !strings.Contains(codex, "model and reasoning_effort") {
		t.Errorf("codex table:\n%s", codex)
	}
}

// policyWith returns the built-in policy after a mutation, validated.
func policyWith(t *testing.T, mutate func(map[string]any)) *Policy {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(defaultPolicyJSON, &m); err != nil {
		t.Fatal(err)
	}
	mutate(m)
	raw, _ := json.Marshal(m)
	p, err := parsePolicy(raw)
	if err != nil {
		t.Fatalf("mutated policy invalid: %v", err)
	}
	return p
}
