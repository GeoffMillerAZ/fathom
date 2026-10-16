package proofofwork_test

import (
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pow "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/proof-of-work"
)

// ---- helpers ----------------------------------------------------------------

var epoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

func fixedNow(t time.Time) func() time.Time { return func() time.Time { return t } }

// advancingClock returns a clock whose time can be advanced manually.
func advancingClock(start time.Time) (func() time.Time, func(time.Duration)) {
	var mu sync.Mutex
	cur := start
	get := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return cur
	}
	adv := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		cur = cur.Add(d)
	}
	return get, adv
}

const testRandHex = "deadbeefcafe0123456789abcdef0123"

// testSecret is a fixed 32-byte test key. Never use a fixed secret in production.
var testSecret = []byte("0123456789abcdef0123456789abcdef")

// otherSecret is a distinct 32-byte test key.
var otherSecret = []byte("fedcba9876543210fedcba9876543210")

func defaultCfg() pow.IssuerConfig {
	return pow.IssuerConfig{Bits: 8, MinBits: 1, TTL: time.Minute, MaxSeen: 1024}
}

// mustIssuer builds an Issuer or fails the test.
func mustIssuer(t *testing.T, secret []byte, cfg pow.IssuerConfig, now func() time.Time) *pow.Issuer {
	t.Helper()
	iss, err := pow.NewIssuer(secret, cfg, now)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	return iss
}

// ---- NewSecret ----------------------------------------------------------------

func TestNewSecret(t *testing.T) {
	s1, err := pow.NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	s2, err := pow.NewSecret()
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if len(s1) < pow.MinSecretLen {
		t.Errorf("secret too short: %d < %d", len(s1), pow.MinSecretLen)
	}
	if string(s1) == string(s2) {
		t.Error("two NewSecret calls must not return the same bytes")
	}
}

// ---- NewIssuer validation -----------------------------------------------------

func TestNewIssuer_Validation(t *testing.T) {
	now := fixedNow(epoch)
	valid := defaultCfg()

	tests := []struct {
		name    string
		secret  []byte
		cfg     pow.IssuerConfig
		now     func() time.Time
		wantErr bool
	}{
		{"valid", testSecret, valid, now, false},
		{"nil secret", nil, valid, now, true},
		{"short secret", []byte("tooshort"), valid, now, true},
		{"zero MinBits", testSecret, pow.IssuerConfig{Bits: 8, MinBits: 0, TTL: time.Minute, MaxSeen: 10}, now, true},
		{"negative MinBits", testSecret, pow.IssuerConfig{Bits: 8, MinBits: -1, TTL: time.Minute, MaxSeen: 10}, now, true},
		{"bits below floor", testSecret, pow.IssuerConfig{Bits: 4, MinBits: 8, TTL: time.Minute, MaxSeen: 10}, now, true},
		{"bits above 256", testSecret, pow.IssuerConfig{Bits: 257, MinBits: 1, TTL: time.Minute, MaxSeen: 10}, now, true},
		{"zero TTL", testSecret, pow.IssuerConfig{Bits: 8, MinBits: 1, TTL: 0, MaxSeen: 10}, now, true},
		{"zero MaxSeen", testSecret, pow.IssuerConfig{Bits: 8, MinBits: 1, TTL: time.Minute, MaxSeen: 0}, now, true},
		{"nil clock", testSecret, valid, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pow.NewIssuer(tc.secret, tc.cfg, tc.now)
			if (err != nil) != tc.wantErr {
				t.Errorf("NewIssuer error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

// ---- Issue ------------------------------------------------------------------

func TestIssue_Fields(t *testing.T) {
	iss := mustIssuer(t, testSecret, pow.IssuerConfig{Bits: 8, MinBits: 1, TTL: 5 * time.Minute, MaxSeen: 10}, fixedNow(epoch))
	c := iss.Issue(testRandHex)
	if c.Nonce == "" {
		t.Error("Nonce must not be empty")
	}
	if c.Bits != 8 {
		t.Errorf("Bits: want 8, got %d", c.Bits)
	}
	want := epoch.Add(5 * time.Minute)
	if !c.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt: want %v, got %v", want, c.ExpiresAt)
	}
	raw, err := hex.DecodeString(c.MAC)
	if err != nil {
		t.Fatalf("MAC must be hex: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("MAC must be 32 bytes (HMAC-SHA256), got %d", len(raw))
	}
}

func TestIssue_Deterministic(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c1 := iss.Issue(testRandHex)
	c2 := iss.Issue(testRandHex)
	if c1 != c2 {
		t.Errorf("Issue must be deterministic for the same inputs: %+v vs %+v", c1, c2)
	}
}

func TestIssue_DifferentRandHex(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c1 := iss.Issue("aaa")
	c2 := iss.Issue("bbb")
	if c1.Nonce == c2.Nonce {
		t.Error("different randHex must produce different nonces")
	}
	if c1.MAC == c2.MAC {
		t.Error("different nonces must produce different MACs")
	}
}

// ---- Solve + Verify round-trip ----------------------------------------------

func TestSolveVerify_RoundTrip(t *testing.T) {
	for _, bits := range []int{1, 4, 8, 12} {
		t.Run(strconv.Itoa(bits)+"bits", func(t *testing.T) {
			cfg := pow.IssuerConfig{Bits: bits, MinBits: 1, TTL: time.Minute, MaxSeen: 10}
			iss := mustIssuer(t, testSecret, cfg, fixedNow(epoch))
			c := iss.Issue(testRandHex)
			sol := pow.Solve(c)
			ok, reason := iss.Verify(c, sol)
			if !ok {
				t.Errorf("Verify failed at %d bits: %s", bits, reason)
			}
		})
	}
}

// ---- Forged challenges rejected ----------------------------------------------

func TestVerify_ForgedChallenge_Rejected(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	farFuture := epoch.Add(100 * 365 * 24 * time.Hour)

	tests := []struct {
		name string
		c    pow.Challenge
		sol  string
	}{
		{
			// The original exploit: a self-minted zero-difficulty challenge
			// with a far-future expiry and a fresh nonce per request.
			name: "bits zero, no MAC",
			c:    pow.Challenge{Nonce: "attacker-nonce-1", Bits: 0, ExpiresAt: farFuture},
			sol:  "anything",
		},
		{
			name: "bits zero, garbage MAC of correct length",
			c: pow.Challenge{
				Nonce: "attacker-nonce-2", Bits: 0, ExpiresAt: farFuture,
				MAC: strings.Repeat("ab", 32),
			},
			sol: "anything",
		},
		{
			name: "negative bits",
			c: pow.Challenge{
				Nonce: "attacker-nonce-3", Bits: -1, ExpiresAt: farFuture,
				MAC: strings.Repeat("00", 32),
			},
			sol: "anything",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := iss.Verify(tc.c, tc.sol)
			if ok {
				t.Fatal("forged challenge must be rejected")
			}
			if reason == "" {
				t.Error("rejection reason must not be empty")
			}
		})
	}
}

func TestVerify_TamperedFields_Rejected(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c := iss.Issue(testRandHex)
	sol := pow.Solve(c)

	// Sanity: untampered passes (uses its own issuer copy to avoid consuming
	// the nonce before the tamper cases run).
	{
		iss2 := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
		if ok, reason := iss2.Verify(c, sol); !ok {
			t.Fatalf("untampered challenge must verify: %s", reason)
		}
	}

	flipped := []byte(c.MAC)
	if flipped[0] == 'f' {
		flipped[0] = '0'
	} else {
		flipped[0] = 'f'
	}

	tests := []struct {
		name   string
		tamper func(pow.Challenge) pow.Challenge
	}{
		{"lowered bits", func(c pow.Challenge) pow.Challenge { c.Bits = 0; return c }},
		{"raised bits", func(c pow.Challenge) pow.Challenge { c.Bits++; return c }},
		{"changed nonce", func(c pow.Challenge) pow.Challenge { c.Nonce += "x"; return c }},
		{"extended expiry", func(c pow.Challenge) pow.Challenge { c.ExpiresAt = c.ExpiresAt.Add(time.Hour); return c }},
		{"flipped MAC bits", func(c pow.Challenge) pow.Challenge { c.MAC = string(flipped); return c }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := iss.Verify(tc.tamper(c), sol)
			if ok {
				t.Fatal("tampered challenge must be rejected")
			}
			if reason == "" {
				t.Error("rejection reason must not be empty")
			}
		})
	}
}

func TestVerify_WrongSecret_Rejected(t *testing.T) {
	issA := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	issB := mustIssuer(t, otherSecret, defaultCfg(), fixedNow(epoch))

	c := issA.Issue(testRandHex)
	sol := pow.Solve(c)

	ok, reason := issB.Verify(c, sol)
	if ok {
		t.Fatal("challenge signed under a different secret must be rejected")
	}
	if !strings.Contains(reason, "MAC") {
		t.Errorf("expected MAC-related reason, got %q", reason)
	}
}

func TestVerify_MalformedMAC_Rejected(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c := iss.Issue(testRandHex)
	sol := pow.Solve(c)

	tests := []struct {
		name string
		mac  string
	}{
		{"empty", ""},
		{"not hex", strings.Repeat("zz", 32)},
		{"truncated", c.MAC[:32]},
		{"overlong", c.MAC + "00"},
		{"odd length", c.MAC[:31]},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bad := c
			bad.MAC = tc.mac
			ok, reason := iss.Verify(bad, sol)
			if ok {
				t.Fatal("malformed MAC must be rejected")
			}
			if reason == "" {
				t.Error("rejection reason must not be empty")
			}
		})
	}
}

// ---- Difficulty floor ---------------------------------------------------------

func TestVerify_MinBitsFloor(t *testing.T) {
	// Both issuers share the secret; the "low" one signs 1-bit challenges,
	// the "high" one requires at least 8 bits. A validly signed low challenge
	// must still be rejected by the high verifier (floor is enforced even on
	// authentic challenges, e.g. after a difficulty upgrade).
	low := mustIssuer(t, testSecret, pow.IssuerConfig{Bits: 1, MinBits: 1, TTL: time.Minute, MaxSeen: 10}, fixedNow(epoch))
	high := mustIssuer(t, testSecret, pow.IssuerConfig{Bits: 8, MinBits: 8, TTL: time.Minute, MaxSeen: 10}, fixedNow(epoch))

	c := low.Issue(testRandHex)
	sol := pow.Solve(c)

	if ok, reason := low.Verify(c, sol); !ok {
		t.Fatalf("low issuer must accept its own challenge: %s", reason)
	}
	ok, reason := high.Verify(c, sol)
	if ok {
		t.Fatal("challenge below the difficulty floor must be rejected")
	}
	if !strings.Contains(reason, "minimum") {
		t.Errorf("expected floor-related reason, got %q", reason)
	}
}

// ---- Wrong / empty solution ----------------------------------------------------

func TestVerify_WrongSolution(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c := iss.Issue(testRandHex)
	ok, reason := iss.Verify(c, "definitely-wrong-solution")
	if ok {
		t.Error("wrong solution must be rejected")
	}
	if reason == "" {
		t.Error("rejection reason must not be empty")
	}
}

func TestVerify_EmptySolution(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c := iss.Issue(testRandHex)
	if ok, _ := iss.Verify(c, ""); ok {
		t.Error("empty solution must be rejected")
	}
}

func TestVerify_FailedAttemptDoesNotConsumeNonce(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c := iss.Issue(testRandHex)
	sol := pow.Solve(c)

	if ok, _ := iss.Verify(c, "wrong"); ok {
		t.Fatal("wrong solution must be rejected")
	}
	if ok, reason := iss.Verify(c, sol); !ok {
		t.Fatalf("correct solution after a failed attempt must pass: %s", reason)
	}
}

// ---- Expiry -----------------------------------------------------------------

func TestVerify_Expired(t *testing.T) {
	now, adv := advancingClock(epoch)
	iss := mustIssuer(t, testSecret, pow.IssuerConfig{Bits: 4, MinBits: 1, TTL: time.Minute, MaxSeen: 10}, now)
	c := iss.Issue(testRandHex)
	sol := pow.Solve(c)

	adv(2 * time.Minute)
	ok, reason := iss.Verify(c, sol)
	if ok {
		t.Error("expired challenge must be rejected")
	}
	if reason == "" {
		t.Error("expiry rejection must provide a reason")
	}
}

func TestVerify_NotYetExpired(t *testing.T) {
	now, adv := advancingClock(epoch)
	iss := mustIssuer(t, testSecret, pow.IssuerConfig{Bits: 4, MinBits: 1, TTL: 10 * time.Minute, MaxSeen: 10}, now)
	c := iss.Issue(testRandHex)
	sol := pow.Solve(c)

	adv(9 * time.Minute)
	if ok, reason := iss.Verify(c, sol); !ok {
		t.Errorf("should accept before expiry: %s", reason)
	}
}

// ---- Replay protection ------------------------------------------------------

func TestVerify_ReplayRejected(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c := iss.Issue(testRandHex)
	sol := pow.Solve(c)

	if ok, _ := iss.Verify(c, sol); !ok {
		t.Fatal("first verification should succeed")
	}
	ok, reason := iss.Verify(c, sol)
	if ok {
		t.Error("replay must be rejected")
	}
	if !strings.Contains(reason, "replay") {
		t.Errorf("expected replay reason, got %q", reason)
	}
}

func TestVerify_ReplayWithDifferentSolutionRejected(t *testing.T) {
	// Replay protection is keyed on the nonce, so even a DIFFERENT valid
	// solution for the same challenge must be rejected after redemption.
	cfg := pow.IssuerConfig{Bits: 4, MinBits: 1, TTL: time.Minute, MaxSeen: 10}
	iss := mustIssuer(t, testSecret, cfg, fixedNow(epoch))
	c := iss.Issue(testRandHex)

	sol1 := pow.Solve(c)
	// Find a second, distinct valid solution by continuing the search.
	first, err := strconv.Atoi(sol1)
	if err != nil {
		t.Fatalf("Solve returned non-integer solution %q", sol1)
	}
	sol2 := ""
	for i := first + 1; i < first+1_000_000; i++ {
		cand := strconv.Itoa(i)
		if pow.MeetsDifficulty(c, cand) {
			sol2 = cand
			break
		}
	}
	if sol2 == "" {
		t.Fatal("could not find a second valid solution at 4 bits")
	}

	if ok, _ := iss.Verify(c, sol1); !ok {
		t.Fatal("first verification should succeed")
	}
	if ok, _ := iss.Verify(c, sol2); ok {
		t.Error("second valid solution for a redeemed challenge must be rejected")
	}
}

func TestVerify_DistinctChallengesNotReplay(t *testing.T) {
	iss := mustIssuer(t, testSecret, pow.IssuerConfig{Bits: 4, MinBits: 1, TTL: time.Minute, MaxSeen: 10}, fixedNow(epoch))
	c1 := iss.Issue("rand1")
	c2 := iss.Issue("rand2")

	ok1, _ := iss.Verify(c1, pow.Solve(c1))
	ok2, _ := iss.Verify(c2, pow.Solve(c2))
	if !ok1 || !ok2 {
		t.Errorf("distinct challenges should both verify: ok1=%v ok2=%v", ok1, ok2)
	}
}

// ---- SeenSet bounds: TTL eviction + fail-closed cap ---------------------------

func TestSeenSet_CapFailsClosed_ThenTTLEvictionRecovers(t *testing.T) {
	now, adv := advancingClock(epoch)
	cfg := pow.IssuerConfig{Bits: 4, MinBits: 1, TTL: time.Minute, MaxSeen: 2}
	iss := mustIssuer(t, testSecret, cfg, now)

	// Fill the seen set to its cap of 2.
	c1 := iss.Issue("r1")
	c2 := iss.Issue("r2")
	if ok, reason := iss.Verify(c1, pow.Solve(c1)); !ok {
		t.Fatalf("c1: %s", reason)
	}
	if ok, reason := iss.Verify(c2, pow.Solve(c2)); !ok {
		t.Fatalf("c2: %s", reason)
	}

	// Third verification within the TTL window: every entry is unexpired, so
	// the cap forces a FAIL-CLOSED rejection — never eviction of live entries.
	c3 := iss.Issue("r3")
	ok, reason := iss.Verify(c3, pow.Solve(c3))
	if ok {
		t.Fatal("verification at capacity with no evictable entries must fail closed")
	}
	if !strings.Contains(reason, "capacity") {
		t.Errorf("expected capacity reason, got %q", reason)
	}

	// After the TTL passes, c1/c2 entries are expired and evictable; a fresh
	// challenge verifies again.
	adv(2 * time.Minute)
	c4 := iss.Issue("r4")
	if ok, reason := iss.Verify(c4, pow.Solve(c4)); !ok {
		t.Fatalf("after TTL eviction a fresh challenge must verify: %s", reason)
	}
}

func TestNewSeenSet_Direct(t *testing.T) {
	now, adv := advancingClock(epoch)
	s := pow.NewSeenSet(2, now)
	exp := epoch.Add(time.Minute)

	if got := s.TryAdd("a", exp); got != pow.AddOK {
		t.Fatalf("first add: want AddOK, got %v", got)
	}
	if got := s.TryAdd("a", exp); got != pow.AddReplay {
		t.Fatalf("duplicate add: want AddReplay, got %v", got)
	}
	if got := s.TryAdd("b", exp); got != pow.AddOK {
		t.Fatalf("second key: want AddOK, got %v", got)
	}
	if got := s.TryAdd("c", exp); got != pow.AddAtCapacity {
		t.Fatalf("at cap with live entries: want AddAtCapacity, got %v", got)
	}

	// Advance past the entries' expiry: the same key is usable again (its
	// challenge is expired, so this cannot enable a replay), and the sweep
	// frees room for new keys.
	adv(2 * time.Minute)
	exp2 := epoch.Add(3 * time.Minute)
	if got := s.TryAdd("a", exp2); got != pow.AddOK {
		t.Fatalf("re-add after expiry: want AddOK, got %v", got)
	}
	if got := s.TryAdd("c", exp2); got != pow.AddOK {
		t.Fatalf("new key after sweep: want AddOK, got %v", got)
	}
}

func TestSeenSet_MemoryBound(t *testing.T) {
	// All entries unexpired: the set must never exceed its cap, rejecting
	// every add past it (fail closed), regardless of how many keys arrive.
	s := pow.NewSeenSet(8, fixedNow(epoch))
	exp := epoch.Add(time.Hour)
	okCount := 0
	for i := 0; i < 1000; i++ {
		if s.TryAdd("key-"+strconv.Itoa(i), exp) == pow.AddOK {
			okCount++
		}
	}
	if okCount != 8 {
		t.Errorf("want exactly 8 accepted adds (the cap), got %d", okCount)
	}
}

// ---- Fail-closed plumbing via the mock ----------------------------------------

func TestVerify_FailsClosedOnSeenSetCapacity(t *testing.T) {
	mock := pow.NewMockSeenSet(pow.AddAtCapacity)
	iss, err := pow.NewIssuerWithSeenSet(testSecret, defaultCfg(), fixedNow(epoch), mock)
	if err != nil {
		t.Fatalf("NewIssuerWithSeenSet: %v", err)
	}
	c := iss.Issue(testRandHex)
	ok, reason := iss.Verify(c, pow.Solve(c))
	if ok {
		t.Fatal("AddAtCapacity from the seen set must fail closed")
	}
	if !strings.Contains(reason, "capacity") {
		t.Errorf("expected capacity reason, got %q", reason)
	}
	calls := mock.Calls()
	if len(calls) != 1 || calls[0].Key != c.Nonce {
		t.Errorf("expected one TryAdd keyed on the nonce, got %+v", calls)
	}
}

// ---- Determinism ------------------------------------------------------------

func TestSolve_Deterministic(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c := iss.Issue(testRandHex)
	if s1, s2 := pow.Solve(c), pow.Solve(c); s1 != s2 {
		t.Errorf("Solve must be deterministic: got %q then %q", s1, s2)
	}
}

// ---- Concurrency safety -------------------------------------------------------

func TestVerify_ConcurrentSafety(t *testing.T) {
	iss := mustIssuer(t, testSecret, pow.IssuerConfig{Bits: 4, MinBits: 1, TTL: time.Minute, MaxSeen: 1024}, fixedNow(epoch))
	c := iss.Issue(testRandHex)
	sol := pow.Solve(c)

	var wg sync.WaitGroup
	results := make([]bool, 50)
	for i := 0; i < 50; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _ := iss.Verify(c, sol)
			results[i] = ok
		}()
	}
	wg.Wait()

	accepted := 0
	for _, ok := range results {
		if ok {
			accepted++
		}
	}
	if accepted != 1 {
		t.Errorf("expected exactly 1 accepted verification, got %d", accepted)
	}
}

// ---- Constant-time comparison guard --------------------------------------------

// TestVerify_UsesConstantTimeCompare is a source-level regression guard: the
// MAC comparison must go through hmac.Equal (constant-time, built on
// crypto/subtle), never bytes.Equal / == / reflect.DeepEqual, so that timing
// side-channels cannot leak MAC bytes. Behavioural timing assertions are
// inherently flaky, so this asserts the construction instead.
func TestVerify_UsesConstantTimeCompare(t *testing.T) {
	src, err := os.ReadFile("proofofwork.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "hmac.Equal(") {
		t.Error("proofofwork.go must compare MACs with hmac.Equal (constant-time)")
	}
	if strings.Contains(s, "bytes.Equal(") {
		t.Error("proofofwork.go must not use bytes.Equal for MAC comparison")
	}
}

// ---- MeetsDifficulty ------------------------------------------------------------

func TestMeetsDifficulty(t *testing.T) {
	iss := mustIssuer(t, testSecret, defaultCfg(), fixedNow(epoch))
	c := iss.Issue(testRandHex)
	sol := pow.Solve(c)
	if !pow.MeetsDifficulty(c, sol) {
		t.Error("Solve's solution must meet difficulty")
	}
	harder := c
	harder.Bits = 256
	if pow.MeetsDifficulty(harder, sol) {
		t.Error("a 256-bit difficulty must not be met by a cheap solution")
	}
}

// ---- Fuzz: robustness on malformed inputs ---------------------------------------

// FuzzVerify exercises Verify with arbitrary challenge fields and solutions.
// It covers PARSING ROBUSTNESS (no panics on malformed MACs, hostile nonces,
// extreme timestamps and bit counts) and cheap behavioural invariants — it is
// NOT a validation of the cryptographic design (HMAC's security is assumed,
// not fuzzed).
func FuzzVerify(f *testing.F) {
	iss, err := pow.NewIssuer(testSecret, pow.IssuerConfig{Bits: 8, MinBits: 4, TTL: time.Minute, MaxSeen: 1024}, fixedNow(epoch))
	if err != nil {
		f.Fatalf("NewIssuer: %v", err)
	}

	// Seed with a genuine challenge plus assorted malformed shapes.
	genuine := iss.Issue(testRandHex)
	f.Add(genuine.Nonce, genuine.Bits, genuine.ExpiresAt.UnixNano(), genuine.MAC, pow.Solve(genuine))
	f.Add("", 0, int64(0), "", "")
	f.Add("nonce", -5, epoch.Add(time.Hour).UnixNano(), "zz", "sol")
	f.Add("a:b:c", 1<<30, int64(-1), strings.Repeat("00", 32), "0")
	f.Add("\x00\xff", 256, epoch.UnixNano(), strings.Repeat("ff", 64), strings.Repeat("9", 100))

	f.Fuzz(func(t *testing.T, nonce string, bits int, expNano int64, mac, solution string) {
		c := pow.Challenge{
			Nonce:     nonce,
			Bits:      bits,
			ExpiresAt: time.Unix(0, expNano).UTC(),
			MAC:       mac,
		}
		ok, reason := iss.Verify(c, solution) // must never panic
		if !ok && reason == "" {
			t.Error("every rejection must carry a reason")
		}
		if ok {
			// Acceptance invariants: a fuzzer cannot forge an HMAC, so any
			// accepted input must have carried a well-formed 32-byte MAC and
			// satisfied the floor, expiry, and difficulty checks.
			raw, err := hex.DecodeString(c.MAC)
			if err != nil || len(raw) != 32 {
				t.Errorf("accepted challenge with malformed MAC %q", c.MAC)
			}
			if c.Bits < 4 {
				t.Errorf("accepted challenge below MinBits floor: %d", c.Bits)
			}
			if !epoch.Before(c.ExpiresAt) {
				t.Error("accepted expired challenge")
			}
			if !pow.MeetsDifficulty(c, solution) {
				t.Error("accepted solution that does not meet difficulty")
			}
		}
	})
}
