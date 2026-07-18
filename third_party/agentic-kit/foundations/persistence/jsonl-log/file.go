package jsonllog

import (
	"fmt"
	"os"
)

// File is an append-only JSONL log backed by an *os.File. It embeds a
// *Writer — use Append to add entries — and adds Close with sync-on-close.
//
// File inherits Writer's concurrency contract: Append and Close serialize on
// the same mutex, so a Close that wins the lock syncs everything previously
// appended, and any Append that loses the race fails with the underlying
// "file already closed" error rather than writing to a closed descriptor.
type File struct {
	*Writer
	f *os.File
}

// OpenFile opens the JSONL log at path for appending, creating it (mode
// 0o644) if it does not exist. The file is opened O_WRONLY|O_APPEND|O_CREATE:
// existing content is never truncated or rewritten — the log is append-only
// by construction, and reopening an existing log continues it.
func OpenFile(path string) (*File, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("jsonllog: open: %w", err)
	}
	return &File{Writer: NewWriter(f), f: f}, nil
}

// Close syncs the file and then closes it. Call Close exactly once; a second
// Close returns an error from the already-closed descriptor.
//
// Durability, honestly stated:
//
//   - Individual Appends are NOT individually synced — they reach the OS page
//     cache immediately but stable storage only when the OS decides to flush
//     or when Close runs. A crash (process or machine) before Close can lose
//     recently appended lines, and a machine crash can tear the last line
//     mid-write. ReadAll's partial-trailing-line recovery mode exists for
//     exactly that case.
//   - Close calls (*os.File).Sync before closing, which asks the OS to flush
//     the file to stable storage. What "stable" means is platform- and
//     hardware-dependent (drive write caches, virtualized storage); Sync is a
//     best-effort durability point, not an absolute guarantee.
//   - If Sync fails, Close still closes the descriptor and reports the sync
//     error (the close error is reported only if sync succeeded).
//
// Callers needing per-entry durability should call Close after each batch and
// reopen via OpenFile, or use an fsync-per-write store instead.
func (fl *File) Close() error {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	syncErr := fl.f.Sync()
	closeErr := fl.f.Close()
	if syncErr != nil {
		return fmt.Errorf("jsonllog: sync: %w", syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("jsonllog: close: %w", closeErr)
	}
	return nil
}
