package cost

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPriceTableNoNetwork (PE-16, docs/design/progress-and-economics.md
// §5): loadPriceTable parses a user-editable ~/.fathom/prices.toml with no
// network call — proven structurally (this package imports nothing from
// net, so no dial or DNS lookup can be written here without failing the
// test; a wall-clock bound would flip under -race on a loaded machine and
// would not prove the absence anyway) — and prices tokens by model; an
// unpriced model is named, never a guessed rate.
func TestPriceTableNoNetwork(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("read package imports: %v", err)
	}
	if len(pkg.Imports) == 0 {
		t.Fatal("no imports found: the no-network check would prove nothing")
	}
	for _, imp := range pkg.Imports {
		if imp == "net" || strings.HasPrefix(imp, "net/") {
			t.Errorf("internal/cost imports %q: the price table must never make a network call (PE-16)", imp)
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "prices.toml")
	body := `
[models.claude-sonnet-5]
input_per_mtok = 3.0
output_per_mtok = 15.0
cache_read_per_mtok = 0.3
cache_write_per_mtok = 3.75
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	pt, err := loadPriceTable(path)
	if err != nil {
		t.Fatalf("loadPriceTable: %v", err)
	}
	if len(pt.Models) != 1 {
		t.Fatalf("Models = %d entries, want 1: %+v", len(pt.Models), pt.Models)
	}

	usd, priced := pt.USD("claude-sonnet-5", 1_000_000, 1_000_000, 1_000_000, 1_000_000)
	if !priced {
		t.Fatal("claude-sonnet-5 should be priced")
	}
	if wantUSD := 3.0 + 15.0 + 0.3 + 3.75; usd != wantUSD {
		t.Fatalf("USD = %v, want %v", usd, wantUSD)
	}
	if _, priced := pt.USD("some-unpriced-model", 100, 100, 0, 0); priced {
		t.Fatal("an unpriced model must report priced=false, never a guessed rate")
	}

	// A missing file is not an error: the price table is optional, every
	// model unpriced until the user edits it in.
	missing, err := loadPriceTable(filepath.Join(dir, "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("missing price table: %v", err)
	}
	if len(missing.Models) != 0 {
		t.Fatalf("missing file should yield an empty table, got %+v", missing.Models)
	}

	// An empty path (e.g. home dir unresolved) also yields an empty table,
	// not an error — never blocks the caller on a config problem.
	if empty, err := loadPriceTable(""); err != nil || len(empty.Models) != 0 {
		t.Fatalf("empty path: got (%+v, %v), want empty table, nil error", empty, err)
	}

	// Malformed TOML is a named error, not a silently empty table — a
	// mangled price table must not price everything at 0 looking like a
	// real, user-chosen zero (law 12).
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("not [ valid toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPriceTable(bad); err == nil {
		t.Fatal("malformed TOML should error, not silently load empty")
	}
}

func TestPriceTablePathEnvOverride(t *testing.T) {
	t.Setenv("FATHOM_PRICE_TABLE", "/tmp/custom-prices.toml")
	if got := PriceTablePath(); got != "/tmp/custom-prices.toml" {
		t.Fatalf("PriceTablePath() = %q, want the env override", got)
	}
}
