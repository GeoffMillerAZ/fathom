package readouts

// INVALID examples, kept commented so `cue vet ./...` stays green; the Go
// test (internal/readout/schema_test.go, TestReadoutsSchemaNegatives)
// asserts each fails validation (schema/AGENTS.md).
//
// 1. An unknown key: a field that would name what to run.
// bad1: #File & {schema: 1, rule: [{id: "x", version: 1, about: "a", match: argv0: ["make"], command: "make all", stage: [{kind: "drop", pattern: "a"}], test: [{input: "a"}]}]}
//
// 2. argv0 as a path or shell text.
// bad2: #File & {schema: 1, rule: [{id: "x", version: 1, about: "a", match: argv0: ["make && rm -rf /"], stage: [{kind: "drop", pattern: "a"}], test: [{input: "a"}]}]}
//
// 3. A join outside the built-in table.
// bad3: #File & {schema: 1, rule: [{id: "x", version: 1, about: "a", match: argv0: ["make"], stage: [{kind: "join", join: "curl"}], test: [{input: "a"}]}]}
//
// 4. A URL in free text.
// bad4: #File & {schema: 1, rule: [{id: "x", version: 1, about: "see https://example.invalid", match: argv0: ["make"], stage: [{kind: "drop", pattern: "a"}], test: [{input: "a"}]}]}
//
// 5. A stage kind outside the closed set.
// bad5: #File & {schema: 1, rule: [{id: "x", version: 1, about: "a", match: argv0: ["make"], stage: [{kind: "exec"}], test: [{input: "a"}]}]}
//
// 6. No fixture.
// bad6: #File & {schema: 1, rule: [{id: "x", version: 1, about: "a", match: argv0: ["make"], stage: [{kind: "drop", pattern: "a"}], test: []}]}
//
// 7. A schema version other than 1.
// bad7: #File & {schema: 2, rule: [{id: "x", version: 1, about: "a", match: argv0: ["make"], stage: [{kind: "drop", pattern: "a"}], test: [{input: "a"}]}]}
//
// 8. A fixture file that climbs out of the rule's directory.
// bad8: #File & {schema: 1, rule: [{id: "x", version: 1, about: "a", match: argv0: ["make"], stage: [{kind: "drop", pattern: "a"}], test: [{input_file: "../secret"}]}]}
