### Added
- `fathom export receipt --kind references` writes the declaring file and the file of each direct call or value use, each a whole-file `reference` fact that `fathom receipt verify` checks by its git rung. A Go struct field's references receipt is withheld, like its callers receipt.
- `fathom export receipt --kind certification` (`--test`/`--target` pick one certifications-layer claim) writes the target-file symbols the test reaches over direct calls. A target that names no symbol, such as an invariant or incident, is refused because its reach is vacuous.
- `fathom export receipt --require-clean` writes the receipt, then exits 1 naming each anchor that is not committed.
