package main

// verify_platform_test.go -- a symbol row whose symbol is declared only in
// a Go file the indexing build context excludes (a _windows.go file is
// excluded on every host this suite runs on) is unreadable evidence: exit 2,
// never a failing claim. A symbol missing everywhere still fails (exit 1).

import (
	"strings"
	"testing"
)

func TestVerifyPlatformExcluded_ExitCodes(t *testing.T) {
	repo := newGoneFixtureRepo(t)
	repo.writePlan("winpkg/host_windows.go", "package winpkg\n\ntype winHost struct{}\n\nfunc (h *winHost) ReadHost() {}\n")
	repo.writePlan("keptpkg/kept_test.go", "package keptpkg\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) { Kept() }\n")
	onlyWin := repo.writePlan("docs/plans/win.md", "- [x] win only {id: PL-1, symbol: winHost.ReadHost, package: winpkg}\n")
	missing := repo.writePlan("docs/plans/missing.md", "- [x] ghost {id: PL-2, symbol: GhostEverywhere}\n")
	included := repo.writePlan("docs/plans/included.md", "- [x] kept {id: PL-3, symbol: Kept}\n")
	repo.commitAll("initial")
	store := ingestGoneFixture(t, repo)

	code, so, se := verifyStrict(t, repo, onlyWin, store)
	if code != 2 || !strings.Contains(so+se, "platform: defined only in winpkg/host_windows.go, excluded on ") {
		t.Fatalf("windows-only symbol: want 2 with platform detail, got %d\n%s\n%s", code, so, se)
	}
	if code, so, se := verifyStrict(t, repo, missing, store); code != 1 {
		t.Fatalf("symbol missing everywhere: want 1, got %d\n%s\n%s", code, so, se)
	}
	if code, so, se := verifyStrict(t, repo, included, store); code != 0 {
		t.Fatalf("included symbol: want 0, got %d\n%s\n%s", code, so, se)
	}
}
