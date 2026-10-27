# Pulumi SDKs: cheap oracle-first triage

2026-10-26. Partial input to IAC-4 / GAP1-36 / issue #55; this note does not satisfy either completion row. Controller I-R5 authorizes triage only
while NP-Q5 is unanswered. Engine main `01ea58c6b`; frozen protocol `ca92a09bd`.
[Counts and receipts](data/pulumi-sdk-triage-2026-10-26/receipt.json),
[protocol and exact invocation](../../calibration/oracles/pulumi-sdk-triage/README.md).

## Triage outcome (TypeScript control)

Ordinary TypeScript tooling resolves the installed AWS, Azure Native and GCP SDK
symbols in one authored control. The current library importer keeps their external
targets unresolved. This observation does not isolate a Pulumi-specific defect from
the dependency-program target-admission gap. Keep IAC-14 deferred and the typed
schema layer deferred to G18 under I-R5. No language level, refusal, ceiling,
queue change or completed plan claim follows. Python and Go programs remain untried.

This is previously explored, privately selected authored code, not a sample of real
Pulumi repositories or a holdout. It exercises three resource constructors, not
stack deployment, dynamic providers or all SDK surface. Existing Pulumi YAML
readings are unchanged. IAC-4's full multi-language question remains open. Its shared completion path,
`docs/assessments/2026-10-26-pulumi-probe.md`, is intentionally absent, so both
IAC-4 and GAP1-36 fail their lexical completion contracts. This partial assessment
uses a distinct path and heading.

## Owner tools and selection

[Pulumi's TypeScript documentation](https://www.pulumi.com/docs/iac/languages-sdks/javascript/)
places SDK programs in TypeScript/Node.js. [scip-typescript](https://github.com/sourcegraph/scip-typescript)
indexes TypeScript and JavaScript. Sources checked 2026-10-26; the locally selected
versions are pinned, not a claim about latest releases.

| Installed input | Version | Fresh static result |
|---|---|---|
| Pulumi AWS | 7.0.0 | 4 SDK symbol occurrences |
| Pulumi Azure Native | 3.0.0 | 3 SDK symbol occurrences |
| Pulumi GCP | 9.0.0 | 5 SDK symbol occurrences |
| Pulumi core | 3.180.0 | Installed dependency; no separate grade |
| TypeScript | 5.9.3 | `--noEmit` exits 0 |
| scip-typescript | 0.4.0 | `index` exits 0; one document |

The committed privacy-filtered lock removes optional funding and deprecation
prose without changing versions, dependency edges, resolved URLs or integrities.
The six registry metadata records retain SHA-512 archive integrity; redundant
legacy SHA-1 fields are omitted. This makes dependency recovery possible from
an operator-provided npm mirror/cache without publishing people metadata.
The source, tsconfig, package/lock, compiler/indexer entrypoint hashes and package
version/integrity metadata were fixed before the fresh run. Full installed-tree
counts and hashes agree before/after. These establish local byte conservation;
installed SDK payloads were not authenticated against registry archive bytes.
`skipLibCheck: true` skips diagnostics inside declaration files. No Pulumi CLI,
program function, preview, update, provider plugin or cloud operation was executed.

## Candidate dispositions

| Candidate | Disposition | Evidence or constraint |
|---|---|---|
| TypeScript 5.9.3 `tsc` + scip-typescript 0.4.0 | Tried | Static compiler and indexer only; authored counts above |
| [Pulumi mock runtime](https://www.pulumi.com/docs/iac/guides/testing/unit/) | Out of scope; needs-owner-approval | Mocks can register resources offline, but execute the program; the accepted design forbids executing programs in this probe |
| [Pulumi preview on a local file backend](https://www.pulumi.com/docs/iac/concepts/state-and-backends/) | Out of scope; needs-owner-approval | A local backend does not remove program execution or provider/API behavior; preview is forbidden by the current brief |
| [scip-python](https://github.com/sourcegraph/scip-python) / [pyright](https://github.com/microsoft/pyright) | Untried | Static index/type/definition candidates; no Python SDK probe or candidate version pin yet |
| [scip-go](https://github.com/sourcegraph/scip-go) / [gopls](https://pkg.go.dev/golang.org/x/tools/gopls) | Untried | Static compiler/index/LSP candidates; core SDK cached, three cloud SDK modules absent in the scoped module cache; no candidate version pin yet |

NP-Q5 prioritization alone does not authorize mock-runtime or preview execution.
Those routes require separate controller authorization; none was attempted.

## Imported external edges

`NewTSExtractor("typescript")` extracts two nodes from the authored source.
Library `scip.ImportIndex` receives that source's fresh native index:

| SDK | Native symbol occurrences | Direct edges | Unresolved edges |
|---|---:|---:|---:|
| AWS | 4 | 0 | 4 |
| Azure Native | 3 | 0 | 3 |
| GCP | 5 | 0 | 5 |
| Total | 12 | 0 | 12 |

Occurrences include module, type and member symbols, not twelve calls. These are
library-import observations, not full ingest or a measured L4 denominator.
The source hash is pinned. Two fresh native runs in different scratch roots
produce byte-identical public importer JSON, including the normalized index hash. There is no
claim that all dependencies or compiler bindings are admitted into the graph. The
complete importer report also counts **7 unmapped definition occurrences** and
**3 skipped local-target occurrences** outside the retained SDK edges. The helper reports these counters together with all twelve SDK counts in the
fresh capture. These limits remain visible in
the receipt; no full-program completeness is claimed.

## Execution and remaining work

Fresh scratch, scrubbed environment/HOME, no credentials, OS network denial and
cache-write denial were enforced. A parent-owned live loopback listener was
reachable outside the sandbox and denied with EPERM inside. Every recorded native
launch reports actual nice >=10 and load <=9, with 120-second and 1 MiB caps.
Go helper build/import ran serially under the same scheduling limits with module
network access disabled and `-trimpath` enabled. Raw stdout/stderr byte lengths
and digests, raw SCIP digests, helper-binary digests and build/import-log digests
are omitted. Before hashing, the helper clones the parsed SCIP index, sets only
`Metadata.project_root` to `file:///subject`, and deterministically re-encodes
it. It imports the original index bytes. Both fresh roots produce the same
normalized digest and exact public importer JSON. Scheduling/load measurements
are reported separately as operational evidence and naturally vary.

Earlier unpublished attempts treated raw output/index digests as harmless
content hashes. That was incorrect: the bytes encoded the private scratch
root. This clean recut has no earlier triage commits or those digests in its
history. The installed-tree digest also refuses absolute symlink targets before
hashing; all 25 actual SDK symlinks have relative targets. The tree's relative
names/targets and file-content digest agree before/after both native runs.

Ten Python refusal/conservation/privacy tests and the race-enabled Go importer
test pass. The Python test launches real children printing cwd to both output
streams and, with clock/load fixed independently, requires identical public
launch records in different root lengths. The Go test requires identical
importer receipts for indices differing only in `project_root`, then changes a
document language and requires a different normalized digest. Each new leak
regression was first observed failing and then passing. The importer was compiled from main's
unchanged product sources; no product code, readings, queue or claimed boxes changed.
Python/Go toolchain probing, public subject selection, full ingest, independent
output perturbations, anchors and persistent-store costs remain unmeasured.
No complete oracle search or ceiling is claimed. After NP-Q5 authorizes lane work,
pin real subjects and test ordinary language/dependency support first. Reopen
IAC-14 only if a provider-specific gap remains after that comparison or SG-37 lands.
