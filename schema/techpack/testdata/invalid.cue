package techpack

// INVALID examples, kept commented so `cue vet ./...` stays green; the Go
// tests (internal/projectdir/techpack_test.go) assert each fails.
//
// 1. A kit pack with no header:
// bad1: #Pack & {guards: [{scopes: ["**/*.go"]}]}
//
// 2. A kit pack carrying layer_rules (only the repository governs):
// bad2: #Pack & {pack: {name: "k", version: 1, about: "a"}, layer_rules: []}
//
// 3. capabilities:
// bad3: #Pack & {pack: {name: "k", version: 1, about: "a"}, capabilities: {}}
//
// 4. bindings:
// bad4: #Pack & {pack: {name: "k", version: 1, about: "a"}, bindings: []}
//
// 5. exclude_packs:
// bad5: #Pack & {pack: {name: "k", version: 1, about: "a"}, exclude_packs: ["x"]}
//
// 6. orientation (the project pack's alone):
// bad6: #Pack & {pack: {name: "k", version: 1, about: "a"}, orientation: [{anchor: "a", why: "b"}]}
//
// 7. A generated entry with no sources:
// bad7: #Pack & {pack: {name: "k", version: 1, about: "a"}, generated: [{output: "o", source: [], regenerate: "r"}]}
//
// 8. A header with version 0:
// bad8: #Pack & {pack: {name: "k", version: 0, about: "a"}}
//
// 9. A readouts entry with an unknown key (the rule shape is closed):
// bad9: #Pack & {pack: {name: "k", version: 1, about: "a"}, readouts: [{id: "x", version: 1, about: "a", match: argv0: ["x"], stage: [{kind: "extract"}], exec: "sh"}]}
