package extractor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/fstest"
)

func secretPackFS() fstest.MapFS {
	return fstest.MapFS{
		"dialects/secret-spec/dialect.json": {Data: []byte(`{
  "id": "secret-spec", "display": "x", "kind": "tooling", "engine": "doc", "lang_tag": "secret-spec",
  "match": {"extensions": [], "basenames": ["secrets.sample.json"], "globs": [], "shebangs": [], "sniff": null},
  "variants": [],
  "kinds": {"secret": {"source_kind": "SecretSpecEntry", "canonical": "key"}, "plain": {"source_kind": "SecretSpecPlain", "canonical": "key"}},
  "required_kinds": ["secret"], "oracle_kinds": {}, "name_match": "identity",
  "package": {"strategy": "none"}, "signature": {"max_chars": 200}, "target_level": "L2", "queue_id": "secret-spec"
}`)},
		"dialects/secret-spec/doc.json": {Data: []byte(`{
  "decode": "json",
  "detect": {"basenames": ["secrets.sample.json"]},
  "entities": [
    {"select": "$.secrets.*", "where": {"is_map": true}, "kind": "key", "source_kind": "SecretSpecEntry", "name": "$key", "sensitive": true},
    {"select": "$.plain.*", "where": {"is_map": true}, "kind": "key", "source_kind": "SecretSpecPlain", "name": "$key"}
  ]
}`)},
	}
}

func TestPackCanonicalKinds(t *testing.T) {
	p, err := NewPack(secretPackFS())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := CanonicalKind("SecretSpecEntry"); ok {
		t.Fatal("the package function learned a pack's kind")
	}
	if c, ok := p.CanonicalKind("SecretSpecEntry"); !ok || c != "key" {
		t.Errorf("pack CanonicalKind = %q, %v", c, ok)
	}
	if c, ok := p.CanonicalKind("TerraformResource"); !ok || c != "resource" {
		t.Errorf("a built-in kind through the pack = %q, %v", c, ok)
	}
	if _, ok := p.CanonicalKind("NoSuchKind"); ok {
		t.Error("an undeclared kind has a canonical kind")
	}
	if SensitiveKind("SecretSpecEntry") {
		t.Error("the package function learned a pack's sensitive kind")
	}
	built, err := p.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := built["secret-spec"].(*DocExtractor); !ok {
		t.Fatalf("built %T", built["secret-spec"])
	}
}

func TestPackAddNativeRules(t *testing.T) {
	mk := func(id string) Dialect {
		return Dialect{ID: id, Display: id, Kind: "language", Match: DialectMatch{Extensions: []string{".zzq"}},
			Kinds: map[string]DialectKind{"Thing": {SourceKind: "Thing", Canonical: "class"}}}
	}
	reason := func(err error) string {
		var pe *PackError
		if !errors.As(err, &pe) {
			t.Fatalf("%v is not a PackError", err)
		}
		return pe.Reason
	}
	p, err := NewPack(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.AddNative(mk("zzq")); err != nil {
		t.Fatal(err)
	}
	if c, ok := p.CanonicalKind("Thing"); !ok || c != "class" {
		t.Errorf("native kind: %q %v", c, ok)
	}
	dup := mk("zzq2") // same extension as zzq
	dup.Kinds = map[string]DialectKind{"Other": {SourceKind: "Other", Canonical: "class"}}
	if r := reason(p.AddNative(dup)); r != PackConflict {
		t.Errorf("same extension: %s", r)
	}
	if r := reason(p.AddNative(mk("python"))); r != PackConflict {
		t.Errorf("built-in id: %s", r)
	}
	noKinds := mk("zzr")
	noKinds.Match.Extensions = []string{".zzr"}
	noKinds.Kinds = nil
	if r := reason(p.AddNative(noKinds)); r != PackManifest {
		t.Errorf("no kinds: %s", r)
	}
	noClaim := mk("zzs")
	noClaim.Match.Extensions = nil
	if r := reason(p.AddNative(noClaim)); r != PackClaims {
		t.Errorf("claims nothing: %s", r)
	}
	badKind := mk("zzt")
	badKind.Match.Extensions = []string{".zzt"}
	badKind.Kind = "weird"
	if r := reason(p.AddNative(badKind)); r != PackManifest {
		t.Errorf("kind: %s", r)
	}
}

// N9: auditPackFiles names a variant's tags query and nothing else of the
// variant, because a variant can carry no other query file
// (DialectVarQueries has the one field), so a valid pack is never refused for
// a file it must hold. A field added to it must be named by auditPackFiles.
func TestVariantQueriesCarryOnlyATagsQuery(t *testing.T) {
	typ := reflect.TypeOf(DialectVarQueries{})
	if typ.NumField() != 1 || typ.Field(0).Name != "Tags" {
		t.Fatalf("DialectVarQueries has %d fields: name each new query file in auditPackFiles, then update this test", typ.NumField())
	}
}
