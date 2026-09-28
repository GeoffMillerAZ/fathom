package docindex

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/store"
)

// TestDocIndexBudgetNamesElision pins KR-9 (docs/design/knowledge-retrieval.md
// §8): a service's candidate doc_units over the byte budget are cut at a
// deterministic prefix boundary, exactly like mcp/budget.go's own
// fitToolBudget convention — the first candidate is always kept even alone
// over budget ("never refuses": a budget is a target, not a hard wall) —
// and everything elided is named by path and heading path, never a silent
// drop (I38). A stale_by_location candidate is elided before any live one,
// even when it would otherwise sort first (§3: stale ranks below live,
// never silently above it).
func TestDocIndexBudgetNamesElision(t *testing.T) {
	mk := func(path, heading, body string, stale bool) store.DocUnit {
		return store.DocUnit{
			UnitID:          path + "#" + heading,
			Path:            path,
			HeadingPath:     heading,
			Body:            body,
			StaleByLocation: stale,
		}
	}

	candidates := []store.DocUnit{
		mk("z.md", "Z", "0123456789", false),            // 11 bytes (heading 1 + body 10)
		mk("a.md", "A", "0123456789", false),            // 11 bytes
		mk("archive/old.md", "Old", "0123456789", true), // 13 bytes, stale
		mk("m.md", "M", "0123456789", false),            // 11 bytes
	}

	// Budget fits exactly the two live docs sorted first (a.md, m.md) at
	// 11 bytes each = 22; z.md would push to 33 > 30, so z.md and the
	// stale unit are elided.
	res := enforceDocBudget(candidates, 30)

	if len(res.Kept) != 2 {
		t.Fatalf("Kept = %+v, want 2 units", res.Kept)
	}
	if res.Kept[0].Path != "a.md" || res.Kept[1].Path != "m.md" {
		t.Fatalf("Kept order = %v, %v, want a.md, m.md (live docs, path order)", res.Kept[0].Path, res.Kept[1].Path)
	}
	if res.KeptBytes != 22 {
		t.Fatalf("KeptBytes = %d, want 22", res.KeptBytes)
	}

	if len(res.Elided) != 2 {
		t.Fatalf("Elided = %+v, want 2 units named", res.Elided)
	}
	elidedPaths := map[string]bool{}
	for _, e := range res.Elided {
		elidedPaths[e.Path] = true
		if e.Bytes <= 0 {
			t.Errorf("elided %s: Bytes = %d, want > 0 (named, not a bare drop)", e.Path, e.Bytes)
		}
	}
	if !elidedPaths["z.md"] || !elidedPaths["archive/old.md"] {
		t.Fatalf("Elided = %+v, want z.md and archive/old.md named", res.Elided)
	}
	if res.ElidedBytes != 11+13 {
		t.Fatalf("ElidedBytes = %d, want %d", res.ElidedBytes, 11+13)
	}

	// Never refuses: a single candidate alone over budget is still kept.
	huge := []store.DocUnit{mk("big.md", "Big", "0123456789012345678901234567890123456789", false)}
	res2 := enforceDocBudget(huge, 5)
	if len(res2.Kept) != 1 || len(res2.Elided) != 0 {
		t.Fatalf("enforceDocBudget(huge, 5) = %+v, want the sole candidate kept (never refuses)", res2)
	}

	// budgetBytes <= 0 uses DefaultDocBudgetBytes (mcp/budget.go's own
	// "non-positive means default" convention) — nothing elided for a
	// small candidate set.
	res3 := enforceDocBudget(candidates, 0)
	if len(res3.Elided) != 0 {
		t.Fatalf("enforceDocBudget(candidates, 0) = %+v, want the default budget to fit everything", res3)
	}
}
