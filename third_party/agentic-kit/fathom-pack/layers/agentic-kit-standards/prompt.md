# agentic-kit-standards — compile prompt

You are compiling agentic-kit's doctrine into obligations: short, cited,
glob-scoped rules served at edit time inside the kit repo and in consumers.

Source corpus: CLAUDE.md (the gate, hard rules, package shape, layering),
VISION.md (the one-sentence test and junk gate), docs/adopting.md,
docs/why-cue.md, docs/governance.md, and specs/.

Emit a claim ONLY when it: (1) can be violated in a single edit; (2) cites
its source section; (3) scopes to globs (go.mod, foundations/**, kits/**,
TRACKING.md, *_test.go, *.cue). Imperative, ≤40 words, source named. The
load-bearing families, in priority order:

1. stdlib-only and the empty require block.
2. Layering: imports point downward; foundations import stdlib + siblings.
3. The package shape: schema.cue ↔ types.go mirroring (CUE owns shape, Go
   owns behavior — drift is a bug), mock.go where an interface exists,
   testdata fixtures.
4. Determinism: clock seams, seedable randomness, -race, no sleeps/network.
5. Governance WASM-safety; concurrency contracts in doc comments.
6. TRACKING.md checkbox honesty; the gate before any commit.
7. Consumer discipline: extend the kit not the app; mocks are adapters;
   org identity is validated data, never code.

Reject: style opinions without a doc citation, anything the guard pack
already states verbatim (guards warn, obligations cite), and narrative
claims no single edit can violate.
