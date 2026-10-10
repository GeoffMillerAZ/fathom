package certify

// port_test.go — TC-20's own test: TestPortGaps_ImplementationsWithoutConformance.
import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

const testPortService = "svc"

// seedPortFixture builds a service whose corpus root (on disk) carries
// real Go source for:
//   - Ledger (exported interface, "billing/ports.go"), implemented by
//     Wallet ("billing/wallet.go") via a compile-time `var _ Ledger =
//     (*Wallet)(nil)` assertion — "assertion" tier.
//   - Voucher (exported interface, "billing/coupon.go"), implemented by
//     Paper ("billing/paper.go") with NO assertion anywhere — "method_set"
//     tier, the fallback.
//   - Orphan (exported interface, "billing/orphan.go") with NO indexed
//     implementer at all — excluded from PortGaps entirely.
//   - hiddenIface (UNEXPORTED interface, "billing/hidden.go") with an
//     implementer — excluded because it is not exported.
//
// The store carries the matching indexed symbols directly (PutObservations,
// the same hermetic pattern seedResolveFixture/seedCompileFixture already
// use) rather than running a real `ingest` — PortGaps reads the interface's
// and each candidate's OWN source text from corpusRoot for method names and
// assertion evidence, so the files on disk still have to be real Go.
func seedPortFixture(t *testing.T) (context.Context, store.Store, string) {
	t.Helper()
	ctx := context.Background()
	st := store.NewMemoryStore()
	t.Cleanup(func() { _ = st.Close() })
	root := t.TempDir()

	mustWrite(t, filepath.Join(root, "billing", "ports.go"), "package billing\n\n"+
		"type Ledger interface {\n\tDebit(amount int) error\n\tCredit(amount int) error\n}\n")
	mustWrite(t, filepath.Join(root, "billing", "wallet.go"), "package billing\n\n"+
		"var _ Ledger = (*Wallet)(nil)\n\n"+
		"type Wallet struct{}\n\n"+
		"func (w *Wallet) Debit(amount int) error  { return nil }\n"+
		"func (w *Wallet) Credit(amount int) error { return nil }\n")
	mustWrite(t, filepath.Join(root, "billing", "coupon.go"), "package billing\n\n"+
		"type Voucher interface {\n\tRedeem() error\n}\n")
	mustWrite(t, filepath.Join(root, "billing", "paper.go"), "package billing\n\n"+
		"type Paper struct{}\n\n"+
		"func (p Paper) Redeem() error { return nil }\n")
	mustWrite(t, filepath.Join(root, "billing", "orphan.go"), "package billing\n\n"+
		"type Orphan interface {\n\tNope()\n}\n")
	mustWrite(t, filepath.Join(root, "billing", "hidden.go"), "package billing\n\n"+
		"type hiddenIface interface {\n\tSecret()\n}\n\n"+
		"type Keeper struct{}\n\n"+
		"func (k Keeper) Secret() {}\n")

	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: testPortService, Path: root}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}

	obs := []core.Observation{
		{ServiceID: testPortService, SourceRef: "billing/ports.go", SourceKind: core.KindGoInterface, EvidenceTier: core.TierVerified, Symbol: "Ledger", Package: "billing", Line: 3},
		{ServiceID: testPortService, SourceRef: "billing/wallet.go", SourceKind: core.KindGoType, EvidenceTier: core.TierVerified, Symbol: "Wallet", Package: "billing", Line: 5},
		{ServiceID: testPortService, SourceRef: "billing/wallet.go", SourceKind: core.KindGoMethod, EvidenceTier: core.TierVerified, Symbol: "Wallet.Debit", Package: "billing", Line: 7},
		{ServiceID: testPortService, SourceRef: "billing/wallet.go", SourceKind: core.KindGoMethod, EvidenceTier: core.TierVerified, Symbol: "Wallet.Credit", Package: "billing", Line: 8},

		{ServiceID: testPortService, SourceRef: "billing/coupon.go", SourceKind: core.KindGoInterface, EvidenceTier: core.TierVerified, Symbol: "Voucher", Package: "billing", Line: 3},
		{ServiceID: testPortService, SourceRef: "billing/paper.go", SourceKind: core.KindGoType, EvidenceTier: core.TierVerified, Symbol: "Paper", Package: "billing", Line: 3},
		{ServiceID: testPortService, SourceRef: "billing/paper.go", SourceKind: core.KindGoMethod, EvidenceTier: core.TierVerified, Symbol: "Paper.Redeem", Package: "billing", Line: 5},

		{ServiceID: testPortService, SourceRef: "billing/orphan.go", SourceKind: core.KindGoInterface, EvidenceTier: core.TierVerified, Symbol: "Orphan", Package: "billing", Line: 3},

		{ServiceID: testPortService, SourceRef: "billing/hidden.go", SourceKind: core.KindGoInterface, EvidenceTier: core.TierVerified, Symbol: "hiddenIface", Package: "billing", Line: 3},
		{ServiceID: testPortService, SourceRef: "billing/hidden.go", SourceKind: core.KindGoType, EvidenceTier: core.TierVerified, Symbol: "Keeper", Package: "billing", Line: 5},
		{ServiceID: testPortService, SourceRef: "billing/hidden.go", SourceKind: core.KindGoMethod, EvidenceTier: core.TierVerified, Symbol: "Keeper.Secret", Package: "billing", Line: 7},
	}
	for i := range obs {
		obs[i].IngestedAt = time.Now()
	}
	if err := st.PutObservations(ctx, obs); err != nil {
		t.Fatalf("seed PutObservations: %v", err)
	}
	return ctx, st, root
}

func TestPortGaps_ImplementationsWithoutConformance(t *testing.T) {
	t.Run("finds implementers, evidenced assertion > method_set, excludes zero-impl and unexported interfaces", func(t *testing.T) {
		ctx, st, root := seedPortFixture(t)

		targets, err := PortGaps(ctx, st, testPortService, root)
		if err != nil {
			t.Fatalf("PortGaps: %v", err)
		}
		if len(targets) != 2 {
			t.Fatalf("targets = %+v, want exactly 2 (Ledger, Voucher — Orphan has no implementer, hiddenIface is unexported)", targets)
		}
		if targets[0].ID != "Ledger" || targets[1].ID != "Voucher" {
			t.Fatalf("target IDs = [%s, %s], want [Ledger, Voucher] (sorted)", targets[0].ID, targets[1].ID)
		}

		ledger := targets[0]
		if len(ledger.Implementations) != 1 || ledger.Implementations[0].Name != "Wallet" {
			t.Fatalf("Ledger implementations = %+v, want [Wallet]", ledger.Implementations)
		}
		if ledger.Implementations[0].Via != "assertion" {
			t.Errorf("Ledger/Wallet Via = %q, want assertion (a `var _ Ledger = (*Wallet)(nil)` line is present)", ledger.Implementations[0].Via)
		}

		voucher := targets[1]
		if len(voucher.Implementations) != 1 || voucher.Implementations[0].Name != "Paper" {
			t.Fatalf("Voucher implementations = %+v, want [Paper]", voucher.Implementations)
		}
		if voucher.Implementations[0].Via != "method_set" {
			t.Errorf("Voucher/Paper Via = %q, want method_set (no assertion, no typed edge — structural fallback)", voucher.Implementations[0].Via)
		}
	})

	t.Run("typed edge evidence outranks assertion and method_set", func(t *testing.T) {
		ctx, st, root := seedPortFixture(t)
		debitKey := core.SymbolNode{Service: testPortService, Package: "billing", Name: "Wallet.Debit", FilePath: "billing/wallet.go"}.Key()
		if err := st.PutTypedEdges(ctx, testPortService, "billing", []core.Edge{
			{FromService: testPortService, FromFile: "billing/ledger_client.go", FromSymbol: "callSite", ToSymbol: debitKey, Kind: core.EdgeIfaceDispatch},
		}); err != nil {
			t.Fatalf("PutTypedEdges: %v", err)
		}

		targets, err := PortGaps(ctx, st, testPortService, root)
		if err != nil {
			t.Fatalf("PortGaps: %v", err)
		}
		var ledger *PortTarget
		for i := range targets {
			if targets[i].ID == "Ledger" {
				ledger = &targets[i]
			}
		}
		if ledger == nil {
			t.Fatalf("no Ledger target found in %+v", targets)
		}
		if len(ledger.Implementations) != 1 || ledger.Implementations[0].Via != "typed" {
			t.Errorf("Ledger implementations = %+v, want [{Wallet typed}] once a real EdgeIfaceDispatch edge exists", ledger.Implementations)
		}
	})

	t.Run("certify gaps --kind port: a port with implementations and no conformance certificate is a Gap", func(t *testing.T) {
		ctx, st, root := seedPortFixture(t)

		report, err := Gaps(ctx, st, testPortService, root, []string{"port"})
		if err != nil {
			t.Fatalf("Gaps: %v", err)
		}
		if len(report.Targets) != 2 {
			t.Fatalf("targets = %+v, want 2 port targets", report.Targets)
		}
		for _, tg := range report.Targets {
			if tg.Kind != "port" {
				t.Errorf("target %+v: Kind = %q, want port (the --kind filter)", tg, tg.Kind)
			}
			if !tg.Gap {
				t.Errorf("target %s: Gap = false, want true (nothing declared for it yet)", tg.ID)
			}
			if tg.Remedy == "" {
				t.Errorf("target %s: Remedy is empty, want a non-empty remedy naming its implementations", tg.ID)
			}
		}
		if report.Summary.Live != 0 || report.Summary.Gaps != 2 {
			t.Errorf("Summary = %+v, want Live=0 Gaps=2", report.Summary)
		}
	})

	t.Run("certify gaps --kind port: a live CONFORMANCE certificate closes the gap; a live but weaker one does not", func(t *testing.T) {
		ctx, st, root := seedPortFixture(t)

		claim := func(id, claimID, strength string) core.SemanticClaim {
			meta, _ := json.Marshal(Meta{Test: "TestLedger_" + strength, Strength: strength, State: string(StateLive)})
			return core.SemanticClaim{
				ClaimID: claimID, ServiceID: testPortService, Layer: Layer, Kind: Kind,
				Subject: "port:" + id, Rule: "TestLedger_" + strength + " certifies port " + id,
				Tier: core.TierVerified, Status: core.ClaimFresh, Meta: meta,
			}
		}
		if err := st.PutClaims(ctx, []core.SemanticClaim{
			claim("Ledger", "c-ledger-conformance", "conformance"),
			claim("Voucher", "c-voucher-asserts", "asserts"),
		}, nil); err != nil {
			t.Fatalf("seed PutClaims: %v", err)
		}

		report, err := Gaps(ctx, st, testPortService, root, []string{"port"})
		if err != nil {
			t.Fatalf("Gaps: %v", err)
		}
		byID := map[string]bool{}
		for _, tg := range report.Targets {
			byID[tg.ID] = tg.Gap
		}
		if gap, ok := byID["Ledger"]; !ok || gap {
			t.Errorf("Ledger Gap = %v (ok=%v), want false — a live conformance certificate closes it", gap, ok)
		}
		if gap, ok := byID["Voucher"]; !ok || !gap {
			t.Errorf("Voucher Gap = %v (ok=%v), want true — a live but non-conformance certificate never closes a port", gap, ok)
		}
	})
}
