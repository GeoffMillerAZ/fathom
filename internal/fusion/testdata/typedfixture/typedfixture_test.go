package typedfixture

import "testing"

// TestRunAdapter is the adapter-style residue case's Test* caller: it
// reaches RunAdapter directly, which in turn dispatches to Complete through
// the Adapter interface (two in-scope implementers).
func TestRunAdapter(t *testing.T) {
	if got := RunAdapter(AdapterA{}); got != "A:x" {
		t.Errorf("RunAdapter(AdapterA{}) = %q, want %q", got, "A:x")
	}
}

// TestRunContract is the Store-contract-suite residue case's Test* caller.
func TestRunContract(t *testing.T) {
	if err := RunContract(&MemStore{}); err != nil {
		t.Errorf("RunContract: %v", err)
	}
}

// TestCheckPing is the Ping residue case's Test* caller.
func TestCheckPing(t *testing.T) {
	if err := CheckPing(LocalPing{}); err != nil {
		t.Errorf("CheckPing: %v", err)
	}
}
