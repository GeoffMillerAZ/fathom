// invalid.cue — negative fixtures for the support matrix schema,
// documented commented-out so live `cue vet` stays green (repo convention).
// Each case is asserted to fail by internal/support's schema test
// (TestSupportSchema_Negatives, plan row SM-2).
package support

// 1. a language target in the support targets file (languages live in the queue):
// bad1: #Target & {kind: "language", id: "go", name: "Go", target: {L1: "L1"}, plan_row: "SM-1", wave: 1}

// 2. a proven cell with no reading — a level without its evidence:
// bad2: #Cell & {capability: "L2", state: "proven-good", level_met: "L2"}

// 3. a tolerated cell that claims a level:
// bad3: #Cell & {capability: "L3", state: "tolerated", level_met: "L3"}

// 4. a state outside the computed vocabulary (a hand-set status):
// bad4: #Cell & {capability: "L1", state: "done"}

// 5. a reading that types a grade outside the closed list:
// bad5: #Reading & {kind: "rules", id: "cursor-rules", reading_date: "2026-09-27", commit: "86d4c5e", instrument: "x", metrics: {}, met: {read: "great"}}

// 6. a target row whose plan_row is not a row id:
// bad6: #Target & {kind: "spec", id: "spec-kit", name: "Spec Kit", target: {read: true}, plan_row: "soon", wave: 1}
