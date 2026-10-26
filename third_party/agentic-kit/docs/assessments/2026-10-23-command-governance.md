# Command safety and coding-harness governance assessment

Baseline: `7f92d50d8a692f05848fd4eeee471a2dc1f9f7ea`, 2026-10-23.
Decision [0035](../intent/decisions/0035-command-safety-starts-with-offline-qualification.md),
[specification](../designs/command-governance.md). This assessment separates source
research, exploratory sampling, executable qualification and future enforcement.

## Pages and source coverage

Located the **Daily AI Release Brief**, including the October 23, 2026 comparison
and private-local-model addendum, through its Scheduled task in the user's browser.
The Pages connector's accessible listings/search did not expose that scheduled run.
The related September 29 Developer Tools Page was not used as today's source.
No Page, chat or schedule was edited. Private hardware/profile details and the brief
text are not copied here. Public tool hypotheses were checked against primary sources
below. The brief's price/latency/general-accuracy rankings are not security calibration
or measurements on this project. Finding a source is separate from validating its claims.

Brief-derived choices: deterministic scanner coverage first; native local typed
decisions as an optional profile; specialized command-risk triage as a second signal;
strictly local-required workloads never fall back to a hosted provider. Reject automatic
model installation, unverified model tags, raw secret-scanner logging, universal
"safe command" labels and hardware sizing promises without a measured workload.

## Current mechanisms and gaps

| ID | Inspected source | Consequence |
|---|---|---|
| CS-01 | `cmd/agent-hooks/doc.go`, decision 0022 | Economic guidance deliberately fails open on faults and releases repeated spawn denials. Preserve it; safety needs a different controlled boundary. |
| CS-02 | `cmd/quality-gate`, decisions 0030/0031 | Immutable Git objects, explicit trusted policy and required coverage are useful patterns. The command is not a general Bash authorizer. |
| CS-03 | `foundations/agent/bounded-exec/types.go` | Time/output/env bounds do not remove host filesystem/network authority. PATH lookup and escaped process groups remain relevant limits. |
| CS-04 | `foundations/security/pre-llm-dlp-gate` | Reuse best-effort credential-shape detection carefully; `Finding.Excerpt` contains original data and must never reach an audit report or model projection. |
| CS-05 | Existing decision port and decision-lab | None/replay, exact identities and independent studies are available. Structured output and confidence alone cannot authorize execution. No native Ollama adapter was established. |
| CS-06 | Catalog Kit and practice-review | Identity, explicit tailoring and independent expected scope can support review criteria; they do not confer organizational authority. |
| CS-07 | Platform Policy/Review Kit, revision `821c509035a8bbd98907c83b54372bec70116425` | Policy scope defines non-lowerable floors and bounded exceptions; Review R6/R7/R20/R21 separate floors, external authority, invocation and coverage. Reuse concepts, not a duplicate engine or a claim that all mechanisms are installed. |
| CS-08 | Fathom O11 | Raw/incomplete obligations and known drift are evidence gaps. Optional enrichment cannot substitute for policy, runtime enforcement or complete audit coverage. |

## Exploratory command sample, not a qualified audit

A read-only sampling agent selected 22 Codex body files with agentic-kit metadata,
66,967,853 bytes, with session timestamps 2026-10-21T07:50:13Z through
2026-10-23T17:19:26Z. It found 1,501 wrapper tool-call records and 521 other tool calls.
Extracted command literals are **proposed input**, not correlated process executions.

| Closed known family | Exploratory literal occurrences |
|---|---:|
| File reads/search: cat, sed, tail, nl, rg | 1,003 |
| Git, all observed subcommands | 480 |
| Git diff/status subset | 243 |
| Python interpreter, opaque payload | 126 |
| Fathom | 89 |
| gofmt / cue / go / task | 73 / 30 / 27 / 16 |

There were 325 wrappers without a supported literal and 91 dynamic/unparseable
shell inputs. A wrapper can contain several candidates; these denominators are not
interchangeable. Claude command frequency is unmeasured, not zero.

**Method failure:** discovery read first records from 1,070 Codex files and up to
100 records in each of 4,208 Claude files, exceeding the brief's <=100-file envelope.
Discovery bytes/record counts were not retained. Only selected-body bytes were bounded.
The script was not saved and its aggregate vocabulary was not closed. No full commands
or arguments were reported, but that does not establish a privacy guarantee. INC-20
records the failure; do not cite this sample as bounded or reproducible evidence.
The known-family counts above are exploratory prioritization signals only.

Priority: typed bounded read/search first, immutable Git inspection second. Guarded
writes/worktrees and fixed test recipes need separate effect contracts. A published
safe-command list must never infer permission from historical frequency.

## Primary-source tool research

Reviewed 2026-10-23; none installed, executed, benchmarked or qualified here.

| Source | Useful adoption hypothesis | Boundary retained |
|---|---|---|
| [Destructive Command Guard](https://github.com/Dicklesworthstone/destructive_command_guard) | Established destructive-command packs, structural analysis and explicit indeterminate outcomes are preferable research inputs to an improvised regex allowlist | Its hook coverage/fault policy and script limitations need exact qualification; parsing does not establish full runtime effects or permission. No automatic adoption. |
| [Gitleaks](https://github.com/gitleaks/gitleaks) | External pinned scanner for secret-shaped content, with redacted machine output | Detection is incomplete; raw scanner output and errors can leak. A missing/partial scanner is unknown, not clean. No dependency or scanner install. |
| [ShellCheck](https://www.shellcheck.net/) | Static shell diagnostics and quoting/portability checks | Script lint is not authorization, data-flow containment or a proof that execution is harmless. |
| [Ollama structured outputs](https://docs.ollama.com/capabilities/structured-outputs) | Local schema-constrained classifier behind the existing judgment port, after endpoint/model/disclosure qualification | Output shape is not calibration. Localhost can proxy elsewhere; endpoint ownership, residency and egress need evidence. Current docs distinguish local structured output from cloud support. |
| [Ollama native decision API](https://ollama.com/blog/ollama-now-supports-jev-style-decision-models) | Prefer a native `/v1/systemone` adapter over pretending every local decision model is a chat model; choice/predicate/score questions fit the existing port | API and benchmark descriptions do not establish command-safety accuracy, our wire parity or any runtime speed here. No adapter or model is installed by this slice. |
| [Nimble](https://ollama.com/library/nimble), [Tev1](https://ollama.com/library/tev1), [Laya](https://ollama.com/library/laya) | Candidates for a bounded locally labeled study, varying resource/quality tradeoffs | Pin model bytes, runtime and license. Original Nimble advertises Apache 2.0; [Nimble v3](https://ollama.com/library/nimble-v3) is noncommercial. Do not silently substitute aliases or choose on general benchmark rank. |
| [LANCET Nano](https://huggingface.co/fingerthief/lancet-nano) | Specialized local command-risk triage with explicit review/unsupported limits | Publisher reports 78% risky caught and 12.4% safe stopped on its harder test. These are not independent project results; `not_flagged` is no authorization. Misses rule out a sole safety gate. |
| [TruffleHog](https://github.com/trufflesecurity/trufflehog), [Presidio](https://presidio.dataprivacystack.org/) | Complement credential signatures with configurable sensitive-data recognition | TruffleHog credential verification can contact external services; local-only profiles require verification disabled and enforced egress control. Presidio's recognizers can be fallible; neither supplies universal absence proof. |
| [Ollama authentication](https://docs.ollama.com/api/authentication) and [FAQ](https://docs.ollama.com/faq) | Local-required deployment needs explicit cloud-off, owned endpoint, authenticated gateway and enforced egress policy | Local API requires no authentication and can dispatch cloud models. A loopback URL or config flag alone does not prove data stayed local; no network configuration was changed here. |
| [OPA policy testing](https://www.openpolicyagent.org/docs/policy-testing) | Versioned positive/negative policy cases and explicit test coverage | Policy evaluation and actual dispatcher enforcement are separate; retain an external-engine adapter instead of a new evaluator language. |
| [Go os.Root](https://pkg.go.dev/os#Root) | Future traversal-resistant root-relative file operations without shell expansion | A root handle is not sensitivity authorization, network isolation or a solution to every hardlink/source-alias case. Runtime cases still required. |

The Astra consultation recommends extending the offline W1 reference first, separating
safety from economic fail-open hooks and keeping typed execution/maintenance queued.
[Consultation receipt](../evidence/command-governance/2026-10-23/astra-consultation.md).

## Owner request coverage

| ID | Ask / durable disposition |
|---|---|
| CG-R01 | Today's brief and local-classifier addendum located; native local decision API, candidate model/license differences, specialized triage and scanners evaluated above; installation and quality claims rejected. |
| CG-R02 | Build but do not install: CG-1..7 offline CLI; no host/global changes. |
| CG-R03 | Advanced deterministic/local/remote classification: CG-6/CG-11, typed unknown, explicit opt-in and per-site qualification. |
| CG-R04 | Common Bash audit and safer replacements: CG-2/3/8/9, exploratory limits and bounded intake regression. |
| CG-R05 | Secrets before disclosure or execution: CG-7/10, minimize first and explicit scanner coverage; not a universal clean verdict. |
| CG-R06 | Go/Rust and optional Fathom: 0035 chooses Go; CG-12 keeps source engine optional and authority independent. |
| CG-R07 | Org/team/global/project/personal configuration: CG-4/13, constraint intersection and separately authenticated delegation. |
| CG-R08 | One-approved hook maintenance and evaluation: CG-14, scoped finite lease, mandatory controls intact, deterministic re-arm/rollback. |
| CG-R09 | Policy/Review/Catalog interoperability: CG-13/15, separate owners, expected scope and preserved floors. |
| CG-R10 | Harness/AGENTS/hooks best practices, antipatterns and project intent: CG-15, criterion catalog with applicability and actual invocation evidence. |
| CG-R11 | Audit/fix skills: project-local skill routes evidence-bound proposals; installation remains a separate operation. |
| CG-R12 | Astra consultation and all decisions/rejections durable: 0035, consultation receipt and conditional work order. |
