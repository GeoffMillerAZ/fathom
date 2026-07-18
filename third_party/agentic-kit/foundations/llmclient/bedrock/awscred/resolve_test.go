package awscred

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestResolve_ExplicitWins(t *testing.T) {
	dir := t.TempDir()
	hermeticEnv(t, dir+"/no-config", dir+"/no-credentials")

	creds, err := Resolve(context.Background(), Config{
		AccessKeyID: "AKIDEXPLICIT", SecretAccessKey: "secretexplicit", SessionToken: "tokenexplicit", Region: "us-gov-west-1",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.AccessKeyID != "AKIDEXPLICIT" || creds.SecretAccessKey != "secretexplicit" || creds.SessionToken != "tokenexplicit" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Source != SourceExplicit {
		t.Errorf("Source = %q, want explicit", creds.Source)
	}
	if creds.Region != "us-gov-west-1" {
		t.Errorf("Region = %q", creds.Region)
	}
	if !creds.Expiry.IsZero() {
		t.Errorf("Expiry = %v, want zero for explicit creds", creds.Expiry)
	}
}

func TestResolve_EnvFallback(t *testing.T) {
	dir := t.TempDir()
	hermeticEnv(t, dir+"/no-config", dir+"/no-credentials")
	t.Setenv(envAccessKeyID, "AKIDENV")
	t.Setenv(envSecretAccessKey, "secretenv")
	t.Setenv(envSessionToken, "tokenenv")
	t.Setenv(envRegion, "us-east-1")

	creds, err := Resolve(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.AccessKeyID != "AKIDENV" || creds.SecretAccessKey != "secretenv" || creds.SessionToken != "tokenenv" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Source != SourceEnv {
		t.Errorf("Source = %q, want env", creds.Source)
	}
	if creds.Region != "us-east-1" {
		t.Errorf("Region = %q", creds.Region)
	}
}

// populatedSharedFiles writes a fake ~/.aws-style config+credentials fixture
// pair carrying a default profile with static keys, and points the env at it.
// Used by the partial-pair tests to prove a partial explicit/env pair NEVER
// silently adopts these ambient credentials.
func populatedSharedFiles(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", `
[default]
aws_access_key_id = AKIDAMBIENT
aws_secret_access_key = secretambient
`)
	cfgPath := writeFile(t, dir, "config", `
[default]
region = us-east-1
`)
	hermeticEnv(t, cfgPath, credsPath)
}

func TestResolve_PartialExplicitPair_HardErrors(t *testing.T) {
	// A partial explicit pair must be a hard typed error — it must NOT be
	// completed from the environment and must NOT fall through to the shared
	// files' default profile (which would silently sign as the wrong
	// principal). Real AWS SDKs hard-error on partial static credentials too.
	t.Run("KeyWithoutSecret", func(t *testing.T) {
		populatedSharedFiles(t)
		t.Setenv(envSecretAccessKey, "secretenv") // env could complete it — must not

		_, err := Resolve(context.Background(), Config{AccessKeyID: "AKIDEXPLICIT", Region: "r"})
		var pce *PartialCredentialsError
		if !errors.As(err, &pce) {
			t.Fatalf("err = %T %v, want *PartialCredentialsError", err, err)
		}
		if pce.Source != SourceExplicit {
			t.Errorf("Source = %q, want explicit", pce.Source)
		}
		if !strings.Contains(err.Error(), "secret access key") {
			t.Errorf("error %q must name the missing secret access key", err)
		}
		if strings.Contains(err.Error(), "AKIDAMBIENT") || strings.Contains(err.Error(), "secretambient") {
			t.Errorf("error %q must not carry ambient credentials", err)
		}
	})
	t.Run("SecretWithoutKey", func(t *testing.T) {
		populatedSharedFiles(t)
		t.Setenv(envAccessKeyID, "AKIDENV") // env could complete it — must not

		_, err := Resolve(context.Background(), Config{SecretAccessKey: "secretexplicit", Region: "r"})
		var pce *PartialCredentialsError
		if !errors.As(err, &pce) {
			t.Fatalf("err = %T %v, want *PartialCredentialsError", err, err)
		}
		if !strings.Contains(err.Error(), "access key") {
			t.Errorf("error %q must name the missing access key", err)
		}
	})
}

func TestResolve_PartialEnvPair_HardErrors(t *testing.T) {
	// Same misattribution hazard one leg down: one env var without the other
	// must hard-error, never fall through to the shared files. (This also
	// matches the pre-awscred adapter, which errored on a lone env var.)
	populatedSharedFiles(t)
	t.Setenv(envAccessKeyID, "AKIDENV")

	_, err := Resolve(context.Background(), Config{Region: "r"})
	var pce *PartialCredentialsError
	if !errors.As(err, &pce) {
		t.Fatalf("err = %T %v, want *PartialCredentialsError", err, err)
	}
	if pce.Source != SourceEnv {
		t.Errorf("Source = %q, want env", pce.Source)
	}
	if !strings.Contains(err.Error(), envSecretAccessKey) {
		t.Errorf("error %q must name the missing env var", err)
	}
}

// makeUnreadable chmods path to 0o000 (restored on cleanup) so reads fail
// with EACCES — a read ERROR, distinct from the IsNotExist absence the file
// legs forgive. Skipped for root, who bypasses permission checks.
func makeUnreadable(t *testing.T, path string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits do not deny reads")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { os.Chmod(path, info.Mode()) })
}

func TestResolve_ExplicitComplete_NeverTouchesFiles(t *testing.T) {
	// A complete explicit pair short-circuits BEFORE any filesystem access:
	// an unreadable (permission-denied, not merely absent) shared config
	// must not break an explicit-credential caller.
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "config", "[default]\nregion = us-east-1\n")
	credsPath := writeFile(t, dir, "credentials", "")
	hermeticEnv(t, cfgPath, credsPath)
	makeUnreadable(t, cfgPath)
	makeUnreadable(t, credsPath)

	creds, err := Resolve(context.Background(), Config{
		AccessKeyID: "AKIDEXPLICIT", SecretAccessKey: "secretexplicit", Region: "us-gov-west-1",
	})
	if err != nil {
		t.Fatalf("Resolve: %v (explicit-complete must never read the shared files)", err)
	}
	if creds.AccessKeyID != "AKIDEXPLICIT" || creds.Source != SourceExplicit {
		t.Errorf("creds = %+v", creds)
	}
}

func TestResolve_EnvComplete_NeverTouchesFiles(t *testing.T) {
	// Same guarantee for env-complete callers, exercised through the DEFAULT
	// file paths: HOME points at an unreadable directory, so ~/.aws/* reads
	// would fail with EACCES if attempted at all.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(envConfigFile, "")
	t.Setenv(envSharedCredentialsFile, "")
	t.Setenv(envProfile, "")
	t.Setenv(envIMDSDisabled, "true")
	t.Setenv(envAccessKeyID, "AKIDENV")
	t.Setenv(envSecretAccessKey, "secretenv")
	t.Setenv(envSessionToken, "")
	t.Setenv(envRegion, "us-east-1")
	t.Setenv(envDefaultRegion, "")
	makeUnreadable(t, home)

	creds, err := Resolve(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Resolve: %v (env-complete must never read the shared files)", err)
	}
	if creds.AccessKeyID != "AKIDENV" || creds.Source != SourceEnv {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Region != "us-east-1" {
		t.Errorf("Region = %q", creds.Region)
	}
}

func TestResolve_UnreadableSharedConfig_SurfacesErrorForFileLegUsers(t *testing.T) {
	// When the chain genuinely NEEDS the file legs (no explicit/env creds),
	// an unreadable config is a real configuration problem the caller must
	// hear about — only IsNotExist is forgiven, never EACCES.
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "config", "[default]\nregion = us-east-1\n")
	credsPath := writeFile(t, dir, "credentials", "")
	hermeticEnv(t, cfgPath, credsPath)
	makeUnreadable(t, cfgPath)

	_, err := Resolve(context.Background(), Config{})
	if err == nil {
		t.Fatal("want error for an unreadable shared config when the file leg is needed")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("err = %v, want a permission error (errors.Is os.ErrPermission)", err)
	}
}

func TestResolve_FallsThroughToSharedStaticProfile(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", `
[default]
aws_access_key_id = AKIDSHARED
aws_secret_access_key = secretshared
`)
	cfgPath := writeFile(t, dir, "config", `
[default]
region = ap-southeast-2
`)
	hermeticEnv(t, cfgPath, credsPath)

	creds, err := Resolve(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.AccessKeyID != "AKIDSHARED" || creds.SecretAccessKey != "secretshared" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Source != SourceSharedStatic {
		t.Errorf("Source = %q, want shared-static", creds.Source)
	}
	if creds.Region != "ap-southeast-2" {
		t.Errorf("Region = %q", creds.Region)
	}
}

func TestResolve_FallsThroughToCredentialProcess(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `echo '{"Version":1,"AccessKeyId":"AKIDPROC","SecretAccessKey":"s"}'
`)
	credsPath := writeFile(t, dir, "credentials", "")
	cfgPath := writeFile(t, dir, "config", "[default]\ncredential_process = "+script+"\n")
	hermeticEnv(t, cfgPath, credsPath)

	creds, err := Resolve(context.Background(), Config{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.AccessKeyID != "AKIDPROC" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Source != SourceCredentialProcess {
		t.Errorf("Source = %q, want credential-process", creds.Source)
	}
}

func TestResolve_FallsThroughToSSO(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", "")
	cfgPath := writeFile(t, dir, "config", `
[profile sso-user]
sso_session = my-sso
sso_account_id = 111122223333
sso_role_name = MyRole
region = us-east-1

[sso-session my-sso]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
`)
	hermeticEnv(t, cfgPath, credsPath)
	t.Setenv(envProfile, "sso-user")
	t.Setenv("HOME", home)

	writeSSOTokenCache(t, home, ssoCacheKey("my-sso"), ssoTokenCache{
		AccessToken: "cached-token",
		ExpiresAt:   fixedNow()().Add(time.Hour),
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"roleCredentials":{"accessKeyId":"AKIDVIASSO","secretAccessKey":"s","sessionToken":"t","expiration":1784304000000}}`))
	}))
	defer srv.Close()

	creds, err := Resolve(context.Background(), Config{SSOPortalBaseURL: srv.URL, Now: fixedNow()})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.AccessKeyID != "AKIDVIASSO" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Source != SourceSSO {
		t.Errorf("Source = %q, want sso", creds.Source)
	}
	if creds.Region != "us-east-1" {
		t.Errorf("Region = %q", creds.Region)
	}
}

func TestResolve_FallsThroughToIMDS(t *testing.T) {
	dir := t.TempDir()
	hermeticEnv(t, dir+"/no-config", dir+"/no-credentials")
	t.Setenv(envIMDSDisabled, "") // this test wants IMDS enabled, overriding hermeticEnv's default disable

	srv := httptest.NewServer(imdsHandler(t, "role", `{"AccessKeyId":"AKIDIMDSVIARESOLVE","SecretAccessKey":"s","Token":"t","Expiration":"2030-01-01T00:00:00Z"}`))
	defer srv.Close()

	creds, err := Resolve(context.Background(), Config{IMDSBaseURL: srv.URL})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if creds.AccessKeyID != "AKIDIMDSVIARESOLVE" {
		t.Errorf("creds = %+v", creds)
	}
	if creds.Source != SourceIMDS {
		t.Errorf("Source = %q, want imds", creds.Source)
	}
}

func TestResolve_NothingResolves_ReturnsDescriptiveError(t *testing.T) {
	dir := t.TempDir()
	hermeticEnv(t, dir+"/no-config", dir+"/no-credentials")

	_, err := Resolve(context.Background(), Config{})
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "access key") {
		t.Errorf("error %q must mention access key", err)
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Errorf("error %q must mention secret", err)
	}
}

func TestResolve_CredentialProcessFailure_PropagatesTypedError(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cred.sh", `echo "denied" >&2
exit 1
`)
	credsPath := writeFile(t, dir, "credentials", "")
	cfgPath := writeFile(t, dir, "config", "[default]\ncredential_process = "+script+"\n")
	hermeticEnv(t, cfgPath, credsPath)

	_, err := Resolve(context.Background(), Config{})
	var cpe *CredentialProcessError
	if !errors.As(err, &cpe) {
		t.Fatalf("err = %T %v, want *CredentialProcessError", err, err)
	}
}

func TestResolve_NoCredentialsButSSONamed_PropagatesExpiredSSOTokenError(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", "")
	cfgPath := writeFile(t, dir, "config", `
[default]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 1
sso_role_name = R
`)
	hermeticEnv(t, cfgPath, credsPath)
	t.Setenv("HOME", home) // no cached token written — never logged in

	_, err := Resolve(context.Background(), Config{})
	var expErr *ExpiredSSOTokenError
	if !errors.As(err, &expErr) {
		t.Fatalf("err = %T %v, want *ExpiredSSOTokenError", err, err)
	}
}
