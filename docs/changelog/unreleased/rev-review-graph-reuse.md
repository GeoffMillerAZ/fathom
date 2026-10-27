### Changed

- `review_diff` builds one call graph per cold review instead of three: `test_impact`, `resolution`, `layers` and the receipt read the impact dimension's own graph and symbol snapshot, so a refresh landing mid-review no longer makes them disagree with the verdict. A review whose graph could not be loaded does not retry the build in the same call, and `test_impact` names the load failure instead of "no symbols indexed". On a service with scoped suppressions, the envelope's `suppressed` count for a review now counts the hidden symbols once instead of once per snapshot read.
