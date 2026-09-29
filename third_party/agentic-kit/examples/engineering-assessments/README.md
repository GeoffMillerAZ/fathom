# Two domain collectors, one assessment contract

Run from the repository root:

```sh
go run ./examples/engineering-assessments
go test -race ./examples/engineering-assessments
```

The JSON includes complete profiles, observations, rule traces and content-addressed
artifacts. No provider, secret, external effect, service or evidence database is needed.

- The retained resume_as_code receipt goes through the real benchmark summarizer.
  Missing cost produces required `unknown` and no optimization score.
- Scripted prompt classifications run through `PromptScreener.Run`. A refused prompt
  cannot reach the protected continuation; a benign request executes a counter tool.
- A separate actual harness strict-effect check denies a destructive-marked counter
  tool even after a clear prompt classification. Disabling only that effect control
  makes the required assessment fail while the benign criterion still passes.

Every handler only increments an integer. Even the negative control is safe to run.
The fixed clock, scripted classifiers and retained synthetic consumer data establish
wiring and decision semantics, **not** live classification accuracy, accepted model
quality, actual latency, billed cost or deployment authorization.

The assessment subject is a receipt of observed facts, not a fully fingerprinted
production solution. Criterion definitions bind this collector's compiled source;
library versions and deployment dependencies need the fuller workload manifest when
making a solution-selection claim. Artifact hashes establish byte identity, not truth.

To adopt: supply a domain collector, select required/scored criteria, retain its input
and evidence, then call `fitnessfunctionregistry.AssessProfile` with your evaluation
time. Use `guardrails-pipeline`'s [prompt-screening guide](../../kits/guardrails-pipeline/README.md)
for real input handling. [Design and research](../../docs/designs/engineering-assessments.md).
