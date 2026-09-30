package langbench

// grammarpin_test.go — LB-F50 (the catalogue pre-fill) and LB-F51 (the
// licence verdict at pin), design §8.2 and §13.1 steps 5-6.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

const fixtureCatalogue = "testdata/grammarpin/catalogue"

const fixtureRev = "0123456789abcdef0123456789abcdef01234567"

func fixtureQueue() []QueueEntry {
	tags := func(id, engine, src string) QueueEntry {
		return QueueEntry{ID: id, Kind: "language", Engine: engine, GrammarSource: src, PlanRow: "LB-Q-" + id}
	}
	return []QueueEntry{
		tags("alpha", "tags", "acme/tree-sitter-alpha"),
		tags("beta", "tags+doc", "acme/tree-sitter-beta (weak)"),
		tags("gamma", "tags", "acme/tree-sitter-gamma"),
		tags("delta", "tags", "acme/tree-sitter-delta"),
		tags("epsilon", "tags", "acme/tree-sitter-epsilon"),
		tags("iota", "tags", "acme/tree-sitter-iota"),
		tags("theta", "tags", "acme/tree-sitter-theta"),
		tags("eta", "tags", "acme/tree-sitter-eta"),
		tags("zeta", "tags", "acme/tree-sitter-zeta"),
		tags("objective-c", "tags", "ACME/tree-sitter-objc"),
		tags("missing", "tags", "acme/tree-sitter-missing"),
		tags("declared", "tags", "acme/tree-sitter-declared"),
		tags("pl", "tags", "acme/tree-sitter-pl-agpl"),
		tags("nosource", "tags", "none consolidated"),
		// Skipped: the blob is already locked, the grammar is locked under
		// another blob (a variant), the engine is not a tags one, and a
		// pure-go source has no grammar.
		tags("locked", "tags", "acme/tree-sitter-locked"),
		tags("variant", "tags", "Acme/Tree-Sitter-Shared"),
		tags("docs", "doc", "acme/tree-sitter-docs"),
		tags("nat", "native", "acme/tree-sitter-nat"),
		tags("jup", "tags+doc", "pure-go:encoding/json + python dialect"),
	}
}

func fixtureLock() tsengine.GrammarLock {
	return tsengine.GrammarLock{Grammars: []tsengine.GrammarLockRow{
		{Blob: "locked", Repo: "acme/tree-sitter-locked"},
		{Blob: "shared", Repo: "acme/tree-sitter-shared"},
	}}
}

func fixturePrior() GrammarPrefill {
	return GrammarPrefill{
		Overrides: map[string]string{"objective-c": "objc"},
		Verdicts: map[string]PinVerdict{
			// The queue's own grammar, absent from the catalogue, accepted as declared.
			"declared": {Repo: "acme/tree-sitter-declared", Commit: strings.Repeat("d", 40), LicenceVerdict: LicenceVerdict{Licence: "MIT", Basis: BasisDeclared}},
			// The queue's grammar is AGPL; the catalogue's alternative is another repo.
			"pl": {Repo: "acme/tree-sitter-pl-agpl", Commit: strings.Repeat("e", 40), LicenceVerdict: LicenceVerdict{Licence: "AGPL-3.0", Basis: BasisFile, Refused: true, Reason: "AGPL"}},
		},
	}
}

func fixtureData(t *testing.T) GrammarPrefill {
	t.Helper()
	cat, err := LoadCatalogue(fixtureCatalogue)
	if err != nil {
		t.Fatal(err)
	}
	d, err := PrefillLockRows(fixtureQueue(), fixtureLock(), cat, fixturePrior(), fixtureRev)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestPrefillLockRows: the mapping (id equality, the override map), every
// flag, the skipped items, byte-stable output, and rows that pass the lock's
// own field rules apart from the digests — the flags predict exactly which
// rows the lock would refuse.
func TestPrefillLockRows(t *testing.T) {
	d := fixtureData(t)

	if d.Source != CatalogueSource || d.Revision != fixtureRev || d.Licence != CatalogueLicence {
		t.Errorf("header = %q %q %q", d.Source, d.Revision, d.Licence)
	}
	var ids []string
	for id := range d.Grammars {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	wantIDs := []string{"alpha", "beta", "declared", "delta", "epsilon", "eta", "gamma", "iota", "missing", "nosource", "objective-c", "pl", "theta", "zeta"}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("entries = %v, want %v (locked, variant, doc, native and pure-go items are skipped)", ids, wantIDs)
	}

	h := func(c byte) string { return strings.Repeat(string(c), 40) }
	want := map[string]PrefillEntry{
		"alpha": {Catalogue: "alpha", Repo: "acme/tree-sitter-alpha", Commit: h('1'), ABI: 14, Licence: "MIT", LicenceBasis: BasisCatalogue, Flags: []string{}},
		"beta": {Catalogue: "beta", Repo: "acme2/tree-sitter-beta", Commit: h('2'), ABI: 15, Generate: true, Directory: "grammars/beta", CSymbol: "beta_lang",
			Licence: "Apache-2.0", LicenceBasis: BasisCatalogue, Flags: []string{"grammar-differs:acme/tree-sitter-beta"}},
		"gamma":   {Catalogue: "gamma", Repo: "acme/tree-sitter-gamma", Commit: h('3'), ABI: 14, Licence: "AGPL-3.0", LicenceBasis: BasisCatalogue, Flags: []string{"refused:AGPL-3.0"}},
		"delta":   {Catalogue: "delta", Repo: "acme/tree-sitter-delta", Commit: h('4'), ABI: 14, Flags: []string{"unlicensed"}},
		"epsilon": {Catalogue: "epsilon", Repo: "acme/tree-sitter-epsilon", Commit: h('5'), ABI: 16, Generate: true, Licence: "MIT", LicenceBasis: BasisCatalogue, Flags: []string{"abi-outside:16"}},
		"iota":    {Catalogue: "iota", Repo: "acme/tree-sitter-iota", Commit: h('7'), ABI: 12, Generate: true, Licence: "Apache-2.0", LicenceBasis: BasisCatalogue, Flags: []string{"abi-outside:12"}},
		"theta":   {Catalogue: "theta", Repo: "acme/tree-sitter-theta", Commit: h('a'), ABI: 14, Licence: "MIT-0", LicenceBasis: BasisCatalogue, Flags: []string{"refused:MIT-0"}},
		// An inline licence beats the cache; a trailing .git is dropped from the repo.
		"eta": {Catalogue: "eta", Repo: "acme/tree-sitter-eta", Commit: h('6'), ABI: 14, Licence: "BSD-3-Clause", LicenceBasis: BasisCatalogue, Flags: []string{}},
		// A local grammar has no upstream to pin: not in the catalogue for our purpose.
		"zeta": {Repo: "acme/tree-sitter-zeta", Flags: []string{"not-in-catalogue", "unlicensed"}},
		// The override map maps objective-c to objc; ACME/... equals acme/... case-insensitively.
		"objective-c": {Catalogue: "objc", Repo: "acme/tree-sitter-objc", Commit: h('8'), ABI: 14, Licence: "MIT", LicenceBasis: BasisCatalogue, Flags: []string{}},
		"missing":     {Repo: "acme/tree-sitter-missing", Flags: []string{"not-in-catalogue", "unlicensed"}},
		// Absent from the catalogue but the queue's own grammar has a recorded verdict.
		"declared": {Repo: "acme/tree-sitter-declared", Licence: "MIT", LicenceBasis: BasisDeclared, Flags: []string{"not-in-catalogue"}},
		// The catalogue offers another repo (ISC); the queue's AGPL verdict is not the entry's.
		"pl":       {Catalogue: "pl", Repo: "other/tree-sitter-pl", Commit: h('9'), ABI: 14, Licence: "ISC", LicenceBasis: BasisCatalogue, Flags: []string{"grammar-differs:acme/tree-sitter-pl-agpl"}},
		"nosource": {Repo: "", Flags: []string{"not-in-catalogue", "unlicensed"}},
	}
	for id, w := range want {
		if got := d.Grammars[id]; !reflect.DeepEqual(got, w) {
			t.Errorf("%s = %+v\nwant   %+v", id, got, w)
		}
	}

	// Byte-stable, and re-reading the output as the prior file reproduces it.
	a, err := MarshalPrefill(d)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := LoadCatalogue(fixtureCatalogue)
	prior, err := ParseGrammarPrefill(a)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := PrefillLockRows(fixtureQueue(), fixtureLock(), cat, prior, fixtureRev)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := MarshalPrefill(d2)
	if !bytes.Equal(a, b) {
		t.Errorf("regenerating from the written file changed it:\n%s\nvs\n%s", a, b)
	}
	if !bytes.HasSuffix(a, []byte("}\n")) || bytes.Contains(a, []byte("\t")) {
		t.Errorf("output is not two-space indented JSON with a trailing newline")
	}

	// Row shape: fill the digests the build step owns and hand the row to
	// VerifyGrammarLock. It refuses a row exactly when a flag says it would.
	for _, id := range wantIDs {
		dr, err := d.Draft(id)
		if err != nil {
			t.Fatalf("Draft(%s): %v", id, err)
		}
		verr := verifyDraftRow(t, dr.Row)
		blocking := false
		for _, f := range d.Grammars[id].Flags {
			if !strings.HasPrefix(f, "grammar-differs:") {
				blocking = true
			}
		}
		if blocking && verr == nil {
			t.Errorf("%s: flags %v but VerifyGrammarLock accepts the row", id, d.Grammars[id].Flags)
		}
		if !blocking && verr != nil {
			t.Errorf("%s: no blocking flag but VerifyGrammarLock refuses the row: %v", id, verr)
		}
	}
}

// verifyDraftRow fills what build.sh owns (digests, sizes) with valid values
// and runs the lock's own verifier over a lock holding just this row.
func verifyDraftRow(t *testing.T, row tsengine.GrammarLockRow) error {
	t.Helper()
	raw := []byte("\x00asm\x01\x00\x00\x00")
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	sum := sha256.Sum256(raw)
	row.TarballSHA256 = strings.Repeat("a", 64)
	row.BlobSHA256 = hex.EncodeToString(sum[:])
	row.BlobBytes = int64(len(raw))
	row.GzBytes = int64(gz.Len())
	if row.Blob == "" {
		return errors.New("draft row has no blob")
	}
	lock := tsengine.GrammarLock{
		Core:      tsengine.LockCore{Repo: "tree-sitter/tree-sitter", Tag: "v0.26.11", Commit: strings.Repeat("c", 40), TarballSHA256: strings.Repeat("b", 64), Licence: "MIT"},
		Toolchain: tsengine.LockToolchain{Zig: "0.16.0", WasmOpt: "129"},
		Grammars:  []tsengine.GrammarLockRow{row},
	}
	dir := fstest.MapFS{row.Blob + ".wasm.gz": &fstest.MapFile{Data: gz.Bytes()}}
	return tsengine.VerifyGrammarLock(lock, dir)
}

// TestPrefillDraft: the draft row's fields (tag is the commit, entry point
// from c_symbol, subdirs from the directory), the verdict commit standing in
// for a missing catalogue commit, and the refusal of a queue grammar whose
// own verdict is refused.
func TestPrefillDraft(t *testing.T) {
	d := fixtureData(t)

	dr, err := d.Draft("beta")
	if err != nil {
		t.Fatal(err)
	}
	wantRow := tsengine.GrammarLockRow{
		Blob: "beta", Repo: "acme2/tree-sitter-beta", Tag: strings.Repeat("2", 40), Commit: strings.Repeat("2", 40), Licence: "Apache-2.0",
		EntryPoints: []string{"tree_sitter_beta_lang"}, Subdirs: []string{"grammars/beta"}, Generate: true, ABI: 15,
	}
	if !reflect.DeepEqual(dr.Row, wantRow) {
		t.Errorf("beta row = %+v, want %+v", dr.Row, wantRow)
	}
	if dr.Refused {
		t.Errorf("beta is not refused: %v", dr.Reasons)
	}

	dr, err = d.Draft("objective-c")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dr.Row.EntryPoints, []string{"tree_sitter_objc"}) || dr.Row.Blob != "objective-c" {
		t.Errorf("objective-c row = %+v (the entry point is the catalogue's name)", dr.Row)
	}

	dr, err = d.Draft("declared")
	if err != nil {
		t.Fatal(err)
	}
	if dr.Row.Commit != strings.Repeat("d", 40) || dr.Row.Tag != dr.Row.Commit || dr.Row.Licence != "MIT" || dr.Refused {
		t.Errorf("declared row = %+v refused=%v: the verdict's commit stands in when the catalogue has none", dr.Row, dr.Refused)
	}

	for _, id := range []string{"gamma", "delta", "theta", "pl"} {
		dr, err = d.Draft(id)
		if err != nil {
			t.Fatal(err)
		}
		if !dr.Refused || len(dr.Reasons) == 0 {
			t.Errorf("%s: refused=%v reasons=%v, want refused with a reason", id, dr.Refused, dr.Reasons)
		}
	}
	if dr, _ := d.Draft("pl"); !strings.Contains(strings.Join(dr.Reasons, "\n"), "AGPL-3.0") || !strings.Contains(strings.Join(dr.Reasons, "\n"), "other/tree-sitter-pl") {
		t.Errorf("pl reasons should name the refused AGPL grammar and the catalogue alternative: %v", dr.Reasons)
	}

	if _, err := d.Draft("alpha-not-there"); err == nil {
		t.Error("Draft of an id with no entry: want an error")
	}
}

// TestPrefillRejectsBadHandKeptData: a typo in the override map or a verdict
// for an id that is not a queue item fails loudly instead of dropping.
func TestPrefillRejectsBadHandKeptData(t *testing.T) {
	cat, err := LoadCatalogue(fixtureCatalogue)
	if err != nil {
		t.Fatal(err)
	}
	bad := []struct {
		name  string
		prior GrammarPrefill
		rev   string
	}{
		{"override to a name not in the catalogue", GrammarPrefill{Overrides: map[string]string{"alpha": "nope"}}, fixtureRev},
		{"override for a non-queue id", GrammarPrefill{Overrides: map[string]string{"ghost": "alpha"}}, fixtureRev},
		{"verdict for a non-queue id", GrammarPrefill{Verdicts: map[string]PinVerdict{"ghost": {}}}, fixtureRev},
		{"revision not 40 hex", GrammarPrefill{}, "abc123"},
	}
	for _, c := range bad {
		if _, err := PrefillLockRows(fixtureQueue(), fixtureLock(), cat, c.prior, c.rev); err == nil {
			t.Errorf("%s: want an error", c.name)
		}
	}
}

// TestGrammarPrefillDataMatchesQueue: the committed pre-fill file. It does
// not require every queue item (a later wave adds lock rows and regenerates
// it), but what it holds must be well formed and consistent with the queue and
// the licence allowlist.
func TestGrammarPrefillDataMatchesQueue(t *testing.T) {
	q, err := LoadQueue("../../docs/plans/language-breadth.queue.json")
	if err != nil {
		t.Fatal(err)
	}
	queueIDs := map[string]QueueEntry{}
	for _, e := range q {
		queueIDs[e.ID] = e
	}
	b, err := os.ReadFile("../../" + GrammarPrefillPath)
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseGrammarPrefill(b)
	if err != nil {
		t.Fatal(err)
	}
	if !hex40.MatchString(d.Revision) {
		t.Errorf("revision %q is not 40 lowercase hex", d.Revision)
	}
	if d.Source != CatalogueSource || d.Licence != CatalogueLicence || !tsengine.LicenceAllowed(d.Licence) {
		t.Errorf("source %q licence %q", d.Source, d.Licence)
	}
	if len(d.Grammars) == 0 {
		t.Fatal("no grammars pre-filled")
	}
	for id, name := range d.Overrides {
		if _, ok := queueIDs[id]; !ok {
			t.Errorf("override key %q is not a queue id", id)
		}
		if name == "" {
			t.Errorf("override %q maps to nothing", id)
		}
	}
	for id, v := range d.Verdicts {
		e, ok := queueIDs[id]
		if !ok {
			t.Errorf("verdict key %q is not a queue id", id)
			continue
		}
		if !hex40.MatchString(v.Commit) || v.Repo == "" || v.Basis == "" {
			t.Errorf("verdict %s incomplete: %+v", id, v)
		}
		if !strings.EqualFold(v.Repo, queueRepo(firstToken(e.GrammarSource))) {
			t.Errorf("verdict %s is for %s, but the queue's grammar is %s (a verdict is the queue grammar's own)", id, v.Repo, e.GrammarSource)
		}
		if v.Refused != (v.Flag() != "") {
			t.Errorf("verdict %s: refused=%v but Flag()=%q", id, v.Refused, v.Flag())
		}
		if v.Refused == tsengine.LicenceAllowed(v.Licence) && v.Licence != "" {
			t.Errorf("verdict %s: licence %q with refused=%v disagrees with the allowlist", id, v.Licence, v.Refused)
		}
	}
	for id, e := range d.Grammars {
		qe, ok := queueIDs[id]
		if !ok {
			t.Errorf("grammar key %q is not a queue id", id)
			continue
		}
		if qe.Engine != "tags" && qe.Engine != "tags+doc" {
			t.Errorf("%s: engine %q is not a tags one", id, qe.Engine)
		}
		if e.Commit != "" && !hex40.MatchString(e.Commit) {
			t.Errorf("%s: commit %q is not 40 lowercase hex", id, e.Commit)
		}
		if !sort.StringsAreSorted(e.Flags) {
			t.Errorf("%s: flags %v not sorted", id, e.Flags)
		}
		has := func(f string) bool {
			for _, g := range e.Flags {
				if g == f {
					return true
				}
			}
			return false
		}
		switch {
		case e.Licence == "":
			if !has("unlicensed") {
				t.Errorf("%s: no licence but no unlicensed flag: %v", id, e.Flags)
			}
		case !tsengine.LicenceAllowed(e.Licence):
			if !has("refused:" + e.Licence) {
				t.Errorf("%s: licence %q is off the allowlist but flags are %v", id, e.Licence, e.Flags)
			}
		default:
			if has("unlicensed") || has("refused:"+e.Licence) {
				t.Errorf("%s: licence %q is allowed but flags are %v", id, e.Licence, e.Flags)
			}
		}
		for _, f := range e.Flags {
			if strings.HasPrefix(f, "refused:") && strings.TrimPrefix(f, "refused:") != e.Licence {
				t.Errorf("%s: flag %s does not name the entry's licence %q", id, f, e.Licence)
			}
		}
		if has("not-in-catalogue") != (e.Catalogue == "") {
			t.Errorf("%s: not-in-catalogue flag and catalogue name %q disagree", id, e.Catalogue)
		}
		if e.Catalogue != "" {
			if e.Repo == "" || e.Commit == "" || e.ABI == 0 {
				t.Errorf("%s: a catalogue entry needs repo, commit and abi: %+v", id, e)
			}
			if outside := e.ABI < 13 || e.ABI > 15; outside != has("abi-outside:"+strconv.Itoa(e.ABI)) {
				t.Errorf("%s: abi %d and the abi-outside flag disagree: %v", id, e.ABI, e.Flags)
			}
			same := strings.EqualFold(e.Repo, queueRepo(firstToken(qe.GrammarSource)))
			differs := false
			for _, f := range e.Flags {
				if strings.HasPrefix(f, "grammar-differs:") {
					differs = true
				}
			}
			if same == differs {
				t.Errorf("%s: repo %s vs queue %q but grammar-differs flag is %v", id, e.Repo, qe.GrammarSource, differs)
			}
		}
	}
}

// TestGrammarLicenceVerdict: the recorded evidence and every rule of the
// verdict at pin (design §8.2): a top-level LICENSE decides by its text; with
// none, a licence declared consistently in the manifests counts; AGPL is
// always refused; a disagreeing licence file under queries/ or src/ refuses.
func TestGrammarLicenceVerdict(t *testing.T) {
	cases := []struct {
		tree    string
		licence string
		basis   string
		refused bool
		flag    string
		reason  string // substring
	}{
		{"mit", "MIT", BasisFile, false, "", ""}, // package.json says Apache-2.0: the LICENSE decides
		{"agpl", "AGPL-3.0", BasisFile, true, "refused:AGPL-3.0", "AGPL is refused"},
		{"declared-agree", "MIT", BasisDeclared, false, "", ""},
		{"declared-one", "Apache-2.0", BasisDeclared, false, "", ""},
		{"declared-disagree", "", "", true, "unlicensed", "disagree"},
		{"none", "", "", true, "unlicensed", "no LICENSE"},
		{"declared-agpl", "AGPL-3.0-or-later", BasisDeclared, true, "refused:AGPL-3.0-or-later", "AGPL is refused"},
		{"declared-gpl", "GPL-3.0", BasisDeclared, true, "refused:GPL-3.0", "allowlist"},
		{"declared-unreadable", "", "", true, "unlicensed", "package.json"},
		{"queries-mismatch", "", "", true, "unlicensed", "queries/LICENSE"},
		{"src-agpl", "AGPL-3.0", BasisFile, true, "refused:AGPL-3.0", "src/COPYING"},
		{"queries-agree", "MIT", BasisFile, false, "", ""},
		{"dual", "Apache-2.0", BasisFile, false, "", "LICENSE-MIT"},
		{"unclear", "", "", true, "unlicensed", "not recognised"},
	}
	for _, c := range cases {
		files, err := LicenceTreeFiles(os.DirFS(filepath.Join("testdata", "grammarpin", "trees", c.tree)))
		if err != nil {
			t.Fatalf("%s: %v", c.tree, err)
		}
		v := GrammarLicenceVerdict(files)
		if v.Licence != c.licence || v.Basis != c.basis || v.Refused != c.refused || v.Flag() != c.flag {
			t.Errorf("%s = %+v (flag %q), want licence %q basis %q refused %v flag %q", c.tree, v, v.Flag(), c.licence, c.basis, c.refused, c.flag)
		}
		if !strings.Contains(v.Reason, c.reason) {
			t.Errorf("%s reason %q lacks %q", c.tree, v.Reason, c.reason)
		}
		if v.Refused && v.Reason == "" {
			t.Errorf("%s: a refusal must say why", c.tree)
		}
	}
}

// TestLicenceTreeFromTarball: a GitHub-style tarball (one top directory) reads
// as the same tree, gzipped or not.
func TestLicenceTreeFromTarball(t *testing.T) {
	for _, tree := range []string{"mit", "declared-agree", "queries-mismatch"} {
		root := filepath.Join("testdata", "grammarpin", "trees", tree)
		want, err := LicenceTreeFiles(os.DirFS(root))
		if err != nil {
			t.Fatal(err)
		}
		build := func(prefix string) []byte {
			var tarBuf bytes.Buffer
			tw := tar.NewWriter(&tarBuf)
			_ = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": strings.Repeat("c", 40)}})
			_ = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: prefix, Mode: 0o755})
			// A vendored dependency's licence under node_modules/ is not the grammar's.
			b := []byte("GNU AFFERO GENERAL PUBLIC LICENSE Version 3")
			_ = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: prefix + "node_modules/dep/src/LICENSE", Mode: 0o644, Size: int64(len(b))})
			_, _ = tw.Write(b)
			_ = fs.WalkDir(os.DirFS(root), ".", func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				b, _ := os.ReadFile(filepath.Join(root, p))
				_ = tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: prefix + p, Mode: 0o644, Size: int64(len(b))})
				_, _ = tw.Write(b)
				return nil
			})
			_ = tw.Close()
			return tarBuf.Bytes()
		}
		gz := func(b []byte) []byte {
			var gzBuf bytes.Buffer
			zw := gzip.NewWriter(&gzBuf)
			_, _ = zw.Write(b)
			_ = zw.Close()
			return gzBuf.Bytes()
		}
		nested, flat := build("acme-tree-sitter-x-abc1234/"), build("")
		for name, data := range map[string][]byte{"tar": nested, "tar.gz": gz(nested), "flat tar": flat, "flat tar.gz": gz(flat)} {
			got, err := LicenceTreeFilesTar(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("%s %s: %v", tree, name, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s %s: files %v, want %v", tree, name, keys(got), keys(want))
			}
		}
	}
}

func keys(m map[string][]byte) []string {
	var k []string
	for p := range m {
		k = append(k, p)
	}
	sort.Strings(k)
	return k
}

// TestIdentifyLicenceText: every allowlisted licence, and the refused
// families, identified from their text (§8.2).
func TestIdentifyLicenceText(t *testing.T) {
	mitBody := `Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction. The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software. THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.`
	iscBody := `Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted, provided that the above copyright notice and this permission notice appear in all copies.`
	bsd2 := `Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.`
	cases := []struct{ name, text, want string }{
		{"MIT", "MIT License\n\nCopyright (c) 2026 X\n\n" + mitBody, "MIT"},
		{"MIT-0", strings.Replace(mitBody, "The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.", "", 1), "MIT-0"},
		{"Apache-2.0", "                                 Apache License\n                           Version 2.0, January 2004", "Apache-2.0"},
		{"BSD-2-Clause", bsd2, "BSD-2-Clause"},
		{"BSD-3-Clause", bsd2 + "\n3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.", "BSD-3-Clause"},
		{"BSD-4-Clause", bsd2 + "\n3. All advertising materials mentioning features or use of this software must display the following acknowledgement.\n4. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products.", "BSD-4-Clause"},
		{"ISC", "ISC License\n\nCopyright (c) 2026 X\n\n" + iscBody, "ISC"},
		{"0BSD", "Zero-Clause BSD\n\nPermission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted.", "0BSD"},
		{"Unlicense", "This is free and unencumbered software released into the public domain.\n\nAnyone is free to copy, modify, publish, use, compile, sell, or distribute this software.", "Unlicense"},
		{"CC0-1.0", "CC0 1.0 Universal\n\nStatement of Purpose", "CC0-1.0"},
		{"Zlib", "This software is provided 'as-is', without any express or implied warranty. In no event will the authors be held liable for any damages arising from the use of this software. 3. This notice may not be removed. 2. Altered source versions must be plainly marked as such.", "Zlib"},
		{"MPL-2.0", "Mozilla Public License Version 2.0\n==================================", "MPL-2.0"},
		{"MPL-1.1", "Mozilla Public License Version 1.1", "MPL-1.1"},
		{"GPL-3.0", "                    GNU GENERAL PUBLIC LICENSE\n                       Version 3, 29 June 2007\n ... you have permission to link or combine any covered work with a work licensed under version 3 of the GNU Affero General Public License", "GPL-3.0"},
		{"GPL-2.0", "GNU GENERAL PUBLIC LICENSE\nVersion 2, June 1991\n ... use the GNU Lesser General Public License instead", "GPL-2.0"},
		{"LGPL-3.0", "GNU LESSER GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007\n This version of the GNU Lesser General Public License incorporates the terms of version 3 of the GNU General Public License", "LGPL-3.0"},
		{"LGPL-2.1", "GNU LESSER GENERAL PUBLIC LICENSE\nVersion 2.1, February 1999", "LGPL-2.1"},
		{"AGPL-3.0", "GNU AFFERO GENERAL PUBLIC LICENSE\nVersion 3, 19 November 2007\n ... GNU General Public License ...", "AGPL-3.0"},
		{"SSPL-1.0", "Server Side Public License\nVERSION 1, OCTOBER 16, 2018", "SSPL-1.0"},
		{"Commons-Clause", mitBody + "\n\n\"Commons Clause\" License Condition v1.0", "Commons-Clause"},
		{"WTFPL", "DO WHAT THE FUCK YOU WANT TO PUBLIC LICENSE", "WTFPL"},
		{"unrecognised", "All rights reserved.", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := IdentifyLicenceText(c.text); got != c.want {
			t.Errorf("%s: identified %q, want %q", c.name, got, c.want)
		}
	}
}
