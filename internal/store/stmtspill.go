package store

// stmtspill.go — LB-F208 (wave lg, LB-M5): raise SQLite's statement-journal spill threshold.
//
// An INSERT into observations fires the call-index trigger (derived.go), so inside the
// ingest's explicit transaction every row is a multi-write statement and SQLite opens a
// statement journal for it: the original image of each page the statement touches, kept in
// memory until it passes SQLITE_STMTJRNL_SPILL (64 KiB by default) and then spilled to a
// temporary file. Raising the threshold avoids that file for small call-index updates.
//
// The spill threshold is a process-wide sqlite3_config option that only takes effect before
// SQLite initializes, so it is set once, before the first connection of any store opens.
// Keep that threshold within the driver's write-size limit so large statements can spill.
// Memory remains bounded by the threshold and the hard heap limit applyTenancyPragmas sets.

import (
	"sync"

	"modernc.org/libc"
	sqlite3 "modernc.org/sqlite/lib"
)

// stmtJournalSpillBytes keeps small journals in memory, at almost twice SQLite's 64 KiB
// default. modernc.org/sqlite v1.52.0's seekAndWriteFd masks write lengths with 0x1ffff.
// SQLite flushes spill-sized chunks, so the former 4 MiB threshold became a zero-byte
// write and SQLITE_FULL on spill, even with free disk. Stay at the largest safe chunk
// size: larger journals spill to disk instead of aborting the observation transaction.
const stmtJournalSpillBytes int32 = (128 << 10) - 1

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
