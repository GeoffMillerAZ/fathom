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
// Keep that threshold within SQLite's unix VFS write limit so large statements can spill.
// Memory remains bounded by the threshold and the hard heap limit applyTenancyPragmas sets.

import (
	"sync"

	"modernc.org/libc"
	sqlite3 "modernc.org/sqlite/lib"
)

// stmtJournalSpillBytes keeps small journals in memory, at almost twice SQLite's 64 KiB
// default. SQLite's own src/os_unix.c seekAndWriteFd asserts nBuf==(nBuf&0x1ffff),
// then masks nBuf with 0x1ffff. memjrnlCreateFile flushes the in-memory journal in a
// spill-threshold-sized chunk via one xWrite; at 131,072 bytes or more the unix VFS
// short write yields SQLITE_FULL, even with free disk. 131,071 is a safe bound: any
// spill at or under 0x1ffff keeps every flush write unmasked, for every page size, on
// all unix targets (including Linux), also with cgo; Windows is unaffected. (A journal
// of k*(page+4) bytes never lands on exactly 2^17, so 131,072 also passes in practice,
// but nothing here pins that.) A driver upgrade alone cannot justify raising it.
//
// Cost: LB-F208 raised this to 4 MiB to keep a typical row's roughly 60-page (246 KB)
// journal in memory, avoiding a temp file per row (3.5 s of a 17 s polyglot build).
// At 4 KiB pages this cap holds about 31 pages, so typical rows spill again. A noisy
// review micro-benchmark saw system CPU of 3.4-5.0 s versus 1.6-2.3 s before, not a
// stable performance estimate. LB-F208-bulk defers replacing the per-row call-index
// trigger with one bulk statement per batch, measured before and after.
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
