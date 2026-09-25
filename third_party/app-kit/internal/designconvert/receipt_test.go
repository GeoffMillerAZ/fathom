package designconvert

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func receiptSource(body []byte, member string) Source {
	return Source{ArchiveID: "marketing@" + strings.Repeat("a", 64), Path: member, SHA256: digest(body), Size: len(body)}
}

func TestTranslationIdentityReplayAndRefusal(t *testing.T) {
	for _, test := range []struct{ name, member, body, status string }{
		{"static", "html/example.html", `<p class="dark:text-white  text-black">A &amp; B</p>`, "translated"},
		{"react", "demo/page.tsx", `export default () => <p>Hello</p>`, "unsupported"},
		{"runtime", "html/menu.html", `<el-menu><button>Open</button></el-menu>`, "unsupported"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := []byte(test.body)
			source := receiptSource(body, test.member)
			first, err := Translate(source, body)
			if err != nil {
				t.Fatal(err)
			}
			second, err := Translate(source, body)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(first)
			b, _ := json.Marshal(second)
			if !bytes.Equal(a, b) || first.Status != test.status || first.Verification != "not-verified" {
				t.Fatalf("nondeterministic or incorrect assessment: %s", a)
			}
			if first.Source != source || first.Transformer.SHA256 != ruleDigest() {
				t.Fatal("lost original/rule identity")
			}
			if (first.Artifact != nil) != (test.status == "translated") {
				t.Fatal("unsupported output leaked or translated artifact missing")
			}
			if first.Artifact != nil && first.Artifact.SHA256 != digest([]byte(first.Artifact.Content)) {
				t.Fatal("incorrect artifact digest")
			}
			if err := VerifyReplay(first, body); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReplayRejectsTamperingAndInventedProof(t *testing.T) {
	body := []byte(`<p class="dark:text-white">Stable</p>`)
	for name, mutate := range map[string]func(*Translation){
		"source hash":           func(r *Translation) { r.Source.SHA256 = strings.Repeat("0", 64) },
		"source parent path":    func(r *Translation) { r.Source.Path = ".." },
		"rule hash":             func(r *Translation) { r.Transformer.SHA256 = strings.Repeat("0", 64) },
		"toolchain":             func(r *Translation) { r.Transformer.GoVersion = "different-go" },
		"compiler":              func(r *Translation) { r.Transformer.RequiredTempl = "different-templ" },
		"invented verification": func(r *Translation) { r.Verification = "verified" },
		"status":                func(r *Translation) { r.Status = "verified" },
		"missing artifact":      func(r *Translation) { r.Artifact = nil },
		"artifact hash":         func(r *Translation) { r.Artifact.SHA256 = strings.Repeat("0", 64) },
		"changed output rehashed": func(r *Translation) {
			r.Artifact.Content = strings.ReplaceAll(r.Artifact.Content, "dark:text-white", "text-white")
			r.Artifact.SHA256 = digest([]byte(r.Artifact.Content))
		},
		"source map":      func(r *Translation) { r.Artifact.Mappings = []Mapping{} },
		"hidden omission": func(r *Translation) { r.OmittedDiagnostics = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			r, err := Translate(receiptSource(body, "html/page.html"), body)
			if err != nil {
				t.Fatal(err)
			}
			mutate(&r)
			if err := VerifyReplay(r, body); err == nil {
				t.Fatal("tampered translation passed replay")
			}
		})
	}
}

func TestTranslationRefusesInvalidOriginalIdentity(t *testing.T) {
	body := []byte(`<p>Original</p>`)
	for _, member := range []string{"", ".", "..", "../page.html", "/page.html", "html/../page.html", "html\\page.html", "a\n.html", "\xff.html"} {
		if _, err := Translate(receiptSource(body, member), body); err == nil {
			t.Errorf("accepted invalid path %q", member)
		}
	}
	for name, mutate := range map[string]func(*Source){
		"size":       func(s *Source) { s.Size++ },
		"hash":       func(s *Source) { s.SHA256 = strings.Repeat("0", 64) },
		"archive ID": func(s *Source) { s.ArchiveID = "mutable-name" },
	} {
		source := receiptSource(body, "html/page.html")
		mutate(&source)
		if _, err := Translate(source, body); err == nil {
			t.Errorf("accepted corrupt %s", name)
		}
	}
	for _, invalid := range [][]byte{{0xff}, {'a', 0, 'b'}, bytes.Repeat([]byte("a"), MaxSourceBytes+1)} {
		if _, err := Translate(receiptSource(invalid, "html/page.html"), invalid); err == nil {
			t.Error("accepted oversized or invalid UTF-8 input")
		}
	}
}

func TestReceiptBoundsRejectBeforeSchemaOrReplay(t *testing.T) {
	body := []byte(`<p>Bounded fixture</p>`)
	base, err := Translate(receiptSource(body, "html/page.html"), body)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Translation){
		"source.archive_id":          func(r *Translation) { r.Source.ArchiveID = strings.Repeat("a", maxArchiveIDBytes+1) },
		"source.path":                func(r *Translation) { r.Source.Path = strings.Repeat("a", maxSourcePathBytes+1) },
		"source.sha256":              func(r *Translation) { r.Source.SHA256 = strings.Repeat("a", 65) },
		"target":                     func(r *Translation) { r.Target = strings.Repeat("a", 129) },
		"transformer.id":             func(r *Translation) { r.Transformer.ID = strings.Repeat("a", 129) },
		"transformer.sha256":         func(r *Translation) { r.Transformer.SHA256 = strings.Repeat("a", 65) },
		"transformer.go_version":     func(r *Translation) { r.Transformer.GoVersion = strings.Repeat("a", 257) },
		"transformer.html_parser":    func(r *Translation) { r.Transformer.HTMLParser = strings.Repeat("a", 257) },
		"transformer.required_templ": func(r *Translation) { r.Transformer.RequiredTempl = strings.Repeat("a", 257) },
		"status":                     func(r *Translation) { r.Status = strings.Repeat("a", 33) },
		"verification":               func(r *Translation) { r.Verification = strings.Repeat("a", 33) },
		"diagnostics":                func(r *Translation) { r.Diagnostics = make([]Diagnostic, MaxDiagnostics+1) },
		"diagnostics.code":           func(r *Translation) { r.Diagnostics = []Diagnostic{{Code: strings.Repeat("a", 129)}} },
		"diagnostics.message":        func(r *Translation) { r.Diagnostics = []Diagnostic{{Message: strings.Repeat("a", 1025)}} },
		"artifact.filename":          func(r *Translation) { r.Artifact.Filename = strings.Repeat("a", 129) },
		"artifact.sha256":            func(r *Translation) { r.Artifact.SHA256 = strings.Repeat("a", 65) },
		"artifact.content":           func(r *Translation) { r.Artifact.Content = strings.Repeat("a", maxOutputBytes+1) },
		"artifact.mappings":          func(r *Translation) { r.Artifact.Mappings = make([]Mapping, maxTokens+1) },
	} {
		t.Run(name, func(t *testing.T) {
			r := base
			artifact := *base.Artifact
			r.Artifact = &artifact
			mutate(&r)
			for _, err := range []error{validateShape(r), VerifyReplay(r, body)} {
				if err == nil || !strings.HasPrefix(err.Error(), "designconvert: receipt limit: "+name+" exceeds ") {
					t.Fatalf("receipt did not stop at its pre-marshal limit: %v", err)
				}
			}
		})
	}
}

func TestReceiptBoundsAreInclusive(t *testing.T) {
	// Exercise only the resource preflight: schema validity and replay are separate
	// checks. Every variable-size field is exactly at its byte or count bound.
	r := Translation{
		Source: Source{ArchiveID: strings.Repeat("a", maxArchiveIDBytes),
			Path: strings.Repeat("a", maxSourcePathBytes), SHA256: strings.Repeat("a", 64)},
		Target: strings.Repeat("a", 128),
		Transformer: Transformer{ID: strings.Repeat("a", 128), SHA256: strings.Repeat("a", 64),
			GoVersion: strings.Repeat("a", 256), HTMLParser: strings.Repeat("a", 256), RequiredTempl: strings.Repeat("a", 256)},
		Status: strings.Repeat("a", 32), Verification: strings.Repeat("a", 32),
		Diagnostics: make([]Diagnostic, MaxDiagnostics),
		Artifact: &Artifact{Filename: strings.Repeat("a", 128), SHA256: strings.Repeat("a", 64),
			Content: strings.Repeat("a", maxOutputBytes), Mappings: make([]Mapping, maxTokens)},
	}
	for i := range r.Diagnostics {
		r.Diagnostics[i] = Diagnostic{Code: strings.Repeat("a", 128), Message: strings.Repeat("a", 1024)}
	}
	if err := validateReceiptBounds(r); err != nil {
		t.Fatal(err)
	}
	// Bounds are bytes, not runes, and malformed strings must not be silently
	// replaced during JSON encoding and accepted under a different byte identity.
	r.Artifact.Content = strings.Repeat("é", maxOutputBytes/2+1)
	if err := validateReceiptBounds(r); err == nil {
		t.Fatal("accepted oversized multibyte content")
	}
	r.Artifact.Content = "\xff"
	if err := validateShape(r); err == nil || !strings.HasPrefix(err.Error(), "designconvert: receipt encoding: artifact.content") {
		t.Fatalf("invalid UTF-8 reached JSON/CUE validation: %v", err)
	}
}

func TestReceiptAcceptsEmitterAtSourceByteLimit(t *testing.T) {
	body := []byte("<p>" + strings.Repeat("x", MaxSourceBytes-len("<p></p>")) + "</p>")
	r, err := Translate(receiptSource(body, "html/maximum.html"), body)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "translated" || r.Artifact == nil {
		t.Fatal("receipt limits rejected a supported source at the inclusive input bound")
	}
	if err := VerifyReplay(r, body); err != nil {
		t.Fatal(err)
	}
}

func TestTranslationDependencyPinsMatchModule(t *testing.T) {
	module, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	for _, pin := range []string{HTMLParserVersion, RequiredTempl} {
		if !strings.Contains(string(module), strings.Replace(pin, "@", " ", 1)) {
			t.Fatalf("translation dependency receipt needs review for module drift: %s", pin)
		}
	}
}
