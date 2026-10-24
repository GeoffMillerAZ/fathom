package wiring

import "testing"

// A legacy key already at the template still needs an ownership record.
// This exercises an adoption with no file write to carry the ledger flush.
func TestAdoptUntrackedCurrentKeyRecordsOwnership(t *testing.T) {
	root := t.TempDir()
	key := Key{File: ".claude/settings.json", Kind: KindJSONHook, Path: []string{"Stop", ""}, Marker: "fathom stop-hook", Body: "fathom stop-hook", Timeout: 10, AdoptUntracked: true}
	content, err := key.place("{}")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, key.File, content)
	res, err := ApplyWiring(root, Spec{Keys: []Key{key}}, ApplyOptions{Now: fixedClock()})
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := OwnedKeys(ledger, key.File); len(got) != 1 || got[0].ID != key.ID() {
		t.Fatalf("adoption not recorded: %v", got)
	}
	if got := read(t, root, key.File); got != content {
		t.Fatal("current key changed during adoption")
	}
	if res.Receipt == nil {
		t.Fatal("adoption has no undo receipt")
	}
	if _, err := Undo(root, res.Receipt.ID, ApplyOptions{Now: fixedClock()}); err != nil {
		t.Fatal(err)
	}
	ledger, err = LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(OwnedKeys(ledger, key.File)) != 0 {
		t.Fatal("undo retained adopted ownership")
	}
}
