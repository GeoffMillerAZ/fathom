# GT13b native CUE failure, read-only diagnosis

Root cause: `#ScannerCompleteCoverage` is a disjunction. For an actual legacy receipt with `expectedFiles=completedFiles=3` and no `reuse`, CUE 0.16/0.17 leaves both branches viable: the absent-reuse branch and a hypothetical present `reuse.files=0` branch. `cue vet -c` correctly rejects that unresolved value. The native job failed at its final CUE check, so its qualification is failed despite prior runtime checks passing.

**Recommended one-definition change** in `schema.cue`:

```cue
#ScannerCompleteCoverage: {
    expectedFiles: int
    completedFiles: int
    reuse?: #ScannerReuse
    if reuse != _|_ {
        _coverage: true & (completedFiles + reuse.files == expectedFiles)
    }
    if reuse == _|_ {
        completedFiles: expectedFiles
    }
    ...
}
```

The `...` is necessary: this named definition is otherwise closed and rejects the receipt's other fields when intersected with `#ExternalScanner`. No wire discriminant or default field is added. Keep existing partial-coverage bounds in `#ExternalScanner`/`#GitExternalScanner` and the existing application sites in complete/finding file/Git ordinary/compact variants.

Scratch proof at `/Volumes/m2ext/tmp/reuse-cue-probe/full-schema.cue`: both CUE 0.16.1 and 0.17.1 accept actual native legacy, disabled, enabled JSON/compact and late-finding receipts (6 each). Both reject 8 malformed absent/present-reuse undercoverage fixtures each. The toy probe also accepts present zero reuse when completed equals expected and rejects unknown fields. No repository file, Go code or binary changed.

Regression: vet actual disabled and enabled complete/finding file and Git ordinary, text-as-JSON, compact under both versions; remove `reuse` from an undercovered complete/finding receipt and alter present `reuse.files` to prove rejection. Re-run the native job from a fresh source pin; prior runtime passes do not qualify the failed run.
