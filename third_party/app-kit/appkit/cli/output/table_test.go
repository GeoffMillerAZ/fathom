package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"

	"github.com/geoffmilleraz/app-kit/appkit/cli/output"
)

// wantPlainFailure is byte-for-byte what
// kits/env-kit/internal/doctor.Render produces for the same rows, per its
// format `"%-7s %-*s  %s\n"` + the failure-remedy block. It is embedded
// here (rather than imported — this package cannot depend on the
// platform-engineering repo) as the migration's byte-compatibility proof:
// S1 emits this shape independently, S3 later asserts bosun's own golden
// still matches after switching to RenderStatusTable.
const wantPlainFailure = "OK      git         found at /usr/bin/git\n" +
	"FAIL    nix-daemon  nix store ping --store daemon exited 1: connection refused\n" +
	"UNKNOWN direnv      not installed (optional)\n" +
	"\n1 check failed. Remedies:\n" +
	"  nix-daemon\n" +
	"    sudo launchctl kickstart -k system/org.nixos.nix-daemon\n"

func TestStatusTablePlainMatchesDoctorGolden(t *testing.T) {
	rows := []output.StatusRow{
		{ID: "git", Status: "ok", Detail: "found at /usr/bin/git"},
		{ID: "nix-daemon", Status: "fail", Detail: "nix store ping --store daemon exited 1: connection refused", Remedy: "sudo launchctl kickstart -k system/org.nixos.nix-daemon"},
		{ID: "direnv", Status: "unknown", Detail: "not installed (optional)"},
	}

	for _, profile := range []output.Profile{colorprofile.Ascii, colorprofile.NoTTY} {
		var buf bytes.Buffer
		output.RenderStatusTable(&buf, rows, profile)
		if buf.String() != wantPlainFailure {
			t.Errorf("profile %s:\ngot:\n%s\nwant:\n%s", profile, buf.String(), wantPlainFailure)
		}
	}
}

func TestStatusTablePlainAllOK(t *testing.T) {
	rows := []output.StatusRow{
		{ID: "git", Status: "ok", Detail: "found at /usr/bin/git"},
	}
	var buf bytes.Buffer
	output.RenderStatusTable(&buf, rows, colorprofile.Ascii)
	want := "OK      git  found at /usr/bin/git\n\nhealthy\n"
	if buf.String() != want {
		t.Errorf("got:\n%q\nwant:\n%q", buf.String(), want)
	}
}

func TestStatusTablePlainUnknownOnly(t *testing.T) {
	rows := []output.StatusRow{
		{ID: "direnv", Status: "unknown", Detail: "not installed (optional)"},
	}
	var buf bytes.Buffer
	output.RenderStatusTable(&buf, rows, colorprofile.NoTTY)
	want := "UNKNOWN direnv  not installed (optional)\n\n1 check could not be determined.\n"
	if buf.String() != want {
		t.Errorf("got:\n%q\nwant:\n%q", buf.String(), want)
	}
}

// TestStatusTableStyledContainsANSI forces TrueColor and asserts the
// styled path actually emits ANSI escapes, while the visible text (status
// word, ids, details, remedy ids) survives stripping them.
func TestStatusTableStyledContainsANSI(t *testing.T) {
	rows := []output.StatusRow{
		{ID: "git", Status: "ok", Detail: "found at /usr/bin/git"},
		{ID: "nix-daemon", Status: "fail", Detail: "daemon unreachable", Remedy: "restart the daemon"},
	}
	var buf bytes.Buffer
	output.RenderStatusTable(&buf, rows, colorprofile.TrueColor)
	got := buf.String()

	const esc = "\x1b["
	if !strings.Contains(got, esc) {
		t.Fatalf("styled render contains no ANSI escapes:\n%q", got)
	}
	for _, want := range []string{"OK", "FAIL", "git", "nix-daemon", "daemon unreachable", "restart the daemon"} {
		if !strings.Contains(got, want) {
			t.Errorf("styled render missing substring %q:\n%q", want, got)
		}
	}
}

// TestStatusTableStyledPreservesAlignment proves the styled path's ANSI
// codes don't throw off column alignment: after stripping escapes, the
// output is byte-identical to the plain render.
func TestStatusTableStyledPreservesAlignment(t *testing.T) {
	rows := []output.StatusRow{
		{ID: "git", Status: "ok", Detail: "found at /usr/bin/git"},
		{ID: "nix-daemon", Status: "fail", Detail: "boom", Remedy: "fix it"},
	}
	var plain, styled bytes.Buffer
	output.RenderStatusTable(&plain, rows, colorprofile.Ascii)
	output.RenderStatusTable(&styled, rows, colorprofile.TrueColor)

	if got, want := stripANSI(styled.String()), plain.String(); got != want {
		t.Errorf("stripped styled output does not match plain:\ngot:  %q\nwant: %q", got, want)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if r == 'm' {
				inEsc = false
			}
		case r == '\x1b':
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
