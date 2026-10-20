package fusion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// epochForm is the derived epoch's shape: the bump count over the frozen base, a dot, 12 hex. It
// never holds ':' (a file_stats row carries the epoch before its first ':', statRowHash).
var epochForm = regexp.MustCompile(`^([0-9]+)\.([0-9a-f]{12})$`)

// epochParts splits a derived epoch into its number and its hash part.
func epochParts(t *testing.T, v string) (int, string) {
	t.Helper()
	m := epochForm.FindStringSubmatch(v)
	if m == nil {
		t.Fatalf("epoch %q is not <number>.<12 hex>", v)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n, m[2]
}

// digestOf is a stand-in data digest: sha256 hex of b.
func digestOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// withBumps is bumps plus each named bump file.
func withBumps(bumps map[string][]byte, names ...string) map[string][]byte {
	out := map[string][]byte{}
	maps.Copy(out, bumps)
	for _, n := range names {
		out[n] = []byte("Epoch bump " + n + ": what changed, and why stored observations are stale.\n")
	}
	return out
}

// TestEpochOfChangesWithABumpFileOrADataByte pins the derived epoch (ADR-075 D4): a new bump file
// raises the number and changes the value, one changed data byte changes the hash part, and two
// waves' bump files merged give a value distinct from each wave's. The binary's ExtractorEpoch is
// this value over its embedded bump files and data.
func TestEpochOfChangesWithABumpFileOrADataByte(t *testing.T) {
	data := []byte(`{"id":"go","display":"Go"}`)
	d := digestOf(data)
	base := withBumps(nil, "lang4.md")
	v0 := epochOf(base, d)
	if n, _ := epochParts(t, v0); n != 68 {
		t.Fatalf("one bump file over base 67 numbers %d, want 68 (lang4's, no jump)", n)
	}
	if again := epochOf(maps.Clone(base), d); again != v0 {
		t.Fatalf("same inputs, two values: %s then %s", v0, again)
	}

	// A new bump file changes the value and raises the number.
	v1 := epochOf(withBumps(base, "ap3.md"), d)
	if n, _ := epochParts(t, v1); n != 69 || v1 == v0 {
		t.Fatalf("with a second bump file: %s (was %s), want 69.<new hash>", v1, v0)
	}
	// Editing a bump file's paragraph changes the hash part.
	edited := maps.Clone(base)
	edited["lang4.md"] = append(append([]byte{}, base["lang4.md"]...), '.')
	if epochOf(edited, d) == v0 {
		t.Fatal("an edited bump file left the epoch unchanged")
	}

	// One data byte changes the hash part; the number stays.
	changed := append([]byte{}, data...)
	changed[len(changed)/2] ^= 0x01
	v2 := epochOf(base, digestOf(changed))
	n0, h0 := epochParts(t, v0)
	n2, h2 := epochParts(t, v2)
	if n2 != n0 || h2 == h0 {
		t.Fatalf("one data byte changed: %s (was %s), want the same number and a new hash part", v2, v0)
	}

	// Two waves that each add a bump file, merged: distinct from both, and both from each other.
	va := epochOf(withBumps(base, "wavea.md"), d)
	vb := epochOf(withBumps(base, "waveb.md"), d)
	vab := epochOf(withBumps(base, "wavea.md", "waveb.md"), d)
	if va == vb {
		t.Fatalf("two waves' bump files gave one value %s", va)
	}
	if vab == va || vab == vb {
		t.Fatalf("merged value %s equals a wave's (%s, %s)", vab, va, vb)
	}
	if n, _ := epochParts(t, vab); n != 70 {
		t.Fatalf("merged number %d, want 70", n)
	}

	// The binary's value: the embedded bump files over the embedded data digest.
	bumps := embeddedBumps()
	if _, ok := bumps["lang4.md"]; !ok {
		t.Fatal("internal/fusion/epochs/lang4.md (epoch 68) is not embedded")
	}
	if want := epochOf(bumps, extractor.EmbeddedDataDigest()); ExtractorEpoch != want {
		t.Fatalf("ExtractorEpoch %s, want %s from the embedded bump files and data", ExtractorEpoch, want)
	}
	if n, _ := epochParts(t, ExtractorEpoch); n != epochBase+len(bumps) {
		t.Fatalf("ExtractorEpoch numbers %d, want %d + %d bump files", n, epochBase, len(bumps))
	}
	for name, b := range bumps {
		if !strings.HasSuffix(name, ".md") || strings.Contains(name, "/") || len(strings.TrimSpace(string(b))) == 0 {
			t.Errorf("bump file %q is not a non-empty epochs/<wave>.md", name)
		}
	}
}

// TestFingerprintHashSaltsWithTheDerivedEpoch pins the hash gate's salt (ADR-075 D4):
// FingerprintHash keeps its formula and salts with the derived epoch, so another derived value
// changes every file's fingerprint and an ingest after a bump file or a data change re-extracts
// every file; the same value is stable across runs and machines. A store the last literal epoch
// ("68") fingerprinted is re-extracted once, then gated again.
func TestFingerprintHashSaltsWithTheDerivedEpoch(t *testing.T) {
	contents := [][]byte{[]byte("package a\n\nfunc A() {}\n"), []byte("# readme\n"), {}}
	next := epochOf(withBumps(embeddedBumps(), "next.md"), extractor.EmbeddedDataDigest())
	for _, c := range contents {
		formula := sha256.Sum256(append(append([]byte{}, c...), []byte("\x00fathom-extractor-epoch:"+ExtractorEpoch)...))
		if got := FingerprintHash(c); got != hex.EncodeToString(formula[:]) {
			t.Fatalf("FingerprintHash(%q) = %s, not sha256(bytes, NUL, fathom-extractor-epoch:<ExtractorEpoch>)", c, got)
		}
		if FingerprintHash(c) != saltedFingerprint(c, ExtractorEpoch) {
			t.Fatalf("FingerprintHash(%q) does not salt with ExtractorEpoch", c)
		}
		if saltedFingerprint(c, next) == FingerprintHash(c) {
			t.Errorf("another derived epoch left %q's fingerprint unchanged", c)
		}
		if saltedFingerprint(c, "68") == FingerprintHash(c) {
			t.Errorf("the literal epoch 68 and the derived one give %q one fingerprint", c)
		}
	}

	// Stable across runs and machines: the value depends on the inputs' bytes alone. Both
	// literals below follow from the formulas, not from this binary.
	v := epochOf(map[string][]byte{"lang4.md": []byte("x\n")}, digestOf([]byte("d")))
	if v != goldenEpoch {
		t.Fatalf("epochOf over fixed inputs = %s, want %s", v, goldenEpoch)
	}
	if got := saltedFingerprint([]byte("package p\n"), goldenEpoch); got != goldenFingerprint {
		t.Fatalf("fingerprint under %s = %s, want %s", goldenEpoch, got, goldenFingerprint)
	}

	// A store fingerprinted under the literal "68": the first ingest re-extracts every file, the
	// next is hash-gated.
	files := map[string]string{"a.go": "package a\n\nfunc A() {}\n", "b.py": "def b():\n    return 1\n"}
	dir := t.TempDir()
	writeCorpus(t, dir, files)
	st := store.NewMemoryStore()
	ctx := context.Background()
	svc := core.ServiceRef{ID: "epoch-derived", Path: dir}
	for name, src := range files {
		if err := st.SetFingerprint(ctx, svc.ID, name, saltedFingerprint([]byte(src), "68")); err != nil {
			t.Fatal(err)
		}
	}
	p := New(newCountingConn(), st, Config{Concurrency: 1, DisableTypedEdges: true})
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped: a literal-epoch-68 fingerprint was honored under %s", rep.Skipped, ExtractorEpoch)
	}
	for name, src := range files {
		got, found, err := st.Fingerprint(ctx, svc.ID, name)
		if err != nil || !found || got != FingerprintHash([]byte(src)) {
			t.Fatalf("%s's stored fingerprint %q (found %v, %v), want the derived epoch's", name, got, found, err)
		}
	}
	if rep, err = p.Ingest(ctx, svc); err != nil || rep.Skipped != len(files) {
		t.Fatalf("second ingest skipped %d of %d (%v): the derived epoch's fingerprints are not stable", rep.Skipped, len(files), err)
	}
}

// The golden values TestFingerprintHashSaltsWithTheDerivedEpoch pins.
const (
	goldenEpoch       = "68.58cc10a90b37"
	goldenFingerprint = "a9d8dad98807d13ffaa38775dcb594c5f455cec6d34fb11c59aa5fc28b1332fa"
)

// TestStatGateDoesNotVouchUnderTheLiteralEpoch (RT-180, ADR-075 D4): a stat record the last
// literal-epoch binary wrote ("68", its hash and the stored fingerprint both salted with "68")
// agrees with the store, yet vouches for nothing under the derived epoch: every file is read and
// re-extracted once after the upgrade, then the records written under the derived epoch vouch.
func TestStatGateDoesNotVouchUnderTheLiteralEpoch(t *testing.T) {
	files := map[string]string{"a.go": "package a\n\nfunc A() {}\n", "b.go": "package a\n\nfunc B() {}\n"}
	dir := t.TempDir()
	writeCorpus(t, dir, files)
	backdate(t, dir)
	conn := newCountingConn()
	st := store.NewMemoryStore()
	p := New(conn, st, Config{Concurrency: 2, DisableTypedEdges: true})
	t.Cleanup(p.Close)
	var recorded []StatRecord
	var source map[string]StatRecord
	p.WithStatSource(func(context.Context, string) (map[string]StatRecord, bool) { return source, source != nil })
	p.WithStatSink(func(_ context.Context, _ string, recs []StatRecord) error { recorded = recs; return nil })
	svc := core.ServiceRef{ID: "ep-literal", Path: dir}
	ctx := context.Background()
	if _, err := p.Ingest(ctx, svc); err != nil {
		t.Fatal(err)
	}
	conn.take()
	if len(recorded) != len(files) {
		t.Fatalf("sink got %d records, want %d", len(recorded), len(files))
	}
	// Rewrite the store and the records as the literal-epoch binary left them.
	source = map[string]StatRecord{}
	for _, r := range recorded {
		old := saltedFingerprint([]byte(files[r.Path]), "68")
		if err := st.SetFingerprint(ctx, svc.ID, r.Path, old); err != nil {
			t.Fatal(err)
		}
		r.Epoch, r.ContentHash = "68", old
		source[r.Path] = r
	}
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatal(err)
	}
	if got := conn.take(); len(got) != len(files) || rep.StatSkipped != 0 || rep.Skipped != 0 {
		t.Fatalf("records under the literal epoch 68: read %v, stat-skipped %d, hash-skipped %d; want every file read and re-extracted", got, rep.StatSkipped, rep.Skipped)
	}
	source = map[string]StatRecord{}
	for _, r := range recorded {
		if r.Epoch != ExtractorEpoch {
			t.Fatalf("record for %s written under %q, want %s", r.Path, r.Epoch, ExtractorEpoch)
		}
		source[r.Path] = r
	}
	if rep, err := p.Ingest(ctx, svc); err != nil || rep.StatSkipped != len(files) {
		t.Fatalf("records under the derived epoch: stat-skipped %d of %d (%v)", rep.StatSkipped, len(files), err)
	}
	if got := conn.take(); len(got) != 0 {
		t.Fatalf("records under the derived epoch: read %v, want nothing", got)
	}
}
