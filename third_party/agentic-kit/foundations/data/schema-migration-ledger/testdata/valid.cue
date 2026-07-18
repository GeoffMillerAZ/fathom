// Package schemamigrationledger — valid test data that must unify with schema.cue.
package schemamigrationledger

import "github.com/geoffmilleraz/agentic-kit/foundations/data/schema-migration-ledger:schemamigrationledger"

// initialMigration is the first migration (From is allowed to be empty).
initialMigration: schemamigrationledger.#MigrationRecord & {
	ID:        "0001_init"
	From:      ""
	To:        "v1"
	AppliedAt: 1717200000
}

// addColumnsMigration shows a migration from one schema version to another.
addColumnsMigration: schemamigrationledger.#MigrationRecord & {
	ID:        "0002_add_columns"
	From:      "v1"
	To:        "v2"
	AppliedAt: 1717201000
}

// indexMigration demonstrates a migration with a descriptive ID.
indexMigration: schemamigrationledger.#MigrationRecord & {
	ID:        "0003_add_index"
	From:      "v2"
	To:        "v3"
	AppliedAt: 1717202000
}

// minAppliedAt shows that AppliedAt = 1 (the minimum positive int) is valid.
minAppliedAt: schemamigrationledger.#MigrationRecord & {
	ID:        "0004_min_ts"
	From:      "v3"
	To:        "v4"
	AppliedAt: 1
}
