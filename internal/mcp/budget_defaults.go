// Package mcp — per-tool budget defaults (docs/design/budgeted-answers.md
// §3, BA-5).
//
// An agent that never passes a budget still gets sane behaviour: the
// measured defaults below, resolved project → user → built-in — the same
// most-specific-wins precedence internal/capability.Resolution.For uses
// (law 13) — so a project sets its own budgets once rather than at every
// call site. One settings key family (toolBudgetClass), kept minimal: this
// package does not itself load .fathom/capabilities.cue; a caller wires a
// loaded budgetSettings in when project/user overrides exist, and every
// truncating tool still works correctly with a zero-value budgetSettings
// (falls straight through to the measured built-in default).
package mcp

// toolBudgetClass categorizes truncating tools per the design's §3 table.
type toolBudgetClass string

const (
	classList      toolBudgetClass = "list"      // references, find_callers, query_symbols, find_symbol
	classComposite toolBudgetClass = "composite" // symbol_context, pack_context
	classMap       toolBudgetClass = "map"       // repo_map, outline
)

// measuredDefaults are the design's §3 measured defaults, in approximate
// tokens — BA-8 labels any tokens-from-bytes conversion an ESTIMATE, and
// this is that same estimate run in reverse (charsPerToken converts once,
// here, into the char units fitToBudget/applyBudget operate in).
var measuredDefaults = map[toolBudgetClass]int{
	classList:      1500,
	classComposite: 4000,
	classMap:       2500,
}

// charsPerToken is the same ~4-bytes-per-token estimate used fleet-wide
// (internal/context/packer.tokenEstimate; BA-8's ESTIMATE label).
const charsPerToken = 4

// toolBudgetClasses maps each truncating tool to its design-table class.
// A tool absent here has no declared class — budgetDefaults returns 0
// (defaultCharBudget applies) — which is correct for find_symbol: it is a
// point lookup by the design table (§3), and its substring-tier cap is a
// safety valve, not a class this table sizes.
var toolBudgetClasses = map[string]toolBudgetClass{
	"query_symbols": classList,
	"references":    classList,
	"pack_context":  classComposite,
	"repo_map":      classMap,
}

// budgetSettings is one project's resolved override, keyed by tool-class id
// — the "one settings key family" BA-5 asks for. A zero value means no
// override at either scope, and budgetDefaults falls back to the measured
// built-in.
type budgetSettings struct {
	User    map[string]int
	Project map[string]int
}

// budgetDefaults resolves the char budget for one tool by its design-table
// class: project scope wins over user scope wins over the measured
// built-in default — mirroring capability.Resolution.For's precedence
// exactly (law 13: a project sets its own behaviour; a user scope may only
// default it). Returns 0 (no declared class) for a tool budgetDefaults does
// not size — callers treat 0 as "use fitToBudget's own default".
func budgetDefaults(tool string, s budgetSettings) int {
	v, ok := budgetDefaultTokens(tool, s)
	if !ok {
		return 0
	}
	return v * charsPerToken
}

// budgetDefaultTokens resolves the raw token default (design §3's own
// units) for tools whose own budget parameter is already token-shaped —
// repo_map and pack_context pass their budget straight to
// internal/context/packer, which token-estimates internally — so it skips
// applyBudget/fitToBudget's char conversion entirely. Same project → user →
// built-in precedence as budgetDefaults. ok is false for a tool with no
// declared class.
func budgetDefaultTokens(tool string, s budgetSettings) (int, bool) {
	class, ok := toolBudgetClasses[tool]
	if !ok {
		return 0, false
	}
	if v, ok := s.Project[string(class)]; ok {
		return v, true
	}
	if v, ok := s.User[string(class)]; ok {
		return v, true
	}
	return measuredDefaults[class], true
}
