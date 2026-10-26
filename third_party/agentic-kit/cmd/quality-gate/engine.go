package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"
)

// evaluateRules applies policy rules to the immutable snapshot. Rule order is
// policy order and paths within each rule are sorted, so findings are stable.
func evaluateRules(p policy, boundary string, s snapshot) ([]ruleResult, []finding) {
	results := make([]ruleResult, 0, len(p.Rules))
	findings := make([]finding, 0)
	changes := slices.Clone(s.Changes)
	slices.SortFunc(changes, func(a, b change) int { return strings.Compare(a.Path, b.Path) })
	for _, r := range p.Rules {
		result := ruleResult{ID: r.ID, State: "not-applicable"}
		if !includes(r.Boundaries, boundary) {
			result.Reason = "boundary-not-configured"
			results = append(results, result)
			continue
		}
		selected := make([]change, 0)
		for _, c := range changes {
			if matches(r.Select, c.Path) {
				selected = append(selected, c)
			}
		}
		result.Matched = len(selected)
		if len(selected) == 0 {
			result.Reason = "no-selected-changes"
			results = append(results, result)
			continue
		}
		applicable := false
		failed, unknown := false, false
		add := func(c change, kind, reason string, line int) {
			findings = append(findings, finding{RuleID: r.ID, Kind: kind, Path: c.Path, Line: line, Reason: reason})
			if kind == "unknown" {
				unknown = true
			} else {
				failed = true
			}
		}
		for _, c := range selected {
			// Deletions have no after-content or forbidden surviving path, but
			// deleting a contract can still require a changed companion.
			if c.AfterOID == "" && r.Kind != "require-companion" {
				continue
			}
			applicable = true
			switch r.Kind {
			case "forbidden-path":
				add(c, r.Kind, "forbidden path added or modified", 0)
			case "require-companion":
				if r.Companion == nil || !hasCompanion(changes, *r.Companion, c.Path) {
					add(c, r.Kind, "required changed companion is absent", 0)
				}
			case "conflict-markers", "max-bytes", "json-valid":
				if reason := ruleContentReason(c); reason != "" {
					add(c, "unknown", reason, 0)
					continue
				}
				switch r.Kind {
				case "conflict-markers":
					for _, line := range conflictMarkerLines(c.After) {
						add(c, r.Kind, "conflict marker present", line)
					}
				case "max-bytes":
					if len(c.After) > r.MaxBytes {
						add(c, r.Kind, "file exceeds maximum bytes", 0)
					}
				case "json-valid":
					if !json.Valid(c.After) {
						add(c, r.Kind, "invalid JSON", 0)
					}
				}
			default:
				add(c, "unknown", "unsupported rule kind", 0)
			}
		}
		switch {
		case failed:
			result.State = "fail"
		case unknown:
			result.State = "unknown"
		case applicable:
			result.State = "pass"
		default:
			result.Reason = "no-surviving-content"
		}
		results = append(results, result)
	}
	return results, findings
}

func hasCompanion(changes []change, sel selector, triggerPath string) bool {
	for _, c := range changes {
		if c.Path != triggerPath && c.AfterOID != "" && matches(sel, c.Path) {
			return true
		}
	}
	return false
}

func ruleContentReason(c change) string {
	if c.AfterMode != "100644" && c.AfterMode != "100755" {
		return "selected file is not regular"
	}
	if c.After == nil {
		return "selected content is unavailable"
	}
	if bytes.IndexByte(c.After, 0) >= 0 || !utf8.Valid(c.After) {
		return "selected content is binary or invalid UTF-8"
	}
	return ""
}

func conflictMarkerLines(content []byte) []int {
	lines := make([]int, 0)
	for i, line := range bytes.Split(content, []byte{'\n'}) {
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if hasRun(line, '<') || hasRun(line, '=') || hasRun(line, '>') {
			lines = append(lines, i+1)
		}
	}
	return lines
}

func hasRun(line []byte, marker byte) bool {
	if len(line) < 7 {
		return false
	}
	for _, b := range line[:7] {
		if b != marker {
			return false
		}
	}
	return true
}
