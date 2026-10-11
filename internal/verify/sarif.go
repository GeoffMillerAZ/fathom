package verify

// sarif.go — SARIF 2.1.0 for gate refusals (PC-21, docs/plans/proof-credibility.md).
// `fathom verify --format sarif` renders every refusal as one result whose
// rule id is the criterion that failed (symbol_present, test_reaches,
// lexical_oracle, …) and whose location is the plan file and the row's
// line, so a code-scanning upload or an editor problem pane shows the
// refusal at the claim. Every SARIF caller (verify, obligations, rules
// replay, config audit, intent audit, `review --receipt`, review_diff's
// receipt_sarif) shares the emitter through RenderSARIFRun with its own rule
// vocabulary and the run's provenance (EVX-2, docs/plans/portable-receipts.md).
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
	// EndLine is the region's last line (1-based, inclusive); 0 or a value
	// not past Line writes no endLine.
	EndLine int
	// Fingerprint is the anchor's excerpt sha256 (internal/receipt's
	// AnchorAt), written as partialFingerprints["fathomExcerpt/v1"]; ""
	// writes none (no anchor, a withheld file, an uncommitted read).
	Fingerprint string
	// Help is the rule's help text; the first non-empty Help among a
	// rule's results is the rule's. "" falls back to a line naming the tool
	// and the rule.
	Help string
}

// sarifSchema is the published SARIF 2.1.0 schema location — a document
// identity, not a network fetch.
const sarifSchema = "https://json.schemastore.org/sarif-2.1.0.json"

// renderSARIF builds the SARIF 2.1.0 log for tool over results. One run,
// one driver, one rule per distinct RuleID, each with help text. Always
// valid JSON, always one run even with zero results (an empty results array
// is "the gate ran and refused nothing"; a missing run would be
// indistinguishable from "did not run"). prov, when set, adds the run's
// provenance (RenderSARIFRun); results' fingerprints are written either way.
func renderSARIF(tool string, results []SARIFResult, prov *RunProvenance) []byte {
	ruleIDs := map[string]bool{}
	help := map[string]string{}
	for _, r := range results {
		ruleIDs[r.RuleID] = true
		if help[r.RuleID] == "" {
			help[r.RuleID] = r.Help
		}
	}
	ids := make([]string, 0, len(ruleIDs))
	for id := range ruleIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rules := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		text := help[id]
		if text == "" {
			text = fmt.Sprintf("%s reports %s.", tool, id)
		}
		rules = append(rules, map[string]any{
			"id":               id,
			"shortDescription": map[string]any{"text": id},
			"help":             map[string]any{"text": text},
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
			region := map[string]any{"startLine": r.Line}
			if r.EndLine > r.Line {
				region["endLine"] = r.EndLine
			}
			loc["region"] = region
		}
		res := map[string]any{
			"ruleId":    r.RuleID,
			"level":     level,
			"message":   map[string]any{"text": r.Message},
			"locations": []map[string]any{{"physicalLocation": loc}},
		}
		if r.Fingerprint != "" {
			res["partialFingerprints"] = map[string]any{ExcerptFingerprintKey: r.Fingerprint}
		}
		out = append(out, res)
	}

	driver := map[string]any{
		"name":  tool,
		"rules": rules,
	}
	run := map[string]any{
		"tool":    map[string]any{"driver": driver},
		"results": out,
	}
	if prov != nil {
		if prov.Producer.Version != "" {
			driver["version"] = prov.Producer.Version
		}
		run["automationDetails"] = map[string]any{"id": automationID(tool, prov.Commit)}
		run["properties"] = map[string]any{"fathom": prov.properties()}
		if prov.RepositoryURI != "" {
			vcs := map[string]any{"repositoryUri": prov.RepositoryURI}
			if prov.Commit != "" {
				vcs["revisionId"] = prov.Commit
			}
			run["versionControlProvenance"] = []map[string]any{vcs}
		}
	}
	doc := map[string]any{
		"$schema": sarifSchema,
		"version": "2.1.0",
		"runs":    []map[string]any{run},
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

// RenderSARIFRun is the shared emitter every SARIF caller renders through
// (EVX-2, EVX-3): tool's results with the run's provenance — run.properties.
// fathom (commit, clean, producer, extractor epoch), automationDetails.id,
// rule help, each anchored result's partialFingerprints["fathomExcerpt/v1"],
// and versionControlProvenance only when prov names a repository URI. Two
// renders of the same input are byte-identical.
func RenderSARIFRun(tool string, prov RunProvenance, results []SARIFResult) []byte {
	return renderSARIF(tool, results, &prov)
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
					// An informational criterion (runtime_pass, I8) is
					// reported, never a refusal.
					if c.Status != Fail || informational(c.Name) {
						continue
					}
					out = append(out, SARIFResult{
						RuleID: c.Name, Level: "error",
						Message: fmt.Sprintf("%s %s: %s", tv.TaskID, tv.Title, c.Detail),
						File:    file, Line: tv.Line,
						Help: fmt.Sprintf("A plan row checked [x] failed its %s criterion: fathom verify cannot prove the claim. "+
							"Run `fathom verify --plan <plan> --row <id>` for the evidence, then fix the code or uncheck the row.", c.Name),
					})
				}
			case strict && tv.Status == Unknown:
				out = append(out, SARIFResult{
					RuleID: tv.Label, Level: "warning",
					Message: fmt.Sprintf("%s %s: claimed [x] but unknown under --strict (%s)", tv.TaskID, tv.Title, gateFailDetail(tv, true)),
					File:    file, Line: tv.Line,
					Help: fmt.Sprintf("A plan row checked [x] reads unknown (%s) under --strict: fathom verify could not evaluate it. "+
						"Give the row a contract fathom can check, or uncheck it.", tv.Label),
				})
			}
		}
	}
	return out
}
