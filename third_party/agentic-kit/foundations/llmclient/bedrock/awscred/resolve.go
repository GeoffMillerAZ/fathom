package awscred

import (
	"context"
	"fmt"
	"os"
)

// Resolve walks the AWS default credential chain — explicit Config fields,
// then the AWS_* environment variables, then the shared config/credentials
// files (static keys, credential_process, or SSO), then IMDSv2 last — and
// returns the first Credentials it finds, source-labeled, with Region
// resolved via resolveRegion and Expiry set for any leg that produces
// temporary credentials.
//
// The explicit and environment legs are each ATOMIC and SHORT-CIRCUITING:
//
//   - A complete pair on a leg (AccessKeyID + SecretAccessKey both set, plus
//     that same leg's SessionToken) resolves immediately, BEFORE any ambient
//     state is consulted — no file read, no credential_process execution, no
//     SSO cache access, no IMDS call. A caller who names its credentials
//     gets exactly those credentials, and an unreadable ~/.aws (permission
//     denied, not merely absent) can never break an explicit- or
//     env-credentialed caller. Region for a short-circuited leg resolves
//     from Config.Region / AWS_REGION / AWS_DEFAULT_REGION only (the shared
//     profile's region key is a file read, so it is off-limits here).
//
//   - A PARTIAL pair on a leg (exactly one of the two halves set) is a hard
//     *PartialCredentialsError — never completed from the next leg, never
//     fallen through past. Falling through would silently sign requests as
//     whatever identity the ambient state names (e.g. the ~/.aws default
//     profile) instead of the principal the caller half-named —
//     request-level misattribution, the worst failure mode a credential
//     chain has. The AWS SDKs hard-error on partial static and partial env
//     credentials for the same reason. (This also matches the pre-awscred
//     bedrock adapter, which errored on an incomplete pair rather than
//     resolving anything else.)
//
// Only when BOTH those legs are empty does Resolve touch the filesystem:
// the shared config/credentials files (static keys, then
// credential_process, then SSO), then IMDSv2. On these legs a file that
// exists but cannot be read is a hard error (only IsNotExist is forgiven —
// see readINIFile): a caller that genuinely depends on the file legs should
// hear about an unreadable config, not silently skip to IMDS.
//
// A file-leg profile that names credentials but cannot produce them (an
// expired SSO token, a failing credential_process) returns that leg's typed
// error immediately rather than silently falling through — a profile that
// names SSO with an expired token is a configuration the caller should fix,
// not a "try IMDS instead" situation. Only IMDS itself treats
// unreachability as "found nothing" (see resolveIMDS), because that is the
// expected non-error outcome for every non-EC2 host.
//
// Resolve returns an error only when every leg is exhausted with nothing
// usable, or when a chosen leg failed outright; it never returns a partial
// Credentials.
func Resolve(ctx context.Context, cfg Config) (Credentials, error) {
	now := cfg.clock()

	// Leg 1: explicit Config fields — atomic, ambient-blind.
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		if cfg.SecretAccessKey == "" {
			return Credentials{}, &PartialCredentialsError{
				Source:  SourceExplicit,
				Present: "access key (Config.AccessKeyID)",
				Missing: "secret access key (Config.SecretAccessKey)",
			}
		}
		if cfg.AccessKeyID == "" {
			return Credentials{}, &PartialCredentialsError{
				Source:  SourceExplicit,
				Present: "secret access key (Config.SecretAccessKey)",
				Missing: "access key (Config.AccessKeyID)",
			}
		}
		return Credentials{
			AccessKeyID:     cfg.AccessKeyID,
			SecretAccessKey: cfg.SecretAccessKey,
			SessionToken:    cfg.SessionToken,
			Source:          SourceExplicit,
			Region:          resolveRegion(cfg, ""),
		}, nil
	}

	// Leg 2: environment variables — likewise atomic, no file access.
	envKey, envSecret := os.Getenv(envAccessKeyID), os.Getenv(envSecretAccessKey)
	if envKey != "" || envSecret != "" {
		if envSecret == "" {
			return Credentials{}, &PartialCredentialsError{
				Source:  SourceEnv,
				Present: "access key (" + envAccessKeyID + ")",
				Missing: "secret access key (" + envSecretAccessKey + ")",
			}
		}
		if envKey == "" {
			return Credentials{}, &PartialCredentialsError{
				Source:  SourceEnv,
				Present: "secret access key (" + envSecretAccessKey + ")",
				Missing: "access key (" + envAccessKeyID + ")",
			}
		}
		return Credentials{
			AccessKeyID:     envKey,
			SecretAccessKey: envSecret,
			SessionToken:    os.Getenv(envSessionToken),
			Source:          SourceEnv,
			Region:          resolveRegion(cfg, ""),
		}, nil
	}

	// Legs 3–4: only now is any ambient file/process/network state touched.
	shared, err := loadSharedProfile(cfg)
	if err != nil {
		return Credentials{}, err
	}
	region := resolveRegion(cfg, shared.Region)

	if shared.AccessKeyID != "" && shared.SecretAccessKey != "" {
		return Credentials{
			AccessKeyID: shared.AccessKeyID, SecretAccessKey: shared.SecretAccessKey, SessionToken: shared.SessionToken,
			Source: SourceSharedStatic, Region: region,
		}, nil
	}

	if shared.CredentialProcess != "" {
		creds, err := runCredentialProcess(ctx, shared.CredentialProcess)
		if err != nil {
			return Credentials{}, err
		}
		creds.Source = SourceCredentialProcess
		creds.Region = region
		return creds, nil
	}

	if shared.SSO != nil {
		creds, err := resolveSSO(ctx, cfg, shared.Name, *shared.SSO, now)
		if err != nil {
			return Credentials{}, err
		}
		creds.Region = region
		return creds, nil
	}

	if !imdsDisabled() {
		creds, ok, err := resolveIMDS(ctx, cfg, now)
		if err != nil {
			return Credentials{}, err
		}
		if ok {
			creds.Region = region
			return creds, nil
		}
	}

	return Credentials{}, fmt.Errorf(
		"awscred: no credentials resolved: no explicit access key/secret, no %s/%s environment variables, no usable profile %q (static keys, credential_process, or sso) in the shared config/credentials files, and IMDS unavailable — configure credentials, e.g. `aws configure` or `aws sso login --profile %s`",
		envAccessKeyID, envSecretAccessKey, shared.Name, shared.Name,
	)
}
