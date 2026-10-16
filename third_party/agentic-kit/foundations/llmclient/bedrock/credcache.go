package bedrock

import (
	"context"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/bedrock/awscred"
)

// credentialExpiryMargin is how long before a temporary credential's actual
// expiry the cache proactively re-resolves, so a request signed a moment
// before expiry never rides a rejected signature to the wire.
const credentialExpiryMargin = 5 * time.Minute

// credentialCache holds the Client's active credential triple plus its
// expiry, re-resolving through resolve when the cached value is within
// credentialExpiryMargin of Expiry (or has none cached yet). Static
// credentials (Expiry zero — explicit Config, env vars, shared-file static
// keys) are never re-resolved after the first call.
//
// Concurrency: mu guards every field; get holds mu for its full duration
// (including any resolve call), so concurrent callers never observe a torn
// read of creds/expiry and a resolve in flight is never raced against a
// concurrent write. Single-flight de-duplication is deliberately not
// implemented — two goroutines racing to refresh at the same expiry boundary
// each perform a resolve, which is correct, just occasionally redundant work,
// not a correctness or safety issue.
type credentialCache struct {
	mu      sync.Mutex
	creds   credentials
	expiry  time.Time
	resolve func(ctx context.Context) (awscred.Credentials, error)
	now     func() time.Time
}

// get returns the current credentials, re-resolving first if none are cached
// yet or the cached ones are within credentialExpiryMargin of expiry.
func (c *credentialCache) get(ctx context.Context) (credentials, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.expiry.IsZero() && !c.now().Add(credentialExpiryMargin).Before(c.expiry) {
		fresh, err := c.resolve(ctx)
		if err != nil {
			return credentials{}, err
		}
		c.creds = credentials{
			accessKeyID:     fresh.AccessKeyID,
			secretAccessKey: fresh.SecretAccessKey,
			sessionToken:    fresh.SessionToken,
		}
		c.expiry = fresh.Expiry
	}
	return c.creds, nil
}

// snapshot returns the last-known credentials without triggering a
// re-resolve — used only for defense-in-depth secret redaction in error
// paths, where a best-effort recent value is sufficient (see Client.redact).
func (c *credentialCache) snapshot() credentials {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.creds
}
