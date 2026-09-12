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

// 6. an event catalogue row with no `wired` verdict — a row that does not
// say whether fathom wires the event is the one shape doctor's
// hooks_coverage cannot report honestly (HP-32):
// bad6: #EventCatalogEntry & {event: "SessionStart", reason: "maybe?"}

// 7. a canonical event outside the vocabulary:
// bad7: #EventBinding & {capability: "prime", event: "telepathy", harness_event: "X", verb: "v", marker: "m", timeout_sec: 1, vehicle: "hook-inject"}

// 8. a capability outside the vocabulary (wave 3 added five; a typo in one
// of them must fail rather than render as a blank table row):
// bad8: #EventBinding & {capability: "subagent-primer", event: "subagent-start", harness_event: "SubagentStart", verb: "v", marker: "m", timeout_sec: 1, vehicle: "hook-inject"}
