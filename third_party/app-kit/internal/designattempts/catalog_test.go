package designattempts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

func metadataFixture(n int) Entry {
	return Entry{
		ID:     fmt.Sprintf("syntax@%064x", n),
		Source: designconvert.Source{ArchiveID: "studio@" + strings.Repeat("a", 64), Path: "html/page.html", SHA256: strings.Repeat("b", 64), Size: 12},
		Target: "go-templ", Transformer: designconvert.Transformer{ID: designconvert.RuleID, SHA256: strings.Repeat("c", 64), GoVersion: "historical-go", HTMLParser: designconvert.HTMLParserVersion, RequiredTempl: designconvert.RequiredTempl},
		Status: "translated", Verification: "not-verified", ArtifactSHA256: strings.Repeat("d", 64), DiagnosticCodes: []string{},
	}
}

func unsupportedFixture(n int) Entry {
	e := metadataFixture(n)
	e.Status, e.ArtifactSHA256 = "unsupported", ""
	e.DiagnosticCodes, e.OmittedDiagnostics = []string{"unsupported-element", "unsupported-element"}, 2
	return e
}

func metadataCatalog(t *testing.T, entries ...Entry) (*Catalog, []byte) {
	t.Helper()
	body, err := encodeCatalog(entries)
	if err != nil {
		t.Fatal(err)
	}
	c, err := decodeCatalog(body)
	if err != nil {
		t.Fatal(err)
	}
	return c, body
}

func TestMetadataCanonicalIdentityAndHistory(t *testing.T) {
	a, b := metadataFixture(1), unsupportedFixture(2)
	input := []Entry{b, a}
	c, body := metadataCatalog(t, input...)
	_, same := metadataCatalog(t, a, b)
	if !bytes.Equal(body, same) || input[0].ID != b.ID || bytes.HasSuffix(body, []byte("\n")) {
		t.Fatal("encoding changed input order or is not deterministic compact JSON")
	}
	canonical, _ := json.Marshal(c)
	if !bytes.Equal(canonical, body) || c.SchemaVersion != 1 || len(c.Revision) != 64 {
		t.Fatal("catalog encoding is not canonical")
	}
	identity, _ := json.Marshal(struct {
		SchemaVersion int     `json:"schema_version"`
		Entries       []Entry `json:"entries"`
	}{1, c.Entries})
	if c.Revision != digest(identity) {
		t.Fatal("revision is not the canonical version-and-entry hash")
	}
	newer := a
	newer.ID = fmt.Sprintf("syntax@%064x", 3)
	newer.Transformer.GoVersion, newer.Transformer.SHA256 = "different-go", strings.Repeat("f", 64)
	changed, _ := metadataCatalog(t, a, b, newer)
	if changed.Revision == c.Revision || len(changed.List()) != 3 {
		t.Fatal("new attempt replaced history or reused old revision")
	}
	if old, ok := changed.Get(a.ID); !ok || !reflect.DeepEqual(old, a) {
		t.Fatal("old historical metadata disappeared")
	}
	if c.Entries[0].Transformer.GoVersion != "historical-go" || c.Entries[0].Verification != "not-verified" {
		t.Fatal("metadata load invented current replay or compatibility")
	}
	empty, _ := metadataCatalog(t)
	page, err := empty.Search(Query{})
	if err != nil || page.Total != 0 || page.Entries == nil || page.NextCursor != "" {
		t.Fatal("empty store must return an explicit empty metadata page", err)
	}
}

func TestEntryForHashesCompleteCanonicalReceipt(t *testing.T) {
	for _, source := range []string{`<p>Hello</p>`, `<el-dialog></el-dialog><el-menu></el-menu>`} {
		body := []byte(source)
		origin := designconvert.Source{ArchiveID: "studio@" + strings.Repeat("a", 64), Path: "html/page.html", SHA256: digest(body), Size: len(body)}
		result, err := designconvert.Translate(origin, body)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		e := entryFor(result, canonical)
		if e.ID != "syntax@"+digest(canonical) || e.Source != origin || e.Transformer != result.Transformer || e.Verification != "not-verified" {
			t.Fatal("entry lost canonical receipt identity")
		}
		if result.Artifact != nil && e.ArtifactSHA256 != result.Artifact.SHA256 {
			t.Fatal("artifact hash changed")
		}
		for i, d := range result.Diagnostics {
			if e.DiagnosticCodes[i] != d.Code {
				t.Fatal("diagnostic order or duplicates changed")
			}
		}
		if len(result.Diagnostics) > 0 {
			before := slices.Clone(e.DiagnosticCodes)
			result.Diagnostics[0].Code = "changed"
			if !slices.Equal(e.DiagnosticCodes, before) {
				t.Fatal("entry aliases caller diagnostics")
			}
		}
		other := result
		other.Source.Path = "html/identical-copy.html"
		otherBody, _ := json.Marshal(other)
		if entryFor(other, otherBody).ID == e.ID {
			t.Fatal("distinct attribution collapsed to artifact-only identity")
		}
		metadataCatalog(t, e)
	}
}

func TestMetadataRejectsInvalidContracts(t *testing.T) {
	for name, change := range map[string]func(*Entry){
		"bad ID":                 func(e *Entry) { e.ID = "syntax@wrong" },
		"uppercase ID":           func(e *Entry) { e.ID = "syntax@" + strings.Repeat("A", 64) },
		"unknown ID family":      func(e *Entry) { e.ID = "target@" + strings.Repeat("a", 64) },
		"bad archive":            func(e *Entry) { e.Source.ArchiveID = "studio" },
		"parent path":            func(e *Entry) { e.Source.Path = "../page.html" },
		"parent only":            func(e *Entry) { e.Source.Path = ".." },
		"absolute path":          func(e *Entry) { e.Source.Path = "/page.html" },
		"backslash":              func(e *Entry) { e.Source.Path = `html\page.html` },
		"drive path":             func(e *Entry) { e.Source.Path = "C:page.html" },
		"normalized alias":       func(e *Entry) { e.Source.Path = "html/../page.html" },
		"path control":           func(e *Entry) { e.Source.Path = "html/\tpage.html" },
		"empty path":             func(e *Entry) { e.Source.Path = "" },
		"bad source hash":        func(e *Entry) { e.Source.SHA256 = "x" },
		"negative size":          func(e *Entry) { e.Source.Size = -1 },
		"oversized source":       func(e *Entry) { e.Source.Size = designconvert.MaxSourceBytes + 1 },
		"unknown target":         func(e *Entry) { e.Target = "react" },
		"unknown rule":           func(e *Entry) { e.Transformer.ID = "future-rule" },
		"bad transformer hash":   func(e *Entry) { e.Transformer.SHA256 = "bad" },
		"empty Go pin":           func(e *Entry) { e.Transformer.GoVersion = "" },
		"unknown parser pin":     func(e *Entry) { e.Transformer.HTMLParser = "future-parser" },
		"unknown templ pin":      func(e *Entry) { e.Transformer.RequiredTempl = "future-templ" },
		"unknown status":         func(e *Entry) { e.Status = "verified" },
		"invented proof":         func(e *Entry) { e.Verification = "verified" },
		"missing artifact":       func(e *Entry) { e.ArtifactSHA256 = "" },
		"translated diagnostics": func(e *Entry) { e.DiagnosticCodes = []string{"unsupported"} },
		"translated omissions":   func(e *Entry) { e.OmittedDiagnostics = 1 },
		"nil codes":              func(e *Entry) { e.DiagnosticCodes = nil },
		"negative omissions":     func(e *Entry) { e.OmittedDiagnostics = -1 },
		"bad utf8":               func(e *Entry) { e.Transformer.GoVersion = "\xff" },
		"unsupported artifact":   func(e *Entry) { *e = unsupportedFixture(1); e.ArtifactSHA256 = strings.Repeat("a", 64) },
		"unsupported no reason":  func(e *Entry) { *e = unsupportedFixture(1); e.DiagnosticCodes = []string{} },
		"empty diagnostic code":  func(e *Entry) { *e = unsupportedFixture(1); e.DiagnosticCodes = []string{""} },
	} {
		t.Run(name, func(t *testing.T) {
			e := metadataFixture(1)
			change(&e)
			if _, err := encodeCatalog([]Entry{e}); err == nil {
				t.Fatal("invalid metadata accepted")
			}
		})
	}
	if _, err := encodeCatalog([]Entry{metadataFixture(1), metadataFixture(1)}); err == nil {
		t.Fatal("duplicate IDs accepted")
	}
}

func TestMetadataStrictCanonicalDecoding(t *testing.T) {
	_, body := metadataCatalog(t, metadataFixture(1), unsupportedFixture(2))
	for name, mutant := range map[string][]byte{
		"whitespace":         append([]byte(" "), body...),
		"newline":            append(bytes.Clone(body), '\n'),
		"trailing value":     append(bytes.Clone(body), []byte("{}")...),
		"unknown field":      bytes.Replace(body, []byte(`{"schema_version":1`), []byte(`{"unknown":1,"schema_version":1`), 1),
		"duplicate field":    bytes.Replace(body, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		"case alias":         bytes.Replace(body, []byte(`"schema_version"`), []byte(`"SCHEMA_VERSION"`), 1),
		"nested case alias":  bytes.Replace(body, []byte(`"archive_id"`), []byte(`"ARCHIVE_ID"`), 1),
		"nested unknown":     bytes.Replace(body, []byte(`"path":"html/page.html"`), []byte(`"path":"html/page.html","body":"not allowed"`), 1),
		"duplicate nested":   bytes.Replace(body, []byte(`"path":"html/page.html"`), []byte(`"path":"html/page.html","path":"html/page.html"`), 1),
		"missing required":   bytes.Replace(body, []byte(`"verification":"not-verified",`), nil, 1),
		"bad UTF8":           bytes.Replace(body, []byte("historical-go"), []byte{0xff}, 1),
		"new schema":         bytes.Replace(body, []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1),
		"oversized revision": bytes.Replace(body, []byte(`"revision":"`), []byte(`"revision":"`+strings.Repeat("x", 1024)), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeCatalog(mutant); err == nil {
				t.Fatal("noncanonical metadata accepted")
			}
		})
	}
	var wire Catalog
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	wire.Revision = strings.Repeat("0", 64)
	forged, _ := json.Marshal(wire)
	if _, err := decodeCatalog(forged); err == nil {
		t.Fatal("stale metadata revision accepted")
	}
	slices.Reverse(wire.Entries)
	forged, _ = json.Marshal(wire)
	if _, err := decodeCatalog(forged); err == nil {
		t.Fatal("unsorted wire entries accepted")
	}
}

func TestMetadataResourcePreflight(t *testing.T) {
	if _, err := decodeCatalog(make([]byte, MaxCatalogBytes+1)); err == nil {
		t.Fatal("metadata byte bound ignored")
	}
	if _, err := encodeCatalog(make([]Entry, MaxEntries+1)); err == nil {
		t.Fatal("metadata entry bound ignored")
	}
	tooMany := []byte(`{"entries":[` + strings.Repeat(`{},`, MaxEntries) + `{}]}`)
	if _, err := decodeCatalog(tooMany); err == nil || !strings.Contains(err.Error(), "array") {
		t.Fatal("entry allocation was not bounded before typed decoding", err)
	}
	if err := preflightJSON([]byte(`{"entries":[{"diagnostic_codes":[` + strings.Repeat(`"x",`, designconvert.MaxDiagnostics) + `"x"]}]}`)); err == nil {
		t.Fatal("diagnostic array was not bounded before typed decoding")
	}
	if err := preflightJSON([]byte(strings.Repeat("[", 10) + "0" + strings.Repeat("]", 10))); err == nil {
		t.Fatal("deep JSON accepted")
	}
	for name, change := range map[string]func(*Entry){
		"path": func(e *Entry) { e.Source.Path = strings.Repeat("x", maxSourcePathBytes+1) },
		"archive": func(e *Entry) {
			e.Source.ArchiveID = strings.Repeat("a", maxArchiveIDBytes) + "@" + strings.Repeat("0", 64)
		},
		"Go pin": func(e *Entry) { e.Transformer.GoVersion = strings.Repeat("g", 257) },
		"code":   func(e *Entry) { *e = unsupportedFixture(1); e.DiagnosticCodes = []string{strings.Repeat("x", 129)} },
		"codes": func(e *Entry) {
			*e = unsupportedFixture(1)
			e.DiagnosticCodes = make([]string, designconvert.MaxDiagnostics+1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := metadataFixture(1)
			change(&e)
			if _, err := encodeCatalog([]Entry{e}); err == nil {
				t.Fatal("field bound ignored")
			}
		})
	}
	// Exercise inclusive count bounds without thousands of repeated CUE builds.
	entries := make([]Entry, MaxEntries)
	for i := range entries {
		entries[i] = metadataFixture(i + 1)
	}
	if err := validateEntries(entries); err != nil {
		t.Fatal("inclusive entry bound refused", err)
	}
	e := unsupportedFixture(1)
	e.DiagnosticCodes = make([]string, designconvert.MaxDiagnostics)
	for i := range e.DiagnosticCodes {
		e.DiagnosticCodes[i] = strings.Repeat("x", 128)
	}
	e.Source.Path = strings.Repeat("a", maxSourcePathBytes)
	e.Transformer.GoVersion = strings.Repeat("g", 256)
	metadataCatalog(t, e)
}

func TestMetadataDetachedReturns(t *testing.T) {
	original := unsupportedFixture(1)
	c, _ := metadataCatalog(t, original)
	c.Entries[0].Source.Path, c.Entries[0].DiagnosticCodes[0], c.Revision = "changed", "changed", "changed"
	list := c.List()
	list[0].DiagnosticCodes[0] = "changed"
	get, ok := c.Get(original.ID)
	if !ok {
		t.Fatal("missing entry")
	}
	get.DiagnosticCodes[0], get.Transformer.GoVersion = "changed", "changed"
	page, err := c.Search(Query{})
	if err != nil {
		t.Fatal(err)
	}
	page.Entries[0].DiagnosticCodes[0] = "changed"
	again, _ := c.Get(original.ID)
	if !reflect.DeepEqual(original, again) {
		t.Fatal("caller mutation changed snapshot")
	}
	page, _ = c.Search(Query{})
	if page.Revision == "changed" || !reflect.DeepEqual(page.Entries[0], original) {
		t.Fatal("public field/page mutation changed search")
	}
	var nilCatalog *Catalog
	if nilCatalog.List() == nil {
		t.Fatal("nil List must be explicit empty")
	}
	if _, ok := nilCatalog.Get(original.ID); ok {
		t.Fatal("nil Get found an entry")
	}
	if _, err := nilCatalog.Search(Query{}); err == nil {
		t.Fatal("nil Search accepted")
	}
}

func TestMetadataSearchFiltersPaginationAndCursors(t *testing.T) {
	a, b, c := metadataFixture(1), unsupportedFixture(2), metadataFixture(3)
	c.Source.Path = "html/other.html"
	cat, _ := metadataCatalog(t, c, a, b)
	for _, tc := range []struct {
		query Query
		ids   []string
	}{
		{Query{}, []string{a.ID, b.ID, c.ID}},
		{Query{ArchiveID: a.Source.ArchiveID, Path: a.Source.Path}, []string{a.ID, b.ID}},
		{Query{Status: "unsupported"}, []string{b.ID}},
		{Query{Status: "translated", Path: c.Source.Path}, []string{c.ID}},
		{Query{Path: "html/Page.html"}, []string{}},
		{Query{ArchiveID: "other@" + strings.Repeat("a", 64)}, []string{}},
	} {
		page, err := cat.Search(tc.query)
		if err != nil {
			t.Fatal(err)
		}
		ids := []string{}
		for _, e := range page.Entries {
			ids = append(ids, e.ID)
		}
		if !slices.Equal(ids, tc.ids) || page.Total != len(tc.ids) || page.Entries == nil {
			t.Fatalf("wrong exact-filter result: %+v", page)
		}
	}
	first, err := cat.Search(Query{Limit: 1})
	if err != nil || first.NextCursor == "" {
		t.Fatal("missing bounded continuation", err)
	}
	second, err := cat.Search(Query{Limit: 1, Cursor: first.NextCursor})
	if err != nil || second.Entries[0].ID != b.ID {
		t.Fatal("bad continuation", err)
	}
	last, err := cat.Search(Query{Limit: 1, Cursor: second.NextCursor})
	if err != nil || last.Entries[0].ID != c.ID || last.NextCursor != "" {
		t.Fatal("bad final page", err)
	}
	for _, query := range []Query{
		{Limit: -1}, {Limit: 51}, {Status: "verified"}, {ArchiveID: "bad"}, {Path: "../escape"}, {Path: "html//page.html"},
		{Path: strings.Repeat("a", 1025)}, {ArchiveID: strings.Repeat("a", 257)}, {Cursor: strings.Repeat("a", 2049)},
		{Cursor: "not base64"}, {Limit: 2, Cursor: first.NextCursor}, {Limit: 1, Status: "translated", Cursor: first.NextCursor},
		{Limit: 1, Cursor: first.NextCursor[:20] + "\r\n" + first.NextCursor[20:]},
	} {
		if _, err := cat.Search(query); err == nil {
			t.Fatalf("invalid query/cursor accepted: %+v", query)
		}
	}
	newer, _ := metadataCatalog(t, a, b, c, metadataFixture(4))
	if _, err := newer.Search(Query{Limit: 1, Cursor: first.NextCursor}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatal("old revision cursor accepted", err)
	}
	decoded, _ := base64.RawURLEncoding.DecodeString(first.NextCursor)
	var continuationValue continuation
	if err := json.Unmarshal(decoded, &continuationValue); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{-1, 0, 3, 100} {
		continuationValue.Offset = offset
		body, _ := json.Marshal(continuationValue)
		if _, err := cat.Search(Query{Limit: 1, Cursor: base64.RawURLEncoding.EncodeToString(body)}); err == nil {
			t.Fatal("out-of-range cursor accepted")
		}
	}
	for _, body := range [][]byte{append(bytes.Clone(decoded), ' '), bytes.Replace(decoded, []byte(`"v":1`), []byte(`"v":1,"v":1`), 1), bytes.Replace(decoded, []byte(`"v":1`), []byte(`"v":2`), 1)} {
		if _, err := cat.Search(Query{Limit: 1, Cursor: base64.RawURLEncoding.EncodeToString(body)}); err == nil {
			t.Fatal("noncanonical cursor accepted")
		}
	}
}
