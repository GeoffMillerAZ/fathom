// Package schemamigrationledger — invalid examples.
// ALL blocks below are commented out so that `cue vet ./...` passes.
// Uncomment individual blocks to observe the validation error described.
package schemamigrationledger

// import "github.com/geoffmilleraz/agentic-kit/foundations/data/schema-migration-ledger:schemamigrationledger"

// -- Example 1: empty ID ----------------------------------------------------
// emptyID would fail because ID must be a non-empty string.
//
// emptyID: schemamigrationledger.#MigrationRecord & {
// 	ID:        ""    // ERROR: violates & !=""
// 	From:      "v1"
// 	To:        "v2"
// 	AppliedAt: 1717200000
// }

// -- Example 2: empty To ----------------------------------------------------
// emptyTo would fail because To must be a non-empty string.
//
// emptyTo: schemamigrationledger.#MigrationRecord & {
// 	ID:        "0001_bad"
// 	From:      "v1"
// 	To:        ""    // ERROR: violates & !=""
// 	AppliedAt: 1717200000
// }

// -- Example 3: zero AppliedAt ----------------------------------------------
// zeroAppliedAt would fail because AppliedAt must be >0.
//
// zeroAppliedAt: schemamigrationledger.#MigrationRecord & {
// 	ID:        "0002_bad"
// 	From:      "v1"
// 	To:        "v2"
// 	AppliedAt: 0    // ERROR: violates int & >0
// }
