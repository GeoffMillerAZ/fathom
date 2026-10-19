package specdialect

import (
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

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
