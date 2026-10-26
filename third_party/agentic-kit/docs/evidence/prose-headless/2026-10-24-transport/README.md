# Two synthetic provider transport observations

2026-10-24, macOS arm64, Go1.26.3. [Receipt](receipt.json).

Both actual calls returned HTTP200 through the existing adapters: Anthropic
completed a fixed synthetic JSON canary with `end_turn`; OpenRouter/Jev selected
`investigate` for an independently seeded conflict between two dates. Native usage
fields were present in both responses. Together their reported usage prices to
**$0.000029**, conservatively rounding each token class to integer micro-USD.
This is a calculation from published price data, not a provider invoice.
Jev additionally reported its fractional cost in the retained response metadata.

This proves these two API/usage formats and observed outcomes only. It does not
prove document generation, semantic support, source truth, goal acceptance,
confidence calibration, decision quality across cases, cost savings, publication,
UI entry or production readiness. An `investigate` probability of 1 in this one
synthetic case is not calibrated certainty.

The private journal reserved and synchronized each attempt before transport,
retained exact wire hashes, and kept raw successful responses outside the repo.
No key, authentication header, private user source or raw session transcript is
included. Requests were public synthetic data. No retry or provider fallback ran.

The observed binary/source hashes and selected study digest are in the receipt.
The original uncommitted source snapshot and binary remain privately retained.
After these observations, the command's configurable model-family patterns were
replaced with explicit exact resolved model IDs. Focused offline tests cover that
refinement, including broad wildcard refusal and pricing against exact aliases.
No second live call is implied for the refined binary. Source/static and actual
provider observations remain separate evidence.

The normal pre-push validation and Fathom gates passed for `d9469e6`. Hosted
[CI](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40164559395),
[maintenance observations](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40164559408)
and [native observations](https://github.com/GeoffMillerAZ/agentic-kit/actions/runs/40164559398)
all refused startup: eight jobs, zero steps. Every failure annotation reports an
account payment or spending-limit restriction. Hosted source execution remains
unobserved; no retries or account-setting changes were made.

Interfaces/pricing: [Anthropic pricing](https://platform.claude.com/docs/en/about-claude/pricing),
[Jev model pricing](https://openrouter.ai/typesafe/jev-1.14),
[Jev typed decisions](https://openrouter.ai/docs/guides/community/jev).
These sources describe current documented interfaces/pricing, not the quality of
the unrun [HP document cohort](../../../designs/prose-headless-proof.md).
