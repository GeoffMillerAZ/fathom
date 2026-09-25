package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Pin and execute the same private copy. Hashing a mutable source and later
// executing its pathname would leave a verification/use race. This is provenance,
// not signing or isolation from another process running as the same OS user.
type preparedCompanion struct{ path, dir string }

func prepareCompanion(path, digest string) (*preparedCompanion, error) {
	fail := errors.New("native companion identity check failed; select an absolute executable and its exact lowercase SHA256")
	want, err := hex.DecodeString(digest)
	if err != nil || len(want) != sha256.Size || hex.EncodeToString(want) != digest || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fail
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 || info.Size() <= 0 || info.Size() > 256<<20 {
		return nil, fail
	}
	source, err := os.Open(path)
	if err != nil {
		return nil, fail
	}
	defer source.Close()
	actual, err := source.Stat()
	if err != nil || !os.SameFile(info, actual) {
		return nil, fail
	}
	dir, err := os.MkdirTemp("", "appkit-native-companion-")
	if err != nil {
		return nil, errors.New("could not stage native companion")
	}
	p := &preparedCompanion{dir: dir, path: filepath.Join(dir, "gallery-native")}
	keep := false
	defer func() {
		if !keep {
			p.close()
		}
	}()
	dest, err := os.OpenFile(p.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fail
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(dest, hash), io.LimitReader(source, (256<<20)+1))
	closeErr := dest.Close()
	if copyErr != nil || closeErr != nil || n != info.Size() || n > 256<<20 || hex.EncodeToString(hash.Sum(nil)) != digest {
		return nil, fail
	}
	if err := os.Chmod(p.path, 0500); err != nil {
		return nil, fail
	}
	keep = true
	return p, nil
}

func (p *preparedCompanion) close() {
	// Only these two exact paths created by this invocation; never recursive.
	_ = os.Remove(p.path)
	_ = os.Remove(p.dir)
}

// One outstanding request, one correctly typed reply. Unsolicited/duplicate
// replies cannot be queued for a later action even on this private channel.
type companionReplies struct {
	mu      sync.Mutex
	pending string
}

func (r *companionReplies) begin(kind string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending != "" || (kind != "launch-request" && kind != "browser") {
		return false
	}
	r.pending = kind
	return true
}

func (r *companionReplies) accept(kind string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	valid := (r.pending == "launch-request" && kind == "launch") ||
		(r.pending == "browser" && (kind == "browser-ok" || kind == "browser-failed"))
	if valid {
		r.pending = ""
	}
	return valid
}
