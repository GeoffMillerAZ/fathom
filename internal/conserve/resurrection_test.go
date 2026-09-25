package conserve

// resurrection_test.go — CV-32 (design §3.3): a function deleted in the
// range whose body reappears at head under another name is a
// resurrection finding. Includes the Blindness fixture: an old
// implementation copied behind a new name while the old package import
// is removed (vision 34 §2's SWE Refactor Bench failure mode) — the
// deterministic detector this whole plane exists to be.
//
// Resurrection honesty (amended 2026-09-24, design §3.3): the same
// body-hash match now splits on whether a retirement was actually
// CLAIMED (in.Retired — a retires: key or a gone: import row, base or
// head). TestResurrected_DeletedBodyReappears and
// TestResurrected_BlindnessFixture both now declare one, so they stay
// what they always were — proof that a claimed-gone body which reappears
// anyway is flagged as a loss.
// TestResurrected_UndeclaredMoveIsInformationalNotLoss is the OTHER path:
// the identical fixture, no declaration at all — an ordinary rename,
// informational, never a loss.

import "testing"

// oldImplBody is a substantial (>=5 statement) function body, used
// verbatim at base under one name and, in the Blindness fixture, copied
// byte-for-byte under another name at head.
const oldImplBody = `func Compute(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += i
	}
	if total > 100 {
		total = 100
	}
	total = total + 1
	return total
}`

func TestResurrected_DeletedBodyReappears(t *testing.T) {
	base := map[string]string{
		"pkg/old.go": "package pkg\n\n" + oldImplBody + "\n",
	}
	head := map[string]string{
		// Compute is gone from pkg/old.go; the identical body reappears
		// under a new name, Recompute, in a different changed file.
		"pkg/old.go": "package pkg\n\nfunc Unrelated() {}\n",
		"pkg/new.go": "package pkg\n\n" + renamedBody("Compute", "Recompute") + "\n",
	}
	in := Input{
		ChangedFiles: []string{"pkg/new.go", "pkg/old.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
		// Compute was declared retired (retires: api:pkg.Compute) — the
		// SAME range that deletes it also claims it gone, which still
		// counts here (unlike I45's ordinary excusal). The body turning
		// up anyway makes the claim false: a loss.
		Retired: Retirement{keys: []string{"api:pkg.Compute"}},
	}

	findings, missing := Resurrected(in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1", findings)
	}
	f := findings[0]
	if f.Plane != "resurrection" || f.Kind != "body_resurrected" || f.Key != "Compute" {
		t.Errorf("finding = %+v, want plane=resurrection kind=body_resurrected key=Compute", f)
	}
	if f.Declared != DeclaredNone {
		t.Errorf("Declared = %q, want none", f.Declared)
	}
	if f.Base == "" || f.Head == "" {
		t.Errorf("finding must name BOTH old.go's base site and new.go's head site: %+v", f)
	}
}

// TestResurrected_UndeclaredMoveIsInformationalNotLoss is resurrection
// honesty's other path (design §3.3, amended 2026-09-24): the IDENTICAL
// fixture to TestResurrected_DeletedBodyReappears, minus the retires:
// declaration — nothing ever claimed Compute gone, so its reappearance
// under a new name is an ordinary, undeclared rename: kind "moved",
// Declared "prior", and never a loss.
func TestResurrected_UndeclaredMoveIsInformationalNotLoss(t *testing.T) {
	base := map[string]string{
		"pkg/old.go": "package pkg\n\n" + oldImplBody + "\n",
	}
	head := map[string]string{
		"pkg/old.go": "package pkg\n\nfunc Unrelated() {}\n",
		"pkg/new.go": "package pkg\n\n" + renamedBody("Compute", "Recompute") + "\n",
	}
	in := Input{
		ChangedFiles: []string{"pkg/new.go", "pkg/old.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
		// No Retired at all — nothing was ever claimed retired.
	}

	findings, missing := Resurrected(in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1", findings)
	}
	f := findings[0]
	if f.Plane != "resurrection" || f.Kind != "moved" || f.Key != "Compute" {
		t.Errorf("finding = %+v, want plane=resurrection kind=moved key=Compute", f)
	}
	if f.Declared != DeclaredPrior {
		t.Errorf("Declared = %q, want prior (informational, never a loss)", f.Declared)
	}
	if f.Base == "" || f.Head == "" {
		t.Errorf("finding must name BOTH old.go's base site and new.go's head site: %+v", f)
	}
	if planeVerdict(nil, findings) != Conserved {
		t.Errorf("planeVerdict = %q, want conserved — an undeclared move must never be a loss", planeVerdict(nil, findings))
	}
}

// TestResurrected_BlindnessFixture is the fixture the task explicitly
// requires: an old implementation copied behind a new name while the old
// package import that made it reachable is removed — the classic
// SWE-Refactor-Bench failure mode (vision 34 §2) — AND, per resurrection
// honesty (design §3.3, amended 2026-09-24), the legacy package is
// explicitly declared retired (`gone: import example.com/x/legacy`,
// exactly the row an agent would write to satisfy the surface-absence
// gate). Resurrected must still flag it: the declared retirement is
// false, since the body demonstrably survives — the actual fake-
// preservation case ADR-044 exists to catch, not a mere undeclared move.
func TestResurrected_BlindnessFixture(t *testing.T) {
	base := map[string]string{
		// legacy/impl.go held the real implementation; caller.go imported
		// the legacy package to reach it.
		"legacy/impl.go": "package legacy\n\n" + oldImplBody + "\n",
		"caller.go":      "package pkg\n\nimport \"example.com/x/legacy\"\n\nfunc Use() int { return legacy.Compute(10) }\n",
	}
	head := map[string]string{
		// legacy/impl.go's Compute is deleted outright...
		"legacy/impl.go": "package legacy\n\n// Compute was removed.\n",
		// ...but the exact same body reappears, renamed, inlined directly
		// into caller.go — and the import of legacy is dropped, exactly
		// the "old package import is removed" tell.
		"caller.go": "package pkg\n\n" + renamedBody("Compute", "computeLocally") + "\n\nfunc Use() int { return computeLocally(10) }\n",
	}
	in := Input{
		ChangedFiles: []string{"caller.go", "legacy/impl.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
		// The refactor's own plan claims the legacy package retired —
		// exactly the "gone: import <path>" row a real refactor program
		// (ADR-044 §4) would emit for it.
		Retired: Retirement{imports: []string{"example.com/x/legacy"}},
	}

	findings, missing := Resurrected(in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("Blindness fixture: findings = %+v, want exactly 1 — resurrection must flag it", findings)
	}
	f := findings[0]
	if f.Kind != "body_resurrected" {
		t.Errorf("Kind = %q, want body_resurrected", f.Kind)
	}
	if f.Base == "" {
		t.Errorf("Base citation empty: %+v", f)
	}
	if f.Head == "" {
		t.Errorf("Head citation empty: %+v", f)
	}
}

func TestResurrected_UnchangedFunctionIsNotFlagged(t *testing.T) {
	same := map[string]string{"a.go": "package pkg\n\n" + oldImplBody + "\n"}
	in := Input{
		ChangedFiles: []string{"a.go"},
		BaseFile:     mapFile(same),
		HeadFile:     mapFile(same),
	}
	findings, _ := Resurrected(in)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none — the function never left", findings)
	}
}

func TestResurrected_FewerThanFiveStatementsNeverFlags(t *testing.T) {
	small := `func Tiny() int {
	return 1
}`
	base := map[string]string{"a.go": "package pkg\n\n" + small + "\n"}
	head := map[string]string{
		"a.go": "package pkg\n\nfunc Unrelated() {}\n",
		"b.go": "package pkg\n\n" + renamedBody("Tiny", "Renamed") + "\n",
	}
	in := Input{
		ChangedFiles: []string{"a.go", "b.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
	}
	findings, _ := Resurrected(in)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none — under the 5-statement floor", findings)
	}
}

func TestResurrected_MovedUnderSameNameIsNotResurrection(t *testing.T) {
	// A legitimate file move: Compute leaves old.go and reappears, under
	// the SAME name, in new.go. Not a resurrection — nothing changed
	// name, so there is nothing to flag.
	base := map[string]string{"old.go": "package pkg\n\n" + oldImplBody + "\n"}
	head := map[string]string{
		"old.go": "package pkg\n\nfunc Unrelated() {}\n",
		"new.go": "package pkg\n\n" + oldImplBody + "\n",
	}
	in := Input{
		ChangedFiles: []string{"new.go", "old.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
	}
	findings, _ := Resurrected(in)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none — same name, ordinary move", findings)
	}
}

func TestResurrected_StructuralChangeIsNotFlagged(t *testing.T) {
	base := map[string]string{"old.go": "package pkg\n\n" + oldImplBody + "\n"}
	structurallyDifferent := `func Recompute(m int) int {
	sum := 0
	for i := 0; i < m; i++ {
		sum += i * 2
	}
	if sum > 200 {
		sum = 200
	}
	return sum
}`
	head := map[string]string{
		"old.go": "package pkg\n\nfunc Unrelated() {}\n",
		"new.go": "package pkg\n\n" + structurallyDifferent + "\n",
	}
	in := Input{
		ChangedFiles: []string{"new.go", "old.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
	}
	findings, _ := Resurrected(in)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none — structurally different body", findings)
	}
}

// duckClaimEventsBody and sqliteClaimEventsBody are CV-M5's own fixture
// shape (docs/bench/readings/cv-m5-blindness-replay-2026-09-24.md): two
// long-lived, INDEPENDENTLY-written implementations of the same interface
// (the real ADR-025 case: DuckStore.ClaimEvents and
// SQLiteStore.ClaimEvents) sharing an identical, statement-for-statement
// body for years before one twin is deleted — clone.FuncBodyHash's
// identifier-insensitive normalization makes their differing receiver and
// method names irrelevant to the hash.
const duckClaimEventsBody = `func (s *DuckStore) ClaimEvents(id string) ([]Event, error) {
	rows, err := s.db.Query(claimEventsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.At, &e.Verdict); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}`

const sqliteClaimEventsBody = `func (s *SQLiteStore) ClaimEvents(id string) ([]Event, error) {
	rows, err := s.db.Query(claimEventsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.At, &e.Verdict); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}`

// TestResurrected_ConvergentImplementationsNeverFlag is CV-M5's own
// regression: the genuine ADR-025 DuckDB removal produced 24 false
// body_resurrected findings because Resurrected matched a deleted base
// function against ANY function in a changed file at head sharing its
// hash, without checking whether that MATCHING head function was itself
// new or changed in this range. sqlite/store.go here is a changed file
// only because of an unrelated edit elsewhere in it (an added helper
// shifting line numbers) — SQLiteStore.ClaimEvents' own body never changes
// between base and head. It must never be treated as
// DuckStore.ClaimEvents' resurrection, retirement declared or not: it was
// never new, and its hash never changed in this range — it simply always
// existed, independently, since before the range began.
func TestResurrected_ConvergentImplementationsNeverFlag(t *testing.T) {
	base := map[string]string{
		"duck/store.go":   "package duck\n\n" + duckClaimEventsBody + "\n",
		"sqlite/store.go": "package sqlite\n\nfunc unrelatedHelper() {}\n\n" + sqliteClaimEventsBody + "\n",
	}
	head := map[string]string{
		// The genuine ADR-025 shape: DuckStore's twin deleted outright.
		"duck/store.go": "package duck\n\n// DuckStore was removed (ADR-025).\n",
		// sqlite/store.go changes for a wholly unrelated reason — an extra
		// helper lands earlier in the file, shifting line numbers —
		// SQLiteStore.ClaimEvents' own body is byte-for-byte unchanged.
		"sqlite/store.go": "package sqlite\n\nfunc unrelatedHelper() {}\n\nfunc anotherHelper() {}\n\n" + sqliteClaimEventsBody + "\n",
	}
	in := Input{
		ChangedFiles: []string{"duck/store.go", "sqlite/store.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
		// The duck package is declared retired — exactly the row a real
		// refactor program (ADR-044 §4) emits for the deletion.
		Retired: Retirement{keys: []string{"api:duck.DuckStore.ClaimEvents"}},
	}

	findings, missing := Resurrected(in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none — SQLiteStore.ClaimEvents already existed at base with an unchanged body; it is not a resurrection candidate (CV-M5)", findings)
	}
}

// TestResurrected_ConvergentImplementations_SeededCopyStillFlags is the
// OTHER half of the same CV-M5 reading, in the SAME convergent-
// implementation repository as the test above: a genuinely SEEDED copy — a
// brand-new file attached to the surviving implementation, copying the
// retired body under a new name — must still flag. The fix must exempt
// only a candidate that is itself unchanged from base, never suppress a
// real resurrection merely because some unrelated, long-lived function
// elsewhere happens to share its hash.
func TestResurrected_ConvergentImplementations_SeededCopyStillFlags(t *testing.T) {
	const seededClaimEventsBody = `func (s *SQLiteStore) SeededClaimEvents(id string) ([]Event, error) {
	rows, err := s.db.Query(claimEventsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.At, &e.Verdict); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}`
	base := map[string]string{
		"duck/store.go":   "package duck\n\n" + duckClaimEventsBody + "\n",
		"sqlite/store.go": "package sqlite\n\nfunc unrelatedHelper() {}\n\n" + sqliteClaimEventsBody + "\n",
	}
	head := map[string]string{
		"duck/store.go":   "package duck\n\n// DuckStore was removed (ADR-025).\n",
		"sqlite/store.go": "package sqlite\n\nfunc unrelatedHelper() {}\n\nfunc anotherHelper() {}\n\n" + sqliteClaimEventsBody + "\n",
		// The seeded copy: a brand-new file, attached to the surviving
		// implementation, copying DuckStore.ClaimEvents' exact body under a
		// new name — the actual Blindness fixture this check exists to
		// catch.
		"sqlite/seeded.go": "package sqlite\n\n" + seededClaimEventsBody + "\n",
	}
	in := Input{
		ChangedFiles: []string{"duck/store.go", "sqlite/seeded.go", "sqlite/store.go"},
		BaseFile:     mapFile(base),
		HeadFile:     mapFile(head),
		Retired:      Retirement{keys: []string{"api:duck.DuckStore.ClaimEvents"}},
	}

	findings, missing := Resurrected(in)
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want exactly 1 — the seeded copy must still flag", findings)
	}
	f := findings[0]
	if f.Plane != "resurrection" || f.Kind != "body_resurrected" || f.Key != "DuckStore.ClaimEvents" {
		t.Errorf("finding = %+v, want plane=resurrection kind=body_resurrected key=DuckStore.ClaimEvents", f)
	}
	wantHeadPrefix := "sqlite/seeded.go:"
	if len(f.Head) < len(wantHeadPrefix) || f.Head[:len(wantHeadPrefix)] != wantHeadPrefix {
		t.Errorf("Head = %q, want the seeded file's site (prefix %q), not sqlite/store.go's unchanged SQLiteStore.ClaimEvents", f.Head, wantHeadPrefix)
	}
}

func TestResurrected_PlaneName(t *testing.T) {
	var p resurrectionPlane
	if got := p.Name(); got != "resurrection" {
		t.Errorf("Name() = %q, want resurrection", got)
	}
}

// renamedBody returns oldImplBody-shaped source with its function name
// swapped from oldName to newName — used to build a "same structure,
// different name" fixture without hand-duplicating the body text.
func renamedBody(oldName, newName string) string {
	return "func " + newName + oldImplBody[len("func "+oldName):]
}
