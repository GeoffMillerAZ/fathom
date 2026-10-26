# Scaffold rollback closure review

Scope: current `docs/ci-qualification-closure` diff for `cmd/kit-new/main_test.go`, rollback evidence README, GT-C3 design and ADR 0039 text. Read-only review; no builds or tests run here.

**No production or documentation blocker found.** `git diff --check` is clean and `cmd/kit-new/main.go` has no diff. Go's Windows `os.Chmod` observes the owner-write bit, so `0400` makes the replacement read-only where the previous `0600` might not change it. The `t.Cleanup` restoration runs before `t.TempDir` cleanup. The rollback README accurately separates the green Linux CI run from native Windows qualification. The GT-C3/ADR rejection describes an unadmitted standalone Luhn profile and makes no card-authenticity or PII-absence claim.

**Minor test reliability finding:** `main_test.go:524` ignores the `os.Chmod(path, 0600)` restoration error. If restoration fails, the test may report only a later temporary-directory cleanup error or obscure the direct cause. Report that error with `t.Errorf` inside the cleanup callback. This does not alter production behavior and is not a rollback safety blocker.

Evidence: GitHub run 40093730634 concluded success at c7633a428d4203418b24313436a626be4506a06a; full gate, WASI governance, and WASM example steps succeeded. I did not run them. Native Windows remains unqualified. Fathom review_diff gave incomplete working-tree dimensions, so this judgment is based on the actual source diff and CI run metadata.
