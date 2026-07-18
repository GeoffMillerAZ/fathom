package awscred

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// maxCredentialProcessStderr caps how many bytes of a credential_process's
// stderr are captured for error messages. The cap is enforced at capture
// time (boundedBuffer) — bytes past it are counted and dropped, never
// buffered — so a misbehaving tool that traces itself with `set -x` and
// echoes its own credentials JSON deep into stderr cannot leak them into a
// CredentialProcessError and thence logs, and cannot grow memory without
// bound either.
const maxCredentialProcessStderr = 4 << 10 // 4 KiB

// credentialProcessTimeout is the default bound on one credential_process
// execution when the caller's ctx has no earlier deadline (aws-sdk-go-v2's
// processcreds provider uses the same 1-minute default). A var, not a
// const, only so tests can shrink it; production code never mutates it.
var credentialProcessTimeout = 1 * time.Minute

// credentialProcessWaitDelay bounds how long cmd.Run may keep waiting for
// I/O after the process is killed on ctx expiry: without it, a grandchild
// process inheriting the stderr/stdout pipes could hold Run open past the
// kill indefinitely.
const credentialProcessWaitDelay = 2 * time.Second

// boundedBuffer is an io.Writer that keeps at most max bytes and counts the
// overflow instead of storing it. Write never returns an error (an error
// would sever the child process's pipe mid-write; dropping is the point).
type boundedBuffer struct {
	buf      bytes.Buffer
	max      int
	overflow int64
}

// Write implements io.Writer, retaining only the first max bytes.
func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if room := b.max - b.buf.Len(); room > 0 {
		if len(p) > room {
			p = p[:room]
		}
		b.buf.Write(p)
		b.overflow += int64(n - len(p))
	} else {
		b.overflow += int64(n)
	}
	return n, nil
}

// message returns the trimmed captured bytes, with a truncation marker
// appended when anything was dropped.
func (b *boundedBuffer) message() string {
	s := strings.TrimSpace(b.buf.String())
	if b.overflow > 0 {
		s += fmt.Sprintf(" [stderr truncated: %d bytes dropped past the %d-byte cap]", b.overflow, b.max)
	}
	return s
}

// splitCommandLine tokenizes a credential_process command line the way the
// AWS CLI/SDK do: whitespace-separated tokens, with single- or double-quoted
// segments preserved as one token (quotes stripped; no backslash-escape
// processing inside quotes — that already covers every credential_process
// value in the AWS documentation's own examples and this package's tests).
// No shell is invoked to run the result — see runCredentialProcess — which
// avoids shell-metacharacter surprises from a value that ultimately comes
// from a config FILE, not a trusted caller.
func splitCommandLine(s string) []string {
	var tokens []string
	var cur strings.Builder
	inSingle, inDouble, hasCur := false, false, false
	for _, r := range s {
		switch {
		case inSingle:
			if r == '\'' {
				inSingle = false
			} else {
				cur.WriteRune(r)
			}
		case inDouble:
			if r == '"' {
				inDouble = false
			} else {
				cur.WriteRune(r)
			}
		case r == '\'':
			inSingle = true
			hasCur = true
		case r == '"':
			inDouble = true
			hasCur = true
		case r == ' ' || r == '\t':
			if hasCur {
				tokens = append(tokens, cur.String())
				cur.Reset()
				hasCur = false
			}
		default:
			cur.WriteRune(r)
			hasCur = true
		}
	}
	if hasCur {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

// credentialProcessOutput is the credential_process JSON contract:
// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sourcing-external.html
type credentialProcessOutput struct {
	Version         int    `json:"Version"`
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration"`
}

// runCredentialProcess executes cmdline (argv-tokenized via
// splitCommandLine, no shell) and parses its stdout as the credential_process
// JSON contract. A non-zero exit, a malformed stdout, stdout missing
// AccessKeyId/SecretAccessKey, or an unparseable Expiration all return a
// *CredentialProcessError carrying the process's captured stderr (bounded at
// maxCredentialProcessStderr — see boundedBuffer) so the underlying tool's
// own diagnostic reaches the caller without becoming a leak vector.
//
// Execution is time-bounded even when ctx has no deadline: a default of
// credentialProcessTimeout (1 minute, aws-sdk-go-v2 processcreds parity)
// applies unless the caller's ctx already expires sooner, and cmd.WaitDelay
// guarantees Run returns shortly after the kill even if a grandchild
// process holds the output pipes open. A hung credential tool therefore
// bounds — never hangs — the resolution chain and everything above it
// (bedrock.New, per-request re-resolution).
func runCredentialProcess(ctx context.Context, cmdline string) (Credentials, error) {
	argv := splitCommandLine(cmdline)
	if len(argv) == 0 {
		return Credentials{}, &CredentialProcessError{Command: cmdline, Err: errors.New("empty credential_process command")}
	}

	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > credentialProcessTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, credentialProcessTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.WaitDelay = credentialProcessWaitDelay
	var stdout bytes.Buffer
	stderr := &boundedBuffer{max: maxCredentialProcessStderr}
	cmd.Stdout = &stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return Credentials{}, &CredentialProcessError{Command: cmdline, Err: err, Stderr: stderr.message()}
	}

	var out credentialProcessOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return Credentials{}, &CredentialProcessError{Command: cmdline, Err: err, Stderr: stderr.message()}
	}
	if out.AccessKeyId == "" || out.SecretAccessKey == "" {
		return Credentials{}, &CredentialProcessError{
			Command: cmdline,
			Err:     errors.New("stdout missing AccessKeyId/SecretAccessKey"),
			Stderr:  stderr.message(),
		}
	}

	creds := Credentials{
		AccessKeyID:     out.AccessKeyId,
		SecretAccessKey: out.SecretAccessKey,
		SessionToken:    out.SessionToken,
	}
	if out.Expiration != "" {
		t, err := time.Parse(time.RFC3339, out.Expiration)
		if err != nil {
			// Silently treating an unparseable Expiration as "static" would
			// cache the credential forever and reintroduce the exact
			// silent-signature-failure-at-expiry mode this package exists
			// to kill — fail loud instead. RFC3339 is the accepted format
			// (the ISO8601 profile the credential_process contract's own
			// examples use); a tool emitting anything else must be fixed,
			// not guessed at.
			return Credentials{}, &CredentialProcessError{
				Command: cmdline,
				Err:     fmt.Errorf("Expiration %q is not RFC3339 (e.g. 2030-01-01T00:00:00Z): %w", out.Expiration, err),
				Stderr:  stderr.message(),
			}
		}
		creds.Expiry = t
	}
	return creds, nil
}
