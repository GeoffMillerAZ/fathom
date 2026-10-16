package bedrock

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/bedrock/awscred"
)

// TestCredentialCache_StaticNeverReResolves proves a zero-Expiry (static)
// credential is cached forever: resolve is called exactly once even across
// many get calls and simulated elapsed time.
func TestCredentialCache_StaticNeverReResolves(t *testing.T) {
	var calls int32
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := fixed
	cache := &credentialCache{
		now: func() time.Time { return now },
		resolve: func(ctx context.Context) (awscred.Credentials, error) {
			atomic.AddInt32(&calls, 1)
			return awscred.Credentials{AccessKeyID: "should-not-be-used"}, nil
		},
		creds:  credentials{accessKeyID: "static-key", secretAccessKey: "static-secret"},
		expiry: time.Time{}, // static
	}

	for i := 0; i < 5; i++ {
		now = now.Add(24 * time.Hour)
		creds, err := cache.get(context.Background())
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if creds.accessKeyID != "static-key" {
			t.Errorf("iter %d: accessKeyID = %q, want static-key", i, creds.accessKeyID)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Errorf("resolve called %d times, want 0 for a static credential", got)
	}
}

// TestCredentialCache_ReResolvesWithinExpiryMargin proves a credential
// nearing its Expiry within credentialExpiryMargin triggers a re-resolve,
// and one comfortably outside the margin does not.
func TestCredentialCache_ReResolvesWithinExpiryMargin(t *testing.T) {
	expiry := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var calls int32
	var now time.Time
	cache := &credentialCache{
		now: func() time.Time { return now },
		resolve: func(ctx context.Context) (awscred.Credentials, error) {
			atomic.AddInt32(&calls, 1)
			return awscred.Credentials{
				AccessKeyID: "refreshed-key", SecretAccessKey: "refreshed-secret",
				Expiry: expiry.Add(1 * time.Hour),
			}, nil
		},
		creds:  credentials{accessKeyID: "original-key", secretAccessKey: "original-secret"},
		expiry: expiry,
	}

	// Comfortably outside the 5-minute margin: no re-resolve.
	now = expiry.Add(-1 * time.Hour)
	creds, err := cache.get(context.Background())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if creds.accessKeyID != "original-key" {
		t.Errorf("accessKeyID = %q, want original-key (outside margin)", creds.accessKeyID)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("resolve called %d times, want 0 outside the margin", got)
	}

	// Inside the margin: re-resolve fires, returning the fresh credential.
	now = expiry.Add(-4 * time.Minute)
	creds, err = cache.get(context.Background())
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if creds.accessKeyID != "refreshed-key" {
		t.Errorf("accessKeyID = %q, want refreshed-key (inside margin)", creds.accessKeyID)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("resolve called %d times, want 1", got)
	}

	// A second get before the NEW expiry's margin: no further re-resolve.
	now = expiry.Add(-3 * time.Minute)
	if _, err := cache.get(context.Background()); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("resolve called %d times after second get, want still 1 (fresh expiry not yet in margin)", got)
	}
}

// TestCredentialCache_ResolveErrorPropagates proves a resolve failure
// surfaces to the caller rather than silently reusing stale creds.
func TestCredentialCache_ResolveErrorPropagates(t *testing.T) {
	wantErr := errors.New("sso token expired")
	expiry := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cache := &credentialCache{
		now:     func() time.Time { return expiry },
		resolve: func(ctx context.Context) (awscred.Credentials, error) { return awscred.Credentials{}, wantErr },
		creds:   credentials{accessKeyID: "stale-key"},
		expiry:  expiry,
	}
	_, err := cache.get(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestCredentialCache_ConcurrentGetsRaceClean proves concurrent get calls
// under -race never see a torn read, even while re-resolves are actively
// happening.
func TestCredentialCache_ConcurrentGetsRaceClean(t *testing.T) {
	expiry := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cache := &credentialCache{
		now: func() time.Time { return expiry }, // always exactly at expiry: every get re-resolves
		resolve: func(ctx context.Context) (awscred.Credentials, error) {
			return awscred.Credentials{AccessKeyID: "k", SecretAccessKey: "s", Expiry: expiry.Add(1 * time.Minute)}, nil
		},
		creds:  credentials{accessKeyID: "k", secretAccessKey: "s"},
		expiry: expiry,
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.get(context.Background()); err != nil {
				t.Errorf("get: %v", err)
			}
		}()
	}
	wg.Wait()
}
