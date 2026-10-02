package store

// stmtspill.go — LB-F208 (wave lg, LB-M5): SQLite's statement journal stays in memory.
//
// An INSERT into observations fires the call-index trigger (derived.go), so inside the
// ingest's explicit transaction every row is a multi-write statement and SQLite opens a
// statement journal for it: the original image of each page the statement touches, kept in
// memory until it passes SQLITE_STMTJRNL_SPILL (64 KiB) and then spilled to a temporary
// file. A function that calls thirty things writes about sixty random index pages through
// observation_calls' two name indexes, so a row's journal is a few hundred KiB: each such
// statement created, wrote (sixty pwrites) and deleted a temp file. On the polyglot set
// that was 3.5 s of the 17 s base build's CPU and 3.5 s of the head's, 70% of
// PutObservations, all of it system time (docs/bench/readings/lang/footprint.md, wave lg).
//
// The spill threshold is a process-wide sqlite3_config option that only takes effect before
// SQLite initializes, so it is set once, before the first connection of any store opens.
// Memory is bounded by the largest single statement's journal (freed when it finishes) and
// by the hard heap limit applyTenancyPragmas sets; nothing else about the journal changes,
// so the rows written are the rows written before.

import (
	"sync"

	"modernc.org/libc"
	sqlite3 "modernc.org/sqlite/lib"
)

// stmtJournalSpillBytes is the in-memory statement journal's size before it spills to a
// temporary file: 4 MiB, 64 times SQLite's default.
const stmtJournalSpillBytes int32 = 4 << 20

var (
	stmtSpillOnce    sync.Once
	stmtSpillApplied bool
)

// raiseStatementJournalSpill sets SQLITE_CONFIG_STMTJRNL_SPILL once per process, before
// SQLite's first connection. It reports whether the option took (SQLITE_OK); a process
// that opened a SQLite connection first (the cmd package's pack reader) keeps SQLite's
// default, which is only slower.
func raiseStatementJournalSpill() bool {
	stmtSpillOnce.Do(func() {
		tls := libc.NewTLS()
		defer tls.Close()
		bp := tls.Alloc(16)
		defer tls.Free(16)
		rc := sqlite3.Xsqlite3_config(tls, sqlite3.SQLITE_CONFIG_STMTJRNL_SPILL, libc.VaList(bp, stmtJournalSpillBytes))
		stmtSpillApplied = rc == sqlite3.SQLITE_OK
	})
	return stmtSpillApplied
}
