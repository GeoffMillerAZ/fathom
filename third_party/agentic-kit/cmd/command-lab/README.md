# Command lab

Offline reference only. `audit` counts proposals from explicitly listed structured JSONL logs; `assess` simulates typed operation constraints. Neither command runs a proposed operation or grants permission. Synthetic fixtures are guide inputs, not empirical security evidence.

From the repository root:

```sh
go run ./cmd/command-lab audit --manifest cmd/command-lab/fixtures/manifest-direct.json --format json
go run ./cmd/command-lab audit --manifest cmd/command-lab/fixtures/manifest.json --format json
go run ./cmd/command-lab assess --scope cmd/command-lab/fixtures/scope.json --policy cmd/command-lab/fixtures/policy.json --request cmd/command-lab/fixtures/request.json --format json
```

`audit` exits 0 for complete coverage and 2 for partial or invalid input. `assess` exits 0 for advisory eligibility, 1 for deterministic blockers, 2 for invalid or incomplete input. All output includes `noActionAuthority: true`. Source IDs and paths are never printed. Family counts and `functions.exec` literal candidate counts are distinct. Every `functions.exec` wrapper is unsupported and makes coverage incomplete, even when it contains recognizable literals. Its candidate counts are frequency hints and do not prove execution or complete wrapper coverage. The direct fixture finishes complete; the fixture containing a Codex JS wrapper exits 2. Unsupported records make coverage incomplete.

A captured classifier response is optional. First run `assess` without `--capture` to get its `requestKey`. Create a capture with version `command-lab.capture/1`, that exact key, and either a fixed cause or a response matching the pinned provider and model. The response shape mirrors `decision.Response` (see `#PortResponse` in `schema.cue`). This is a captured port response, not a live provider call. A classifier answer remains advisory and cannot clear a deterministic blocker.

Resolved model references allow up to 256 ASCII characters with native `/`, `:`, `@`
and `+` separators. They are compared exactly, not normalized or resolved. Accepting
a reference does not establish model availability, local residency or calibration.

Control inputs use strict JSON, at most 1 MiB each. JSONL logs have a 4 MiB per source limit, 64 MiB total, 1 MiB per line and 50,000 records. The reader stops on overflow and keeps partial counts. Input paths must name regular files directly; symlinks and special files are rejected. Paths in a manifest are explicit input selectors, not discovery. CLI diagnostics use fixed codes and do not include raw input.
