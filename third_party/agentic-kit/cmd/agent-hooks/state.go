package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// State retention: once-per-session markers and contained outputs older than
// this are pruned, at most once a day per directory.
const stateRetention = 7 * 24 * time.Hour

// stateDir resolves the state root: $AGENT_HOOKS_STATE, then
// $XDG_STATE_HOME/agent-hooks, then ~/.local/state/agent-hooks.
func (a *app) stateDir() string {
	if d := a.env("AGENT_HOOKS_STATE"); d != "" {
		return d
	}
	if x := a.env("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "agent-hooks")
	}
	return filepath.Join(a.home, ".local", "state", "agent-hooks")
}

// sessionKey maps a host session id to a fixed-length directory name, so a
// hostile or malformed id can never escape the state directory.
func sessionKey(session string) string {
	if session == "" {
		session = "no-session"
	}
	sum := sha256.Sum256([]byte(session))
	return hex.EncodeToString(sum[:8])
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_.-]`)

// safeName makes a value usable as one path element.
func safeName(s string) string {
	s = unsafeName.ReplaceAllString(s, "_")
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" || s == "." || s == ".." {
		return "_"
	}
	return s
}

// once reports whether key fires for the first time in this session. The
// marker is created with O_EXCL, so concurrent hook processes agree on a
// single winner. Any filesystem error yields false: advice is optional, and
// silence is the safe failure.
func (a *app) once(session, key string) bool {
	root := filepath.Join(a.stateDir(), "sessions")
	dir := filepath.Join(root, sessionKey(session))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	a.pruneDaily(root)
	f, err := os.OpenFile(filepath.Join(dir, safeName(key)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// saveOutput stores a contained tool output and returns its path.
func (a *app) saveOutput(session, toolUseID, text string) (string, error) {
	root := filepath.Join(a.stateDir(), "outputs")
	dir := filepath.Join(root, sessionKey(session))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	a.pruneDaily(root)
	name := a.now().UTC().Format("20060102T150405.000000000Z") + "-" + safeName(toolUseID) + ".txt"
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	if _, err = f.WriteString(text); err != nil {
		_ = f.Close()
		return "", err
	}
	return path, f.Close()
}

// pruneDaily removes per-session directories under root whose newest entry
// is older than stateRetention. A dated O_EXCL stamp limits it to one run
// per directory per day, so hooks on the hot path stay cheap.
//
// A directory is judged by its newest file, not its own timestamp, and is
// moved aside with an atomic rename before it is deleted. A hook writing into
// a session directory at that moment either sees the directory before the
// rename (and its fresh file makes the directory recent, so it is kept) or
// after it (and its write fails, so it passes the output through untouched).
// The remaining window, between reading the newest timestamp and the rename,
// is one rename long and needs a session idle for seven days to write in it.
func (a *app) pruneDaily(root string) {
	stampName := ".pruned-" + a.now().UTC().Format("20060102")
	f, err := os.OpenFile(filepath.Join(root, stampName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return // already pruned today, or unwritable
	}
	_ = f.Close()
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := a.now().Add(-stateRetention)
	for _, e := range entries {
		name, path := e.Name(), filepath.Join(root, e.Name())
		switch {
		case strings.HasPrefix(name, ".pruned-"):
			if name != stampName {
				_ = os.Remove(path) // an earlier day's stamp
			}
		case strings.HasPrefix(name, ".trash-"):
			_ = os.RemoveAll(path) // left by an interrupted prune
		case e.IsDir() && newestModTime(path).Before(cutoff):
			trash := filepath.Join(root, ".trash-"+name)
			if os.Rename(path, trash) == nil {
				_ = os.RemoveAll(trash)
			}
		}
	}
}

// newestModTime is the latest modification time of a directory and its
// direct entries.
func newestModTime(dir string) time.Time {
	var newest time.Time
	if info, err := os.Stat(dir); err == nil {
		newest = info.ModTime()
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

// isNotExist reports whether err means the path is absent.
func isNotExist(err error) bool { return errors.Is(err, fs.ErrNotExist) }
