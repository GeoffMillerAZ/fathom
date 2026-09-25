package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTableReadFileRefusesMissingSymlinkAndOversize(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, []byte("fixed"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := tableReadFile(regular, 5); err != nil || string(got) != "fixed" {
		t.Fatalf("regular input = %q, %v", got, err)
	}
	symlink := filepath.Join(dir, "symlink")
	if err := os.Symlink(regular, symlink); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		path  string
		limit int
	}{
		{name: "missing", path: filepath.Join(dir, "missing"), limit: 5},
		{name: "symlink", path: symlink, limit: 5},
		{name: "oversize", path: regular, limit: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tableReadFile(tc.path, tc.limit); err == nil || err.Error() != "P04 preparation refused: regular-input" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestTableSnapshotExecutablePinsBytesAndOwnsCopy(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "cached-tailwind")
	content := []byte("synthetic executable bytes")
	if err := os.WriteFile(source, content, 0755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "owned-tailwind")
	if err := tableSnapshotExecutable(source, destination, tableDigest(content), 1024); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != string(content) {
		t.Fatalf("snapshot = %q, %v", got, err)
	}
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	snapshotInfo, err := os.Lstat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(sourceInfo, snapshotInfo) || sourceInfo.Mode().Perm() != 0755 || snapshotInfo.Mode().Perm() != 0700 {
		t.Fatalf("source/snapshot modes or identity = %v/%v same=%v", sourceInfo.Mode(), snapshotInfo.Mode(), os.SameFile(sourceInfo, snapshotInfo))
	}
}

func TestTableSnapshotExecutableFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, string) string
		limit   int64
		sha     string
		stage   string
	}{
		{name: "missing", prepare: func(_ *testing.T, dir string) string { return filepath.Join(dir, "missing") }, limit: 10, sha: tableDigest([]byte("x")), stage: "missing-cached-tailwind"},
		{name: "symlink", prepare: func(t *testing.T, dir string) string {
			target := filepath.Join(dir, "target")
			if err := os.WriteFile(target, []byte("x"), 0755); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, "link")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			return link
		}, limit: 10, sha: tableDigest([]byte("x")), stage: "cached-tailwind-integrity"},
		{name: "oversize", prepare: func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "large")
			if err := os.WriteFile(path, []byte("xx"), 0755); err != nil {
				t.Fatal(err)
			}
			return path
		}, limit: 1, sha: tableDigest([]byte("xx")), stage: "cached-tailwind-integrity"},
		{name: "not-executable", prepare: func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "plain")
			if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
				t.Fatal(err)
			}
			return path
		}, limit: 10, sha: tableDigest([]byte("x")), stage: "cached-tailwind-integrity"},
		{name: "wrong-digest", prepare: func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "wrong")
			if err := os.WriteFile(path, []byte("x"), 0755); err != nil {
				t.Fatal(err)
			}
			return path
		}, limit: 10, sha: tableDigest([]byte("y")), stage: "cached-tailwind-integrity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := tc.prepare(t, dir)
			destination := filepath.Join(dir, "snapshot")
			err := tableSnapshotExecutable(source, destination, tc.sha, tc.limit)
			if err == nil || err.Error() != "P04 preparation refused: "+tc.stage {
				t.Fatalf("error = %v", err)
			}
			if tc.name == "wrong-digest" {
				if got, readErr := os.ReadFile(destination); readErr != nil || string(got) != "x" {
					t.Fatalf("inspectable failed snapshot = %q, %v", got, readErr)
				}
				if info, statErr := os.Lstat(destination); statErr != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("unverified snapshot is executable: %v, %v", info, statErr)
				}
			}
		})
	}
}

func TestTableCSSAndStyleBoundariesFailClosed(t *testing.T) {
	if strings.Count(tableCSSInput, `@source "./source.html";`) != 1 ||
		strings.Count(tableCSSInput, `@import "tailwindcss" source(none);`) != 1 ||
		!strings.Contains(tableCSSInput, `@custom-variant dark`) ||
		strings.Contains(strings.ToLower(tableCSSInput), "http") {
		t.Fatal("CSS compiler input escaped its one local source and theme boundary")
	}
	if err := tableSafeCSS([]byte(".safe{display:block}")); err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"", `a{background:URL(https://example.test/x)}`, `@IMPORT "x";`, `@source "x";`, `<style>.x{}</style>`} {
		if err := tableSafeCSS([]byte(unsafe)); err == nil {
			t.Fatalf("accepted unsafe CSS %q", unsafe)
		}
	}
	if err := tableSafeCSS(make([]byte, tableMaxDocument+1)); err == nil {
		t.Fatal("accepted oversized CSS")
	}
	body := "<main>synthetic fixed source</main>"
	for _, style := range []string{"<STYLE>.x{}</STYLE>", ".x{} </style>"} {
		if _, err := tableDocumentForSource(body, style, "light", tableDigest([]byte(body))); err == nil || err.Error() != "P04 preparation refused: style-boundary" {
			t.Fatalf("style %q error = %v", style, err)
		}
	}
}

func TestTableDocumentIsDeterministicAndRejectsSourceOrThemeDrift(t *testing.T) {
	body := "<main><p>Synthetic directory</p></main>"
	style := ".x{display:block}"
	sha := tableDigest([]byte(body))
	one, err := tableDocumentForSource(body, style, "dark", sha)
	if err != nil {
		t.Fatal(err)
	}
	two, err := tableDocumentForSource(body, style, "dark", sha)
	if err != nil || string(one) != string(two) {
		t.Fatalf("document replay differs: %v", err)
	}
	want := "<!doctype html>\n<html lang=\"en\" class=\"dark\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>P04 table: controlled source reference</title><style>" + style + "</style></head><body>" + body + "</body></html>\n"
	if string(one) != want || strings.Count(string(one), body) != 1 || strings.Count(string(one), style) != 1 {
		t.Fatalf("unexpected document composition: %q", one)
	}
	if _, err = tableDocumentForSource(body, style, "light", tableDigest([]byte("changed"))); err == nil || err.Error() != "P04 preparation refused: document-source" {
		t.Fatalf("source drift error = %v", err)
	}
	if _, err = tableDocumentForSource(body, style, "sepia", sha); err == nil || err.Error() != "P04 preparation refused: theme" {
		t.Fatalf("theme error = %v", err)
	}
}

func TestTableFailureMessageDisclosesOnlyOwnedPartialDirectory(t *testing.T) {
	err := tableRefuse("css-output")
	if got := tableFailureMessage("", err); got != err.Error() {
		t.Fatalf("early failure = %q", got)
	}
	if got := tableFailureMessage("/tmp/owned-partial", err); got != err.Error()+"; partial=/tmp/owned-partial" {
		t.Fatalf("partial failure = %q", got)
	}
}
