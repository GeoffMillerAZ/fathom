# Command governance: offline reference and later admission

Request (Geoff, 2026-10-23): "lets start building (but not yet installing) tooling for advanced classifiers and safety and governance decision tooling. we can also audit our most common bash commands as bash is likely the most dangerous tool."

Authority: [decision 0035](../intent/decisions/0035-command-safety-starts-with-offline-qualification.md), [CG specification](../designs/command-governance.md). This is an offline W1 reference only. CG-1..7 passed the full gate, independent negative review and guide-only trial. The daemon index was refreshed before strict verification of the claimed rows. This is offline qualification, not installed-hook or runtime execution acceptance.

[0039](../intent/decisions/0039-governed-command-tools-preserve-evidence-and-authority.md)
extends the same W1 slot with a separate exact-content inspection command. The
[GT work order](governed-command-tools.md) tracks that increment and full follow-through.
Partial read/search delivery does not check off all of CG-8/10/16.

- [x] CG-1: command-local audit/assess CLI with no action authority {id: CG1, symbol: run, package: cmd/command-lab}
- [x] CG-2: strict explicit-source manifest, caps and honest partial coverage {id: CG2, symbol: audit, package: cmd/command-lab, needs: CG1}
- [x] CG-3: closed-family proposal and literal candidate accounting without raw output {id: CG3, symbol: auditLine, package: cmd/command-lab, needs: CG2}
- [x] CG-4: independent required layer set and restrictive composition {id: CG4, symbol: validateAssess, package: cmd/command-lab, needs: CG1}
- [x] CG-5: typed request blockers and advisory-only disposition {id: CG5, symbol: assess, package: cmd/command-lab, needs: CG4}
- [x] CG-6: sanitized classifier projection and captured response validation {id: CG6, symbol: projectedRequest, package: cmd/command-lab, needs: CG5}
- [x] CG-7: strict input codec, fixtures, privacy tests and guide-only trial {id: CG7, symbol: readStrict, package: cmd/command-lab, needs: CG1}

CG1..7 acceptance: CG-A01..09; `task validate` green, direct binary 0/1/2
checks, and `fathom verify --service agentic-kit --plan docs/plans/command-governance.md`
after indexing. Any `[x]` requires that row's actual proof. Synthetic fixtures do
not measure live security or host interception.

- [-] CG-8: typed execution descriptors and adapters {id: CG8, disposition: deferred, reason: "offline reference has no execution admission", trigger: "named consumer admits a typed operation with negative proof"}
- [-] CG-9: independent authorization and destination effect gate {id: CG9, disposition: deferred, reason: "receiver authority and effect gate are not admitted", trigger: "named receiver defines principal and controlled path"}
- [-] CG-10: pinned secret scanner and disclosure coverage adapters {id: CG10, disposition: deferred, reason: "no scanner or raw-output adapter admitted", trigger: "typed read or output consumer declares disclosure controls"}
- [-] CG-11: live local or advanced provider profiles {id: CG11, disposition: deferred, reason: "captured classifier seam is sufficient for offline reference", trigger: "calibrated site has eligible residency and spend owner"}
- [-] CG-12: optional source intelligence adapter {id: CG12, disposition: deferred, reason: "offline reference needs no evidence engine", trigger: "named consumer requires revision-bound source evidence"}
- [-] CG-13: authenticated policy/delegation authority {id: CG13, disposition: deferred, reason: "caller-supplied layers are a simulation", trigger: "receiving deployment identifies issuer and trust roots"}
- [-] CG-14: governed hook maintenance transaction {id: CG14, disposition: deferred, reason: "no installer or host controller admitted", trigger: "host owner admits bounded maintenance and failure probes"}
- [-] CG-15: harness review catalog and goal mapping {id: CG15, disposition: deferred, reason: "skill provides review method without catalog implementation", trigger: "named harness owner supplies criteria and independent expected scope"}
- [-] CG-16: controlled consumer pilot and measured rollout {id: CG16, disposition: deferred, reason: "offline sample has no controlled consumer", trigger: "one receiver admits versioned operation and baseline trial"}
