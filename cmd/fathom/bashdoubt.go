package main

// bashdoubt.go — EW-1 (O122): the once-per-session-per-directory memory of
// the Bash-write hook's doubt line. A write-shaped Bash command whose target
// the parser could not name used to serve the cwd's nearest guide (six
// unrelated root rules) on EVERY such call; it now serves one line, and only
// the first time a session asks from a given directory. This marker is what
// remembers that it did.
//
// Mechanism mirrors editset.go and guardstamp.go exactly: os.TempDir() (never
// a repo artifact), a 0700 directory, a 0600 file created O_EXCL|O_NOFOLLOW,
// and an owner / permission / regular-file check on the OPEN FD when reading.
// The key is sha256(session, directory), so a session with no id has no
// marker at all: a predictable key with no session scope would let anyone who
// can write to a shared temp dir plant a marker and silence the line. With no
// session id the hook simply serves its one line each time, which costs a
// line, never correctness. gc prunes the entries after a day
// (gcTmpCacheDirs).

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// bashDoubtTTL bounds how long a marker is honored: a session that outlives
// it hears the line once more, which is the cheap failure. It matches gc's
// retention (gcTmpAge) so a marker never outlives its own cleanup.
const bashDoubtTTL = 24 * time.Hour

// bashDoubtDir is os.TempDir()/fathom-bash-doubt — a package var (not a
// const) so tests can point it at a t.TempDir().
var bashDoubtDir = filepath.Join(os.TempDir(), "fathom-bash-doubt")

func bashDoubtPath(sessionID, dir string) string {
	h := sha256.Sum256([]byte(sessionID + "\x1f" + dir))
	return filepath.Join(bashDoubtDir, hex.EncodeToString(h[:16])+".stamp")
}

// markBashDoubtServed records that sessionID was told the doubt line for dir.
// A no-op with no session id; every failure is swallowed (hook doctrine).
func markBashDoubtServed(sessionID, dir string, now time.Time) {
	if sessionID == "" {
		return
	}
	if err := os.MkdirAll(bashDoubtDir, 0o700); err != nil {
		return
	}
	path := bashDoubtPath(sessionID, dir)
	_ = os.Remove(path) // never write through a pre-existing entry
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write([]byte(now.UTC().Format(time.RFC3339)))
}

// bashDoubtServed reports whether sessionID was already told the doubt line
// for dir within bashDoubtTTL. false on any absence, corruption, expiry,
// ownership mismatch or missing session id: the fail-toward-serving default.
func bashDoubtServed(sessionID, dir string, now time.Time) bool {
	if sessionID == "" {
		return false
	}
	f, err := os.OpenFile(bashDoubtPath(sessionID, dir), os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return false
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(f, 64))
	if err != nil {
		return false
	}
	writtenAt, err := time.Parse(time.RFC3339, string(data))
	return err == nil && now.Sub(writtenAt) <= bashDoubtTTL
}
