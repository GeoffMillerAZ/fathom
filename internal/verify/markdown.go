package verify

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/sdd"
)

// taskLineRe matches a Markdown checklist item: "- [ ] title", "* [x] title",
// "- [~] title", "- [-] title". Group 1 is the checkbox char; group 2 is the
// title text.
var taskLineRe = regexp.MustCompile(`^\s*[-*]\s+\[([ xX~\-])\]\s+(.*\S)\s*$`)

// ParseMarkdownPlan turns a Markdown checklist (the kind people actually write —
// e.g. tmp/TRACKING.md) into a verify.Plan, so verify_plan / gen-status-cue can
// run against a real plan file rather than only structured tasks.
//
// Each checklist line becomes a Task. The title may carry:
//   - bracketed spec IDs ([US-042], [EP-013], …), extracted via sdd; and
//   - an optional annotation block "{symbol: Foo, package: dir/of/foo,
//     reachable_from: Bar, min_tier: verified, skip_test: true,
//     allow_contradiction: false}" that supplies the acceptance contract. A
//     line with no annotation and no spec IDs verifies as "no-contract"
//     (honestly Unknown) rather than a false pass.
//
// The annotation must sit on the checkbox line itself: this parser is
// line-based, so an annotation on a continuation line is silently ignored.
//
// Task IDs are assigned deterministically as "md-001", "md-002", … in document
// order (zero-padded so they sort correctly).
func ParseMarkdownPlan(service string, md []byte) Plan {
	plan := Plan{Service: service}
	n := 0
	for _, line := range strings.Split(string(md), "\n") {
		m := taskLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		check := m[1] // ' ', 'x', 'X', '~', or '-'
		raw := m[2]
		ann, title, suspect := parseAnnotation(raw)
		n++
		t := Task{
			ID:         fmt.Sprintf("md-%03d", n),
			Title:      title,
			SpecIDs:    sdd.ExtractRefs(raw),
			Claimed:    check == "x" || check == "X",
			Skipped:    check == "-",
			InProgress: check == "~",

			AnnotationSuspect: suspect,
		}
		if ann != nil {
			if id := ann["id"]; id != "" {
				t.ID = id // author-owned stable ID overrides positional md-NNN
			}
			t.Disposition = ann["disposition"]
			t.Reason = ann["reason"]
			t.Trigger = ann["trigger"]
			t.Expect.Symbol = ann["symbol"]
			t.Expect.Package = ann["package"]
			t.Expect.ReachableFrom = ann["reachable_from"]
			if mt := core.EvidenceTier(ann["min_tier"]); mt.Valid() {
				t.Expect.MinTier = mt
			}
			t.Expect.SkipTest = truthy(ann["skip_test"])
			t.Expect.SkipReason = ann["skip"]
			t.Expect.AllowContradiction = truthy(ann["allow_contradiction"])
			if v, ok := ann["ctx"]; ok {
				if v == "" {
					t.CtxMalformed = true
				} else {
					t.Ctx = strings.Fields(v)
				}
			}
			if v, ok := ann["needs"]; ok {
				for _, id := range strings.Split(v, ",") {
					if id = strings.TrimSpace(id); id != "" {
						t.Needs = append(t.Needs, id)
					}
				}
			}
		}
		plan.Tasks = append(plan.Tasks, t)
	}
	return plan
}

// parseAnnotation splits a "{key: value, …}" block out of s, returning the
// parsed key→value map (nil when there is no block), s with the block
// removed and trimmed, and a non-empty suspect string when a brace block
// LOOKED like an annotation but yielded no keys — the O4 silent-void class,
// surfaced so LintPlan can be loud instead of the row quietly degrading to
// a positional md-NNN id with no contract.
//
// Values may be double-quoted; a quoted value may contain commas AND braces
// — the notation's own vocabulary ("never write {symbol, package} in a
// value") is exactly what authors quote, and it must never terminate the
// block (O4). Unquoted values must not contain commas or braces.
func parseAnnotation(s string) (map[string]string, string, string) {
	// The annotation is the TRAILING brace block: earlier braces in the
	// title (code literals like Beat{...}, prose examples of annotations)
	// stay title text (GI-7). Brace pairing is QUOTE-AWARE: braces inside
	// double-quoted spans are value content, never delimiters.
	type pair struct{ open, close int }
	var (
		pairs   []pair
		stack   []int
		inQuote bool
	)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '{':
			if !inQuote {
				stack = append(stack, i)
			}
		case '}':
			if !inQuote && len(stack) > 0 {
				pairs = append(pairs, pair{open: stack[len(stack)-1], close: i})
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(pairs) == 0 {
		return nil, strings.TrimSpace(s), ""
	}
	// The block whose close brace is last is the trailing annotation.
	best := pairs[0]
	for _, p := range pairs[1:] {
		if p.close > best.close {
			best = p
		}
	}
	i, j := best.open, best.close
	block := s[i+1 : j]
	title := strings.TrimSpace(strings.TrimSpace(s[:i]) + " " + strings.TrimSpace(s[j+1:]))
	m := map[string]string{}
	for _, kvPair := range splitPairs(block) {
		kv := strings.SplitN(kvPair, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := unquote(strings.TrimSpace(kv[1]))
		if k != "" {
			m[k] = v
		}
	}
	if len(m) == 0 {
		if suspectAnnotation(block) {
			return nil, title, block
		}
		return nil, title, ""
	}
	return m, title, ""
}

// suspectAnnotation reports whether a zero-key brace block was PROBABLY a
// mangled annotation rather than innocent title text (Beat{stuck}): it
// contains a known contract key as a whole word. Loudness over silence for
// exactly these — a lost contract is the gate's biggest honesty risk (O4).
func suspectAnnotation(block string) bool {
	for _, f := range strings.FieldsFunc(block, func(r rune) bool {
		return r == ' ' || r == ',' || r == ':' || r == '\t'
	}) {
		for _, k := range annotationKeys {
			if f == k {
				return true
			}
		}
	}
	return false
}

// annotationKeys is every key the tokenizer recognises as evidence that a
// brace block was MEANT to be an annotation. Plan-grammar keys came first;
// the oracle keys (ADR-026 §3) joined them when intent conditions started
// carrying `{oracle: …}` blocks parsed by this same tokenizer — without
// them a mangled oracle block would yield zero keys, look like prose, and
// vanish silently: the exact O4 class this list exists to catch.
var annotationKeys = []string{
	// plan grammar v2
	"id", "symbol", "package", "skip_test", "skip", "reachable_from",
	"min_tier", "disposition", "reason", "trigger", "allow_contradiction",
	"ctx", "needs",
	// intent-condition oracles (ADR-026)
	"oracle", "paths", "pattern", "min", "max", "eq", "run", "path",
	"service", "present", "after", "plan", "rows", "row", "partial",
}

// ParseAnnotation is parseAnnotation's exported face for callers outside
// this package — internal/intent parses `{oracle: …}` blocks on intent
// condition lines with the SAME tokenizer the plan grammar uses, so the two
// grammars can never drift into two quoting rules (O4's lesson).
//
// It returns the key→value map (nil when the line carries no block), the
// line with the block removed, and a non-empty suspect string when a block
// looked like an annotation but yielded no keys.
func ParseAnnotation(s string) (map[string]string, string, string) {
	return parseAnnotation(s)
}

// splitPairs splits an annotation block on commas, treating commas inside
// double-quoted spans as literal value content.
func splitPairs(block string) []string {
	var (
		out     []string
		start   int
		inQuote bool
	)
	for i := 0; i < len(block); i++ {
		switch block[i] {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				out = append(out, block[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, block[start:])
	return out
}

// unquote strips one pair of surrounding double quotes, if present.
func unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1":
		return true
	}
	return false
}
