# Mutation-testing fixtures (VB-34)

These are REAL `go-mutesting` transcripts, not hand-written approximations
— captured against a live run so `ParseMutationReport`'s go-mutesting text
parser is proven against the tool's actual wire format, not a guess.

## Provenance

Built and run with `github.com/zimmski/go-mutesting/cmd/go-mutesting`
(module `v0.0.0-20210610104036-6d9217011a00`, the `@latest` resolved on
2026-09-13). The tool's own dependency (`golang.org/x/tools`
`v0.0.0-20191018212557-ed542cd5b28a`) panics inside `go/types` when
compiled and run against a modern Go toolchain (go1.26.3), so the binary
was built with an OLDER toolchain instead of hand-transcribing the format:

```sh
go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest   # fails to RUN under go1.26.3 (go/types panic)
GOTOOLCHAIN=go1.19.13+auto go install github.com/zimmski/go-mutesting/cmd/go-mutesting@latest   # builds and runs fine
```

Fixture module (a throwaway two-package module, `widget` + `widget/sub`,
one function each with a test that only exercises the `>` branch, so the
`>=` mutant survives and the "return a+b" replacement mutant is caught):

```go
// widget/widget.go
package widget

func Add(a, b int) int {
	if a > 0 {
		return a + b
	}
	return b
}
```

```go
// widget/widget_test.go
package widget

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Fatalf("Add(2,3) = %d, want 5", got)
	}
}
```

(`widget/sub/sub.go` + `sub_test.go` for the multi-package fixture mirror
the same shape with `Neg`.)

## Files

- `go-mutesting-debug.txt` — `go-mutesting --debug ./...` from the
  `widget` module root: the debug transcript, where BOTH the killed (PASS)
  and survived (FAIL) mutant get a `diff -u` block (go-mutesting's default
  behavior: FAIL always gets a diff; PASS only under `--debug`).
- `go-mutesting-nodebug.txt` — the same run WITHOUT `--debug`: the PASS
  record has no diff at all (only the FAIL one does) — this is what proves
  `ParseMutationReport` must recover a killed mutant's location from its
  quoted temp-file path, not just from a diff, and that a non-debug PASS
  genuinely loses its line (kept as `Unattributed`, never dropped).
- `go-mutesting-nodebug-multipkg.txt` — the same, but run over both
  `widget` and `widget/sub`, proving the per-package path resolution
  (`sub/sub.go`) and that go-mutesting's summary counts (now 2 passed / 2
  failed) still cross-check.

Regenerate by copying the module above into a scratch directory and
running the two commands under "Files".
