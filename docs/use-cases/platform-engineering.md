# Platform engineering: readiness evidence, inventories and investigations

Use fathom to gather code and configuration evidence for Well-Architected and
production-readiness reviews. An adopting platform team supplies its Standard and
decides what the evidence means. Fathom supplies no readiness or compliance verdict.
Every reading here means **declared at a commit, not deployed state**.

This guide implements the recipes in the [platform-engineering design](../design/platform-engineering.md#44-investigations-and-reviews-plt-10-plt-11).
The commands below were run against a Kubernetes-containing fixture in an isolated
store. Their results establish the narrow capabilities described here, not a
framework assessment or a fleet-wide reading.

## What can be answered today

| Question within a review | Verb demonstrated | Result and boundary |
|---|---|---|
| Where is the declared workload? | `find_symbol` | Names have shape `Kind/namespace/name`, or `Kind/name` when the manifest omits the namespace. The exact `Deployment/default/polyglot-app` and a substring match on bare `polyglot-app` both resolve in `deploy.yaml`. |
| What replica count is written in this manifest? | `find_symbol`, `search` | `Deployment/default/polyglot-app#replicas` is a `K8sFact` with `signature: "1"`; the search hit at `deploy.yaml:7` names that symbol. Facts are reachable as symbols, without a fact sheet with anchors and coverage (WAE-5), or a fact receipt. This is declared configuration, not available capacity. |
| What context surrounds the workload or a function? | `symbol_context` | Workload definition, snippet and index commit. For `greet_all`, a callee is returned with a file-resolved edge and `heuristic: 1`; the workload has no fact sheet. Preserve the resolution disclosure. |
| What did the manifest say at the reviewed revision? | `read_source` with `ref` | `HEAD` resolves to a commit and returns the manifest with a content digest. Substitute the actual reviewed or deployed SHA when known. This does not inspect the cluster. |
| What governs an edit to this manifest? | `obligations` | The unconfigured fixture reports no governing rule and unknown layer coverage. It demonstrates an honest empty answer, not Standard delivery. |
| Where should an investigation start from a frame? | `localize` | A frame at `app.py:10` resolves to `greet_all` and expands to `Greeter`. These are investigation candidates, never a cause. |
| What indexed code is touched by a changed file? | `blast_radius` | `app.py` returns four changed symbols and no affected tests. No cross-service impact or test adequacy was established. |
| What evidence and unchecked dimensions accompany a change? | `review_diff` | For `app.py`, three of eight dimensions are checked, four symbols have a test gap, and missing inputs are named. Its low change-risk band is not a readiness verdict. |

For a production-readiness review, these are inputs to questions about declared
configuration, dependencies, instrumentation or change safety. Staffing, drills,
actual capacity, runtime health and deployed configuration need other evidence or
human attestation. A matching string alone does not satisfy a requirement.

## Who reads the evidence

| Consumer | Use now | Planned interface and acceptance rule |
|---|---|---|
| facts lane | Retain cited source and context responses alongside the requirement the operator names. The review kit judges sufficiency. | Fact and join receipts are unbuilt. When available, the lane accepts a receipt only when its index commit matches the reviewed revision and its basis is `exact`, then derives its own pass or fail. Lens mappings stay in the review kit. |
| readiness inventory | Record the subject, source, revision and the limits of each manual reading. Keep subjects separate. | A per-subject, per-kind fact inventory is planned. Its `not-found` is a lower bound over a read set and maps to `unknown`, never `absent`. One workload must not make a whole service read clean. Readiness remains derived from the Standard, evidence snapshot and time. |
| change-risk tierer | Read the existing change-review report or neutral check receipt as cited input, retaining its missing-input disclosures. | The concern-delta file is unbuilt. The planned reader uses only `exact` deltas as named reasons and fails open. The tierer owns the tier. Deltas will be information, not conservation losses or a second gate. |

The appendix demonstrates `fathom review --since` in report mode and its
`--receipt --format check` output with `conclusion: neutral`. It also demonstrates
`fathom conserve`'s plane report and `fathom certify gaps`'s explicit
"nothing declared" result. These outputs must travel with their coverage notes.
In `review_diff`, the
`certificates_affected` factor says "every touched certificate is live" and
`rows_proven` says "every claimed row is proven" even when no certificates or
claimed plan rows are declared. These are vacuous passes, not positive coverage
or proof that any certificate or plan row was checked.

## Policy recipe: consume receipt JSON unchanged

Export an available receipt and pass the file directly to the policy tool. The
demonstrated export is a `symbol_location` receipt for `greet_all`: its `facts`
array contains a definition and git anchors. It contains no Kubernetes resource
values. The exported receipt carries `service: "@"`, not the ingested service
name. Policy authors must inspect that field rather than assume it identifies
`platform-fixture`. Do not treat a definition receipt as a workload fact inventory.

```sh
"$BIN" export receipt -service platform-fixture -store "$STORE" \
  -symbol greet_all -kind symbol_location -o "$LOGS/symbol-receipt.json"

# Operator-authored policies inspect this receipt's actual JSON fields.
conftest test -p policy/rego "$LOGS/symbol-receipt.json"
kyverno-json scan --policy policy/receipt.yaml --payload "$LOGS/symbol-receipt.json"
```

No renderer or intermediate policy format is needed. Keep requirement mappings,
resource rules and decisions in the policy tool. Check the receipt kind and commit
before interpreting its contents, and retain unknown or withheld evidence instead
of converting it to a pass. When fact receipts ship, write policies against their
actual schema rather than assuming their planned shape already exists.

The external invocations follow the [Conftest documentation](https://www.conftest.dev/options/)
and [kyverno-json quick start](https://kyverno.github.io/kyverno-json/main/quick-start/).
Conftest 0.60.0 was run with the three-rule policy in the appendix over the
unchanged receipt: **3 tests, 2 passed, 1 expected warning, 0 failures**, exit 0.
The warning discloses `service: "@"`. This measures JSON consumption and those
rules only. kyverno-json was **not run** and remains an integration recipe.

## Architecture-review packet recipe

Assemble files explicitly. There is no demonstrated single packet-directory verb.

1. Use a clean checkout of the reviewed commit, with no tracked or untracked
   changes, and index that revision. Record `git status --porcelain` and
   `git rev-parse HEAD` before collecting the packet. Include the ADR being
   reviewed, the operator's requirement identifiers and the comparison base.
2. Save `fathom review --since BASE --no-persist --json` and
   `fathom review --since BASE --no-persist --receipt --format check`. Keep the full
   report beside the neutral receipt, so a band cannot hide unchecked dimensions.
3. Save `fathom conserve --since BASE --json` and
   `fathom certify gaps --json`. The demonstrated gaps response has no manifests
   and no targets. It provides no certificate coverage assurance. `conserve`
   reads the working tree and reports `"head": "worktree"`, even on the clean
   checkout. Keep the full `residue`, including `residue.unanalyzed`; in this
   demonstration every changed file is unanalyzed.
4. Save `obligations` responses for touched manifest and IaC paths. Include cited
   source responses and any available exported symbol receipts. Preserve the
   commit, anchors, unresolved inputs and verification results for each claim.
5. Let the reviewer or a review kit decide whether those inputs answer the
   requirement. No readiness, compliance or architecture approval follows from
   the packet itself.

The appendix runs this CLI sequence against the enclosing repository's `HEAD~1`
range. That range does not change the indexed fixture. It proves the output shapes
and disclosures only, not detection of a platform regression. For a real packet,
index the complete subject repository at the reviewed revision and choose its
actual base.

## Investigation packet recipe

1. Record the symptom, deployed SHA and previous SHA. Start with `localize` on an
   alert or frame, then read the candidate's `symbol_context`. The demonstrated
   `localize` expansion calls `greet_all` to `Greeter` "typed", while
   `symbol_context` reports that same edge as `file-resolved` with `heuristic: 1`.
   Carry `symbol_context`'s resolution in the packet. For a workload, retain its
   definition, snippet and any `K8sFact` symbols. The fact sheet is unbuilt.
2. Read the relevant text with `read_source ref=<deployed>`. The demonstrated
   `HEAD` read is not evidence that `HEAD` was deployed.
3. Run `blast_radius` on changed files and `fathom review --since <previous>` at the
   deployed revision in a clean checkout, as required by the review packet above.
   For a known good-to-bad range, `fathom culprit GOOD..BAD`
   supplies static candidates from the current index. The demonstrated range has
   no failing tests and incomplete reach evidence. It establishes no regression
   boundary or cause and executes no bisect or mitigation.
4. Keep an append-only journal in the vendored investigation-journal shape
   (`third_party/agentic-kit/foundations/investigation/investigation-journal/`):
   each entry has `Step`, `Note` and an RFC 3339 `At`, with optional `Author` and
   evidence `SpanRefs`. Put the question, hypothesis, next check and result in
   the note. Label each hypothesis **not cause** until a human decides. Attach
   saved responses or receipts to every factual statement, distinguishing source
   citations from independently re-derived evidence.
5. List blind spots: files outside the read set, index/revision skew, heuristic
   links, missing telemetry, absent certificates and unavailable attestations.

The packet is the journal plus its evidence files. It does not require a new
investigation service or a new result format. The packaged skills
`fathom-investigate` (PLT-10) and `fathom-platform-review` (PLT-11) are unbuilt;
these are manual recipes.

## The fact at the edit

An adopting platform team's Standard is its own content, supplied as
configuration. Follow the [platform-module and guarded-edit recipe](platform-modules.md)
to deliver its code-facing requirements at the edit. **Platform modules and
guarded edits are both experimental, with no external consumer yet.** This
recipe adds no runtime code or workload fact sheet.

First, compile the Standard once as a platform module and have the operator mount
its packed store read-only on the daemon, as the linked guide describes. Mounting
makes it available. Each adopting repository must explicitly bind it in its
committed `.fathom/modules.cue`, for example:

```cue
bindings: [{
    module:    "platform-standards"
    scopes:    ["deploy.yaml", "deploy/**", "infra/**", "*.tf", "**/*.tf"]
    max_rules: 2
    min_tier:  "doc-claimed"
}]
```

The intended delivery is module-tagged requirements alongside repository rules
through `obligations`. Binding scopes only restrict the module's own scopes, and
repository rules keep priority. On a daemon started without any `--module`,
`obligations` does not run the binding lookup and is silent about missing mounts.
Run `fathom doctor` after binding: `binding_modules_mounted` is the check that
fails loudly for an unmounted binding when a mounted-module list is available.
On this build, `list_modules` is unavailable and the scratch check is **skipped**.
A skipped check is not confirmation of delivery. This fixture's empty obligations
response does not prove delivery either.

Second, guard the manifest and IaC paths in committed `.fathom/guard.cue` so the
configured pre-edit hook can preview the requirement before a change:

```cue
guards: [{
    scopes:          ["deploy.yaml", "deploy/**", "infra/**", "*.tf", "**/*.tf"]
    note:            "Read the Standard requirement before changing manifests or IaC."
    max_rules:       4
    include_modules: true
}]
```

These are instance fields: write `scopes:`, not `scopes?:`. Optional fields are
omitted when unused. `include_modules: true` requests module rules in the
pre-edit preview. This inject-only guard injects the note plus up to four
obligations. It is advisory and does not enforce a resource policy or certify
compliance. The linked guide covers hook setup,
budgets, freshness, pre/post deduplication and binding diagnostics.

Finally, use `fathom research report` after a representative edit window. Inspect
guard pre/post seen and served counts, dedup suppression and module serving
separately. Module counts include only edit-hook `module-served` beats, not
direct `obligations` calls. The hooks log has no repository or service dimension,
so these counts cannot establish use by a particular repository. The isolated
report below uses an empty hooks log: it shows the report's shape, **not a
reading**. Both channels say **not measurable** because no pre-edit invocation
or module-tagged rule was observed. That is not a measured zero, proof of
adoption or an external-consumer reading. A binding alone does not establish use.

## Limits

- **L1. Concern facts:** Kubernetes `K8sFact` rows are served as symbols named
  `<workload>#<key>`, with the value in `signature`. There is no fact sheet with
  anchors and coverage (WAE-5), or fact receipt. A missing `#readinessProbe`
  symbol means **unknown**, never absent. Documents that fail YAML decoding,
  including Helm templates, are silently skipped by the Kubernetes extractor;
  zero read failures does not establish complete extraction. The workload
  context has a snippet and an empty `observed_by` list. No monitor selection,
  retry-chain or code-to-config join was demonstrated.
- **L2. Receipts and deltas:** `export receipt --kind fact` is rejected with
  "want symbol_location or callers". Fact and join receipts, the multi-kind
  inventory, its basis/coverage classes and the concern-delta file are unbuilt.
  Their descriptions above are consumer contracts, not working export commands.
- **L3. Verification:** exporting the Python definition receipt succeeds.
  `receipt verify` confirms its git anchor, then exits 2 because the toolchain
  verifier expects Go source. Full re-derivation was not demonstrated for this
  receipt, or for any manifest fact.
- **L4. State and verdicts:** all evidence is declared state. There are no
  Well-Architected, production-readiness or compliance verdicts. Existing review
  risk bands and conservation plane verdicts do not certify those outcomes.
- **L5. Coverage:** no rules, coverage profile or certifications are configured in
  the fixture. An unchecked dimension, an empty list or no governing rule is not a
  clean bill of health. The tested CLI range is outside the fixture's changes.
- **L6. Scope:** one small fixture is not a fleet reading. Cross-repository
  propagation, runtime controls and human attestations were not demonstrated.
  Policy evaluation was limited to the three Conftest receipt rules. A missing
  hit never proves global absence.
- **L7. Edit delivery:** module-tagged obligations and pre-edit Standard delivery
  were not exercised. Both experimental channels are unmeasured in the isolated
  report. The configuration above is a recipe, not a claim that either fired.

## Continue with the focused guides

- [Incident investigation](incident-investigation.md) for symptom-led triage.
- [Change review](change-review.md) for diff review and its disclosure contract.
- [Cross-repository blast radius](cross-repo-blast-radius.md) for fleet impact.
- [Platform modules](platform-modules.md) for experimental Standard delivery and
  guarded edits.

## Commands behind this guide

Run from the repository root with `JOB` set to a writable, isolated job directory.
All stores, builds and logs below live under its `workspace/`. Never substitute a
shared daemon store. The fixture search returned the path used here. The build
and ingest completed successfully: nine files extracted, 31 symbols, no read
failures. Results were read from the saved JSON, including their error payloads.

```sh
mkdir -p "$JOB/workspace/bin" "$JOB/workspace/db" "$JOB/workspace/logs"
# Collect from a clean checkout of the reviewed commit.
test -z "$(git status --porcelain)"
git rev-parse HEAD > "$JOB/workspace/logs/reviewed-commit.txt"
go build -o "$JOB/workspace/bin/fathom" ./cmd/fathom
BIN="$JOB/workspace/bin/fathom"
STORE="$JOB/workspace/db/platform.sqlite"
LOGS="$JOB/workspace/logs"
FIXTURE=internal/connector/testdata/fauxtown/polyglot-service
rg -l 'kind: Deployment' internal/connector/testdata testdata | head
"$BIN" ingest -path "$FIXTURE" -service platform-fixture -store "$STORE" -json > "$LOGS/ingest.json"

cat > "$LOGS/requests.jsonl" <<'JSONL'
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"find_symbol","arguments":{"service":"platform-fixture","name":"Deployment/default/polyglot-app"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"symbol_context","arguments":{"service":"platform-fixture","symbol":"platform-fixture::deploy.Deployment/default/polyglot-app"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"symbol_context","arguments":{"service":"platform-fixture","symbol":"greet_all"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search","arguments":{"service":"platform-fixture","pattern":"replicas|kind:","path":"deploy.yaml"}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"read_source","arguments":{"service":"platform-fixture","file":"deploy.yaml","ref":"HEAD","start_line":1,"end_line":14}}}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"obligations","arguments":{"service":"platform-fixture","file":"deploy.yaml"}}}
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"localize","arguments":{"service":"platform-fixture","stack_trace":"File \"app.py\", line 10, in greet_all"}}}
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"blast_radius","arguments":{"service":"platform-fixture","changed_files":["app.py"]}}}
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"review_diff","arguments":{"service":"platform-fixture","changed_files":["app.py"]}}}
{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"find_symbol","arguments":{"service":"platform-fixture","name":"polyglot-app"}}}
{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"find_symbol","arguments":{"service":"platform-fixture","name":"Deployment/default/polyglot-app#readinessProbe"}}}
JSONL
"$BIN" serve-mcp -store "$STORE" -auto-refresh=false < "$LOGS/requests.jsonl" > "$LOGS/responses.jsonl"

"$BIN" review -service platform-fixture -store "$STORE" -since HEAD~1 \
  -no-persist -json -call-log "$LOGS/review-calls.jsonl" > "$LOGS/review.json"
"$BIN" review -service platform-fixture -store "$STORE" -since HEAD~1 \
  -no-persist -receipt -format check -call-log "$LOGS/review-calls.jsonl" > "$LOGS/review-check.json"
"$BIN" conserve -service platform-fixture -store "$STORE" -since HEAD~1 -json > "$LOGS/conserve.json"
"$BIN" certify gaps -service platform-fixture -store "$STORE" -json > "$LOGS/certify-gaps.json"
"$BIN" culprit HEAD~1..HEAD -service platform-fixture -store "$STORE" > "$LOGS/culprit.txt"
"$BIN" export receipt -service platform-fixture -store "$STORE" \
  -symbol greet_all -kind symbol_location -o "$LOGS/symbol-receipt.json"

# Negative checks, run separately: export exits 1; verification exits 2.
"$BIN" export receipt -service platform-fixture -store "$STORE" \
  -symbol Deployment/default/polyglot-app -kind fact
"$BIN" receipt verify --repo . "$LOGS/symbol-receipt.json"
```

The added `polyglot-app` lookup returns both the workload and its replica fact
(`signature: "1"`). The readiness-probe query returns no symbols; that is unknown
coverage. `localize` calls the expanded edge "typed" while `symbol_context`
records `file-resolved` and `heuristic: 1`. The `review_diff` response names the
vacuous `certificates_affected` and `rows_proven` factors described above.
`conserve.json` reports `head: "worktree"` and retains all changed files in
`residue.unanalyzed`.

For the measured policy run, install the pinned Conftest release into the job
only, then create this three-rule policy. No receipt fields are rewritten.
The relative `policy/rego` path in the recipe is under `$JOB/workspace`.

```sh
curl -fL -o "$JOB/workspace/conftest.tar.gz" \
  https://github.com/open-policy-agent/conftest/releases/download/v0.60.0/conftest_0.60.0_Darwin_arm64.tar.gz
tar -xzf "$JOB/workspace/conftest.tar.gz" -C "$JOB/workspace/bin" conftest
"$JOB/workspace/bin/conftest" --version
mkdir -p "$JOB/workspace/policy/rego"
cat > "$JOB/workspace/policy/rego/receipt.rego" <<'REGO'
package main
import rego.v1

deny contains "expected a symbol_location receipt" if {
    input.kind != "symbol_location"
}

deny contains "index commit is required" if {
    object.get(input, "index_commit", "") == ""
}

warn contains "service is @; do not infer the ingested service name" if {
    input.service == "@"
}
REGO
shasum -a 256 "$LOGS/symbol-receipt.json" > "$LOGS/receipt-before.sha256"
(
  cd "$JOB/workspace"
  PATH="$JOB/workspace/bin:$PATH"
  conftest test -p policy/rego "$LOGS/symbol-receipt.json" > "$LOGS/conftest.txt"
)
shasum -a 256 "$LOGS/symbol-receipt.json" > "$LOGS/receipt-after.sha256"
diff -u "$LOGS/receipt-before.sha256" "$LOGS/receipt-after.sha256"
```

Conftest reported version 0.60.0, OPA 1.4.2. The exact test invocation returned
**3 tests, 2 passed, 1 warning, 0 failures, 0 exceptions**, exit 0. The expected
warning was `service is @; do not infer the ingested service name`. The checksum
comparison returned exit 0. kyverno-json was **not run**.

A second, job-owned fixture copies the unchanged manifest, appends a document
that fails Kubernetes YAML decoding, and binds a module that is not mounted.
It uses the same scratch store, with a separate service. No repository binding
or shared daemon is changed.

```sh
export FATHOM_HOME="$JOB/workspace/daemon-home"
DIAG="$JOB/workspace/binding-fixture"
mkdir -p "$DIAG/.fathom"
cp "$FIXTURE/deploy.yaml" "$DIAG/deploy.yaml"
cat >> "$DIAG/deploy.yaml" <<'YAML'
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.name }}
YAML
cat > "$DIAG/.fathom/modules.cue" <<'CUE'
bindings: [{module: "platform-standards"}]
CUE
"$BIN" ingest -path "$DIAG" -service binding-fixture \
  -store "$STORE" -json > "$LOGS/binding-ingest.json"
cat > "$LOGS/binding-requests.jsonl" <<'JSONL'
{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"obligations","arguments":{"service":"binding-fixture","file":"deploy.yaml"}}}
{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"find_symbol","arguments":{"service":"binding-fixture","name":"polyglot-app"}}}
JSONL
"$BIN" serve-mcp -store "$STORE" -auto-refresh=false \
  < "$LOGS/binding-requests.jsonl" > "$LOGS/binding-responses.jsonl"
# Ask the OS for a port and verify the spawned daemon's PID and store.
export BIN STORE LOGS DIAG
python3 - <<'PYTHON'
import json, os, pathlib, subprocess, time, urllib.request
logs = pathlib.Path(os.environ["LOGS"])
with (logs / "doctor-server.log").open("w") as log:
    server = subprocess.Popen([
        os.environ["BIN"], "serve-http", "-store", os.environ["STORE"],
        "-addr", "127.0.0.1:0", "-auto-refresh=false",
        "-call-log", str(logs / "doctor-calls.jsonl"),
    ], stdout=log, stderr=subprocess.STDOUT)
    try:
        for attempt in range(50):
            assert server.poll() is None, "scratch daemon exited"
            records = pathlib.Path(os.environ["FATHOM_HOME"]).glob("daemon/*/record.json")
            record = next((r for p in records if
                           (r := json.loads(p.read_text()))["pid"] == server.pid), None)
            if record:
                break
            time.sleep(0.1)
        assert record, "scratch daemon did not publish its address"
        assert pathlib.Path(record["store"]).resolve() == pathlib.Path(os.environ["STORE"]).resolve()
        url = "http://" + record["addr"]
        for attempt in range(50):
            assert server.poll() is None, "scratch daemon exited"
            try:
                with urllib.request.urlopen(url + "/healthz", timeout=1) as response:
                    assert response.status == 200
                break
            except OSError:
                time.sleep(0.1)
        else:
            raise RuntimeError("scratch daemon did not become ready")
        (logs / "doctor-server-record.json").write_text(json.dumps(record))
        with (logs / "binding-doctor.json").open("w") as out:
            result = subprocess.run([
                os.environ["BIN"], "doctor", "-path", os.environ["DIAG"],
                "-service", "binding-fixture", "-url", url, "-json",
            ], stdout=out)
        assert result.returncode == 1, result.returncode
    finally:
        server.terminate()
        server.wait(timeout=10)
PYTHON
```

The valid workload and `#replicas` symbol still resolve. Kubernetes extraction
silently skips the undecodable document; the ingest reports no read failures.
The bound fixture's `obligations` response has no missing-mount note.
`binding_modules_mounted` in `binding-doctor.json` is **skipped**, naming the
unavailable `list_modules`; it does not establish mount health. The isolated
`doctor` command exits 1 because other wiring checks fail on this unwired fixture.

The edit-use check was run separately, with an empty job-owned hooks log so the
absence of observations is explicit: this shows the report's shape, not a
reading. Module serving counts only edit-hook `module-served` beats, excludes
direct `obligations` calls, and has no repository or service dimension in the
hooks log. This run prints the report without writing into the fixture or changing a shared store.

```sh
: > "$LOGS/empty-hooks.jsonl"
"$BIN" research report -root "$FIXTURE" -service platform-fixture \
  -call-log "$LOGS/review-calls.jsonl" -hooks-log "$LOGS/empty-hooks.jsonl" \
  -bench-dir "$JOB/workspace/empty-bench" -no-write -json > "$LOGS/research-report.json"
```

The stdio server above opens only the scratch store. On a shared HTTP daemon,
`localize` is listed on the extended and full surfaces. The seven other distinct
MCP tools used here are on the default agent surface. Off-surface calls still
execute and are flagged. This appendix uses stdio so every request and store is
explicit.

## Maintained

This doc covers: `find_symbol`, `symbol_context`, `search`, `read_source`,
`obligations`, `localize`, `blast_radius`, `review_diff`, `fathom ingest`,
`fathom serve-mcp`, `fathom serve-http`, `fathom review`, `fathom conserve`, `fathom certify gaps`,
`fathom culprit`, `fathom export receipt`, `fathom receipt verify`,
`fathom research report` and `fathom doctor`, plus the `.fathom/modules.cue`
binding and
`.fathom/guard.cue` pre-edit recipe. Update it when these change.
