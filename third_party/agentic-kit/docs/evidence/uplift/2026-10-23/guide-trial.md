# Offline practice-review guide trial

- Trial: 2026-10-23 09:59:07–09:59:32 UTC (25 seconds); first useful result at 09:59:13 UTC (6 seconds from start).
- Method: read only `examples/practice-review/README.md` and its named JSON fixtures. Built `./examples/practice-review` to `/Volumes/m2ext/tmp/agentic-kit-build/guide-trial-20261023/practice-review`; copied fixtures there before mutation. Invoked the binary directly with the documented `--at` and `--format json` arguments. Root repository inputs were left unchanged. No network, provider, hooks, or global configuration used.
- Normal fixture: process exit 0; status `pass`; two synthetic subjects, six prepared bindings, zero coverage gaps. Required criteria passed.
- Work-limit violation: changed `review-alpha` bounded-work `steps` from 2 to 4. Exit 1; status `fail`; trace reason `work exceeds declared positive limit`.
- Missing review data: removed that check. Exit 2; status `unknown`; coverage gaps named `missing criterion review-alpha/review.bounded-work` and `unknown assessment review-alpha`; trace outcome `unknown` / `no observation supplied`.
- Unknown field: added `max_steps` to the subject check. Exit 2; rejected with `invalid subjects: json: unknown field "max_steps"`.
- Guide friction: none observed. All listed behaviors were actionable and matched the CLI. This was an agent-assisted synthetic fixture trial only. It does not establish independent adoption, human usability, source authenticity, evidence truth, retention, effect authorization, Review Kit exchange, or production readiness.

## Local quality-gate configuration diagnosis

- 2026-10-23 10:07:37–10:07:58 UTC (21 seconds). Used root-built `/Volumes/m2ext/tmp/agentic-kit-uplift/quality-gate`, Go 1.26.3, module `agentic-kit` revision `1aca7869b0659a853e96f9788864e10a867a2d4b` (binary metadata marks source modified), and external Git 2.54.0. In a fresh external-temp Git repository, `init` exited 0, generated provider `none` policy; `validate` exited 0. Bootstrap local-file doctor exited 0, `scope: configuration-only`, `policy_source: local-untrusted`, state `pass`. After committing policy and schema, doctor against `HEAD` exited 0, state `pass`, policy commit `65e9ff47ef7e3f2a796b557a5a16ed05e367cff1`. Missing-policy doctor exited 2, state `unknown`, reason `policy-unavailable`.
- The reports explicitly say no candidate snapshot or enforcement verdict was checked. No hook was installed or invoked; no CI status, provider, cache, project-specific rules, or adoption was verified. The optional shadow site was advisory/unavailable. This establishes CLI configuration diagnosis only, not repository adoption or effectiveness.
