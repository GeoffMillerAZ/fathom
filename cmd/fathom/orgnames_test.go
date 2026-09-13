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

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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

var (
	// An internal-network host: a name under one of the private TLDs
	// (corp, internal, intranet, lan).
	orgInternalHostRe = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9-]*(?:\.[a-z0-9-]+)*\.(?:corp|internal|intranet|lan)\b`)
	// An email address; the domain is checked against the neutral set.
	orgEmailRe = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@((?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,})\b`)
	// A cloud account in an ARN (12 digits) other than the documented
	// placeholder accounts.
	orgARNRe = regexp.MustCompile(`arn:aws:[a-z0-9-]*:[a-z0-9-]*:(\d{12}):`)
	// A storage bucket URI.
	orgBucketRe = regexp.MustCompile(`\b(?:gs|s3)://([a-z0-9][a-z0-9._-]{2,})`)
)

// orgNeutralEmailDomain reports whether an email domain is a documented
// placeholder (RFC 2606/6761 names) or tooling identity, not an org.
func orgNeutralEmailDomain(d string) bool {
	d = strings.ToLower(d)
	switch {
	case d == "example.com", d == "example.net", d == "example.org",
		strings.HasSuffix(d, ".example.com"), strings.HasSuffix(d, ".example.net"), strings.HasSuffix(d, ".example.org"),
		strings.HasSuffix(d, ".example"), strings.HasSuffix(d, ".test"), strings.HasSuffix(d, ".invalid"), strings.HasSuffix(d, ".localhost"),
		d == "localhost", d == "github.com", d == "anthropic.com":
		return true
	}
	return false
}

func orgPlaceholderAccount(acct string) bool {
	switch acct {
	case "123456789012", "000000000000", "111111111111", "999999999999":
		return true
	}
	return false
}

func orgNeutralBucket(b string) bool {
	b = strings.ToLower(b)
	return strings.Contains(b, "example") || strings.HasPrefix(b, "my-") || strings.HasPrefix(b, "bucket") || strings.Contains(b, "fauxtown")
}

// orgHits scans one line and returns the org-shaped tokens on it.
func orgHits(line string, denylist []string) []string {
	var hits []string
	lower := strings.ToLower(line)
	// Cheap substring gates before each regexp: the scan covers every
	// tracked line of the repo and must stay well under a second.
	if strings.Contains(lower, ".corp") || strings.Contains(lower, ".internal") || strings.Contains(lower, ".intranet") || strings.Contains(lower, ".lan") {
		hits = append(hits, orgInternalHostRe.FindAllString(line, -1)...)
	}
	if strings.Contains(line, "@") {
		for _, m := range orgEmailRe.FindAllStringSubmatch(line, -1) {
			if !orgNeutralEmailDomain(m[1]) && !strings.HasPrefix(strings.ToLower(m[0]), "noreply@") {
				hits = append(hits, m[0])
			}
		}
	}
	if strings.Contains(line, "arn:aws") {
		for _, m := range orgARNRe.FindAllStringSubmatch(line, -1) {
			if !orgPlaceholderAccount(m[1]) {
				hits = append(hits, m[0])
			}
		}
	}
	if strings.Contains(line, "://") {
		for _, m := range orgBucketRe.FindAllStringSubmatch(line, -1) {
			if !orgNeutralBucket(m[1]) {
				hits = append(hits, m[0])
			}
		}
	}
	for _, tok := range denylist {
		if tok != "" && strings.Contains(lower, tok) {
			hits = append(hits, tok)
		}
	}
	return hits
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
	self := filepath.ToSlash(filepath.Join("cmd", "fathom", "orgnames_test.go"))
	for _, rel := range orgScanFiles(t, root) {
		if rel == self || orgDerivedCorpora[rel] {
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
			if hits := orgHits(sc.Text(), denylist); len(hits) > 0 {
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
