# Seed labs

These 16 cards transcribe design section 4.3. Feature IDs are cross-references,
not the shipped-surface inventory. Readouts postdate the scorecard, so their
feature list is empty until FM-1 supplies an ID.

`plugin-minus-<lab slug>` names the plugin with only that card's feature
removed, with all other features retained and `--ablation none` in both arms.
The skills card requires `plugin-minus-skill:{skill}`, a singular template
instantiated once per selected skill, removing only that skill. The decision
card requires `plugin-minus-decide:{site}`, using provider `none` only at the
selected site. Broad removal of all skills or decision sites is refused.
The stanza arm removes only the stanza block and keeps the wired hooks.
Multi-surface cards cover one feature delivered through those surfaces.

The schema requires a positive minimum effect: the initial cards use one call,
token, byte or broken caller for decreasing counts and 0.01 for increasing
outcome proportions. These are initial arm specifications, not measured effects
or extra kill criteria. A future run must preregister the metric and its units.

Owner ruling L-R1 supplies the same K4 and K1 wording in every card. Each points
to `docs/bench/readings/labs/<lab-id>.md`. Those readings do not exist yet.
Deterministic, always-loaded and configured-site rows use `n/a` with a reason.
Quality readings and stage owners remain in their existing programs.

| Lab | Features | Surfaces | Stages owned elsewhere |
|---|---|---|---|
| [lab-find-symbol](lab-find-symbol.yaml) | F-01 | `mcp:find_symbol` |  |
| [lab-search](lab-search.yaml) | F-09 | `mcp:search` |  |
| [lab-symbol-context](lab-symbol-context.yaml) | F-03 | `mcp:symbol_context` | RT-M14 |
| [lab-read-source](lab-read-source.yaml) | F-08 | `mcp:read_source` |  |
| [lab-blast-radius](lab-blast-radius.yaml) | F-12 | `mcp:blast_radius` | agent-level H2: bench-prep |
| [lab-review-diff](lab-review-diff.yaml) | F-34 | `mcp:review_diff` |  |
| [lab-verify-plan](lab-verify-plan.yaml) | F-36, F-37, F-39 | `mcp:verify_plan`, `hook:stop-hook` | agent-level H1: bench-prep |
| [lab-obligations](lab-obligations.yaml) | F-26, F-27, F-64, F-65 | `mcp:obligations`, `hook:edit-hook` | AD-20, AD-21, AD-M1 to AD-M4 |
| [lab-guards](lab-guards.yaml) | F-29 | `hook:edit-hook --pre` | RS-M1 to RS-M4 |
| [lab-prompt-hook](lab-prompt-hook.yaml) | F-28 | `hook:prompt-hook` | ADR-076 |
| [lab-session-brief](lab-session-brief.yaml) | F-48 | `hook:session-hook` |  |
| [lab-subagent-brief](lab-subagent-brief.yaml) | F-48 | `hook:session-hook --event subagent-start` | AD-M2 |
| [lab-readouts](lab-readouts.yaml) | (postdates the scorecard) | `hook:readout-hook --event post-tool`, `hook:readout-hook --event tool-failure` | RO-M1, RO-M4 to RO-M10 |
| [lab-stanza](lab-stanza.yaml) | F-58, F-86 | `stanza:agents-md` | AG-OP1 |
| [lab-skills](lab-skills.yaml) | F-60 | `skill:fathom-plan`, `skill:fathom-verify`, `skill:fathom-onboard`, `skill:fathom-feedback`, `skill:fathom-doctor`, `skill:fathom-impact`, `skill:fathom-rewire`, `skill:fathom-activate`, `skill:fathom-custodian`, `skill:fathom-findings`, `skill:fathom-offload` |  |
| [lab-decision-port](lab-decision-port.yaml) | F-69, F-70, F-71 | `decide:route`, `decide:compile_prefilter`, `decide:obligations_rerank`, `decide:certify_suggest`, `decide:assess`, `decide:preflight`, `decide:readout_mode`, `decide:compact_advice` | DP, EXP, JF |
