package verify

import (
	"fmt"
	"strings"
)

// LintPlan reports plan-grammar-v2 convention violations (ADR-018 §1). Lint
// findings are warnings: they never change gate exit codes in v0, but they
// surface the honesty conventions — stable IDs, reasoned dispositions,
// justified skips, a well-formed needs: DAG — that keep gated plans
// machine-legible. Deterministic: findings come out in task order (needs:
// findings appended last, also in task order), one pass, no I/O.
func LintPlan(p Plan) []string {
	var out []string
	seen := map[string]string{}
	for _, t := range p.Tasks {
		if prev, dup := seen[t.ID]; dup {
			out = append(out, fmt.Sprintf("%s: duplicate task id (first used by %q)", t.ID, prev))
		} else {
			seen[t.ID] = t.Title
		}
		if t.Disposition != "" {
			if t.Disposition != "refused" && t.Disposition != "deferred" {
				out = append(out, fmt.Sprintf("%s: unknown disposition %q (want refused|deferred)", t.ID, t.Disposition))
			}
			if !t.Skipped {
				out = append(out, fmt.Sprintf("%s: disposition %q requires a [-] checkbox", t.ID, t.Disposition))
			}
			if t.Reason == "" {
				out = append(out, fmt.Sprintf("%s: disposition %q requires a reason", t.ID, t.Disposition))
			}
		}
		if t.Trigger != "" && t.Disposition != "deferred" {
			out = append(out, fmt.Sprintf("%s: trigger only applies to disposition: deferred", t.ID))
		}
		if t.Expect.SkipTest && t.Expect.SkipReason == "" {
			out = append(out, fmt.Sprintf("%s: skip_test without a skip: justification", t.ID))
		}
		if t.Expect.Library {
			switch {
			case t.Expect.Symbol == "":
				out = append(out, fmt.Sprintf("%s: library: true applies only to a symbol: row (it waives the product-wiring check)", t.ID))
			case t.Reason == "":
				out = append(out, fmt.Sprintf("%s: library: true without a reason: justification — it waives nothing until the row says why the API itself is the deliverable", t.ID))
			}
		}
		if msg := uncontractedClaim(t); msg != "" {
			out = append(out, fmt.Sprintf("%s: %s", t.ID, msg))
		}
		if t.AnnotationSuspect != "" {
			out = append(out, fmt.Sprintf("%s: annotation block could not be parsed (%q) — the row's contract and author-owned id are VOID; check braces and quoting (O4)", t.ID, t.AnnotationSuspect))
		}
		if t.UnreadAnnotation != "" {
			out = append(out, fmt.Sprintf("%s: annotation %s sits on an unindented line under the row and is NOT read — the row has no contract; indent that continuation line under the checkbox, or join it to the checkbox line (G2-5)", t.ID, t.UnreadAnnotation))
		}
		if t.CtxMalformed {
			out = append(out, fmt.Sprintf("%s: empty ctx: value (want space-separated pointers)", t.ID))
		}
		for _, key := range t.BoundsInvalid {
			out = append(out, fmt.Sprintf("%s: %s: value is not an integer", t.ID, key))
		}
		if t.WeightInvalid != "" {
			out = append(out, fmt.Sprintf("%s: weight: %s is not a positive integer (want 1 or more; the row counts as weight 1)", t.ID, t.WeightInvalid))
		}
		if t.Oracle == "ledger" {
			out = append(out, lintLedgerOracle(t)...)
		} else {
			out = append(out, lintLexicalOracle(t)...)
		}
	}
	out = append(out, lintNeeds(p)...)
	return out
}

// uncontractedClaim reports a claimed ([x]) row that carries no contract
// (claimedWithoutContract): verifyTask reads it no-contract (Unknown), which
// the default gate lets through and only --strict blocks. Two shapes, one
// message each. A skip waiver — `skip_test: true` and/or `skip: "…"` — only
// exempts a symbol from test_reaches; twenty-eight rows sat claimed carrying
// nothing else (2026-09-25) because a detailed `skip:` naming the evidence
// reads like a proof to its author. A bare row carries no annotation beyond
// its id. Only claimed rows are named: the claim is the moment the mistake
// bites, and an open "(op)" row carrying its waiver early is not yet wrong. A
// row whose annotation block did not parse, or that carries a disposition on
// an [x] box, is left to its own lint line, which names the real fix. "" when
// the row is not that shape. Advisory like every lint: what fails a push on
// one of this repository's plans is
// TestRepoPlans_EveryClaimedRowCarriesAContract under `task validate`.
func uncontractedClaim(t Task) string {
	if !claimedWithoutContract(t) || t.AnnotationSuspect != "" || t.Disposition != "" {
		return ""
	}
	const remedy = "give it symbol:, oracle: lexical|ledger (path + pattern or run_id naming the evidence), or gone:"
	if t.Expect.SkipTest || t.Expect.SkipReason != "" {
		return "skip_test/skip is a waiver, not a contract — this claimed row verifies as no-contract (Unknown; --strict blocks it); " + remedy
	}
	return "claimed with no contract — this row verifies as no-contract (Unknown; --strict blocks it); " + remedy
}

// claimedWithoutContract reports whether verifyTask will read t as a claimed
// no-contract row: [x] with none of symbol:, oracle:, gone:, or a spec ID.
func claimedWithoutContract(t Task) bool {
	return t.Claimed && t.Expect.Symbol == "" && t.Oracle == "" && t.Gone == "" && len(t.SpecIDs) == 0
}

// lintNeeds validates the needs: eligibility DAG (WF-3): an unknown id, a
// self-reference, or a dependency cycle. Unknown-id and self-reference are a
// direct per-edge scan; cycles come from a standard white/gray/black DFS
// over row ids, walked in task order for determinism. A cycle is reported
// once, from the edge whose target closes the loop back onto a row still on
// the current path.
func lintNeeds(p Plan) []string {
	var out []string
	ids := map[string]bool{}
	byID := map[string]Task{}
	for _, t := range p.Tasks {
		ids[t.ID] = true
		byID[t.ID] = t
	}
	for _, t := range p.Tasks {
		for _, dep := range t.Needs {
			switch {
			case strings.Contains(dep, "#"):
				// A cross-plan target (PC-10): only its shape is checkable
				// without I/O; resolveCrossPlanNeed reads the sibling.
				if _, ok := ParseCrossPlanNeed(dep); !ok {
					out = append(out, fmt.Sprintf("%s: needs malformed cross-plan target %q (want <sibling>.md#<id>)", t.ID, dep))
				}
			case dep == t.ID:
				out = append(out, fmt.Sprintf("%s: needs self-reference", t.ID))
			case !ids[dep]:
				out = append(out, fmt.Sprintf("%s: needs unknown id %q", t.ID, dep))
			}
		}
	}

	const (
		white = iota
		gray
		black
	)
	state := map[string]int{}
	var visit func(id string, path []string)
	visit = func(id string, path []string) {
		state[id] = gray
		path = append(path, id)
		for _, dep := range byID[id].Needs {
			if dep == id || !ids[dep] {
				continue // self-reference / unknown id already reported above
			}
			switch state[dep] {
			case white:
				visit(dep, path)
			case gray:
				cycle := append(append([]string{}, path...), dep)
				out = append(out, fmt.Sprintf("%s: needs cycle (%s)", id, strings.Join(cycle, " -> ")))
			}
		}
		state[id] = black
	}
	for _, t := range p.Tasks {
		if state[t.ID] == white {
			visit(t.ID, nil)
		}
	}
	return out
}
