package awscred

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// imdsDefaultBaseURL is the well-known, non-routable IMDS origin.
	imdsDefaultBaseURL = "http://169.254.169.254"

	// imdsTimeout bounds every individual IMDS call. Off an EC2 instance this
	// leg is expected to fail fast, not hang the whole resolution chain.
	imdsTimeout = 1 * time.Second

	// imdsTokenTTLHeader and imdsTokenTTLSeconds request an IMDSv2 session
	// token good for the lifetime of one Resolve call — this package never
	// caches the token across calls.
	imdsTokenTTLHeader  = "X-aws-ec2-metadata-token-ttl-seconds"
	imdsTokenTTLSeconds = "21600"

	// imdsTokenHeader carries the session token on the two follow-up GETs.
	imdsTokenHeader = "X-aws-ec2-metadata-token"

	// imdsMaxBodyBytes caps every IMDS response read.
	imdsMaxBodyBytes = 1 << 16
)

// imdsDisabled reports whether AWS_EC2_METADATA_DISABLED is set to "true"
// (case-insensitive, matching the AWS CLI/SDK convention). Resolve checks
// this before calling resolveIMDS at all, so resolveIMDS's own tests can
// exercise the real request path without env-var interference.
func imdsDisabled() bool {
	return strings.EqualFold(os.Getenv(envIMDSDisabled), "true")
}

// imdsRoleCredentials is the JSON shape of the IMDSv2
// security-credentials/<role> response.
type imdsRoleCredentials struct {
	Code            string    `json:"Code"`
	AccessKeyId     string    `json:"AccessKeyId"`
	SecretAccessKey string    `json:"SecretAccessKey"`
	Token           string    `json:"Token"`
	Expiration      time.Time `json:"Expiration"`
}

// resolveIMDS resolves credentials from the EC2 Instance Metadata Service
// using IMDSv2 only (the token-based flow; IMDSv1's tokenless GETs are never
// attempted, matching modern AWS SDK default behavior): a token PUT, a
// role-name GET, then a role-credentials GET, each bounded by imdsTimeout.
//
// ok is false with a nil error whenever IMDS could not be reached or did not
// answer with a usable 200 within the timeout — that is the expected,
// non-error outcome off EC2, and Resolve treats it as "this leg found
// nothing," not a hard failure. A non-nil error is reserved for a reachable
// IMDS that returned a 200 with an unparseable body — a genuine surprise
// worth surfacing rather than silently swallowing.
func resolveIMDS(ctx context.Context, cfg Config, now func() time.Time) (Credentials, bool, error) {
	base := firstNonEmpty(cfg.IMDSBaseURL, imdsDefaultBaseURL)
	client := &http.Client{Timeout: imdsTimeout}

	token, ok := imdsToken(ctx, client, base)
	if !ok {
		return Credentials{}, false, nil
	}

	role, ok := imdsGet(ctx, client, base+"/latest/meta-data/iam/security-credentials/", token)
	if !ok {
		return Credentials{}, false, nil
	}
	role = strings.TrimSpace(strings.SplitN(role, "\n", 2)[0])
	if role == "" {
		return Credentials{}, false, nil
	}

	credBody, ok := imdsGet(ctx, client, base+"/latest/meta-data/iam/security-credentials/"+url.PathEscape(role), token)
	if !ok {
		return Credentials{}, false, nil
	}

	var out imdsRoleCredentials
	if err := json.Unmarshal([]byte(credBody), &out); err != nil {
		return Credentials{}, false, fmt.Errorf("awscred: parsing IMDS role-credentials response: %w", err)
	}
	if out.AccessKeyId == "" || out.SecretAccessKey == "" {
		return Credentials{}, false, nil
	}

	return Credentials{
		AccessKeyID:     out.AccessKeyId,
		SecretAccessKey: out.SecretAccessKey,
		SessionToken:    out.Token,
		Source:          SourceIMDS,
		Expiry:          out.Expiration,
	}, true, nil
}

// imdsToken performs the token PUT and returns (token, true) on a 200,
// or ("", false) for any failure (unreachable, timeout, non-200) — all
// treated identically as "IMDS not available."
func imdsToken(ctx context.Context, client *http.Client, base string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, base+"/latest/api/token", nil)
	if err != nil {
		return "", false
	}
	req.Header.Set(imdsTokenTTLHeader, imdsTokenTTLSeconds)
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, imdsMaxBodyBytes))
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	return strings.TrimSpace(string(body)), true
}

// imdsGet performs one token-gated IMDS GET and returns (body, true) on a
// 200, or ("", false) otherwise.
func imdsGet(ctx context.Context, client *http.Client, url, token string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set(imdsTokenHeader, token)
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, imdsMaxBodyBytes))
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	return string(body), true
}
