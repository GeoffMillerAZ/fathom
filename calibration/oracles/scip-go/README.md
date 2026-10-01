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
