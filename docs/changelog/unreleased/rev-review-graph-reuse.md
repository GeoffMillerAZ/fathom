### Changed

- `review_diff` builds one call graph per cold review instead of three, and `test_impact`, `resolution`, `layers` and the receipt read it, so a refresh landing mid-review no longer makes them disagree with the verdict. A graph that fails to load is not rebuilt in the same call (`test_impact` names the failure instead of "no symbols indexed"), and a review's `suppressed` count counts each hidden symbol once.
