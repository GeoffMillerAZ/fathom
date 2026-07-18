package awscred

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile writes contents to dir/name, creating parent dirs, and fails the
// test on error.
func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// hermeticEnv points AWS_CONFIG_FILE/AWS_SHARED_CREDENTIALS_FILE at fixture
// paths (which need not exist) and clears AWS_PROFILE, isolating the test
// from whatever the host machine happens to have under ~/.aws. Every shared/
// resolve test in this package uses this instead of relying on ambient HOME.
func hermeticEnv(t *testing.T, configPath, credsPath string) {
	t.Helper()
	t.Setenv(envConfigFile, configPath)
	t.Setenv(envSharedCredentialsFile, credsPath)
	t.Setenv(envProfile, "")
	t.Setenv(envAccessKeyID, "")
	t.Setenv(envSecretAccessKey, "")
	t.Setenv(envSessionToken, "")
	t.Setenv(envRegion, "")
	t.Setenv(envDefaultRegion, "")
	t.Setenv(envIMDSDisabled, "true")
}

func TestParseINI(t *testing.T) {
	data := []byte(`
# a comment
[default]
region = us-east-1
aws_access_key_id = AKIDDEFAULT

[profile named]
region = us-west-2
; semicolon comment
sso_session = my-sso

[sso-session my-sso]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
`)
	sections := parseINI(data)
	if got := sections["default"]["region"]; got != "us-east-1" {
		t.Errorf("default.region = %q, want us-east-1", got)
	}
	if got := sections["default"]["aws_access_key_id"]; got != "AKIDDEFAULT" {
		t.Errorf("default.aws_access_key_id = %q", got)
	}
	if got := sections["profile named"]["sso_session"]; got != "my-sso" {
		t.Errorf("profile named.sso_session = %q", got)
	}
	if got := sections["sso-session my-sso"]["sso_start_url"]; got != "https://example.awsapps.com/start" {
		t.Errorf("sso-session my-sso.sso_start_url = %q", got)
	}
}

func TestLoadSharedProfile_StaticKeysFromCredentialsFile(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", `
[default]
aws_access_key_id = AKIDSTATIC
aws_secret_access_key = secretstatic
aws_session_token = tokenstatic
`)
	cfgPath := writeFile(t, dir, "config", `
[default]
region = us-east-1
`)
	hermeticEnv(t, cfgPath, credsPath)

	sp, err := loadSharedProfile(Config{})
	if err != nil {
		t.Fatalf("loadSharedProfile: %v", err)
	}
	if sp.AccessKeyID != "AKIDSTATIC" || sp.SecretAccessKey != "secretstatic" || sp.SessionToken != "tokenstatic" {
		t.Errorf("static creds = %+v", sp)
	}
	if sp.Region != "us-east-1" {
		t.Errorf("region = %q, want us-east-1", sp.Region)
	}
	if sp.CredentialProcess != "" {
		t.Errorf("CredentialProcess = %q, want empty", sp.CredentialProcess)
	}
	if sp.SSO != nil {
		t.Errorf("SSO = %+v, want nil", sp.SSO)
	}
}

func TestLoadSharedProfile_NamedProfileUsesProfilePrefixInConfigFile(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", `
[named]
aws_access_key_id = AKIDNAMED
aws_secret_access_key = secretnamed
`)
	cfgPath := writeFile(t, dir, "config", `
[profile named]
region = eu-west-1
`)
	hermeticEnv(t, cfgPath, credsPath)
	t.Setenv(envProfile, "named")

	sp, err := loadSharedProfile(Config{})
	if err != nil {
		t.Fatalf("loadSharedProfile: %v", err)
	}
	if sp.AccessKeyID != "AKIDNAMED" {
		t.Errorf("AccessKeyID = %q", sp.AccessKeyID)
	}
	if sp.Region != "eu-west-1" {
		t.Errorf("region = %q, want eu-west-1", sp.Region)
	}
}

func TestLoadSharedProfile_CredentialProcess(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", "")
	cfgPath := writeFile(t, dir, "config", `
[default]
credential_process = /path/to/tool --profile default
`)
	hermeticEnv(t, cfgPath, credsPath)

	sp, err := loadSharedProfile(Config{})
	if err != nil {
		t.Fatalf("loadSharedProfile: %v", err)
	}
	if sp.CredentialProcess != "/path/to/tool --profile default" {
		t.Errorf("CredentialProcess = %q", sp.CredentialProcess)
	}
	if sp.AccessKeyID != "" {
		t.Errorf("AccessKeyID = %q, want empty when only credential_process is set", sp.AccessKeyID)
	}
}

func TestLoadSharedProfile_ModernSSOSession(t *testing.T) {
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

	sp, err := loadSharedProfile(Config{})
	if err != nil {
		t.Fatalf("loadSharedProfile: %v", err)
	}
	if sp.SSO == nil {
		t.Fatal("SSO = nil, want populated")
	}
	if sp.SSO.SessionName != "my-sso" {
		t.Errorf("SessionName = %q", sp.SSO.SessionName)
	}
	if sp.SSO.StartURL != "https://example.awsapps.com/start" {
		t.Errorf("StartURL = %q", sp.SSO.StartURL)
	}
	if sp.SSO.SSORegion != "us-east-1" {
		t.Errorf("SSORegion = %q", sp.SSO.SSORegion)
	}
	if sp.SSO.AccountID != "111122223333" || sp.SSO.RoleName != "MyRole" {
		t.Errorf("AccountID/RoleName = %q/%q", sp.SSO.AccountID, sp.SSO.RoleName)
	}
}

func TestLoadSharedProfile_LegacyInlineSSO(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", "")
	cfgPath := writeFile(t, dir, "config", `
[profile legacy-sso]
sso_start_url = https://legacy.awsapps.com/start
sso_region = us-west-2
sso_account_id = 444455556666
sso_role_name = LegacyRole
`)
	hermeticEnv(t, cfgPath, credsPath)
	t.Setenv(envProfile, "legacy-sso")

	sp, err := loadSharedProfile(Config{})
	if err != nil {
		t.Fatalf("loadSharedProfile: %v", err)
	}
	if sp.SSO == nil {
		t.Fatal("SSO = nil, want populated")
	}
	if sp.SSO.SessionName != "" {
		t.Errorf("SessionName = %q, want empty for legacy inline form", sp.SSO.SessionName)
	}
	if sp.SSO.StartURL != "https://legacy.awsapps.com/start" || sp.SSO.SSORegion != "us-west-2" {
		t.Errorf("StartURL/SSORegion = %q/%q", sp.SSO.StartURL, sp.SSO.SSORegion)
	}
}

func TestLoadSharedProfile_CredentialsFileWinsOverConfigFile(t *testing.T) {
	// When the SAME profile carries static keys in BOTH files, the
	// credentials file wins — the config file's copy is a fallback consulted
	// only when the credentials file supplied no access key at all.
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", `
[default]
aws_access_key_id = AKIDFROMCREDS
aws_secret_access_key = secretfromcreds
`)
	cfgPath := writeFile(t, dir, "config", `
[default]
aws_access_key_id = AKIDFROMCONFIG
aws_secret_access_key = secretfromconfig
region = us-east-1
`)
	hermeticEnv(t, cfgPath, credsPath)

	sp, err := loadSharedProfile(Config{})
	if err != nil {
		t.Fatalf("loadSharedProfile: %v", err)
	}
	if sp.AccessKeyID != "AKIDFROMCREDS" || sp.SecretAccessKey != "secretfromcreds" {
		t.Errorf("creds = %q/%q, want the credentials file's values to win", sp.AccessKeyID, sp.SecretAccessKey)
	}
	if sp.Region != "us-east-1" {
		t.Errorf("Region = %q (region still comes from the config file)", sp.Region)
	}
}

func TestLoadSharedProfile_MissingFilesAreNotAnError(t *testing.T) {
	dir := t.TempDir()
	hermeticEnv(t, filepath.Join(dir, "no-such-config"), filepath.Join(dir, "no-such-credentials"))

	sp, err := loadSharedProfile(Config{})
	if err != nil {
		t.Fatalf("loadSharedProfile: %v", err)
	}
	if sp.AccessKeyID != "" || sp.Region != "" || sp.SSO != nil || sp.CredentialProcess != "" {
		t.Errorf("expected zero-value profile for missing files, got %+v", sp)
	}
}

func TestLoadSharedProfile_DefaultProfileNameIsDefault(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeFile(t, dir, "credentials", "")
	cfgPath := writeFile(t, dir, "config", "")
	hermeticEnv(t, cfgPath, credsPath)

	sp, err := loadSharedProfile(Config{})
	if err != nil {
		t.Fatalf("loadSharedProfile: %v", err)
	}
	if sp.Name != "default" {
		t.Errorf("Name = %q, want default", sp.Name)
	}
}
