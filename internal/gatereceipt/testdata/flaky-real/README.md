# Real gate-log fixture

`gate.fail.log` and `gate.pass.log` are the unedited saved stdout logs produced by
the repository's `scripts/gate-log.sh`, with Task 3.54.0 and a generated Go module
named `fixture/gate`. Both runs used the same clean git tree, gate name and Go
environment. Task's command echoes appeared only on stderr, outside these logs.

The generated repository tracked `go.mod`, `retry_test.go`, `lefthook.yml`
(`pre-push: {}`), `.gitignore` (ignoring `workspace/` and `bin/`), and this Taskfile:

```yaml
version: '3'
tasks:
  test:
    cmds:
      - go test -count=1 -p 4 -v ./...
```

`TestRetry` created `workspace/attempted` using `os.O_CREATE|os.O_EXCL`, closed it,
and called `t.Fatal("first attempt")`. When that file already existed, it passed.
It measured no time. With `GOWORK=off`, `CGO_ENABLED=0` and empty `GOFLAGS`, the
generator invoked `bash scripts/gate-log.sh test task test` from that module twice
(using the source checkout's script), failing then passing without changing the
tree. A delay between invocations kept their second-resolution headers disjoint.
`bin/fathom` was a no-op shell stub: receipt publication is outside this fixture.

Tests replay the captured logs and assert identity and one recovered failure;
they never run Go or Task, wait, or measure elapsed time.
