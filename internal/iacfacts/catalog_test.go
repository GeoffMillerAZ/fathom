package iacfacts

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func twoVersionCatalog(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	same := fxResource("name", "tags")
	return fxCatalog(t, dir, IaCLimits{}, map[string][]byte{
		"1.0.0": fxExport(t, fxSource, fxProvider(), map[string]any{"widget_thing": same, "widget_old": fxResource("a")}, map[string]any{"widget_thing": fxResource("id")}),
		"1.1.0": fxExport(t, fxSource, fxProvider(), map[string]any{"widget_thing": same, "widget_new": fxResource("b")}, nil),
	})
}

func asJSON(t testing.TB, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	return v
}

func wantRefusal(t testing.TB, err error, scope, code string) *IaCRefusal {
	t.Helper()
	var ref *IaCRefusal
	if !errors.As(err, &ref) {
		t.Fatalf("want a refusal %s/%s, got %v", scope, code, err)
	}
	if ref.Scope != scope || ref.Code != code {
		t.Fatalf("want refusal %s/%s, got %s/%s: %s", scope, code, ref.Scope, ref.Code, ref.Reason)
	}
	if ref.Reason == "" {
		t.Fatalf("refusal %s/%s carries no reason", scope, code)
	}
	return ref
}

func TestReadIaCCatalogReadsOnePath(t *testing.T) {
	p := twoVersionCatalog(t)
	f, err := ReadIaCCatalog(p, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing", Path: []string{"block", "attributes", "secret"}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"type": "string", "optional": true, "sensitive": true}
	if got := asJSON(t, f.Value); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("value %s, want %v", f.Value, want)
	}
	if f.Source != fxSource || f.Version != "1.0.0" || f.Type != "widget_thing" || f.Path != "block.attributes.secret" || len(f.Digest) != 64 || f.Catalog != "cat.iaccat.zip" {
		t.Fatalf("fact does not cite its sources: %+v", f)
	}
	// descriptions are pruned, but an attribute named description stays.
	whole, err := ReadIaCCatalog(p, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(whole.Value, []byte("drop me")) || !bytes.Contains(whole.Value, []byte(`"description":{`)) {
		t.Fatalf("pruning wrong: %s", whole.Value)
	}
	// array-free nested read, a data source, a provider block, and the nested block.
	for _, req := range []IaCCatalogRequest{
		{Version: "1.0.0", Kind: SectionData, Type: "widget_thing", Path: []string{"block", "attributes", "id", "type"}},
		{Version: "1.0.0", Kind: SectionProvider, Path: []string{"block", "attributes", "region"}},
		{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing", Path: []string{"block", "block_types", "rule", "block", "attributes", "port", "required"}},
	} {
		if _, err := ReadIaCCatalog(p, req); err != nil {
			t.Fatalf("%+v: %v", req, err)
		}
	}
}

func TestReadIaCCatalogRefusesWithAReason(t *testing.T) {
	p := twoVersionCatalog(t)
	for name, tc := range map[string]struct {
		req IaCCatalogRequest
	}{
		"version": {IaCCatalogRequest{Version: "2.0.0", Kind: SectionResource, Type: "widget_thing"}},
		"type":    {IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "nope"}},
		"path":    {IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing", Path: []string{"block", "attributes", "ghost"}}},
	} {
		_, err := ReadIaCCatalog(p, tc.req)
		ref := wantRefusal(t, err, ScopeAnswer, RefuseNotFound)
		t.Logf("%s: %s", name, ref.Reason)
	}
	// the version refusal names the line to add.
	_, err := ReadIaCCatalog(p, IaCCatalogRequest{Version: "2.0.0", Kind: SectionResource, Type: "x"})
	if !strings.Contains(err.Error(), "line") {
		t.Fatalf("version refusal should name the line: %v", err)
	}
}

func TestCatalogDeduplicatesUnchangedSchemas(t *testing.T) {
	p := twoVersionCatalog(t)
	r, err := zip.OpenReader(p)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	blobs := 0
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "blobs/") {
			blobs++
		}
	}
	// provider (shared), widget_thing resource (shared), widget_old, widget_new, data widget_thing = 5.
	if blobs != 5 {
		t.Fatalf("%d blobs, want 5 (the unchanged schema is stored once)", blobs)
	}
}

func TestCatalogWriteIsDeterministic(t *testing.T) {
	a, b := twoVersionCatalog(t), twoVersionCatalog(t)
	ab, _ := os.ReadFile(a)
	bb, _ := os.ReadFile(b)
	if !bytes.Equal(ab, bb) {
		t.Fatal("two builds of the same exports differ")
	}
	r, _ := zip.OpenReader(a)
	defer r.Close()
	names := []string{}
	for _, f := range r.File {
		if !f.Modified.IsZero() && f.Modified.Year() > 1980 {
			t.Fatalf("%s carries a timestamp %v", f.Name, f.Modified)
		}
		names = append(names, f.Name)
	}
	if names[0] != "manifest.json" || !strings.HasPrefix(names[1], "versions/") {
		t.Fatalf("entry order %v", names[:3])
	}
}

func TestCatalogManifestSaysLineAndReplacement(t *testing.T) {
	p := twoVersionCatalog(t)
	m, err := ReadIaCCatalogManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Line != "1" || m.Source != fxSource || m.Replacement != ReplacementAbsent || m.Kind != KindTerraformProvider || strings.Join(m.Versions, ",") != "1.0.0,1.1.0" {
		t.Fatalf("manifest %+v", m)
	}
	if got := CatalogFileName(m); got != "registry.opentofu.org_example_widget.v1.iaccat.zip" {
		t.Fatalf("file name %q", got)
	}
	if len(m.Inputs["1.0.0"]) != 64 {
		t.Fatalf("manifest lacks the input digest: %+v", m.Inputs)
	}
}

func TestCatalogWriterRefusesMixedInputs(t *testing.T) {
	w, err := NewIaCCatalogWriter(IaCWriterOptions{WorkDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	e1 := fxExport(t, fxSource, fxProvider(), map[string]any{"a": fxResource("x")}, nil)
	if _, err := w.AddExport("1.0.0", bytes.NewReader(e1)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddExport("2.0.0", bytes.NewReader(e1)); err == nil || !strings.Contains(err.Error(), "one major line") {
		t.Fatalf("a second line must be refused: %v", err)
	}
	if _, err := w.AddExport("v1.1.0", bytes.NewReader(e1)); err == nil {
		t.Fatal("a non-canonical version must be refused")
	}
	other := fxExport(t, "registry.opentofu.org/example/other", fxProvider(), map[string]any{"a": fxResource("x")}, nil)
	if _, err := w.AddExport("1.1.0", bytes.NewReader(other)); err == nil || !strings.Contains(err.Error(), "holds") {
		t.Fatalf("a second provider must be refused: %v", err)
	}
	if _, err := w.AddExport("1.0.0", bytes.NewReader(e1)); err == nil {
		t.Fatal("a version added twice must be refused")
	}
}

func TestDocumentedBoundsAreTheAmendedOnes(t *testing.T) {
	l := DefaultIaCLimits()
	if l.BlobBytes != 4<<20 || l.CatalogBytes != 256<<20 || l.Entries != 500_000 || l.IndexBytes != 4<<20 || l.ManifestBytes != 1<<20 || l.Depth != 64 || l.FactBytes != 8<<20 {
		t.Fatalf("bounds drifted from docs/design/iac-facts.md section 9: %+v", l)
	}
}

// entrySize returns an entry's uncompressed size.
func entrySize(t testing.TB, path, name string) int64 {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			return int64(f.UncompressedSize64)
		}
	}
	t.Fatalf("no entry %s", name)
	return 0
}

func entryCount(t testing.TB, path string) int {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	return len(r.File)
}

// TestCatalogBoundsRefuseAtCapPlusOne reads each bound at cap (allowed) and cap + 1 (refused), with the scope the
// design gives it.
func TestCatalogBoundsRefuseAtCapPlusOne(t *testing.T) {
	p := twoVersionCatalog(t)
	req := IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing", Path: []string{"block"}}
	st, _ := os.Stat(p)
	whole, err := ReadIaCCatalog(p, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing"})
	if err != nil {
		t.Fatal(err)
	}
	blobAddr := whole.Digest
	cases := []struct {
		name  string
		set   func(*IaCLimits, int64)
		size  int64
		scope string
		code  string
	}{
		{"catalog bytes", func(l *IaCLimits, n int64) { l.CatalogBytes = n }, st.Size(), ScopeCatalog, RefuseCatalogBytes},
		{"entries", func(l *IaCLimits, n int64) { l.Entries = int(n) }, int64(entryCount(t, p)), ScopeCatalog, RefuseEntries},
		{"manifest bytes", func(l *IaCLimits, n int64) { l.ManifestBytes = n }, entrySize(t, p, "manifest.json"), ScopeCatalog, RefuseManifestBytes},
		{"index bytes", func(l *IaCLimits, n int64) { l.IndexBytes = n }, entrySize(t, p, "versions/1.0.0.json"), ScopeAnswer, RefuseIndexBytes},
		{"blob bytes", func(l *IaCLimits, n int64) { l.BlobBytes = n }, entrySize(t, p, "blobs/"+blobAddr), ScopeAnswer, RefuseBlobBytes},
	}
	for _, c := range cases {
		lim := DefaultIaCLimits()
		c.set(&lim, c.size)
		if _, err := readIaCCatalog(p, req, lim); err != nil {
			t.Errorf("%s at cap: %v", c.name, err)
		}
		c.set(&lim, c.size-1)
		_, err := readIaCCatalog(p, req, lim)
		wantRefusal(t, err, c.scope, c.code)
	}
}

func TestCatalogFactAndDepthBounds(t *testing.T) {
	p := twoVersionCatalog(t)
	req := IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing", Path: []string{"block", "attributes"}}
	f, err := ReadIaCCatalog(p, req)
	if err != nil {
		t.Fatal(err)
	}
	lim := DefaultIaCLimits()
	lim.FactBytes = int64(len(f.Value))
	if _, err := readIaCCatalog(p, req, lim); err != nil {
		t.Fatalf("fact at cap: %v", err)
	}
	lim.FactBytes--
	_, err = readIaCCatalog(p, req, lim)
	ref := wantRefusal(t, err, ScopeAnswer, RefuseFactBytes)
	if !strings.Contains(ref.Reason, "I48") {
		t.Fatalf("the reason should name the ceiling: %s", ref.Reason)
	}
	// depth: the nested rule block sits at nesting depth d; a bound of d reads, d-1 refuses.
	lim = DefaultIaCLimits()
	whole := IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing"}
	lim.Depth = 7 // {block{block_types{rule{block{attributes{port{type[...] is 8 deep at its leaves
	depth := 0
	for d := 1; d < 20; d++ {
		lim.Depth = d
		if _, err := readIaCCatalog(p, whole, lim); err == nil {
			depth = d
			break
		}
	}
	if depth == 0 {
		t.Fatal("no depth read the blob")
	}
	lim.Depth = depth - 1
	_, err = readIaCCatalog(p, whole, lim)
	wantRefusal(t, err, ScopeAnswer, RefuseDepth)
	if _, err := verifyIaCCatalog(p, lim); err == nil {
		t.Fatal("verify at depth-1 must refuse the whole catalog")
	} else {
		wantRefusal(t, err, ScopeCatalog, RefuseDepth)
	}
}

// TestIaCCatalogLargestBlobHoldsOneFactUnderTheCeiling is IF-M0 A1: a synthetic blob as large as the 2.43 MiB AWS
// type, one fact read from it, and the bytes allocated for the whole lookup (open, directory scan, index, blob)
// stay under I48's 8 MiB. The prototype decoded whole blobs and measured up to 41.8 MB.
func TestIaCCatalogLargestBlobHoldsOneFactUnderTheCeiling(t *testing.T) {
	const want = 2_548_039 // 2.43 MiB, the largest AWS type at its pin
	dir := t.TempDir()
	res := map[string]any{"widget_huge": fxBig(t, want+want/10)}
	for i := 0; i < 300; i++ { // a realistic number of neighbours: the directory scan must not allocate per entry
		res[fmt.Sprintf("widget_small_%03d", i)] = fxResource(fmt.Sprintf("n%d", i))
	}
	p := fxCatalog(t, dir, IaCLimits{}, map[string][]byte{"1.0.0": fxExport(t, fxSource, fxProvider(), res, nil)})
	whole, err := ReadIaCCatalog(p, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_huge"})
	if err != nil {
		t.Fatal(err)
	}
	if len(whole.Value) < want {
		t.Fatalf("the synthetic blob is %d bytes, want at least %d", len(whole.Value), want)
	}
	if int64(len(whole.Value)) > MaxBlobBytes {
		t.Fatalf("the synthetic blob is %d bytes, over the %d bound", len(whole.Value), MaxBlobBytes)
	}
	for name, req := range map[string]IaCCatalogRequest{
		"one attribute":    {Version: "1.0.0", Kind: SectionResource, Type: "widget_huge", Path: []string{"block", "attributes", "attribute_000010", "type"}},
		"a late attribute": {Version: "1.0.0", Kind: SectionResource, Type: "widget_huge", Path: []string{"block", "attributes", "attribute_020000", "optional"}},
		"the whole blob":   {Version: "1.0.0", Kind: SectionResource, Type: "widget_huge"},
	} {
		runtime.GC()
		var a, b runtime.MemStats
		runtime.ReadMemStats(&a)
		f, err := ReadIaCCatalog(p, req)
		runtime.ReadMemStats(&b)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		alloc := b.TotalAlloc - a.TotalAlloc
		t.Logf("%s: fact %d bytes, %d bytes allocated", name, len(f.Value), alloc)
		if alloc >= uint64(MaxFactBytes) {
			t.Errorf("%s: %d bytes allocated, at or over I48's %d byte ceiling", name, alloc, MaxFactBytes)
		}
		if name != "the whole blob" && alloc > 1<<20 {
			t.Errorf("%s: a one-attribute fact allocated %d bytes; the scanner should not buffer the blob", name, alloc)
		}
	}
}

func TestCatalogCorruptionIsRefused(t *testing.T) {
	p := twoVersionCatalog(t)
	whole, err := ReadIaCCatalog(p, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing"})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := zip.OpenReader(p)
	var off int64
	for _, f := range r.File {
		if f.Name == "blobs/"+whole.Digest {
			off, _ = f.DataOffset()
		}
	}
	r.Close()
	b, _ := os.ReadFile(p)
	b[off+3] ^= 0x55
	bad := filepath.Join(t.TempDir(), "bad.iaccat.zip")
	if err := os.WriteFile(bad, b, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ReadIaCCatalog(bad, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "widget_thing"})
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
	_, err = VerifyIaCCatalog(bad)
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
	// not a zip at all.
	junk := filepath.Join(t.TempDir(), "junk.iaccat.zip")
	os.WriteFile(junk, []byte("not a zip, but long enough to look at, really"), 0o644)
	_, err = ReadIaCCatalog(junk, IaCCatalogRequest{Version: "1.0.0", Kind: SectionProvider})
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
}

// writeRawCatalog writes a catalog by hand, for faults the writer would never produce.
func writeRawCatalog(t testing.TB, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "raw.iaccat.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for n, c := range files {
		w, _ := zw.Create(n)
		w.Write([]byte(c))
	}
	zw.Close()
	f.Close()
	return p
}

func TestVerifyIaCCatalogFindsMissingAndForgedBlobs(t *testing.T) {
	man := `{"format":1,"kind":"terraform-provider","source":"h/e/w","line":"1","versions":["1.0.0"],"inputs":{},"replacement":"absent"}`
	missing := strings.Repeat("a", 64)
	p := writeRawCatalog(t, map[string]string{
		"manifest.json":       man,
		"versions/1.0.0.json": `{"resources":{"r":"` + missing + `"},"data_sources":{},"ephemeral_resources":{}}`,
	})
	_, err := VerifyIaCCatalog(p)
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
	forged := writeRawCatalog(t, map[string]string{
		"manifest.json":       man,
		"versions/1.0.0.json": `{"resources":{"r":"` + missing + `"},"data_sources":{},"ephemeral_resources":{}}`,
		"blobs/" + missing:    `{"version":0}`,
	})
	_, err = VerifyIaCCatalog(forged)
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
	if !strings.Contains(err.Error(), "hash") {
		t.Fatalf("a forged blob should say it does not hash to its address: %v", err)
	}
	// a manifest version outside the named line.
	wrongLine := writeRawCatalog(t, map[string]string{
		"manifest.json": strings.Replace(man, `"1.0.0"`, `"2.0.0"`, 1),
	})
	_, err = VerifyIaCCatalog(wrongLine)
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
}

// TestCatalogZipReaderPastSixtyFiveThousandEntries covers the zip64 end record (more than 65,535 entries) and the
// entry bound, on a hand-built zip so the test stays fast.
func TestCatalogZipReaderPastSixtyFiveThousandEntries(t *testing.T) {
	const n = 70_000
	p := filepath.Join(t.TempDir(), "many.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for i := 0; i < n; i++ {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("blobs/%064d", i), Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(w, "%d", i)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	rf, _ := os.Open(p)
	defer rf.Close()
	st, _ := rf.Stat()
	lim := DefaultIaCLimits()
	z, err := openCatZip(rf, st.Size(), lim)
	if err != nil {
		t.Fatal(err)
	}
	if z.entries != n {
		t.Fatalf("%d entries, want %d", z.entries, n)
	}
	name := fmt.Sprintf("blobs/%064d", n-1)
	e, ok, err := z.find(name)
	if err != nil || !ok {
		t.Fatalf("find: %v %v", ok, err)
	}
	r, err := z.open(name, e, 100, RefuseBlobBytes)
	if err != nil {
		t.Fatal(err)
	}
	b := new(bytes.Buffer)
	b.ReadFrom(r)
	if b.String() != fmt.Sprint(n-1) {
		t.Fatalf("read %q", b.String())
	}
	lim.Entries = n
	if _, err := openCatZip(rf, st.Size(), lim); err != nil {
		t.Fatalf("at the entry bound: %v", err)
	}
	lim.Entries = n - 1
	_, err = openCatZip(rf, st.Size(), lim)
	wantRefusal(t, err, ScopeCatalog, RefuseEntries)
}
