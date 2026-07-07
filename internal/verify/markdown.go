package verify

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/sdd"
)

// taskLineRe matches a Markdown checklist item: "- [ ] title", "* [x] title",
// "- [-] title". The captured group is the title text after the checkbox.
var taskLineRe = regexp.MustCompile(`^\s*[-*]\s+\[[ xX\-]\]\s+(.*\S)\s*$`)

// ParseMarkdownPlan turns a Markdown checklist (the kind people actually write —
// e.g. tmp/TRACKING.md) into a verify.Plan, so verify_plan / gen-status-cue can
// run against a real plan file rather than only structured tasks.
//
// Each checklist line becomes a Task. The title may carry:
//   - bracketed spec IDs ([US-042], [EP-013], …), extracted via sdd; and
//   - an optional annotation block "{symbol: Foo, reachable_from: Bar,
//     min_tier: verified, skip_test: true, allow_contradiction: false}" that
//     supplies the acceptance contract. A line with no annotation and no spec
//     IDs verifies as "no-contract" (honestly Unknown) rather than a false pass.
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
		raw := m[1]
		ann, title := parseAnnotation(raw)
		n++
		t := Task{
			ID:      fmt.Sprintf("md-%03d", n),
			Title:   title,
			SpecIDs: sdd.ExtractRefs(raw),
		}
		if ann != nil {
			t.Expect.Symbol = ann["symbol"]
			t.Expect.ReachableFrom = ann["reachable_from"]
			if mt := core.EvidenceTier(ann["min_tier"]); mt.Valid() {
				t.Expect.MinTier = mt
			}
			t.Expect.SkipTest = truthy(ann["skip_test"])
			t.Expect.AllowContradiction = truthy(ann["allow_contradiction"])
		}
		plan.Tasks = append(plan.Tasks, t)
	}
	return plan
}

// parseAnnotation splits a "{key: value, …}" block out of s, returning the
// parsed key→value map (nil when there is no block) and s with the block
// removed and trimmed.
func parseAnnotation(s string) (map[string]string, string) {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j < 0 || j < i {
		return nil, strings.TrimSpace(s)
	}
	block := s[i+1 : j]
	title := strings.TrimSpace(strings.TrimSpace(s[:i]) + " " + strings.TrimSpace(s[j+1:]))
	title = strings.TrimSpace(title)
	m := map[string]string{}
	for _, pair := range strings.Split(block, ",") {
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		if k != "" {
			m[k] = v
		}
	}
	return m, title
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1":
		return true
	}
	return false
}
