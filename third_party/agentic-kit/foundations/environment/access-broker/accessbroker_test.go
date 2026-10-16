package accessbroker_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	accessbroker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/environment/access-broker"
)

// ── CredentialScope.Validate ──────────────────────────────────────────────────

func TestCredentialScope_Validate_valid(t *testing.T) {
	cases := []accessbroker.CredentialScope{
		{Resource: "db/analytics", Level: accessbroker.LevelRead},
		{Resource: "bucket/reports", Level: accessbroker.LevelWrite},
		{Resource: "cluster/control-plane", Level: accessbroker.LevelAdmin},
	}
	for _, s := range cases {
		if err := s.Validate(); err != nil {
			t.Errorf("Validate(%+v): unexpected error: %v", s, err)
		}
	}
}

func TestCredentialScope_Validate_allLevels(t *testing.T) {
	levels := []accessbroker.Level{
		accessbroker.LevelRead,
		accessbroker.LevelWrite,
		accessbroker.LevelAdmin,
	}
	for _, l := range levels {
		t.Run(string(l), func(t *testing.T) {
			s := accessbroker.CredentialScope{Resource: "res/x", Level: l}
			if err := s.Validate(); err != nil {
				t.Fatalf("unexpected error for level %q: %v", l, err)
			}
		})
	}
}

func TestCredentialScope_Validate_invalid(t *testing.T) {
	cases := []struct {
		name    string
		scope   accessbroker.CredentialScope
		wantErr string
	}{
		{
			"empty Resource",
			accessbroker.CredentialScope{Resource: "", Level: accessbroker.LevelRead},
			"Resource must be non-empty",
		},
		{
			"whitespace Resource",
			accessbroker.CredentialScope{Resource: "  ", Level: accessbroker.LevelRead},
			"Resource must be non-empty",
		},
		{
			"unknown Level",
			accessbroker.CredentialScope{Resource: "db/x", Level: "superuser"},
			"not valid",
		},
		{
			"empty Level",
			accessbroker.CredentialScope{Resource: "db/x", Level: ""},
			"not valid",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.scope.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ── NewBroker ─────────────────────────────────────────────────────────────────

func TestNewBroker_nilPolicy(t *testing.T) {
	_, err := accessbroker.NewBroker(nil)
	if err == nil {
		t.Fatal("expected error for nil policy")
	}
}

func TestNewBroker_valid(t *testing.T) {
	policy := func(_ accessbroker.CredentialScope, _ string) (string, time.Time, error) {
		return "tok", time.Time{}, nil
	}
	b, err := accessbroker.NewBroker(policy)
	if err != nil {
		t.Fatalf("NewBroker: unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("NewBroker: returned nil Broker")
	}
}

// ── Broker.Grant ──────────────────────────────────────────────────────────────

func makeSimplePolicy(token string) accessbroker.PolicyFunc {
	return func(_ accessbroker.CredentialScope, _ string) (string, time.Time, error) {
		return token, time.Time{}, nil
	}
}

func TestGrant_valid(t *testing.T) {
	b, _ := accessbroker.NewBroker(makeSimplePolicy("my-token"))

	req := accessbroker.GrantRequest{
		Scope:    accessbroker.CredentialScope{Resource: "db/analytics", Level: accessbroker.LevelRead},
		CallerID: "agent/recon-01",
	}
	grant, err := b.Grant(req)
	if err != nil {
		t.Fatalf("Grant: unexpected error: %v", err)
	}
	if grant.Token != "my-token" {
		t.Errorf("Token: got %q, want %q", grant.Token, "my-token")
	}
	if grant.Scope != req.Scope {
		t.Errorf("Scope mismatch: got %+v, want %+v", grant.Scope, req.Scope)
	}
}

func TestGrant_invalidScope(t *testing.T) {
	b, _ := accessbroker.NewBroker(makeSimplePolicy("tok"))

	req := accessbroker.GrantRequest{
		Scope:    accessbroker.CredentialScope{Resource: "", Level: accessbroker.LevelRead},
		CallerID: "agent/01",
	}
	_, err := b.Grant(req)
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
	if !strings.Contains(err.Error(), "invalid scope") {
		t.Errorf("error %q does not mention invalid scope", err.Error())
	}
}

func TestGrant_emptyCallerID(t *testing.T) {
	b, _ := accessbroker.NewBroker(makeSimplePolicy("tok"))

	req := accessbroker.GrantRequest{
		Scope:    accessbroker.CredentialScope{Resource: "db/x", Level: accessbroker.LevelRead},
		CallerID: "",
	}
	_, err := b.Grant(req)
	if err == nil {
		t.Fatal("expected error for empty CallerID")
	}
	if !strings.Contains(err.Error(), "CallerID") {
		t.Errorf("error %q does not mention CallerID", err.Error())
	}
}

func TestGrant_policyError(t *testing.T) {
	policyErr := errors.New("credential backend unavailable")
	failPolicy := func(_ accessbroker.CredentialScope, _ string) (string, time.Time, error) {
		return "", time.Time{}, policyErr
	}
	b, _ := accessbroker.NewBroker(failPolicy)

	req := accessbroker.GrantRequest{
		Scope:    accessbroker.CredentialScope{Resource: "db/x", Level: accessbroker.LevelRead},
		CallerID: "agent/01",
	}
	_, err := b.Grant(req)
	if err == nil {
		t.Fatal("expected error from policy function")
	}
	if !strings.Contains(err.Error(), "policy error") {
		t.Errorf("error %q does not mention policy error", err.Error())
	}
	if !errors.Is(err, policyErr) {
		t.Errorf("expected wrapped policyErr, got: %v", err)
	}
}

func TestGrant_withExpiry(t *testing.T) {
	expiry := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expiryPolicy := func(_ accessbroker.CredentialScope, _ string) (string, time.Time, error) {
		return "exp-tok", expiry, nil
	}
	b, _ := accessbroker.NewBroker(expiryPolicy)

	req := accessbroker.GrantRequest{
		Scope:    accessbroker.CredentialScope{Resource: "bucket/reports", Level: accessbroker.LevelWrite},
		CallerID: "agent/writer-01",
	}
	grant, err := b.Grant(req)
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if !grant.ExpiresAt.Equal(expiry) {
		t.Errorf("ExpiresAt: got %v, want %v", grant.ExpiresAt, expiry)
	}
}

func TestGrant_policyReceivesCorrectArgs(t *testing.T) {
	var capturedScope accessbroker.CredentialScope
	var capturedCallerID string

	capturePolicy := func(scope accessbroker.CredentialScope, callerID string) (string, time.Time, error) {
		capturedScope = scope
		capturedCallerID = callerID
		return "tok", time.Time{}, nil
	}
	b, _ := accessbroker.NewBroker(capturePolicy)

	wantScope := accessbroker.CredentialScope{Resource: "cluster/cp", Level: accessbroker.LevelAdmin}
	wantCaller := "agent/admin-01"

	req := accessbroker.GrantRequest{Scope: wantScope, CallerID: wantCaller}
	if _, err := b.Grant(req); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if capturedScope != wantScope {
		t.Errorf("policy got scope %+v, want %+v", capturedScope, wantScope)
	}
	if capturedCallerID != wantCaller {
		t.Errorf("policy got callerID %q, want %q", capturedCallerID, wantCaller)
	}
}
