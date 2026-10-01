## Prompts

Fathom's four canned prompts (`investigate_incident`, `review_change`, `onboard_service`,
`plan_requirement`) were removed by the surface reckoning (SR-20, F-78): zero lifetime
calls in `calls.json`, and each one's workflow is a direct call away — `review_diff` for
review, `entrypoints`/`find_callers`/`staleness_report` for investigation and onboarding,
`entrypoints`/`find_callers` for planning. `prompts/list` and `prompts/get` stay wired
(the MCP prompts capability is kept) and answer an empty list / a not-found error —
never a protocol-level refusal.

---
