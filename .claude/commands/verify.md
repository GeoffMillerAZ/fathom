---
description: Evidence-check a plan/checklist against code — which "done" items aren't actually backed by code?
argument-hint: "[plan file, default tmp/TRACKING.md] [service, default sextant]"
---

Run an **evidence-backed definition-of-done** check before trusting any "done" claim.

1. Resolve the plan file and service from `$ARGUMENTS` (default: plan = `tmp/TRACKING.md`, service = `sextant`).
2. Read the plan file's contents.
3. Call the sextant MCP tool **`verify_plan`** with:
   - `service`: the service
   - `plan_markdown`: the full contents of the plan file
   (The tool parses each `- [ ] / [x]` line; add `{symbol: Foo}` / `[US-042]` annotations to a line to give it a checkable contract.)
4. Report the **claimed-vs-verified delta**, focusing on the honest gaps:
   - items marked done (`[x]`) whose verdict is **not** `verified` — especially `claimed-but-absent` (the symbol isn't in the index), `present-but-untested` (no test reaches it), `contradicted`, or `likely-unconfirmed` (heuristic tier only);
   - items with `no-contract` (no symbol/spec to check — annotate them if they matter).
5. Be explicit that this is a **static** check: `test_reaches` means a test reaches the symbol, not that it passes; `runtime_pass` is always `unknown` until CI/coverage evidence is ingested.

Do not declare the work done if any `[x]` item fails verification — surface it instead.
