package contextbench

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestTasksDeclareExercises is BP-6's anchor: every home task says which
// agent-surface tools it was written to exercise, so a diff can select it.
// A task with no declaration is invisible to every targeted run — the
// silent way a corpus shrinks.
func TestTasksDeclareExercises(t *testing.T) {
	tasks, err := LoadTasks(homeCorpusPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	surface := map[string]bool{}
	for _, name := range AgentSurfaceTools() {
		surface[name] = true
	}
	for _, task := range tasks {
		if len(task.Exercises) == 0 {
			t.Errorf("%s: declares no exercises — no diff can ever select it", task.ID)
			continue
		}
		seen := map[string]bool{}
		for _, e := range task.Exercises {
			if !surface[e] {
				t.Errorf("%s: exercises %q, which is not an agent-surface tool", task.ID, e)
			}
			if seen[e] {
				t.Errorf("%s: exercises %q twice", task.ID, e)
			}
			seen[e] = true
		}
	}

	// An unknown tool name must fail validation, not be quietly carried:
	// it would make the task unselectable forever.
	bad := Task{ID: "X", Prompt: "p", GroundTruth: []string{"g"}, ScoringRule: AnyOf, Exercises: []string{"find_symbols"}}
	if err := bad.Validate(); err == nil {
		t.Error("Task.Validate accepted an unknown tool name in exercises")
	}
}

// TestAgentSurfaceToolsMatchRegistry holds the committed copy of the
// agent-surface tool names to internal/mcp/surface.go's companionTools,
// without importing the server package into the bench.
func TestAgentSurfaceToolsMatchRegistry(t *testing.T) {
	raw, err := os.ReadFile("../mcp/surface.go")
	if err != nil {
		t.Fatalf("reading the registry: %v", err)
	}
	body := string(raw)
	// companionTools is built by a function since VB-1: a base map literal,
	// the verbs promotedVerbs() names, and safe_delete_check. Parse all three
	// without importing the server package into the bench.
	start := strings.Index(body, "var companionTools = func() map[string]bool {")
	if start < 0 {
		t.Fatal("companionTools is no longer built by a func literal in internal/mcp/surface.go — re-point this test at the registry")
	}
	end := strings.Index(body[start:], "\n}()")
	if end < 0 {
		t.Fatal("could not find the end of the companionTools builder")
	}
	builder := body[start : start+end]
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`"([a-z_]+)":\s*true`).FindAllStringSubmatch(builder, -1) {
		seen[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`m\["([a-z_]+)"\] = true`).FindAllStringSubmatch(builder, -1) {
		seen[m[1]] = true
	}
	verbs, err := os.ReadFile("../mcp/promoted_verbs.go")
	if err != nil {
		t.Fatalf("reading the promoted verbs: %v", err)
	}
	for _, m := range regexp.MustCompile(`Name:\s*"([a-z_]+)"`).FindAllStringSubmatch(string(verbs), -1) {
		seen[m[1]] = true
	}
	var want []string
	for k := range seen {
		want = append(want, k)
	}
	sort.Strings(want)
	got := AgentSurfaceTools()
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Errorf("the bench's agent-surface copy has drifted from internal/mcp/surface.go\n registry: %v\n bench:    %v", want, got)
	}
}

// TestSelectTasksMapsDiffToTasks is BP-7: a change under a handler file
// selects the tasks that exercise the tools that file registers, and
// nothing else.
func TestSelectTasksMapsDiffToTasks(t *testing.T) {
	tasks, err := LoadTasks(homeCorpusPath)
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}

	sel := SelectTasks([]string{"internal/mcp/semantic_tools.go"}, tasks)
	if len(sel.TaskIDs) == 0 {
		t.Fatal("a change to the obligations handler selects no task")
	}
	for _, id := range sel.TaskIDs {
		if !declares(tasks, id, "obligations") {
			t.Errorf("%s was selected for an obligations-handler change but does not exercise obligations", id)
		}
	}
	if got := strings.Join(sel.Tools, ","); got != "obligations" {
		t.Errorf("semantic_tools.go maps to %q, want obligations alone", got)
	}

	// internal/semantic is the compilation layer behind the governance
	// tools; internal/query is behind navigation.
	if tools, _ := ToolsForFiles([]string{"internal/semantic/compile.go"}); !contains(tools, "obligations") || !contains(tools, "staleness_report") {
		t.Errorf("internal/semantic maps to %v", tools)
	}
	if tools, _ := ToolsForFiles([]string{"internal/query/engine.go"}); !contains(tools, "find_callers") {
		t.Errorf("internal/query maps to %v", tools)
	}
	if tools, _ := ToolsForFiles([]string{"internal/fusion/fusion.go"}); !contains(tools, "ingest") {
		t.Errorf("internal/fusion maps to %v", tools)
	}

	// Shared MCP plumbing can move every tool on the surface.
	full := SelectTasks([]string{"internal/mcp/dispatch.go"}, tasks)
	if len(full.TaskIDs) != len(tasks) {
		t.Errorf("a dispatch change selects %d of %d tasks", len(full.TaskIDs), len(tasks))
	}

	// A diff the table does not understand selects nothing and SAYS so.
	none := SelectTasks([]string{"README.md", "internal/store/sqlite.go"}, tasks)
	if len(none.TaskIDs) != 0 {
		t.Errorf("an unrelated diff selected %v", none.TaskIDs)
	}
	if len(none.Unmapped) != 2 {
		t.Errorf("unmapped files must be named, got %v", none.Unmapped)
	}

	// Determinism: same diff, same selection, in corpus order.
	a := SelectTasks([]string{"internal/mcp/dispatch.go"}, tasks)
	b := SelectTasks([]string{"internal/mcp/dispatch.go"}, tasks)
	if strings.Join(a.TaskIDs, ",") != strings.Join(b.TaskIDs, ",") {
		t.Error("SelectTasks is not deterministic")
	}
}

func declares(tasks []Task, id, tool string) bool {
	for _, t := range tasks {
		if t.ID == id {
			return contains(t.Exercises, tool)
		}
	}
	return false
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
