package awscred

import (
	"os"
	"path/filepath"
)

// ssoProfile is the SSO configuration named by a shared-file profile, in
// either its modern (SessionName non-empty) or legacy inline (SessionName
// empty) form. See loadSharedProfile and resolveSSO.
type ssoProfile struct {
	// SessionName is the profile's sso_session value (modern form). Empty
	// for the legacy inline form.
	SessionName string

	// StartURL is the SSO start URL: read from the referenced
	// [sso-session <name>] section for the modern form, or inline
	// sso_start_url for the legacy form.
	StartURL string

	// SSORegion is the SSO portal's region, resolved the same way as
	// StartURL.
	SSORegion string

	// AccountID is the profile's sso_account_id.
	AccountID string

	// RoleName is the profile's sso_role_name.
	RoleName string
}

// sharedProfile is everything loadSharedProfile discovers about the active
// profile after parsing the shared config and credentials files. Resolve
// interprets these fields in AWS default-chain precedence: static keys, then
// CredentialProcess, then SSO.
type sharedProfile struct {
	// Name is the resolved profile name (AWS_PROFILE, else "default").
	Name string

	// Region is the profile's "region" key, or "" if absent.
	Region string

	// AccessKeyID, SecretAccessKey, SessionToken are static keys found in
	// the profile section. All empty when the profile has none.
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string

	// CredentialProcess is the profile's raw credential_process command
	// line, or "" if absent.
	CredentialProcess string

	// SSO is non-nil when the profile names SSO (either form).
	SSO *ssoProfile
}

// loadSharedProfile resolves the active profile name (AWS_PROFILE, else
// "default") and parses the shared config file (AWS_CONFIG_FILE, else
// ~/.aws/config) and credentials file (AWS_SHARED_CREDENTIALS_FILE, else
// ~/.aws/credentials). A missing file at either path is not an error.
//
// loadSharedProfile is pure parsing: it never executes credential_process,
// never reads the SSO token cache, and never touches the network. Resolve
// performs those only for the leg it actually needs, once this function has
// told it which one that is.
func loadSharedProfile(cfg Config) (sharedProfile, error) {
	profile := os.Getenv(envProfile)
	if profile == "" {
		profile = "default"
	}
	sp := sharedProfile{Name: profile}

	credPath := firstNonEmpty(os.Getenv(envSharedCredentialsFile), defaultCredentialsPath())
	cfgPath := firstNonEmpty(os.Getenv(envConfigFile), defaultConfigPath())

	credSections, err := readINIFile(credPath)
	if err != nil {
		return sp, err
	}
	cfgSections, err := readINIFile(cfgPath)
	if err != nil {
		return sp, err
	}

	// Credentials file: bare profile name, no "profile " prefix, ever.
	if sec, ok := credSections[profile]; ok {
		sp.AccessKeyID = sec["aws_access_key_id"]
		sp.SecretAccessKey = sec["aws_secret_access_key"]
		sp.SessionToken = sec["aws_session_token"]
		sp.CredentialProcess = sec["credential_process"]
	}

	// Config file: "default" bare, everything else "profile <name>". The
	// config file can carry region, credential_process, and SSO fields; it
	// can also (rarely, but per the AWS CLI's own tolerance) carry static
	// keys, only used here when the credentials file didn't already supply
	// them.
	if sec, ok := cfgSections[configProfileSection(profile)]; ok {
		sp.Region = sec["region"]
		if sp.AccessKeyID == "" && sec["aws_access_key_id"] != "" {
			sp.AccessKeyID = sec["aws_access_key_id"]
			sp.SecretAccessKey = sec["aws_secret_access_key"]
			sp.SessionToken = sec["aws_session_token"]
		}
		if sp.CredentialProcess == "" {
			sp.CredentialProcess = sec["credential_process"]
		}

		if sessionName := sec["sso_session"]; sessionName != "" {
			sso := &ssoProfile{
				SessionName: sessionName,
				AccountID:   sec["sso_account_id"],
				RoleName:    sec["sso_role_name"],
			}
			if ssoSec, ok := cfgSections["sso-session "+sessionName]; ok {
				sso.StartURL = ssoSec["sso_start_url"]
				sso.SSORegion = ssoSec["sso_region"]
			}
			sp.SSO = sso
		} else if sec["sso_start_url"] != "" {
			sp.SSO = &ssoProfile{
				StartURL:  sec["sso_start_url"],
				SSORegion: sec["sso_region"],
				AccountID: sec["sso_account_id"],
				RoleName:  sec["sso_role_name"],
			}
		}
	}

	return sp, nil
}

// defaultConfigPath returns ~/.aws/config, or "" if the home directory
// cannot be resolved (readINIFile then correctly treats "" as absent).
func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aws", "config")
}

// defaultCredentialsPath returns ~/.aws/credentials, or "" if the home
// directory cannot be resolved.
func defaultCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aws", "credentials")
}
