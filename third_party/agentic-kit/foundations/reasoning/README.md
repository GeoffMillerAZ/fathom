# foundations/reasoning

Reasoning primitives that consume graded evidence
(see `foundations/evidence/README.md` for the pipeline that feeds this layer).

| Package | Purpose | Concurrency contract |
|---|---|---|
| `defeasible` | Generalized "effective state" engine: a base set of provisions plus layered, typed modifiers (each rulable valid/invalid). `Resolve` computes the effective state from active modifiers — with conflict/supersession handling — plus a full trace and confidence. Flip a modifier's status (`WithStatus`) and re-`Resolve` to pivot contingencies instantly | Not documented — `Resolve` is a pure function over caller-owned slices; assume single-goroutine |

Domain-agnostic by design: the package knows no "contract" semantics — callers
supply provision keys and modifier effects. The confidence model is
weakest-link (effective confidence ≤ min of base and every applied modifier),
deliberately consistent with the `evidence-grade` bounding ethos.
