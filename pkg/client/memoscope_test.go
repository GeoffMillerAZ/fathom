package client_test

import (
	"context"
	"errors"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

type memoScopeBackend struct {
	id, watermark, suppression string
	err                        error
}

func (b memoScopeBackend) StoreID() string { return b.id }
func (b memoScopeBackend) StoreWatermark(context.Context, string) (string, error) {
	return b.watermark, b.err
}
func (b memoScopeBackend) SuppressionToken(string) string { return b.suppression }

type memoScopeIDOnly struct{}

func (memoScopeIDOnly) StoreID() string { return "store" }

func TestMemoScopeSharesSuppressionIdentity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		backend memoScopeBackend
		want    memo.Scope
	}{
		{"suppressed", memoScopeBackend{id: "store", watermark: "w1", suppression: "token"}, memo.Scope{Store: "store", Service: "svc", Watermark: "w1|suppressed:token"}},
		{"unsuppressed", memoScopeBackend{id: "store", watermark: "w1"}, memo.Scope{Store: "store", Service: "svc", Watermark: "w1"}},
		{"no-watermark", memoScopeBackend{id: "store", suppression: "token"}, memo.Scope{Store: "store", Service: "svc"}},
		{"watermark-error", memoScopeBackend{id: "store", watermark: "untrusted", suppression: "token", err: errors.New("unavailable")}, memo.Scope{Store: "store", Service: "svc"}},
		{"no-store-identity", memoScopeBackend{watermark: "w1", suppression: "token"}, memo.Scope{Service: "svc"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := client.MemoScope(context.Background(), tc.backend, "svc"); got != tc.want {
				t.Fatalf("scope=%+v want %+v", got, tc.want)
			}
		})
	}
	if got := client.MemoScope(context.Background(), memoScopeIDOnly{}, "svc"); got != (memo.Scope{Store: "store", Service: "svc"}) {
		t.Fatalf("optional watermark missing: %+v", got)
	}
}
