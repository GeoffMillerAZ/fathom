package main

import (
	"fmt"
	"maps"
	"sort"
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
	d.ruleDigests = make(map[string]string)
	// CB-1's whole-payload slot is context-specific too. Keep the original
	// digest: filtering a changed payload must not make its next touch new.
	d.key = hookDigest("payload\x00" + context + "\x00" + channel + "\x00" + target)
	previousPayload, payloadSeen := d.state.Digests[d.key]
	var kept []semantic.Obligation
	withheld := 0
	duplicates := false
	pending := make(map[string]bool)
	for _, rule := range rules {
		identity := "claim\x00" + rule.ClaimID
		if rule.ClaimID == "" {
			identity = "raw\x00" + rule.Rule + "\x00" + rule.Src
		}
		key := hookDigest("rule\x00" + context + "\x00" + service + "\x00" + rule.Module + "\x00" + identity)
		if pending[key] {
			duplicates = true
			continue
		}
		pending[key] = true
		var bullet strings.Builder
		writeRuleBullet(&bullet, rule)
		digest := hookDigest(bullet.String())
		previous, seen := d.state.Digests[key]
		if seen && previous.Digest == digest && !now.Before(previous.EmittedAt) && now.Sub(previous.EmittedAt) < hookDedupeRefresh && previous.Suppressions < hookDedupeMaxSuppressions {
			previous.Suppressions++
			d.state.Digests[key] = previous
			withheld++
			continue
		}
		d.ruleKeys = append(d.ruleKeys, key)
		d.ruleDigests[key] = digest
		kept = append(kept, rule)
	}
	d.state.At = now
	// Check both bounds with the prospective deliveries, but never persist
	// those identities before stdout succeeds. Refreshing a rule uses its slot.
	prior := d.state.Digests
	d.state.Digests = maps.Clone(prior)
	for _, key := range d.ruleKeys {
		d.state.Digests[key] = hookDedupeEntry{Digest: d.ruleDigests[key], EmittedAt: now, Rule: true}
	}
	d.state.Digests[d.key] = hookDedupeEntry{Digest: d.digest, EmittedAt: now}
	// Make room in the same bounded file by retiring the oldest rule
	// deliveries first. Preserve pending deliveries and all other channels.
	fits := func() bool { return len(d.state.Digests) <= hookDedupeMaxEntries && d.fits() }
	room := fits()
	if !room {
		var evictable []string
		for key, entry := range d.state.Digests {
			// Before rendered digests, per-rule entries used Digest == key.
			if key != d.key && d.ruleDigests[key] == "" && (entry.Rule || entry.Digest == key) {
				evictable = append(evictable, key)
			}
		}
		sort.Slice(evictable, func(i, j int) bool {
			a, b := d.state.Digests[evictable[i]], d.state.Digests[evictable[j]]
			if a.EmittedAt.Equal(b.EmittedAt) {
				return evictable[i] < evictable[j]
			}
			return a.EmittedAt.Before(b.EmittedAt)
		})
		for _, key := range evictable {
			if fits() {
				break
			}
			delete(d.state.Digests, key)
		}
		room = fits()
	}
	if room {
		for key := range prior {
			if _, retained := d.state.Digests[key]; !retained {
				delete(prior, key)
			}
		}
	}
	d.state.Digests = prior
	if !room {
		d.stateError = true
		d.close()
		return payload, d
	}
	d.firstTouch = !payloadSeen
	if withheld == 0 && !duplicates {
		return payload, d
	}
	// An unchanged whole payload stays silent unless a per-rule backstop
	// makes at least one rule due again. Suppressions count in either case.
	if payloadSeen && previousPayload.Digest == d.digest && len(d.ruleKeys) == 0 {
		ok := d.persist()
		d.close()
		d.suppressed = ok && !d.stateError
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
	filtered := strings.Replace(payload, before, render(kept), 1)
	if withheld > 0 {
		noun := "rules"
		if withheld == 1 {
			noun = "rule"
		}
		filtered += fmt.Sprintf("\n(%d %s already shown in this session)", withheld, noun)
	}
	if id == harness.Codex && len(filtered) > harness.CodexAdditionalContextCap {
		d.close() // capped output cannot establish delivery of every rule
		return payload, d
	}
	// A write fault must not withhold. Persist only old-rule counters here;
	// newly delivered rules and the original payload commit after stdout.
	if !d.persist() {
		d.close()
		return payload, d
	}
	return filtered, d
}
