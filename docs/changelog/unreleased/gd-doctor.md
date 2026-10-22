### Changed

- Doctor recognizes project hook and MCP wrappers without executing them and distinguishes configured wiring from firing evidence. Definite missing hooks remain FAIL even alongside uncertain wrappers, unrelated tools or unreadable settings scopes.
- Doctor exit status now distinguishes timeout and unknown rows, which do not count as FAIL. Only unresolved project wrapper wiring changes exit 1 to exit 0: unreadable, oversized, dynamic or deeper-than-two-level project scripts; unresolved Task entries or dependencies; unresolved lefthook entries, scripts or jobs; and MCP stdio wrappers with the same limits. This applies only when no definite failure remains. A daemon reachability timeout remains FAIL.
- `--json` adds the `unknown` and `timeout` status values on individual rows; both count under `summary.skipped`. `--brief` lists them with separate counts. Request cancellation preserves completed verdicts and the existing provider ping budgets.
- Foreign hook commands count as missing fathom hooks. Missing project scripts fail with their path and an init remedy. Setup brief descriptions include FAIL, TIMEOUT and UNKNOWN rows within eight lines.
