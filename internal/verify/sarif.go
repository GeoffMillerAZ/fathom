package verify

// sarif.go — SARIF 2.1.0 for gate refusals (PC-21, docs/plans/proof-credibility.md).
// `fathom verify --format sarif` renders every refusal as one result whose
// rule id is the criterion that failed (symbol_present, test_reaches,
// lexical_oracle, …) and whose location is the plan file and the row's
// line, so a code-scanning upload or an editor problem pane shows the
// refusal at the claim. `intent audit` and `config audit` share the
// emitter through RenderSARIF with their own rule vocabularies.
//
// Deterministic: rules are sorted by id; results keep the caller's order
// (the caller passes them in report order, which is sorted by task id).

import (
	"encoding/json"
	"fmt"
	"sort"
)

// SARIFResult is one finding for renderSARIF.
type SARIFResult struct {
	RuleID  string // the criterion / claim kind / concern id
	Level   string // "error" | "warning" | "note" (SARIF vocabulary)
	Message string
	File    string // repo-relative artifact path
	Line    int    // 1-based; 0 omits the region
}

// sarifSchema is the published SARIF 2.1.0 schema location — a document
// identity, not a network fetch.
const sarifSchema = "https://json.schemastore.org/sarif-2.1.0.json"

// renderSARIF builds the SARIF 2.1.0 log for tool over results. One run,
// one driver, one rule per distinct RuleID. Always valid JSON, always one
// run even with zero results (an empty results array is "the gate ran and
// refused nothing"; a missing run would be indistinguishable from "did not
// run").
func renderSARIF(tool string, results []SARIFResult) []byte {
	ruleIDs := map[string]bool{}
	for _, r := range results {
		ruleIDs[r.RuleID] = true
	}
	ids := make([]string, 0, len(ruleIDs))
	for id := range ruleIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rules := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		rules = append(rules, map[string]any{
			"id":               id,
			"shortDescription": map[string]any{"text": id},
		})
	}

	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		level := r.Level
		if level == "" {
			level = "error"
		}
		loc := map[string]any{
			"artifactLocation": map[string]any{"uri": r.File},
		}
		if r.Line > 0 {
			loc["region"] = map[string]any{"startLine": r.Line}
		}
		out = append(out, map[string]any{
			"ruleId":    r.RuleID,
			"level":     level,
			"message":   map[string]any{"text": r.Message},
			"locations": []map[string]any{{"physicalLocation": loc}},
		})
	}

	doc := map[string]any{
		"$schema": sarifSchema,
		"version": "2.1.0",
		"runs": []map[string]any{{
			"tool": map[string]any{"driver": map[string]any{
				"name":  tool,
				"rules": rules,
			}},
			"results": out,
		}},
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		// Only map[string]any of strings/ints/bools is marshalled above;
		// an error here is a programming error, surfaced as a SARIF note
		// rather than swallowed.
		return []byte(fmt.Sprintf(`{"$schema":%q,"version":"2.1.0","runs":[{"tool":{"driver":{"name":%q}},"results":[],"invocations":[{"executionSuccessful":false,"toolExecutionNotifications":[{"message":{"text":%q}}]}]}]}`, sarifSchema, tool, err.Error()))
	}
	return append(b, '\n')
}

// RenderSARIF is renderSARIF's exported face — intent audit and config
// audit hand it their own results.
func RenderSARIF(tool string, results []SARIFResult) []byte {
	return renderSARIF(tool, results)
}

// GateSARIF collects the refusals Gate would report for reports — one
// result per failing criterion of each claimed, non-skipped task, plus,
// under strict, one warning per claimed task whose status is Unknown (the
// row's label is the rule: "no-contract", "oracle-unevaluated"). planFiles
// and reports are parallel, as cmdVerify holds them.
func GateSARIF(planFiles []string, reports []Report, strict bool) []SARIFResult {
	var out []SARIFResult
	for i, rep := range reports {
		file := ""
		if i < len(planFiles) {
			file = planFiles[i]
		}
		for _, tv := range rep.Tasks {
			if !tv.Claimed || tv.Skipped {
				continue
			}
			switch {
			case tv.Status == Fail:
				for _, c := range tv.Criteria {
					if c.Status != Fail {
						continue
					}
					out = append(out, SARIFResult{
						RuleID: c.Name, Level: "error",
						Message: fmt.Sprintf("%s %s: %s", tv.TaskID, tv.Title, c.Detail),
						File:    file, Line: tv.Line,
					})
				}
			case strict && tv.Status == Unknown:
				out = append(out, SARIFResult{
					RuleID: tv.Label, Level: "warning",
					Message: fmt.Sprintf("%s %s: claimed [x] but unknown under --strict (%s)", tv.TaskID, tv.Title, gateFailDetail(tv, true)),
					File:    file, Line: tv.Line,
				})
			}
		}
	}
	return out
}
