package specdialect

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/internal/worktree"
)

var foreignTaskHeadingRe = regexp.MustCompile(`^\s*#{1,6}(?:\s|$)`)

// foreignPlanTestRefs preserves references while the foreign source is still
// available, before Synthesize renders its canonical rows. Dialect parsers
// remain pure; only Spec Kit's explicitly named test files are read here.
func foreignPlanTestRefs(d SpecDialect, root, file string, content []byte, plan verify.Plan) verify.Plan {
	var strategies map[string]string
	switch d.ID() {
	case "kiro":
	case "speckit":
		if root == "" && specKitPath(file) {
			root = worktree.RootForPath(file)
			if root == "" {
				// Exported specs may have no Git metadata; retain the layout root.
				root = filepath.Dir(filepath.Dir(filepath.Dir(file)))
			}
		}
	case "taskmaster":
		var doc struct {
			Tasks []tmTask `json:"tasks"`
		}
		if json.Unmarshal(content, &doc) != nil {
			return plan
		}
		strategies = map[string]string{}
		var walk func([]tmTask, string)
		walk = func(tasks []tmTask, parent string) {
			for _, task := range tasks {
				id := strings.TrimSpace(task.ID.String())
				if parent != "" {
					id = parent + "." + id
				}
				strategies[id] = task.TestStrategy
				walk(task.Subtasks, id)
			}
		}
		walk(doc.Tasks, "")
	default:
		return plan
	}
	lines := strings.Split(string(content), "\n")
	plan.Tasks = append([]verify.Task(nil), plan.Tasks...)
	for i := range plan.Tasks {
		t := &plan.Tasks[i]
		// Derived names only tighten symbol contracts. An explicit
		// certified_by is author-owned, including its name order.
		if t.Expect.Symbol == "" || len(t.Expect.CertifiedBy) > 0 {
			continue
		}
		text := t.Title
		switch d.ID() {
		case "kiro":
			for j := t.Line; j > 0 && j < len(lines) && !checkboxLine.MatchString(lines[j]) && !foreignTaskHeadingRe.MatchString(lines[j]); j++ {
				text += " " + lines[j]
			}
		case "taskmaster":
			text = strategies[t.ID]
		}
		t.DerivedTests = mapForeignTestRefs(root, text, d.ID() == "speckit" && root != "")
	}
	return plan
}

// Synthesize renders plan as a native grammar-v2 checklist: one row per
// task, every task's id and contract written into its annotation block, so
// verify.ParseMarkdownPlan reads back the same tasks (Line aside). It is
// how a foreign plan reaches a consumer that only speaks the native
// grammar — the daemon's verify_plan tool — without the foreign file ever
// being rewritten: the synthesized text is sent, never written to disk.
func Synthesize(plan verify.Plan) []byte {
	var b strings.Builder
	for _, t := range plan.Tasks {
		check := " "
		switch {
		case t.Claimed:
			check = "x"
		case t.Skipped:
			check = "-"
		case t.InProgress:
			check = "~"
		}
		title := strings.TrimSpace(t.Title)
		if title == "" {
			title = t.ID
		}
		b.WriteString("- [" + check + "] " + title + " {" + strings.Join(synthPairs(t), ", ") + "}\n")
	}
	return []byte(b.String())
}

// synthPairs is the annotation block for t, in a fixed key order.
func synthPairs(t verify.Task) []string {
	var out []string
	add := func(k, v string) {
		if v != "" {
			out = append(out, k+": "+synthValue(v))
		}
	}
	addBool := func(k string, v bool) {
		if v {
			out = append(out, k+": true")
		}
	}
	addInt := func(k string, v *int) {
		if v != nil {
			out = append(out, k+": "+strconv.Itoa(*v))
		}
	}
	e := t.Expect
	add("id", t.ID)
	add("symbol", e.Symbol)
	add("package", e.Package)
	add("reachable_from", e.ReachableFrom)
	add("min_tier", string(e.MinTier))
	addBool("skip_test", e.SkipTest)
	add("skip", e.SkipReason)
	addBool("allow_contradiction", e.AllowContradiction)
	addBool("library", e.Library)
	add("certified_by", strings.Join(e.CertifiedBy, ","))
	add("derived_tests", strings.Join(t.DerivedTests, ","))
	add("strength", e.CertifyStrength)
	add("disposition", t.Disposition)
	add("reason", t.Reason)
	add("trigger", t.Trigger)
	if t.Oracle != "" {
		add("oracle", t.Oracle)
		add("path", t.OraclePath)
		add("pattern", t.OraclePattern)
		add("run_id", t.OracleRunID)
		add("task", t.OracleTaskID)
		add("arm", t.OracleArm)
		add("min_trials", t.OracleMinTrials)
		add("criterion", t.OracleCriterion)
		add("accept_verdict", t.OracleAcceptVerdict)
	}
	add("ctx", strings.Join(t.Ctx, " "))
	add("needs", strings.Join(t.Needs, ","))
	add("gone", t.Gone)
	add("tests", t.Tests)
	add("paths", strings.Join(t.GonePaths, ","))
	add("retires", strings.Join(t.Retires, ","))
	add("surface_change", strings.Join(t.SurfaceChange, ","))
	addInt("max", t.Max)
	addInt("min", t.Min)
	addInt("eq", t.Eq)
	if t.Weight > 1 {
		out = append(out, "weight: "+strconv.Itoa(t.Weight))
	}
	add("accept", t.Accept)
	return out
}

// synthValue quotes v when the grammar requires it: a comma or a brace
// (which would end the pair or the block), or surrounding space. A value
// is otherwise written as-is, inner double quotes included — the
// tokenizer toggles on each bare one and reads an escaped one as data, so
// a value that parsed from a row reads back the same when written the way
// that row wrote it.
func synthValue(v string) string {
	if strings.ContainsAny(v, ",{}") || strings.TrimSpace(v) != v {
		return `"` + v + `"`
	}
	return v
}
