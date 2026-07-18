package retentionpolicy_test

import (
	"testing"
	"time"

	retentionpolicy "github.com/geoffmilleraz/agentic-kit/foundations/governance/retention-policy"
)

// ---- helpers -----------------------------------------------------------------

func fixedNow(t time.Time) time.Time { return t }

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// ---- ActionKind validation ---------------------------------------------------

func TestActionKindValidate(t *testing.T) {
	t.Parallel()

	valid := []retentionpolicy.ActionKind{
		retentionpolicy.ActionShred,
		retentionpolicy.ActionFlag,
		retentionpolicy.ActionReport,
	}
	for _, k := range valid {
		if err := k.Validate(); err != nil {
			t.Errorf("kind %q should be valid: %v", k, err)
		}
	}

	invalid := []retentionpolicy.ActionKind{"", "delete", "SHRED"}
	for _, k := range invalid {
		if err := k.Validate(); err == nil {
			t.Errorf("kind %q should be invalid but Validate returned nil", k)
		}
	}
}

// ---- Policy validation -------------------------------------------------------

func TestPolicyValidate(t *testing.T) {
	t.Parallel()

	good := retentionpolicy.Policy{
		Name:   "p1",
		MaxAge: 24 * time.Hour,
		Action: retentionpolicy.ActionShred,
	}
	if err := good.Validate(); err != nil {
		t.Errorf("valid policy: %v", err)
	}

	// Empty name.
	bad := good
	bad.Name = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty Name must be invalid")
	}

	// Zero MaxAge.
	bad = good
	bad.MaxAge = 0
	if err := bad.Validate(); err == nil {
		t.Error("zero MaxAge must be invalid")
	}

	// Negative MaxAge.
	bad = good
	bad.MaxAge = -time.Hour
	if err := bad.Validate(); err == nil {
		t.Error("negative MaxAge must be invalid")
	}

	// Invalid action.
	bad = good
	bad.Action = "unknown"
	if err := bad.Validate(); err == nil {
		t.Error("invalid Action must be invalid")
	}
}

// ---- Due: basic cases --------------------------------------------------------

func TestDueNoPolicies(t *testing.T) {
	t.Parallel()

	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "ns1", Class: "pii", CreatedAt: epoch.Add(-10 * 365 * 24 * time.Hour)},
	}
	out := retentionpolicy.Due(nil, inv, epoch)
	if len(out) != 0 {
		t.Errorf("no policies: want empty, got %d actions", len(out))
	}
}

func TestDueNoInventory(t *testing.T) {
	t.Parallel()

	policies := []retentionpolicy.Policy{
		{Name: "p1", MaxAge: 24 * time.Hour, Action: retentionpolicy.ActionShred},
	}
	out := retentionpolicy.Due(policies, nil, epoch)
	if len(out) != 0 {
		t.Errorf("no inventory: want empty, got %d actions", len(out))
	}
}

func TestDueOverageTriggersAction(t *testing.T) {
	t.Parallel()

	const maxAge = 7 * 24 * time.Hour
	policies := []retentionpolicy.Policy{
		{Name: "weekly-shred", MaxAge: maxAge, Action: retentionpolicy.ActionShred},
	}
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "ns-old", Class: "pii", CreatedAt: epoch.Add(-8 * 24 * time.Hour)},
		{Namespace: "ns-new", Class: "pii", CreatedAt: epoch.Add(-1 * 24 * time.Hour)},
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 1 {
		t.Fatalf("want 1 DueAction (ns-old), got %d", len(out))
	}
	if out[0].Entry.Namespace != "ns-old" {
		t.Errorf("want ns-old, got %q", out[0].Entry.Namespace)
	}
	if out[0].Kind != retentionpolicy.ActionShred {
		t.Errorf("want shred, got %q", out[0].Kind)
	}
}

func TestDueUnderageNotTriggered(t *testing.T) {
	t.Parallel()

	policies := []retentionpolicy.Policy{
		{Name: "p1", MaxAge: 30 * 24 * time.Hour, Action: retentionpolicy.ActionFlag},
	}
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "ns-fresh", Class: "pii", CreatedAt: epoch.Add(-10 * 24 * time.Hour)},
	}
	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 0 {
		t.Errorf("under-age entry must not trigger action, got %d actions", len(out))
	}
}

// ---- Due: filter matching ----------------------------------------------------

func TestDueNamespacePrefixFilter(t *testing.T) {
	t.Parallel()

	policies := []retentionpolicy.Policy{
		{
			Name:            "case-only-shred",
			NamespacePrefix: "case-",
			MaxAge:          24 * time.Hour,
			Action:          retentionpolicy.ActionShred,
		},
	}
	old := epoch.Add(-48 * time.Hour)
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "case-001", Class: "pii", CreatedAt: old}, // matches prefix, over age
		{Namespace: "other-ns", Class: "pii", CreatedAt: old}, // does NOT match prefix
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 1 {
		t.Fatalf("want 1 action (case-001 only), got %d", len(out))
	}
	if out[0].Entry.Namespace != "case-001" {
		t.Errorf("want case-001, got %q", out[0].Entry.Namespace)
	}
}

func TestDueClassFilter(t *testing.T) {
	t.Parallel()

	policies := []retentionpolicy.Policy{
		{Name: "pii-only", Class: "pii", MaxAge: 24 * time.Hour, Action: retentionpolicy.ActionShred},
	}
	old := epoch.Add(-48 * time.Hour)
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "ns-pii", Class: "pii", CreatedAt: old},             // matches class
		{Namespace: "ns-financial", Class: "financial", CreatedAt: old}, // wrong class
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 1 {
		t.Fatalf("want 1 action, got %d", len(out))
	}
	if out[0].Entry.Namespace != "ns-pii" {
		t.Errorf("want ns-pii, got %q", out[0].Entry.Namespace)
	}
}

func TestDueBlankFiltersMatchAll(t *testing.T) {
	t.Parallel()

	// Policy with no prefix or class filter matches everything.
	policies := []retentionpolicy.Policy{
		{Name: "catch-all", MaxAge: time.Hour, Action: retentionpolicy.ActionReport},
	}
	old := epoch.Add(-2 * time.Hour)
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "alpha", Class: "pii", CreatedAt: old},
		{Namespace: "beta", Class: "financial", CreatedAt: old},
		{Namespace: "gamma", Class: "", CreatedAt: old},
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 3 {
		t.Errorf("want 3 actions (catch-all), got %d", len(out))
	}
}

// ---- Due: first-policy wins per entry ----------------------------------------

func TestDueFirstMatchingPolicyWins(t *testing.T) {
	t.Parallel()

	// Two policies match the same entry. Only the FIRST should fire.
	old := epoch.Add(-100 * 24 * time.Hour)
	policies := []retentionpolicy.Policy{
		{Name: "flag-first", MaxAge: 30 * 24 * time.Hour, Action: retentionpolicy.ActionFlag},
		{Name: "shred-second", MaxAge: 60 * 24 * time.Hour, Action: retentionpolicy.ActionShred},
	}
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "ns-multi", Class: "pii", CreatedAt: old},
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 1 {
		t.Fatalf("want 1 action, got %d", len(out))
	}
	if out[0].Kind != retentionpolicy.ActionFlag {
		t.Errorf("first policy must win: want flag, got %q", out[0].Kind)
	}
	if out[0].Policy.Name != "flag-first" {
		t.Errorf("policy name: want flag-first, got %q", out[0].Policy.Name)
	}
}

// ---- Due: edge cases ---------------------------------------------------------

func TestDueSkipsZeroCreatedAt(t *testing.T) {
	t.Parallel()

	policies := []retentionpolicy.Policy{
		{Name: "p1", MaxAge: time.Hour, Action: retentionpolicy.ActionShred},
	}
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "ns-zero", Class: "pii", CreatedAt: time.Time{}}, // zero CreatedAt → skip
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 0 {
		t.Errorf("zero CreatedAt must be skipped, got %d actions", len(out))
	}
}

func TestDueSkipsBlankNamespace(t *testing.T) {
	t.Parallel()

	policies := []retentionpolicy.Policy{
		{Name: "p1", MaxAge: time.Hour, Action: retentionpolicy.ActionShred},
	}
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "", Class: "pii", CreatedAt: epoch.Add(-2 * time.Hour)},
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 0 {
		t.Errorf("blank namespace must be skipped, got %d actions", len(out))
	}
}

func TestDueExactlyAtMaxAgeNotDue(t *testing.T) {
	t.Parallel()

	// age == MaxAge exactly: the condition is age > MaxAge, so not triggered.
	const maxAge = 30 * 24 * time.Hour
	policies := []retentionpolicy.Policy{
		{Name: "p1", MaxAge: maxAge, Action: retentionpolicy.ActionShred},
	}
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "ns-exact", Class: "pii", CreatedAt: epoch.Add(-maxAge)},
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 0 {
		t.Errorf("age == MaxAge must NOT trigger action, got %d actions", len(out))
	}
}

func TestDueNowZeroPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Due with zero now must panic")
		}
	}()
	retentionpolicy.Due([]retentionpolicy.Policy{{Name: "p", MaxAge: time.Hour, Action: retentionpolicy.ActionShred}},
		[]retentionpolicy.NamespaceEntry{{Namespace: "ns", CreatedAt: epoch}},
		time.Time{})
}

// ---- Validate ----------------------------------------------------------------

func TestValidateSlice(t *testing.T) {
	t.Parallel()

	good := []retentionpolicy.Policy{
		{Name: "p1", MaxAge: time.Hour, Action: retentionpolicy.ActionShred},
		{Name: "p2", MaxAge: 24 * time.Hour, Action: retentionpolicy.ActionFlag},
	}
	if err := retentionpolicy.Validate(good); err != nil {
		t.Errorf("valid slice: %v", err)
	}

	bad := []retentionpolicy.Policy{
		{Name: "p1", MaxAge: time.Hour, Action: retentionpolicy.ActionShred},
		{Name: "", MaxAge: 0, Action: "bad"}, // multiple violations
	}
	if err := retentionpolicy.Validate(bad); err == nil {
		t.Error("invalid slice must return error")
	}
}

func TestValidateNilSlice(t *testing.T) {
	t.Parallel()

	if err := retentionpolicy.Validate(nil); err != nil {
		t.Errorf("nil slice must be valid: %v", err)
	}
}

// ---- DueAction fields correctness --------------------------------------------

func TestDueActionFieldsPopulated(t *testing.T) {
	t.Parallel()

	const maxAge = 7 * 24 * time.Hour
	policies := []retentionpolicy.Policy{
		{
			Name:            "weekly-pii",
			NamespacePrefix: "case-",
			Class:           "pii",
			MaxAge:          maxAge,
			Action:          retentionpolicy.ActionShred,
		},
	}
	createdAt := epoch.Add(-10 * 24 * time.Hour)
	inv := []retentionpolicy.NamespaceEntry{
		{Namespace: "case-007", Class: "pii", CreatedAt: createdAt},
	}

	out := retentionpolicy.Due(policies, inv, epoch)
	if len(out) != 1 {
		t.Fatalf("want 1 action, got %d", len(out))
	}
	da := out[0]
	if da.Entry.Namespace != "case-007" {
		t.Errorf("Entry.Namespace: want case-007, got %q", da.Entry.Namespace)
	}
	if da.Policy.Name != "weekly-pii" {
		t.Errorf("Policy.Name: want weekly-pii, got %q", da.Policy.Name)
	}
	wantAge := epoch.Sub(createdAt)
	if da.Age != wantAge {
		t.Errorf("Age: want %v, got %v", wantAge, da.Age)
	}
	if da.Kind != retentionpolicy.ActionShred {
		t.Errorf("Kind: want shred, got %q", da.Kind)
	}
}
