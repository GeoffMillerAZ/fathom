package main

// Tests for ExtractSection and the testable run() entry point, following the
// same in-package-tests-exported-in-main pattern as cmd/constitution-lint.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return string(data)
}

func TestExtractSection(t *testing.T) {
	changelog := readTestdata(t, "changelog-basic.md")

	tests := []struct {
		name        string
		version     string
		wantContain []string // substrings that must appear in the extracted section
		wantAbsent  []string // substrings that must NOT appear (boundary check)
		wantErr     bool
	}{
		{
			name:    "Unreleased section",
			version: "Unreleased",
			wantContain: []string{
				"## [Unreleased]",
				"Something new not yet released.",
			},
			wantAbsent: []string{"## [1.2.0]", "Feature A."},
		},
		{
			name:    "middle version, bare",
			version: "1.2.0",
			wantContain: []string{
				"## [1.2.0] — 2026-01-02",
				"Feature A.",
				"Feature B.",
				"Bug X.",
			},
			wantAbsent: []string{"## [1.1.0]", "Initial widget support.", "## [Unreleased]"},
		},
		{
			name:    "middle version, v-prefixed",
			version: "v1.2.0",
			wantContain: []string{
				"## [1.2.0] — 2026-01-02",
				"Feature A.",
			},
			wantAbsent: []string{"## [1.1.0]"},
		},
		{
			name:    "another middle version",
			version: "1.1.0",
			wantContain: []string{
				"## [1.1.0] — 2025-12-01",
				"Initial widget support.",
			},
			wantAbsent: []string{"## [1.2.0]", "## [1.0.0]", "Feature A."},
		},
		{
			name:    "last version, no trailing heading (EOF)",
			version: "1.0.0",
			wantContain: []string{
				"## [1.0.0] — 2025-11-15",
				"Initial release. No heading follows this one — end of file.",
			},
		},
		{
			name:    "last version, EOF, v-prefixed",
			version: "v1.0.0",
			wantContain: []string{
				"## [1.0.0] — 2025-11-15",
			},
		},
		{
			name:    "version not found",
			version: "9.9.9",
			wantErr: true,
		},
		{
			name:    "bare version not found even with v requested",
			version: "vNope",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractSection(changelog, tc.version)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ExtractSection(%q) = %q, want error", tc.version, got)
				}
				if !strings.Contains(err.Error(), tc.version) {
					t.Errorf("error %q does not mention version %q", err, tc.version)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractSection(%q) unexpected error: %v", tc.version, err)
			}
			for _, want := range tc.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("ExtractSection(%q) missing %q; got:\n%s", tc.version, want, got)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("ExtractSection(%q) unexpectedly contains %q; got:\n%s", tc.version, absent, got)
				}
			}
		})
	}
}

// TestExtractSectionEOFIsExactSuffix pins down the EOF case precisely: the
// extracted section for the file's last heading must run all the way to the
// literal end of the input, byte for byte.
func TestExtractSectionEOFIsExactSuffix(t *testing.T) {
	changelog := readTestdata(t, "changelog-basic.md")
	got, err := ExtractSection(changelog, "1.0.0")
	if err != nil {
		t.Fatalf("ExtractSection: %v", err)
	}
	wantStart := strings.Index(changelog, "## [1.0.0]")
	if wantStart < 0 {
		t.Fatal("testdata fixture missing ## [1.0.0] heading")
	}
	want := changelog[wantStart:]
	if got != want {
		t.Errorf("EOF section not an exact suffix:\ngot:  %q\nwant: %q", got, want)
	}
}

// TestExtractSectionRealChangelog guards against the parser being subtly
// wrong on the real CHANGELOG.md's actual formatting — fixtures alone might
// not catch a quirk (extra header prose, differing dash characters, etc.)
// that only the real file has.
func TestExtractSectionRealChangelog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("read real CHANGELOG.md: %v", err)
	}
	changelog := string(data)

	for _, version := range []string{"0.5.0", "v0.5.0"} {
		t.Run(version, func(t *testing.T) {
			got, err := ExtractSection(changelog, version)
			if err != nil {
				t.Fatalf("ExtractSection(%q): %v", version, err)
			}
			if got == "" {
				t.Fatal("ExtractSection returned an empty string")
			}
			if !strings.Contains(got, "## [0.5.0]") {
				t.Errorf("result missing the 0.5.0 heading; got first 200 bytes:\n%s", firstN(got, 200))
			}
			if strings.Contains(got, "## [0.4.0]") {
				t.Errorf("result bled into the next section (## [0.4.0] present)")
			}
		})
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// --- run() / CLI-level tests -------------------------------------------

// writeTempChangelog copies testdata/changelog-basic.md into a fresh temp
// dir as CHANGELOG.md, mirroring the real repo-root layout run() expects,
// and returns the temp dir path.
func writeTempChangelog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	content := readTestdata(t, "changelog-basic.md")
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write temp CHANGELOG.md: %v", err)
	}
	return dir
}

func TestRunSuccess(t *testing.T) {
	dir := writeTempChangelog(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-root", dir, "1.2.0"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Feature A.") {
		t.Errorf("stdout missing expected section content:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunVersionNotFound(t *testing.T) {
	dir := writeTempChangelog(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-root", dir, "9.9.9"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "9.9.9") {
		t.Errorf("stderr missing version in message: %s", stderr.String())
	}
}

func TestRunMissingChangelog(t *testing.T) {
	dir := t.TempDir() // no CHANGELOG.md written
	var stdout, stderr bytes.Buffer
	code := run([]string{"-root", dir, "1.0.0"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr:\n%s", code, stderr.String())
	}
	if stderr.Len() == 0 {
		t.Error("expected an stderr message on read failure")
	}
}

func TestRunUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no version arg", []string{}},
		{"too many args", []string{"1.0.0", "extra"}},
		{"unknown flag", []string{"-definitely-not-a-flag", "1.0.0"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			if code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", code, stderr.String())
			}
		})
	}
}
