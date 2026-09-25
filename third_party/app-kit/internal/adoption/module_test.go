package adoption

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestObserveModuleDeclarations(t *testing.T) {
	const header = "module example.test/consumer\n\ngo 1.26.0\n"
	for _, tc := range []struct {
		name, source, version string
		replacement           *ModuleReplacement
	}{
		{name: "absent", source: "require example.test/other v1.2.3\n"},
		{name: "exact", source: "require github.com/geoffmilleraz/app-kit v0.2.3\n", version: "v0.2.3"},
		{name: "indirect", source: "require (\n github.com/geoffmilleraz/app-kit v0.2.3 // indirect\n)\n", version: "v0.2.3"},
		{name: "prefix is not identity", source: "require github.com/geoffmilleraz/app-kit-other v0.2.3\n"},
		{name: "local path preserved", source: "require github.com/geoffmilleraz/app-kit v0.0.0\nreplace github.com/geoffmilleraz/app-kit => ./../absent-kit/\n", version: "v0.0.0", replacement: &ModuleReplacement{Path: "./../absent-kit/"}},
		{name: "quoted local path", source: "require github.com/geoffmilleraz/app-kit v0.0.0\nreplace github.com/geoffmilleraz/app-kit => \"../absent kit/source\"\n", version: "v0.0.0", replacement: &ModuleReplacement{Path: "../absent kit/source"}},
		{name: "absolute local path", source: "require github.com/geoffmilleraz/app-kit v0.0.0\nreplace github.com/geoffmilleraz/app-kit => /not-a-real-adoption-fixture/kit\n", version: "v0.0.0", replacement: &ModuleReplacement{Path: "/not-a-real-adoption-fixture/kit"}},
		{name: "remote replacement", source: "require github.com/geoffmilleraz/app-kit v0.2.3\nreplace github.com/geoffmilleraz/app-kit => example.invalid/fork v0.9.1\n", version: "v0.2.3", replacement: &ModuleReplacement{Path: "example.invalid/fork", Version: "v0.9.1"}},
		{name: "nonmatching version", source: "require github.com/geoffmilleraz/app-kit v0.2.3\nreplace github.com/geoffmilleraz/app-kit v0.2.2 => ../unused\n", version: "v0.2.3"},
		{name: "replace without require selects nothing", source: "replace github.com/geoffmilleraz/app-kit => ../unused\n"},
		{name: "unrelated replacement", source: "require github.com/geoffmilleraz/app-kit v0.2.3\nreplace example.test/other => ../other\n", version: "v0.2.3"},
		{name: "unrelated duplicates not interpreted", source: "require example.test/other v1.0.0\nrequire example.test/other v1.0.1\nreplace example.test/other => ../one\nreplace example.test/other => ../two\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(header + tc.source)
			before := bytes.Clone(body)
			got, err := ObserveModule(body)
			want := ModuleEvidence{DeclaredVersion: tc.version, Replacement: tc.replacement, Resolution: "declarations-only"}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v, %v; want %+v", got, err, want)
			}
			if !bytes.Equal(body, before) {
				t.Fatal("observation changed source bytes")
			}
		})
	}
}

func TestObserveModuleVersionSpecificReplacementPrecedence(t *testing.T) {
	const header = "module example.test/consumer\nrequire github.com/geoffmilleraz/app-kit v0.2.3\n"
	const wildcard = "replace github.com/geoffmilleraz/app-kit => ../wildcard\n"
	const specific = "replace github.com/geoffmilleraz/app-kit v0.2.3 => example.invalid/specific v0.7.0\n"
	const irrelevant = "replace github.com/geoffmilleraz/app-kit v0.2.2 => ../unselected\n"
	for _, suffix := range []string{wildcard + specific + irrelevant, specific + irrelevant + wildcard} {
		got, err := ObserveModule([]byte(header + suffix))
		if err != nil || got.Replacement == nil || *got.Replacement != (ModuleReplacement{Path: "example.invalid/specific", Version: "v0.7.0"}) {
			t.Fatalf("version-specific replacement did not win independently of order: %+v, %v", got, err)
		}
	}
}

func TestObserveModuleRejectsMalformedAndAmbiguousInput(t *testing.T) {
	const header = "module example.test/consumer\n"
	for _, source := range []string{
		"", "require github.com/geoffmilleraz/app-kit v0.2.3\n",
		header + "require github.com/geoffmilleraz/app-kit\n",
		header + "require github.com/geoffmilleraz/app-kit v0.2.3\nrequire github.com/geoffmilleraz/app-kit v0.2.4\n",
		header + "require (\n github.com/geoffmilleraz/app-kit v0.2.3\n github.com/geoffmilleraz/app-kit v0.2.3\n)\n",
		header + "replace github.com/geoffmilleraz/app-kit => ../one\nreplace github.com/geoffmilleraz/app-kit => ../two\n",
		header + "replace github.com/geoffmilleraz/app-kit v0.2.3 => ../one\nreplace github.com/geoffmilleraz/app-kit v0.2.3 => ../one\n",
		header + "require github.com/geoffmilleraz/app-kit v0.2.4\nreplace github.com/geoffmilleraz/app-kit v0.2.3 => ../one\nreplace github.com/geoffmilleraz/app-kit v0.2.3 => ../two\n",
		header + "require (\n github.com/geoffmilleraz/app-kit v0.2.3\n",
		header + "replace github.com/geoffmilleraz/app-kit =>\n",
	} {
		got, err := ObserveModule([]byte(source))
		if err == nil || !reflect.DeepEqual(got, ModuleEvidence{}) {
			t.Fatalf("malformed or ambiguous declaration accepted: %+v, %v", got, err)
		}
	}
}

func TestObserveModuleBoundsAndSafeErrors(t *testing.T) {
	const header = "module example.test/consumer\n//"
	boundary := []byte(header + strings.Repeat("x", maxObservedModuleBytes-len(header)))
	if _, err := ObserveModule(boundary); err != nil {
		t.Fatalf("exact byte bound refused: %v", err)
	}
	for _, source := range [][]byte{
		append(bytes.Clone(boundary), 'x'),
		append([]byte("module example.test/consumer\n//"), 0xff),
		[]byte("module example.test/consumer\nsecret-source-excerpt /private/token\n"),
		[]byte("module example.test/consumer\nrequire secret-source-excerpt nope\n"),
		[]byte("module example.test/consumer\n\x00secret-source-excerpt\n"),
	} {
		if _, err := ObserveModule(source); err == nil || strings.Contains(err.Error(), "secret-source-excerpt") || strings.Contains(err.Error(), "/private/token") {
			t.Fatalf("unsafe input accepted or source leaked in error: %v", err)
		}
	}
}

func TestObserveModuleJSONAndDetachedResults(t *testing.T) {
	body := []byte("module example.test/consumer\n")
	got, err := ObserveModule(body)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(got)
	if err != nil || string(encoded) != `{"declared_version":"","resolution":"declarations-only"}` {
		t.Fatalf("unexpected absent-module JSON: %s, %v", encoded, err)
	}
	body = []byte("module example.test/consumer\nrequire github.com/geoffmilleraz/app-kit v0.2.3\nreplace github.com/geoffmilleraz/app-kit => ../kit\n")
	first, err := ObserveModule(body)
	if err != nil {
		t.Fatal(err)
	}
	first.Replacement.Path = "changed caller view"
	second, err := ObserveModule(body)
	if err != nil || second.Replacement == nil || second.Replacement.Path != "../kit" || second.Resolution != "declarations-only" {
		t.Fatalf("independent observations share mutable metadata: %+v, %v", second, err)
	}
	encoded, err = json.Marshal(second)
	if err != nil || string(encoded) != `{"declared_version":"v0.2.3","replacement":{"path":"../kit","version":""},"resolution":"declarations-only"}` {
		t.Fatalf("unexpected replacement JSON: %s, %v", encoded, err)
	}
}
