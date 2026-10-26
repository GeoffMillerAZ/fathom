# Decision site skill forward trial

## Initial run, 12:42:45Z–12:43:00Z

Built the CLI outside the module and copied fixtures before mutation. Direct binary invocation; no provider/key access. Retrieval `prepare` on generic `case-1.input.json` exited 0, but retrieval study fixtures were absent. The guide's review-triage `check` and synthetic `study` exited 0; study had 3 cases, one unknown, one false favorable. Removing a capture exited 2 (`incomplete_coverage`).

Initial friction: retrieval recipe lacked its own study cohort. **Disposition: resolved by the updated fixtures** used below. `go test` status is already exact and computable from the process exit code; a Decision model must not certify success or trigger commands. It may classify parsed diagnostics for investigation only.

## Final retrieval run, 13:01:35Z–13:01:43Z

First useful result: 13:01:37Z, retrieval prepare. Built `/Volumes/m2ext/tmp/agentic-kit-build/decision-site-trial-final-2026-10-23/decision-lab` with `TMPDIR=/Volumes/m2ext/tmp/agentic-kit-build`; copied fixtures to the same external temp directory. No repo fixture mutation.

Direct CLI commands: `prepare --pack retrieval-support.pack.json --input retrieval-1.input.json`; `check` with that pack/input and `retrieval-1.capture.json`; `study` with `retrieval-support.manifest.json` and `retrieval-support.captures.json`. All exited 0. Pack digest: `sha256:96527aae7d109d94e817d5a5736e1dc2a85c1bb4afa33b0f8156c3ef11b32b31`. Check selected `supported`, confidence 0.75, no answer-cache hit. Study: synthetic, 7/7 cases, 5 answered, 2 unknown, 1 false favorable, 1 expected-favorable abstention. Labels are `frontier`-authored, not independently adjudicated; this is not quality evidence.

Removed one capture from a copied file and reran study: exit 2, `missing_cases`, missing `retrieval-7`. No authority or consumer adoption follows. Remaining confusion: none blocking the offline prepare/check/study path; a human-adjudicated observed cohort and baseline comparison remain necessary for adoption. `go test` success remains a process exit-code question, not a model-judgment site.
