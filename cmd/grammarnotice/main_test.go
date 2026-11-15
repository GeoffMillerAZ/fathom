package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// These are conspicuously synthetic source texts, not fallback licences.
const mitFixture = "Copyright fixture authors\nMIT fixture licence body\n"
const apacheFixture = "Copyright other fixture authors\nApache-2.0 fixture licence body\n"
const noticeFixture = "Apache upstream NOTICE fixture\n"

func writeTar(t *testing.T, file string, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		body := files[name]
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pinnedFixture(t *testing.T) ([]byte, string) {
	t.Helper()
	cache := t.TempDir()
	lock := tsengine.GrammarLock{}
	for i, name := range []string{"alpha", "beta"} {
		files := map[string]string{"upstream/LICENSE": mitFixture}
		licence := "MIT"
		if i == 1 {
			files["upstream/LICENSE"] = apacheFixture
			files["upstream/NOTICE"] = noticeFixture
			licence = "Apache-2.0"
		}
		file := filepath.Join(cache, "source.tar.gz")
		raw := writeTar(t, file, files)
		sum := sha256.Sum256(raw)
		digest := hex.EncodeToString(sum[:])
		if err := os.Rename(file, filepath.Join(cache, digest+".tar.gz")); err != nil {
			t.Fatal(err)
		}
		lock.Grammars = append(lock.Grammars, tsengine.GrammarLockRow{Blob: name, Repo: "fixture/" + name, Commit: strings.Repeat(string(rune('a'+i)), 40), Licence: licence, TarballSHA256: digest})
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	return raw, cache
}

func TestReleaseArchiveRequiresGrammarNotice(t *testing.T) {
	lock, cache := pinnedFixture(t)
	notice, err := GenerateNotice(lock, cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha", "beta", "https://github.com/fixture/alpha", strings.Repeat("a", 40), "MIT", "Apache-2.0", mitFixture, apacheFixture, noticeFixture} {
		if !bytes.Contains(notice, []byte(want)) {
			t.Fatalf("notice missing %q", want)
		}
	}
	var parsed tsengine.GrammarLock
	if err := json.Unmarshal(lock, &parsed); err != nil {
		t.Fatal(err)
	}
	parsed.Grammars[0], parsed.Grammars[1] = parsed.Grammars[1], parsed.Grammars[0]
	reordered, _ := json.Marshal(parsed)
	got, err := GenerateNotice(reordered, cache)
	if err != nil || !bytes.Equal(got, notice) {
		t.Fatalf("reordered lock changed notice: %v", err)
	}
	cases := []struct {
		name, body, want string
		missing          bool
	}{
		{name: "complete", body: string(notice)},
		{name: "missing", missing: true, want: "missing notice"},
		{name: "grammar removed", body: strings.ReplaceAll(string(notice), "alpha", "removed"), want: "alpha"},
		{name: "apache text removed", body: strings.ReplaceAll(string(notice), apacheFixture, ""), want: "notice"},
		{name: "apache notice removed", body: strings.ReplaceAll(string(notice), noticeFixture, ""), want: "notice"},
		{name: "stale pin", body: strings.ReplaceAll(string(notice), strings.Repeat("a", 40), strings.Repeat("c", 40)), want: "notice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "release.tar.gz")
			files := map[string]string{"fathom": "fixture binary"}
			if !tc.missing {
				files["THIRD-PARTY-NOTICES.txt"] = tc.body
			}
			writeTar(t, archive, files)
			err := CheckReleaseArchiveNotice(archive, notice)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "release.tar.gz") {
				t.Fatalf("want archive and %q: %v", tc.want, err)
			}
		})
	}
}

func TestNoticeRefusesMissingOrChangedSources(t *testing.T) {
	lock, cache := pinnedFixture(t)
	if _, err := GenerateNotice(lock, t.TempDir()); err == nil {
		t.Fatal("accepted absent pinned sources")
	}
	var l tsengine.GrammarLock
	if err := json.Unmarshal(lock, &l); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(cache, l.Grammars[0].TarballSHA256+".tar.gz")
	if err := os.WriteFile(file, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateNotice(lock, cache); err == nil {
		t.Fatal("accepted changed source digest")
	}
}

func TestNoticeRefusesSourceWithoutLicence(t *testing.T) {
	cache := t.TempDir()
	raw := writeTar(t, filepath.Join(cache, "source.tar.gz"), map[string]string{"upstream/README": "no licence"})
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	if err := os.Rename(filepath.Join(cache, "source.tar.gz"), filepath.Join(cache, hash+".tar.gz")); err != nil {
		t.Fatal(err)
	}
	lock, _ := json.Marshal(tsengine.GrammarLock{Grammars: []tsengine.GrammarLockRow{{Blob: "missing", Repo: "fixture/missing", Commit: strings.Repeat("a", 40), Licence: "MIT", TarballSHA256: hash}}})
	if _, err := GenerateNotice(lock, cache); err == nil || !strings.Contains(err.Error(), "licence") {
		t.Fatalf("missing licence accepted: %v", err)
	}
}

func TestNoticeRejectsAmbiguousLockAndExcludesCore(t *testing.T) {
	raw, cache := pinnedFixture(t)
	var lock tsengine.GrammarLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatal(err)
	}
	// W1-3 covers third-party grammars only, not the core or project licence.
	lock.Core = tsengine.LockCore{Repo: "fixture/core", Licence: "MIT", Commit: strings.Repeat("f", 40)}
	encoded, _ := json.Marshal(lock)
	got, err := GenerateNotice(encoded, cache)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("https://github.com/fixture/core")) {
		t.Fatal("included core outside W1-3 scope")
	}
	lock.Grammars = append(lock.Grammars, lock.Grammars[0])
	encoded, _ = json.Marshal(lock)
	if _, err := GenerateNotice(encoded, cache); err == nil {
		t.Fatal("accepted duplicate grammar name")
	}
	if _, err := GenerateNotice([]byte(`{"grammars":[]}`), cache); err == nil {
		t.Fatal("accepted empty grammar inventory")
	}
}

func TestReleaseArchiveRejectsDuplicateOrLinkedNotice(t *testing.T) {
	lock, cache := pinnedFixture(t)
	notice, err := GenerateNotice(lock, cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "duplicate", true: "symlink"}[linked], func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "bad.tar.gz")
			var b bytes.Buffer
			gz := gzip.NewWriter(&b)
			tw := tar.NewWriter(gz)
			count := 2
			if linked {
				count = 1
			}
			for i := 0; i < count; i++ {
				h := &tar.Header{Name: "THIRD-PARTY-NOTICES.txt", Mode: 0600, Size: int64(len(notice))}
				if linked {
					h.Typeflag = tar.TypeSymlink
					h.Linkname = "elsewhere"
					h.Size = 0
				}
				if err := tw.WriteHeader(h); err != nil {
					t.Fatal(err)
				}
				if !linked {
					if _, err := tw.Write(notice); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, b.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if err := CheckReleaseArchiveNotice(file, notice); err == nil {
				t.Fatal("accepted ambiguous or linked notice")
			}
		})
	}
}
