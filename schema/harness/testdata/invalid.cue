// invalid.cue — negative fixtures for the harness descriptor schema,
// documented commented-out so live `cue vet` stays green (repo convention).
// Each case is asserted to actually fail by
// internal/harness/schema_test.go (TestHarnessSchema_Negatives).
package harness

// 1. tier outside the degradation ladder:
// bad1: #Descriptor & {id: "x", display: "X", label: "x", tier: "E", detect: [], instruction_files: [{path: "AGENTS.md", mode: "native"}], vehicles: _, fail_mode: "", gate_posture: "", skills: {variant: "SKILL.md", verified: false}, mcp: {add_command: ""}, env: {}, verified_against: "unverified"}

// 2. a vehicle outside the vocabulary:
// bad2: #Descriptor & {vehicles: {prime: "telepathy"}}

// 3. a descriptor missing a capability (vehicles must name every one):
// bad3: #Descriptor & {vehicles: {prime: "pull"}} & {vehicles: close({prime: "pull"})}

// 4. an event binding with a non-positive timeout:
// bad4: #EventBinding & {capability: "prime", event: "session-start", harness_event: "SessionStart", verb: "session-hook", marker: "fathom session-hook", timeout_sec: 0, vehicle: "hook-inject"}

// 5. a codec landing date that is not ISO:
// bad5: #Descriptor & {codec_landed: "yesterday"}
