package main

// stalerules.go — J2 (app-kit O6/O16/O24/O26/O29/O30, the dogfood
// journal's P0 "withhold stale obligations from authoritative injection").
//
// Before this file existed, every hook renderer (edithook.go's post-edit
// payload, edithook_pre.go's guard preview, prompthook.go's prompt
// payload) rendered a stale/contradicted rule through the SAME
// writeRuleBullet path as a fresh, binding one — interleaved in the same
// governing list, distinguished only by a trailing ", stale" label an
// agent under budget pressure has every incentive to skip past. A rule
// whose own citation no longer hashes against the current file is not
// binding; it must read as a diagnostic, never as governance.
//
// writeRuleBullets is the one place that decision is made: it partitions a
// rendered rule set into governing (Status == "") and stale
// (Status != "") — semantic.Obligation's own doc comment: Status is
// present ONLY when not fresh — writes the governing bullets exactly as
// writeRuleBullet always has, then appends a trailing block naming the
// stale rules and one remedy line. A rule set with nothing stale renders
// byte-identically to before this file existed.

import (
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// staleRuleTextMaxBytes caps how much of a stale rule's own text renders in
// the trailing block — the block names WHICH rule is non-binding and
// where, not its full restatement (that is what `fathom compile` and a
// direct read of the current source are for).
const staleRuleTextMaxBytes = 100

// partitionRules splits rules into governing (fresh — Status == "") and
// stale (Status != "", i.e. "stale" or "contradicted" — the only two
// values Obligation.Status ever carries, since a refuted/superseded claim
// is never served at all), preserving each subset's relative order.
func partitionRules(rules []semantic.Obligation) (governing, stale []semantic.Obligation) {
	governing = make([]semantic.Obligation, 0, len(rules))
	for _, r := range rules {
		if r.Status == "" {
			governing = append(governing, r)
			continue
		}
		stale = append(stale, r)
	}
	return governing, stale
}

// writeRuleBullets renders rules' governing bullets (unchanged
// writeRuleBullet, one per fresh rule) and then, only when at least one
// rule is stale/contradicted, a trailing block for those — never
// interleaved with the governing list. service names the compile remedy's
// --service value. Byte-identical to a bare writeRuleBullet loop when
// nothing is stale.
func writeRuleBullets(sb *strings.Builder, rules []semantic.Obligation, service string) {
	governing, stale := partitionRules(rules)
	for _, r := range governing {
		writeRuleBullet(sb, r)
	}
	if len(stale) > 0 {
		writeStaleRulesBlock(sb, stale, service)
	}
}

// writeStaleRulesBlock appends the one trailing block every hook renderer
// uses for stale/contradicted rules: a header naming the count, one line
// per rule (its citation and rule text, truncated), then exactly one
// remedy line. layer is the first stale rule's own compiled layer
// (staleRemedy defaults to the obligations layer when none is set).
func writeStaleRulesBlock(sb *strings.Builder, stale []semantic.Obligation, service string) {
	fmt.Fprintf(sb, "\nstale rules — not binding until recompiled (%d):", len(stale))
	layer := ""
	for _, r := range stale {
		fmt.Fprintf(sb, "\n%s · %s", r.Src, truncateBytes(r.Rule, staleRuleTextMaxBytes))
		if layer == "" && r.Layer != "" {
			layer = r.Layer
		}
	}
	fmt.Fprintf(sb, "\nremedy: %s", staleRemedy(layer, service))
}

// staleRemedy is cmd/fathom's own copy of internal/mcp's identically-named
// helper (semantic_tools.go) — the two packages never share an internal
// import for one format string, but the string itself must read
// identically wherever a stale rule surfaces it (J2 (d)).
func staleRemedy(layer, service string) string {
	if layer == "" {
		layer = semantic.LayerObligations
	}
	return fmt.Sprintf("fathom compile --layer %s --service %s", layer, service)
}

// truncateBytes returns s cut to at most max bytes, backing off to a UTF-8
// rune boundary so the result is never invalid UTF-8 (mirrors
// internal/mcp/obligationsnippet.go's utf8Start discipline — this package
// cannot import that internal one, and the byte-cap behavior is simple
// enough not to need a shared helper).
func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut]
}
