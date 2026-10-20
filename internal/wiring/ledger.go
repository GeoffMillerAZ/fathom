package wiring

// ledger.go — owned.json (state directory; legacy .fathom/owned.json read), tier 3's ownership map (design §3).
//
// There is nowhere to put a comment marker in JSON, and a YAML comment is
// too fragile to trust with someone's CI configuration. So for structured
// files fathom does not own, ownership is KEYED and the key is recorded out
// of band: fathom adds only entries whose identity it can state exactly,
// and records the set of them here at the same instant as the edit.
//
// Losing this file does not cause damage. It causes fathom to consider
// itself the owner of nothing, and to propose additions that already exist
// — which surfaces as a no-op, not as a duplicate. That is precisely the
// structural fix for the `fathom init` hook-duplication incident.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/statedir"
)

const (
	ledgerRelPath = projectdir.Dir + "/" + string(projectdir.Owned)
	ledgerVersion = 1
)

// OwnedKey is one entry fathom added to a structured file it does not own.
type OwnedKey struct {
	// ID is the key's stable address inside its file — the same string the
	// planner computes from the Key it is reconciling, so the ledger and the
	// spec can never disagree about what "this key" means.
	ID string `json:"id"`
	// Digest is the sha256 of the value AS FATHOM WROTE IT. A current value
	// that hashes to something else is drift: stop and report.
	Digest string `json:"digest"`
	// Wrote is the RFC3339 instant fathom last wrote this key, from the
	// caller's injected clock.
	Wrote string `json:"wrote"`
}

// Ledger is the whole of .fathom/owned.json: per file, the keys fathom
// added. A file absent from Files is a file fathom owns nothing in.
type Ledger struct {
	Version int                   `json:"version"`
	Files   map[string][]OwnedKey `json:"files"`
}

// NewLedger returns an empty ledger — fathom owning nothing, which is the
// correct starting assumption in a repository it has never edited.
func NewLedger() Ledger {
	return Ledger{Version: ledgerVersion, Files: map[string][]OwnedKey{}}
}

// OwnedKeys returns the keys fathom recorded for one file, sorted by ID.
// Everything else in that file is someone else's and is invisible to
// fathom: reconciliation touches only what this returns.
func OwnedKeys(l Ledger, file string) []OwnedKey {
	src := l.Files[file]
	out := make([]OwnedKey, len(src))
	copy(out, src)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ownedKey returns the ledger entry for one key, and whether fathom owns it
// at all.
func ownedKey(l Ledger, file, id string) (OwnedKey, bool) {
	for _, k := range l.Files[file] {
		if k.ID == id {
			return k, true
		}
	}
	return OwnedKey{}, false
}

// Record upserts one key. Recording is what makes a key fathom's; nothing
// else does, which is why every write path calls this in the same operation
// as the edit itself.
func (l *Ledger) Record(file string, k OwnedKey) {
	if l.Files == nil {
		l.Files = map[string][]OwnedKey{}
	}
	if l.Version == 0 {
		l.Version = ledgerVersion
	}
	keys := l.Files[file]
	for i := range keys {
		if keys[i].ID == k.ID {
			keys[i] = k
			l.Files[file] = keys
			return
		}
	}
	keys = append(keys, k)
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
	l.Files[file] = keys
}

// Forget drops one key: the entry it named is gone (a file fathom removed
// when it replaced its earlier rendering with a link, AG-8). A file left
// with no keys leaves the ledger.
func (l *Ledger) Forget(file, id string) {
	keys := l.Files[file]
	for i := range keys {
		if keys[i].ID == id {
			keys = append(keys[:i], keys[i+1:]...)
			break
		}
	}
	if len(keys) == 0 {
		delete(l.Files, file)
		return
	}
	l.Files[file] = keys
}

// ledgerPath is where the ledger is written: owned.json in the state
// directory (docs/design/fathom-dir.md M4).
func ledgerPath(root string) string {
	return filepath.Join(statedir.Resolve(root), string(projectdir.Owned))
}

// LedgerPath is where the ledger is written, for a caller that lists the
// files an apply touches.
func LedgerPath(root string) string { return ledgerPath(root) }

// legacyLedgerPath is the pre-M4 home, <root>/.fathom/owned.json. It is read
// while the state-directory copy is absent and never written or deleted
// (I63).
func legacyLedgerPath(root string) string { return projectdir.Owned.Path(root) }

// LedgerLegacyStale reports whether a legacy <root>/.fathom/owned.json sits
// beside a state-directory copy that now supersedes it, for doctor to name.
// fathom deletes neither.
func LedgerLegacyStale(root string) bool {
	if _, err := os.Stat(legacyLedgerPath(root)); err != nil {
		return false
	}
	_, err := os.Stat(ledgerPath(root))
	return err == nil
}

// LoadLedger reads owned.json from the state directory, falling back to the
// legacy <root>/.fathom/owned.json only when the new path is absent. An
// absent file is normal and yields an empty ledger with no error. An UNREADABLE or unparsable one
// yields an empty ledger AND an error: the caller must name the reason
// rather than silently proceeding as though fathom owned nothing, because
// "fathom owns nothing" and "fathom cannot tell what it owns" license the
// same conservative behaviour but deserve different words (law 3).
func LoadLedger(root string) (Ledger, error) {
	data, err := os.ReadFile(ledgerPath(root))
	if os.IsNotExist(err) {
		data, err = os.ReadFile(legacyLedgerPath(root))
	}
	if err != nil {
		if os.IsNotExist(err) {
			return NewLedger(), nil
		}
		return NewLedger(), fmt.Errorf("reading %s: %w", ledgerRelPath, err)
	}
	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return NewLedger(), fmt.Errorf("parsing %s: %w", ledgerRelPath, err)
	}
	if l.Files == nil {
		l.Files = map[string][]OwnedKey{}
	}
	return l, nil
}

// SaveLedger writes the ledger deterministically: map keys sorted by
// encoding/json, key lists sorted by Record, two-space indent, one trailing
// newline. Two runs over the same state produce byte-identical files.
func SaveLedger(root string, l Ledger) error {
	if l.Version == 0 {
		l.Version = ledgerVersion
	}
	if l.Files == nil {
		l.Files = map[string][]OwnedKey{}
	}
	for f := range l.Files {
		keys := l.Files[f]
		sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
		l.Files[f] = keys
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	p := ledgerPath(root)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}
