# scip-go — tier-A Go index (calls, def_mapping, typed_recall, file edges)

    GOBIN=<tools>/bin go install github.com/scip-code/scip-go/cmd/scip-go@v0.2.7   # go1.26.3

Run, per subject (a clean clone at the reading's SHA):

    run.sh <tools>/bin/scip-go <clone>/cobra out/go-cobra.scip   # spf13/cobra 40b5bc14 (tag v1.9.1's current target): 35 documents, 0.7 s
    run.sh <tools>/bin/scip-go <clone>/cli   out/go-cli.scip     # cli/cli v2.69.0 (45ffa3c6): 728 documents, 10.9 s

(2026-10-01, wave lb.) The measured files are 36 and 736: the index holds no document for a file no
package of the build loads (a build-tagged or generated file outside every package's file set).

What the index carries, read by `langbench.SCIPOptionsFor("go")` (LB-F65): an interface's method is written
as a term (`Getter#Get.`), as is a func-typed field, so a call through an interface or a field is a term
reference whose name is written followed by `(`; the harness counts it as a call (`scip_called_names`,
disclosed). A function referenced as a value stays a call pair, as the Go extractor records one (VS-6).
Go's predeclared functions and conversions (`len`, `append`, `make`, `string(x)`) have no symbol, so the
pairs fathom records for them (1,440 on cli/cli) are absent from the truth: a verified oracle deficiency,
disclosed in the reading. Definitions carry an `enclosing_range` except where noted in the reading (D5).

## C-R11 field-only measurement

The current audit uses Go 1.27.3 with scip-go 0.2.7 and Universal Ctags 6.2.1.
Run from each clean, pinned module, with tests included and no package filter:

```sh
"$SCIP_GO" index --module-root=. --module-version="$SUBJECT_SHA" \
  --go-version=go1.27.3 --repository-remote="$SUBJECT_REMOTE" \
  --output "$SCIP_OUTPUT" ./...
```

For cli/cli, SUBJECT_SHA is 45ffa3c668f7105e92089220d42aa64c9acbd11c and
SUBJECT_REMOTE is github.com/cli/cli; for cobra they are
40b5bc1437a564fc795d388b23835e84f54cd1d1 and github.com/spf13/cobra.
Then run from the Fathom checkout:

```sh
go run ./cmd/indexbench lang-oracle -dialect go -subject "$SUBJECT_ROOT" \
  -subject-name "$SUBJECT_NAME" -ctags "$CTAGS_BINARY" \
  -scip "$SCIP_OUTPUT" -out "$GO_READING" -commit "$FROZEN_FATHOM_COMMIT"
go run ./cmd/indexbench lang-retime -reading "$GO_READING" \
  -subject "cli/cli=$CLI_ROOT" -subject "spf13/cobra=$COBRA_ROOT" -runs 3 -write
```

Both canonical subjects feed the same reading. Repeat the producer/index/grade
sequence into fresh outputs as the independent reading. The adoption protocol
freezes those results before the separately pinned Gin holdout. All runs use
isolated Go caches, GOWORK=off, GOFLAGS=-p=2, GOMAXPROCS=2, nice 10 and a
one-minute launch load no greater than 9. Raw producer indexes remain private;
public evidence retains counts, hashes and source/build identities only.
