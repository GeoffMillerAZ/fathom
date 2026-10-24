package receipt

// anchor_test.go — EVX-1 (docs/plans/portable-receipts.md; vision 43 D3 and
// section 6.2): AnchorAt over fixture repositories, one per anchor state.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

// fixtureRepo is a git repository in a temporary directory, configured
// apart from the machine's (no global or system config, fixed identity and
// dates), so a fixture commit is the same on every machine.
type fixtureRepo struct {
	t   *testing.T
	dir string
}

func newFixtureRepo(t *testing.T, initArgs ...string) *fixtureRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := &fixtureRepo{t: t, dir: dir}
	r.git(append([]string{"init", "-q"}, initArgs...)...)
	// No background auto-maintenance: after a commit git may start a detached
	// `maintenance run --auto` that creates and removes .git/objects/maintenance.lock
	// while a test snapshots the git directory (TestAnchorAt_ReadOnly flaked on CI).
	r.git("config", "maintenance.auto", "false")
	r.git("config", "gc.auto", "0")
	return r
}

func (r *fixtureRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *fixtureRepo) write(rel, content string) {
	r.t.Helper()
	p := filepath.Join(r.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *fixtureRepo) commitAll(msg string) string {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

func mustResolve(t *testing.T, dir, rev string) Commit {
	t.Helper()
	c, err := ResolveCommit(context.Background(), dir, rev)
	if err != nil {
		t.Fatalf("ResolveCommit(%s, %s): %v", dir, rev, err)
	}
	return c
}

func TestAnchorAt_Committed(t *testing.T) {
	r := newFixtureRepo(t)
	content := "one\ntwo\nthree" // no final newline: the last line ends at EOF
	r.write("docs/a.txt", content)
	head := r.commitAll("a")

	c := mustResolve(t, filepath.Join(r.dir, "docs"), "HEAD")
	if c.Root != r.dir || c.ID != head || c.ObjectFormat != "sha1" {
		t.Fatalf("ResolveCommit = %+v, want root %s, id %s, sha1", c, r.dir, head)
	}
	a, err := AnchorAt(context.Background(), c, Span{Path: "docs/a.txt", StartLine: 2, EndLine: 3}, Source{})
	if err != nil {
		t.Fatal(err)
	}
	want := Anchor{
		Commit: head, ObjectFormat: "sha1", Path: "docs/a.txt", StartLine: 2, EndLine: 3,
		BlobID:            r.git("rev-parse", "HEAD:docs/a.txt"),
		ExcerptSHA256:     sha256Hex([]byte("two\nthree")),
		FathomFingerprint: fusion.FingerprintHash([]byte(content)),
		State:             StateCommitted, Clean: true,
	}
	if a != want {
		t.Fatalf("AnchorAt =\n %+v\nwant\n %+v", a, want)
	}
	// The stored fingerprint, when the index holds one, is what the
	// producer read: equal to the commit's, the anchor is committed even
	// when the work tree has moved on.
	r.write("docs/a.txt", "edited\n")
	a2, err := AnchorAt(context.Background(), c, Span{Path: "docs/a.txt", StartLine: 1, EndLine: 1}, Source{Fingerprint: fusion.FingerprintHash([]byte(content))})
	if err != nil || a2.State != StateCommitted || !a2.Clean || a2.ExcerptSHA256 != sha256Hex([]byte("one\n")) {
		t.Fatalf("stored fingerprint equal to the commit's: %+v, %v", a2, err)
	}
}

// TestAnchorAt_WholeFile: start = end = 0 is the whole file (SpanRef: zero
// lines mean the whole file).
func TestAnchorAt_WholeFile(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("a.txt", "one\ntwo")
	r.commitAll("a")
	c := mustResolve(t, r.dir, "HEAD")
	a, err := AnchorAt(context.Background(), c, Span{Path: "a.txt"}, Source{})
	if err != nil || a.State != StateCommitted || a.ExcerptSHA256 != sha256Hex([]byte("one\ntwo")) {
		t.Fatalf("whole file: %+v, %v", a, err)
	}
}

func TestAnchorAt_SHA256ObjectFormat(t *testing.T) {
	r := newFixtureRepo(t, "--object-format=sha256")
	r.write("a.go", "package a\n")
	head := r.commitAll("a")
	c := mustResolve(t, r.dir, "HEAD")
	if c.ObjectFormat != "sha256" || len(c.ID) != 64 || c.ID != head {
		t.Fatalf("ResolveCommit = %+v, want a 64-hex sha256 commit %s", c, head)
	}
	a, err := AnchorAt(context.Background(), c, Span{Path: "a.go", StartLine: 1, EndLine: 1}, Source{})
	if err != nil {
		t.Fatal(err)
	}
	if a.ObjectFormat != "sha256" || len(a.BlobID) != 64 || a.BlobID != r.git("rev-parse", "HEAD:a.go") || a.State != StateCommitted {
		t.Fatalf("sha256 anchor = %+v", a)
	}
}

func TestAnchorAt_Uncommitted(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("a.txt", "one\ntwo\n")
	r.commitAll("a")
	c := mustResolve(t, r.dir, "HEAD")
	r.write("a.txt", "one\nTWO\n") // the producer read the work tree's bytes

	a, err := AnchorAt(context.Background(), c, Span{Path: "a.txt", StartLine: 2, EndLine: 2}, Source{})
	if err != nil {
		t.Fatal(err)
	}
	if a.State != StateUncommitted || a.Clean {
		t.Fatalf("an edited work tree reads %s clean=%v, want uncommitted, not clean", a.State, a.Clean)
	}
	if a.ExcerptSHA256 != "" {
		t.Errorf("an uncommitted anchor carries an excerpt digest %s of bytes the producer did not read", a.ExcerptSHA256)
	}
	if a.BlobID != r.git("rev-parse", "HEAD:a.txt") || a.FathomFingerprint != fusion.FingerprintHash([]byte("one\ntwo\n")) {
		t.Errorf("an uncommitted anchor still names the commit's blob and fingerprint: %+v", a)
	}

	// A stored fingerprint that differs from the commit's is uncommitted too.
	a, err = AnchorAt(context.Background(), c, Span{Path: "a.txt", StartLine: 1, EndLine: 1}, Source{Fingerprint: fusion.FingerprintHash([]byte("other"))})
	if err != nil || a.State != StateUncommitted || a.Clean {
		t.Fatalf("stored fingerprint differs: %+v, %v", a, err)
	}

	// A file the commit does not hold, read from the work tree.
	r.write("new.txt", "fresh\n")
	a, err = AnchorAt(context.Background(), c, Span{Path: "new.txt", StartLine: 1, EndLine: 1}, Source{})
	if err != nil || a.State != StateUncommitted || a.Clean || a.BlobID != "" || a.ExcerptSHA256 != "" {
		t.Fatalf("a file not at the commit: %+v, %v", a, err)
	}
}

func TestAnchorAt_Filtered(t *testing.T) {
	r := newFixtureRepo(t)
	r.write(".gitattributes", "*.crlf text eol=crlf\n*.id ident\n")
	r.write("a.crlf", "one\ntwo\n")
	r.write("b.id", "// $Id$\nbody\n")
	r.commitAll("a")
	// Check the files out again, as a clone would: git writes the
	// filtered bytes (CRLF line ends, an expanded $Id$).
	for _, f := range []string{"a.crlf", "b.id"} {
		if err := os.Remove(filepath.Join(r.dir, f)); err != nil {
			t.Fatal(err)
		}
	}
	r.git("checkout", "--", "a.crlf", "b.id")
	worktree, err := os.ReadFile(filepath.Join(r.dir, "a.crlf"))
	if err != nil || string(worktree) != "one\r\ntwo\r\n" {
		t.Fatalf("fixture: eol=crlf checkout wrote %q", worktree)
	}
	c := mustResolve(t, r.dir, "HEAD")

	a, err := AnchorAt(context.Background(), c, Span{Path: "a.crlf", StartLine: 2, EndLine: 2}, Source{})
	if err != nil {
		t.Fatal(err)
	}
	if a.State != StateFiltered || !a.Clean {
		t.Fatalf("eol=crlf: state %s clean %v, want filtered and clean (the producer read the checkout's bytes)", a.State, a.Clean)
	}
	if a.FathomFingerprint != fusion.FingerprintHash(worktree) {
		t.Errorf("fathom_fingerprint = %s, want FingerprintHash of the checkout's bytes", a.FathomFingerprint)
	}
	if a.ExcerptSHA256 != sha256Hex([]byte("two\r\n")) {
		t.Errorf("excerpt digest = %s, want the checkout's line, CR kept", a.ExcerptSHA256)
	}

	b, err := AnchorAt(context.Background(), c, Span{Path: "b.id", StartLine: 2, EndLine: 2}, Source{})
	if err != nil || b.State != StateFiltered {
		t.Fatalf("ident: %+v, %v, want filtered", b, err)
	}
}

func TestAnchorAt_LFSPointerIsFiltered(t *testing.T) {
	r := newFixtureRepo(t)
	real := "large\ncontent\n"
	pointer := "version https://git-lfs.github.com/spec/v1\noid sha256:" + sha256Hex([]byte(real)) + "\nsize 14\n"
	r.write("big.bin", pointer)
	r.commitAll("a")
	r.write("big.bin", real) // what an LFS checkout leaves in the work tree
	c := mustResolve(t, r.dir, "HEAD")

	a, err := AnchorAt(context.Background(), c, Span{Path: "big.bin", StartLine: 2, EndLine: 2}, Source{})
	if err != nil {
		t.Fatal(err)
	}
	if a.State != StateFiltered || !a.Clean || a.FathomFingerprint != fusion.FingerprintHash([]byte(real)) {
		t.Fatalf("LFS pointer with its object in the work tree: %+v, want filtered, clean, the object's fingerprint", a)
	}
	r.write("big.bin", "not the object\n")
	a, err = AnchorAt(context.Background(), c, Span{Path: "big.bin", StartLine: 1, EndLine: 1}, Source{})
	if err != nil || a.State != StateFiltered || a.Clean || a.ExcerptSHA256 != "" {
		t.Fatalf("LFS pointer whose object is absent: %+v, %v, want filtered, not clean, no digest", a, err)
	}
}

func TestAnchorAt_Withheld(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("alertmanager.yml", "receivers:\n- name: x\n  slack_api_url: s3cr3t\n")
	r.commitAll("a")
	c := mustResolve(t, r.dir, "HEAD")
	a, err := AnchorAt(context.Background(), c, Span{Path: "alertmanager.yml", StartLine: 3, EndLine: 3}, Source{Withheld: "a file whose values may be secrets (AlertmanagerReceiver)"})
	if err != nil {
		t.Fatal(err)
	}
	if a.State != StateWithheld || !a.Clean || a.Reason == "" {
		t.Fatalf("withheld: %+v", a)
	}
	if a.ExcerptSHA256 != "" || a.BlobID != "" || a.FathomFingerprint != "" {
		t.Fatalf("a withheld anchor carries a digest a short file's secret could be guessed from (ADR-065): %+v", a)
	}
}

func TestAnchorAt_Refusals(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("d/a.txt", "one\n")
	r.commitAll("a")
	c := mustResolve(t, r.dir, "HEAD")
	for name, s := range map[string]Span{
		"past the end":    {Path: "d/a.txt", StartLine: 2, EndLine: 2},
		"end before":      {Path: "d/a.txt", StartLine: 1, EndLine: 0},
		"absolute":        {Path: filepath.Join(r.dir, "d/a.txt"), StartLine: 1, EndLine: 1},
		"escapes":         {Path: "../a.txt", StartLine: 1, EndLine: 1},
		"a directory":     {Path: "d", StartLine: 1, EndLine: 1},
		"nowhere at all":  {Path: "missing.txt", StartLine: 1, EndLine: 1},
		"empty path":      {StartLine: 1, EndLine: 1},
		"option-shaped":   {Path: "-c", StartLine: 1, EndLine: 1},
		"negative start":  {Path: "d/a.txt", StartLine: -1, EndLine: 1},
		"start zero":      {Path: "d/a.txt", StartLine: 0, EndLine: 1},
		"unclean segment": {Path: "d/../d/a.txt", StartLine: 1, EndLine: 1},
	} {
		if a, err := AnchorAt(context.Background(), c, s, Source{}); err == nil {
			t.Errorf("%s: AnchorAt(%+v) = %+v, want an error", name, s, a)
		}
	}
	if _, err := ResolveCommit(context.Background(), t.TempDir(), "HEAD"); err == nil {
		t.Error("ResolveCommit outside a repository: want an error")
	}
	if _, err := ResolveCommit(context.Background(), r.dir, "--output=x"); err == nil {
		t.Error("ResolveCommit of an option-shaped rev: want an error")
	}
}

// TestAnchorAt_ReadOnly: anchoring writes nothing into the repository —
// refs, the index and the object store are byte-identical before and after.
func TestAnchorAt_ReadOnly(t *testing.T) {
	r := newFixtureRepo(t)
	r.write("a.txt", "one\ntwo\n")
	r.commitAll("a")
	r.write("a.txt", "one\nTWO\n")
	before := snapshotGitDir(t, filepath.Join(r.dir, ".git"))
	c := mustResolve(t, r.dir, "HEAD")
	for i := 0; i < 2; i++ {
		if _, err := AnchorAt(context.Background(), c, Span{Path: "a.txt", StartLine: 1, EndLine: 2}, Source{}); err != nil {
			t.Fatal(err)
		}
	}
	if after := snapshotGitDir(t, filepath.Join(r.dir, ".git")); after != before {
		t.Fatal("AnchorAt wrote into .git")
	}
}

func snapshotGitDir(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		b.WriteString(p + " " + sha256Hex(raw) + " " + info.ModTime().String() + "\n")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
