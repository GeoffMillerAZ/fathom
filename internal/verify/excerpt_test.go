package verify

// excerpt_test.go — EVX-1's shared excerpt vectors and EVX-5's line-shift
// fixture (docs/plans/portable-receipts.md; vision 43 section 6.2).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type excerptVector struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	OK        bool   `json:"ok"`
	Excerpt   string `json:"excerpt"`
	SHA256    string `json:"sha256"`
}

func loadExcerptVectors(t *testing.T) []excerptVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "excerpt", "vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Vectors []excerptVector `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Vectors) == 0 {
		t.Fatal("no vectors")
	}
	return doc.Vectors
}

// TestExcerptFingerprint_SharedVectors pins the excerpt bytes and their
// digest: Review Kit's SpanRef.ContentHash formula (lowercase hex SHA-256 of
// the raw bytes, review-kit internal/evidence contentHash) over the bytes
// from the first byte of start_line through the last byte of end_line, its
// newline included when it has one. A file with no final newline is covered.
func TestExcerptFingerprint_SharedVectors(t *testing.T) {
	sawNoFinalNewline := false
	for _, v := range loadExcerptVectors(t) {
		got, ok := excerptFingerprint([]byte(v.Content), v.StartLine, v.EndLine)
		if ok != v.OK {
			t.Errorf("%s: ok = %v, want %v", v.Name, ok, v.OK)
			continue
		}
		if !v.OK {
			if got != "" {
				t.Errorf("%s: a range the file does not hold has fingerprint %q, want none", v.Name, got)
			}
			continue
		}
		sum := sha256.Sum256([]byte(v.Excerpt))
		if want := hex.EncodeToString(sum[:]); want != v.SHA256 {
			t.Fatalf("%s: vector's sha256 %s is not the ContentHash of its excerpt (%s)", v.Name, v.SHA256, want)
		}
		if got != v.SHA256 {
			t.Errorf("%s: excerptFingerprint = %s, want %s (excerpt %q)", v.Name, got, v.SHA256, v.Excerpt)
		}
		if exp, _ := ExcerptSHA256([]byte(v.Content), v.StartLine, v.EndLine); exp != got {
			t.Errorf("%s: ExcerptSHA256 = %s, excerptFingerprint = %s: one formula", v.Name, exp, got)
		}
		if !strings.HasSuffix(v.Content, "\n") && v.EndLine > 0 && strings.HasSuffix(v.Content, v.Excerpt) {
			sawNoFinalNewline = true
		}
	}
	if !sawNoFinalNewline {
		t.Error("the shared vectors must cover a file with no final newline")
	}
}

// fixtureGit runs git in dir with an isolated configuration (no global or
// system config, a fixed identity and dates) and fails the test on error.
func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// TestExcerptFingerprint_SurvivesLineShift is EVX-5: a finding's fingerprint
// survives a 40-line shift above it. Commit A holds the cited lines at 3-4;
// commit B inserts 40 lines above them, so they sit at 43-44. The excerpt
// fingerprint of the cited lines, read from each commit's blob, is the same,
// while an edit inside the cited lines changes it.
func TestExcerptFingerprint_SurvivesLineShift(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	fixtureGit(t, dir, "init", "-q")
	cited := "func Risky() {\n\tpanic(\"boom\")\n"
	body := "package p\n\n" + cited + "}\n"
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(body)
	fixtureGit(t, dir, "add", "p.go")
	fixtureGit(t, dir, "commit", "-q", "-m", "a")
	commitA := strings.TrimSpace(fixtureGit(t, dir, "rev-parse", "HEAD"))

	var shift strings.Builder
	for i := 0; i < 40; i++ {
		shift.WriteString("// filler line\n")
	}
	write("package p\n\n" + shift.String() + cited + "}\n")
	fixtureGit(t, dir, "commit", "-q", "-am", "b")
	commitB := strings.TrimSpace(fixtureGit(t, dir, "rev-parse", "HEAD"))

	blobA := []byte(fixtureGit(t, dir, "cat-file", "blob", commitA+":p.go"))
	blobB := []byte(fixtureGit(t, dir, "cat-file", "blob", commitB+":p.go"))
	fa, okA := excerptFingerprint(blobA, 3, 4)
	fb, okB := excerptFingerprint(blobB, 43, 44)
	if !okA || !okB {
		t.Fatalf("cited lines not found: A %v, B %v", okA, okB)
	}
	if fa != fb {
		t.Fatalf("fingerprint moved with a 40-line shift above the finding: %s at A:3-4, %s at B:43-44", fa, fb)
	}
	if fs, _ := excerptFingerprint(blobB, 3, 4); fs == fa {
		t.Fatal("the fingerprint ignores the cited bytes: B's lines 3-4 are filler, yet they read the same")
	}
	write("package p\n\n" + shift.String() + "func Risky() {\n\tpanic(\"changed\")\n}\n")
	fixtureGit(t, dir, "commit", "-q", "-am", "c")
	blobC := []byte(fixtureGit(t, dir, "cat-file", "blob", "HEAD:p.go"))
	if fc, _ := excerptFingerprint(blobC, 43, 44); fc == fa {
		t.Fatal("an edit inside the cited lines kept the fingerprint")
	}
}
