package main

// orgnames_test.go — TestNoOrgSpecificNames (PC-14): invariant I19 /
// ADR-011 lifted from `stated` to a test. The repo must carry no
// organization-specific names, hosts, catalogs or accounts in code, docs,
// tests or fixtures; anything org-specific is configuration.
//
// The denylist is itself org-agnostic by construction — a test that
// spelled out the names it hunts would put them in the repo. It scans for
// the SHAPES org content takes (an internal-TLD host, a non-example email
// domain, a real cloud account or bucket) and reads any literal tokens the
// operator wants denied from FATHOM_ORG_DENYLIST (comma-separated,
// case-insensitive) — local, never committed, the same law-13 shape as
// every other per-machine setting.
//
// The scanner itself (guardspec.OrgHits and its supporting helpers) is
// lifted into internal/guardspec (ADR-038 item 2) so this gate test and
// the enforced "org_token" pre-write check (enforce.go) share exactly one
// rule set — a hook denying an edit and this test rejecting the same
// commit must never be able to disagree about what counts as org-specific.

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/guardspec"
)

// orgSkipDirs are never scanned: vendored mirrors carry their own
// fixtures (agentic-kit's redaction corpus deliberately contains
// realistic emails), and build/scratch trees are not the repo.
var orgSkipDirs = map[string]bool{
	".git": true, "third_party": true, "workspace": true, "ref": true, "tmp": true,
	"node_modules": true, "bin": true, ".claude": true, "calibration": true,
}

// orgDerivedCorpora are committed files whose content is PINNED UPSTREAM
// DATA, not ours: a scenario corpus derived from a public benchmark
// carries that project's own test names verbatim (cli/cli's
// "at@signs.jpg" fixture reads as an email to the scanner). They are
// derived, hash-checked against their sources, and never hand-edited —
// ADR-011 governs what fathom writes, not what it quotes.
var orgDerivedCorpora = map[string]bool{
	"testdata/scenariobench/public-go.json": true,
	"testdata/scenariobench/reuse-go.json":  true, // re-shaped from public-go.json, same upstream names
}

// orgSkipExt marks binary or generated files.
var orgSkipExt = map[string]bool{
	".sqlite": true, ".db": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".svg": true, ".wasm": true, ".so": true, ".dylib": true, ".a": true, ".zip": true,
	".gz": true, ".tar": true, ".pdf": true, ".woff": true, ".woff2": true, ".ico": true,
	".sum": true, ".css": true,
}

// orgInternalHostSuffixes mirrors guardspec.OrgInternalHostSuffixes — the
// ONE list the scanner is built from and
// TestADR011EnforcedPatternsMatchOrgNames checks ADR-011's own served text
// against (O57 item 7): the ADR's prose must name exactly these, in this
// shape, so a reader of the ADR — human or a compiled obligation — learns
// what the gate actually checks instead of "invented, neutral names" alone.
var orgInternalHostSuffixes = guardspec.OrgInternalHostSuffixes

// orgHits scans one line and returns the org-shaped tokens on it —
// guardspec.OrgHits, the shared rule set (see this file's doc comment).
func orgHits(line string, denylist []string) []string {
	return guardspec.OrgHits(line, denylist)
}

// orgIdentityFile is the one tracked file whose identity lines are exempt
// from the shape scan: git's own author map. It carries the maintainer's own
// name and addresses, which every commit already publishes, so the scan's
// "non-placeholder email" shape reads an identity as an organization there.
const orgIdentityFile = ".mailmap"

// mailmapIdentityRe is a .mailmap identity line and nothing more: names, then
// one or two angle-bracketed addresses. No '@', ':' or '/' outside the
// brackets, so a URL, a bare address or a trailing note is not an identity line.
var mailmapIdentityRe = regexp.MustCompile(`^[^<>#@:/]*<[^<>\s]+>(?:[^<>#@:/]*<[^<>\s]+>)?\s*$`)

// orgLineHits scans one line of the tracked file rel: the shared rule set
// (orgHits), except that on a well-formed identity line of the root .mailmap
// the email-shape rule is lifted. The denylist, the internal-host rule and
// every other rule still scan it. A comment, a stray line and the same line
// in any other file are scanned in full.
func orgLineHits(rel, line string, denylist []string) []string {
	if rel == orgIdentityFile && mailmapIdentityRe.MatchString(line) {
		// The email-shape rule is the only one that needs an '@'; masking
		// it lifts that rule alone and leaves every domain for the others.
		return orgHits(strings.ReplaceAll(line, "@", " "), denylist)
	}
	return orgHits(line, denylist)
}

// orgScanFiles lists the files ADR-011 governs: the git-tracked tree
// (repo-relative, forward slashes). Untracked scratch is not the repo.
// Without git the whole tree is walked instead, minus the skip dirs.
func orgScanFiles(t *testing.T, root string) []string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err == nil {
		var files []string
		for _, f := range bytes.Split(out, []byte{0}) {
			if len(f) > 0 {
				files = append(files, string(f))
			}
		}
		return files
	}
	var files []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && orgSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files
}

// adr011SectionRe isolates ADR-011's own section of ADR.md — from its
// heading up to (not including) the next "## " heading — so the guard
// below reads exactly what a session sees when it reads ADR-011, not the
// whole document.
var adr011SectionRe = regexp.MustCompile(`(?s)## ADR-011 —.*?(?:\n## |\z)`)

// adr011BacktickedSuffixRe finds every backticked "*.suffix" pattern in
// ADR-011's text — the shape the ADR states its enforced host patterns in.
var adr011BacktickedSuffixRe = regexp.MustCompile("`\\*\\.([a-z]+)`")

// TestADR011EnforcedPatternsMatchOrgNames pins O57 item 7: ADR-011's own
// served text named "invented, neutral names" while the test it describes
// rejects any host under *.corp/*.internal/*.intranet/*.lan specifically —
// a good-faith reader of the served rule had no way to know that. This
// guard reads ADR.md's ADR-011 section directly and fails if the
// backticked "*.suffix" patterns it states ever diverge from
// orgInternalHostSuffixes (the list orgInternalHostRe is actually built
// from) in EITHER direction: the ADR claiming a pattern the code does not
// enforce, or enforcing one the ADR never names.
func TestADR011EnforcedPatternsMatchOrgNames(t *testing.T) {
	root := repoRootForDocs(t)
	raw, err := os.ReadFile(filepath.Join(root, "ADR.md"))
	if err != nil {
		t.Fatalf("reading ADR.md: %v", err)
	}
	section := adr011SectionRe.FindString(string(raw))
	if section == "" {
		t.Fatal("ADR.md has no ## ADR-011 section")
	}

	stated := map[string]bool{}
	for _, m := range adr011BacktickedSuffixRe.FindAllStringSubmatch(section, -1) {
		stated[m[1]] = true
	}
	enforced := map[string]bool{}
	for _, s := range orgInternalHostSuffixes {
		enforced[s] = true
	}

	for _, s := range orgInternalHostSuffixes {
		if !stated[s] {
			t.Errorf("ADR-011's text does not name `*.%s`, but orgInternalHostRe (TestNoOrgSpecificNames) enforces it", s)
		}
	}
	for s := range stated {
		if !enforced[s] {
			t.Errorf("ADR-011's text names `*.%s` as enforced, but orgInternalHostRe does not check it — the ADR promises more than the gate delivers", s)
		}
	}
}

// TestOrgLineHitsExemptsOnlyMailmapIdentityLines pins the one exemption to
// the shape scan: a well-formed identity line of the tracked .mailmap (names
// and angle-bracketed addresses, nothing else) is git identity data, the
// maintainer's own address that every commit already publishes, not an
// organization's name. A comment, a stray line, or the same line in any other
// file is scanned as before.
func TestOrgLineHitsExemptsOnlyMailmapIdentityLines(t *testing.T) {
	identity := "Jane Roe <jane@" + "mail.example.org> <jane@" + "corp-mail.example.net>"
	realShape := "Jane Roe <jane.roe@" + "acme-widgets.com> <jane@" + "host.example.net>"
	cases := []struct {
		name, rel, line string
		wantHit         bool
	}{
		{"an identity line of .mailmap", ".mailmap", realShape, false},
		{"a name-only identity line", ".mailmap", "Jane Roe <jane.roe@" + "acme-widgets.com>", false},
		{"the same line in another file", "docs/usage.md", realShape, true},
		{"the same line in a nested .mailmap", "docs/.mailmap", realShape, true},
		{"a comment naming an address", ".mailmap", "# reach jane.roe@" + "acme-widgets.com", true},
		{"a stray line with a bare address", ".mailmap", "jane.roe@" + "acme-widgets.com", true},
		{"a line carrying a URL", ".mailmap", "Jane Roe <jane.roe@" + "acme-widgets.com> https://acme-widgets.com/x", true},
		{"a placeholder identity is clean everywhere", "docs/usage.md", identity, false},
	}
	for _, tc := range cases {
		got := len(orgLineHits(tc.rel, tc.line, nil)) > 0
		if got != tc.wantHit {
			t.Errorf("%s: orgLineHits(%q, %q) hit = %v, want %v", tc.name, tc.rel, tc.line, got, tc.wantHit)
		}
	}

	// The exemption lifts the email-shape rule alone. An organization named
	// by the configured denylist, or an internal hostname, is still a hit on
	// a .mailmap identity line: a third party's work address is no
	// maintainer's identity.
	if hits := orgLineHits(".mailmap", realShape, []string{"acme-widgets"}); len(hits) == 0 {
		t.Errorf("a denylisted organization on a .mailmap identity line must hit: orgLineHits(%q) = none", realShape)
	}
	internalHost := "Jane Roe <jane@build." + "corp>"
	if hits := orgLineHits(".mailmap", internalHost, nil); len(hits) == 0 {
		t.Errorf("an internal hostname on a .mailmap identity line must hit: orgLineHits(%q) = none", internalHost)
	}
}

// certifies: invariant I19 (asserts)
func TestNoOrgSpecificNames(t *testing.T) {
	root := repoRootForDocs(t)
	var denylist []string
	for _, tok := range strings.Split(os.Getenv("FATHOM_ORG_DENYLIST"), ",") {
		if tok = strings.ToLower(strings.TrimSpace(tok)); tok != "" {
			denylist = append(denylist, tok)
		}
	}

	// The scanner itself, pinned: each shape is caught, each placeholder passes.
	for _, bad := range []string{
		"host = \"ci.build.corp\"", "reach me at dev@acme-widgets.com",
		"arn:aws:iam::210987654321:role/x", "gs://prod-ledger-eu/x",
	} {
		if len(orgHits(bad, nil)) == 0 {
			t.Errorf("scanner missed %q", bad)
		}
	}
	for _, ok := range []string{
		"alice@example.com wrote", "noreply@anthropic.com", "git@github.com:x/y.git",
		"arn:aws:s3:::example", "arn:aws:iam::123456789012:root", "s3://example-bucket/k",
		"fathom/internal/verify", "the internal package",
	} {
		if h := orgHits(ok, nil); len(h) > 0 {
			t.Errorf("scanner false-positive on %q: %v", ok, h)
		}
	}
	if h := orgHits("service: Widgets Inc fleet", []string{"widgets inc"}); len(h) != 1 {
		t.Errorf("FATHOM_ORG_DENYLIST tokens must be honored, got %v", h)
	}

	var findings []string
	// selfFixtures are files whose whole content IS org-shaped scanner
	// fixtures (pinned true-positive examples, ADR-038's enforce_test.go
	// among them) — shapes demonstrating the detector, never a real name.
	selfFixtures := map[string]bool{
		filepath.ToSlash(filepath.Join("cmd", "fathom", "orgnames_test.go")):         true,
		filepath.ToSlash(filepath.Join("cmd", "fathom", "edithook_enforce_test.go")): true,
		filepath.ToSlash(filepath.Join("internal", "guardspec", "enforce_test.go")):  true,
	}
	for _, rel := range orgScanFiles(t, root) {
		if selfFixtures[rel] || orgDerivedCorpora[rel] {
			continue // the scanner's own fixtures are shapes, not names; derived corpora quote upstream
		}
		if orgSkipDirs[strings.SplitN(rel, "/", 2)[0]] || orgSkipExt[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() > 4<<20 {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for n := 1; sc.Scan(); n++ {
			if hits := orgLineHits(rel, sc.Text(), denylist); len(hits) > 0 {
				findings = append(findings, rel+":"+itoa(n)+": "+strings.Join(hits, ", "))
			}
		}
		f.Close()
	}
	for i, f := range findings {
		if i >= 50 {
			t.Errorf("… %d more", len(findings)-50)
			break
		}
		t.Errorf("org-specific token (ADR-011 / I19): %s", f)
	}
}
