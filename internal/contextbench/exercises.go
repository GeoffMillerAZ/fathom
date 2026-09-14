package contextbench

import (
	"fmt"
	"sort"
	"strings"
)

// Targeted runs (ADR-032 §1 L3; docs/design/bench-pyramid.md §3.4). A full
// three-arm home run is ~117 cells and ~15M tokens, which is a quarterly
// budget, not a feedback loop. Most changes cannot move most cells: a fix
// in the obligations handler cannot change what `find_callers` returns,
// and nothing in fathom's tree can change what the grep arm does at all.
//
// So each task declares the tools it was written to exercise, a committed
// table maps changed files to those tool names, and a targeted run re-runs
// only the intersection — fathom arm only. The row it writes is scoped
// `targeted` and the evaluator refuses to turn it into a verdict
// (Criterion.Evaluate): per-task evidence is the point, and a corpus-level
// claim from a diff-selected subset would be a claim about a sample that
// the change itself chose.

// agentSurfaceTools is the coding-agent surface's tool names, mirroring
// companionTools in internal/mcp/surface.go. It is a committed copy, not
// an import: this package is the bench, and a bench that imports the
// server it measures gains a build edge for no gain in truth.
// TestAgentSurfaceToolsMatchRegistry re-reads internal/mcp/surface.go and
// fails when the two drift, so the copy cannot go quietly stale.
var agentSurfaceTools = []string{
	"blast_radius",
	"capabilities",
	"discoveries",
	"entrypoints",
	"find_callers",
	"find_symbol",
	"ingest",
	"list_services",
	"localize",
	"memory_search",
	"obligations",
	"outline",
	"pack_context",
	"query_symbols",
	"read_source",
	"references",
	"rename_impact",
	"repo_map",
	"review_diff",
	"safe_delete_check",
	"search",
	"staleness_report",
	"symbol_context",
	"test_impact",
	"verify_plan",
}

// AgentSurfaceTools returns the agent-surface tool names a task may
// declare in `exercises`, sorted.
func AgentSurfaceTools() []string {
	out := append([]string(nil), agentSurfaceTools...)
	sort.Strings(out)
	return out
}

func isAgentSurfaceTool(name string) bool {
	for _, t := range agentSurfaceTools {
		if t == name {
			return true
		}
	}
	return false
}

// handlerRule maps a repo path — an exact file, or a directory prefix
// ending in "/" — to the agent-surface tools a change there can move.
// Prefix rules are deliberately generous: over-selecting costs one extra
// cell, while under-selecting means a change ships unmeasured, and only
// one of those two errors is silent.
type handlerRule struct {
	Path  string
	Tools []string
	Why   string
}

// handlerTable is the committed diff→tool map (BP-7). The internal/mcp
// rows are the handler files each tool is registered in (grep
// `registerTool(toolDescriptor{` there); the directory rows are the query
// and compilation layers those handlers read through.
var handlerTable = []handlerRule{
	{Path: "internal/mcp/tools.go", Tools: []string{"find_symbol", "find_callers", "query_symbols"}, Why: "the three symbol tools are registered here"},
	{Path: "internal/mcp/nav_tools.go", Tools: []string{"entrypoints", "outline", "symbol_context"}},
	{Path: "internal/mcp/references_tool.go", Tools: []string{"references"}},
	{Path: "internal/mcp/discoveries_tool.go", Tools: []string{"discoveries"}},
	{Path: "internal/mcp/analysis_tools.go", Tools: []string{"rename_impact", "test_impact"}, Why: "the impact verbs promoted by VB-1"},
	{Path: "internal/mcp/extra_tools.go", Tools: []string{"blast_radius"}, Why: "promoted by VB-1"},
	{Path: "internal/mcp/review_tool.go", Tools: []string{"review_diff"}, Why: "promoted by VB-1"},
	{Path: "internal/mcp/safe_delete.go", Tools: []string{"safe_delete_check"}, Why: "VB-2"},
	{Path: "internal/mcp/localize_tool.go", Tools: []string{"localize"}, Why: "promoted by VB-26 (wave G)"},
	{Path: "internal/mcp/readsource.go", Tools: []string{"read_source"}},
	{Path: "internal/mcp/semantic_tools.go", Tools: []string{"obligations"}},
	{Path: "internal/mcp/verify_plan_tool.go", Tools: []string{"verify_plan"}},
	{Path: "internal/mcp/meta.go", Tools: []string{"capabilities", "staleness_report"}},
	{Path: "internal/mcp/platform_tools.go", Tools: []string{"list_services"}},
	{Path: "internal/mcp/memory_tools.go", Tools: []string{"memory_search"}},
	{Path: "internal/mcp/extra_tools.go", Tools: []string{"ingest", "pack_context", "repo_map"}},
	{Path: "internal/mcp/budget.go", Tools: []string{"query_symbols", "references", "repo_map", "pack_context", "outline"}, Why: "the shared payload-budget helper truncates every list-returning tool"},
	// Anything else under internal/mcp is shared plumbing (dispatch, the
	// surface allowlist, schemas): it can move every tool on the surface.
	{Path: "internal/mcp/", Tools: agentSurfaceTools, Why: "shared MCP plumbing: dispatch, surfaces, schemas"},
	{Path: "internal/semantic/", Tools: []string{"obligations", "staleness_report", "verify_plan"}, Why: "the layer compiler and claim server feed the governance tools"},
	{Path: "internal/query/", Tools: []string{"find_symbol", "find_callers", "references", "symbol_context", "outline", "entrypoints", "query_symbols", "repo_map", "pack_context"}, Why: "the query engine every navigation tool reads through"},
	{Path: "internal/fusion/", Tools: []string{"find_symbol", "find_callers", "references", "symbol_context", "outline", "entrypoints", "query_symbols", "repo_map", "pack_context", "ingest"}, Why: "extraction and fusion decide what the index contains at all"},
}

// ToolsForFiles maps changed repo-relative paths to the agent-surface
// tools a change there can move, and names the files that map to nothing.
// Both outputs are sorted.
func ToolsForFiles(files []string) (tools []string, unmapped []string) {
	hit := map[string]bool{}
	for _, f := range files {
		f = strings.TrimPrefix(strings.TrimSpace(f), "./")
		if f == "" {
			continue
		}
		matched := false
		for _, rule := range handlerTable {
			if strings.HasSuffix(rule.Path, "/") {
				if !strings.HasPrefix(f, rule.Path) {
					continue
				}
			} else if f != rule.Path {
				continue
			}
			matched = true
			for _, t := range rule.Tools {
				hit[t] = true
			}
			// Exact file rules win outright; prefix rules keep scanning so
			// a file under two prefixes collects both.
			if !strings.HasSuffix(rule.Path, "/") {
				break
			}
		}
		if !matched {
			unmapped = append(unmapped, f)
		}
	}
	for t := range hit {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	sort.Strings(unmapped)
	return tools, unmapped
}

// Selection is SelectTasks' answer: which tasks a diff makes worth
// re-running, why, and what the selector could not account for.
type Selection struct {
	TaskIDs []string `json:"task_ids"`
	Tools   []string `json:"tools"`
	// Unmapped lists changed files no handler rule covers. They are
	// reported, never hidden: a targeted run over a diff the table does
	// not understand is exactly the case where the targeted row must not
	// become a verdict.
	Unmapped []string `json:"unmapped,omitempty"`
	// Undeclared lists tasks that declare no `exercises` — they can never
	// be selected, so a corpus that lets them accumulate silently shrinks
	// what a targeted run covers.
	Undeclared []string `json:"undeclared,omitempty"`
}

// SelectTasks returns the tasks whose declared `exercises` intersect the
// tools the diff's files can move. Order is corpus order for tasks and
// sorted for everything else, so the same diff always selects the same
// cells (the determinism contract).
func SelectTasks(diffFiles []string, tasks []Task) Selection {
	tools, unmapped := ToolsForFiles(diffFiles)
	want := make(map[string]bool, len(tools))
	for _, t := range tools {
		want[t] = true
	}
	sel := Selection{Tools: tools, Unmapped: unmapped}
	for _, task := range tasks {
		if len(task.Exercises) == 0 {
			sel.Undeclared = append(sel.Undeclared, task.ID)
			continue
		}
		for _, e := range task.Exercises {
			if want[e] {
				sel.TaskIDs = append(sel.TaskIDs, task.ID)
				break
			}
		}
	}
	sort.Strings(sel.Undeclared)
	return sel
}

// validateExercises is Task.Validate's clause for the `exercises` list: a
// tool name nobody serves is a typo that would silently make a task
// unselectable.
func validateExercises(t Task) error {
	for _, e := range t.Exercises {
		if !isAgentSurfaceTool(e) {
			return fmt.Errorf("task %s: exercises names %q, which is not an agent-surface tool (want one of %s)", t.ID, e, strings.Join(AgentSurfaceTools(), ", "))
		}
	}
	return nil
}
