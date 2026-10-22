package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// hookRuleContext is the context identity carried by post-tool payloads.
// Codex supplies no agent_id on those events: transcript_path distinguishes
// contexts when present, otherwise only session_id is available. Other
// harnesses retain full injections until their context reset is proven.
func hookRuleContext(id string, in harness.HookInput) (string, bool) {
	switch id {
	case harness.ClaudeCode:
		return id + "\x00" + in.AgentID, true
	case harness.Codex:
		return id + "\x00" + in.TranscriptPath, true
	default:
		return "", false
	}
}

// dedupeHookRules filters the structured rules' exact rendered block. The
// rest of the payload (findings, coverage, claim checks and test advice) is
// preserved. New identities are committed only after successful delivery.
func dedupeHookRules(id, root string, in harness.HookInput, channel, target, service, payload string, rules []semantic.Obligation, now time.Time) (string, *hookDedupeCheck) {
	context, supported := hookRuleContext(id, in)
	if !supported || in.SessionID == "" || len(rules) == 0 || !hookDedupeEnabled(root) || (id == harness.Codex && len(payload) > harness.CodexAdditionalContextCap) {
		return payload, newHookDedupeCheck(channel, target, payload)
	}
	d := loadHookDedupe(in.SessionID, channel, target, payload, now, false)
	if d.file == nil {
		return payload, d
	}
	d.ruleMode = true
	var kept []semantic.Obligation
	withheld := 0
	pending := make(map[string]bool)
	for _, rule := range rules {
		identity := "claim\x00" + rule.ClaimID
		if rule.ClaimID == "" {
			identity = "raw\x00" + rule.Rule + "\x00" + rule.Src
		}
		key := hookDigest("rule\x00" + context + "\x00" + service + "\x00" + rule.Module + "\x00" + identity)
		if _, seen := d.state.Digests[key]; seen || pending[key] {
			withheld++
			continue
		}
		pending[key] = true
		d.ruleKeys = append(d.ruleKeys, key)
		kept = append(kept, rule)
	}
	// Capacity failure never authorizes withholding even an already-seen rule.
	if len(d.state.Digests)+len(d.ruleKeys) > hookDedupeMaxEntries {
		d.stateError = true
		d.close()
		return payload, d
	}
	// A write fault must not suppress. Probe the existing state, without
	// recording any newly delivered rule, before withholding old rules.
	if withheld > 0 && !d.persist() {
		d.close()
		return payload, d
	}
	// Check the byte bound before withholding, including new identities.
	for _, key := range d.ruleKeys {
		d.state.Digests[key] = hookDedupeEntry{Digest: key, EmittedAt: now}
	}
	if !d.fits() {
		d.stateError = true
		d.close()
		return payload, d
	}
	d.firstTouch = withheld == 0
	if withheld == 0 {
		return payload, d
	}
	render := func(rs []semantic.Obligation) string {
		var sb strings.Builder
		if channel == "post-bash" {
			for _, rule := range rs {
				writeRuleBullet(&sb, rule)
			}
		} else {
			writeRuleBullets(&sb, rs, service)
		}
		return sb.String()
	}
	before := render(rules)
	if before == "" || !strings.Contains(payload, before) {
		d.stateError = true
		d.close()
		return payload, d
	}
	filtered := strings.Replace(payload, before, render(kept), 1) + fmt.Sprintf("\n(%d rules already shown in this session)", withheld)
	if id == harness.Codex && len(filtered) > harness.CodexAdditionalContextCap {
		d.close() // capped output cannot establish delivery of every rule
		return payload, d
	}
	return filtered, d
}
