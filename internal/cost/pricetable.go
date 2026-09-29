package cost

// pricetable.go — PE-16 (docs/design/progress-and-economics.md §5,
// "outcome economics"): a user-editable price table, read once from
// ~/.fathom/prices.toml, that prices a session's transcript-fold tokens
// (foldTranscriptCost, cmd/fathom, PE-13) into USD per proven row. No
// network call is ever made to price a token — the owner edits the file
// by hand; fathom never fetches or guesses a rate.

import (
	"fmt"
	"os"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
	toml "github.com/pelletier/go-toml/v2"
)

// PriceTable is the parsed contents of prices.toml: USD rates per model,
// per million tokens, by token kind. The zero value is a valid empty
// table (every model unpriced).
type PriceTable struct {
	Models map[string]ModelRate `toml:"models"`
}

// ModelRate is one model's USD rate per million tokens, split the same
// way transcriptCost (cmd/fathom) splits a session's tokens.
type ModelRate struct {
	InputPerMTok      float64 `toml:"input_per_mtok"`
	OutputPerMTok     float64 `toml:"output_per_mtok"`
	CacheReadPerMTok  float64 `toml:"cache_read_per_mtok"`
	CacheWritePerMTok float64 `toml:"cache_write_per_mtok"`
}

// PriceTablePath resolves the user's price table: $FATHOM_PRICE_TABLE,
// defaulting to ~/.fathom/prices.toml (mirrors sessionLedgerPath's own
// per-user, env-overridable convention in cmd/fathom).
func PriceTablePath() string {
	return fathomhome.PerFile("FATHOM_PRICE_TABLE", "prices.toml")
}

// loadPriceTable reads and parses path (typically PriceTablePath's
// return). A missing file, or an empty path, is not an error: the table
// is optional setup, and loadPriceTable returns an empty PriceTable
// (every model unpriced, USD's priced return false) rather than block the
// caller on a config problem. A file that exists but fails to parse is a
// named error — a mangled table must never silently price everything at a
// real-looking zero.
func loadPriceTable(path string) (PriceTable, error) {
	if path == "" {
		return PriceTable{Models: map[string]ModelRate{}}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PriceTable{Models: map[string]ModelRate{}}, nil
		}
		return PriceTable{}, fmt.Errorf("read %s: %w", path, err)
	}
	var pt PriceTable
	if err := toml.Unmarshal(data, &pt); err != nil {
		return PriceTable{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if pt.Models == nil {
		pt.Models = map[string]ModelRate{}
	}
	return pt, nil
}

// USD prices in/out/cache-read/cache-write token counts for model. priced
// is false — and usd is always 0 — when model has no entry in the table,
// so a caller can say "unpriced" instead of showing a $0.00 that looks
// like a real, computed zero (law 12).
func (pt PriceTable) USD(model string, in, out, cacheRead, cacheWrite int) (usd float64, priced bool) {
	r, ok := pt.Models[model]
	if !ok {
		return 0, false
	}
	usd = float64(in)/1e6*r.InputPerMTok +
		float64(out)/1e6*r.OutputPerMTok +
		float64(cacheRead)/1e6*r.CacheReadPerMTok +
		float64(cacheWrite)/1e6*r.CacheWritePerMTok
	return usd, true
}
