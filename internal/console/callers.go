package console

// callers.go — who calls a tool (vision 35 §3, design §3 "Operate", survey
// gap 5: "the tool bars are dominated by plumbing"). A call-log row names
// its tool, the surface it arrived on and, since GI-5, the origin its
// caller declared (X-Fathom-Origin). One classification labels every row
// with the caller class it belongs to, so a tool table can never let fathom
// talking to itself pass for an agent choosing to look something up.
//
// The classes, in the order the console lists them:
//
//   - agent      — an agent chose the call (a named MCP surface, no hook origin)
//   - edit hook  — rules injected at an edit or a prompt (edit-hook, prompt-hook)
//   - gate       — the done-means-proven gate (verify-cli, stop-hook, verify_plan/review off the agent surface)
//   - refresh    — keeping the index current (the ingest tool)
//   - plumbing   — fathom's own round trips (session-hook, prime, init, doctor, registry reads)
//   - unattributed — no origin and no surface: a caller older than the origin header (GI-5), shown as its own row, never folded into another
//
// The console's own reads (origin console) are not usage: ClassConsole marks
// them so a reader can count and exclude them, as every usage reader does.

import "sort"

// CallerClass is who made a call.
type CallerClass string

const (
	ClassAgent        CallerClass = "agent"
	ClassEditHook     CallerClass = "edit hook"
	ClassGate         CallerClass = "gate"
	ClassRefresh      CallerClass = "refresh"
	ClassPlumbing     CallerClass = "plumbing"
	ClassUnattributed CallerClass = "unattributed"
	ClassConsole      CallerClass = "console"
)

// CallerClasses is the fixed display order of the usage classes (console
// reads are never usage and are not listed).
var CallerClasses = []CallerClass{ClassAgent, ClassEditHook, ClassGate, ClassRefresh, ClassPlumbing, ClassUnattributed}

// classRank orders classes for display and for breaking a dominance tie.
func classRank(c CallerClass) int {
	for i, x := range CallerClasses {
		if x == c {
			return i
		}
	}
	return len(CallerClasses)
}

// plumbingTools are fathom's registry and identity round trips: a caller
// reading them is plumbing whatever surface it arrived on (the survey's
// list_services, 33k calls, "passing for adoption").
var plumbingTools = map[string]bool{"list_services": true, "status": true}

// gateTools are the gate's own verbs; off the agent surface they are the
// gate running, not an agent asking.
var gateTools = map[string]bool{"verify_plan": true, "review_diff": true}

// ClassifyCaller labels one call-log row by its tool, surface and declared
// origin. The origin wins when present: a hook that happens to call through
// the agent surface is still the hook.
func ClassifyCaller(tool, surface, origin string) CallerClass {
	switch origin {
	case "console":
		return ClassConsole
	case "edit-hook", "prompt-hook":
		return ClassEditHook
	case "verify-cli", "stop-hook", "claim-moment":
		return ClassGate
	case "session-hook", "prime", "init", "doctor", "worker", "post-commit-hook":
		if tool == "ingest" {
			return ClassRefresh
		}
		return ClassPlumbing
	}
	switch {
	case tool == "ingest":
		return ClassRefresh
	case plumbingTools[tool]:
		return ClassPlumbing
	}
	onNamedSurface := surface != ""
	switch {
	case gateTools[tool] && !onNamedSurface:
		return ClassGate
	case onNamedSurface:
		return ClassAgent
	}
	return ClassUnattributed
}

// ClassCount is one caller class's share of a tool's calls.
type ClassCount struct {
	Class CallerClass `json:"class"`
	Calls int         `json:"calls"`
}

// ToolLoad is one tool's calls in a window, split by caller class.
type ToolLoad struct {
	Tool  string       `json:"tool"`
	Calls int          `json:"calls"`
	Share float64      `json:"share"` // of every usage call in the window, 0..1
	Class CallerClass  `json:"class"` // the class making most of its calls
	By    []ClassCount `json:"by_class"`
}

// CallRow is the minimal call-log row the console projections read.
type CallRow struct {
	TS        string
	Tool      string
	Service   string
	Surface   string
	Origin    string
	LatencyMS float64
	Error     string
}

// ToolLoads folds usage rows (console reads excluded) into per-tool loads,
// most-called first (ties by name), each with its dominant class.
func ToolLoads(rows []CallRow) []ToolLoad {
	counts := map[CallerCount]int{}
	for _, r := range rows {
		counts[CallerCount{Tool: r.Tool, Surface: r.Surface, Origin: r.Origin}]++
	}
	out := make([]CallerCount, 0, len(counts))
	for k, n := range counts {
		k.Calls = n
		out = append(out, k)
	}
	return ToolLoadsCounted(out)
}

// CallerCount is the calls of one tool from one surface and origin — what
// a caller class is decided from — so an all-time fold can keep counts
// instead of rows (RT-112).
type CallerCount struct {
	Tool, Surface, Origin string
	Calls                 int
}

// ToolLoadsCounted is ToolLoads over counted rows.
func ToolLoadsCounted(counts []CallerCount) []ToolLoad {
	byTool := map[string]map[CallerClass]int{}
	total := 0
	for _, r := range counts {
		if r.Tool == "" || r.Calls <= 0 {
			continue
		}
		c := ClassifyCaller(r.Tool, r.Surface, r.Origin)
		if c == ClassConsole {
			continue
		}
		if byTool[r.Tool] == nil {
			byTool[r.Tool] = map[CallerClass]int{}
		}
		byTool[r.Tool][c] += r.Calls
		total += r.Calls
	}
	out := make([]ToolLoad, 0, len(byTool))
	for tool, counts := range byTool {
		tl := ToolLoad{Tool: tool}
		best := -1
		for _, c := range CallerClasses {
			n := counts[c]
			if n == 0 {
				continue
			}
			tl.Calls += n
			tl.By = append(tl.By, ClassCount{Class: c, Calls: n})
			if n > best {
				best, tl.Class = n, c
			}
		}
		if total > 0 {
			tl.Share = float64(tl.Calls) / float64(total)
		}
		out = append(out, tl)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Calls != out[j].Calls {
			return out[i].Calls > out[j].Calls
		}
		return out[i].Tool < out[j].Tool
	})
	return out
}
