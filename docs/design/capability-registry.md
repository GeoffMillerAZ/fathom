# Capability registry — one list, one toggle vocabulary, no fifth seam

> Phase T of `docs/vision/20-ambient-fathom.md`. Answers the owner's ask for
> "excellent global and per project configuration to turn on and off
> features" and gates the ask for "a plugin system" (vision 20 §4, R2).
>
> **Status:** BUILT (`internal/capability`, `cmd/fathom/capabilities.go`,
> `cmd/fathom/capabilityenforce.go`, `cmd/fathom/doctor_capabilities.go`).
> §2's `switchable` rule was added after adversarial review caught the
> first implementation reporting switches it did not exercise.

---

## 1. The problem, stated precisely

Fathom has grown four extension seams and one implicit one. They work. But:

- There is **no single list** of what can be extended or switched off. A
  user learns them one at a time, from four different docs.
- There is **no shared toggle vocabulary**. Layers are enabled by existing
  in a directory. Guards are enabled by a CUE file being present and
  non-empty. Modules are enabled by a binding. MCP profiles are enabled by
  a URL path. Four mechanisms, four failure modes.
- **`doctor` cannot answer "why isn't X happening here?"** — the single
  most common adoption question — because nothing knows the full set of
  X's. Every onboarding defect of the last two weeks was a variant of this
  question going unanswered.
- Consequently, **new capability = new bespoke on/off mechanism**, and the
  next four will be as inconsistent as the first four.

The fix is not more mechanism. It is a registry over the mechanisms that
exist, and a rule that forbids a fifth.

---

## 2. What a capability is

```
capability := {
  id            // stable, kebab-case, never renamed: "guards", "obligations"
  seam          // which of the four seams delivers it
  summary       // one line, written for someone who has never used fathom
  default       // on | off  — what happens with no configuration at all
  scope_rules   // which scopes may set it (law 13)
  detect        // a pure function of repo+store state → state
  why_off       // when detected off, the reason AND the command
  cost          // what it costs when on: hook latency, schema bytes, or none
}
```

Two things make this more than a config list.

**`detect` is a function, not a flag.** A capability is "on" when it is
*observably operating*, never when a file says it should be. `guards: on`
in a config file with no guard rules compiled is **off, with a reason**.
This is the same discipline as the observatory's value-or-named-reason
primitive (law 3), applied to configuration.

**`why_off` is required.** A capability that can be off without being able
to say why is not registrable.

**`switchable` is required, and it is the rule the first implementation
broke.** §2's observation rule was written only in the ON direction
("configured on but not working is off"), and the implementation took the
unstated OFF direction: resolution was read by the two *reporting*
surfaces and by nothing else. So `fathom capabilities set guards off`
wrote a file, the table reported "switched off by this repository", and
guards kept firing on every edit — a config read rendered as a
behavioural fact, shipped inside the package built to prevent exactly
that (found by adversarial review of this document, 2026-08-08).

The rule, in both directions now:

> A capability may be marked `switchable` **only** if some enforcement
> site reads the resolution and acts on it. Marking it switchable obliges
> you to add a case to `TestSwitchableCapabilitiesAreEnforced`, which
> proves that flipping the setting changes *observed behaviour*, not just
> reported state. Everything else is **status**: it reports what is
> happening, `set` refuses it naming what the real control is, and the
> renderer never prints provenance beside it (provenance implies a switch).

Today three are switchable — `obligations`, `guards`, `verify-gate` —
enforced in `edithook.go`, `edithook_pre.go` and `stophook.go`
respectively, each gated *before* any daemon traffic so that off means
silent rather than merely quieter. Enforcement **fails open**: a config
file that does not load never disarms a safety feature; it is reported
loudly by the verb and failed loudly by doctor instead.

Together, `why_off` and `switchable` are what make `doctor` able to answer
the adoption question truthfully, and they are why the registry is worth
building at all.

---

## 3. The four seams, and the refusal of a fifth

| Seam | What it extends | Existing mechanism |
|---|---|---|
| **Expert modules** | domain knowledge answerable by query | `.fathom/modules.cue` binding, served on `/rpc/modules` |
| **Layer packs** | semantic layers over a corpus | `fathom layers install`, `.fathom/layers/<name>/` |
| **Guards** | pre-edit and pre-command guidance | `.fathom/guard.cue` |
| **MCP profiles** | which tools a surface exposes | server config, `/rpc/agent` vs `/rpc` |

**The refusal, stated so it can be enforced in review:** a proposed
capability that does not fit one of these four seams is rejected until its
author writes down, for each seam, why it does not fit. Three of the four
rejections will usually be obvious; the fourth is where the real design
conversation happens, and it is nearly always cheaper to widen a seam than
to add one.

This is a *documentation* gate, not a code gate — but it is written here so
that "which seam?" is the first question asked in any review of a new
feature.

---

## 4. Scope resolution (law 13, made concrete)

Three scopes, resolved in this order, most specific winning:

| Scope | Location | May do |
|---|---|---|
| **built-in** | compiled defaults | define `default` |
| **user** | `~/.fathom/capabilities.cue` | change a default **for repositories that have not spoken** |
| **project** | `<repo>/.fathom/capabilities.cue` | **govern** — the final word for that repository |

Law 13's asymmetry survives intact: **user scope may set a default, only
project scope may govern.** A user-scope setting never overrides a
project-scope one, and never silently changes behaviour in a repository
that has made its own choice. Concretely, `resolve()` returns not just a
value but the scope that supplied it, and every surface that renders a
capability renders that provenance beside it. "Guards: off (your user
default)" and "Guards: off (this repo's .fathom/capabilities.cue)" are
different facts and a user debugging their setup needs to tell them apart.

**Refusals, loudly:**

- A user-scope file that tries to set a project-only key is **rejected at
  load with the key named** — not ignored. Silent ignoring of
  configuration is the defect class that produced O22.
- An unknown capability id is rejected with the closest known id
  suggested. A typo that silently disables a safety feature is the
  worst possible failure and it has already happened once in this
  program (the phase-N tier typo).

---

## 5. The verb

Law 10 (verb-first): the UI renders what the verb returns; the verb never
learns anything the UI knows.

```
fathom capabilities                    # table: id, state, scope, why
fathom capabilities --json             # the model the UI renders
fathom capabilities --service <svc>    # resolved for one repository
fathom capabilities explain <id>       # what it is, what it costs, how to turn it on
fathom capabilities set <id> <on|off> [--scope user|project]
```

`explain` is the teaching surface and deserves care: it states what the
capability does in one sentence, what it costs when on (hook latency in
milliseconds, schema bytes, or "nothing"), what turns it on, and what
*evidence* would appear on the dashboard if it were on. That last clause
connects configuration to impact, which is the whole point of the program.

---

## 6. Doctor integration

`doctor` gains one section, and it is the section that answers the
adoption question:

```
capabilities (7 of 11 operating)
  ✓ index               fathom (60,782 symbols, fresh at HEAD)
  ✓ obligations         6 rules compiled, injected on edit
  ✗ guards              off — no .fathom/guard.cue in this repo
                        → fathom layers install ui-appkit   (or write one)
  ✗ modules             off — no binding in .fathom/modules.cue
                        → nothing to run: no module is published for this repo yet
  ✗ verify-gate         off — lefthook.yml has no pre-push verify step
                        → fathom init --hooks   (adds only fathom's own region)
```

Every ✗ carries a reason and either a command or an explicit "nothing to
run, here's why". This is the same contract the value brief's work order
uses, and it should share the type.

---

## 7. Cost accounting

Each capability declares its cost, and the registry can total it. This
matters because the honest answer to "should I turn everything on?" is
usually no, and fathom should be the one saying so:

| Capability | Cost when on |
|---|---|
| index | none per-session; disk + ingest time |
| agent surface | 11,839 bytes of tool schema in every session's context |
| obligations | one hook round-trip per edit (~15 ms local) |
| guards | one hook round-trip per edit, shared with obligations |
| session ledger | one transcript parse per session end |
| narrate | story rows written per session |
| assess | model tokens per explicit `fathom assess` run, reported in its own output; off by default (ADR-046) |

`fathom capabilities --cost` totals the standing per-session cost. When
phase X (budgeted answers) lands, this table stops being hand-maintained
and starts being measured.

---

## 8. Storage format

CUE, consistent with the rest of `.fathom/`, and validated at load:

```cue
// ~/.fathom/capabilities.cue — defaults for repositories that have not spoken
capabilities: {
    narrate: off: true      // I don't want story capture anywhere by default
}
```

```cue
// <repo>/.fathom/capabilities.cue — governing, this repository only
capabilities: {
    narrate: off:    false  // ...except here
    guards:  strict: true
}
```

The schema is a closed struct over known ids, so an unknown key is a CUE
error before it is a Go error — the same guarantee the other `.fathom`
files already have, and the reason the `?:` trust-boundary inversion (phase
O) cannot recur here: **no known field may be declared optional in an
instance.** The loader refuses it.

---

## 9. Test obligations

- Resolution: built-in → user → project, with provenance returned, proven
  by table test over all three scopes.
- A user-scope file setting a project-only key is **rejected with the key
  named**, not ignored.
- An unknown id is rejected with a suggestion.
- `detect` returns off-with-reason, never a bare boolean, for every
  registered capability.
- Every registered capability has a non-empty `why_off` — enforced by a
  test that iterates the registry, so a new capability cannot be added
  without one.
- Doctor's capability section renders in a repository with **no** fathom
  configuration at all and produces no known-false failures (the O23 class).

---

## 10. What this does not do

- It does not execute anything. Turning a capability on may print a command
  (phase U executes them).
- It does not add a seam, a hook type, or a plugin loading path.
- It does not make capabilities dynamic at runtime — resolution happens at
  session start and at explicit reload, so behaviour within a session is
  stable and explicable.
