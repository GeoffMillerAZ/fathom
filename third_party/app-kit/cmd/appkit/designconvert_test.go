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

	"github.com/geoffmilleraz/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designattempts"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

func TestAssetsTranslateCLIOriginalAndReplay(t *testing.T) {
	var zipBytes bytes.Buffer
	z := zip.NewWriter(&zipBytes)
	members := []struct{ path, body string }{
		{"html/page.html", `<p class="dark:text-white  text-black">One &amp; two</p>`},
		{"html/menu.html", `<el-menu><button>Action</button></el-menu>`},
		{"react/page.tsx", `export default () => <p>Client-owned</p>`},
	}
	for _, member := range members {
		w, err := z.Create(member.path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(member.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	src, root := t.TempDir(), filepath.Join(t.TempDir(), "private")
	originalPath := filepath.Join(src, "marketing-v4.zip")
	if err := os.WriteFile(originalPath, zipBytes.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := designassets.Import(context.Background(), src, root)
	if err != nil {
		t.Fatal(err)
	}
	id := c.List()[0].ID
	for i, member := range members {
		var result designconvert.Translation
		args := []string{"assets-translate", "-root", root, "-id", id, "-path", member.path}
		first, firstErr := captureAssetsCLI(t, args...)
		second, secondErr := captureAssetsCLI(t, args...)
		wantCode := 1
		if i == 0 {
			wantCode = 0
		}
		if command.Code(firstErr) != wantCode || command.Code(secondErr) != wantCode {
			t.Fatalf("wrong translation exit codes: %v, %v", firstErr, secondErr)
		}
		if err := json.Unmarshal(first, &result); err != nil {
			t.Fatal("translation did not emit exactly one JSON value")
		}
		if !bytes.Equal(first, second) || result.Source.Path != member.path || result.Source.ArchiveID != id {
			t.Fatal("CLI translation lost source identity or determinism")
		}
		if err := designconvert.VerifyReplay(result, []byte(member.body)); err != nil {
			t.Fatal(err)
		}
		if (result.Status == "translated") != (i == 0) || result.Verification != "not-verified" {
			t.Fatalf("incorrect syntax-only classification: %+v", result)
		}
	}
	for _, args := range [][]string{
		{}, {"-id", id}, {"-path", "html/page.html"},
		{"-id", id, "-path", "html/page.html", "-target", "react"},
		{"-id", id, "-path", "../secret.html"},
		{"-id", "marketing@" + strings.Repeat("0", 64), "-path", "html/page.html"},
		{"-id", id, "-path", "html/missing.html"},
		{"-id", id, "-path", "html/page.html", "extra"},
	} {
		body, err := captureAssetsCLI(t, append([]string{"assets-translate", "-root", root}, args...)...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatalf("bad arguments emitted success/data: %v %s %v", args, body, err)
		}
	}
	for _, p := range []string{originalPath, filepath.Join(root, "archives", c.List()[0].SHA256+".zip")} {
		body, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(body, zipBytes.Bytes()) {
			t.Fatal("translation modified an original")
		}
	}
	files, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Name() != "archives" && file.Name() != "catalog.json" {
			t.Fatalf("read-only translation wrote %s", file.Name())
		}
	}
	// The command emits one machine-readable result, never a target approval.
	encoded, err := captureAssetsCLI(t, "assets-translate", "-root", root, "-id", id, "-path", "react/page.tsx")
	if command.Code(err) != 1 || !json.Valid(encoded) {
		t.Fatal("unsupported source lost its structured diagnostic result")
	}
	if bytes.Contains(encoded, []byte(`"artifact"`)) {
		t.Fatal("unsupported source emitted a partial artifact")
	}
}

type attemptCLIFixture struct {
	root, archive string
	members       map[string]string
}

func newAttemptCLIFixture(t *testing.T) attemptCLIFixture {
	t.Helper()
	fixture := attemptCLIFixture{
		root: filepath.Join(t.TempDir(), "private"),
		members: map[string]string{
			"html/page.html":   "<p>CLI_ATTEMPT_SOURCE_ONLY &amp; private</p>",
			"html/second.html": "<p>Second deterministic specimen</p>",
			"html/menu.html":   "<el-menu><button>Unsupported client control</button></el-menu>",
		},
	}
	var body bytes.Buffer
	z := zip.NewWriter(&body)
	for _, path := range []string{"html/page.html", "html/second.html", "html/menu.html"} {
		w, err := z.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(fixture.members[path])); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "marketing-v4.zip"), body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := designassets.Import(context.Background(), source, fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	fixture.archive = c.List()[0].ID
	return fixture
}

func saveAttemptCLI(t *testing.T, fixture attemptCLIFixture, member string, wantCode int) savedTranslationOutput {
	t.Helper()
	args := []string{"assets-translate", "-root", fixture.root, "-id", fixture.archive, "-path", member}
	plain, err := captureAssetsCLI(t, args...)
	if command.Code(err) != wantCode {
		t.Fatalf("default translation exit=%d; want %d", command.Code(err), wantCode)
	}
	disabled, err := captureAssetsCLI(t, append(args, "-save=false")...)
	if command.Code(err) != wantCode || !bytes.Equal(plain, disabled) {
		t.Fatal("explicitly disabled saving changed the original output contract")
	}
	body, err := captureAssetsCLI(t, append(args, "-save")...)
	if command.Code(err) != wantCode {
		t.Fatalf("saving changed syntax classification: %v", err)
	}
	var saved savedTranslationOutput
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &saved); err != nil {
		t.Fatal("save did not emit exactly one JSON result")
	}
	if err := json.Unmarshal(body, &fields); err != nil || len(fields) != 2 || fields["id"] == nil || fields["translation"] == nil {
		t.Fatal("save result must have exactly id and translation fields")
	}
	translation, err := json.Marshal(saved.Translation)
	if err != nil || !bytes.Equal(plain, append(translation, '\n')) || saved.Translation.Verification != "not-verified" {
		t.Fatal("save wrapped a modified translation or fabricated proof")
	}
	if !strings.HasPrefix(saved.ID, "syntax@") || len(saved.ID) != len("syntax@")+64 {
		t.Fatal("saved attempt did not receive an immutable syntax ID")
	}
	if err := designconvert.VerifyReplay(saved.Translation, []byte(fixture.members[member])); err != nil {
		t.Fatal(err)
	}
	repeated, err := captureAssetsCLI(t, append(args, "-save")...)
	if command.Code(err) != wantCode || !bytes.Equal(body, repeated) {
		t.Fatal("repeating the same saved attempt changed its identity or JSON")
	}
	return saved
}

func TestAssetsConversionSavedAttemptsRoundTrip(t *testing.T) {
	fixture := newAttemptCLIFixture(t)
	catalogBefore, err := os.ReadFile(filepath.Join(fixture.root, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, archiveHash, _ := strings.Cut(fixture.archive, "@")
	archivePath := filepath.Join(fixture.root, "archives", archiveHash+".zip")
	archiveBefore, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	saved := map[string]savedTranslationOutput{}
	for _, member := range []string{"html/page.html", "html/second.html", "html/menu.html"} {
		code := 0
		if member == "html/menu.html" {
			code = 1
		}
		saved[member] = saveAttemptCLI(t, fixture, member, code)
	}
	for member, saved := range saved {
		var actual map[string]json.RawMessage
		body := assetsCLIJSON(t, &actual, "assets-conversion", "-root", fixture.root, "-id", saved.ID)
		expected, err := designattempts.Read(fixture.root, saved.ID)
		if err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(expected)
		if err != nil || !bytes.Equal(body, append(want, '\n')) {
			t.Fatal("exact-get JSON differs from the shared attempt contract")
		}
		for _, field := range []string{"entry", "translation", "replay", "revision"} {
			if actual[field] == nil {
				t.Fatalf("exact-get omitted %s", field)
			}
		}
		var translation designconvert.Translation
		if err := json.Unmarshal(actual["translation"], &translation); err != nil || translation.Source.Path != member || translation.Verification != "not-verified" {
			t.Fatal("exact-get lost the original source or syntax-only qualification")
		}
	}
	c, err := designattempts.Load(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	var all json.RawMessage
	listed := assetsCLIJSON(t, &all, "assets-conversions", "-root", fixture.root)
	page, err := c.Search(designattempts.Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.Marshal(page)
	if err != nil || !bytes.Equal(listed, append(expected, '\n')) {
		t.Fatal("listing differs from the shared bounded catalog contract")
	}
	if bytes.Contains(listed, []byte("CLI_ATTEMPT_SOURCE_ONLY")) || bytes.Contains(listed, []byte("\"artifact\"")) {
		t.Fatal("list exposed derivative/source bodies rather than metadata")
	}
	type listedPage struct {
		Revision   string `json:"revision"`
		Total      int    `json:"total"`
		NextCursor string `json:"next_cursor"`
		Entries    []struct {
			ID string `json:"id"`
		} `json:"entries"`
	}
	seen, cursor, revision := map[string]bool{}, "", ""
	for pages := 0; ; pages++ {
		if pages >= 3 {
			t.Fatal("pagination did not terminate for three attempts")
		}
		var page listedPage
		assetsCLIJSON(t, &page, "assets-conversions", "-root", fixture.root, "-limit", "1", "-cursor", cursor)
		if page.Total != 3 || len(page.Entries) != 1 || page.Revision == "" || seen[page.Entries[0].ID] {
			t.Fatal("paginated listing lost count, identity or boundedness")
		}
		if revision != "" && revision != page.Revision {
			t.Fatal("read-only listing changed catalog revision")
		}
		revision, cursor = page.Revision, page.NextCursor
		seen[page.Entries[0].ID] = true
		if cursor == "" {
			break
		}
	}
	if len(seen) != 3 {
		t.Fatal("pagination silently omitted a saved attempt")
	}
	for _, test := range []struct {
		args  []string
		want  int
		exact string
	}{
		{[]string{"-id", fixture.archive}, 3, ""},
		{[]string{"-id", fixture.archive, "-path", "html/page.html"}, 1, saved["html/page.html"].ID},
		{[]string{"-status", "translated"}, 2, ""},
		{[]string{"-status", "unsupported"}, 1, saved["html/menu.html"].ID},
		{[]string{"-id", fixture.archive, "-path", "html/absent.html"}, 0, ""},
	} {
		var page listedPage
		assetsCLIJSON(t, &page, append([]string{"assets-conversions", "-root", fixture.root}, test.args...)...)
		if page.Total != test.want || len(page.Entries) != test.want || test.exact != "" && page.Entries[0].ID != test.exact {
			t.Fatalf("listing filter %v returned the wrong attempts", test.args)
		}
	}
	catalogAfter, err := os.ReadFile(filepath.Join(fixture.root, "catalog.json"))
	if err != nil || !bytes.Equal(catalogBefore, catalogAfter) {
		t.Fatal("saving or discovery changed the original catalog")
	}
	archiveAfter, err := os.ReadFile(archivePath)
	if err != nil || !bytes.Equal(archiveBefore, archiveAfter) {
		t.Fatal("saving or discovery changed the original archive")
	}
}

func TestAssetsConversionFailuresHaveNoStdout(t *testing.T) {
	fixture := newAttemptCLIFixture(t)
	saved := saveAttemptCLI(t, fixture, "html/page.html", 0)
	for _, args := range [][]string{
		{"assets-conversions", "extra"},
		{"assets-conversions", "-limit", "not-an-integer"},
		{"assets-conversions", "-limit", "-1"},
		{"assets-conversions", "-limit", "51"},
		{"assets-conversions", "-status", "verified"},
		{"assets-conversions", "-id", "mutable-name"},
		{"assets-conversions", "-path", "../secret"},
		{"assets-conversions", "-cursor", "invalid-cursor"},
		{"assets-conversion"},
		{"assets-conversion", "-id", "mutable-name"},
		{"assets-conversion", "-id", "syntax@" + strings.Repeat("0", 64)},
		{"assets-conversion", "-id", saved.ID, "extra"},
		{"assets-translate", "-id", fixture.archive, "-path", "html/page.html", "-save=invalid"},
	} {
		args = append([]string{args[0], "-root", fixture.root}, args[1:]...)
		body, err := captureAssetsCLI(t, args...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatalf("%v did not return empty-stdout operational failure: %v", args, err)
		}
	}
	for _, commandName := range []string{"assets-conversions", "assets-conversion"} {
		args := []string{commandName, "-root", filepath.Join(t.TempDir(), "missing")}
		if commandName == "assets-conversion" {
			args = append(args, "-id", saved.ID)
		}
		body, err := captureAssetsCLI(t, args...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatalf("missing explicit root accepted by %s: %v", commandName, err)
		}
	}
}

func TestAssetsTranslateSaveFailureDoesNotChangeDefault(t *testing.T) {
	fixture := newAttemptCLIFixture(t)
	obstruction := filepath.Join(fixture.root, "translations")
	sentinel := []byte("Synthetic obstruction: must not be replaced.\n")
	if err := os.WriteFile(obstruction, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"html/page.html", "html/menu.html"} {
		args := []string{"assets-translate", "-root", fixture.root, "-id", fixture.archive, "-path", member}
		wantCode := 0
		if member == "html/menu.html" {
			wantCode = 1
		}
		plain, err := captureAssetsCLI(t, args...)
		if command.Code(err) != wantCode || !json.Valid(plain) {
			t.Fatal("unsaved translation unnecessarily depends on attempt storage")
		}
		saved, err := captureAssetsCLI(t, append(args, "-save")...)
		if command.Code(err) != 2 || len(saved) != 0 {
			t.Fatal("failed save emitted a result or retained syntax exit 0/1")
		}
		after, err := captureAssetsCLI(t, args...)
		if command.Code(err) != wantCode || !bytes.Equal(plain, after) {
			t.Fatal("failed opt-in save changed default translation")
		}
	}
	current, err := os.ReadFile(obstruction)
	if err != nil || !bytes.Equal(current, sentinel) {
		t.Fatal("save overwrote the obstructing local file")
	}
}

func TestAssetsConversionsReadOnlyBeforeFirstSave(t *testing.T) {
	fixture := newAttemptCLIFixture(t)
	var empty struct {
		Revision string            `json:"revision"`
		Total    int               `json:"total"`
		Entries  []json.RawMessage `json:"entries"`
	}
	assetsCLIJSON(t, &empty, "assets-conversions", "-root", fixture.root)
	if empty.Revision == "" || empty.Total != 0 || len(empty.Entries) != 0 {
		t.Fatal("unsaved originals must have an explicit empty attempt listing")
	}
	body, err := captureAssetsCLI(t, "assets-conversion", "-root", fixture.root, "-id", "syntax@"+strings.Repeat("0", 64))
	if command.Code(err) != 2 || len(body) != 0 {
		t.Fatal("unknown attempt before the first save must fail without stdout")
	}
	if _, err := os.Lstat(filepath.Join(fixture.root, "translations")); !os.IsNotExist(err) {
		t.Fatalf("read-only discovery created attempt storage: %v", err)
	}
}
