package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

func TestAssetsTranslateElementsSeparateReadOnlyFamily(t *testing.T) {
	members := []struct {
		path, body string
		code       int
	}{
		{"html/faq.html", `<div class="dark:text-white  text-black"><button command="--toggle" commandfor="faq-example">Toggle</button><el-disclosure id="faq-example" hidden>Safe &amp; literal {"x"}</el-disclosure></div>`, 0},
		{"html/unknown.html", `<el-unknown>Unsupported</el-unknown>`, 1},
		{"html/event.html", `<el-disclosure oncommand="alert(1)">Unsafe</el-disclosure>`, 1},
		{"html/dangling.html", `<button command="--toggle" commandfor="missing">Toggle</button><el-disclosure id="present">No binding</el-disclosure>`, 1},
		{"react/page.tsx", `export default () => <p>Client-owned</p>`, 1},
	}
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	for _, m := range members {
		w, err := z.Create(m.path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(m.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	src, root := t.TempDir(), filepath.Join(t.TempDir(), "private")
	input := filepath.Join(src, "marketing-v4.zip")
	if err := os.WriteFile(input, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := designassets.Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	entry := catalog.List()[0]
	for _, m := range members {
		args := []string{"assets-translate-elements", "-root", root, "-id", entry.ID, "-path", m.path}
		first, firstErr := captureAssetsCLI(t, args...)
		second, secondErr := captureAssetsCLI(t, args...)
		if command.Code(firstErr) != m.code || command.Code(secondErr) != m.code || !bytes.Equal(first, second) {
			t.Fatalf("%s: wrong exit or nondeterministic output: %v / %v", m.path, firstErr, secondErr)
		}
		var result designconvert.ElementsTranslation
		if err := json.Unmarshal(first, &result); err != nil {
			t.Fatal("not exactly one Elements JSON receipt", err)
		}
		if result.Kind != "elements-syntax" || result.Verification != "not-verified" || result.Source.ArchiveID != entry.ID || result.Source.Path != m.path {
			t.Fatal("family, original identity or evidence boundary lost")
		}
		if err := designconvert.VerifyElementsReplay(result, []byte(m.body)); err != nil {
			t.Fatal(err)
		}
		if (result.Artifact != nil) != (m.code == 0) {
			t.Fatal("wrong artifact presence")
		}
	}
	// Opt-in does not change the old refusal or teach its store a new envelope.
	legacy, legacyErr := captureAssetsCLI(t, "assets-translate", "-root", root, "-id", entry.ID, "-path", members[0].path)
	var old designconvert.Translation
	if command.Code(legacyErr) != 1 || json.Unmarshal(legacy, &old) != nil || old.Transformer.ID != "html-to-templ-v1" || old.Artifact != nil {
		t.Fatal("legacy translator changed")
	}
	for _, extra := range [][]string{
		{}, {"-id", entry.ID}, {"-path", members[0].path},
		{"-id", entry.ID, "-path", members[0].path, "-save"},
		{"-id", entry.ID, "-path", members[0].path, "-target", "react"},
		{"-id", entry.ID, "-path", members[0].path, "extra"},
		{"-id", entry.ID, "-path", "../outside.html"},
		{"-id", "marketing@" + strings.Repeat("0", 64), "-path", members[0].path},
	} {
		output, err := captureAssetsCLI(t, append([]string{"assets-translate-elements", "-root", root}, extra...)...)
		if command.Code(err) != 2 || len(output) != 0 {
			t.Fatal("invalid invocation emitted receipt or wrong exit", extra)
		}
	}
	for _, file := range []string{input, filepath.Join(root, "archives", entry.SHA256+".zip")} {
		body, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(body, archive.Bytes()) {
			t.Fatal("original changed")
		}
	}
	files, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Name() != "archives" && file.Name() != "catalog.json" {
			t.Fatal("read-only command wrote", file.Name())
		}
	}
}
