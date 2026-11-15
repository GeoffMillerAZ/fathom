package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
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

type archiveMember struct {
	name string
	kind byte
	link string
	body string
}

func writeMemberArchive(t *testing.T, format tar.Format, members []archiveMember) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "members.tar.gz")
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		h := &tar.Header{Name: m.name, Typeflag: m.kind, Linkname: m.link, Mode: 0600, Format: format}
		if m.kind == tar.TypeReg {
			h.Size = int64(len(m.body))
		}
		if m.kind == tar.TypeDir {
			h.Mode = 0700
		}
		if format == tar.FormatPAX {
			h.PAXRecords = map[string]string{"comment": "synthetic member-path fixture"}
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if m.kind == tar.TypeReg {
			if _, err := tw.Write([]byte(m.body)); err != nil {
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
	return file
}

func TestReleaseArchiveRejectsMemberAliases(t *testing.T) {
	lock, cache := pinnedFixture(t)
	notice, err := GenerateNotice(lock, cache)
	if err != nil {
		t.Fatal(err)
	}
	aliases := []string{
		"./" + noticeName, "././" + noticeName, ".//" + noticeName, "././/" + noticeName,
		"folder/../" + noticeName, "folder//../" + noticeName, "../" + noticeName,
		"/" + noticeName, "//" + noticeName, ".\\" + noticeName,
		"folder\\..\\" + noticeName, "C:/" + noticeName, "C:\\" + noticeName,
		"//server/share/" + noticeName, "\\\\server\\share\\" + noticeName,
		strings.ToLower(noticeName), noticeName + ".", noticeName + " ",
		"folder//file", "folder/./file",
	}
	for _, format := range []tar.Format{tar.FormatUSTAR, tar.FormatPAX, tar.FormatGNU} {
		t.Run(format.String(), func(t *testing.T) {
			for _, alias := range aliases {
				for _, kind := range []byte{tar.TypeReg, tar.TypeSymlink, tar.TypeLink} {
					t.Run(alias+"/"+string(kind), func(t *testing.T) {
						members := []archiveMember{{noticeName, tar.TypeReg, "", string(notice)}, {"replacement.txt", tar.TypeReg, "", "replacement without licence\n"}, {alias, kind, "", "replacement without licence\n"}}
						if kind != tar.TypeReg {
							members[2].link = "replacement.txt"
							members[2].body = ""
						}
						archive := writeMemberArchive(t, format, members)
						if err := CheckReleaseArchiveNotice(archive, notice); err == nil {
							t.Fatal("accepted unsafe/noncanonical member alias")
						} else if !strings.Contains(err.Error(), "members.tar.gz") {
							t.Fatalf("missing archive diagnostic: %v", err)
						}
					})
				}
			}
			// Links can also redirect later ordinary paths through an alias inode or
			// directory even when the link itself has an unrelated canonical name.
			for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink} {
				t.Run("indirect-link/"+string(kind), func(t *testing.T) {
					target := noticeName
					later := "alias"
					if kind == tar.TypeSymlink {
						target = "."
						later = "alias/" + noticeName
					}
					archive := writeMemberArchive(t, format, []archiveMember{{noticeName, tar.TypeReg, "", string(notice)}, {"alias", kind, target, ""}, {later, tar.TypeReg, "", "replacement without licence\n"}})
					if err := CheckReleaseArchiveNotice(archive, notice); err == nil {
						t.Fatal("accepted indirect link alias")
					}
				})
			}
			for _, target := range aliases {
				for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink} {
					t.Run("link-target/"+target+"/"+string(kind), func(t *testing.T) {
						archive := writeMemberArchive(t, format, []archiveMember{{noticeName, tar.TypeReg, "", string(notice)}, {"alias-link", kind, target, ""}, {"alias-link", tar.TypeReg, "", "replacement without licence\n"}})
						if err := CheckReleaseArchiveNotice(archive, notice); err == nil {
							t.Fatal("accepted link target alias")
						}
					})
				}
			}
			// A directory at the notice destination is not a regular notice.
			archiveDir := writeMemberArchive(t, format, []archiveMember{{noticeName, tar.TypeReg, "", string(notice)}, {noticeName + "/", tar.TypeDir, "", ""}})
			if err := CheckReleaseArchiveNotice(archiveDir, notice); err == nil {
				t.Fatal("accepted directory alias at notice destination")
			}
			// Canonical nested files and conventional directory headers remain valid.
			archive := writeMemberArchive(t, format, []archiveMember{{"assets/", tar.TypeDir, "", ""}, {"assets/data", tar.TypeReg, "", "fixture"}, {noticeName, tar.TypeReg, "", string(notice)}})
			if err := CheckReleaseArchiveNotice(archive, notice); err != nil {
				t.Fatalf("canonical archive rejected: %v", err)
			}
		})
	}
}

func TestReleaseArchiveNoticeExtractionWitness(t *testing.T) {
	tarBin, err := exec.LookPath("tar")
	if err != nil {
		t.Skip("native tar unavailable for extraction witness")
	}
	lock, cache := pinnedFixture(t)
	notice, err := GenerateNotice(lock, cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []tar.Format{tar.FormatUSTAR, tar.FormatPAX, tar.FormatGNU} {
		for _, alias := range []bool{false, true} {
			name := "canonical"
			if alias {
				name = "repeated-dot-alias"
			}
			t.Run(format.String()+"/"+name, func(t *testing.T) {
				members := []archiveMember{{noticeName, tar.TypeReg, "", string(notice)}}
				replacement := "REPLACED WITHOUT LICENCE\n"
				if alias {
					members = append(members, archiveMember{"././" + noticeName, tar.TypeReg, "", replacement})
				}
				archive := writeMemberArchive(t, format, members)
				checkErr := CheckReleaseArchiveNotice(archive, notice)
				dest := t.TempDir()
				// Only an in-directory dot-prefix alias is extracted. Traversal, absolute
				// and link fixtures above are never passed to a filesystem extractor.
				if out, err := exec.Command(tarBin, "-xzf", archive, "-C", dest).CombinedOutput(); err != nil {
					t.Fatalf("native tar extraction: %v: %s", err, out)
				}
				got, err := os.ReadFile(filepath.Join(dest, noticeName))
				if err != nil {
					t.Fatal(err)
				}
				if alias {
					if string(got) != replacement {
						t.Fatalf("alias did not reproduce overwrite: %q", got)
					}
					t.Log("native tar overwrote canonical notice through ././ alias")
					if checkErr == nil {
						t.Fatal("validator accepted archive whose extracted notice was overwritten")
					}
				} else {
					if checkErr != nil {
						t.Fatal(checkErr)
					}
					if !bytes.Equal(got, notice) {
						t.Fatal("accepted archive extracted a different notice")
					}
				}
			})
		}
	}
}
