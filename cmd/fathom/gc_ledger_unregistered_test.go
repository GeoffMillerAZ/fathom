package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// PE-29 (docs/design/progress-and-economics.md §11 rule 3, PE-X5, ADR-047
// decision 13): `fathom gc` gains the class ledger-unregistered. The dry run
// counts rows and bytes per unregistered root in the verify and intent
// ledgers; --apply, run by the owner, moves them under <FATHOM_HOME>/archive
// in the owner's 2026-09-29 layout (plain JSONL, SHA256SUMS, a README row),
// never deleting a row and never losing a concurrent append.

var luRegistered = map[string]bool{"/reg/a": true, "/reg/b": true}

func luRoots() *gcLedgerRoots {
	return &gcLedgerRoots{Registered: func(root string) bool { return luRegistered[root] }}
}

func luVerifyRow(root string, n int) string {
	return fmt.Sprintf(`{"schema_version":3,"at":"2026-09-28T10:%02d:00Z","repo_root":%q,"service":"s","plan":"docs/plans/p.md","total":2,"claimed":1,"verified":1}`, n, root) + "\n"
}

func luIntentRow(root string, n int) string {
	return fmt.Sprintf(`{"schema_version":1,"kind":"debt","at":"2026-09-28T11:%02d:00Z","repo_root":%q,"by_kind":{"invariant":%d}}`, n, root, n) + "\n"
}

// luSegments splits a ledger into its rows, each with its newline (a
// trailing partial row keeps none).
func luSegments(b []byte) []string {
	var out []string
	for _, s := range strings.SplitAfter(string(b), "\n") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func luMultiset(parts ...[]string) []string {
	var all []string
	for _, p := range parts {
		all = append(all, p...)
	}
	sort.Strings(all)
	return all
}

func luRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func luAppend(t *testing.T, p string, rows ...string) {
	t.Helper()
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, r := range rows {
		if _, err := f.WriteString(r); err != nil {
			t.Fatal(err)
		}
	}
}

const luOwnerReadme = `# Ledger scratch archive, 2026-09-29

Rows moved out of the live ledgers by the owner-approved cleanup.

| archive | source ledger | rule |
|---|---|---|
| verify-ledger-scratch-2026-09-29.jsonl | verify-ledger.jsonl | repo_root not a registered service root |

Restore a ledger: append its archive file back.
`

// luFixture writes a FATHOM_HOME whose ledgers hold rows from mixed roots,
// beside the owner's 2026-09-29 archive (a scratch file, SHA256SUMS and the
// README) when owner is set. It returns the verify and intent ledgers'
// rows.
func luFixture(t *testing.T, fh string, owner bool) (verify, intent []string) {
	t.Helper()
	verify = []string{
		luVerifyRow("/reg/a", 1),
		luVerifyRow("/private/tmp/scratch-1", 2),
		luVerifyRow("/reg/b", 3),
		luVerifyRow("/Volumes/x/tmp/clone", 4),
		`{"schema_version":3,"at":"2026-09-28T10:05:00Z","service":"s","plan":"p.md"}` + "\n", // no root: never moved
		"not json {\n", // malformed: never moved
		luVerifyRow("/private/tmp/scratch-1", 7),
		luVerifyRow("/reg/a", 8),
		luVerifyRow("/private/tmp/scratch-1", 9),
		luVerifyRow("/Volumes/x/tmp/clone", 10),
	}
	intent = []string{
		luIntentRow("/reg/a", 1),
		luIntentRow("/private/tmp/wt", 2),
		`{"schema_version":1,"kind":"learning","at":"2026-09-28T11:03:00Z","id":"c1"}` + "\n",
		luIntentRow("/private/tmp/wt", 4),
		`{"schema_version":1,"kind":"oracle","at":"2026-09-2`, // a writer's partial row: carried
	}
	if err := os.MkdirAll(fh, 0o755); err != nil {
		t.Fatal(err)
	}
	luAppend(t, filepath.Join(fh, "verify-ledger.jsonl"), verify...)
	luAppend(t, filepath.Join(fh, "intent-ledger.jsonl"), intent...)
	if owner {
		arch := filepath.Join(fh, "archive")
		if err := os.MkdirAll(arch, 0o755); err != nil {
			t.Fatal(err)
		}
		scratch := luVerifyRow("/private/tmp/old", 0)
		if err := os.WriteFile(filepath.Join(arch, "verify-ledger-scratch-2026-09-29.jsonl"), []byte(scratch), 0o644); err != nil {
			t.Fatal(err)
		}
		sums := fmt.Sprintf("%x  verify-ledger-scratch-2026-09-29.jsonl\n", sha256.Sum256([]byte(scratch)))
		if err := os.WriteFile(filepath.Join(arch, "SHA256SUMS"), []byte(sums), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(arch, "README.md"), []byte(luOwnerReadme), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return verify, intent
}

func luClassItems(rep gcReport) []gcItem {
	var out []gcItem
	for _, it := range rep.Items {
		if it.Class == gcClassLedgerUnregistered {
			out = append(out, it)
		}
	}
	return out
}

func luRowsBytes(rows ...string) int64 {
	var n int64
	for _, r := range rows {
		n += int64(len(r))
	}
	return n
}

// luSums parses SHA256SUMS into name -> hex.
func luSums(t *testing.T, p string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(luRead(t, p))), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			t.Fatalf("SHA256SUMS line %q", line)
		}
		if _, dup := out[f[1]]; dup {
			t.Fatalf("SHA256SUMS names %s twice", f[1])
		}
		out[f[1]] = f[0]
	}
	return out
}

func TestGCLedgerUnregisteredArchivesNeverDeletes(t *testing.T) {
	home, fh := t.TempDir(), t.TempDir()
	t.Setenv("FATHOM_HOME", fh)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	verify, intent := luFixture(t, fh, true)
	vLedger, iLedger := filepath.Join(fh, "verify-ledger.jsonl"), filepath.Join(fh, "intent-ledger.jsonl")
	arch := filepath.Join(fh, "archive")
	vArch, iArch := filepath.Join(arch, "verify-ledger-unregistered-2026-09-29.jsonl"), filepath.Join(arch, "intent-ledger-unregistered-2026-09-29.jsonl")
	scratch := filepath.Join(arch, "verify-ledger-scratch-2026-09-29.jsonl")
	scratchBytes := luRead(t, scratch)
	ownerSums := luSums(t, filepath.Join(arch, "SHA256SUMS"))

	// Dry run: rows and bytes per unregistered root, nothing changed.
	before := gcTreeDigest(t, fh)
	dry := runGC(gcOptions{Home: home, Now: now, LedgerRoots: luRoots()})
	if after := gcTreeDigest(t, fh); after != before {
		t.Fatal("the dry run changed FATHOM_HOME")
	}
	type key struct{ ledger, root string }
	want := map[key][2]int64{
		{"verify-ledger.jsonl", "/private/tmp/scratch-1"}: {3, luRowsBytes(verify[1], verify[6], verify[8])},
		{"verify-ledger.jsonl", "/Volumes/x/tmp/clone"}:   {2, luRowsBytes(verify[3], verify[9])},
		{"intent-ledger.jsonl", "/private/tmp/wt"}:        {2, luRowsBytes(intent[1], intent[3])},
	}
	got := map[key][2]int64{}
	for _, it := range luClassItems(dry) {
		if it.Action != "archive" || it.Done || it.Reason == "" || !strings.Contains(it.Reason, it.Root) {
			t.Errorf("dry-run item %+v: want an undone archive naming its root", it)
		}
		got[key{filepath.Base(it.Path), it.Root}] = [2]int64{int64(it.Rows), it.Bytes}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("dry run per root (rows, bytes) = %v, want %v", got, want)
	}
	if _, ok := dry.Reclaimable[gcClassLedgerUnregistered]; ok {
		t.Fatalf("moved rows reclaim nothing: %v", dry.Reclaimable)
	}

	// Apply, with a writer appending between the read and the swap: one
	// registered row and one unregistered row (no sleeps, no clocks).
	late := []string{luVerifyRow("/reg/b", 11), luVerifyRow("/private/tmp/late", 12)}
	hooked := 0
	o := gcOptions{Home: home, Now: now, Apply: true, LedgerRoots: luRoots(), ledgerBeforeSwap: func(ledger string) {
		if filepath.Base(ledger) == "verify-ledger.jsonl" {
			hooked++
			luAppend(t, ledger, late...)
		}
	}}
	rep := runGC(o)
	if len(rep.Errors) > 0 || hooked != 1 {
		t.Fatalf("apply errors %v (hook ran %d times)", rep.Errors, hooked)
	}
	for _, it := range luClassItems(rep) {
		if !it.Done || it.Error != "" {
			t.Errorf("applied item %+v not done", it)
		}
	}
	vMoved := []string{verify[1], verify[3], verify[6], verify[8], verify[9]}
	vKept := append([]string{verify[0], verify[2], verify[4], verify[5], verify[7]}, late...)
	if g, w := string(luRead(t, vArch)), strings.Join(vMoved, ""); g != w {
		t.Fatalf("verify archive is not the moved rows byte-identical in order:\n%s\nwant\n%s", g, w)
	}
	if g, w := string(luRead(t, vLedger)), strings.Join(vKept, ""); g != w {
		t.Fatalf("live verify ledger is not the kept rows plus the concurrent appends in order:\n%s\nwant\n%s", g, w)
	}
	iMoved := []string{intent[1], intent[3]}
	iKept := []string{intent[0], intent[2], intent[4]}
	if g, w := string(luRead(t, iArch)), strings.Join(iMoved, ""); g != w {
		t.Fatalf("intent archive = %q, want %q", g, w)
	}
	if g, w := string(luRead(t, iLedger)), strings.Join(iKept, ""); g != w {
		t.Fatalf("live intent ledger = %q, want %q (the partial row carried)", g, w)
	}
	// Moved, never deleted: live ∪ archive is the original multiset.
	if g, w := luMultiset(luSegments(luRead(t, vLedger)), luSegments(luRead(t, vArch))), luMultiset(verify, late); fmt.Sprint(g) != fmt.Sprint(w) {
		t.Fatalf("verify rows lost or duplicated:\n%v\nwant\n%v", g, w)
	}
	if g, w := luMultiset(luSegments(luRead(t, iLedger)), luSegments(luRead(t, iArch))), luMultiset(intent); fmt.Sprint(g) != fmt.Sprint(w) {
		t.Fatalf("intent rows lost or duplicated:\n%v\nwant\n%v", g, w)
	}
	// SHA256SUMS covers what gc wrote and keeps the owner's lines; the
	// owner's scratch archive is never rewritten.
	sums := luSums(t, filepath.Join(arch, "SHA256SUMS"))
	for _, p := range []string{vArch, iArch} {
		if w := fmt.Sprintf("%x", sha256.Sum256(luRead(t, p))); sums[filepath.Base(p)] != w {
			t.Errorf("SHA256SUMS %s = %q, want %q", filepath.Base(p), sums[filepath.Base(p)], w)
		}
	}
	if sums["verify-ledger-scratch-2026-09-29.jsonl"] != ownerSums["verify-ledger-scratch-2026-09-29.jsonl"] {
		t.Error("the owner's SHA256SUMS line changed")
	}
	if string(luRead(t, scratch)) != string(scratchBytes) {
		t.Error("the owner's scratch archive was rewritten")
	}
	// One README row per archive file, inside the owner's table.
	readme := string(luRead(t, filepath.Join(arch, "README.md")))
	for _, p := range []string{vArch, iArch} {
		if n := strings.Count(readme, "| "+filepath.Base(p)+" |"); n != 1 {
			t.Errorf("README names %s %d times, want once:\n%s", filepath.Base(p), n, readme)
		}
	}
	if !strings.Contains(readme, "| verify-ledger-scratch-2026-09-29.jsonl | verify-ledger.jsonl | repo_root not a registered service root |\n| verify-ledger-unregistered-2026-09-29.jsonl | verify-ledger.jsonl |") ||
		!strings.HasSuffix(readme, "\nRestore a ledger: append its archive file back.\n") {
		t.Errorf("the README row is not appended to the owner's table:\n%s", readme)
	}
	// Nothing left behind: no temp file beside the ledgers or in archive/.
	for _, d := range []string{fh, arch} {
		ents, _ := os.ReadDir(d)
		for _, e := range ents {
			if strings.Contains(e.Name(), ".gc-") || strings.HasSuffix(e.Name(), ".tmp") {
				t.Errorf("apply left %s in %s", e.Name(), d)
			}
		}
	}

	// A second apply on the same day appends to the same archive file.
	more := []string{luVerifyRow("/private/tmp/scratch-2", 13), luVerifyRow("/private/tmp/scratch-2", 14)}
	luAppend(t, vLedger, more...)
	rep2 := runGC(gcOptions{Home: home, Now: now.Add(3 * time.Hour), Apply: true, LedgerRoots: luRoots()})
	if len(rep2.Errors) > 0 {
		t.Fatalf("second apply errors: %v", rep2.Errors)
	}
	vMoved2 := append(append(append([]string{}, vMoved...), late[1]), more...)
	if g, w := string(luRead(t, vArch)), strings.Join(vMoved2, ""); g != w {
		t.Fatalf("second apply did not append to the day's archive:\n%s\nwant\n%s", g, w)
	}
	if g, w := string(luRead(t, vLedger)), strings.Join([]string{verify[0], verify[2], verify[4], verify[5], verify[7], late[0]}, ""); g != w {
		t.Fatalf("live verify ledger after the second apply:\n%s\nwant\n%s", g, w)
	}
	if g, w := luMultiset(luSegments(luRead(t, vLedger)), luSegments(luRead(t, vArch))), luMultiset(verify, late, more); fmt.Sprint(g) != fmt.Sprint(w) {
		t.Fatalf("second apply lost or duplicated rows")
	}
	sums = luSums(t, filepath.Join(arch, "SHA256SUMS"))
	if w := fmt.Sprintf("%x", sha256.Sum256(luRead(t, vArch))); sums[filepath.Base(vArch)] != w {
		t.Errorf("SHA256SUMS not updated after the second apply")
	}
	if n := strings.Count(string(luRead(t, filepath.Join(arch, "README.md"))), "| "+filepath.Base(vArch)+" |"); n != 1 {
		t.Errorf("the second apply added a second README row for %s (%d)", filepath.Base(vArch), n)
	}
	// Restore is `cat <archive> >> <ledger>`.
	restored := append(luRead(t, vLedger), luRead(t, vArch)...)
	if g, w := luMultiset(luSegments(restored)), luMultiset(verify, late, more); fmt.Sprint(g) != fmt.Sprint(w) {
		t.Fatal("cat <archive> >> <ledger> does not restore every row")
	}
}

// With no archive/ yet, --apply creates it: the archive file, SHA256SUMS
// and a README whose table has the owner's header.
func TestGCLedgerUnregisteredCreatesArchiveLayout(t *testing.T) {
	home, fh := t.TempDir(), t.TempDir()
	t.Setenv("FATHOM_HOME", fh)
	luFixture(t, fh, false)
	rep := runGC(gcOptions{Home: home, Now: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Apply: true, LedgerRoots: luRoots()})
	if len(rep.Errors) > 0 {
		t.Fatalf("apply errors: %v", rep.Errors)
	}
	arch := filepath.Join(fh, "archive")
	readme := string(luRead(t, filepath.Join(arch, "README.md")))
	if !strings.Contains(readme, "| archive | source ledger | rule |\n|---|---|---|\n| verify-ledger-unregistered-2026-10-01.jsonl | verify-ledger.jsonl |") ||
		!strings.Contains(readme, "| intent-ledger-unregistered-2026-10-01.jsonl | intent-ledger.jsonl |") ||
		!strings.Contains(readme, "cat <archive> >> <ledger>") {
		t.Fatalf("created README:\n%s", readme)
	}
	sums := luSums(t, filepath.Join(arch, "SHA256SUMS"))
	if len(sums) != 2 {
		t.Fatalf("SHA256SUMS covers %v, want the two archives gc wrote", sums)
	}
}

// Any failure leaves the live ledger untouched and names the error: here
// SHA256SUMS cannot be written after the archive append, which is undone.
func TestGCLedgerUnregisteredFailureLeavesLedgerUntouched(t *testing.T) {
	home, fh := t.TempDir(), t.TempDir()
	t.Setenv("FATHOM_HOME", fh)
	luFixture(t, fh, false)
	arch := filepath.Join(fh, "archive")
	if err := os.MkdirAll(filepath.Join(arch, "SHA256SUMS"), 0o755); err != nil {
		t.Fatal(err)
	}
	vBefore, iBefore := luRead(t, filepath.Join(fh, "verify-ledger.jsonl")), luRead(t, filepath.Join(fh, "intent-ledger.jsonl"))
	rep := runGC(gcOptions{Home: home, Now: time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC), Apply: true, LedgerRoots: luRoots()})
	if len(rep.Errors) == 0 || !strings.Contains(strings.Join(rep.Errors, " "), "SHA256SUMS") {
		t.Fatalf("errors %v: want the failure named", rep.Errors)
	}
	for _, it := range luClassItems(rep) {
		if it.Done || it.Error == "" {
			t.Errorf("failed item %+v: want not done, with its error", it)
		}
	}
	if string(luRead(t, filepath.Join(fh, "verify-ledger.jsonl"))) != string(vBefore) || string(luRead(t, filepath.Join(fh, "intent-ledger.jsonl"))) != string(iBefore) {
		t.Fatal("a failed apply changed a live ledger")
	}
	for _, name := range []string{"verify-ledger-unregistered-2026-09-29.jsonl", "intent-ledger-unregistered-2026-09-29.jsonl", "README.md"} {
		if _, err := os.Stat(filepath.Join(arch, name)); !os.IsNotExist(err) {
			t.Errorf("a failed apply left %s (%v)", name, err)
		}
	}
	ents, _ := os.ReadDir(fh)
	for _, e := range ents {
		if strings.Contains(e.Name(), ".gc-") {
			t.Errorf("a failed apply left the temp file %s", e.Name())
		}
	}
}

// An unread registry changes nothing and says why: one report item, no
// counts guessed.
func TestGCLedgerUnregisteredUnreadRegistryChangesNothing(t *testing.T) {
	home, fh := t.TempDir(), t.TempDir()
	t.Setenv("FATHOM_HOME", fh)
	luFixture(t, fh, true)
	before := gcTreeDigest(t, fh)
	rep := runGC(gcOptions{Home: home, Apply: true, LedgerRoots: &gcLedgerRoots{Unread: "the daemon did not answer list_services: connection refused"}})
	if after := gcTreeDigest(t, fh); after != before {
		t.Fatal("an unread registry changed FATHOM_HOME")
	}
	items := luClassItems(rep)
	if len(items) != 1 || items[0].Action != "report" || items[0].Rows != 0 || items[0].Bytes != 0 || items[0].Done ||
		!strings.Contains(items[0].Reason, "connection refused") {
		t.Fatalf("unread registry items = %+v, want one report naming why, no counts", items)
	}
}

// The daemon's daily pass (Apply with no roots) and doctor's dry run never
// plan the class.
func TestGCLedgerUnregisteredNeverPlannedWithoutRoots(t *testing.T) {
	home, fh := t.TempDir(), t.TempDir()
	t.Setenv("FATHOM_HOME", fh)
	luFixture(t, fh, true)
	before := gcTreeDigest(t, fh)
	rep := runGC(gcOptions{Home: home, TmpDir: t.TempDir(), Apply: true, Now: time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)})
	if items := luClassItems(rep); len(items) != 0 {
		t.Fatalf("the daily pass planned %+v", items)
	}
	d := checkDiskRetentionFromDisk(t.TempDir())
	if strings.Contains(d.Reason, gcClassLedgerUnregistered) {
		t.Fatalf("doctor's dry run planned the class: %s", d.Reason)
	}
	if after := gcTreeDigest(t, fh); after != before {
		t.Fatal("the daily pass or doctor changed the ledgers")
	}
}
