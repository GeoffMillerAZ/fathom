package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// scopeHome isolates every user-scope file phase V touches — suppressions,
// the per-service ignore directory, the purge log — behind their env
// overrides plus a temp HOME, so no test can read or write the
// developer's real ~/.fathom (V6: the gate must be machine-independent).
func scopeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(scope.EnvVar, scope.SuppressionsPathIn(home))
	t.Setenv(connector.ServiceIgnoreDirEnvVar, filepath.Join(home, ".fathom", "ignore.d"))
	t.Setenv("FATHOM_PURGES", filepath.Join(home, ".fathom", "purges.jsonl"))
	return home
}

// seedScopeStore writes a small two-path corpus and returns the store path.
func seedScopeStore(t *testing.T, svc string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fathom.sqlite")
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer st.Close()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	var obs []core.Observation
	// vendor/a/one.go deliberately carries TWO symbols: the file count and
	// the symbol count must differ, or a preview that reported symbols
	// where it promised files would pass unnoticed.
	for _, ref := range []string{"vendor/a/one.go", "vendor/a/one.go", "vendor/a/two.go", "vendor/b/three.go", "internal/keep.go"} {
		obs = append(obs, core.Observation{
			ServiceID: svc, SourceRef: ref, SourceKind: core.KindGoFunc,
			EvidenceTier: core.TierVerified, Symbol: fmt.Sprintf("S%s%d", filepath.Base(ref), len(obs)),
			Package: filepath.Dir(ref), Claim: "func S()", IngestedAt: now,
		})
		if err := st.SetFingerprint(context.Background(), svc, ref, "h"); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	if err := st.PutObservations(context.Background(), obs); err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	return path
}

// TestCmdScopePreviewMatchesEffect is SP-4: `fathom scope preview` exists
// precisely because a glob that matches more than you think is the main way
// this feature hurts someone (§2.4), so its count must equal EXACTLY what
// suppressing then hides — not approximately, not "up to".
func TestCmdScopePreviewMatchesEffect(t *testing.T) {
	scopeHome(t)
	const svc = "svc"
	storePath := seedScopeStore(t, svc)

	// An overlapping suppression is ALREADY in force. Preview must read
	// past it — it answers "what would be hidden", and a preview computed
	// through the filter under-reports exactly the paths already hidden.
	var out, errOut bytes.Buffer
	if rc := run([]string{"scope", "suppress", "vendor/a/**", "--service", svc, "--reason", "an earlier, narrower call"}, &out, &errOut); rc != 0 {
		t.Fatalf("pre-suppress rc=%d stderr=%s", rc, errOut.String())
	}

	out.Reset()
	errOut.Reset()
	if rc := run([]string{"scope", "preview", "vendor/**", "--service", svc, "--store", storePath, "--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("scope preview rc=%d stderr=%s", rc, errOut.String())
	}
	var preview struct {
		Pattern string   `json:"pattern"`
		Files   []string `json:"files"`
		Count   int      `json:"count"`
		Symbols int      `json:"symbols"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("preview json: %v (%s)", err, out.String())
	}
	if preview.Count != 3 {
		t.Fatalf("preview count = %d, want 3 files (%v)", preview.Count, preview.Files)
	}
	if preview.Symbols != 4 {
		t.Fatalf("preview symbols = %d, want 4 — count and symbols are different questions", preview.Symbols)
	}
	// Deterministic: sorted file list.
	for i := 1; i < len(preview.Files); i++ {
		if preview.Files[i-1] >= preview.Files[i] {
			t.Fatalf("preview file list is not sorted: %v", preview.Files)
		}
	}

	// Now suppress the same glob and count what actually disappears.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{"scope", "suppress", "vendor/**", "--service", svc, "--reason", "third-party corpus"}, &out, &errOut); rc != 0 {
		t.Fatalf("scope suppress rc=%d stderr=%s", rc, errOut.String())
	}

	before := 5 // total observation rows seeded (one per entry above)
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()
	got, err := st.QueryObservations(context.Background(), svc, "")
	if err != nil {
		t.Fatal(err)
	}
	hiddenSymbols := before - len(got)
	if hiddenSymbols != preview.Symbols {
		t.Fatalf("preview promised %d hidden symbols, suppress actually hid %d", preview.Symbols, hiddenSymbols)
	}
	for _, o := range got {
		if strings.HasPrefix(o.SourceRef, "vendor/") {
			t.Fatalf("a suppressed ref is still served: %s", o.SourceRef)
		}
	}
}

// TestCmdScopeListAndUnsuppress covers the rest of SP-4's verb set: list
// renders state and reason, unsuppress reverses instantly, and the history
// survives.
func TestCmdScopeListAndUnsuppress(t *testing.T) {
	home := scopeHome(t)
	const svc = "svc"

	var out, errOut bytes.Buffer
	if rc := run([]string{"scope", "suppress", "vendor/**", "--service", svc, "--reason", "third-party corpus", "--until", "7d"}, &out, &errOut); rc != 0 {
		t.Fatalf("suppress rc=%d stderr=%s", rc, errOut.String())
	}

	out.Reset()
	errOut.Reset()
	if rc := run([]string{"scope", "list", "--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("list rc=%d stderr=%s", rc, errOut.String())
	}
	var listed []struct {
		ID      string `json:"id"`
		Pattern string `json:"pattern"`
		Reason  string `json:"reason"`
		State   string `json:"state"`
		Expires string `json:"expires_at"`
	}
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatalf("list json: %v (%s)", err, out.String())
	}
	if len(listed) != 1 {
		t.Fatalf("list returned %d entries: %s", len(listed), out.String())
	}
	if listed[0].State != string(scope.StateActive) {
		t.Errorf("state = %q, want active", listed[0].State)
	}
	if listed[0].Reason == "" || listed[0].Expires == "" {
		t.Errorf("list dropped the reason or the expiry: %+v", listed[0])
	}
	id := listed[0].ID

	// A suppression with no reason is refused by the verb, not stored.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{"scope", "suppress", "x/**"}, &out, &errOut); rc == 0 {
		t.Fatal("scope suppress accepted a pattern with no --reason")
	}

	out.Reset()
	errOut.Reset()
	if rc := run([]string{"scope", "unsuppress", id}, &out, &errOut); rc != 0 {
		t.Fatalf("unsuppress rc=%d stderr=%s", rc, errOut.String())
	}
	all, err := scope.LoadSuppressions(scope.SuppressionsPathIn(home))
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("unsuppress destroyed the history: %d entries", len(all))
	}
	if got := scope.SuppressionState(all[0], time.Now()); got != scope.StateRemoved {
		t.Fatalf("state after unsuppress = %q", got)
	}

	// Unsuppressing an unknown id is a named refusal.
	out.Reset()
	errOut.Reset()
	if rc := run([]string{"scope", "unsuppress", "deadbeef"}, &out, &errOut); rc == 0 {
		t.Fatal("unsuppress accepted an unknown id")
	}
	if !strings.Contains(errOut.String(), "deadbeef") {
		t.Errorf("the refusal does not name the id: %s", errOut.String())
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
