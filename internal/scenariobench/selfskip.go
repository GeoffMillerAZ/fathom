// selfskip.go is wave N15 item 3: the scenariobench DRIVER's own
// instrument-truth check for O56's self-grading pattern, independent of
// the verify_plan MCP tool's own git-based self-skip gate
// (internal/verify/selfskip.go, internal/mcp/selfskip.go).
//
// That gate only runs when a live verify_plan call supplies BOTH
// plan_markdown AND plan_path (internal/mcp/verify_plan_tool.go): a call
// with a structured `tasks` array skips it entirely (the annotate step
// is only wired into the plan_markdown branch), and a plan_markdown call
// with no plan_path never fetches a prior git version to compare against
// (selfSkipPriorPlan requires it). reuse-pilot-4 10048/full/t2's own raw
// transcript shows exactly the second shape: the worker's verify_plan
// call that added `skip: "…"` alongside `skip_test: true` carried
// plan_markdown but no plan_path, so AnnotateSelfSkip never ran and the
// row that had just been correctly refused (GIST-TTY-3, a leaf test)
// came back claimed.
//
// The driver does not depend on what the agent chose to pass the MCP
// tool. It reads the SAME plan file's own edit history the session
// actually produced: BoxCheck, recorded (stream.go's recordBoxChecks)
// from every Edit/Write/MultiEdit tool call that writes a NEW "- [x]"
// checklist line, deduped by the line's own exact text — never by row
// id. A row claimed without skip_test, later re-edited to ADD
// skip_test (O56's "refusal reversed" pattern) produces a SECOND,
// textually distinct BoxCheck for the same row, caught the same way a
// simultaneous claim+skip is.
package scenariobench

// SelfSkipRows returns the row labels (RowID, falling back to Symbol,
// falling back to the raw checklist text) of every BoxCheck, across
// every session, whose own claimed ("- [x]") checklist line carries
// skip_test: true in its v2-grammar annotation block — the driver's own
// proof that a session both checked a row done AND gave itself the
// skip_test escape in the same edit. Empty (nil), never an error, when
// no session's diff did this — sessions with no BoxChecks at all (a
// single-session or non-plan-first task) are simply not measurable here,
// exactly as PlanCoverage/ContractDrift degrade on rows they cannot
// adjudicate.
func SelfSkipRows(sessions []SessionResult) []string {
	var rows []string
	for _, s := range sessions {
		for _, bc := range s.BoxChecks {
			block := trailingBraceBlock(bc.Text)
			if block == "" {
				continue
			}
			if parseSimpleAnnotation(block)["skip_test"] != "true" {
				continue
			}
			label := bc.RowID
			if label == "" {
				label = bc.Symbol
			}
			if label == "" {
				label = bc.Text
			}
			rows = append(rows, label)
		}
	}
	return rows
}

// SelfSkipDetected is SelfSkipRows' own bool face for a caller
// (SuccessFor) that only needs to know whether to fail the cell, not
// which rows.
func SelfSkipDetected(sessions []SessionResult) bool {
	return len(SelfSkipRows(sessions)) > 0
}
