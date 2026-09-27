package store

// pragmas.go — RT-78 (docs/design/responsible-tenant.md §6, I48): SQLite's
// own memory is outside Go's accounting, so the store bounds it at open.
// Every pooled connection carries an 8 MiB page cache and a 64 MiB WAL
// journal size limit (DSN pragmas, applied to each new physical
// connection); the process carries SQLite's advisory soft heap limit
// (256 MiB) and its hard heap limit (512 MiB) — library-wide settings,
// set once per open. temp_store stays at file: the observation reads sort
// without a temp B-tree instead (queryObservationsSQL), so a big read
// never spills a sort to TMPDIR.

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	// tenancyCacheSize is PRAGMA cache_size: negative is KiB, so 8 MiB per
	// connection.
	tenancyCacheSize int64 = -8192
	// tenancyJournalSizeLimit truncates the WAL back to 64 MiB after a
	// checkpoint.
	tenancyJournalSizeLimit int64 = 64 << 20
	// tenancySoftHeapLimit is SQLite's advisory heap limit (256 MiB).
	tenancySoftHeapLimit int64 = 256 << 20
	// tenancyHardHeapLimit is SQLite's hard heap limit (512 MiB): past it
	// an allocation fails with SQLITE_NOMEM instead of growing the daemon.
	tenancyHardHeapLimit int64 = 512 << 20
)

// tenancyConnPragmas are the per-connection pragmas, in DSN form.
var tenancyConnPragmas = []string{
	fmt.Sprintf("cache_size(%d)", tenancyCacheSize),
	fmt.Sprintf("journal_size_limit(%d)", tenancyJournalSizeLimit),
}

// applyTenancyPragmas sets the process-wide heap limits and, on db's
// current connections, the per-connection pragmas (an in-memory store has
// one connection and no DSN; a file store's DSN already carries them to
// every new connection).
func applyTenancyPragmas(ctx context.Context, db *sql.DB) error {
	for _, p := range []string{
		fmt.Sprintf("PRAGMA soft_heap_limit = %d", tenancySoftHeapLimit),
		fmt.Sprintf("PRAGMA hard_heap_limit = %d", tenancyHardHeapLimit),
		fmt.Sprintf("PRAGMA cache_size = %d", tenancyCacheSize),
		fmt.Sprintf("PRAGMA journal_size_limit = %d", tenancyJournalSizeLimit),
	} {
		rows, err := db.QueryContext(ctx, p)
		if err != nil {
			return fmt.Errorf("store: %s: %w", p, err)
		}
		_ = rows.Close()
	}
	return nil
}
