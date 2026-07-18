package awscred

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// writeScript writes an executable shell script to dir/name and returns its
// path. Skips the test on non-Unix (the credential_process contract is
// exec-based and this repo's CI/dev targets darwin/linux).
func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("credential_process fixture scripts are POSIX shell scripts")
	}
	path := filepath.Join(dir, name)
	full := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(full), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return path
}

func TestSplitCommandLine(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"tool", []string{"tool"}},
		{"tool --arg value", []string{"tool", "--arg", "value"}},
		{`tool --arg "quoted value"`, []string{"tool", "--arg", "quoted value"}},
		{"tool --arg 'single quoted'", []string{"tool", "--arg", "single quoted"}},
		{"  tool   --spaced  ", []string{"tool", "--spaced"}},
		{"", nil},
	}
	for _, tc := range cases {
		got := splitCommandLine(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("splitCommandLine(%q) = %#v, want %#v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("splitCommandLine(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func TestRunCredentialProcess_Success(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `cat <<'EOF'
{"Version":1,"AccessKeyId":"AKIDPROCESS","SecretAccessKey":"secretprocess","SessionToken":"tokenprocess","Expiration":"2030-01-01T00:00:00Z"}
EOF
`)
	creds, err := runCredentialProcess(context.Background(), script)
	if err != nil {
		t.Fatalf("runCredentialProcess: %v", err)
	}
	if creds.AccessKeyID != "AKIDPROCESS" || creds.SecretAccessKey != "secretprocess" || creds.SessionToken != "tokenprocess" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Expiry.IsZero() {
		t.Error("Expiry not parsed")
	}
}

func TestRunCredentialProcess_ArgsPassedThrough(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `
if [ "$1" != "--profile" ] || [ "$2" != "default" ]; then
  echo "bad args: $@" >&2
  exit 1
fi
echo '{"Version":1,"AccessKeyId":"AKIDARGS","SecretAccessKey":"s"}'
`)
	creds, err := runCredentialProcess(context.Background(), script+" --profile default")
	if err != nil {
		t.Fatalf("runCredentialProcess: %v", err)
	}
	if creds.AccessKeyID != "AKIDARGS" {
		t.Errorf("AccessKeyID = %q", creds.AccessKeyID)
	}
}

func TestRunCredentialProcess_NonZeroExitSurfacesStderr(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `echo "boom: bad profile" >&2
exit 1
`)
	_, err := runCredentialProcess(context.Background(), script)
	if err == nil {
		t.Fatal("want error")
	}
	var cpe *CredentialProcessError
	if !errors.As(err, &cpe) {
		t.Fatalf("err = %T, want *CredentialProcessError", err)
	}
	if !strings.Contains(cpe.Stderr, "boom: bad profile") {
		t.Errorf("Stderr = %q, want to contain the script's stderr", cpe.Stderr)
	}
}

func TestRunCredentialProcess_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `echo "not json"
`)
	_, err := runCredentialProcess(context.Background(), script)
	var cpe *CredentialProcessError
	if !errors.As(err, &cpe) {
		t.Fatalf("err = %T, want *CredentialProcessError", err)
	}
}

func TestRunCredentialProcess_MissingRequiredFields(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `echo '{"Version":1}'
`)
	_, err := runCredentialProcess(context.Background(), script)
	var cpe *CredentialProcessError
	if !errors.As(err, &cpe) {
		t.Fatalf("err = %T, want *CredentialProcessError", err)
	}
}

func TestRunCredentialProcess_CtxCancelled(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `sleep 5
echo '{"Version":1,"AccessKeyId":"x","SecretAccessKey":"y"}'
`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runCredentialProcess(ctx, script)
	if err == nil {
		t.Fatal("want error for a cancelled context")
	}
}

func TestRunCredentialProcess_StderrBounded(t *testing.T) {
	// A broken credential tool tracing itself (set -x) can echo its own
	// credentials JSON to stderr. Captured stderr is bounded at
	// maxCredentialProcessStderr, so a secret written PAST the cap can never
	// reach the error message (and thence logs); the message notes the
	// truncation instead.
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `i=0
while [ $i -lt 200 ]; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' >&2; i=$((i+1)); done
printf 'SENTINEL_LIVE_SECRET' >&2
exit 1
`)
	_, err := runCredentialProcess(context.Background(), script)
	if err == nil {
		t.Fatal("want error")
	}
	var cpe *CredentialProcessError
	if !errors.As(err, &cpe) {
		t.Fatalf("err = %T, want *CredentialProcessError", err)
	}
	if strings.Contains(err.Error(), "SENTINEL_LIVE_SECRET") {
		t.Errorf("error carried stderr content past the %d-byte cap", maxCredentialProcessStderr)
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("error %q must note the stderr truncation", err)
	}
	if len(cpe.Stderr) > maxCredentialProcessStderr+128 { // cap + marker slack
		t.Errorf("Stderr len = %d, want bounded near %d", len(cpe.Stderr), maxCredentialProcessStderr)
	}
}

func TestRunCredentialProcess_DefaultTimeoutBoundsHungTool(t *testing.T) {
	// A hung credential tool must not hang the caller forever when the
	// incoming ctx has no deadline: the process leg applies its own default
	// timeout. The default is injected shorter here so the test stays fast.
	old := credentialProcessTimeout
	credentialProcessTimeout = 300 * time.Millisecond
	t.Cleanup(func() { credentialProcessTimeout = old })

	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `sleep 30
echo '{"Version":1,"AccessKeyId":"x","SecretAccessKey":"y"}'
`)
	start := time.Now()
	_, err := runCredentialProcess(context.Background(), script)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("want error for a hung credential_process")
	}
	var cpe *CredentialProcessError
	if !errors.As(err, &cpe) {
		t.Fatalf("err = %T, want *CredentialProcessError", err)
	}
	// Bound: timeout + WaitDelay + generous scheduling slack.
	if elapsed > 5*time.Second {
		t.Errorf("runCredentialProcess took %s, want bounded near the injected 300ms timeout", elapsed)
	}
}

func TestRunCredentialProcess_CallerDeadlineWinsWhenEarlier(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `sleep 30
echo '{"Version":1,"AccessKeyId":"x","SecretAccessKey":"y"}'
`)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := runCredentialProcess(ctx, script)
	if err == nil {
		t.Fatal("want error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %s, want bounded by the caller's 300ms deadline", elapsed)
	}
}

func TestRunCredentialProcess_UnparseableExpirationIsTypedError(t *testing.T) {
	// Silently ignoring a bad Expiration would cache the credential forever
	// and reintroduce the exact silent-signature-failure-at-expiry mode this
	// package exists to kill. It must be a typed error naming the bad value
	// and the accepted format.
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `echo '{"Version":1,"AccessKeyId":"AKID","SecretAccessKey":"s","Expiration":"tomorrow-ish"}'
`)
	_, err := runCredentialProcess(context.Background(), script)
	var cpe *CredentialProcessError
	if !errors.As(err, &cpe) {
		t.Fatalf("err = %T %v, want *CredentialProcessError", err, err)
	}
	if !strings.Contains(err.Error(), "tomorrow-ish") {
		t.Errorf("error %q must name the unparseable Expiration value", err)
	}
	if !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("error %q must name the accepted format", err)
	}
}
