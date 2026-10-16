package tokenlifecycle_test

import (
	"reflect"
	"testing"
	"time"

	tokenlifecycle "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/token-lifecycle"
)

// now is the fixed observation instant every table case is evaluated at —
// clocks are inputs here, never time.Now().
var now = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		cred tokenlifecycle.Credential
		want tokenlifecycle.State
	}{
		{
			name: "zero credential is not_found",
			cred: tokenlifecycle.Credential{},
			want: tokenlifecycle.StateNotFound,
		},
		{
			name: "unexpired access token is valid",
			cred: tokenlifecycle.Credential{
				AccessToken: "at",
				ExpiresAt:   now.Add(time.Hour),
			},
			want: tokenlifecycle.StateValid,
		},
		{
			name: "no skew in classification: expiry 1s away is still valid",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(time.Second),
			},
			want: tokenlifecycle.StateValid,
		},
		{
			name: "expiry exactly now is expired (validity is strict Before)",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now,
			},
			want: tokenlifecycle.StateRefreshable,
		},
		{
			name: "expiry one nanosecond ahead is valid",
			cred: tokenlifecycle.Credential{
				AccessToken: "at",
				ExpiresAt:   now.Add(time.Nanosecond),
			},
			want: tokenlifecycle.StateValid,
		},
		{
			name: "zero ExpiresAt means expired: refresh token present -> refreshable",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
			},
			want: tokenlifecycle.StateRefreshable,
		},
		{
			name: "zero ExpiresAt means expired: no refresh token -> reauth_needed",
			cred: tokenlifecycle.Credential{
				AccessToken: "at",
			},
			want: tokenlifecycle.StateReauthNeeded,
		},
		{
			name: "expired with refresh token is refreshable",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(-time.Hour),
			},
			want: tokenlifecycle.StateRefreshable,
		},
		{
			name: "expired without refresh token is reauth_needed",
			cred: tokenlifecycle.Credential{
				AccessToken: "at",
				ExpiresAt:   now.Add(-time.Hour),
			},
			want: tokenlifecycle.StateReauthNeeded,
		},
		{
			name: "refresh-token-only credential is refreshable, not not_found",
			cred: tokenlifecycle.Credential{
				RefreshToken: "rt",
			},
			want: tokenlifecycle.StateRefreshable,
		},
		{
			name: "scopes-only credential is not the zero credential -> reauth_needed",
			cred: tokenlifecycle.Credential{
				Scopes: []string{"mail.read"},
			},
			want: tokenlifecycle.StateReauthNeeded,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenlifecycle.Classify(tt.cred, now); got != tt.want {
				t.Errorf("Classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNextAction(t *testing.T) {
	tests := []struct {
		name   string
		cred   tokenlifecycle.Credential
		policy tokenlifecycle.Policy
		want   tokenlifecycle.Action
	}{
		{
			name: "well within expiry -> use",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(time.Hour),
			},
			policy: tokenlifecycle.DefaultPolicy(),
			want:   tokenlifecycle.ActionUse,
		},
		{
			name: "exact skew boundary (now == ExpiresAt-60s) -> refresh",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(60 * time.Second),
			},
			policy: tokenlifecycle.DefaultPolicy(),
			want:   tokenlifecycle.ActionRefresh,
		},
		{
			name: "one nanosecond outside the skew window -> use",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(60*time.Second + time.Nanosecond),
			},
			policy: tokenlifecycle.DefaultPolicy(),
			want:   tokenlifecycle.ActionUse,
		},
		{
			name: "inside the skew window -> refresh",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(30 * time.Second),
			},
			policy: tokenlifecycle.DefaultPolicy(),
			want:   tokenlifecycle.ActionRefresh,
		},
		{
			name: "zero-value policy defaults to the documented 60s skew",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(60 * time.Second),
			},
			policy: tokenlifecycle.Policy{}, // zero RefreshSkew -> DefaultRefreshSkew
			want:   tokenlifecycle.ActionRefresh,
		},
		{
			name: "zero-value policy: outside default skew -> use",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(61 * time.Second),
			},
			policy: tokenlifecycle.Policy{},
			want:   tokenlifecycle.ActionUse,
		},
		{
			name: "custom skew widens the window: 5m skew, 2m to expiry -> refresh",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(2 * time.Minute),
			},
			policy: tokenlifecycle.Policy{RefreshSkew: 5 * time.Minute},
			want:   tokenlifecycle.ActionRefresh,
		},
		{
			name: "custom skew exact boundary (now == ExpiresAt-5m) -> refresh",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(5 * time.Minute),
			},
			policy: tokenlifecycle.Policy{RefreshSkew: 5 * time.Minute},
			want:   tokenlifecycle.ActionRefresh,
		},
		{
			name: "custom skew one nanosecond outside boundary -> use",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(5*time.Minute + time.Nanosecond),
			},
			policy: tokenlifecycle.Policy{RefreshSkew: 5 * time.Minute},
			want:   tokenlifecycle.ActionUse,
		},
		{
			name: "negative skew disables the window: 1ns to expiry -> use",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now.Add(time.Nanosecond),
			},
			policy: tokenlifecycle.Policy{RefreshSkew: -1},
			want:   tokenlifecycle.ActionUse,
		},
		{
			name: "negative skew: expiry exactly now -> refresh",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
				ExpiresAt:    now,
			},
			policy: tokenlifecycle.Policy{RefreshSkew: -1},
			want:   tokenlifecycle.ActionRefresh,
		},
		{
			name: "zero ExpiresAt means unknown/expired: refresh token present -> refresh",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
			},
			policy: tokenlifecycle.DefaultPolicy(),
			want:   tokenlifecycle.ActionRefresh,
		},
		{
			name: "zero ExpiresAt and no refresh token -> reauth",
			cred: tokenlifecycle.Credential{
				AccessToken: "at",
			},
			policy: tokenlifecycle.DefaultPolicy(),
			want:   tokenlifecycle.ActionReauth,
		},
		{
			name: "expired and no refresh token -> reauth",
			cred: tokenlifecycle.Credential{
				AccessToken: "at",
				ExpiresAt:   now.Add(-time.Hour),
			},
			policy: tokenlifecycle.DefaultPolicy(),
			want:   tokenlifecycle.ActionReauth,
		},
		{
			name:   "zero credential (nothing stored) -> reauth",
			cred:   tokenlifecycle.Credential{},
			policy: tokenlifecycle.DefaultPolicy(),
			want:   tokenlifecycle.ActionReauth,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenlifecycle.NextAction(tt.cred, now, tt.policy); got != tt.want {
				t.Errorf("NextAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeRefreshed(t *testing.T) {
	old := tokenlifecycle.Credential{
		AccessToken:  "old-at",
		RefreshToken: "old-rt",
		ExpiresAt:    now.Add(-time.Hour),
		Scopes:       []string{"mail.read", "calendar.read"},
	}
	tests := []struct {
		name      string
		old       tokenlifecycle.Credential
		refreshed tokenlifecycle.Credential
		want      tokenlifecycle.Credential
	}{
		{
			name: "refresh response omits refresh token: old one is preserved",
			old:  old,
			refreshed: tokenlifecycle.Credential{
				AccessToken: "new-at",
				ExpiresAt:   now.Add(time.Hour),
			},
			want: tokenlifecycle.Credential{
				AccessToken:  "new-at",
				RefreshToken: "old-rt",
				ExpiresAt:    now.Add(time.Hour),
			},
		},
		{
			name: "refresh response carries its own refresh token: it wins",
			old:  old,
			refreshed: tokenlifecycle.Credential{
				AccessToken:  "new-at",
				RefreshToken: "new-rt",
				ExpiresAt:    now.Add(time.Hour),
			},
			want: tokenlifecycle.Credential{
				AccessToken:  "new-at",
				RefreshToken: "new-rt",
				ExpiresAt:    now.Add(time.Hour),
			},
		},
		{
			name: "only the refresh token is back-filled: scopes and expiry come from refreshed as-is",
			old:  old,
			refreshed: tokenlifecycle.Credential{
				AccessToken: "new-at",
				// no expiry, no scopes — stays that way (donor parity)
			},
			want: tokenlifecycle.Credential{
				AccessToken:  "new-at",
				RefreshToken: "old-rt",
			},
		},
		{
			name: "both sides lack a refresh token: merged has none",
			old: tokenlifecycle.Credential{
				AccessToken: "old-at",
			},
			refreshed: tokenlifecycle.Credential{
				AccessToken: "new-at",
				ExpiresAt:   now.Add(time.Hour),
			},
			want: tokenlifecycle.Credential{
				AccessToken: "new-at",
				ExpiresAt:   now.Add(time.Hour),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenlifecycle.MergeRefreshed(tt.old, tt.refreshed)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeRefreshed() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMergeRefreshedDoesNotMutateInputs(t *testing.T) {
	old := tokenlifecycle.Credential{AccessToken: "old-at", RefreshToken: "old-rt"}
	refreshed := tokenlifecycle.Credential{AccessToken: "new-at"}
	_ = tokenlifecycle.MergeRefreshed(old, refreshed)
	if refreshed.RefreshToken != "" {
		t.Errorf("MergeRefreshed mutated its refreshed argument: RefreshToken = %q", refreshed.RefreshToken)
	}
	if old.RefreshToken != "old-rt" {
		t.Errorf("MergeRefreshed mutated its old argument: RefreshToken = %q", old.RefreshToken)
	}
}

func TestRevocationTarget(t *testing.T) {
	tests := []struct {
		name      string
		cred      tokenlifecycle.Credential
		wantToken string
		wantOK    bool
	}{
		{
			name: "refresh token preferred over access token",
			cred: tokenlifecycle.Credential{
				AccessToken:  "at",
				RefreshToken: "rt",
			},
			wantToken: "rt",
			wantOK:    true,
		},
		{
			name: "access token used when no refresh token",
			cred: tokenlifecycle.Credential{
				AccessToken: "at",
			},
			wantToken: "at",
			wantOK:    true,
		},
		{
			name: "refresh token alone is a target",
			cred: tokenlifecycle.Credential{
				RefreshToken: "rt",
			},
			wantToken: "rt",
			wantOK:    true,
		},
		{
			name:      "no tokens at all: nothing to revoke",
			cred:      tokenlifecycle.Credential{},
			wantToken: "",
			wantOK:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotToken, gotOK := tokenlifecycle.RevocationTarget(tt.cred)
			if gotToken != tt.wantToken || gotOK != tt.wantOK {
				t.Errorf("RevocationTarget() = (%q, %v), want (%q, %v)",
					gotToken, gotOK, tt.wantToken, tt.wantOK)
			}
		})
	}
}

// TestEnumStringsMatchSchema pins the wire strings shared with schema.cue —
// drift between the Go constants and the CUE enums is a bug.
func TestEnumStringsMatchSchema(t *testing.T) {
	states := map[tokenlifecycle.State]string{
		tokenlifecycle.StateValid:        "valid",
		tokenlifecycle.StateRefreshable:  "refreshable",
		tokenlifecycle.StateReauthNeeded: "reauth_needed",
		tokenlifecycle.StateNotFound:     "not_found",
	}
	for got, want := range states {
		if string(got) != want {
			t.Errorf("State constant = %q, want %q", got, want)
		}
	}
	actions := map[tokenlifecycle.Action]string{
		tokenlifecycle.ActionUse:     "use",
		tokenlifecycle.ActionRefresh: "refresh",
		tokenlifecycle.ActionReauth:  "reauth",
	}
	for got, want := range actions {
		if string(got) != want {
			t.Errorf("Action constant = %q, want %q", got, want)
		}
	}
}

// TestDefaultPolicy pins the documented 60-second default refresh skew.
func TestDefaultPolicy(t *testing.T) {
	if got := tokenlifecycle.DefaultPolicy().RefreshSkew; got != 60*time.Second {
		t.Errorf("DefaultPolicy().RefreshSkew = %v, want 60s", got)
	}
	if tokenlifecycle.DefaultRefreshSkew != 60*time.Second {
		t.Errorf("DefaultRefreshSkew = %v, want 60s", time.Duration(tokenlifecycle.DefaultRefreshSkew))
	}
}

func TestCredentialIsZero(t *testing.T) {
	tests := []struct {
		name string
		cred tokenlifecycle.Credential
		want bool
	}{
		{"zero credential", tokenlifecycle.Credential{}, true},
		{"access token only", tokenlifecycle.Credential{AccessToken: "at"}, false},
		{"refresh token only", tokenlifecycle.Credential{RefreshToken: "rt"}, false},
		{"expiry only", tokenlifecycle.Credential{ExpiresAt: now}, false},
		{"scopes only", tokenlifecycle.Credential{Scopes: []string{"s"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cred.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}
