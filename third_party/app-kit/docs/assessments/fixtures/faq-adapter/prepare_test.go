package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

func TestStandaloneDependenciesNormalizeOnlyOwnModule(t *testing.T) {
	const external = "fmt /sdk/src/fmt\ngithub.com/a-h/templ /cache/templ\n"
	const root = "/private/app-kit"
	raw, err := standaloneDependencies(external+"example.invalid/faq-proof /tmp/raw\n", root, "/tmp/raw")
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := standaloneDependencies(external+"example.invalid/faq-proof /tmp/formatted\n", root, "/tmp/formatted")
	if err != nil || raw != formatted {
		t.Fatal("own build path prevented deterministic dependency replay", err)
	}
	for _, deps := range []string{
		external,
		external + "example.invalid/faq-proof /tmp/sibling\n",
		external + "github.com/geoffmilleraz/app-kit /cache/kit\nexample.invalid/faq-proof /tmp/raw\n",
		external + "example.invalid/other /private/app-kit/other\nexample.invalid/faq-proof /tmp/raw\n",
	} {
		if _, err := standaloneDependencies(deps, root, "/tmp/raw"); err == nil {
			t.Fatal("missing/misbound module or kit dependency accepted")
		}
	}
	changed, err := standaloneDependencies(strings.Replace(external, "/cache/templ", "/cache/changed", 1)+"example.invalid/faq-proof /tmp/raw\n", root, "/tmp/raw")
	if err != nil || changed == raw {
		t.Fatal("external dependency path drift was normalized away", err)
	}
}

func syntheticPrepared() (preparedManifest, map[string][]byte, sourceCase) {
	source := sourceCase{ID: "P03-faq", Source: sourceIdentity{ArchiveID: faqArchive, Path: faqMember, SHA256: faqSourceSHA, Size: 10}}
	files := map[string][]byte{
		"source.html": []byte("<p>source</p>"), "reference.html": []byte("<p>reference</p>"),
		"reference-formatted.html": []byte("<p>reference</p>\n"), "native.html": []byte("<details>native</details>"),
		"native-formatted.html": []byte("<details>native</details>\n"),
	}
	documents := map[string]map[string]string{}
	for _, lane := range []string{"source", "reference", "reference_formatted", "native", "native_formatted"} {
		documents[lane] = map[string]string{}
		for _, theme := range []string{"light", "dark"} {
			name := lane + "-" + theme + ".html"
			documents[lane][theme] = name
			wrapper := []byte("<link href=\"reference.css\"><script src=\"elements.js\"></script><main>")
			switch lane {
			case "source":
				files[name] = append(append(wrapper, files["source.html"]...), []byte("</main>")...)
			case "reference":
				files[name] = append(append(wrapper, files["reference.html"]...), []byte("</main>")...)
			case "reference_formatted":
				files[name] = append(append(wrapper, files["reference-formatted.html"]...), []byte("</main>")...)
			case "native":
				files[name] = nativeDocument(theme, files["native.html"])
			case "native_formatted":
				files[name] = nativeDocument(theme, files["native-formatted.html"])
			}
		}
	}
	m := preparedManifest{SchemaVersion: 1, Kind: "p03-faq-typed-preparation", SourceManifestSHA256: sourceManifestSHA,
		Source: source.Source, Artifact: artifactIdentity{Adapter: "p03-faq-v1", SHA256: digest([]byte("artifact")), SourceSHA256: faqSourceSHA, Verification: "not-verified"},
		Documents: documents, Files: map[string]fileIdentity{}, Verification: "not-verified"}
	m.Runtime.SourceReference = designconvert.ElementsRuntime{Package: designconvert.ElementsRuntimePackage, Version: designconvert.ElementsRuntimeVersion,
		ESMSHA256: designconvert.ElementsRuntimeESMSHA256, TGZSHA256: designconvert.ElementsRuntimeTGZSHA256, Delivery: designconvert.ElementsRuntimeDelivery}
	for name, body := range files {
		m.Files[name] = identity(body)
	}
	return m, files, source
}

func TestSyntheticManifestTopologyRejectsAliasesAndInventedRuntime(t *testing.T) {
	m, files, source := syntheticPrepared()
	if err := validatePreparedTopology(m, files, source); err != nil {
		t.Fatal(err)
	}
	changed := m
	changed.Documents = map[string]map[string]string{}
	for lane, pair := range m.Documents {
		changed.Documents[lane] = map[string]string{"light": pair["light"], "dark": pair["dark"]}
	}
	changed.Documents["reference"]["light"] = changed.Documents["source"]["light"]
	if validatePreparedTopology(changed, files, source) == nil {
		t.Fatal("aliased lane accepted")
	}
	changed = m
	runtime := changed.Runtime.SourceReference
	changed.Runtime.NativeDetails = &runtime
	if validatePreparedTopology(changed, files, source) == nil {
		t.Fatal("native runtime invented")
	}
	changed = m
	changed.Artifact.Verification = "verified"
	if validatePreparedTopology(changed, files, source) == nil {
		t.Fatal("verification invented")
	}
	changed = m
	changed.Files = map[string]fileIdentity{}
	if validatePreparedTopology(changed, files, source) == nil {
		t.Fatal("file closure loss accepted")
	}
}

func TestDOMOracleAndWrapperRejectSemanticDriftAndAmbiguity(t *testing.T) {
	original := []byte(`<section class="one  two"><h2>Question &amp; answer</h2><!--kept--></section>`)
	if !fragmentEqual(original, bytes.ReplaceAll(original, []byte("&amp;"), []byte("&#38;"))) {
		t.Fatal("equivalent entity changed DOM")
	}
	for _, changed := range [][]byte{
		bytes.ReplaceAll(original, []byte("one  two"), []byte("one two")),
		bytes.ReplaceAll(original, []byte("Question"), []byte("Different")),
		bytes.ReplaceAll(original, []byte("<!--kept-->"), nil),
	} {
		if fragmentEqual(original, changed) {
			t.Fatal("DOM oracle hid semantic drift")
		}
	}
	doc := append(append([]byte("<main>"), original...), []byte("</main>")...)
	if got, err := replaceExact(doc, original, []byte("<section>replacement</section>")); err != nil || !bytes.Contains(got, []byte("replacement")) {
		t.Fatal("exact replacement failed")
	}
	if _, err := replaceExact(append(doc, original...), original, original); err == nil {
		t.Fatal("duplicate source accepted")
	}
}

func TestPrivatePublicationUsesExclusiveModesAndManifestClosure(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	_, root, info, err := openPrivateParent(parent)
	if err != nil {
		t.Fatal(err)
	}
	root.Close()
	files := map[string][]byte{"artifact.json": []byte("{}\n"), "manifest.json": []byte("{}\n")}
	dir, err := publishRandomPrivate(parent, info, files)
	if err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil || dirInfo.Mode() != os.ModeDir|0700 {
		t.Fatal("private directory mode")
	}
	for name := range files {
		stat, err := os.Stat(filepath.Join(dir, name))
		if err != nil || stat.Mode() != 0600 {
			t.Fatal("private file mode")
		}
		if err := writeExclusive(filepath.Join(dir, name), []byte("clobber")); !errors.Is(err, os.ErrExist) {
			t.Fatal("exclusive write did not refuse")
		}
	}
	bad := map[string][]byte{"not-manifest.txt": []byte("x")}
	if _, err := publishRandomPrivate(parent, info, bad); err == nil {
		t.Fatal("manifest-less publication accepted")
	}
}
