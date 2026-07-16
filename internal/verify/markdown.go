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
		ann, title := parseAnnotation(raw)
		n++
		t := Task{
			ID:         fmt.Sprintf("md-%03d", n),
			Title:      title,
			SpecIDs:    sdd.ExtractRefs(raw),
			Claimed:    check == "x" || check == "X",
			Skipped:    check == "-",
			InProgress: check == "~",
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
		}
		plan.Tasks = append(plan.Tasks, t)
	}
	return plan
}

// parseAnnotation splits a "{key: value, …}" block out of s, returning the
// parsed key→value map (nil when there is no block) and s with the block
// removed and trimmed.
//
// Values may be double-quoted; a quoted value may contain commas and its
// surrounding quotes are stripped ({reason: "kit-side done, donor NOT
// deleted"}). Unquoted values must not contain commas.
func parseAnnotation(s string) (map[string]string, string) {
	// The annotation is the TRAILING brace block: earlier braces in the
	// title (code literals like Beat{...}, prose examples of annotations)
	// stay title text. The first non-vacuous gate run over the fleet's
	// plans caught the old first-brace rule hijacking contracts (GI-7).
	j := strings.LastIndexByte(s, '}')
	i := -1
	if j >= 0 {
		i = strings.LastIndexByte(s[:j], '{')
	}
	if i < 0 || j < i {
		return nil, strings.TrimSpace(s)
	}
	block := s[i+1 : j]
	title := strings.TrimSpace(strings.TrimSpace(s[:i]) + " " + strings.TrimSpace(s[j+1:]))
	title = strings.TrimSpace(title)
	m := map[string]string{}
	for _, pair := range splitPairs(block) {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := unquote(strings.TrimSpace(kv[1]))
		if k != "" {
			m[k] = v
		}
	}
	return m, title
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
