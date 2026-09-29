# Why CUE — for engineers who've never heard of it

> **TL;DR** — [CUE](https://cuelang.org) lets you write *types and constraints*
> and *data* in one language, then check, at build time, that the data actually
> satisfies the types. agentic-kit governs AI-driven work, where the **shape of
> data must be guaranteed even though the AI producing it is fuzzy**. CUE is the
> single source of truth for that shape; Go is the source of truth for behavior.
> This doc explains what CUE is and the specific features that make it a great
> fit — using real schemas from this repo.

---

## 1. The one big idea

Most config/schema tools separate three things: a *schema language* (JSON
Schema, protobuf), a *config language* (YAML, JSON, env files), and a
*validator* you run separately. CUE collapses all three into one.

In CUE, **a type is just a value, and a value is just a (very specific) type.**
Everything is built from one operation: **unification** (`&`), which merges two
constraints and fails if they contradict.

```cue
// A "type": any non-empty string
S: string & !=""

// A "value": the string "hello" — which is itself a constraint
H: "hello"

// Unify them — succeeds, because "hello" satisfies (string & !="")
OK: S & H        // => "hello"

// Unify with the empty string — FAILS at evaluation time
Bad: S & ""      // => error: invalid value "" (does not satisfy !="")
```

There's no inheritance, no overrides, no "last writer wins." You only ever
*narrow* constraints by unifying them. That property — **order-independent,
conflict-detecting merge** — is what makes CUE predictable, and it's exactly
what a platform that prizes determinism wants.

---

## 2. The problem it solves here

agentic-kit is a platform for **governed agentic work**: LLMs and agents produce
findings, prose, classifications, and corrections. That output is inherently
probabilistic. But the platform around it — evidence contracts, event shapes,
review records, form definitions, sensitivity catalogs — must be **exact and
auditable**. The architecture rule is:

> **CUE is authoritative for the *shape* of data. Go is authoritative for
> *behavior*. A drift between them is a bug.**

Every foundation package carries a `schema.cue` (the contract) next to a
`types.go` (the Go structs that mirror it). CUE gives us four things Go structs
alone can't:

1. **Constraints, not just field names** — "non-empty string", "value in this
   enum", "list of at least one item", "number between 0 and 1".
2. **Build-time validation** — caught by `cue vet` in the pre-commit hook, long
   before runtime.
3. **Config-as-validated-data** — org-specific catalogs are *CUE data* that must
   satisfy a schema, so the platform stays general-purpose without hard-coding.
4. **Executable test fixtures** — example data that the compiler checks.

---

## 3. The features that make CUE a great fit

### 3.1 Types that *validate*, not just *describe*

Here's a real contract from `foundations/contracts/schema.cue`:

```cue
// #EventShape is an elementary platform event. Fields marked '!' are required.
#EventShape: {
	ID!:        string & !=""          // required, must be non-empty
	Kind!:      string & !=""
	Source!:    string & !=""
	Timestamp!: string & !=""
	Payload!:   {[string]: _}          // a map with string keys, any values
}

// An enum is just a disjunction of values
_nodeTypeEnum: "compute" | "storage" | "gateway" | "inference" | "orchestrator"

#NodeType: {
	ID!:     string & !=""
	Type!:   _nodeTypeEnum             // must be one of the five
	Labels!: {[string]: string}
}

// A non-empty list whose every element is a non-empty string
#AnswerArchetype: {
	Name!:             string & !=""
	Sections!:         [string & !="", ...string & !=""]   // >= 1 section
	RequiredEvidence!: bool
}
```

A Go struct can say "this field is a `string`." It cannot say "this field is a
string that is never empty, and this other field is one of exactly five values,
and this list has at least one element" — without you hand-writing validation
code and remembering to call it. CUE encodes all of that *declaratively*, and
the same definitions are checked by the compiler. The `!` marks a required
field; `&` adds a constraint; `|` is a disjunction (enum); `[T, ...T]` is a
non-empty list of `T`.

### 3.2 One definition, many projections

A CUE definition isn't locked into one output format. The same `schema.cue` can
**export to Go types, JSON Schema, and OpenAPI** (`cue exp gen`, `cue def`,
`cue export`). That's why this repo can keep `schema.cue` as the *canonical*
contract and treat `types.go` as a faithful mirror, rather than maintaining two
independent definitions that silently drift. One source of truth, many
projections — the platform's data model is described once.

A consumer may keep a projection *value* in Go where a Go test pins it; that
is a recorded divergence from this rule, not an exception to it — see
[`docs/intent/decisions/0015`](intent/decisions/0015-projections-in-go-recorded-divergence.md).

### 3.3 Configuration is data — and data is validated

This is the feature that let agentic-kit be **general-purpose / org-agnostic**.
Anything organization-specific (sensitivity catalogs, DLP rules, taxonomies,
LLM-gateway providers) is **pure CUE data that must satisfy a schema** — never
hard-coded Go.

From `foundations/security/sensitivity-classifier/schema.cue`:

```cue
#ClassRule: {
	ID:          string & !=""
	Pattern:     string & !=""              // a Go regexp
	Level:       dst.#SensitivityLevel      // must be a known level (imported)
	Description: string & !=""
}

#Catalog: {
	Name:    string & !=""
	Version: string & !=""
	Rules: [#ClassRule, ...#ClassRule]      // at least one rule
}
```

An organization adds a sensitivity rule by writing CUE data that unifies with
`#Catalog`. If they typo a level, the build fails with a precise message — see
the negative fixtures in `testdata/invalid.cue`:

```
Level: conflicting values "proprietary" and "public" ...   // not a valid level
Rules: length must be >= 1                                  // empty catalog
Pattern: invalid value "" (does not satisfy != "")          // empty pattern
```

The catalog is *human-readable, git-reviewable, and compiler-checked at the same
time*. No code change, no redeploy of logic — just data that has to pass the
gate. (Note the `import` on the `Level` field: definitions compose across
packages by unification, with no inheritance hierarchy to reason about.)

### 3.4 Test data that is also a contract

Each schema package ships `testdata/valid.cue` (a positive example that must
unify with the schema) and a commented `testdata/invalid.cue` documenting the
exact error each violation produces. These aren't decorative — `cue vet` runs
them, so **a schema change that breaks a real example is caught at commit time**,
not in production. Combined with the build gate (next section), the test data
turns "is this shape still valid?" into a question the compiler answers.

### 3.5 Closed by default — typos can't sneak in

CUE definitions (`#Foo`) are **closed**: a field that isn't in the definition is
rejected. Misspell `Timestamp` as `Timetamp` in an event and you get an error,
not a silently-dropped field. For audit-grade data this is a quiet superpower —
the class of "I set the config key but nothing happened" bugs largely disappears.

### 3.6 Gradual and composable

You can start loose (`Port: int`) and tighten later (`Port: int & >0 & <65536`)
without restructuring. Definitions unify and compose with `&`, so shared
constraints live in one place and get pulled in where needed — the opposite of
copy-pasted validation logic scattered across services.

---

## 4. Where it shows up in the build

CUE isn't a side artifact here — it's wired into the **determinism guardrail**:

- **`task cue-vet`** validates every `schema.cue` package (and its positive
  testdata) across the repo.
- **`task validate`** (the full gate) runs `gofmt` + `go vet` + `cue vet` +
  `go test -race`. This is what must be green before anything ships.
- **lefthook pre-commit** runs `cue vet` on staged schema dirs, so a broken
  contract never even reaches a commit; **pre-push** runs the full `task validate`.

So the CUE contracts are continuously enforced, automatically, by the same hooks
that keep the Go code honest.

---

## 5. "Why not just …?"

| Alternative | What CUE adds |
|-------------|---------------|
| **Go structs + hand-written `Validate()`** | Constraints are declarative and compiler-checked, not imperative code you must remember to call. (We still keep `Validate()` in Go for runtime, but it *mirrors* the CUE — the CUE is the spec.) |
| **JSON Schema** | One language for schema *and* data *and* config; real composition via unification; far less verbose; native enums/constraints; round-trips *to* JSON Schema when you need it. |
| **protobuf** | Rich value constraints (ranges, regexes, non-empty, enums-as-disjunctions), and config-as-data — protobuf describes wire shape, not valid *value spaces*. |
| **YAML + a linter** | The "schema" and the "config" are the same artifact, checked by the same tool; no drift between a doc that says what's valid and a file that may not be. |

---

## 6. Learn CUE in 15 minutes

- The core mental model: **everything is a value; types are values; `&` unifies,
  `|` is a disjunction; definitions (`#`) are closed.** Once that clicks, the
  rest follows.
- Official tour & docs: <https://cuelang.org/docs/> and the interactive
  <https://cuelang.org/play/>.
- In this repo: read any `foundations/*/schema.cue` next to its `types.go` — the
  schema is the contract, the Go is the behavior. Start with
  `foundations/contracts/schema.cue`.

If you remember one thing: **CUE makes the *shape* of data a build-time
guarantee.** In a platform whose whole job is to govern fuzzy AI output, that
guarantee is the foundation everything else stands on.
