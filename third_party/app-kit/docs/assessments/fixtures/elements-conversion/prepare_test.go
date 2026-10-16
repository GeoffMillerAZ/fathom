package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

func syntheticTranslation(t *testing.T) ([]byte, designconvert.ElementsTranslation) {
	t.Helper()
	body := []byte(`<el-disclosure id="faq" class="one  two"><button command="--toggle" commandfor="faq">Question</button><div>Answer &amp; more</div></el-disclosure><!--kept-->`)
	result, err := designconvert.TranslateElements(designconvert.Source{
		ArchiveID: "marketing@" + strings.Repeat("a", 64), Path: "html/synthetic-elements.html",
		SHA256: digest(body), Size: len(body),
	}, body)
	if err != nil || result.Status != "translated" || result.Artifact == nil {
		t.Fatal("synthetic Elements translation failed", err)
	}
	return body, result
}

func TestSyntheticCompileBeforeAndAfterFormatPreservesExactDOM(t *testing.T) {
	body, result := syntheticTranslation(t)
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "compile")
	before, after, formattedTempl, err := compilePair(root, directory, []byte(result.Artifact.Content), body)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) == 0 || len(after) == 0 || len(formattedTempl) == 0 ||
		!fragmentEqual(body, before) || !fragmentEqual(body, after) || !fragmentEqual(before, after) {
		t.Fatal("compiled or formatted render changed namespace-aware DOM/text/attributes/comments")
	}
	for _, changed := range [][]byte{
		bytes.ReplaceAll(after, []byte("one  two"), []byte("one two")),
		bytes.ReplaceAll(after, []byte("Answer &amp; more"), []byte("Answer and more")),
		bytes.ReplaceAll(after, []byte("<!--kept-->"), nil),
	} {
		if fragmentEqual(body, changed) {
			t.Fatal("DOM oracle normalized a class, text, or comment change")
		}
	}
}

func TestSyntheticReceiptReplayAndTamperCanaries(t *testing.T) {
	body, result := syntheticTranslation(t)
	if err := designconvert.VerifyElementsReplay(result, body); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil || !bytes.Contains(encoded, []byte(`"version":"1.0.22"`)) ||
		!bytes.Contains(encoded, []byte(designconvert.ElementsRuntimeESMSHA256)) ||
		!bytes.Contains(encoded, []byte(designconvert.ElementsRuntimeTGZSHA256)) ||
		bytes.Contains(encoded, []byte(`"verification":"verified"`)) {
		t.Fatal("receipt lost runtime identity or invented verification", err)
	}
	for name, mutate := range map[string]func(*designconvert.ElementsTranslation){
		"runtime": func(got *designconvert.ElementsTranslation) { got.Runtime.Version = "1.0.23" },
		"rule":    func(got *designconvert.ElementsTranslation) { got.Transformer.SHA256 = strings.Repeat("0", 64) },
		"source":  func(got *designconvert.ElementsTranslation) { got.Source.SHA256 = strings.Repeat("0", 64) },
		"output": func(got *designconvert.ElementsTranslation) {
			got.Artifact.Content = strings.Replace(got.Artifact.Content, "one  two", "one two", 1)
		},
		"proof": func(got *designconvert.ElementsTranslation) { got.Verification = "verified" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := result
			artifact := *result.Artifact
			artifact.Mappings = append([]designconvert.Mapping{}, result.Artifact.Mappings...)
			changed.Artifact = &artifact
			mutate(&changed)
			if designconvert.VerifyElementsReplay(changed, body) == nil {
				t.Fatal("tampered receipt replayed")
			}
		})
	}
}

func TestExactWrapperReplacementRefusesAmbiguity(t *testing.T) {
	original := []byte("<el-menu><button>One</button></el-menu>")
	document := append(append([]byte("<main>"), original...), []byte("</main>")...)
	converted := []byte("<el-menu><button>Two</button></el-menu>")
	got, err := replaceExactFragment(document, original, converted)
	if err != nil || !bytes.Equal(got, []byte("<main>"+string(converted)+"</main>")) {
		t.Fatal("exact wrapper replacement failed", err)
	}
	for _, bad := range [][]byte{[]byte("<main></main>"), append(document, original...)} {
		if _, err := replaceExactFragment(bad, original, converted); err == nil {
			t.Fatal("missing or duplicate original fragment was accepted")
		}
	}
}

func TestRandomPrivatePublicationIsExclusiveAndDetectsReplacement(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	_, root, info, err := openPrivateParent(parent)
	if err != nil {
		t.Fatal(err)
	}
	root.Close()
	files := map[string][]byte{"artifact.txt": []byte("synthetic"), "manifest.json": []byte("{}\n")}
	created, err := publishRandomPrivate(parent, info, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	dirInfo, err := os.Stat(created)
	if err != nil || dirInfo.Mode() != os.ModeDir|0700 {
		t.Fatal("published directory is not private")
	}
	for name := range files {
		fileInfo, err := os.Stat(filepath.Join(created, name))
		if err != nil || fileInfo.Mode() != 0600 {
			t.Fatal("published file is not exclusive/private", name)
		}
	}
	if err := writeExclusive(filepath.Join(created, "artifact.txt"), []byte("clobber")); err == nil {
		t.Fatal("exclusive write clobbered an existing artifact")
	}

	sentinel := filepath.Join(parent, "sentinel")
	if err := os.Mkdir(sentinel, 0700); err != nil {
		t.Fatal(err)
	}
	_, err = publishRandomPrivate(parent, info, files, func(parentRoot *os.Root, leaf string) error {
		if err := parentRoot.Rename(leaf, "displaced"); err != nil {
			return err
		}
		return parentRoot.Symlink("sentinel", leaf)
	})
	if err == nil || !strings.Contains(err.Error(), "destination-changed") {
		t.Fatal("destination replacement was not detected", err)
	}
	entries, readErr := os.ReadDir(sentinel)
	if readErr != nil || len(entries) != 0 {
		t.Fatal("publication followed a replacement into the sentinel")
	}
	if _, statErr := os.Stat(filepath.Join(parent, "displaced", "artifact.txt")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("publication wrote before retaining destination identity")
	}
}
