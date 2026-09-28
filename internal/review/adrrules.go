package review

// adrrules.go — U11 step 1 (docs/design/change-and-compliance.md §5, F24
// promoted; plan CX-19 / CX-23): an ADR stating a dependency direction
// ("X never imports Y", "handlers call services, never stores") is
// PROPOSED as an archconformance rule with its citation. Nothing here
// writes a rule: a human accepts each proposal by copying it into
// .fathom/rules/archconformance.cue (ADR-016's compile discipline).
//
// ProposeADRRules is deterministic (lexical, no provider). proposeRuleFromADR
// is the decision-port path (design §8): it asks a configured
// decision.Decider to pick, among candidate rules built from the package
// names the prose mentions, the one the prose states — only when [decide]
// is configured (a nil or "none" Decider proposes nothing).

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
)

// RuleProposal is one proposed archconformance rule and the ADR prose it
// was compiled from. Status is always "proposed": acceptance is a human's.
type RuleProposal struct {
	FromPkg string `json:"from_pkg"`
	ToPkg   string `json:"to_pkg"`
	Allow   bool   `json:"allow"`
	// Decision is the ADR id whose section states the rule ("ADR-12").
	Decision string `json:"decision"`
	// Citation is file:line of the sentence.
	Citation string `json:"citation"`
	// Quote is the sentence, verbatim (trimmed).
	Quote string `json:"quote"`
	// Source is "lexical" (ProposeADRRules) or "decide:<provider>".
	Source string `json:"source"`
	Status string `json:"status"`
}

const proposalStatus = "proposed"

var (
	adrHeadingRe = regexp.MustCompile(`^#{2,3} (ADR-\d+)\b`)
	pkgTok       = "`?([A-Za-z_][A-Za-z0-9_./-]*)`?"
	// "X never imports Y", "X must not import Y", "X does not depend on Y".
	neverImportRe = regexp.MustCompile(pkgTok + `(?:\s+packages?)?\s+(?:must never|must not|never|does not|do not|cannot|may not|should not)\s+(?:import|imports|depend on|depends on|call|calls)\s+` + pkgTok)
	// "handlers call services, never stores".
	// "analysis packages import neither mcp nor store".
	neitherNorRe = regexp.MustCompile(pkgTok + `(?:\s+packages?)?\s+(?:import|imports|depend on|depends on|call|calls)\s+neither\s+` + pkgTok + `\s+nor\s+` + pkgTok)
	callNeverRe  = regexp.MustCompile(pkgTok + `(?:\s+packages?)?\s+(?:call|calls|import|imports)\s+` + pkgTok + `,?\s+never\s+` + pkgTok)
)

// rulePkg reduces a package mention to the bare package name archconformance
// rules compare against ("internal/query" -> "query").
func rulePkg(s string) string {
	s = path.Base(strings.Trim(s, "`.,;:"))
	// A sentence-initial capital ("Handlers call ...") is prose, not a
	// receiver type: "Handlers" -> "handlers". Mixed case stays as written.
	if len(s) > 1 && s[0] >= 'A' && s[0] <= 'Z' && strings.ToLower(s[1:]) == s[1:] {
		s = strings.ToLower(s)
	}
	return s
}

// ProposeADRRules scans an ADR file's prose and proposes one deny rule per
// dependency statement, each cited (CX-19). Deterministic: the result is
// sorted by citation line then rule.
func ProposeADRRules(file string, src []byte) []RuleProposal {
	var out []RuleProposal
	seen := map[string]bool{}
	adr := ""
	add := func(from, to string, line int, quote string) {
		from, to = rulePkg(from), rulePkg(to)
		if from == "" || to == "" || from == to || adr == "" {
			return
		}
		key := adr + "\x00" + from + "\x00" + to
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, RuleProposal{
			FromPkg: from, ToPkg: to, Allow: false,
			Decision: adr, Citation: fmt.Sprintf("%s:%d", file, line),
			Quote: strings.TrimSpace(quote), Source: "lexical", Status: proposalStatus,
		})
	}
	for i, line := range strings.Split(string(src), "\n") {
		if m := adrHeadingRe.FindStringSubmatch(line); m != nil {
			adr = m[1]
			continue
		}
		if strings.HasPrefix(line, "# ") {
			adr = ""
			continue
		}
		for _, m := range callNeverRe.FindAllStringSubmatch(line, -1) {
			add(m[1], m[3], i+1, line)
		}
		for _, m := range neitherNorRe.FindAllStringSubmatch(line, -1) {
			add(m[1], m[2], i+1, line)
			add(m[1], m[3], i+1, line)
		}
		for _, m := range neverImportRe.FindAllStringSubmatch(line, -1) {
			add(m[1], m[2], i+1, line)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Citation != out[j].Citation {
			return citationLess(out[i].Citation, out[j].Citation)
		}
		if out[i].FromPkg != out[j].FromPkg {
			return out[i].FromPkg < out[j].FromPkg
		}
		return out[i].ToPkg < out[j].ToPkg
	})
	return out
}

func citationLess(a, b string) bool {
	fa, la := splitCitation(a)
	fb, lb := splitCitation(b)
	if fa != fb {
		return fa < fb
	}
	return la < lb
}

func splitCitation(c string) (string, int) {
	i := strings.LastIndex(c, ":")
	if i < 0 {
		return c, 0
	}
	n := 0
	fmt.Sscanf(c[i+1:], "%d", &n)
	return c[:i], n
}

// RenderRuleProposal renders one proposal as the CUE rule a human would
// paste into .fathom/rules/archconformance.cue, citation first.
func RenderRuleProposal(p RuleProposal) string {
	return fmt.Sprintf("// %s (%s, %s): %q\n// status: %s — accept by adding this rule to .fathom/rules/archconformance.cue\n{from_pkg: %q, to_pkg: %q, allow: %t}\n",
		p.Decision, p.Citation, p.Source, p.Quote, p.Status, p.FromPkg, p.ToPkg, p.Allow)
}

// ErrNoDecideProvider: the decision-port proposal path runs only with a
// [decide] provider configured.
var ErrNoDecideProvider = errors.New("review: proposing a rule from ADR prose needs [decide] configured")

// ADRSection is one ADR's prose, for the decision-port path.
type ADRSection struct {
	Decision string
	File     string
	Line     int
	Prose    string
	// Packages are the candidate package names (the caller's known
	// package set intersected with the prose's mentions, or given).
	Packages []string
}

const noRuleOption = "none"

// proposeRuleFromADR asks the configured decider which dependency rule, if
// any, an ADR's prose states (CX-23). It never runs without a provider:
// a nil Decider or the zero-configuration "none" Decider returns
// ErrNoDecideProvider and no proposal. The proposal is only ever
// "proposed" — kept as a path only while SI-style acceptance reads at
// least 8 of 10 (design §8, measured, not asserted here).
func proposeRuleFromADR(ctx context.Context, d decision.Decider, sec ADRSection) (*RuleProposal, error) {
	if d == nil || d.Name() == "none" {
		return nil, ErrNoDecideProvider
	}
	pkgs := append([]string(nil), sec.Packages...)
	sort.Strings(pkgs)
	var opts []decision.Option
	for _, a := range pkgs {
		for _, b := range pkgs {
			if a != b {
				opts = append(opts, decision.Option{Name: a + " -> " + b, Criteria: "the prose says " + a + " must never depend on " + b})
			}
		}
	}
	if len(opts) == 0 {
		return nil, nil
	}
	if len(opts) > 254 {
		opts = opts[:254]
	}
	opts = append(opts, decision.Option{Name: noRuleOption, Criteria: "the prose states no dependency direction"})
	const qid = "adr_rule"
	resp, err := d.Decide(ctx, decision.Request{
		State: sec.Prose,
		Questions: []decision.Question{{
			ID:           qid,
			Kind:         decision.KindChoice,
			Instructions: "Which dependency rule does this architecture decision record state? Choose the forbidden direction, or none.",
			Options:      opts,
		}},
	})
	if err != nil {
		return nil, err
	}
	ans, ok := resp.Answers[qid]
	if !ok || ans.Choice == "" || ans.Choice == noRuleOption {
		return nil, nil
	}
	from, to, ok := strings.Cut(ans.Choice, " -> ")
	if !ok {
		return nil, nil
	}
	// The quote is clipped by rune, never mid-character.
	quote := strings.TrimSpace(sec.Prose)
	if r := []rune(quote); len(r) > 200 {
		quote = string(r[:200])
	}
	return &RuleProposal{
		FromPkg: from, ToPkg: to, Allow: false,
		Decision: sec.Decision, Citation: fmt.Sprintf("%s:%d", sec.File, sec.Line),
		Quote: quote, Source: "decide:" + resp.Provider, Status: proposalStatus,
	}, nil
}
