// Package scope implements phase V's REVERSIBLE half: suppression — hiding
// indexed content from answers without deleting it (docs/design/
// scope-and-purge.md §2). Its irreversible twin, purge, lives in
// internal/store (PurgePaths) and cmd/fathom (`fathom purge`), because the
// two operations have opposite risk profiles and the design forbids
// conflating them (§1).
//
// Four properties are load-bearing and every change here must preserve them:
//
//   - **The filter lives in the store's own read methods.** Every fathom
//     surface — MCP tool, CLI verb, HTTP route, the observatory's counts,
//     the cross-repo platform store — reaches indexed content through the
//     store backends, whose content-bearing reads all consult a Source per
//     call. There is no registration seam, no init() arming, and no
//     decorator a constructor can forget to apply: a store constructed by
//     ANY exported constructor filters, which internal/store's
//     constructor audit proves by enumerating the constructors from the
//     package source itself.
//   - **State is re-read per query.** A Source stat()s the suppression
//     file on every call and reloads on change, so a long-lived handle
//     (the daemon holds one store for its whole life) sees a new
//     suppression on the NEXT query — not on the next restart.
//   - **Never silent.** A read that dropped rows always yields a
//     Disclosure carrying the count and the governing reason. Law 3
//     applied to results: an absence is named, never merely absent.
//   - **Read-time expiry, no sweep.** A suppression's state is a pure
//     function of the record and the clock, evaluated on every read, so a
//     suppression that expires while the daemon is stopped is simply
//     inactive on the next query and no timer can be missed.
//
// Suppression NEVER changes what gets indexed. It is a query-time decision
// about this machine's index (law 13), stored in the file
// $FATHOM_SUPPRESSIONS points at (the fathom binary defaults it to
// ~/.fathom/suppressions.jsonl in main — never in a package init, so
// `go test` over any package stays hermetic and machine-independent),
// never in the repository and never shared. Purge is the operation that
// also writes an ingest-time exclusion (§3.5, service-scoped); suppression
// must not, or a reversible control would quietly become an irreversible
// one.
package scope

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
)

// EnvVar names the environment variable that carries the suppression
// file's path. This is the ONLY channel the store layer reads suppression
// state through (the FATHOM_SESSION_LEDGER pattern): unset means "no
// suppressions", which keeps every test and every library consumer
// hermetic by default. The fathom binary's main() defaults it to
// DefaultUserPath, so the CLI and the daemon are always armed.
const EnvVar = "FATHOM_SUPPRESSIONS"

// SuppressionsFileName is the user-scope suppression log's basename under
// ~/.fathom/. JSON Lines, append-only: a removal is a tombstone record, not
// an erasure, so "what was hidden, when, and why" survives (§2.3).
const SuppressionsFileName = "suppressions.jsonl"

// Origin values for Suppression.CreatedBy — the provenance §2.1 requires.
const (
	OriginUI     = "ui"
	OriginCLI    = "cli"
	OriginConfig = "config"
	OriginPurge  = "purge"
)

// State is a suppression's read-time verdict.
type State string

const (
	// StateActive: the suppression is filtering right now.
	StateActive State = "active"
	// StateExpired: its --until passed. Retained as history (§2.3).
	StateExpired State = "expired"
	// StateRemoved: explicitly unsuppressed. Retained as history.
	StateRemoved State = "removed"
)

// Suppression is one reversible hide rule (§2.1). It is stored as one JSON
// object per line; a later line carrying the same ID supersedes the earlier
// one, which is how RemoveSuppression records a tombstone without rewriting
// history.
type Suppression struct {
	// ID is stable and content-derived (see suppressionID), so the same
	// suppression written twice is the same row and the file is
	// reproducible.
	ID string `json:"id"`
	// Service scopes the rule; "" means fleet-wide.
	Service string `json:"service,omitempty"`
	// Pattern is gitignore-syntax, matched against a row's own path column.
	Pattern string `json:"pattern"`
	// Reason is REQUIRED. §2.1: "a suppression with no reason is a mystery
	// in three weeks."
	Reason    string     `json:"reason"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// RemovedAt marks the tombstone written by `fathom scope unsuppress`.
	RemovedAt *time.Time `json:"removed_at,omitempty"`
	CreatedBy string     `json:"created_by"`
}

// PathFromEnv returns the suppression file path the environment declares,
// or "" when none is declared. "" means the read filter is DISARMED — the
// hermetic default for tests and library consumers (the V6 rework: state
// must resolve through an injectable path, never through an init()-armed
// global that reads the developer's real home directory).
func PathFromEnv() string { return os.Getenv(EnvVar) }

// DefaultUserPath returns ~/.fathom/suppressions.jsonl, or "" when $HOME
// cannot be resolved. main() uses it to default EnvVar for the real
// binary; nothing else calls it implicitly.
func DefaultUserPath() string {
	return fathomhome.Path(SuppressionsFileName)
}

// SuppressionsPathIn returns home's suppression log path. Split from
// DefaultUserPath so tests (and any caller with an explicit home) never
// depend on process state.
func SuppressionsPathIn(home string) string {
	return filepath.Join(home, ".fathom", SuppressionsFileName)
}

// ResolvePath is the CLI verbs' resolution (the FATHOM_SESSION_LEDGER
// pattern): the environment override when set, else the user-scope
// default. Returns "" only when neither resolves.
func ResolvePath() string {
	if p := PathFromEnv(); p != "" {
		return p
	}
	return DefaultUserPath()
}

// suppressionID derives the stable ID from the fields that define the rule.
// Content-derived rather than random so the file is reproducible and a
// test can assert an exact ID.
func suppressionID(s Suppression) string {
	h := sha256.New()
	for _, part := range []string{s.Service, s.Pattern, s.Reason, s.CreatedAt.UTC().Format(time.RFC3339Nano)} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// validate enforces the two fields §2.1 makes mandatory. A pattern may only
// ever HIDE: a gitignore re-include ("!x") would let a suppression file
// widen what is visible, which is not what the verb means.
func (s Suppression) validate() error {
	if strings.TrimSpace(s.Pattern) == "" {
		return fmt.Errorf("scope: a suppression needs a pattern")
	}
	if strings.HasPrefix(strings.TrimSpace(s.Pattern), "!") {
		return fmt.Errorf("scope: %q is a re-include pattern — a suppression may only ever hide, never reveal", s.Pattern)
	}
	if strings.TrimSpace(s.Reason) == "" {
		return fmt.Errorf("scope: a suppression needs a reason (a suppression with no reason is a mystery in three weeks)")
	}
	return nil
}

// AppendSuppression validates s, fills in its ID when empty, and appends it
// as one JSON line to path (creating the parent dir if needed). It returns
// the ID. A refused suppression writes nothing at all — not even the file.
func AppendSuppression(path string, s Suppression) (string, error) {
	if path == "" {
		return "", fmt.Errorf("scope: no suppression file path (set %s or $HOME)", EnvVar)
	}
	if err := s.validate(); err != nil {
		return "", err
	}
	if s.CreatedBy == "" {
		s.CreatedBy = OriginCLI
	}
	if s.ID == "" {
		s.ID = suppressionID(s)
	}
	if err := appendRecord(path, s); err != nil {
		return "", err
	}
	return s.ID, nil
}

// RemoveSuppression appends a tombstone for id: the same record with
// RemovedAt set. It refuses, naming the id, when no such suppression exists
// — a silent no-op here would leave the operator believing content was
// un-hidden when it was not.
func RemoveSuppression(path, id string, at time.Time) error {
	all, err := LoadSuppressions(path)
	if err != nil {
		return err
	}
	for _, s := range all {
		if s.ID != id {
			continue
		}
		if s.RemovedAt != nil {
			return fmt.Errorf("scope: suppression %q was already removed at %s", id, s.RemovedAt.UTC().Format(time.RFC3339))
		}
		removed := at.UTC()
		s.RemovedAt = &removed
		return appendRecord(path, s)
	}
	return fmt.Errorf("scope: no suppression with id %q (run `fathom scope list` to see the ids)", id)
}

func appendRecord(path string, s Suppression) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("scope: mkdir %q: %w", dir, err)
		}
	}
	line, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("scope: marshal suppression: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("scope: open %q: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("scope: append to %q: %w", path, err)
	}
	return nil
}

// LoadSuppressions reads path, folds records by ID (last line wins, which
// is how a tombstone supersedes its original) and returns them sorted by
// ID. A missing file is not an error — it is the overwhelmingly common
// "nothing is suppressed on this machine" case and returns nil. An empty
// path likewise returns nil: the filter is disarmed, not broken.
//
// This is a pure READ: it never rewrites the file, never prunes expired
// entries and never consults a clock. Expiry is SuppressionState's
// judgement, made against the caller's injected time on every read (§2.3) —
// which is what SP-1's "evaluated at read time with no sweep" means: a
// suppression that expires while the daemon is down is inactive on the
// next query with nothing having run in between.
func LoadSuppressions(path string) ([]Suppression, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scope: open %q: %w", path, err)
	}
	defer f.Close()

	byID := map[string]Suppression{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		var s Suppression
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("scope: %s:%d: %w", path, line, err)
		}
		if s.ID == "" {
			s.ID = suppressionID(s)
		}
		byID[s.ID] = s
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scope: read %q: %w", path, err)
	}

	out := make([]Suppression, 0, len(byID))
	for _, s := range byID {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// SuppressionState is the read-time verdict for s at now. Pure: same
// record, same clock, same answer — no sweep, no daemon timer, nothing
// that can be missed while the daemon is down.
func SuppressionState(s Suppression, now time.Time) State {
	if s.RemovedAt != nil && !now.Before(*s.RemovedAt) {
		return StateRemoved
	}
	if s.ExpiresAt != nil && !now.Before(*s.ExpiresAt) {
		return StateExpired
	}
	return StateActive
}

// ActiveSuppressions filters all down to those in StateActive at now,
// preserving the input's (ID) order.
func ActiveSuppressions(all []Suppression, now time.Time) []Suppression {
	var out []Suppression
	for _, s := range all {
		if SuppressionState(s, now) == StateActive {
			out = append(out, s)
		}
	}
	return out
}
