package designconvert

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCanonicalTranslationDecoder(t *testing.T) {
	source := []byte("<p>Original <strong>value</strong></p>")
	result, err := Translate(Source{"marketing@" + strings.Repeat("a", 64), "one.html", digest(source), len(source)}, source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Transformer != CurrentTransformer() {
		t.Fatal("current transformer helper drifted from emitter")
	}
	body, _ := json.Marshal(result)
	decoded, err := DecodeCanonical(body)
	if err != nil || VerifyReplay(decoded, source) != nil {
		t.Fatalf("canonical replay: %v", err)
	}
	for name, invalid := range map[string][]byte{
		"duplicate":    bytes.Replace(body, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		"case alias":   bytes.Replace(body, []byte(`"schema_version"`), []byte(`"Schema_Version"`), 1),
		"unknown":      append(append([]byte{}, body[:len(body)-1]...), []byte(`,"unknown":true}`)...),
		"trailing":     append(append([]byte{}, body...), []byte(" {}")...),
		"whitespace":   append([]byte(" "), body...),
		"invalid UTF8": append(append([]byte{}, body...), 0xff),
		"oversize":     bytes.Repeat([]byte(" "), MaxTranslationBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeCanonical(invalid); err == nil {
				t.Fatal("accepted noncanonical/invalid stored bytes")
			}
		})
	}
	for _, mutate := range []func(*Translation){
		func(r *Translation) { r.Artifact.Content += "changed" },
		func(r *Translation) { r.Source.Path = ".." },
		func(r *Translation) { r.Artifact.Mappings[0].OutputEnd = len(r.Artifact.Content) + 1 },
		func(r *Translation) { r.Artifact.Mappings[0].SourceEnd = r.Source.Size + 1 },
		func(r *Translation) { r.Verification = "verified" },
	} {
		var changed Translation
		_ = json.Unmarshal(body, &changed)
		mutate(&changed)
		b, _ := json.Marshal(changed)
		if _, err := DecodeCanonical(b); err == nil {
			t.Fatal("accepted invalid stored structure")
		}
	}
	// Historical same-schema attempts remain readable, but are not current replay.
	result.Transformer.GoVersion = "go1.25.0"
	result.Transformer.HTMLParser = "golang.org/x/net@v0.53.0"
	result.Transformer.RequiredTempl = "github.com/a-h/templ@v0.3.1000"
	b, _ := json.Marshal(result)
	old, err := DecodeCanonical(b)
	if err != nil || old.Verification != "not-verified" {
		t.Fatalf("historical integrity rejected: %v", err)
	}
	if VerifyReplay(old, source) == nil {
		t.Fatal("stale transformer passed current replay")
	}
	result.Transformer.HTMLParser = "unrelated.example/module@v0.53.0"
	b, _ = json.Marshal(result)
	if _, err := DecodeCanonical(b); err == nil {
		t.Fatal("historical envelope admitted an unrelated module")
	}
}

func TestTranslationPreflightBoundsBeforeTypedDecode(t *testing.T) {
	for _, body := range []string{
		`{"artifact":{"mappings":[` + strings.Repeat("{},", maxTokens) + `{}]}}`,
		`{"diagnostics":[` + strings.Repeat("{},", MaxDiagnostics) + `{}]}`,
		strings.Repeat(`{"nested":`, 10) + `0` + strings.Repeat("}", 10),
		`{"artifact":{"mappings":[]},"artifact":{"mappings":[]}}`,
	} {
		if err := preflightTranslationJSON([]byte(body)); err == nil {
			t.Fatal("unbounded/ambiguous JSON reached typed decode")
		}
	}
}
