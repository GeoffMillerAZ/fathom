package store

// sqlitemem.go — SQLite's own allocator reading (RT-124, design §13.3):
// fathom_sqlite_memory_used_bytes. SQLite's page cache and statement
// memory live outside the Go heap's accounting (modernc.org/sqlite
// allocates through its libc), so /metrics reads sqlite3_memory_used
// directly. Pure Go: modernc's transpiled library, no cgo.

import (
	"modernc.org/libc"
	sqlite3 "modernc.org/sqlite/lib"
)

// SQLiteMemoryUsed is the bytes SQLite's allocator holds now,
// process-wide; ok is false when SQLite is not counting. modernc.org/sqlite
// v1.52 is generated with SQLITE_DEFAULT_MEMSTATUS=0 (and SQLITE_MUTEX_NOOP,
// so turning the status counters on at init would count racily across
// connections): memory_used then reads 0, which is "not counted", never
// "no memory" (I38). A real reading is never 0 — SQLite holds its own
// structures from the first open.
func SQLiteMemoryUsed() (used int64, ok bool) {
	tls := libc.NewTLS()
	defer tls.Close()
	v := int64(sqlite3.Xsqlite3_memory_used(tls))
	return v, v > 0
}
