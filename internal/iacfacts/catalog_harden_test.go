package iacfacts

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const hardenBlob = `{"version":0,"block":{"attributes":{"a":{"type":"string"}}}}`

func addrOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func rawManifest(versions string) string {
	return `{"format":1,"kind":"terraform-provider","source":"h/e/w","line":"1","versions":[` + versions + `],"inputs":{},"replacement":"absent"}`
}

// writeStoredCatalog writes a catalog whose entries are stored uncompressed, so a test can flip one byte of an entry.
func writeStoredCatalog(t testing.TB, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "stored.iaccat.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for n, c := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(c))
	}
	zw.Close()
	f.Close()
	return p
}

// M18: JSON depth is enforced over the whole manifest and the whole index at add, not along the path a lookup walks.
func TestAddRefusesDeepManifestAndIndexWholesale(t *testing.T) {
	blob := addrOf(hardenBlob)
	deep := strings.Repeat("[", MaxJSONDepth+6) + strings.Repeat("]", MaxJSONDepth+6)
	deepManifest := strings.Replace(rawManifest(`"1.0.0"`), `"inputs":{}`, `"inputs":{},"extra":`+strings.Repeat("[", 5000)+strings.Repeat("]", 5000), 1)
	p := writeRawCatalog(t, map[string]string{
		"manifest.json":       deepManifest,
		"versions/1.0.0.json": `{"provider":"` + blob + `","resources":{},"data_sources":{},"ephemeral_resources":{}}`,
		"blobs/" + blob:       hardenBlob,
	})
	_, err := VerifyIaCCatalog(p)
	wantRefusal(t, err, ScopeCatalog, RefuseDepth)

	// the deep value sits after the key a lookup stops at.
	p = writeRawCatalog(t, map[string]string{
		"manifest.json":       rawManifest(`"1.0.0"`),
		"versions/1.0.0.json": `{"provider":"` + blob + `","resources":{},"data_sources":{},"ephemeral_resources":{},"zz":` + deep + `}`,
		"blobs/" + blob:       hardenBlob,
	})
	if _, err := ReadIaCCatalog(p, IaCCatalogRequest{Version: "1.0.0", Kind: SectionProvider}); err != nil {
		t.Fatalf("a lookup that stops before the deep value still reads: %v", err)
	}
	_, err = VerifyIaCCatalog(p)
	wantRefusal(t, err, ScopeCatalog, RefuseDepth)
}

// N1: a lookup against a manifest at the bound allocates under I48's 8 MiB, and one byte more refuses.
func TestLookupAtTheManifestBoundStaysUnderTheCeiling(t *testing.T) {
	blob := addrOf(hardenBlob)
	var b strings.Builder
	for b.Len() < int(MaxManifestBytes)-400 {
		b.WriteString(`"1.0.0",`)
	}
	m := rawManifest(b.String() + `"1.0.0"`)
	m += strings.Repeat(" ", int(MaxManifestBytes)-len(m))
	files := map[string]string{
		"manifest.json":       m,
		"versions/1.0.0.json": `{"provider":"` + blob + `"}`,
		"blobs/" + blob:       hardenBlob,
	}
	p := writeRawCatalog(t, files)
	req := IaCCatalogRequest{Version: "1.0.0", Kind: SectionProvider, Path: []string{"block", "attributes", "a", "type"}}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	f, err := ReadIaCCatalog(p, req)
	runtime.ReadMemStats(&after)
	if err != nil || string(f.Value) != `"string"` {
		t.Fatalf("%s %v", f.Value, err)
	}
	t.Logf("one lookup at the manifest bound allocated %d bytes", after.TotalAlloc-before.TotalAlloc)
	if got := after.TotalAlloc - before.TotalAlloc; got >= uint64(MaxFactBytes) {
		t.Fatalf("one lookup at the manifest bound allocated %d bytes, over I48's %d", got, MaxFactBytes)
	}
	files["manifest.json"] = m + " "
	_, err = ReadIaCCatalog(writeRawCatalog(t, files), req)
	wantRefusal(t, err, ScopeCatalog, RefuseManifestBytes)
}

// N2: corruption is corruption and never "not found".
func TestCorruptionIsNeverNotFound(t *testing.T) {
	blob := addrOf(hardenBlob)
	// a blob that does not hash to its address, asked for a path it lacks.
	tampered := writeRawCatalog(t, map[string]string{
		"manifest.json":       rawManifest(`"1.0.0"`),
		"versions/1.0.0.json": `{"resources":{"r":"` + blob + `"}}`,
		"blobs/" + blob:       `{"version":1}`,
	})
	_, err := ReadIaCCatalog(tampered, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "r", Path: []string{"block", "nope"}})
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
	// a blob whose root is not a container, tampered, is corrupt too.
	scalar := writeRawCatalog(t, map[string]string{
		"manifest.json":       rawManifest(`"1.0.0"`),
		"versions/1.0.0.json": `{"resources":{"r":"` + blob + `"}}`,
		"blobs/" + blob:       `7`,
	})
	_, err = ReadIaCCatalog(scalar, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "r", Path: []string{"block"}})
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
	// an index with a flipped byte after the point a lookup stops reading.
	idx := `{"resources":{}}` + strings.Repeat(" ", 64)
	p := writeStoredCatalog(t, map[string]string{
		"manifest.json":       rawManifest(`"1.0.0"`),
		"versions/1.0.0.json": idx,
	})
	raw, _ := os.ReadFile(p)
	at := strings.Index(string(raw), idx) + len(idx) - 8
	raw[at] = 'x'
	os.WriteFile(p, raw, 0o644)
	_, err = ReadIaCCatalog(p, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "r"})
	wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
	// the same lookup on an untouched copy is an honest not-found.
	ok := writeStoredCatalog(t, map[string]string{"manifest.json": rawManifest(`"1.0.0"`), "versions/1.0.0.json": idx})
	_, err = ReadIaCCatalog(ok, IaCCatalogRequest{Version: "1.0.0", Kind: SectionResource, Type: "r"})
	wantRefusal(t, err, ScopeAnswer, RefuseNotFound)
}

// N9: hostile catalogs are refused at add, and a version the manifest does not list is not readable.
func TestAddRefusesDuplicateKeysUnknownEntriesAndStrayVersions(t *testing.T) {
	blob := addrOf(hardenBlob)
	other := addrOf(hardenBlob + " ")
	good := func() map[string]string {
		return map[string]string{
			"manifest.json":       rawManifest(`"1.0.0"`),
			"versions/1.0.0.json": `{"resources":{"r":"` + blob + `"}}`,
			"blobs/" + blob:       hardenBlob,
		}
	}
	if _, err := VerifyIaCCatalog(writeRawCatalog(t, good())); err != nil {
		t.Fatalf("the baseline must verify: %v", err)
	}
	for name, mutate := range map[string]func(map[string]string){
		"duplicate type key": func(f map[string]string) {
			f["versions/1.0.0.json"] = `{"resources":{"r":"` + blob + `","r":"` + other + `","q":"` + blob + `"}}`
			f["blobs/"+other] = hardenBlob + " "
		},
		"duplicate section key": func(f map[string]string) {
			f["versions/1.0.0.json"] = `{"resources":{"r":"` + blob + `"},"resources":{"q":"` + blob + `"}}`
		},
		"unknown index key": func(f map[string]string) {
			f["versions/1.0.0.json"] = `{"resources":{"r":"` + blob + `"},"deprecation_message":"x"}`
		},
		"unknown entry":       func(f map[string]string) { f["../../evil"] = "x" },
		"stray file":          func(f map[string]string) { f["README"] = "x" },
		"version not listed":  func(f map[string]string) { f["versions/1.9.9.json"] = `{"resources":{}}` },
		"unreferenced blob":   func(f map[string]string) { f["blobs/"+other] = hardenBlob + " " },
		"directory-like name": func(f map[string]string) { f["blobs/"] = "" },
	} {
		f := good()
		mutate(f)
		_, err := VerifyIaCCatalog(writeRawCatalog(t, f))
		if err == nil {
			t.Errorf("%s: add accepted a hostile catalog", name)
			continue
		}
		wantRefusal(t, err, ScopeCatalog, RefuseCorrupt)
	}
	// a version index the manifest does not list is not a release of the catalog, even when the zip holds it.
	f := good()
	f["versions/1.9.9.json"] = `{"resources":{"r":"` + blob + `"}}`
	_, err := ReadIaCCatalog(writeRawCatalog(t, f), IaCCatalogRequest{Version: "1.9.9", Kind: SectionResource, Type: "r"})
	wantRefusal(t, err, ScopeAnswer, RefuseNotFound)
}
