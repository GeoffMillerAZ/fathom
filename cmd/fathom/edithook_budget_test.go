package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestSpecLineTokenBudget_MatchesADR016 ties the edit hook's whole-payload
// ceiling to the number ADR-016's Q21 amendment ratified, so a silent edit to
// either one fails loudly (P22: measured, not argued).
func TestSpecLineTokenBudget_MatchesADR016(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "ADR.md"))
	if err != nil {
		t.Fatalf("read ADR.md: %v", err)
	}
	adr := strings.Join(strings.Fields(string(raw)), " ")
	want := "Raise the whole-payload serving ceiling from 150 to " + strconv.Itoa(specLineTokenBudget) + " tokens"
	if !strings.Contains(adr, "Amendment (2026-09-24, Q21)") || !strings.Contains(adr, want) {
		t.Fatalf("specLineTokenBudget = %d, but ADR-016's ratified Q21 amendment does not say %q — change both together", specLineTokenBudget, want)
	}
}
