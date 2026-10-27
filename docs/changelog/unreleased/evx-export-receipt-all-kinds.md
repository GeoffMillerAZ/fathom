### Added
- `fathom export receipt --kind references` writes, for a Go function or method, the declaring file and the file of each direct call or value use, each a whole-file `reference` fact that `fathom receipt verify` checks by its git rung. Any other symbol is withheld, and a method value taken through an interface can still over-claim, which verify reports as a DIFFERENCE.
- `fathom export receipt --kind certification` (`--test`/`--target` pick one certifications-layer claim) writes the target-file symbols the test reaches over direct calls. A target that names no symbol, such as an invariant or incident, is refused because its reach is vacuous.
- `fathom export receipt --require-clean` writes the receipt, then exits 1 naming each anchor that is not committed.
